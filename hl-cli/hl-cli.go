package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/coreunix"

    "github.com/ipfs/go-blockservice"
    "github.com/ipfs/go-ipfs-files"
    dag "github.com/ipfs/go-merkledag"
    dagtest "github.com/ipfs/go-merkledag/test"
    "github.com/ipfs/go-mfs"
    "github.com/ipfs/go-unixfs"

	// "github.com/libp2p/go-libp2p-core/host"
	// "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"

	// "github.com/multiformats/go-multihash"

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
		fmt.Fprintln(os.Stderr, "Missing command")
		usage()
		return
	}

	cmdFunc, ok := commands[os.Args[1]]
	if !ok {
		fmt.Fprintln(os.Stderr, "Command not recognized")
		usage()
		return
	}

	cmdFunc()
}

func usage() {
    exeName, err := getExeName()
	if err != nil {
		panic(err)
	}

	fmt.Fprintln(os.Stderr, "Usage:", exeName, "<command>")
	fmt.Fprintln(os.Stderr, "<command> : add | get | list")
}

func getExeName() (exeName string, err error) {
	// exePath, err := os.Executable()
	// if err != nil {
	// 	return "", err
	// }
	exeName = filepath.Base(os.Args[0])
	return exeName, nil
}

func addCmd() {
    addFlags := flag.NewFlagSet("add", flag.ExitOnError)
    fileFlag := addFlags.String("file", "", "hash file or directory")
	stdinFlag := addFlags.Bool("stdin", false, "hash from stdin")
	
	addUsage := func() {
	    exeName, err := getExeName()
		if err != nil {
			panic(err)
		}
		
	    fmt.Fprintln(os.Stderr, "Usage:", exeName, "add [<options>] <service name>")
	    fmt.Fprintln(os.Stderr, "<options>")
		addFlags.PrintDefaults()
		fmt.Fprintln(os.Stderr, "<service name> : name of service to associate content with")
	}
	
	addFlags.Usage = addUsage
	addFlags.Parse(os.Args[2:])
	
	if len(addFlags.Args()) != 1 {
		addUsage()
		return
	}
	
	var hash string = ""
	var err error = nil
	
	if *fileFlag != "" {
	    hash, err = fileHash(*fileFlag)
	    if err != nil {
		    panic(err)
	    }
	    fmt.Println("Hashed file:", hash)
	} else if *stdinFlag {
	    hash, err = stdinHash()
	    if err != nil {
		    panic(err)
	    }
	    fmt.Println("Hashed stdin:", hash)
	}

    fmt.Println("Adding {", addFlags.Arg(0), ":", hash, "}")
	reqInfo := common.AddRequest{addFlags.Arg(0), hash, ""}
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

func fileHash(path string) (hash string, err error) {
	stat, err := os.Lstat(path)
    if err != nil {
        return "", err
    }

    fileNode, err := files.NewSerialFile(path, false, stat)
    if err != nil {
        return "", err
    }
    defer fileNode.Close()

    return getHash(fileNode)
}


func stdinHash() (hash string, err error) {
	stdinData, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return "", err
	}

	stdinFile := files.NewBytesFile(stdinData)

	return getHash(stdinFile)
}

func getHash(fileNode files.Node) (hash string, err error) {
    ctx := context.Background()
    nilIpfsNode, err := core.NewNode(ctx, &core.BuildCfg{NilRepo: true})
    if err != nil {
        return "", err
    }

    bserv := blockservice.New(nilIpfsNode.Blockstore, nilIpfsNode.Exchange)
    dserv := dag.NewDAGService(bserv)

    fileAdder, err := coreunix.NewAdder(
		ctx, nilIpfsNode.Pinning, nilIpfsNode.Blockstore, dserv)
    if err != nil {
        return "", err
    }

    fileAdder.Pin = false
    fileAdder.CidBuilder = dag.V0CidPrefix()

    mockDserv := dagtest.Mock()
    emptyDirNode := unixfs.EmptyDirNode()
    emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
    mfsRoot, err := mfs.NewRoot(ctx, mockDserv, emptyDirNode, nil)
    if err != nil {
        return "", err
    }
    fileAdder.SetMfsRoot(mfsRoot)

    dagIpldNode, err := fileAdder.AddAllAndPin(fileNode)
    if err != nil {
        return "", err
    }

	hash = dagIpldNode.String()
	return hash, nil
}

func getCmd() {
	if len(os.Args) < 3 {
		exeName, err := getExeName()
		if err != nil {
			panic(err)
		}

		fmt.Fprintln(os.Stderr, "Usage:", exeName, "get <service name>")
		return
	}

	contentHash, _, err := hashlookup.GetHash(os.Args[2])
	if err != nil {
		panic(err)
	}
	fmt.Println("Response:", contentHash)
}

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
