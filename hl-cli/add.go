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
    "archive/tar"
    "bufio"
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "syscall"

    "golang.org/x/crypto/ssh/terminal"

    "github.com/Multi-Tier-Cloud/common/util"
    driver "github.com/Multi-Tier-Cloud/docker-driver/docker_driver"
    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
    "github.com/Multi-Tier-Cloud/service-manager/conf"
)

func addCmd() {
    addFlags := flag.NewFlagSet("add", flag.ExitOnError)
    proxyVersionFlag := addFlags.String("proxy-version", "",
        "Checkout specific version of proxy by supplying a commit hash. " +
        "By default, will use latest version checked into service-manager master. " +
        "This argument is supplied to git checkout <commit>, so a branch name or tags/<tag-name> works as well.")
    noAddFlag := addFlags.Bool("no-add", false,
        "Build and push image to DockerHub, but do not add to hash-lookup")

    addUsage := func() {
        exeName := getExeName()
        fmt.Fprintln(os.Stderr, "Usage:", exeName, "add [<options>] <config> <dir> <image-name> <service-name>")
        fmt.Fprintln(os.Stderr,
`
<config>
        Configuration file

<dir>
        Directory to find files listed in config

<image-name>
        Docker image of microservice to push to (<username>/<repository>:<tag>)

<service-name>
        Name of microservice to register with hash lookup

<options>`)
        addFlags.PrintDefaults()
    }
    
    addFlags.Usage = addUsage
    addFlags.Parse(flag.Args()[1:])
    
    if len(addFlags.Args()) < 4 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")
        addUsage()
        return
    }

    if len(addFlags.Args()) > 4 {
        fmt.Fprintln(os.Stderr, "Error: too many arguments")
        addUsage()
        return
    }

    configFile := addFlags.Arg(0)
    srcDir := addFlags.Arg(1)
    imageName := addFlags.Arg(2)
    serviceName := addFlags.Arg(3)

    err := buildServiceImage(configFile, srcDir, imageName, serviceName, *proxyVersionFlag)
    if err != nil {
        log.Fatalln(err)
    }

    imageBytes, err := driver.SaveImage(imageName)
    if err != nil {
        log.Fatalln(err)
    }

    hash, err := util.IpfsHashBytes(imageBytes)
    if err != nil {
        log.Fatalln(err)
    }

    digest, err := authAndPushImage(imageName)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Pushed to DockerHub successfully")

    parts := strings.Split(imageName, ":")
    dockerId := parts[0] + "@" + digest

    fmt.Printf("%s : {ContentHash: %s, DockerHash: %s}\n",
        serviceName, hash, dockerId)
    
    if *noAddFlag {
        return
    }

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

    respStr, err := hashlookup.AddHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName, hash, dockerId)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:", respStr)
}

func buildServiceImage(configFile, srcDir, imageName, serviceName, proxyVersion string) error {
    configBytes, err := ioutil.ReadFile(configFile)
    if err != nil {
        return err
    }
    config, err := unmarshalImageConf(configBytes)
    if err != nil {
        return err
    }

    buildContext, err := createDockerBuildContext(config, srcDir, serviceName, proxyVersion)
    if err != nil {
        return err
    }

    err = driver.BuildImage(buildContext, imageName)
    if err != nil {
        return err
    }

    return nil
}

type ImageConf struct {
    PerfConf conf.Config

    DockerConf struct {
        Copy [][2]string
        Run []string
        Cmd string
    }
}

func unmarshalImageConf(configBytes []byte) (config ImageConf, err error) {
    err = json.Unmarshal(configBytes, &config)
    if err != nil {
        return config, err
    }
    return config, nil
}

func createDockerBuildContext(config ImageConf, srcDir, serviceName, proxyVersion string) (
    imageBuildContext *bytes.Buffer, err error) {

    imageBuildContext = new(bytes.Buffer)
    tw := tar.NewWriter(imageBuildContext)
    defer tw.Close()

    dockerfileBytes := createDockerfile(config, serviceName)
    dockerfileHdr := &tar.Header{
        Name: "Dockerfile",
        Mode: 0646,
        Size: int64(len(dockerfileBytes)),
    }
    err = tw.WriteHeader(dockerfileHdr)
    if err != nil {
        return nil, err
    }
    _, err = tw.Write(dockerfileBytes)
    if err != nil {
        return nil, err
    }

    perfconfBytes, err := json.Marshal(config.PerfConf)
    if err != nil {
        return nil, err
    }
    perfconfHdr := &tar.Header{
        Name: "conf.json",
        Mode: 0646,
        Size: int64(len(perfconfBytes)),
    }
    err = tw.WriteHeader(perfconfHdr)
    if err != nil {
        return nil, err
    }
    _, err = tw.Write(perfconfBytes)
    if err != nil {
        return nil, err
    }

    tmpDir, proxyPath, err := buildProxy(proxyVersion)
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(tmpDir)

    err = tarAddFile(tw, proxyPath, "proxy")
    if err != nil {
        return nil, err
    }

    for _, copyArgs := range config.DockerConf.Copy {
        srcPath := filepath.Join(srcDir, copyArgs[0])
        err = tarAddFile(tw, srcPath, copyArgs[0])
        if err != nil {
            return nil, err
        }
    }

    return imageBuildContext, nil
}

