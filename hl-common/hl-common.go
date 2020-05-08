package common

import (
    "context"
    "errors"
    "fmt"
    "math"
    "time"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/protocol"
    "github.com/libp2p/go-libp2p-discovery"

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
    ServiceName string
    ContentHash string
    DockerHash string
}

type GetResponse struct {
    ContentHash string
    DockerHash string
    LookupOk bool
}

type ListResponse struct {
    ServiceNames []string
    ContentHashes []string
    DockerHashes []string
    LookupOk bool
}

func SendRequest(protocolID protocol.ID, request []byte) (
    response []byte, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return nil, err
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    return SendRequestWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, protocolID, request)
}

func SendRequestWithHostRouting(ctx context.Context,
    host host.Host, routingDiscovery *discovery.RoutingDiscovery,
    protocolID protocol.ID, request []byte) (
    response []byte, err error) {

    maxConnAttempts := 5
    for connAttempts := 0; connAttempts < maxConnAttempts; connAttempts++ {
        // Perform simple exponential backoff
        if connAttempts > 0 {
            sleepDuration := int(math.Pow(2, float64(connAttempts)))
            for i := 0; i < sleepDuration; i++ {
                fmt.Printf("\rUnable to connect to any peers, " +
                    "retrying in %d seconds...     ",
                    sleepDuration - i)
                time.Sleep(time.Second)
            }
            fmt.Println()
        }

        peerChan, err := routingDiscovery.FindPeers(
            ctx, HashLookupRendezvousString)
        if err != nil {
            return nil, err
        }

        for peer := range peerChan {
            if peer.ID == host.ID() {
                continue
            }

            fmt.Println("Connecting to:", peer)
            stream, err := host.NewStream(ctx, peer.ID, protocolID)
            if err != nil {
                fmt.Println("Connection failed:", err)
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

    return nil, errors.New(
        "hl-common: Failed to connect to any hash-lookup peers")
}