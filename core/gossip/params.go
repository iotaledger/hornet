package gossip

import (
	"time"

	"github.com/iotaledger/hive.go/core/app"
)

// ParametersRequests contains the definition of the parameters used by the requests.
type ParametersRequests struct {
	// Defines the maximum time a request stays in the request queue.
	DiscardOlderThan time.Duration `default:"15s" usage:"the maximum time a request stays in the request queue"`
	// Defines the interval the pending requests are re-enqueued.
	PendingReEnqueueInterval time.Duration `default:"5s" usage:"the interval the pending requests are re-enqueued"`
}

// ParametersGossip contains the definition of the parameters used by gossip.
type ParametersGossip struct {
	// Defines the maximum amount of unknown peers a gossip protocol connection is established to.
	UnknownPeersLimit int `default:"4" usage:"maximum amount of unknown peers a gossip protocol connection is established to"`
	// Defines the read timeout for reads from the gossip stream.
	StreamReadTimeout time.Duration `default:"60s" usage:"the read timeout for reads from the gossip stream"`
	// Defines the write timeout for writes to the gossip stream.
	StreamWriteTimeout time.Duration `default:"10s" usage:"the write timeout for writes to the gossip stream"`
}

var ParamsRequests = &ParametersRequests{}
var ParamsGossip = &ParametersGossip{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"requests":   ParamsRequests,
		"p2p.gossip": ParamsGossip,
	},
	Masked: nil,
}
