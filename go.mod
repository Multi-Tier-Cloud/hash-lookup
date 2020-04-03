module github.com/Multi-Tier-Cloud/hash-lookup

go 1.13

replace github.com/Multi-Tier-Cloud/hash-lookup/hashlookup => ./hashlookup

replace github.com/Multi-Tier-Cloud/hash-lookup/hl-common => ./hl-common

require (
	github.com/Multi-Tier-Cloud/common v0.0.0-20200403032527-d7311ad845b4
	github.com/ipfs/go-blockservice v0.1.2
	github.com/ipfs/go-ipfs v0.4.22-0.20200219161038-21f6e19f2f37
	github.com/ipfs/go-ipfs-files v0.0.6
	github.com/ipfs/go-merkledag v0.3.1
	github.com/ipfs/go-mfs v0.1.1
	github.com/ipfs/go-unixfs v0.2.4
	github.com/libp2p/go-libp2p v0.5.2
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-discovery v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.5.0
	github.com/multiformats/go-multiaddr v0.2.0
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200212203316-09304a4d8263
	golang.org/x/crypto v0.0.0-20200115085410-6d4e4cb37c7d // indirect
	golang.org/x/net v0.0.0-20191002035440-2ec189313ef0 // indirect
	gopkg.in/yaml.v2 v2.2.8 // indirect
)
