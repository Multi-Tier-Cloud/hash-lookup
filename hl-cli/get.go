package main

import (
    "flag"
    "fmt"
    "os"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

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