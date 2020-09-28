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

    "github.com/PhysarumSM/common/p2putil"
    "github.com/PhysarumSM/common/util"
    driver "github.com/PhysarumSM/docker-driver/docker_driver"
    "github.com/PhysarumSM/service-registry/registry"
    "github.com/PhysarumSM/service-manager/conf"
)

// Decode json config file into this struct
// Defines performance requirements and instructions for building docker image
type ServiceConf struct {
    NetworkSoftReq p2putil.PerfInd
    NetworkHardReq p2putil.PerfInd
    CpuReq int
    MemoryReq int

    DockerConf struct {
        From string
        Copy [][2]string
        Run []string
        Cmd string
        ProxyClientMode bool
    }
}

func addCmd() {
    addFlags := flag.NewFlagSet("add", flag.ExitOnError)
    dirFlag := addFlags.String("dir", ".",
        "Directory to find files listed in config file")
    customProxyFlag := addFlags.String("custom-proxy", "",
        "Use a locally built proxy binary instead of checking out and building one from source.")
    proxyVersionFlag := addFlags.String("proxy-version", "",
        "Checkout specific version of proxy by supplying a commit hash.\n" +
        "By default, will use latest version checked into service-manager master.\n" +
        "This argument is supplied to git checkout, so a branch name or tags/<tag-name> works as well.")
    proxyCmdFlag := addFlags.String("proxy-cmd", "",
        "Use specified command to run proxy. ie. './proxy --configfile conf.json $PROXY_PORT'\n" +
        "Note the automatically generated proxy config file will be named 'conf.json'.")
    noAddFlag := addFlags.Bool("no-add", false,
        "Build image, but do not push to Dockerhub or add to registry-service")
    useExistingImageFlag := addFlags.Bool("use-existing-image", false,
        "Do not build/push new image. Pull an existing image from DockerHub and add it to registry-service.\n" +
        "Note that you still have to provide a config file since it is needed for performance requirements.")

    addUsage := func() {
        exeName := getExeName()
        fmt.Fprintf(os.Stderr, "Usage of %s add:\n", exeName)
        fmt.Fprintf(os.Stderr, "$ %s add [OPTIONS ...] <config> <image-name> <service-name>\n", exeName)
        fmt.Fprintln(os.Stderr,
`
Builds a Docker image for a given microservice, pushes to DockerHub, and adds it to the registry-service

Example:
$ ./registry-service add --dir ./image-files ./service-conf.json username/service:1.0 my-service:1.0

<config>
        Microservice configuration file

<image-name>
        Image name or DockerHub repo to push to (<username>/<repository>:<tag>)

<service-name>
        Name of microservice to register with hash lookup

OPTIONS:`)

        addFlags.PrintDefaults()

        fmt.Fprintln(os.Stderr,
`
Config is a json file used to setup the microservice. Its format is as follows:
{
    "NetworkSoftReq": {
        "RTT": int(milliseconds)
    },
    "NetworkHardReq": {
        "RTT": int(milliseconds)
    },
    "CpuReq": int,
    "MemoryReq": int,

    "DockerConf": {
        "From": string(base docker image; defaults to ubuntu:16.04),
        "Copy": [
            [string(local src path), string(image dst path)]
        ],
        "Run": [
            string(command)
        ],
        "Cmd": string(command to run your microservice),
        "ProxyClientMode": bool(true to run proxy in client mode, false for service mode; defaults to false)
    }
}`)
    }

    addFlags.Usage = addUsage
    addFlags.Parse(flag.Args()[1:])

    if len(addFlags.Args()) < 3 {
        fmt.Fprintln(os.Stderr, "Error: missing required arguments")
        addUsage()
        return
    }

    if len(addFlags.Args()) > 3 {
        fmt.Fprintln(os.Stderr, "Error: too many arguments")
        addUsage()
        return
    }

    // Positional arguments
    configFile := addFlags.Arg(0)
    imageName := addFlags.Arg(1)
    serviceName := addFlags.Arg(2)

    // Read json config file
    configBytes, err := ioutil.ReadFile(configFile)
    if err != nil {
        log.Fatalln(err)
    }
    config, err := unmarshalServiceConf(configBytes)
    if err != nil {
        // Print error + offending line
        syntaxErr, ok := err.(*json.SyntaxError)
        if !ok {
            log.Fatalln(err)
        }
        lineSlices := bytes.SplitAfter(configBytes, []byte("\n"))
        byteSum := 0
        for i, line := range lineSlices {
            if byteSum + len(line) >= int(syntaxErr.Offset) {
                log.Fatalf("%v (Line %d: %s)", err, i+1, bytes.TrimSpace(line))
            }
            byteSum += len(line)
        }
    }

    var digest string

    // Check whether to build new image or pull existing image
    if !(*useExistingImageFlag) {
        fmt.Println("Building new image")
        err = buildServiceImage(config, imageName, serviceName, *dirFlag,
            *customProxyFlag, *proxyVersionFlag, *proxyCmdFlag)
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Println("Built image successfully")
    } else {
        fmt.Println("Pulling existing image")
        digest, err = driver.PullImage(imageName)
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Println("Image pulled successfully")
    }

    if *noAddFlag {
        return // Don't push to DockerHub or add to registry-service, just return early
    }

    // Save tar archive of image
    imageBytes, err := driver.SaveImage(imageName)
    if err != nil {
        log.Fatalln(err)
    }
    fmt.Println("Saved image successfully")

    // Get content hash of image
    hash, err := util.IpfsHashBytes(imageBytes)
    if err != nil {
        log.Fatalln(err)
    }
    fmt.Println("Hashed image successfully:", hash)

    // If used existing image, no need to push to DockerHub
    if !(*useExistingImageFlag) {
        fmt.Println("Pushing to DockerHub")
        digest, err = authAndPushImage(imageName)
        if err != nil {
            log.Fatalln(err)
        }
        fmt.Println("Pushed to DockerHub successfully")
    }

    // Construct docker ID used for pulling image
    // Take beginning <username>/<repo> portion, and concatenate image digest
    parts := strings.Split(imageName, ":")
    dockerId := parts[0] + "@" + digest

    fmt.Printf("%s : {ContentHash: %s, DockerHash: %s}\n", serviceName, hash, dockerId)

    ctx, node, err := setupNode(*bootstraps, *psk)
    if err != nil {
        log.Fatalln(err)
    }
    defer node.Close()

    // Add to registry-service
    info := registry.ServiceInfo{
        ContentHash: hash,
        DockerHash: dockerId,
        NetworkSoftReq: config.NetworkSoftReq,
        NetworkHardReq: config.NetworkHardReq,
        CpuReq: config.CpuReq,
        MemoryReq: config.MemoryReq,
    }
    respStr, err := registry.AddServiceWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName, info)
    if err != nil {
        log.Fatalln(err)
    }

    fmt.Println("Response:")
    fmt.Println(respStr)
}

