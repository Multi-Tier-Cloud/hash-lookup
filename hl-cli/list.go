package main

import (
    "flag"
    "fmt"
    "os"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

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