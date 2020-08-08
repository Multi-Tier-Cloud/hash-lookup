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
package common

// Code reused throughout this repo

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/pnet"
    "github.com/libp2p/go-libp2p-core/protocol"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/multiformats/go-multiaddr"

    "github.com/PhysarumSM/common/p2pnode"
    "github.com/PhysarumSM/common/p2putil"
    "github.com/PhysarumSM/common/util"
)

const (
    RegistryServiceRendezvousString string = "registry-service"

    AddProtocolID protocol.ID = "/add/0.1"
    GetProtocolID protocol.ID = "/get/0.1"
    ListProtocolID protocol.ID = "/list/0.1"
    DeleteProtocolID protocol.ID = "/delete/0.1"
)

// Info field in the following structs should be a json encoding of
// registry.ServiceInfo. Just store this string instead of decoding it so we
// don't need to keep updating registry-service. Encoding/decoding is already
// being done on client-side (registry package and registry-cli).
// Ie. if we add a new field to ServiceInfo and send it as json, the existing
// registry-service instances will store this string, as opposed to having them
// decode the info field into their outdated version of the ServiceInfo struct,
// since they would not contain the new field.

type AddRequest struct {
    Name string
    InfoStr string
}

type GetResponse struct {
    InfoStr string
    LookupOk bool
}

type ListResponse struct {
    NameToInfoStr map[string]string
    LookupOk bool
}

func init() {
    // Set up logging defaults
    log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

// Send request to registry-service, creating temporary libp2p node
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

// Send request to registry-service, without creating temporary libp2p node
func SendRequestWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery,
    protocolID protocol.ID, request []byte) (response []byte, err error) {

    eba, err := util.NewExpoBackoffAttempts(1 * time.Second, 8 * time.Second, 5)
    if err != nil {
        return nil, err
    }
    for eba.Attempt() {
        peerChan, err := routingDiscovery.FindPeers(ctx, RegistryServiceRendezvousString)
        if err != nil {
            return nil, fmt.Errorf("registry: Unable to find peer with service ID %s\n%w",
                                    RegistryServiceRendezvousString, err)
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

    return nil, errors.New("registry: Failed to connect to any registry-service peers")
}
