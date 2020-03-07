package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "net/http"
    "net/url"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/libp2p/go-libp2p-core/network"
    
    "go.etcd.io/etcd/clientv3"

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

// var nameToHash = NewStringMap()

var cli *clientv3.Client

func main() {
    etcdPortFlag := flag.Int("etcdPort", 2379,
        "Port to connect to etcd instance")
    localFlag := flag.Bool("local", false,
        "For debugging: Run locally and do not connect to bootstrap peers")
    flag.Parse()

    // nameToHash.Store("hello-there", "test-test-test")

    ctx := context.Background()

    var err error
    cli, err = clientv3.New(clientv3.Config{
        Endpoints: []string{"localhost:" + strconv.Itoa(*etcdPort)},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer cli.Close()

    resp, err := cli.Put(ctx, "hello-there", "test-test-test")
    if err != nil {
        panic(err)
    }
    fmt.Println("ETCD Response:", resp)

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

    respBytes, err := doLookup(reqStr)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }
    /*fmt.Println("Lookup request:", reqStr)
    
    // contentHash, ok := nameToHash.Load(reqStr)
    contentHash := ""
    ok := false
    ctx := context.Background()
    resp, err := cli.Get(ctx, reqStr)
    if err != nil {
        panic(err)
    }
    for _, ev := range resp.Kvs {
        contentHash = string(ev.Value)
        ok = true
    }

    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err := json.Marshal(respInfo)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }

    fmt.Println("Lookup response: ", string(respBytes))*/
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

    // nameToHash.Store(reqInfo.ServiceName, reqInfo.ContentHash)
    ctx := context.Background()
    resp, err := cli.Put(ctx, reqInfo.ServiceName, reqInfo.ContentHash)
    if err != nil {
        panic(err)
    }
    fmt.Println(resp)
    
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

    respBytes, err := doLookup(reqStr)
    if err != nil {
        fmt.Println(err)
        return
    }
    /*fmt.Println("Lookup request:", reqStr)
    
    // contentHash, ok := nameToHash.Load(reqStr)
    contentHash := ""
    ok := false
    ctx := context.Background()
    resp, err := cli.Get(ctx, reqStr)
    if err != nil {
        panic(err)
    }
    for _, ev := range resp.Kvs {
        contentHash = string(ev.Value)
        ok = true
    }

    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err := json.Marshal(respInfo)
    if err != nil {
        fmt.Println(err)
        return
    }

    fmt.Println("Lookup response: ", string(respBytes))*/
    _, err = w.Write(respBytes)
    if err != nil {
        fmt.Println(err)
        return
    }
}

func doLookup(reqStr string) (respBytes []byte, err error) {
    fmt.Println("Lookup request:", reqStr)
    
    // contentHash, ok := nameToHash.Load(reqStr)
    contentHash := ""
    ok := false

    ctx := context.Background()
    etcdResp, err := cli.Get(ctx, reqStr)
    if err != nil {
        return respBytes, err
    }
    for _, ev := range etcdResp.Kvs {
        contentHash = string(ev.Value)
        ok = true
        break
    }

    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err := json.Marshal(respInfo)
    if err != nil {
        return respBytes, err
    }

    fmt.Println("Lookup response: ", string(respBytes))
    return respBytes, nil
}