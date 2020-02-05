package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
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

var BootstrapPeers []multiaddr.Multiaddr

func init() {
	for _, s := range []string{
		// "/ip4/10.11.17.15/tcp/4001/ipfs/QmeZvvPZgrpgSLFyTYwCUEbyK6Ks8Cjm2GGrP2PA78zjAk",
		"/ip4/10.11.17.32/tcp/4001/ipfs/12D3KooWGegi4bWDPw9f6x2mZ6zxtsjR8w4ax1tEMDKCNqdYBt7X",
	} {
		ma, err := multiaddr.NewMultiaddr(s)
		if err != nil {
			panic(err)
		}
		BootstrapPeers = append(BootstrapPeers, ma)
	}
}

func Libp2pSetup(output bool) (
	ctx context.Context, host host.Host, kademliaDHT *dht.IpfsDHT,
	routingDiscovery *discovery.RoutingDiscovery, err error) {
	
	ctx = context.Background()

	host, err = libp2p.New(ctx)
	if err != nil {
		return ctx, host, kademliaDHT, routingDiscovery, err
	}

	kademliaDHT, err = dht.New(ctx, host)
	if err != nil {
		return ctx, host, kademliaDHT, routingDiscovery, err
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return ctx, host, kademliaDHT, routingDiscovery, err
	}
	
	var wg sync.WaitGroup
	for _, peerAddr := range BootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); output {
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println("Connected to bootstrap:", *peerinfo)
				}
			}
		}()
	}
	wg.Wait()

	routingDiscovery = discovery.NewRoutingDiscovery(kademliaDHT)

	return ctx, host, kademliaDHT, routingDiscovery, nil
}

func ReadSingleMessage(stream network.Stream) (data []byte, err error) {
	data, err = ioutil.ReadAll(stream)
	if err != nil {
		stream.Reset()
		return []byte{}, err
	}

	return data, nil
}

func WriteSingleMessage(stream network.Stream, data []byte) (err error) {
	_, err = stream.Write(data)
	if err != nil {
		stream.Reset()
		return err
	}

	stream.Close()
	return nil
}