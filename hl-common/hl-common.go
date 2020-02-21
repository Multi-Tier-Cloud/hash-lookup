package common

import (
    "github.com/libp2p/go-libp2p-core/protocol"
)

type LookupResponse struct {
    ContentHash string
    DockerHash string
    LookupOk bool
}

type AddRequest struct {
    ServiceName string
    ContentHash string
    DockerHash string
}

var HashLookupRendezvousString string = "hash-lookup";

var LookupProtocolID protocol.ID = "/lookup/1.0";

var AddProtocolID protocol.ID = "/add/1.0";

var HttpLookupRoute string = "/lookup/"
