package common

import (
    "context"
    "errors"
    "fmt"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/protocol"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/Multi-Tier-Cloud/common/p2putil"
)

type LookupResponse struct {
    ContentHash string
    DockerHash string
    LookupOk bool
}

type AddRequest struct {
    ServiceName string
    ContentHash string
    DockerHash string
}

var HashLookupRendezvousString string = "hash-lookup";

var LookupProtocolID protocol.ID = "/lookup/1.0";

var AddProtocolID protocol.ID = "/add/1.0";

var HttpLookupRoute string = "/lookup/"

func SendRequestWithHostRouting(ctx context.Context,
    host host.Host, routingDiscovery *discovery.RoutingDiscovery,
    protocolID protocol.ID, request []byte) (
    response []byte, err error) {

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

    return nil, errors.New(
        "hl-common: Failed to connect to any hash-lookup peers")
}