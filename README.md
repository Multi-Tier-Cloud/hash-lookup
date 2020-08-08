# Service Registry

registry: Client-side library for querying the registry-service.

registry-cli: Client-side CLI tool used to interact with registry-service.

registry-service: Registry-service which stores information about microservices, indexed by name.

common: Code reused throughout this repo.

## Registry Package

Provides 4 registry operations, add/get/list/delete, each with 2 function variants. Functions ending in *Service create a temporary p2p node to communicate with registry-service. Functions ending in *ServiceWithHostRouting take in an existing p2p node and routing discovery to perform the operation without having to create that temporary p2p node. For examples calling these functions, see registry-cli.

```
type ServiceInfo struct {
    ContentHash string
    DockerHash string
    NetworkSoftReq p2putil.PerfInd
    NetworkHardReq p2putil.PerfInd
    CpuReq int
    MemoryReq int
}

// Add service info {serviceName, info} to registry-service
func AddService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string, info ServiceInfo) (
    addResponse string, err error)

func AddServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string, info ServiceInfo) (
    addResponse string, err error)

// Get service info from registry-service by searching for service with a name matching the given query
func GetService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, query string) (
    info ServiceInfo, err error)

func GetServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, query string) (
    info ServiceInfo, err error)

// List all services added to registry-service
// Returns mapping from service name to service info
func ListServicees(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    nameToInfo map[string]ServiceInfo, err error)

func ListServiceesWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery) (
    nameToInfo map[string]ServiceInfo, err error)

// Delete service with given serviceName from registry-service
func DeleteService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string) (
    deleteResponse string, err error)

func DeleteServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string) (
    deleteResponse string, err error)
```

## Registry-CLI

Allows users to easily add/get/list/delete registry-service info. Uses the registry package functions.
For full usage details run `$ registry-cli --help` or `$ registry-cli <command> --help`.

```
Usage of registry-cli:
$ registry-cli [OPTIONS ...] <command>

OPTIONS:
  -bootstrap value
        Multiaddress of a bootstrap node.
        This flag can be specified multiple times.
        Alternatively, an environment variable named P2P_BOOTSTRAPS can
        be set with a space-separated list of bootstrap multiaddresses.
  -psk value
        Passphrase used to create a pre-shared key (PSK) used amongst nodes
        to form a private network. It is HIGHLY RECOMMENDED you use a
        passphrase you can easily memorize, or write it down somewhere safe.
        If you forget the passphrase, you will be unable to join new nodes
        and services to the same network.
        Alternatively, an environment variable named P2P_PSK can
        be set with the passphrase.

Available commands are:
  add
        Builds a Docker image for a given microservice, pushes to DockerHub, and adds it to the registry-service
  get
        Get information about a microservice
  list
        List all microservices and information stored by the registry-service
  delete
        Delete a microservice entry
```

### Add command
```
Usage of registry-cli add:
$ registry-cli add [OPTIONS ...] <config> <image-name> <service-name>

Builds a Docker image for a given microservice, pushes to DockerHub, and adds it to the registry-service

Example:
$ ./registry-service add --dir ./image-files ./service-conf.json username/service:1.0 my-service:1.0

<config>
        Configuration file

<image-name>
        Docker image of microservice to push to (<username>/<repository>:<tag>)

<service-name>
        Name of microservice to register with hash lookup

OPTIONS:
  -custom-proxy string
        Use a locally built proxy binary instead of checking out and building one from source.
  -dir string
        Directory to find files listed in config file (default ".")
  -no-add
        Build image, but do not push to Dockerhub or add to registry-service
  -proxy-cmd string
        Use specified command to run proxy. ie. './proxy --configfile conf.json $PROXY_PORT'
        Note the automatically generated proxy config file will be named 'conf.json'.
  -proxy-version string
        Checkout specific version of proxy by supplying a commit hash.
        By default, will use latest version checked into service-manager master.
        This argument is supplied to git checkout, so a branch name or tags/<tag-name> works as well.
  -use-existing-image
        Do not build/push new image. Pull an existing image from DockerHub and add it to registry-service.
        Note that you still have to provide a config file since it is needed for performance requirements.
```

