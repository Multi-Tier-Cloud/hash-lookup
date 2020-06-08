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

// May need to run with sudo to perform Docker tests
// sudo env "PATH=$PATH" go test
// In case "go" is not in sudo path

import (
    "fmt"
    "io/ioutil"
    "os"
    "testing"
)

var config ImageConf

func TestMain(m *testing.M) {
    configBytes, err := ioutil.ReadFile("image.conf")
    if err != nil {
        panic(err)
    }

    config, err = unmarshalImageConf(configBytes)
    if err != nil {
        panic(err)
    }

    os.Exit(m.Run())
}

func TestCreateDockerfile(t *testing.T) {
    dockerfileBytes := createDockerfile(config, "test-name")
    fmt.Println(string(dockerfileBytes))
}

func TestBuildProxy(t *testing.T) {
    tmpDir, _, err := buildProxy("")
    if err != nil {
        t.Errorf("%v", err)
    }
    defer os.RemoveAll(tmpDir)
}

func TestCreateDockerBuildContext(t *testing.T) {
    buildContext, err := createDockerBuildContext(config, "test", "test-name")
    if err != nil {
        t.Errorf("%v", err)
    }
    err = ioutil.WriteFile("test.tar", buildContext.Bytes(), 0666)
    if err != nil {
        t.Errorf("%v", err)
    }
}

func TestBuildServiceImage(t *testing.T) {
    err := buildServiceImage("image.conf", "test", "hivanco/test-image:1.0" "test-service:1.0")
    if err != nil {
        t.Errorf("%v", err)
    }
}

func TestSaveImage(t *testing.T) {
    imageBytes, err := saveImage("hivanco/test-image:1.0")
    if err != nil {
        t.Errorf("%v", err)
    }
    err = ioutil.WriteFile("test-image.tar.gz", imageBytes, 0666)
    if err != nil {
        t.Errorf("%v", err)
    }
}