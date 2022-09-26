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
			Key:        "a507d2a592a5f0424ed8530603c08acebe088ae26211e90b79bfec0970a2397f",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "71a09774449a081450a51e0245a1e9850190f93508fd8f21bb9b9ca169765f30",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "a375515bfe5adf7fedb64ef4cebe1e621e85a056b0ccd5db72bc0d474325bf38",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "1df26178a7914126fd8cb934c7a7437073794c1c8ce99319172436b1d4973eba",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "45432d7c767e16586403262331a725c7eaa0b2dd79ea442f373c845ae3443aa9",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "9d87b4d2538b10799b582e25ace4726d92d7798ddfb696ff08e450db7917c9ad",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "a921841628d64c3f08bd344118b8106ade072e68c774beff30135e036194493a",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "16ee3356c21e410a0aaab42896021b1a857eb8d97a14a66fed9b13d634c21317",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "99c7d9752c295cb56b550191015ab5a40226fb632e8b02ec15cfe574ea17cf67",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "4af647910ba47000108b87c63abe0545643f9b203eacee2b713729b0450983fe",
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
