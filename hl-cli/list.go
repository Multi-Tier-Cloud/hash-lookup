package main

import (
    "flag"
    "fmt"
    "log"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

func listCmd() {
    listFlags := flag.NewFlagSet("list", flag.ExitOnError)

    listFlags.Parse(flag.Args()[1:])

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

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