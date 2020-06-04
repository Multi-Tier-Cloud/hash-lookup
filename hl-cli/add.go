package main

import (
    "archive/tar"
    "bufio"
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "errors"
    "flag"
    "fmt"
    "io"
    "io/ioutil"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "syscall"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/client"
    
    "github.com/ipfs/go-blockservice"
    "github.com/ipfs/go-ipfs/core"
    "github.com/ipfs/go-ipfs/core/coreunix"
    "github.com/ipfs/go-ipfs-files"
    dag "github.com/ipfs/go-merkledag"
    dagtest "github.com/ipfs/go-merkledag/test"
    "github.com/ipfs/go-mfs"
    "github.com/ipfs/go-unixfs"

    "golang.org/x/crypto/ssh/terminal"

    "github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
    "github.com/Multi-Tier-Cloud/service-manager/conf"
)

func addCmd() {
    addFlags := flag.NewFlagSet("add", flag.ExitOnError)
    // fileFlag := addFlags.String("file", "",
    //     "Hash microservice content from file, or directory (recursively)")
    // stdinFlag := addFlags.Bool("stdin", false,
    //     "Hash microservice content from stdin")
    noAddFlag := addFlags.Bool("no-add", false,
        "Only hash the given content, do not add it to hash-lookup service")
    bootstrapFlag := addFlags.String("bootstrap", "",
        "For debugging: Connect to specified bootstrap node multiaddress")

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
    addFlags.Parse(os.Args[2:])
    
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

    // if *fileFlag != "" && *stdinFlag {
    //     fmt.Fprintln(os.Stderr,
    //         "Error: --file and --stdin are mutually exclusive options")
    //     addUsage()
    //     return
    // }

    // var hash string = ""
    // var err error = nil

    // if *fileFlag != "" {
    //     hash, err = fileHash(*fileFlag)
    //     if err != nil {
    //         panic(err)
    //     }
    //     fmt.Println("Hashed file:", hash)
    // } else if *stdinFlag {
    //     hash, err = stdinHash()
    //     if err != nil {
    //         panic(err)
    //     }
    //     fmt.Println("Hashed stdin:", hash)
    // } else {
    //     fmt.Fprintln(os.Stderr,
    //         "Error: must specify either the --file or --stdin options")
    //     addUsage()
    //     return
    // }

    configFile := addFlags.Arg(0)
    srcDir := addFlags.Arg(1)
    imageName := addFlags.Arg(2)
    serviceName := addFlags.Arg(3)

    err := buildServiceImage(configFile, srcDir, imageName, serviceName)
    if err != nil {
        panic(err)
    }

    imageBytes, err := saveImage(imageName)
    if err != nil {
        panic(err)
    }

    hash, err := bytesHash(imageBytes)
    if err != nil {
        panic(err)
    }

    digest, err := pushImage(imageName)
    if err != nil {
        panic(err)
    }

    fmt.Println("Pushed to DockerHub successfully")

    parts := strings.Split(imageName, ":")
    dockerId := parts[0] + "@" + digest

    fmt.Printf("Adding %s:{ContentHash:%s, DockerHash:%s}\n",
        serviceName, hash, dockerId)
    
    if *noAddFlag {
        return
    }

    ctx, node, err := setupNode(*bootstrapFlag)
    if err != nil {
        panic(err)
    }
    defer node.Host.Close()
    defer node.DHT.Close()

    respStr, err := hashlookup.AddHashWithHostRouting(
        ctx, node.Host, node.RoutingDiscovery, serviceName, hash, dockerId)
    if err != nil {
        panic(err)
    }

    fmt.Println("Response:", respStr)
}

type ImageConf struct {
    PerfConf conf.Config

    DockerConf struct {
        Copy [][2]string
        Run []string
        Cmd string
    }
}

const dockerfileCore string =
`FROM ubuntu:16.04
WORKDIR /app
COPY proxy .
COPY perf.conf .
ENV PROXY_PORT=4201
ENV PROXY_IP=127.0.0.1
ENV SERVICE_PORT=8080
`

func buildServiceImage(configFile, srcDir, imageName, serviceName string) error {
    configBytes, err := ioutil.ReadFile(configFile)
    if err != nil {
        return err
    }
    config, err := unmarshalImageConf(configBytes)
    if err != nil {
        return err
    }

    buildContext, err := createDockerBuildContext(config, srcDir, serviceName)
    if err != nil {
        return err
    }

    err = buildImage(buildContext, imageName)
    if err != nil {
        return err
    }

    return nil
}

