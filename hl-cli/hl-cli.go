package main

import (
    "context"
    "flag"
    "fmt"
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

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

type commandData struct {
    Name string
    Help string
    Run func()
}

var commands = []commandData{
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

func main() {
    if len(os.Args) < 2 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")
        usage()
        return
    }

    cmdArg := os.Args[1]

    var cmd commandData
    ok := false
    for _, cmd = range commands {
        if cmdArg == cmd.Name {
            ok = true
            break
        }
    }

    if !ok {
        fmt.Fprintln(os.Stderr, "Error: <command> not recognized")
        usage()
        return
    }

    cmd.Run()
}

func usage() {
    exeName := getExeName()
    fmt.Fprintln(os.Stderr, "Usage:", exeName, "<command>\n")
    fmt.Fprintln(os.Stderr, "<command>")
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
    bootstrapFlag := addFlags.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")

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
    addFlags.Parse(os.Args[2:])
    
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
            panic(err)
        }
        fmt.Println("Hashed file:", hash)
    } else if *stdinFlag {
        hash, err = stdinHash()
        if err != nil {
            panic(err)
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

    ctx, node, err := setupNode(*bootstrapFlag)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    respStr, err := hashlookup.AddHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName, hash, dockerId)
    if err != nil {
        panic(err)
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
    bootstrapFlag := getFlags.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")

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
    getFlags.Parse(os.Args[2:])

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

    ctx, node, err := setupNode(*bootstrapFlag)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    contentHash, dockerHash, err := hashlookup.GetHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        panic(err)
    }
    fmt.Println(
        "Response: Content Hash:", contentHash, ", Docker Hash:", dockerHash)
}

func listCmd() {
    listFlags := flag.NewFlagSet("list", flag.ExitOnError)
    bootstrapFlag := listFlags.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")
    listFlags.Parse(os.Args[2:])

    ctx, node, err := setupNode(*bootstrapFlag)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    serviceNames, contentHashes, dockerHashes, err :=
        hashlookup.ListHashesWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery)
    if err != nil {
        panic(err)
    }

    fmt.Println("Response:")
    for i := 0; i < len(serviceNames); i++ {
        fmt.Printf("Service Name: %s, Content Hash: %s, Docker Hash: %s\n",
            serviceNames[i], contentHashes[i], dockerHashes[i])
    }
}

func deleteCmd() {
    deleteFlags := flag.NewFlagSet("delete", flag.ExitOnError)
    bootstrapFlag := deleteFlags.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")

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
    deleteFlags.Parse(os.Args[2:])

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

    ctx, node, err := setupNode(*bootstrapFlag)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    respStr, err := hashlookup.DeleteHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        panic(err)
    }

    fmt.Println("Response:", respStr)
}

func setupNode(bootstrap string) (
    ctx context.Context, node p2pnode.Node, err error) {

    ctx = context.Background()
    nodeConfig := p2pnode.NewConfig()
    if bootstrap != "" {
        nodeConfig.BootstrapPeers = []string{bootstrap}
    }
    node, err = p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return ctx, node, err
    }
    return ctx, node, nil
}