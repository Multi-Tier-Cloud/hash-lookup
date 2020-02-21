package hashlookup

import (
    "context"
    "encoding/json"
    "errors"
    "io/ioutil"
    "net/http"
    "net/url"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/protocol"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/Multi-Tier-Cloud/common/p2pnode"
    "github.com/Multi-Tier-Cloud/common/p2putil"
    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

func GetHash(query string) (contentHash, dockerHash string, err error) {
    ctx := context.Background()
    nodeConfig := p2pnode.NewConfig()
    node, err := p2pnode.NewNode(ctx, nodeConfig)
    if err != nil {
        return "", "", err
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    contentHash, dockerHash, err = GetHashExistingRouting(ctx, node.Host,
        node.RoutingDiscovery, query)
    return contentHash, dockerHash, err
}

func GetHashExistingRouting(
    ctx context.Context, host host.Host,
    routingDiscovery *discovery.RoutingDiscovery, query string) (
    contentHash, dockerHash string, err error) {

    peerChan, err := routingDiscovery.FindPeers(ctx,
        common.HashLookupRendezvousString)
    if err != nil {
        return "", "", err
    }

    for peer := range peerChan {
        if peer.ID == host.ID() {
            continue
        }

        stream, err := host.NewStream(ctx, peer.ID,
            protocol.ID(common.LookupProtocolID))
        if err != nil {
            continue
        }

        err = p2putil.WriteMsg(stream, []byte(query))
        if err != nil {
            panic(err)
        }

        data, err := p2putil.ReadMsg(stream)
        if err != nil {
            panic(err)
        }

        var respInfo common.LookupResponse
        err = json.Unmarshal(data, &respInfo)
        if err != nil {
            return "", "", err
        }

        err = nil
        if !respInfo.LookupOk {
            err = errors.New("hashlookup: Error finding hash for " + query)
        }
        return respInfo.ContentHash, respInfo.DockerHash, err
    }

    return "", "", errors.New("hashlookup: No hash-lookup service found")
}

func GetHashHttp(query string) (contentHash, dockerHash string, err error) {
    hardcodedHttpAddr := "http://10.11.17.13:8080"
    urlStr := hardcodedHttpAddr + common.HttpLookupRoute + url.PathEscape(query)
    resp, err := http.Get(urlStr)
    if err != nil {
        return "", "", err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    
    var respInfo common.LookupResponse
    err = json.Unmarshal(body, &respInfo)
    if err != nil {
        return "", "", errors.New(err.Error() + ": " + string(body))
    }

    err = nil
    if !respInfo.LookupOk {
        err = errors.New("hashlookup: Error finding hash for " + query)
    }
    return respInfo.ContentHash, respInfo.DockerHash, err
}
