package main

import (
	"bufio"
	"context"
	"encoding/base64"
    "encoding/json"
    "errors"
    "flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	
	"github.com/ipfs/go-blockservice"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/coreunix"
    "github.com/ipfs/go-ipfs-files"
    dag "github.com/ipfs/go-merkledag"
    dagtest "github.com/ipfs/go-merkledag/test"
    "github.com/ipfs/go-mfs"
    "github.com/ipfs/go-unixfs"

    "golang.org/x/crypto/ssh/terminal"

	"github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

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
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "add [<options>] <docker-image> <name>")
        fmt.Fprintln(os.Stderr,
`
<docker-image>
        Docker image of microservice to push to DockerHub (<username>/<repository>:<tag>)

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

    dockerImage := addFlags.Arg(0)
    serviceName := addFlags.Arg(1)

    digest, err := pushImage(dockerImage)
    if err != nil {
        panic(err)
    }

    fmt.Println("Pushed to DockerHub successfully")

    parts := strings.Split(dockerImage, ":")
    dockerId := parts[0] + "@" + digest

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

func getAuth() (string, error) {
    fmt.Println("DockerHub login")
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Print("Username: ")
    scanner.Scan()
    username := scanner.Text()
    fmt.Print("Password: ")
    passwordBytes, err := terminal.ReadPassword(syscall.Stdin)
    if err != nil {
        return "", err
    }
    fmt.Println()

    authConfig := types.AuthConfig{
        Username: username,
        Password: string(passwordBytes),
    }
    encodedJSON, err := json.Marshal(authConfig)
    if err != nil {
        return "", err
    }
    authStr := base64.URLEncoding.EncodeToString(encodedJSON)
    return authStr, nil
}

func pushImage(image string) (string, error) {
    authStr, err := getAuth()
    if err != nil {
        return "", err
    }

    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return "", err
    }

    resp, err := cli.ImagePush(ctx, image, types.ImagePushOptions{RegistryAuth:authStr})
    if err != nil {
        return "", err
    }
    defer resp.Close()

    scanner := bufio.NewScanner(resp)
    for scanner.Scan() {
        line := scanner.Text()
        // fmt.Println(line)

        var respObject struct {
            Aux struct {
                Digest string
            }
            Error string
        }

        err = json.Unmarshal([]byte(line), &respObject)
        if err != nil {
            return "", err
        }

        if respObject.Aux.Digest != "" {
            return respObject.Aux.Digest, nil
        } else if respObject.Error != "" {
            return "", errors.New(respObject.Error)
        }
    }

    return "", errors.New("hl-cli: Error did not receive digest")
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