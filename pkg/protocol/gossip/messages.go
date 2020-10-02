package gossip

import (
	hiveproto "github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// contains the definition for gossip messages
	gossipMessageRegistry *hiveproto.Registry
)

func init() {
	definitions := []*hiveproto.Definition{
		tlv.HeaderMessageDefinition,
		// TODO: circumvent this weird constraint
		{ID: 1},
		{ID: 2},
		MilestoneRequestMessageDefinition,
		MessageMessageDefinition,
		MessageRequestMessageDefinition,
		HeartbeatMessageDefinition,
	}
	gossipMessageRegistry = hiveproto.NewRegistry(definitions)
}
