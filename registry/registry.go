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
package registry

import (
    "context"
    "encoding/json"
    "errors"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/pnet"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/multiformats/go-multiaddr"

    "github.com/Multi-Tier-Cloud/hash-lookup/common"
)

type ServiceInfo = common.ServiceInfo

// Add

func AddHash(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string, info ServiceInfo) (
    addResponse string, err error) {

    reqBytes, err := marshalAddRequest(serviceName, info)
    if err != nil {
        return "", err
    }

    response, err := common.SendRequest(bootstraps, psk, common.AddProtocolID, reqBytes)
    if err != nil {
        return "", err
    }

    return string(response), nil
}

func AddHashWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery,
    serviceName string, info ServiceInfo) (addResponse string, err error) {

    reqBytes, err := marshalAddRequest(serviceName, info)
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

func marshalAddRequest(serviceName string, info ServiceInfo) (addRequest []byte, err error) {
    reqInfo := common.AddRequest{Name: serviceName, Info: info}
    return json.Marshal(reqInfo)
}

// Get

func GetHash(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, query string) (
    info ServiceInfo, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.GetProtocolID, []byte(query))
    if err != nil {
        return info, err
    }

    return unmarshalGetResponse(response)
}

func GetHashWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, query string) (
    info ServiceInfo, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.GetProtocolID, []byte(query))
    if err != nil {
        return info, err
    }

    return unmarshalGetResponse(response)
}

func unmarshalGetResponse(getResponse []byte) (info ServiceInfo, err error) {
    var respInfo common.GetResponse
    err = json.Unmarshal(getResponse, &respInfo)
    if err != nil {
        return info, err
    }

    if !respInfo.LookupOk {
        return info, errors.New("hashlookup: Error finding hash")
    }

    return respInfo.Info, nil
}

// List

func ListHashes(bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    nameToInfo map[string]ServiceInfo, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.ListProtocolID, []byte{})
    if err != nil {
        return nameToInfo, err
    }

    return unmarshalListResponse(response)
}

func ListHashesWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery) (
    nameToInfo map[string]ServiceInfo, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.ListProtocolID, []byte{})
    if err != nil {
        return nameToInfo, err
    }

    return unmarshalListResponse(response)
}

func unmarshalListResponse(listResponse []byte) (nameToInfo map[string]ServiceInfo, err error) {
    var respInfo common.ListResponse
    err = json.Unmarshal(listResponse, &respInfo)
    if err != nil {
        return nameToInfo, err
    }

    if !respInfo.LookupOk {
        return nameToInfo, errors.New("hashlookup: Error finding hash")
    }

    return respInfo.NameToInfo, nil
}

// Delete

func DeleteHash(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string) (
    deleteResponse string, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}

func DeleteHashWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string) (
    deleteResponse string, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}
