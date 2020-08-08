/* Copyright 2020 PhysarumSM Development Team
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
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/PhysarumSM/service-registry/registry"
)

func deleteCmd() {
    deleteFlags := flag.NewFlagSet("delete", flag.ExitOnError)

    deleteUsage := func() {
        exeName := getExeName()
        fmt.Fprintf(os.Stderr, "Usage of %s delete:\n", exeName)
        fmt.Fprintf(os.Stderr, "$ %s delete [OPTIONS ...] <service-name>\n", exeName)
        fmt.Fprintln(os.Stderr,
`
Delete a microservice entry

<service-name>
        Name of microservice to delete

OPTIONS:`)
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

    respStr, err := registry.DeleteServiceWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:")
    fmt.Println(respStr)
}