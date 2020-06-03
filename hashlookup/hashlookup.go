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
package hashlookup

import (
    "context"
    "encoding/json"
    "errors"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/Multi-Tier-Cloud/hash-lookup/hl-common"
)

func AddHash(serviceName, hash, dockerId string) (
    addResponse string, err error) {

    reqBytes, err := marshalAddRequest(serviceName, hash, dockerId)
    if err != nil {
        return "", err
    }

    response, err := common.SendRequest(common.AddProtocolID, reqBytes)
    if err != nil {
        return "", err
    }

    return string(response), nil
}

func AddHashWithHostRouting(
    ctx context.Context, host host.Host,
    routingDiscovery *discovery.RoutingDiscovery,
    serviceName, hash, dockerId string) (
    addResponse string, err error) {

    reqBytes, err := marshalAddRequest(serviceName, hash, dockerId)
    if err != nil {
        return "", err
    }

    data, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.AddProtocolID, reqBytes)
    if err != nil {
        return "", err
    }

    return string(data), nil
}

func marshalAddRequest(serviceName, hash, dockerId string) (
    addRequest []byte, err error) {

    reqInfo := common.AddRequest{serviceName, hash, dockerId}
    return json.Marshal(reqInfo)
}

func GetHash(query string) (contentHash, dockerHash string, err error) {
    response, err := common.SendRequest(common.GetProtocolID, []byte(query))
    if err != nil {
        return "", "", err
    }

    return unmarshalGetResponse(response)
}

func GetHashWithHostRouting(
    ctx context.Context, host host.Host,
    routingDiscovery *discovery.RoutingDiscovery, query string) (
    contentHash, dockerHash string, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.GetProtocolID, []byte(query))
    if err != nil {
        return "", "", err
    }

    return unmarshalGetResponse(response)
}

func unmarshalGetResponse(getResponse []byte) (
    contentHash, dockerHash string, err error) {

    var respInfo common.GetResponse
    err = json.Unmarshal(getResponse, &respInfo)
    if err != nil {
        return "", "", err
    }

    if !respInfo.LookupOk {
        return "", "", errors.New("hashlookup: Error finding hash")
    }

    return respInfo.ContentHash, respInfo.DockerHash, nil
}

func ListHashes() (
    serviceNames, contentHashes, dockerHashes []string, err error) {

    response, err := common.SendRequest(common.ListProtocolID, []byte{})
    if err != nil {
        return nil, nil, nil, err
    }

    return unmarshalListResponse(response)
}

func ListHashesWithHostRouting(
    ctx context.Context, host host.Host,
    routingDiscovery *discovery.RoutingDiscovery) (
    serviceNames, contentHashes, dockerHashes []string, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.ListProtocolID, []byte{})
    if err != nil {
        return nil, nil, nil, err
    }

    return unmarshalListResponse(response)
}

func unmarshalListResponse(listResponse []byte) (
    serviceNames, contentHashes, dockerHashes []string, err error) {

    var respInfo common.ListResponse
    err = json.Unmarshal(listResponse, &respInfo)
    if err != nil {
        return nil, nil, nil, err
    }

    if !respInfo.LookupOk {
        return nil, nil, nil, errors.New("hashlookup: Error finding hash")
    }

    return respInfo.ServiceNames, respInfo.ContentHashes,
        respInfo.DockerHashes, nil
}

func DeleteHash(serviceName string) (deleteResponse string, err error) {
    response, err := common.SendRequest(
        common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}

func DeleteHashWithHostRouting(
    ctx context.Context, host host.Host,
    routingDiscovery *discovery.RoutingDiscovery, serviceName string) (
    deleteResponse string, err error) {

    response, err := common.SendRequestWithHostRouting(ctx, host,
        routingDiscovery, common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}
