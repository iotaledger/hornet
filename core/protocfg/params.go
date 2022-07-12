package protocfg

import (
	"github.com/iotaledger/hive.go/app"
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
	TargetNetworkName string `default:"alphanet-8" usage:"the initial network name on which this node operates on"`
	// the amount of public keys in a milestone.
	MilestonePublicKeyCount int `default:"3" usage:"the amount of public keys in a milestone"`
	// the ed25519 public key of the coordinator in hex representation.
	PublicKeyRanges ConfigPublicKeyRanges `noflag:"true"`

	BaseToken BaseToken `usage:"the network base token properties"`
}

var ParamsProtocol = &ParametersProtocol{
	PublicKeyRanges: ConfigPublicKeyRanges{
		{
			Key:        "d9922819a39e94ddf3907f4b9c8df93f39f026244fcb609205b9a879022599f2",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "d85e5b1590d898d1e0cdebb2e3b5337c8b76270142663d78811683ba47c17c98",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "f9d9656a60049083eef61487632187b351294c1fa23d118060d813db6d03e8b6",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "3df80964cc83921e08c1fa0a4f5fc05810a634da45461b2b315fcbfd62f7cab7",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "8e222ae7e2adcfb87a2984a19aad52b1979ed1472c3cb17239a73ef1d344c35a",
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
