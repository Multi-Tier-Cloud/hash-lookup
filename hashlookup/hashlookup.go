package hashlookup

import (
	"context"
	"encoding/json"
	"errors"
	// "fmt"
	// "sync"

	// "github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	// "github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"
	// "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

func GetHashExistingRouting(
	ctx context.Context, host host.Host,
	routingDiscovery *discovery.RoutingDiscovery, query string) (
	contentHash, dockerHash string, err error) {

	peerChan, err := routingDiscovery.FindPeers(ctx,
		common.HashLookupRendezvousString)
	if err != nil {
		return "", "", err
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}

		stream, err := host.NewStream(ctx, peer.ID,
			protocol.ID(common.LookupProtocolID))
		if err != nil {
			continue
		}

		err = common.WriteSingleMessage(stream, []byte(query))
		if err != nil {
			panic(err)
		}

		data, err := common.ReadSingleMessage(stream)
		if err != nil {
			panic(err)
		}

		var respInfo common.LookupResponse
		err = json.Unmarshal(data, &respInfo)
		if err != nil {
			return "", "", err
		}

		err = nil
		if !respInfo.LookupOk {
			err = errors.New("hashlookup: Error finding hash for " + query)
		}
		return respInfo.ContentHash, respInfo.DockerHash, err
	}

	return "", "", errors.New("hashlookup: No hash-lookup service found")
}

func GetHash(query string) (contentHash, dockerHash string, err error) {
	ctx, host, kademliaDHT, routingDiscovery, err := common.Libp2pSetup(false)
	if err != nil {
		return "", "", err
	}
	defer host.Close()
	defer kademliaDHT.Close()

	contentHash, dockerHash, err = GetHashExistingRouting(ctx, host,
		routingDiscovery, query)
	return contentHash, dockerHash, err
}