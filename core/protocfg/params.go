package protocfg

import (
	"github.com/iotaledger/hive.go/core/app"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// CfgProtocolPublicKeyRangesJSON defines the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"
)

type ConfigPublicKeyRange struct {
	Key        string                `default:"0000000000000000000000000000000000000000000000000000000000000000" usage:"the ed25519 public key of the coordinator in hex representation" json:"key" koanf:"key"`
	StartIndex iotago.MilestoneIndex `default:"0" usage:"the start milestone index of the public key" json:"start" koanf:"start"`
	EndIndex   iotago.MilestoneIndex `default:"0" usage:"the end milestone index of the public key" json:"end" koanf:"end"`
}

type ConfigPublicKeyRanges []*ConfigPublicKeyRange

type BaseToken struct {
	// the base token name
	Name string `default:"Shimmer" usage:"the base token name" json:"name"`
	// the base token ticker symbol
	TickerSymbol string `default:"SMR" usage:"the base token ticker symbol" json:"tickerSymbol"`
	// the base token unit
	Unit string `default:"SMR" usage:"the base token unit" json:"unit"`
	// the base token subunit
	Subunit string `default:"glow" usage:"the base token subunit" json:"subunit,omitempty"`
	// the base token amount of decimals
	Decimals uint32 `default:"6" usage:"the base token amount of decimals" json:"decimals"`
	// the base token uses the metric prefix
	UseMetricPrefix bool `default:"false" usage:"the base token uses the metric prefix" json:"useMetricPrefix"`
}

// ParametersProtocol contains the definition of the parameters used by protocol.
type ParametersProtocol struct {
	// the initial network name on which this node operates on.
	TargetNetworkName string `default:"shimmer" usage:"the initial network name on which this node operates on"`
	// the amount of public keys in a milestone.
	MilestonePublicKeyCount int `default:"7" usage:"the amount of public keys in a milestone"`
	// the ed25519 public key of the coordinator in hex representation.
	PublicKeyRanges ConfigPublicKeyRanges `noflag:"true"`

	BaseToken BaseToken `usage:"the network base token properties"`
}

var ParamsProtocol = &ParametersProtocol{
	PublicKeyRanges: ConfigPublicKeyRanges{
		{
			Key:        "dfd9436216ecc6e279e96b973aa9d2e11b73a74e425cce7adbac6156e70341c7",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "5b8c574430ad4ff9d2cfee62bed39fea44fdff87b45f475612e85fdb4f892564",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "89509278ee135465659f7ac08e9a3740762760dc1135bcd598b7a1776c7adbab",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "5cc55225809283d24ebab2cc03d70bcca98c36b5246e66d793f05f0429230c09",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "bdc89ffd26a02d0090635fab73392e42cd997e42bd3be7ee0bddec3c4623465f",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "0570042a5f93db2f6f8aae934d89bb8cd8d60d93c21923628ac873486e6a3ba6",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "a9a9ac376c3f997c594f142fdc66bce6920b22bcb48e190cb49ceed1574b77a6",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "693030491afa9b96e4bce49f629e4f6ad89584d82e64854f710042c28527cac4",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "aa33502799b6a932960374fa8ce42ce89b5d4bb055129580e1e0cfcefb49fb47",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "0d1a19fb7920f9d67457f2567dcb0d7590ffc8ee9a19cdc2bf91014e9c5db3d5",
			StartIndex: 0,
			EndIndex:   0,
		},
	},
}

var params = &app.ComponentParams{
	Params: map[string]any{
		"protocol": ParamsProtocol,
	},
	Masked: nil,
}
