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