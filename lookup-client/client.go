package client

import (
	"bufio"
	"context"
	"io/ioutil"
	// "os"
	// "strconv"
	// "strings"
	"sync"

	"github.com/libp2p/go-libp2p"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	"github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/multiformats/go-multiaddr"
	// "github.com/whyrusleeping/go-logging"

	"github.com/ipfs/go-log"

	"github.com/Multi-Tier-Cloud/hash-lookup/lookup-common"
)

var logger = log.Logger("core")

func StringsToAddrs(addrStrings []string) (maddrs []multiaddr.Multiaddr, err error) {
	for _, addrString := range addrStrings {
		addr, err := multiaddr.NewMultiaddr(addrString)
		if err != nil {
			return maddrs, err
		}
		maddrs = append(maddrs, addr)
	}
	return
}

func GetHashExistingRouting(ctx context.Context, host host.Host, kademliaDHT routing.ContentRouting, query string) (result string, ok bool) {
	logger.Info("Searching for other peers...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	peerChan, err := routingDiscovery.FindPeers(ctx, common.LookupRendezvousString)
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}
		logger.Info("Found peer:", peer)

		logger.Info("Connecting to:", peer)
		stream, err := host.NewStream(ctx, peer.ID, protocol.ID(common.LookupProtocolID))

		if err != nil {
			logger.Warning("Connection failed:", err)
			continue
		}

		logger.Info("Connected to:", peer)
		rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

		logger.Info("Writing")
		_, err = rw.WriteString("hello-there\n")
		if err != nil {
			logger.Error("Error writing to buffer")
			panic(err)
		}

		logger.Info("Flushing")
		err = rw.Flush()
		if err != nil {
			logger.Error("Error flushing buffer")
			panic(err)
		}

		logger.Info("Reading")
		out, err := ioutil.ReadAll(stream)
		if err != nil {
			panic(err)
		}
		outStr := string(out)
		logger.Info("Received:", outStr)

		return outStr, true
	}

	return "", false
}

func GetHash(query string) (result string, ok bool) {
	// log.SetAllLoggers(logging.WARNING)
	// log.SetAllLoggers(log.LevelWarn)
	log.SetLogLevel("core", "info")

	ctx := context.Background()

	host, err := libp2p.New(ctx)
	if err != nil {
		panic(err)
	}
	logger.Info("Host created. We are:", host.ID())
	logger.Info(host.Addrs())

	kademliaDHT, err := dht.New(ctx, host)
	if err != nil {
		panic(err)
	}

	logger.Info("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}

    bootstrapPeers, err := StringsToAddrs([]string{
        // custom bootstrap node
        "/ip4/10.11.17.15/tcp/4001/ipfs/QmeZvvPZgrpgSLFyTYwCUEbyK6Ks8Cjm2GGrP2PA78zjAk",
    })
    if err != nil {
        panic(err)
	}
	
	logger.Info("Connecting to BootstrapPeers: ", bootstrapPeers)
	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
	            logger.Warning(err)
			} else {
	            logger.Info("Connection established with bootstrap node:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	return GetHashExistingRouting(ctx, host, kademliaDHT, query)
}