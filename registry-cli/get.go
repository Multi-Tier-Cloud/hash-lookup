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

func getCmd() {
    getFlags := flag.NewFlagSet("get", flag.ExitOnError)

    getUsage := func() {
        exeName := getExeName()
        fmt.Fprintf(os.Stderr, "Usage of %s get:\n", exeName)
        fmt.Fprintf(os.Stderr, "$ %s get [OPTIONS ...] <service-name>\n", exeName)
        fmt.Fprintln(os.Stderr,
`
Get information about a microservice

<service-name>
        Name of microservice to get hash of

OPTIONS:`)
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

    info, err := registry.GetServiceWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName)
    if err != nil {
        log.Fatalln(err)
    }

    infoBytes, err := json.Marshal(info)
    if err != nil {
        log.Fatalln(err)
    }
    fmt.Println("Response:")
    fmt.Println(string(infoBytes))
}