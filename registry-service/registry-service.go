/* Copyright 2020 Multi-Tier-Cloud Development Team
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package main

/*
 * Important note regarding restarting etcd cluster:
 * If you kill majority of etcd nodes, ie. you lose quorum, you will need to restore the cluster.
 * Trying to simply restart any of the nodes doesn't work.
 * Etcd retains its old ID and cluster membership info in its data directory (<name>.etcd).
 * Restarting a node with its old data directory causes it to attempt and fail to reach quorum 
 * among the old members, which might not even be alive anymore.
 * Starting a brand new cluster means you lose key-value store data.
 * Trying to connect a new node to a node from the old cluster causes mismatched cluster IDs.
 *
 * In my own testing, this is how I recovered the key-value store while starting a new cluster.
 *
 * 1. Kill all etcd and hl-service nodes.
 *
 * 2. Pick one node. Save its snapshot "db" file somewhere. It can be found under etcd's data 
 * directory, <name>.etcd/member/snap/db. You will then have to delete (or just move to be safe)
 * the data directory.
 * Note in hl-service the convention for name is <ip>-<client_port>-<peer_port> eg. 10.11.17.11-2379-2380
 *
 * 3. Restore from the snapshot db, creating a new data directory, but with the ID and cluster
 * membership info overwritten, allowing you to start a new cluster, but with the old key-value data.
 *
 * $ etcdctl snapshot restore <path/to/snapshot/db> \
 *   --name <name> \
 *   --initial-cluster <name>=http://<ip>:<peer_port> \
 *   --initial-advertise-peer-urls http://<ip>:<peer_port> \
 *   --skip-hash-check=true
 *
 * eg.
 * $ etcdctl snapshot restore old.etcd/member/snap/db \
 *   --name 10.11.17.11-2379-2380 \
 *   --initial-cluster 10.11.17.11-2379-2380=http://10.11.17.11:2380 \
 *   --initial-advertise-peer-urls http://10.11.17.11:2380 \
 *   --skip-hash-check=true
 *
 * 4. Start hl-service as if you were starting a new cluster. You should find you can successfully
 * run this node and still have the original key-value data.
 * eg.
 * $ ./hl-service --etcd-ip 10.11.17.11 --new-etcd-cluster
 *
 * 5. To restart other nodes, delete the other nodes' etcd data directory and run hl-service as if you
 * were connecting to an existing cluster.
 * eg.
 * $ ./hl-service --etcd-ip 10.11.17.13
 *
 * 6. Done.
 *
 * More info: https://etcd.io/docs/v3.3.12/op-guide/recovery/
 */

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/exec"
    "strconv"
    "time"

    "github.com/libp2p/go-libp2p-core/network"
    "github.com/libp2p/go-libp2p-core/pnet"

    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/common/util"
    "github.com/Multi-Tier-Cloud/hash-lookup/common"
    "github.com/Multi-Tier-Cloud/hash-lookup/registry"

    "github.com/prometheus/client_golang/prometheus/promhttp"

    "github.com/multiformats/go-multiaddr"
)

const defaultKeyFile = "~/.privKeyHashLookup"

func init() {
    // Set up logging defaults
    log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
}

