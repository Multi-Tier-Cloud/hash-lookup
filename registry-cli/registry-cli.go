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
    "context"
    "flag"
    "fmt"
    "log"
    "os"
    "path/filepath"

    "github.com/libp2p/go-libp2p-core/pnet"

    "github.com/multiformats/go-multiaddr"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/util"
)

type commandData struct {
    Name string
    Help string
    Run func()
}

var (
    commands = []commandData{
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

    bootstraps *[]multiaddr.Multiaddr
    psk *pnet.PSK
)

func main() {
    var err error
    if bootstraps, err = util.AddBootstrapFlags(); err != nil {
        log.Fatalln(err)
    }
    if psk, err = util.AddPSKFlag(); err != nil {
        log.Fatalln(err)
    }
    flag.Usage = usage
    flag.Parse()

    // If CLI didn't specify any bootstraps, fallback to environment variable
    if len(*bootstraps) == 0 {
        envBootstraps, err := util.GetEnvBootstraps()
        if err != nil {
            log.Fatalln(err)
        }

        if len(envBootstraps) == 0 {
            fmt.Fprintln(os.Stderr, "Error: Must specify the multiaddr of at least one bootstrap node")
            fmt.Fprintln(os.Stderr)
            usage()
            os.Exit(1)
        }

        *bootstraps = envBootstraps
    }

    // If CLI didn't specify a PSK, check the environment variables
    if *psk == nil {
        envPsk, err := util.GetEnvPSK()
        if err != nil {
            log.Fatalln(err)
        }

        *psk = envPsk
    }

    if flag.NArg() < 1 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")
        fmt.Fprintln(os.Stderr)
        usage()
        os.Exit(1)
    }

    positionalArgs := flag.Args()
    cmdArg := positionalArgs[0]

    var cmd commandData
    ok := false
    for _, cmd = range commands {
        if cmdArg == cmd.Name {
            ok = true
            break
        }
    }

    if !ok {
        fmt.Fprintf(os.Stderr, "Error: Command '%s' not recognized\n\n", cmdArg)
        usage()
        os.Exit(1)
    }

    cmd.Run()
}

func usage() {
    exeName := getExeName()
    fmt.Fprintf(os.Stderr, "Usage of %s:\n", exeName)
    fmt.Fprintf(os.Stderr,
        "$ %s [OPTIONS ...] <command>\n", exeName)

    // NOTE: Bootstrap is technically *mandatory* right now, not optional,
    //       at least until we can get a fallback working (TODO).
    fmt.Fprintf(os.Stderr, "\nOPTIONS:\n")
    flag.PrintDefaults()

    fmt.Fprintf(os.Stderr, "\nAvailable commands are:\n")
    for _, cmd := range commands {
        fmt.Fprintln(os.Stderr, "  " + cmd.Name)
        fmt.Fprintln(os.Stderr, "        " + cmd.Help)
    }
}

func getExeName() (exeName string) {
    return filepath.Base(os.Args[0])
}

func setupNode(bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    ctx context.Context, node p2pnode.Node, err error) {

    ctx = context.Background()
    nodeConfig := p2pnode.NewConfig()
    nodeConfig.PSK = psk
    if len(bootstraps) > 0 {
        nodeConfig.BootstrapPeers = bootstraps
    }
    node, err = p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return ctx, node, err
    }
    return ctx, node, nil
}
