package main

import (
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

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

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

    respStr, err := hashlookup.DeleteHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:", respStr)
}