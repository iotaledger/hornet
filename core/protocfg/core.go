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
						fs.String(CfgProtocolNetworkIDName, "mainnet1", "the network ID on which this node operates on.")
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
		MinPowScore     float64              `name:"minPowScore"`
	}

	if err := c.Provide(func(deps tangledeps) protoresult {

		res := protoresult{
			NetworkID:   iotago.NetworkIDFromString(deps.NodeConfig.String(CfgProtocolNetworkIDName)),
			Bech32HRP:   iotago.NetworkPrefix(deps.NodeConfig.String(CfgProtocolBech32HRP)),
			MinPowScore: deps.NodeConfig.Float64(CfgProtocolMinPoWScore),
		}

		// ToDo: Change these defaults to mainnet values
		if err := deps.NodeConfig.SetDefault(CfgProtocolPublicKeyRanges, &coordinator.PublicKeyRanges{
			&coordinator.PublicKeyRange{Key: "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
			&coordinator.PublicKeyRange{Key: "f1a319ff4e909c0ac9f2771d79feceed3c3bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
			&coordinator.PublicKeyRange{Key: "ced3c3f1a319ff4e909f2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 800, EndIndex: 1000},
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
