package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

func main() {
	if len(os.Args) < 3 {
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		exeName := filepath.Base(exePath)

		fmt.Println("Usage:", exeName, "<service name> <hash>")
		return
	}
	reqInfo := common.AddRequest{os.Args[1], os.Args[2], ""}
	reqBytes, err := json.Marshal(reqInfo)
	if err != nil {
		panic(err)
	}

	ctx, host, kademliaDHT, routingDiscovery, err := common.Libp2pSetup(true)
	if err != nil {
		panic(err)
	}
	defer host.Close()
	defer kademliaDHT.Close()

	peerChan, err := routingDiscovery.FindPeers(ctx,
		common.HashLookupRendezvousString)
	if err != nil {
		panic(err)
	}

	for peer := range peerChan {
		if peer.ID == host.ID() {
			continue
		}

		fmt.Println("Connecting to:", peer)
		stream, err := host.NewStream(ctx, peer.ID,
			protocol.ID(common.AddProtocolID))
		if err != nil {
			fmt.Println("Connection failed:", err)
			continue
		}

		err = common.WriteSingleMessage(stream, reqBytes)
		if err != nil {
			panic(err)
		}

		data, err := common.ReadSingleMessage(stream)
		if err != nil {
			panic(err)
		}
		
		respStr := strings.TrimSpace(string(data))
		fmt.Println("Success:", respStr)

		break
	}
}