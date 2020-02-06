package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-cid"

	// "github.com/libp2p/go-libp2p-core/host"
	// "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/multiformats/go-multihash"

	"github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
	"github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

var commands = map[string]func(){
	"add": addCmd,
	"get": getCmd,
	"list": listCmd,
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
	if len(os.Args) < 4 {
		exeName, err := getExeName()
		if err != nil {
			panic(err)
		}
		
		fmt.Println("Usage:", exeName, "add <service name> <file>")
		return
	}

	hash, err := getFileHash(os.Args[3])
	if err != nil {
		panic(err)
	}
	fmt.Println("Hashed file:", hash)

	reqInfo := common.AddRequest{os.Args[2], hash, ""}
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

func getCmd() {
	if len(os.Args) < 3 {
		exeName, err := getExeName()
		if err != nil {
			panic(err)
		}

		fmt.Println("Usage:", exeName, "get <service name>")
		return
	}

	contentHash, _, err := hashlookup.GetHash(os.Args[2])
	if err != nil {
		panic(err)
	}
	fmt.Println("Response:", contentHash)
}

func listCmd() {}

func getExeName() (exeName string, err error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeName = filepath.Base(exePath)
	return exeName, nil
}

func getFileHash(fileName string) (hashB58Str string, err error) {
	fileData, err := ioutil.ReadFile(fileName)
	if err != nil {
		return "", err
	}

	return getHash(fileData)
}

func getHash(data []byte) (hashB58Str string, err error) {
	// Create a cid manually by specifying the 'prefix' parameters
	pref := cid.Prefix{
		Version: 1,
		Codec: cid.Raw,
		MhType: multihash.SHA2_256,
		MhLength: -1, // default length
	}

	cid, err := pref.Sum(data)
	if err != nil {
		return "", err
	}

	hashB58Str = cid.Hash().B58String()
	return hashB58Str, nil
}

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