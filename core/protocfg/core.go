package protocfg

import (
	"encoding/json"

	flag "github.com/spf13/pflag"
	"go.uber.org/dig"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v2"
)

const (
	// the ed25519 public key of the coordinator in hex representation
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"
	// the ed25519 public key of the coordinator in hex representation
	CfgProtocolPublicKeyRanges = "protocol.publicKeyRanges"
	// the minimum PoW score required by the network
	CfgProtocolMinPoWScore = "protocol.minPoWScore"
	// the network ID on which this node operates on.
	CfgProtocolNetworkIDName = "protocol.networkID"
	// the HRP which should be used for Bech32 addresses.
	CfgProtocolBech32HRP = "protocol.bech32HRP"
	// the amount of public keys in a milestone
	CfgProtocolMilestonePublicKeyCount = "protocol.milestonePublicKeyCount"
)

func init() {
	flag.CommandLine.MarkHidden(CfgProtocolPublicKeyRangesJSON)

	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name: "ProtoCfg",
			Params: &node.PluginParams{
				Params: map[string]*flag.FlagSet{
					"nodeConfig": func() *flag.FlagSet {
						fs := flag.NewFlagSet("", flag.ContinueOnError)
						fs.Float64(CfgProtocolMinPoWScore, 4000, "the minimum PoW score required by the network.")
						fs.Int(CfgProtocolMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
						fs.String(CfgProtocolNetworkIDName, "c2-mainnet", "the network ID on which this node operates on.")
						fs.String(CfgProtocolBech32HRP, string(iotago.PrefixMainnet), "the HRP which should be used for Bech32 addresses.")
						return fs
					}(),
				},
				Masked: nil,
			},
			Provide: provide,
		},
	}
}

var (
	CorePlugin *node.CorePlugin
)

var cooPubKeyRangesFlag = flag.String(CfgProtocolPublicKeyRangesJSON, "", "overwrite public key ranges (JSON)")

func provide(c *dig.Container) {
	type tangledeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	type protoresult struct {
		dig.Out

		PublicKeyRanges coordinator.PublicKeyRanges
		NetworkID       uint64               `name:"networkId"`
		Bech32HRP       iotago.NetworkPrefix `name:"bech32HRP"`
		MinPoWScore     float64              `name:"minPoWScore"`
	}

	if err := c.Provide(func(deps tangledeps) protoresult {

		res := protoresult{
			NetworkID:   iotago.NetworkIDFromString(deps.NodeConfig.String(CfgProtocolNetworkIDName)),
			Bech32HRP:   iotago.NetworkPrefix(deps.NodeConfig.String(CfgProtocolBech32HRP)),
			MinPoWScore: deps.NodeConfig.Float64(CfgProtocolMinPoWScore),
		}

		if err := deps.NodeConfig.SetDefault(CfgProtocolPublicKeyRanges, &coordinator.PublicKeyRanges{
			&coordinator.PublicKeyRange{Key: "a9b46fe743df783dedd00c954612428b34241f5913cf249d75bed3aafd65e4cd", StartIndex: 0, EndIndex: 777600},
			&coordinator.PublicKeyRange{Key: "365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba", StartIndex: 0, EndIndex: 1555200},
		}); err != nil {
			panic(err)
		}

		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &res.PublicKeyRanges); err != nil {
				panic(err)
			}
			return res
		}

		// load from config or default value
		if err := deps.NodeConfig.Unmarshal(CfgProtocolPublicKeyRanges, &res.PublicKeyRanges); err != nil {
			panic(err)
		}

		return res
	}); err != nil {
		panic(err)
	}
}
