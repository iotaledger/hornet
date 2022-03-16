package protocfg

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
	iotago "github.com/iotaledger/iota.go/v3"
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
	// the vByte cost used for the dust protection
	CfgProtocolRentStructureVByteCost = "protocol.vByteCost"
	// the vByte factor used for data fields
	CfgProtocolRentStructureVByteFactorData = "protocol.vByteFactorData"
	// the vByte factor used for key fields
	CfgProtocolRentStructureVByteFactorKey = "protocol.vByteFactorKey"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgProtocolNetworkIDName, "chrysalis-mainnet", "the network ID on which this node operates on.")
			fs.String(CfgProtocolBech32HRP, string(iotago.PrefixMainnet), "the HRP which should be used for Bech32 addresses.")
			fs.Float64(CfgProtocolMinPoWScore, 4000, "the minimum PoW score required by the network.")
			fs.Int(CfgProtocolMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
			fs.Uint64(CfgProtocolRentStructureVByteCost, 500, "the vByte cost used for the dust protection")
			fs.Uint64(CfgProtocolRentStructureVByteFactorData, 1, "the vByte factor used for data fields")
			fs.Uint64(CfgProtocolRentStructureVByteFactorKey, 10, "the vByte factor used for key fields")
			return fs
		}(),
	},
	Masked: nil,
}
