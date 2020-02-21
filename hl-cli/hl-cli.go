package main

import (
    "context"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "strings"

    "github.com/ipfs/go-blockservice"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/coreunix"
    "github.com/ipfs/go-ipfs-files"
    dag "github.com/ipfs/go-merkledag"
    dagtest "github.com/ipfs/go-merkledag/test"
    "github.com/ipfs/go-mfs"
    "github.com/ipfs/go-unixfs"

    "github.com/libp2p/go-libp2p-core/protocol"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
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
        "Get the content hash of a microservice from the hash-lookup service",
        getCmd,
    },
    commandData{
        "list",
        "List all microservices and hashes stored by the hash-lookup service",
        listCmd,
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
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "add [<options>] <name>")
        fmt.Fprintln(os.Stderr,
`
<name>
        Name of microservice to associate hashed content with

<options>`)
        addFlags.PrintDefaults()
    }
    
    addFlags.Usage = addUsage
    addFlags.Parse(os.Args[2:])
    
    if len(addFlags.Args()) != 1 {
        fmt.Fprintln(os.Stderr, "Error: wrong number of required arguments")
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

    fmt.Println("Adding {", addFlags.Arg(0), ":", hash, "}")
    reqInfo := common.AddRequest{addFlags.Arg(0), hash, ""}
    reqBytes, err := json.Marshal(reqInfo)
    if err != nil {
        panic(err)
    }

    data, err := sendRequest(common.AddProtocolID, reqBytes, *bootstrapFlag)
    if err != nil {
        panic(err)
    }

    respStr := strings.TrimSpace(string(data))
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
    if len(os.Args) < 3 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")

        exeName := getExeName()
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "get <name>")
        return
    }

    contentHash, _, err := hashlookup.GetHash(os.Args[2])
    if err != nil {
        panic(err)
    }
    fmt.Println("Response:", contentHash)
}

func listCmd() {}

func sendRequest(protocolID protocol.ID, request []byte, bootstrap string) (
    response []byte, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    if bootstrap != "" {
        nodeConfig.BootstrapPeers = []string{bootstrap}
    }
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return nil, err
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    peerChan, err := node.RoutingDiscovery.FindPeers(ctx,
        common.HashLookupRendezvousString)
    if err != nil {
        return nil, err
    }

    for peer := range peerChan {
        if peer.ID == node.Host.ID() {
            continue
        }

        fmt.Println("Connecting to:", peer)
        stream, err := node.Host.NewStream(ctx, peer.ID, protocolID)
        if err != nil {
            fmt.Println("Connection failed:", err)
            continue
        }

        err = p2putil.WriteMsg(stream, request)
        if err != nil {
            return nil, err
        }

        response, err := p2putil.ReadMsg(stream)
        if err != nil {
            return nil, err
        }

        return response, nil
    }

    return nil, errors.New(
        "hl-cli: Failed to connect to any hash-lookup peers")
}
