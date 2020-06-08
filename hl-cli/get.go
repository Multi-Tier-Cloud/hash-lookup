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
    "os"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

func getCmd() {
    getFlags := flag.NewFlagSet("get", flag.ExitOnError)

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
    getFlags.Parse(flag.Args()[1:])

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

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

    contentHash, dockerHash, err := hashlookup.GetHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }
    fmt.Println(
        "Response: Content Hash:", contentHash, ", Docker Hash:", dockerHash)
}