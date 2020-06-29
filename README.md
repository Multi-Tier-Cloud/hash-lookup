# hash-lookup

registry: Client-side library for querying the registry-service.

registry-cli: Client-side CLI tool used to interact with registry-service.

registry-service: Registry-service which stores information about microservices, indexed by name.

common: Code reused throughout this project.

## Registry

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


func AddService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string, info ServiceInfo) (
    addResponse string, err error)

func AddServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string, info ServiceInfo) (
    addResponse string, err error)


func GetService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, query string) (
    info ServiceInfo, err error)

func GetServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, query string) (
    info ServiceInfo, err error)


func ListServicees(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK) (
    nameToInfo map[string]ServiceInfo, err error)

func ListServiceesWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery) (
    nameToInfo map[string]ServiceInfo, err error)


func DeleteService(
    bootstraps []multiaddr.Multiaddr, psk pnet.PSK, serviceName string) (
    deleteResponse string, err error)

func DeleteServiceWithHostRouting(
    ctx context.Context, host host.Host, routingDiscovery *discovery.RoutingDiscovery, serviceName string) (
    deleteResponse string, err error)
```

## Registry CLI

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
        Hash a microservice and add it to the registry-service
  get
        Get the content hash and Docker ID of a microservice
  list
        List all microservices and data stored by the registry-service
  delete
        Delete a microservice entry
```

### Add command
```
Usage: registry-cli add [<options>] <config> <dir> <image-name> <service-name>

<config>
        Configuration file

<dir>
        Directory to find files listed in config

<image-name>
        Docker image of microservice to push to (<username>/<repository>:<tag>)

<service-name>
        Name of microservice to register with hash lookup

<options>
  -custom-proxy string
        Provide a locally built proxy binary instead of building one from source.
  -no-add
        Build image, but do not push to Dockerhub or add to registry-service
  -proxy-cmd string
        Use specified command to run proxy. ie. './proxy --configfile conf.json $PROXY_PORT'. Note the automatically generated proxy config file will be named 'conf.json'.
  -proxy-version string
        Checkout specific version of proxy by supplying a commit hash. By default, will use latest version checked into service-manager master. This argument is supplied to git checkout <commit>, so a branch name or tags/<tag-name> works as well.
```

Config is a json file of this format:
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
}
```
NetworkSoftReq and NetworkHardReq are performance requirements passed to the proxy, used when the proxy selects microservices to connect to. DockerConf defines instructions for building the docker image for your microservice. They mostly translate to dockerfile directives. For examples, see registry-cli/add-test.

### Get command
```
get [<options>] <name>

<name>
        Name of microservice to get hash of
```

### List command
```
list
```

### Delete command
```
get [<options>] <name>

<name>
        Name of microservice to get hash of
```

## Registry Service

The service that stores information about microservices. Any service needs to be registered here before it can be deployed to the system. Stores info in {key, value} pairs, where key is service name, and value is a json encoded ServiceInfo string. Uses etcd key-value store under the hood.

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
