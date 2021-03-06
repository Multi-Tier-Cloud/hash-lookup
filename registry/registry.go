/* Copyright 2020 PhysarumSM Development Team
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

// Client-side library for querying the registry-service

import (
    "context"
    "encoding/json"
    "errors"

    "github.com/libp2p/go-libp2p-core/host"
    "github.com/libp2p/go-libp2p-core/pnet"
    "github.com/libp2p/go-libp2p-discovery"

    "github.com/multiformats/go-multiaddr"

    "github.com/PhysarumSM/common/p2putil"
    "github.com/PhysarumSM/service-registry/common"
)

// Microservice info stored by registry-service
// Encoding/decoding of this struct is done client-side
// Registry-service simply stores the string it is given
type ServiceInfo struct {
    ContentHash string
    DockerHash string
    NetworkSoftReq p2putil.PerfInd
    NetworkHardReq p2putil.PerfInd
    CpuReq int
    MemoryReq int
}

// Functions ending in *Service create a temporary p2p node to communicate with registry-service
// Must pass in bootstrap addresses to connect to and optional PSK
// Functions ending in *ServiceWithHostRouting take in an existing p2p node and routing discovery
// to perform the operation without having to create that temporary p2p node

// Add service info {serviceName, info} to registry-service
func AddService(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string, info ServiceInfo) (
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

func AddServiceWithHostRouting(
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
    infoBytes, err := json.Marshal(info)
    if err != nil {
        return nil, err
    }
    reqInfo := common.AddRequest{Name: serviceName, InfoStr: string(infoBytes)}
    return json.Marshal(reqInfo)
}

// Get service info from registry-service by searching for service with a name matching the given query
func GetService(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, query string) (
    info ServiceInfo, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.GetProtocolID, []byte(query))
    if err != nil {
        return info, err
    }

    return unmarshalGetResponse(response)
}

func GetServiceWithHostRouting(
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
        return info, errors.New("registry: Error finding service info")
    }

    err = json.Unmarshal([]byte(respInfo.InfoStr), &info)
    if err != nil {
        return info, err
    }

    return info, nil
}

// List all services added to registry-service
// Returns mapping from service name to service info
func ListServices(bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    nameToInfo map[string]ServiceInfo, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.ListProtocolID, []byte{})
    if err != nil {
        return nil, err
    }

    return unmarshalListResponse(response)
}

func ListServicesWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery) (
    nameToInfo map[string]ServiceInfo, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.ListProtocolID, []byte{})
    if err != nil {
        return nil, err
    }

    return unmarshalListResponse(response)
}

func unmarshalListResponse(listResponse []byte) (nameToInfo map[string]ServiceInfo, err error) {
    var respInfo common.ListResponse
    err = json.Unmarshal(listResponse, &respInfo)
    if err != nil {
        return nil, err
    }

    if !respInfo.LookupOk {
        return nil, errors.New("registry: Error finding service info")
    }

    nameToInfo = make(map[string]ServiceInfo)
    for serviceName, infoStr := range respInfo.NameToInfoStr {
        var info ServiceInfo
        err = json.Unmarshal([]byte(infoStr), &info)
        if err != nil {
            return nil, err
        }
        nameToInfo[serviceName] = info
    }

    return nameToInfo, nil
}

// Delete service with given serviceName from registry-service
func DeleteService(bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string) (
    deleteResponse string, err error) {

    response, err := common.SendRequest(bootstraps, psk, common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}

func DeleteServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string) (
    deleteResponse string, err error) {

    response, err := common.SendRequestWithHostRouting(
        ctx, host, routingDiscovery, common.DeleteProtocolID, []byte(serviceName))
    if err != nil {
        return "", err
    }

    return string(response), nil
}
