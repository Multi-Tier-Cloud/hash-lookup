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
    "time"

    "github.com/libp2p/go-libp2p-core/network"
    "github.com/libp2p/go-libp2p-core/protocol"
    
    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

type serviceData struct {
    ContentHash string
    DockerHash string
}

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
        "For debugging: Run locally and do not connect to bootstrap peers\n" +
        "(this option overrides the '--bootstrap' flag)")
    bootstrapFlag := flag.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")
    flag.Parse()

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
    fmt.Println("Lookup request:", reqStr)

    respBytes, err := doLookup(reqStr)
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

    putData := serviceData{reqInfo.ContentHash, reqInfo.DockerHash}
    putDataBytes, err := json.Marshal(putData)
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }

    ctx := context.Background()
    _, err = etcdCli.Put(ctx, reqInfo.ServiceName, string(putDataBytes))
    if err != nil {
        fmt.Println(err)
        stream.Reset()
        return
    }
    
    respStr := fmt.Sprintf("Added %s:%s",
        reqInfo.ServiceName, string(putDataBytes))
    
    fmt.Println("Add response: ", respStr)
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
    fmt.Println("Http lookup request:", reqStr)

    respBytes, err := doLookup(reqStr)
    if err != nil {
        fmt.Println(err)
        return
    }
    fmt.Println("Http lookup response: ", string(respBytes))

    _, err = w.Write(respBytes)
    if err != nil {
        fmt.Println(err)
        return
    }
}

func doLookup(reqStr string) (respBytes []byte, err error) {
    var getData serviceData
    // contentHash := ""
    ok := false

    ctx := context.Background()
    getResp, err := etcdCli.Get(ctx, reqStr)
    if err != nil {
        return nil, err
    }
    for _, kv := range getResp.Kvs {
        err = json.Unmarshal(kv.Value, &getData)
        if err != nil {
            return nil, err
        }
        // contentHash = string(kv.Value)
        ok = true
        break
    }

    respInfo := common.LookupResponse{
        getData.ContentHash, getData.DockerHash, ok}
    respBytes, err = json.Marshal(respInfo)
    if err != nil {
        return nil, err
    }

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
    memAddResp, err := etcdCli.MemberAdd(ctx, []string{newMemPeerUrl})
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
}