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
	TargetNetworkName string `default:"testnet" usage:"the initial network name on which this node operates on"`
	// the amount of public keys in a milestone.
	MilestonePublicKeyCount int `default:"7" usage:"the amount of public keys in a milestone"`
	// the ed25519 public key of the coordinator in hex representation.
	PublicKeyRanges ConfigPublicKeyRanges `noflag:"true"`

	BaseToken BaseToken `usage:"the network base token properties"`
}

var ParamsProtocol = &ParametersProtocol{
	PublicKeyRanges: ConfigPublicKeyRanges{
		{
			Key:        "13ccdc2f5d3d9a3ebe06074c6b49b49090dd79ca72e04abf20f10f871ad8293b",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "f18f3f6a2d940b9bacd3084713f6877db22064ada4335cb53ae1da75044f978d",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "b3b4c920909720ba5f7c30dddc0f9169bf8243b529b601fc4776b8cb0a8ca253",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "bded01e93adf7a623118fd375fd93dc7d7ddf222324239cae33e4e4c47ec3b0e",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "488ac3fb1b8df5ef8c4acb4ef1f3e3d039c5d7197db87094a61af66320722313",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "61f95fed30b6e9bf0b2d03938f56d35789ff7f0ea122d01c5c1b7e869525e218",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "4587040de05907b70806c8725bdae1f7370785993b2a139208e247885d4ed1f8",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "aa6b36116206cc7d6c8f688e22113aa46f0de88d51aa7acf881ec2bd9d015f62",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "ede9760c7f2aaa4618a58a1357705cdc1874962ad369309543230394bb77548b",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "98d1f907caa99f9320f0e0eb64a5cf208751c2171c7938da5659328061e82a8e",
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
