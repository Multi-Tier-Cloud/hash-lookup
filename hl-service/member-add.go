package main

import (
    "context"
    "encoding/json"
    "log"
    "fmt"
    "strings"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

var memberAddProtocolID protocol.ID = "/memberadd/1.0"

type memberAddRequest struct {
    MemberName string
    MemberPeerUrl string
}

func sendMemberAddRequest(
    newMemName, newMemPeerUrl string, local bool, bootstrap string) (
    initialCluster string, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    if local {
        nodeConfig.BootstrapPeers = []string{}
    } else if bootstrap != "" {
        nodeConfig.BootstrapPeers = []string{bootstrap}
    }
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return "", err
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    reqInfo := memberAddRequest{newMemName, newMemPeerUrl}
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
        data, err := p2putil.ReadMsg(stream)
        if err != nil {
            log.Println(err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Member add request:", reqStr)

        var reqInfo memberAddRequest
        err = json.Unmarshal([]byte(reqStr), &reqInfo)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        initialCluster, err := addEtcdMember(
            etcdCli, reqInfo.MemberName, reqInfo.MemberPeerUrl)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        log.Println("Member add response: ", initialCluster)
        err = p2putil.WriteMsg(stream, []byte(initialCluster))
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func addEtcdMember(
    etcdCli *clientv3.Client, newMemName, newMemPeerUrl string) (
    initialCluster string, err error) {

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
            clusterPeerUrls = append(
                clusterPeerUrls, fmt.Sprintf("%s=%s", name, peerUrl))
        }
    }

    initialCluster = strings.Join(clusterPeerUrls, ",")

    return initialCluster, nil
}