func unmarshalServiceConf(configBytes []byte) (config ServiceConf, err error) {
    err = json.Unmarshal(configBytes, &config)
    if err != nil {
        return config, err
    }
    return config, nil
}

func buildServiceImage(config ServiceConf, imageName, serviceName, srcDir,
    customProxy, proxyVersion, proxyCmd string) error {

    buildContext, err := createDockerBuildContext(config, serviceName, srcDir,
        customProxy, proxyVersion, proxyCmd)
    if err != nil {
        return err
    }

    err = driver.BuildImage(buildContext, imageName)
    if err != nil {
        return err
    }

    return nil
}

func createDockerBuildContext(config ServiceConf, serviceName, srcDir,
    customProxy, proxyVersion, proxyCmd string) (imageBuildContext *bytes.Buffer, err error) {

    imageBuildContext = new(bytes.Buffer)
    tw := tar.NewWriter(imageBuildContext)
    defer tw.Close()

    dockerfileBytes := createDockerfile(config, serviceName, proxyCmd)
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

    proxyConfig := conf.Config{
        Perf: struct{
            SoftReq p2putil.PerfInd
            HardReq p2putil.PerfInd
        }{
            SoftReq: config.NetworkSoftReq,
            HardReq: config.NetworkHardReq,
        },
    }
    confJsonBytes, err := json.Marshal(proxyConfig)
    if err != nil {
        return nil, err
    }
    confJsonHdr := &tar.Header{
        Name: "conf.json",
        Mode: 0646,
        Size: int64(len(confJsonBytes)),
    }
    err = tw.WriteHeader(confJsonHdr)
    if err != nil {
        return nil, err
    }
    _, err = tw.Write(confJsonBytes)
    if err != nil {
        return nil, err
    }

    var proxyPath string
    if customProxy != "" {
        proxyPath = customProxy
    } else {
        var tmpDir string
        tmpDir, proxyPath, err = buildProxy(proxyVersion)
        if err != nil {
            return nil, err
        }
        defer os.RemoveAll(tmpDir)
    }

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
`WORKDIR /app
COPY proxy .
COPY conf.json .
ENV PROXY_PORT=4201
ENV PROXY_IP=127.0.0.1
ENV SERVICE_PORT=8080
ENV METRICS_PORT=9010
ENV P2P_BOOTSTRAPS=
ENV P2P_PSK=
`

func createDockerfile(config ServiceConf, serviceName, proxyCmd string) []byte {
    var dockerfile bytes.Buffer

    fromBase := "ubuntu:16.04"
    if config.DockerConf.From != "" {
        fromBase = config.DockerConf.From
    }
    dockerfile.WriteString(fmt.Sprintln("FROM", fromBase))

    dockerfile.WriteString(dockerfileCore)

    for _, copyArgs := range config.DockerConf.Copy {
        dockerfile.WriteString(fmt.Sprintln("COPY", copyArgs[0], copyArgs[1]))
    }

    for _, runCmd := range config.DockerConf.Run {
        dockerfile.WriteString(fmt.Sprintln("RUN", runCmd))
    }

    var finalCmd string
    if proxyCmd != "" {
        finalCmd = fmt.Sprintf("CMD %s & %s\n", proxyCmd, config.DockerConf.Cmd)
    } else if !config.DockerConf.ProxyClientMode {
        finalCmd = fmt.Sprintf(
            "CMD ./proxy --configfile conf.json $PROXY_PORT %s $PROXY_IP:$SERVICE_PORT $METRICS_PORT > proxy.log 2>&1 & %s\n",
            serviceName, config.DockerConf.Cmd)
    } else {
        finalCmd = fmt.Sprintf(
            "CMD ./proxy --configfile conf.json $PROXY_PORT > proxy.log 2>&1 & %s\n", config.DockerConf.Cmd)
    }
    dockerfile.WriteString(finalCmd)

    dockerfileBytes := dockerfile.Bytes()
    fmt.Printf("Dockerfile for %s:\n", serviceName)
    fmt.Println(string(dockerfileBytes))
    return dockerfileBytes
}

func buildProxy(version string) (tmpDir, proxyPath string, err error) {
    tmpDir, err = ioutil.TempDir("", "proxy-")
    if err != nil {
        return "", "", err
    }

    cloneCmd := exec.Command("git", "clone", "https://github.com/PhysarumSM/service-manager.git")
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