const dockerfileCore string =
`FROM ubuntu:16.04
WORKDIR /app
COPY proxy .
COPY conf.json .
ENV PROXY_PORT=4201
ENV PROXY_IP=127.0.0.1
ENV SERVICE_PORT=8080
`

func createDockerfile(config ImageConf, serviceName string) []byte {
    var dockerfile bytes.Buffer
    dockerfile.WriteString(dockerfileCore)
    for _, copyArgs := range config.DockerConf.Copy {
        dockerfile.WriteString(fmt.Sprintln("COPY", copyArgs[0], copyArgs[1]))
    }
    for _, runCmd := range config.DockerConf.Run {
        dockerfile.WriteString(fmt.Sprintln("RUN", runCmd))
    }
    dockerfile.WriteString(fmt.Sprintf(
        "CMD ./proxy --configfile conf.json $PROXY_PORT %s $PROXY_IP:$SERVICE_PORT > proxy.log 2>&1 & %s\n",
        serviceName, config.DockerConf.Cmd))
    return dockerfile.Bytes()
}

func buildProxy(version string) (tmpDir, proxyPath string, err error) {
    tmpDir, err = ioutil.TempDir("", "proxy-")
    if err != nil {
        return "", "", err
    }

    cloneCmd := exec.Command("git", "clone", "https://github.com/Multi-Tier-Cloud/service-manager.git")
    cloneCmd.Dir = tmpDir
    cloneCmd.Stdout = os.Stdout
    cloneCmd.Stderr = os.Stderr
    err = cloneCmd.Run()
    if err != nil {
        os.RemoveAll(tmpDir)
        return "", "", err
    }

    if version != "" {
        checkoutCmd := exec.Command("git", "checkout", version)
        checkoutCmd.Dir = filepath.Join(tmpDir, "service-manager")
        checkoutCmd.Stdout = os.Stdout
        checkoutCmd.Stderr = os.Stderr
        err = checkoutCmd.Run()
        if err != nil {
            os.RemoveAll(tmpDir)
            return "", "", err
        }
    }

    buildCmd := exec.Command("go", "build", "-o", "proxy")
    buildCmd.Dir = filepath.Join(tmpDir, "service-manager/proxy")
    buildCmd.Stdout = os.Stdout
    buildCmd.Stderr = os.Stderr
    err = buildCmd.Run()
    if err != nil {
        os.RemoveAll(tmpDir)
        return "", "", err
    }

    proxyPath = filepath.Join(buildCmd.Dir, "proxy")
    _, err = os.Lstat(proxyPath)
    if err != nil {
        os.RemoveAll(tmpDir)
        return "", "", err
    }

    return tmpDir, proxyPath, nil
}

func tarAddFile(tw *tar.Writer, srcPath, dstPath string) error {
    fileInfo, err := os.Lstat(srcPath)
    if err != nil {
        return err
    }
    hdr, err := tar.FileInfoHeader(fileInfo, fileInfo.Name())
    if err != nil {
        return err
    }
    hdr.Name = dstPath
    err = tw.WriteHeader(hdr)
    if err != nil {
        return err
    }

    fileBytes, err := ioutil.ReadFile(srcPath)
    if err != nil {
        return err
    }
    _, err = tw.Write(fileBytes)
    if err != nil {
        return err
    }

    return nil
}

func authAndPushImage(image string) (string, error) {
    fmt.Println("DockerHub login")
    scanner := bufio.NewScanner(os.Stdin)
    fmt.Print("Username: ")
    scanner.Scan()
    username := scanner.Text()
    fmt.Print("Password: ")
    passwordBytes, err := terminal.ReadPassword(syscall.Stdin)
    if err != nil {
        return "", err
    }
    fmt.Println()

    auth, err := driver.CreateEncodedAuth(username, string(passwordBytes))
    if err != nil {
        return "", err
    }

    digest, err := driver.PushImage(auth, image)
    if err != nil {
        return "", err
    }

    return digest, nil
}
