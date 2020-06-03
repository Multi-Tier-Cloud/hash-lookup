/* Copyright 2020 Multi-Tier-Cloud Development Team
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "io/ioutil"
    "os"
    "path/filepath"

    "github.com/ipfs/go-blockservice"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/coreunix"
    "github.com/ipfs/go-ipfs-files"
    dag "github.com/ipfs/go-merkledag"
    dagtest "github.com/ipfs/go-merkledag/test"
    "github.com/ipfs/go-mfs"
    "github.com/ipfs/go-unixfs"

    "github.com/multiformats/go-multiaddr"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/util"
    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

type commandData struct {
    Name string
    Help string
    Run func()
}

var (
    commands = []commandData{
        commandData{
            "add",
            "Hash a microservice and add it to the hash-lookup service",
            addCmd,
        },
        commandData{
            "get",
            "Get the content hash and Docker ID of a microservice",
            getCmd,
        },
        commandData{
            "list",
            "List all microservices and data stored by the hash-lookup service",
            listCmd,
        },
        commandData{
            "delete",
            "Delete a microservice entry",
            deleteCmd,
        },
    }

    bootstraps *[]multiaddr.Multiaddr
)

func main() {
    var err error
    if bootstraps, err = util.AddBootstrapFlags(); err != nil {
        log.Fatalln(err)
    }
    flag.Usage = usage
    flag.Parse()

    if len(*bootstraps) == 0 {
        // TODO: Fallback to checking environment variables for bootstrap?
        fmt.Println("Error: Must specify the multiaddr of at least one bootstrap node\n")
        usage()
        os.Exit(1)
    }

    if flag.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments\n")
        usage()
        os.Exit(1)
    }

    positionalArgs := flag.Args()
    cmdArg := positionalArgs[0]

    var cmd commandData
    ok := false
    for _, cmd = range commands {
        if cmdArg == cmd.Name {
            ok = true
            break
        }
    }

    if !ok {
        fmt.Fprintf(os.Stderr, "Error: Command '%s' not recognized\n\n", cmdArg)
        usage()
        os.Exit(1)
    }

    cmd.Run()
}

func usage() {
    exeName := getExeName()
    fmt.Fprintf(os.Stderr, "Usage of %s:\n", exeName)
    fmt.Fprintf(os.Stderr,
        "$ %s -bootstrap <multiaddr> <command>\n", exeName)

    flag.PrintDefaults()

    fmt.Fprintf(os.Stderr, "\nAvailable commands are:\n")
    for _, cmd := range commands {
        fmt.Fprintln(os.Stderr, "  " + cmd.Name)
        fmt.Fprintln(os.Stderr, "        " + cmd.Help)
    }
}

func getExeName() (exeName string) {
    return filepath.Base(os.Args[0])
}

func addCmd() {
    addFlags := flag.NewFlagSet("add", flag.ExitOnError)
    fileFlag := addFlags.String("file", "",
        "Hash microservice content from file, or directory (recursively)")
    stdinFlag := addFlags.Bool("stdin", false,
        "Hash microservice content from stdin")
    noAddFlag := addFlags.Bool("no-add", false,
        "Only hash the given content, do not add it to hash-lookup service")

    addUsage := func() {
        exeName := getExeName()
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "add [<options>] <docker-id> <name>")
        fmt.Fprintln(os.Stderr,
`
<docker-id>
        Dockerhub ID of microservice (<username>/<repository>@sha256:<hash>)

<name>
        Name of microservice to associate hashed content with

<options>`)
        addFlags.PrintDefaults()
    }

    addFlags.Usage = addUsage
    addFlags.Parse(flag.Args()[1:])

    if len(addFlags.Args()) < 2 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")
        addUsage()
        return
    }

    if len(addFlags.Args()) > 2 {
        fmt.Fprintln(os.Stderr, "Error: too many arguments")
        addUsage()
        return
    }

    if *fileFlag != "" && *stdinFlag {
        fmt.Fprintln(os.Stderr,
            "Error: --file and --stdin are mutually exclusive options")
        addUsage()
        return
    }

    var hash string = ""
    var err error = nil

    if *fileFlag != "" {
        hash, err = fileHash(*fileFlag)
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Println("Hashed file:", hash)
    } else if *stdinFlag {
        hash, err = stdinHash()
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Println("Hashed stdin:", hash)
    } else {
        fmt.Fprintln(os.Stderr,
            "Error: must specify either the --file or --stdin options")
        addUsage()
        return
    }

    if *noAddFlag {
        return
    }

    dockerId := addFlags.Arg(0)
    serviceName := addFlags.Arg(1)
    fmt.Printf("Adding %s:{ContentHash:%s, DockerHash:%s}\n",
        serviceName, hash, dockerId)

    ctx, node, err := setupNode(*bootstraps)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    respStr, err := hashlookup.AddHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName, hash, dockerId)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:", respStr)
}

func fileHash(path string) (hash string, err error) {
    stat, err := os.Lstat(path)
    if err != nil {
        return "", err
    }

    fileNode, err := files.NewSerialFile(path, false, stat)
    if err != nil {
        return "", err
    }
    defer fileNode.Close()

    return getHash(fileNode)
}

func stdinHash() (hash string, err error) {
    stdinData, err := ioutil.ReadAll(os.Stdin)
    if err != nil {
        return "", err
    }

    stdinFile := files.NewBytesFile(stdinData)

    return getHash(stdinFile)
}

func getHash(fileNode files.Node) (hash string, err error) {
    ctx := context.Background()
    nilIpfsNode, err := core.NewNode(ctx, &core.BuildCfg{NilRepo: true})
    if err != nil {
        return "", err
    }

    bserv := blockservice.New(nilIpfsNode.Blockstore, nilIpfsNode.Exchange)
    dserv := dag.NewDAGService(bserv)

    fileAdder, err := coreunix.NewAdder(
        ctx, nilIpfsNode.Pinning, nilIpfsNode.Blockstore, dserv)
    if err != nil {
        return "", err
    }

    fileAdder.Pin = false
    fileAdder.CidBuilder = dag.V0CidPrefix()

    mockDserv := dagtest.Mock()
    emptyDirNode := unixfs.EmptyDirNode()
    emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
    mfsRoot, err := mfs.NewRoot(ctx, mockDserv, emptyDirNode, nil)
    if err != nil {
        return "", err
    }
    fileAdder.SetMfsRoot(mfsRoot)

    dagIpldNode, err := fileAdder.AddAllAndPin(fileNode)
    if err != nil {
        return "", err
    }

    hash = dagIpldNode.String()
    return hash, nil
}

func getCmd() {
    getFlags := flag.NewFlagSet("get", flag.ExitOnError)

    getUsage := func() {
        exeName := getExeName()
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "get [<options>] <name>")
        fmt.Fprintln(os.Stderr,
`
<name>
        Name of microservice to get hash of

<options>`)
        getFlags.PrintDefaults()
    }

    getFlags.Usage = getUsage
    getFlags.Parse(flag.Args()[1:])

    if len(getFlags.Args()) < 1 {
        fmt.Fprintln(os.Stderr, "Error: missing required argument <name>")
        getUsage()
        return
    }

    if len(getFlags.Args()) > 1 {
        fmt.Fprintln(os.Stderr, "Error: too many arguments")
        getUsage()
        return
    }

    serviceName := getFlags.Arg(0)

    ctx, node, err := setupNode(*bootstraps)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    contentHash, dockerHash, err := hashlookup.GetHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }
    fmt.Println(
        "Response: Content Hash:", contentHash, ", Docker Hash:", dockerHash)
}

func listCmd() {
    listFlags := flag.NewFlagSet("list", flag.ExitOnError)
    listFlags.Parse(flag.Args()[1:])

    ctx, node, err := setupNode(*bootstraps)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    serviceNames, contentHashes, dockerHashes, err :=
        hashlookup.ListHashesWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:")
    for i := 0; i < len(serviceNames); i++ {
        fmt.Printf("Service Name: %s, Content Hash: %s, Docker Hash: %s\n",
            serviceNames[i], contentHashes[i], dockerHashes[i])
    }
}

func deleteCmd() {
    deleteFlags := flag.NewFlagSet("delete", flag.ExitOnError)

    deleteUsage := func() {
        exeName := getExeName()
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "delete [<options>] <name>")
        fmt.Fprintln(os.Stderr,
`
<name>
        Name of microservice to delete

<options>`)
        deleteFlags.PrintDefaults()
    }

    deleteFlags.Usage = deleteUsage
    deleteFlags.Parse(flag.Args()[1:])

    if len(deleteFlags.Args()) < 1 {
        fmt.Fprintln(os.Stderr, "Error: missing required argument <name>")
        deleteUsage()
        return
    }

    if len(deleteFlags.Args()) > 1 {
        fmt.Fprintln(os.Stderr, "Error: too many arguments")
        deleteUsage()
        return
    }

    serviceName := deleteFlags.Arg(0)

    ctx, node, err := setupNode(*bootstraps)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    respStr, err := hashlookup.DeleteHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:", respStr)
}

func setupNode(bootstraps []multiaddr.Multiaddr) (
    ctx context.Context, node p2pnode.Node, err error) {

    ctx = context.Background()
    nodeConfig := p2pnode.NewConfig()
    if len(bootstraps) > 0 {
        nodeConfig.BootstrapPeers = bootstraps
    }
    node, err = p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return ctx, node, err
    }
    return ctx, node, nil
}
