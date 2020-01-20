package main

import (
	"bufio"
	"context"
	// "io/ioutil"
	// "os"
	// "strconv"
	"strings"
	"sync"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-kad-dht"
	// "github.com/ipfs/go-cid"
	// "github.com/multiformats/go-multihash"
	"github.com/multiformats/go-multiaddr"
	// "github.com/whyrusleeping/go-logging"

	"github.com/ipfs/go-log"

	"github.com/Multi-Tier-Cloud/hash-lookup/lookup-common"
)

type StringMap struct {
	sync.RWMutex
	data map[string]string
}

func NewStringMap() *StringMap {
	return &StringMap{
		data: make(map[string]string),
	}
}

func (sm *StringMap) Delete(key string) {
	sm.Lock()
	delete(sm.data, key)
	sm.Unlock()
}

func (sm *StringMap) Load(key string) (value string, ok bool) {
	sm.RLock()
	value, ok = sm.data[key]
	sm.RUnlock()
	return value, ok
}

func (sm *StringMap) Store(key, value string) {
	sm.Lock()
	sm.data[key] = value
	sm.Unlock()
}

var nameToHash = NewStringMap()

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

func handleLookup(stream network.Stream) {
	buf := bufio.NewReader(stream)
	str, err := buf.ReadString('\n')
	logger.Info("Raw:", str)
	if err != nil {
		logger.Error(err)
		stream.Reset()
		return
	}

	str = strings.TrimSpace(str)
	logger.Info("Requested:", str)
	
	writeData, valid := nameToHash.Load(str)
	if !valid {
		writeData = "ERROR"
	}

	_, _ = stream.Write([]byte(writeData))
	stream.Close()
}

func main() {
	// log.SetAllLoggers(logging.WARNING)
	// log.SetAllLoggers(log.LevelWarn)
	log.SetLogLevel("core", "info")

	nameToHash.Store("hello-there", "test-test-test")

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

	host.SetStreamHandler(protocol.ID(common.LookupProtocolID), handleLookup)

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

	logger.Info("Announcing ourselves...")
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	discovery.Advertise(ctx, routingDiscovery, common.LookupRendezvousString)
	logger.Info("Successfully announced!")

	select {}
}