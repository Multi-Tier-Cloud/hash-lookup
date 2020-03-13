package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "net/http"
    "net/url"
    "os"
    "os/exec"
    "strconv"
    "strings"
    // "sync"
    "time"

    "github.com/libp2p/go-libp2p-core/network"
    "github.com/libp2p/go-libp2p-core/protocol"
    
    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)
/*
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
*/
var etcdCli *clientv3.Client

func main() {
    newEtcdClusterFlag := flag.Bool("new-etcd-cluster", false,
        "Start running new etcd cluster")
    etcdIpFlag := flag.String("etcd-ip", "127.0.0.1",
        "Local etcd instance IP address")
    etcdClientPortFlag := flag.Int("etcd-client-port", 2379,
        "Local etcd instance client port")
    etcdPeerPortFlag := flag.Int("etcd-peer-port", 2380,
        "Local etcd instance peer port")
    localFlag := flag.Bool("local", false,
        "For debugging: Run locally and do not connect to bootstrap peers")
    bootstrapFlag := flag.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")
    flag.Parse()

    // nameToHash.Store("hello-there", "test-test-test")

    ctx := context.Background()

    etcdClientEndpoint := *etcdIpFlag + ":" + strconv.Itoa(*etcdClientPortFlag)
    etcdPeerEndpoint := *etcdIpFlag + ":" + strconv.Itoa(*etcdPeerPortFlag)

    etcdClientUrl := "http://" + etcdClientEndpoint
    etcdPeerUrl := "http://" + etcdPeerEndpoint

    etcdName := fmt.Sprintf(
        "%s-%d-%d", *etcdIpFlag, *etcdClientPortFlag, *etcdPeerPortFlag)

    initialCluster := etcdName + "=" + etcdPeerUrl
    clusterState := "new"

    var err error

    if !(*newEtcdClusterFlag) {
        initialCluster, err = sendMemberAddRequest(
            etcdName, etcdPeerUrl, *localFlag, *bootstrapFlag)
        if err != nil {
            panic(err)
        }
        clusterState = "existing"
    }

    etcdArgs := []string{
        "--name", etcdName,
        "--listen-client-urls", etcdClientUrl,
        "--advertise-client-urls", etcdClientUrl,
        "--listen-peer-urls", etcdPeerUrl,
        "--initial-advertise-peer-urls", etcdPeerUrl,
        "--initial-cluster", initialCluster,
        "--initial-cluster-state", clusterState,
    }
    fmt.Println(etcdArgs)
    
    cmd := exec.Command("etcd", etcdArgs...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    go func() {
        err := cmd.Run()
        if err != nil {
            panic(err)
        }
    }()

    etcdCli, err = clientv3.New(clientv3.Config{
        Endpoints: []string{etcdClientEndpoint},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer etcdCli.Close()

    putResp, err := etcdCli.Put(ctx, "hello-there", "general-kenobi")
    if err != nil {
        panic(err)
    }
    fmt.Println("etcd Response:", putResp)

    nodeConfig := p2pnode.NewConfig()
    if *localFlag {
        nodeConfig.BootstrapPeers = []string{}
    } else if *bootstrapFlag != "" {
        nodeConfig.BootstrapPeers = []string{*bootstrapFlag}
    }
    nodeConfig.StreamHandlers = append(nodeConfig.StreamHandlers,
        handleLookup, handleAdd, handleMemberAdd)
    nodeConfig.HandlerProtocolIDs = append(nodeConfig.HandlerProtocolIDs,
        common.LookupProtocolID, common.AddProtocolID, memberAddProtocolID)
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
    putResp, err := etcdCli.Put(ctx, reqInfo.ServiceName, reqInfo.ContentHash)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }
    fmt.Println(putResp)
    
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
    getResp, err := etcdCli.Get(ctx, reqStr)
    if err != nil {
        return respBytes, err
    }
    for _, kv := range getResp.Kvs {
        contentHash = string(kv.Value)
        ok = true
        break
    }

    respInfo := common.LookupResponse{contentHash, "", ok}
    respBytes, err = json.Marshal(respInfo)
    if err != nil {
        return respBytes, err
    }

    fmt.Println("Lookup response: ", string(respBytes))
    return respBytes, nil
}

type memberAddRequest struct {
    MemberName string
    MemberPeerUrl string
}

var memberAddProtocolID protocol.ID = "/memberadd/1.0"

func sendMemberAddRequest(
    newMemName, newMemPeerUrl string, local bool, bootstrap string) (
    initialCluster string, err error) {

    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    if local {
        nodeConfig.BootstrapPeers = []string{}
    } else if bootstrap != "" {
        nodeConfig.BootstrapPeers = []string{bootstrap}
    }
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return "", err
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    reqInfo := memberAddRequest{newMemName, newMemPeerUrl}
    reqBytes, err := json.Marshal(reqInfo)
    if err != nil {
        return "", err
    }

    response, err := common.SendRequestWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, memberAddProtocolID, reqBytes)
    if err != nil {
        return "", err
    }

    initialCluster = strings.TrimSpace(string(response))
    
    return initialCluster, nil
}

func handleMemberAdd(stream network.Stream) {
    data, err := p2putil.ReadMsg(stream)
    if err != nil {
        fmt.Println(err)
        return
    }
    
    reqStr := strings.TrimSpace(string(data))
    fmt.Println("Member add request:", reqStr)

    var reqInfo memberAddRequest
    err = json.Unmarshal([]byte(reqStr), &reqInfo)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }

    initialCluster, err := addEtcdMember(
        reqInfo.MemberName, reqInfo.MemberPeerUrl)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }
    
    fmt.Println("Member add response: ", initialCluster)
    err = p2putil.WriteMsg(stream, []byte(initialCluster))
    if err != nil {
        fmt.Println(err)
        return
    }
}

func addEtcdMember(newMemName, newMemPeerUrl string) (
    initialCluster string, err error) {

    ctx := context.Background()
    memAddResp, err := etcdCli.MemberAdd(ctx, []string{newMemPeerUrl/*"http://localhost:3333"*/})
    if err != nil {
        return "", nil
    }

    newMemId := memAddResp.Member.ID

    clusterPeerUrls := []string{}
    for _, mem := range memAddResp.Members {
        name := mem.Name
        if mem.ID == newMemId {
            name = newMemName
        }
        for _, peerUrl := range mem.PeerURLs {
            clusterPeerUrls = append(
                clusterPeerUrls, fmt.Sprintf("%s=%s", name, peerUrl))
        }
    }

    initialCluster = strings.Join(clusterPeerUrls, ",")

    return initialCluster, nil
    // fmt.Println("added member.Name:", mresp.Member.Name)
    // fmt.Println("added member.PeerURLs:", mresp.Member.PeerURLs)
    // for _, mem := range(mresp.Members) {
    //     fmt.Println("name:", mem.Name, "peer:", mem.PeerURLs)
    // }

    /*
        conf := []string{}
		for _, memb := range resp.Members {
			for _, u := range memb.PeerURLs {
				n := memb.Name
				if memb.ID == newID {
					n = newMemberName
				}
				conf = append(conf, fmt.Sprintf("%s=%s", n, u))
			}
		}

		fmt.Print("\n")
		fmt.Printf("ETCD_NAME=%q\n", newMemberName)
		fmt.Printf("ETCD_INITIAL_CLUSTER=%q\n", strings.Join(conf, ","))
		fmt.Printf("ETCD_INITIAL_ADVERTISE_PEER_URLS=%q\n", memberPeerURLs)
    */
}