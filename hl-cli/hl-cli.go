package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// "github.com/libp2p/go-libp2p-core/host"
	// "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

var commands = map[string]func(){
	"add":addCmd,
	"get":getCmd,
	"list":listCmd,
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Missing command")
		return
	}

	cmdFunc, ok := commands[os.Args[1]]
	if !ok {
		fmt.Println("Command not recognized")
		return
	}

	cmdFunc()
}

func addCmd() {
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

	data, err := sendRequest(common.AddProtocolID, reqBytes)
	if err != nil {
		panic(err)
	}
	
	respStr := strings.TrimSpace(string(data))
	fmt.Println("Response:", respStr)
}

func getCmd() {}

func listCmd() {}

func sendRequest(protocolID protocol.ID, request []byte) (
	response []byte, err error) {

	ctx, host, kademliaDHT, routingDiscovery, err := common.Libp2pSetup(true)
	if err != nil {
		return nil, err
	}
	defer host.Close()
	defer kademliaDHT.Close()

	peerChan, err := routingDiscovery.FindPeers(ctx,
		common.HashLookupRendezvousString)
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

		err = common.WriteSingleMessage(stream, request)
		if err != nil {
			return nil, err
		}

		response, err := common.ReadSingleMessage(stream)
		if err != nil {
			return nil, err
		}

		return response, nil
	}

	return nil, errors.New(
		"hl-cli: Failed to connect to any hash-lookup peers")
}