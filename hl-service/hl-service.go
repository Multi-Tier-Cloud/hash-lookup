package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "net/http"
    "net/url"
    "strings"
    "sync"

    "github.com/libp2p/go-libp2p-core/network"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
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

func main() {
    localFlag := flag.Bool("local", false,
        "For debugging: Run locally and do not connect to bootstrap peers")
    flag.Parse()

    nameToHash.Store("hello-there", "test-test-test")

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    if *localFlag {
        nodeConfig.BootstrapPeers = []string{}
    }
    nodeConfig.StreamHandlers = append(nodeConfig.StreamHandlers,
        handleLookup, handleAdd)
    nodeConfig.HandlerProtocolIDs = append(nodeConfig.HandlerProtocolIDs,
        common.LookupProtocolID, common.AddProtocolID)
    nodeConfig.Rendezvous = append(nodeConfig.Rendezvous,
        common.HashLookupRendezvousString)
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    fmt.Println("Host ID:", node.Host.ID())
    fmt.Println("Listening on:", node.Host.Addrs())
    
    http.HandleFunc(common.HttpLookupRoute, handleHttpLookup)
    go func() {
        fmt.Println("Listening for HTTP requests on port 8080")
        fmt.Println(http.ListenAndServe(":8080", nil))
    }()
    
    fmt.Println("Waiting to serve connections...")

    select {}
}

func handleLookup(stream network.Stream) {
    data, err := p2putil.ReadMsg(stream)
    if err != nil {
        fmt.Println(err)
        return
    }
    
    reqStr := strings.TrimSpace(string(data))
    fmt.Println("Lookup request:", reqStr)
    
    contentHash, ok := nameToHash.Load(reqStr)
    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err := json.Marshal(respInfo)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }

    fmt.Println("Lookup response: ", string(respBytes))
    err = p2putil.WriteMsg(stream, respBytes)
    if err != nil {
        fmt.Println(err)
        return
    }
}

func handleAdd(stream network.Stream) {
    data, err := p2putil.ReadMsg(stream)
    if err != nil {
        fmt.Println(err)
        return
    }
    
    reqStr := strings.TrimSpace(string(data))
    fmt.Println("Add request:", reqStr)

    var reqInfo common.AddRequest
    err = json.Unmarshal([]byte(reqStr), &reqInfo)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }

    nameToHash.Store(reqInfo.ServiceName, reqInfo.ContentHash)
    
    respStr := fmt.Sprintf("Added { %s : %s }",
        reqInfo.ServiceName, reqInfo.ContentHash)
    
    fmt.Println("Lookup response: ", respStr)
    err = p2putil.WriteMsg(stream, []byte(respStr))
    if err != nil {
        fmt.Println(err)
        return
    }
}

func handleHttpLookup(w http.ResponseWriter, r *http.Request) {
    path, err := url.PathUnescape(r.URL.Path)
    if err != nil {
        fmt.Println(err)
        return
    }

    pathSegments := strings.Split(path, "/")
    if len(pathSegments) < 3 {
        fmt.Println("No query found in URL")
        return
    }

    reqStr := pathSegments[2]
    fmt.Println("Lookup request:", reqStr)
    
    contentHash, ok := nameToHash.Load(reqStr)
    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err := json.Marshal(respInfo)
    if err != nil {
        fmt.Println(err)
        return
    }

    fmt.Println("Lookup response: ", string(respBytes))
    _, err = w.Write(respBytes)
    if err != nil {
        fmt.Println(err)
        return
    }
}
