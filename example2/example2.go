package main

import (
	"context"
	"fmt"
	"os"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"

	"github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: example2 <service name>")
		return
	}
	serviceName := os.Args[1]
	fmt.Println("Looking up:", serviceName)

	ctx := context.Background()

	host, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}
	defer host.Close()

	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		panic(err)
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

	// bootstrapMAddrStr := "/ip4/10.11.17.15/tcp/4001/ipfs/QmeZvvPZgrpgSLFyTYwCUEbyK6Ks8Cjm2GGrP2PA78zjAk"
	bootstrapMAddrStr := "/ip4/10.11.17.32/tcp/4001/ipfs/12D3KooWGegi4bWDPw9f6x2mZ6zxtsjR8w4ax1tEMDKCNqdYBt7X"
	bootstrapMAddr, err := multiaddr.NewMultiaddr(bootstrapMAddrStr)
	if err != nil {
        panic(err)
	}

	peerinfo, _ := peer.AddrInfoFromP2pAddr(bootstrapMAddr)
	if err = host.Connect(ctx, *peerinfo); err != nil {
		panic(err)
	}

	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)

	contentHash, _, err := hashlookup.GetHashExistingRouting(ctx, host,
		routingDiscovery, serviceName)
	if err != nil {
		panic(err)
	}

	fmt.Println("Got hash:", contentHash)

	// peerChan, err := routingDiscovery.FindPeers(ctx, contentHash)
}