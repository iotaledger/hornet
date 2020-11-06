package protocfg

import (
	"encoding/json"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/iotaledger/hive.go/configuration"
	flag "github.com/spf13/pflag"
	"go.uber.org/dig"
)

const (
	// the ed25519 public key of the coordinator in hex representation
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"
	// the ed25519 public key of the coordinator in hex representation
	CfgProtocolPublicKeyRanges = "protocol.publicKeyRanges"
	// the minimum PoW score required by the network
	CfgProtocolMinPoWScore = "protocol.minPoWScore"
	// the amount of public keys in a milestone
	CfgProtocolMilestonePublicKeyCount = "protocol.milestonePublicKeyCount"
	// the hash function to use to calculate milestone merkle tree hash (see RFC-0012)
	CfgProtocolMilestoneMerkleTreeHashFunc = "protocol.milestoneMerkleTreeHashFunc"
)

func init() {
	CorePlugin = &node.CorePlugin{
		Pluggable: node.Pluggable{
			Name: "ProtoCfg",
			Params: &node.PluginParams{
				Params: map[string]*flag.FlagSet{
					"nodeConfig": func() *flag.FlagSet {
						fs := flag.NewFlagSet("", flag.ContinueOnError)
						fs.Float64(CfgProtocolMinPoWScore, 4000, "the minimum PoW score required by the network.")
						fs.Int(CfgProtocolMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")
						fs.String(CfgProtocolMilestoneMerkleTreeHashFunc, "BLAKE2b-512", "the hash function the coordinator will use to calculate milestone merkle tree hash (see RFC-0012)")
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

var cooPubKeyRangesFlag = flag.String("publicKeyRanges", "", "overwrite public key ranges (JSON)")

func provide(c *dig.Container) {
	type tangledeps struct {
		dig.In
		NodeConfig *configuration.Configuration `name:"nodeConfig"`
	}

	if err := c.Provide(func(deps tangledeps) coordinator.PublicKeyRanges {
		r := coordinator.PublicKeyRanges{}

		if err := deps.NodeConfig.SetDefault(CfgProtocolPublicKeyRanges, &coordinator.PublicKeyRanges{
			&coordinator.PublicKeyRange{Key: "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
			&coordinator.PublicKeyRange{Key: "f1a319ff4e909c0ac9f2771d79feceed3c3bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 1, EndIndex: 1000},
			&coordinator.PublicKeyRange{Key: "ced3c3f1a319ff4e909f2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c", StartIndex: 800, EndIndex: 1000},
		}); err != nil {
			panic(err)
		}

		if *cooPubKeyRangesFlag != "" {
			// load from special CLI flag
			if err := json.Unmarshal([]byte(*cooPubKeyRangesFlag), &r); err != nil {
				panic(err)
			}
			return r
		}

		// load from config or default value
		if err := deps.NodeConfig.Unmarshal(CfgProtocolPublicKeyRanges, &r); err != nil {
			panic(err)
		}

		return r
	}); err != nil {
		panic(err)
	}
}