func main() {
    var err error
    var keyFlags util.KeyFlags
    var bootstraps *[]multiaddr.Multiaddr
    var psk *pnet.PSK
    if keyFlags, err = util.AddKeyFlags(defaultKeyFile); err != nil {
        log.Fatalln(err)
    }
    if bootstraps, err = util.AddBootstrapFlags(); err != nil {
        log.Fatalln(err)
    }
    if psk, err = util.AddPSKFlag(); err != nil {
        log.Fatalln(err)
    }
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
    promEndpoint := flag.String("prom-listen-addr", ":9102",
        "Listening address/endpoint for Prometheus to scrape")
    flag.Parse()

    // If CLI didn't specify any bootstraps, fallback to environment variable
    if !(*localFlag) && len(*bootstraps) == 0 {
        envBootstraps, err := util.GetEnvBootstraps()
        if err != nil {
            log.Fatalln(err)
        }

        if len(envBootstraps) == 0 {
            log.Fatalln("Error: Must specify the multiaddr of at least one bootstrap node")
        }

        *bootstraps = envBootstraps
    }

    // If CLI didn't specify a PSK, check the environment variables
    if *psk == nil {
        envPsk, err := util.GetEnvPSK()
        if err != nil {
            log.Fatalln(err)
        }

        *psk = envPsk
    }

    priv, err := util.CreateOrLoadKey(keyFlags)
    if err != nil {
        log.Fatalln(err)
    }

    // Start Prometheus endpoint for stats collection
    http.Handle("/metrics", promhttp.Handler())
    go http.ListenAndServe(*promEndpoint, nil)

    ctx := context.Background()

    // etcd setup
    etcdClientEndpoint := *etcdIpFlag + ":" + strconv.Itoa(*etcdClientPortFlag)
    etcdPeerEndpoint := *etcdIpFlag + ":" + strconv.Itoa(*etcdPeerPortFlag)

    etcdClientUrl := "http://" + etcdClientEndpoint
    etcdPeerUrl := "http://" + etcdPeerEndpoint

    etcdName := fmt.Sprintf(
        "%s-%d-%d", *etcdIpFlag, *etcdClientPortFlag, *etcdPeerPortFlag)

    initialCluster := etcdName + "=" + etcdPeerUrl
    clusterState := "new"

    if !(*newEtcdClusterFlag) {
        initialCluster, err = sendMemberAddRequest(
            etcdName, etcdPeerUrl, *localFlag, *bootstraps, *psk)
        if err != nil {
            log.Fatalln(err)
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
            log.Fatalln(err)
        }
    }()

    etcdCli, err := clientv3.New(clientv3.Config{
        Endpoints: []string{etcdClientEndpoint},
        DialTimeout: 5 * time.Second,
    })
    if err != nil {
        log.Fatalln(err)
    }
    defer etcdCli.Close()

    // TODO: Remove this test entry at some point...
    //       Currently useful to serve as a negative test case when pulling images
    // BEGIN test entry
    testEntry := registry.ServiceInfo{
        ContentHash: "UofT",
        DockerHash: "ECE",
        NetworkSoftReq: p2putil.PerfInd{RTT: 2019},
        NetworkHardReq: p2putil.PerfInd{RTT: 2020},
        CpuReq: 50,
        MemoryReq: 496,
    }
    testEntryBytes, err := json.Marshal(testEntry)
    if err != nil {
        log.Fatalln(err)
    }
    err = etcdPut(etcdCli, "test-entry", string(testEntryBytes))
    if err != nil {
        log.Fatalln(err)
    }
    log.Printf("Test entry: {test-entry: %v}\n", testEntry)
    // END test entry

    nodeConfig := p2pnode.NewConfig()
    nodeConfig.PrivKey = priv
    nodeConfig.PSK = *psk
    if *localFlag {
        nodeConfig.BootstrapPeers = []multiaddr.Multiaddr{}
    } else if len(*bootstraps) > 0 {
        nodeConfig.BootstrapPeers = *bootstraps
    }
    nodeConfig.StreamHandlers = append(nodeConfig.StreamHandlers,
        handleAdd(etcdCli), handleGet(etcdCli), handleList(etcdCli),
        handleDelete(etcdCli), handleMemberAdd(etcdCli))
    nodeConfig.HandlerProtocolIDs = append(nodeConfig.HandlerProtocolIDs,
        common.AddProtocolID, common.GetProtocolID, common.ListProtocolID,
        common.DeleteProtocolID, memberAddProtocolID)
    nodeConfig.Rendezvous = append(nodeConfig.Rendezvous, common.RegistryServiceRendezvousString)
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        if *localFlag && err.Error() == "Failed to connect to any bootstraps" {
            log.Println("Local run, not connecting to bootstraps")
        } else {
            log.Fatalln(err)
        }
    }
    defer node.Close()

    // log.Println("Host ID:", node.Host.ID())
    // log.Println("Listening on:", node.Host.Addrs())

    log.Println("Waiting to serve connections...")

    select {}
}

func streamError(stream network.Stream, err error) {
    log.Println(err)
    stream.Reset()
}

func etcdPut(etcdCli *clientv3.Client, serviceName string, putData string) (err error) {
    ctx := context.Background()
    _, err = etcdCli.Put(ctx, serviceName, putData)
    if err != nil {
        return err
    }

    return nil
}

func getServiceInfo(etcdCli *clientv3.Client, query string) (
    infoStr string, queryOk bool, err error) {

    nameToInfoStr, queryOk, err := etcdGet(etcdCli, query, false)
    if err != nil {
        return infoStr, queryOk, err
    }

    infoStr, found := nameToInfoStr[query]
    if !found {
        return infoStr, false, err
    }

    return infoStr, queryOk, err
}

func listServiceInfo(etcdCli *clientv3.Client) (
    nameToInfoStr map[string]string, queryOk bool, err error) {

    return etcdGet(etcdCli, "", true)
}

func etcdGet(etcdCli *clientv3.Client, query string, withPrefix bool) (
    nameToInfoStr map[string]string, queryOk bool, err error) {

    ctx := context.Background()
    var getResp *clientv3.GetResponse
    if withPrefix {
        getResp, err = etcdCli.Get(ctx, query, clientv3.WithPrefix())
    } else {
        getResp, err = etcdCli.Get(ctx, query)
    }
    if err != nil {
        return nameToInfoStr, false, err
    }

    nameToInfoStr = make(map[string]string)
    queryOk = len(getResp.Kvs) > 0
    for _, kv := range getResp.Kvs {
        nameToInfoStr[string(kv.Key)] = string(kv.Value)
    }

    return nameToInfoStr, queryOk, nil
}
