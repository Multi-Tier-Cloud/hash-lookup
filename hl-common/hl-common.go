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
package common

import (
    "context"
    "errors"
    "fmt"
    "log"
    "math"
    "time"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/pnet"
    "github.com/libp2p/go-libp2p-core/protocol"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/multiformats/go-multiaddr"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
)

const (
    HashLookupRendezvousString string = "hash-lookup"

    AddProtocolID protocol.ID = "/add/1.0"
    GetProtocolID protocol.ID = "/get/1.0"
    ListProtocolID protocol.ID = "/list/1.0"
    DeleteProtocolID protocol.ID = "/delete/1.0"
)

type AddRequest struct {
    Name string
    Info ServiceInfo
}

type GetResponse struct {
    Info ServiceInfo
    LookupOk bool
}

type ListResponse struct {
    NameToInfo map[string]ServiceInfo
    LookupOk bool
}

type ServiceInfo struct {
    ContentHash string
    DockerHash string
    AllocationReq p2putil.PerfInd
}

func init() {
    // Set up logging defaults
    log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func SendRequest(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, protocolID protocol.ID, request []byte) (
    response []byte, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    nodeConfig.BootstrapPeers = bootstraps
    nodeConfig.PSK = psk
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return nil, err
    }
    defer node.Close()

    return SendRequestWithHostRouting(ctx, node.Host, node.RoutingDiscovery, protocolID, request)
}

func SendRequestWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery,
    protocolID protocol.ID, request []byte) (response []byte, err error) {

    maxConnAttempts := 5
    for connAttempts := 0; connAttempts < maxConnAttempts; connAttempts++ {
        // Perform simple exponential backoff
        if connAttempts > 0 {
            sleepDuration := int(math.Pow(2, float64(connAttempts)))
            for i := 0; i < sleepDuration; i++ {
                log.Printf("\rUnable to connect to any peers, " +
                    "retrying in %d seconds...     ",
                    sleepDuration - i)
                time.Sleep(time.Second)
            }
            log.Println()
        }

        peerChan, err := routingDiscovery.FindPeers(ctx, HashLookupRendezvousString)
        if err != nil {
            return nil, fmt.Errorf("ERROR: Unable to find peer with service ID %s\n%w",
                                    HashLookupRendezvousString, err)
        }

        for peer := range peerChan {
            if peer.ID == host.ID() {
                continue
            }

            log.Println("Connecting to:", peer)
            stream, err := host.NewStream(ctx, peer.ID, protocolID)
            if err != nil {
                log.Println("Connection failed:", err)
                continue
            }

            err = p2putil.WriteMsg(stream, request)
            if err != nil {
                return nil, err
            }

            response, err := p2putil.ReadMsg(stream)
            if err != nil {
                return nil, err
            }

            return response, nil
        }
    }

    return nil, errors.New("hl-common: Failed to connect to any hash-lookup peers")
}
