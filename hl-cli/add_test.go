// May need to run with sudo to perform Docker tests
// sudo env "PATH=$PATH" go test
// In case "go" is not in sudo path

package main

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