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

 import (
    "encoding/json"
    "io/ioutil"
    "log"
    "strings"

    "github.com/libp2p/go-libp2p-core/network"

    "go.etcd.io/etcd/clientv3"

    "github.com/Multi-Tier-Cloud/service-registry/common"
)

func handleGet(etcdCli *clientv3.Client) func(network.Stream) {
    return func(stream network.Stream) {
        data, err := ioutil.ReadAll(stream)
        if err != nil {
            streamError(stream, err)
            return
        }

        reqStr := strings.TrimSpace(string(data))
        log.Println("Lookup request:", reqStr)

        infoStr, ok, err := getServiceInfo(etcdCli, reqStr)
        if err != nil {
            streamError(stream, err)
            return
        }

        respInfo := common.GetResponse{InfoStr: infoStr, LookupOk: ok}
        respBytes, err := json.Marshal(respInfo)
        if err != nil {
            streamError(stream, err)
            return
        }

        log.Println("Lookup response: ", string(respBytes))

        _, err = stream.Write(respBytes)
        if err != nil {
            streamError(stream, err)
            return
        }

        stream.Close()
    }
}
