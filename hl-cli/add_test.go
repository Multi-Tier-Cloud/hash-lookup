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