module github.com/Multi-Tier-Cloud/hash-lookup

go 1.13

// Required while etcd doesn't support newer version of grpc
require google.golang.org/grpc v1.26.0

require (
	github.com/Multi-Tier-Cloud/common v0.8.1
	github.com/Multi-Tier-Cloud/docker-driver v0.0.0-20200323084307-72982cb10a89
	github.com/Multi-Tier-Cloud/service-manager v0.2.1
	github.com/containerd/containerd v1.3.4 // indirect
	github.com/coreos/etcd v3.3.22+incompatible // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/docker/docker v17.12.0-ce-rc1.0.20200514230353-811a247d06e8+incompatible // indirect
	github.com/envoyproxy/go-control-plane v0.9.4 // indirect
	github.com/golang/protobuf v1.3.3 // indirect
	github.com/libp2p/go-libp2p v0.9.2
	github.com/libp2p/go-libp2p-core v0.5.6
	github.com/libp2p/go-libp2p-discovery v0.4.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.11
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/prometheus/client_golang v1.5.1
	github.com/sirupsen/logrus v1.6.0 // indirect
	//go.etcd.io/etcd v0.5.0-alpha.5.0.20200212203316-09304a4d8263
	go.etcd.io/etcd v3.3.22+incompatible
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
	golang.org/x/net v0.0.0-20200528225125-3c3fba18258b // indirect
)
