package protocfg

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// the network ID on which this node operates on.
	CfgProtocolNetworkIDName = "protocol.networkID"
	// the HRP which should be used for Bech32 addresses.
	CfgProtocolBech32HRP = "protocol.bech32HRP"
	// the minimum PoW score required by the network.
	CfgProtocolMinPoWScore = "protocol.minPoWScore"
	// the amount of public keys in a milestone.
	CfgProtocolMilestonePublicKeyCount = "protocol.milestonePublicKeyCount"
	// the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRanges = "protocol.publicKeyRanges"
	// the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgProtocolNetworkIDName, "chrysalis-mainnet", "the network ID on which this node operates on.")
			fs.String(CfgProtocolBech32HRP, string(iotago.PrefixMainnet), "the HRP which should be used for Bech32 addresses.")
			fs.Float64(CfgProtocolMinPoWScore, 4000, "the minimum PoW score required by the network.")
			fs.Int(CfgProtocolMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
			return fs
		}(),
	},
	Masked: nil,
}
