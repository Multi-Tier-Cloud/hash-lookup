package main

import (
    "context"
    "fmt"
    "os"
    "path/filepath"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
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
    fmt.Fprintln(os.Stderr, "Usage:", exeName, "<command>")
    fmt.Fprintln(os.Stderr)
    fmt.Fprintln(os.Stderr, "<command>")
    for _, cmd := range commands {
        fmt.Fprintln(os.Stderr, "  " + cmd.Name)
        fmt.Fprintln(os.Stderr, "        " + cmd.Help)
    }
}

func getExeName() (exeName string) {
    return filepath.Base(os.Args[0])
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