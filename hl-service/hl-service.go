package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "time"
    "net/http"

    "github.com/libp2p/go-libp2p-core/network"

    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"

    "github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
    // Set up logging defaults
    log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

type etcdData struct {
    ContentHash string
    DockerHash string
}

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
    promEndpoint := flag.String("prom-listen-addr", ":9102",
        "Listening address/endpoint for Prometheus to scrape")
    flag.Parse()

    // Start Prometheus endpoint for stats collection
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(*promEndpoint, nil)

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
    log.Println(etcdArgs)

    cmd := exec.Command("etcd", etcdArgs...)
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    go func() {
        err := cmd.Run()
        if err != nil {
            panic(err)
        }
    }()

    etcdCli, err := clientv3.New(clientv3.Config{
        Endpoints: []string{etcdClientEndpoint},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer etcdCli.Close()

    testData := etcdData{"general", "kenobi"}
    testDataBytes, err := json.Marshal(testData)
    if err != nil {
        panic(err)
        return
    }
    putResp, err := etcdCli.Put(ctx, "hello-there", string(testDataBytes))
    if err != nil {
        panic(err)
    }
    log.Println("etcd Response:", putResp)

    nodeConfig := p2pnode.NewConfig()
    if *localFlag {
        nodeConfig.BootstrapPeers = []string{}
    } else if *bootstrapFlag != "" {
        nodeConfig.BootstrapPeers = []string{*bootstrapFlag}
    }
    nodeConfig.StreamHandlers = append(nodeConfig.StreamHandlers,
        handleAdd(etcdCli), handleGet(etcdCli), handleList(etcdCli),
        handleDelete(etcdCli), handleMemberAdd(etcdCli))
    nodeConfig.HandlerProtocolIDs = append(nodeConfig.HandlerProtocolIDs,
        common.AddProtocolID, common.GetProtocolID, common.ListProtocolID,
        common.DeleteProtocolID, memberAddProtocolID)
    nodeConfig.Rendezvous = append(nodeConfig.Rendezvous,
        common.HashLookupRendezvousString)
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        if *localFlag && err.Error() == "Failed to connect to any bootstraps" {
            log.Println("Local run, not connecting to bootstraps")
        } else {
            panic(err)
        }
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    // log.Println("Host ID:", node.Host.ID())
    // log.Println("Listening on:", node.Host.Addrs())

    log.Println("Waiting to serve connections...")

    select {}
}

func handleAdd(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        data, err := p2putil.ReadMsg(stream)
        if err != nil {
            log.Println(err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Add request:", reqStr)

        var reqInfo common.AddRequest
        err = json.Unmarshal([]byte(reqStr), &reqInfo)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        putData := etcdData{reqInfo.ContentHash, reqInfo.DockerHash}
        putDataBytes, err := json.Marshal(putData)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        ctx := context.Background()
        _, err = etcdCli.Put(ctx, reqInfo.ServiceName, string(putDataBytes))
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        respStr := fmt.Sprintf("Added %s:%s",
            reqInfo.ServiceName, string(putDataBytes))

        log.Println("Add response: ", respStr)
        err = p2putil.WriteMsg(stream, []byte(respStr))
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func handleGet(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        data, err := p2putil.ReadMsg(stream)
        if err != nil {
            log.Println(err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Lookup request:", reqStr)

        contentHash, dockerHash, ok, err := getServiceHash(etcdCli, reqStr)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        respInfo := common.GetResponse{contentHash, dockerHash, ok}
        respBytes, err := json.Marshal(respInfo)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        log.Println("Lookup response: ", string(respBytes))

        err = p2putil.WriteMsg(stream, respBytes)
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func handleList(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        log.Println("List request")

        serviceNames, contentHashes, dockerHashes, ok, err :=
            listServiceHashes(etcdCli)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        respInfo := common.ListResponse{
            serviceNames, contentHashes, dockerHashes, ok}
        respBytes, err := json.Marshal(respInfo)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        log.Println("List response: ", string(respBytes))

        err = p2putil.WriteMsg(stream, respBytes)
        if err != nil {
            log.Println(err)
            return
        }
    }
}

func getServiceHash(etcdCli *clientv3.Client, query string) (
    contentHash, dockerHash string, ok bool, err error) {

    _, contentHashes, dockerHashes, ok, err := etcdGet(etcdCli, query, false)
    if len(contentHashes) > 0 {
        contentHash = contentHashes[0]
    }
    if len(dockerHashes) > 0 {
        dockerHash = dockerHashes[0]
    }
    return contentHash, dockerHash, ok, err
}

func listServiceHashes(etcdCli *clientv3.Client) (
    serviceNames, contentHashes, dockerHashes []string, ok bool, err error) {

    return etcdGet(etcdCli, "", true)
}

func etcdGet(etcdCli *clientv3.Client, query string, withPrefix bool) (
    serviceNames, contentHashes, dockerHashes []string, queryOk bool,
    err error) {

    ctx := context.Background()
    var getResp *clientv3.GetResponse
    if withPrefix {
        getResp, err = etcdCli.Get(ctx, query, clientv3.WithPrefix())
    } else {
        getResp, err = etcdCli.Get(ctx, query)
    }
    if err != nil {
        return serviceNames, contentHashes, dockerHashes, queryOk, err
    }

    queryOk = len(getResp.Kvs) > 0
    for _, kv := range getResp.Kvs {
        var getData etcdData
        err = json.Unmarshal(kv.Value, &getData)
        if err != nil {
            return serviceNames, contentHashes, dockerHashes, queryOk, err
        }
        serviceNames = append(serviceNames, string(kv.Key))
        contentHashes = append(contentHashes, getData.ContentHash)
        dockerHashes = append(dockerHashes, getData.DockerHash)
    }

    return serviceNames, contentHashes, dockerHashes, queryOk, nil
}

func handleDelete(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        data, err := p2putil.ReadMsg(stream)
        if err != nil {
            log.Println(err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Delete request:", reqStr)

        ctx := context.Background()
        deleteResp, err := etcdCli.Delete(ctx, reqStr)
        if err != nil {
            log.Println(err)
            stream.Reset()
            return
        }

        var respStr string
        if deleteResp.Deleted != 0 {
            respStr = fmt.Sprintf(
                "Deleted %d entry from hash lookup", deleteResp.Deleted)
        } else {
            respStr = "Error: Failed to delete any entries from hash lookup"
        }

        log.Println("Delete response: ", respStr)
        err = p2putil.WriteMsg(stream, []byte(respStr))
        if err != nil {
            log.Println(err)
            return
        }
    }
}