Config is a json file used to setup the microservice. Its format is as follows:
```
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
        "From": string(base docker image; default ubuntu:16.04),
        "Copy": [
            [string(local src path), string(image dst path)]
        ],
        "Run": [
            string(command)
        ],
        "Cmd": string(command to run your microservice),
        "ProxyClientMode": bool(true to run proxy in client mode, false for service mode; default false)
    }
}
```
Every microservice gets packaged with a proxy in the same container. NetworkSoftReq and NetworkHardReq are performance requirements passed to the proxy, used when the proxy selects microservices to connect to. DockerConf defines instructions for building the docker image for your microservice. They mostly translate to Dockerfile directives. For examples, see registry-cli/add-test and https://github.com/PhysarumSM/demos.

The Dockerfile generated for building the image starts with the following core directives:
```
WORKDIR /app
COPY proxy .
COPY conf.json .
ENV PROXY_PORT=4201
ENV PROXY_IP=127.0.0.1
ENV SERVICE_PORT=8080
ENV P2P_BOOTSTRAPS=
ENV P2P_PSK=
```
It sets the working directory in the image to /app, and copies in a proxy and its proxy config file. So in the service config's DockerConf.Copy field, the image destination path is relative to /app. This is probably where you microservice's files should go as well.

All the environment variables will be set dynamically when a new container is spun up. In particular, note the SERVICE_PORT and PROXY_IP variables. PROXY_IP is the IP address of the machine that the container will be running on. SERVICE_PORT is the port that the microserivce should listen on. You will probably use these in the service config's DockerConf.Cmd field.

### Get command
```
Usage of registry-cli get:
$ registry-cli get <service-name>

Get information about a microservice

<service-name>
        Name of microservice to get hash of
```

### List command
```
Usage of registry-cli list:
$ registry-cli list

List all microservices and information stored by the registry-service
```

### Delete command
```
Usage of registry-cli delete:
$ registry-cli delete <service-name>

Delete a microservice entry

<service-name>
        Name of microservice to delete
```

## Registry-Service

The service that stores information about microservices. Any service needs to be registered here before it can be deployed to the system. Stores info in {key, value} pairs, where key is service name, and value is a json encoded ServiceInfo string. Uses etcd key-value store under the hood. Each registry-service instance will run its own etcd instance, which will form a cluster together so all instances maintain the same data. When starting a new cluster, run the first registry-service with the --new-etcd-cluster flag. Subsequent instances can omit this flag.

```
Usage of registry-service:
  -algo string
        Cryptographic algorithm to use for generating the key.
        Will be ignored if 'genkey' is false.
        Must be one of {RSA, Ed25519, Secp256k1, ECDSA} (default "RSA")
  -bits int
        Key length, in bits. Will be ignored if 'algo' is not RSA. (default 2048)
  -bootstrap value
        Multiaddress of a bootstrap node.
        This flag can be specified multiple times.
        Alternatively, an environment variable named P2P_BOOTSTRAPS can
        be set with a space-separated list of bootstrap multiaddresses.
  -ephemeral
        Generate a new key just for this run, and don't store it to file.
        If 'keyfile' is specified, it will be ignored.
  -etcd-client-port int
        Local etcd instance client port (default 2379)
  -etcd-ip string
        Local etcd instance IP address (default "127.0.0.1")
  -etcd-peer-port int
        Local etcd instance peer port (default 2380)
  -keyfile string
        Location of private key to read from (or write to, if generating). (default "~/.privKeyHashLookup")
  -local
        For debugging: Run locally and do not connect to bootstrap peers
        (this option overrides the '--bootstrap' flag)
  -new-etcd-cluster
        Start running new etcd cluster
  -prom-listen-addr string
        Listening address/endpoint for Prometheus to scrape (default ":9102")
  -psk value
        Passphrase used to create a pre-shared key (PSK) used amongst nodes
        to form a private network. It is HIGHLY RECOMMENDED you use a
        passphrase you can easily memorize, or write it down somewhere safe.
        If you forget the passphrase, you will be unable to join new nodes
        and services to the same network.
        Alternatively, an environment variable named P2P_PSK can
        be set with the passphrase.
```
