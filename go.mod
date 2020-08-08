module github.com/PhysarumSM/service-registry

go 1.13

// Required while etcd doesn't support newer version of grpc
require google.golang.org/grpc v1.26.0

require (
	github.com/PhysarumSM/common v0.9.0
	github.com/PhysarumSM/docker-driver v0.2.4
	github.com/PhysarumSM/service-manager v0.2.1
	github.com/coreos/etcd v3.3.22+incompatible // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/envoyproxy/go-control-plane v0.9.4 // indirect
	github.com/golang/protobuf v1.3.3 // indirect
	github.com/libp2p/go-libp2p v0.9.2
	github.com/libp2p/go-libp2p-core v0.5.6
	github.com/libp2p/go-libp2p-discovery v0.4.0
	github.com/libp2p/go-libp2p-kad-dht v0.7.11
	github.com/multiformats/go-multiaddr v0.2.2
	github.com/prometheus/client_golang v1.5.1
	//go.etcd.io/etcd v0.5.0-alpha.5.0.20200212203316-09304a4d8263
	go.etcd.io/etcd v3.3.22+incompatible
	golang.org/x/crypto v0.0.0-20200510223506-06a226fb4e37
)
