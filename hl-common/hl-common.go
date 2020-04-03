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

    "github.com/Multi-Tier-Cloud/common/p2putil"
)

const (
    HashLookupRendezvousString string = "hash-lookup";

    LookupProtocolID protocol.ID = "/lookup/1.0";
    ListProtocolID protocol.ID = "/list/1.0";
    AddProtocolID protocol.ID = "/add/1.0";
    DeleteProtocolID protocol.ID = "/delete/1.0";
)

type LookupResponse struct {
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

type AddRequest struct {
    ServiceName string
    ContentHash string
    DockerHash string
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