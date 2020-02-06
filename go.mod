module github.com/Multi-Tier-Cloud/hash-lookup

go 1.13

replace github.com/Multi-Tier-Cloud/hash-lookup/hashlookup => ./hashlookup

replace github.com/Multi-Tier-Cloud/hash-lookup/hl-common => ./hl-common

require (
	github.com/ipfs/go-cid v0.0.4
	github.com/libp2p/go-libp2p v0.5.1
	github.com/libp2p/go-libp2p-core v0.3.0
	github.com/libp2p/go-libp2p-discovery v0.2.0
	github.com/libp2p/go-libp2p-kad-dht v0.5.0
	github.com/multiformats/go-multiaddr v0.2.0
	github.com/multiformats/go-multihash v0.0.10
)
