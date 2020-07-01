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
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/Multi-Tier-Cloud/service-registry/registry"
)

func listCmd() {
    listFlags := flag.NewFlagSet("list", flag.ExitOnError)

    listUsage := func() {
        exeName := getExeName()
        fmt.Fprintf(os.Stderr, "Usage of %s list:\n", exeName)
        fmt.Fprintf(os.Stderr, "$ %s list [OPTIONS ...]\n", exeName)
        fmt.Fprintln(os.Stderr,
`
List all microservices and information stored by the registry-service

OPTIONS:`)
        listFlags.PrintDefaults()
    }
    
    listFlags.Usage = listUsage
    listFlags.Parse(flag.Args()[1:])

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

    nameToInfo, err := registry.ListServicesWithHostRouting(ctx, node.Host, node.RoutingDiscovery)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:")
    for serviceName, info := range nameToInfo {
        infoBytes, err := json.Marshal(info)
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Printf("Service Name: %s, Info: %s\n", serviceName, string(infoBytes))
    }
}