func unmarshalImageConf(configBytes []byte) (config ImageConf, err error) {
    err = json.Unmarshal(configBytes, &config)
    if err != nil {
        return config, err
    }
    return config, nil
}

func createDockerBuildContext(config ImageConf, srcDir, serviceName string) (
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
        Name: "perf.conf",
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

    tmpDir, proxyPath, err := buildProxy("")
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
        "CMD ./proxy $PROXY_PORT %s $PROXY_IP:$SERVICE_PORT > proxy.log 2>&1 & %s\n",
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

func buildImage(buildContext io.Reader, imageName string) error {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return err
    }

    resp, err := cli.ImageBuild(ctx, buildContext, types.ImageBuildOptions{Tags: []string{imageName}})
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        // fmt.Println(line)

        var respBodyObject struct {
            // Normal response lines have "stream" field instead of "error"
            Error string
        }

        err = json.Unmarshal([]byte(line), &respBodyObject)
        if err != nil {
            return err
        }

        if respBodyObject.Error != "" {
            return errors.New(respBodyObject.Error)
        }
    }

    return nil
}

func saveImage(imageName string) ([]byte, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return nil, err
    }

    resp, err := cli.ImageSave(ctx, []string{imageName})
    if err != nil {
        return nil, err
    }
    defer resp.Close()

    respBytes, err := ioutil.ReadAll(resp)
    if err != nil {
        return nil, err
    }

    return respBytes, nil
}

func getAuth() (string, error) {
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

    authConfig := types.AuthConfig{
        Username: username,
        Password: string(passwordBytes),
    }
    encodedJSON, err := json.Marshal(authConfig)
    if err != nil {
        return "", err
    }
    authStr := base64.URLEncoding.EncodeToString(encodedJSON)
    return authStr, nil
}

func pushImage(image string) (string, error) {
    authStr, err := getAuth()
    if err != nil {
        return "", err
    }

    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return "", err
    }

    resp, err := cli.ImagePush(ctx, image, types.ImagePushOptions{RegistryAuth:authStr})
    if err != nil {
        return "", err
    }
    defer resp.Close()

    scanner := bufio.NewScanner(resp)
    for scanner.Scan() {
        line := scanner.Text()
        // fmt.Println(line)

        var respObject struct {
            Aux struct {
                Digest string
            }
            Error string
        }

        err = json.Unmarshal([]byte(line), &respObject)
        if err != nil {
            return "", err
        }

        if respObject.Aux.Digest != "" {
            return respObject.Aux.Digest, nil
        } else if respObject.Error != "" {
            return "", errors.New(respObject.Error)
        }
    }

    return "", errors.New("hl-cli: Error did not receive digest")
}

func fileHash(path string) (hash string, err error) {
    stat, err := os.Lstat(path)
    if err != nil {
        return "", err
    }

    fileNode, err := files.NewSerialFile(path, false, stat)
    if err != nil {
        return "", err
    }
    defer fileNode.Close()

    return getHash(fileNode)
}

func stdinHash() (hash string, err error) {
    stdinData, err := ioutil.ReadAll(os.Stdin)
    if err != nil {
        return "", err
    }

    stdinFile := files.NewBytesFile(stdinData)

    return getHash(stdinFile)
}

func bytesHash(data []byte) (hash string, err error) {
    bytesFile := files.NewBytesFile(data)
    return getHash(bytesFile)
}

func getHash(fileNode files.Node) (hash string, err error) {
    ctx := context.Background()
    nilIpfsNode, err := core.NewNode(ctx, &core.BuildCfg{NilRepo: true})
    if err != nil {
        return "", err
    }

    bserv := blockservice.New(nilIpfsNode.Blockstore, nilIpfsNode.Exchange)
    dserv := dag.NewDAGService(bserv)

    fileAdder, err := coreunix.NewAdder(
        ctx, nilIpfsNode.Pinning, nilIpfsNode.Blockstore, dserv)
    if err != nil {
        return "", err
    }

    fileAdder.Pin = false
    fileAdder.CidBuilder = dag.V0CidPrefix()

    mockDserv := dagtest.Mock()
    emptyDirNode := unixfs.EmptyDirNode()
    emptyDirNode.SetCidBuilder(fileAdder.CidBuilder)
    mfsRoot, err := mfs.NewRoot(ctx, mockDserv, emptyDirNode, nil)
    if err != nil {
        return "", err
    }
    fileAdder.SetMfsRoot(mfsRoot)

    dagIpldNode, err := fileAdder.AddAllAndPin(fileNode)
    if err != nil {
        return "", err
    }

    hash = dagIpldNode.String()
    return hash, nil
}