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
    "encoding/json"
    "io/ioutil"
    "log"
    "fmt"
    "strings"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/pnet"
	"github.com/libp2p/go-libp2p-core/protocol"

    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"

    "github.com/multiformats/go-multiaddr"
)

var memberAddProtocolID protocol.ID = "/memberadd/0.1"

type memberAddRequest struct {
    MemberName string
    MemberPeerUrl string
}

func sendMemberAddRequest(
    newMemName, newMemPeerUrl string, local bool, bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    initialCluster string, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    nodeConfig.PSK = psk
    if local {
        nodeConfig.BootstrapPeers = []multiaddr.Multiaddr{}
    } else if len(bootstraps) > 0 {
        nodeConfig.BootstrapPeers = bootstraps
    }
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return "", err
    }
    defer node.Close()

    reqInfo := memberAddRequest{MemberName: newMemName, MemberPeerUrl: newMemPeerUrl}
    reqBytes, err := json.Marshal(reqInfo)
    if err != nil {
        return "", err
    }

    response, err := common.SendRequestWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, memberAddProtocolID, reqBytes)
    if err != nil {
        return "", err
    }

    initialCluster = strings.TrimSpace(string(response))

    return initialCluster, nil
}

func handleMemberAdd(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        data, err := ioutil.ReadAll(stream)
        if err != nil {
            streamError(stream, err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Member add request:", reqStr)

        var reqInfo memberAddRequest
        err = json.Unmarshal([]byte(reqStr), &reqInfo)
        if err != nil {
            streamError(stream, err)
            return
        }

        initialCluster, err := addEtcdMember(etcdCli, reqInfo.MemberName, reqInfo.MemberPeerUrl)
        if err != nil {
            streamError(stream, err)
            return
        }

        log.Println("Member add response: ", initialCluster)
        _, err = stream.Write([]byte(initialCluster))
        if err != nil {
            streamError(stream, err)
            return
        }

        stream.Close()
    }
}

func addEtcdMember(
    etcdCli *clientv3.Client, newMemName, newMemPeerUrl string) (initialCluster string, err error) {

    ctx := context.Background()
    memAddResp, err := etcdCli.MemberAdd(ctx, []string{newMemPeerUrl})
    if err != nil {
        return "", nil
    }

    newMemId := memAddResp.Member.ID

    clusterPeerUrls := []string{}
    for _, mem := range memAddResp.Members {
        name := mem.Name
        if mem.ID == newMemId {
            name = newMemName
        }
        for _, peerUrl := range mem.PeerURLs {
            clusterPeerUrls = append(clusterPeerUrls, fmt.Sprintf("%s=%s", name, peerUrl))
        }
    }

    initialCluster = strings.Join(clusterPeerUrls, ",")

    return initialCluster, nil
}
