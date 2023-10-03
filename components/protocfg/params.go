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
	Name string `default:"IOTA" usage:"the base token name" json:"name"`
	// the base token ticker symbol
	TickerSymbol string `default:"IOTA" usage:"the base token ticker symbol" json:"tickerSymbol"`
	// the base token unit
	Unit string `default:"IOTA" usage:"the base token unit" json:"unit"`
	// the base token subunit
	Subunit string `default:"micro" usage:"the base token subunit" json:"subunit,omitempty"`
	// the base token amount of decimals
	Decimals uint32 `default:"6" usage:"the base token amount of decimals" json:"decimals"`
	// the base token uses the metric prefix
	UseMetricPrefix bool `default:"false" usage:"the base token uses the metric prefix" json:"useMetricPrefix"`
}

// ParametersProtocol contains the definition of the parameters used by protocol.
type ParametersProtocol struct {
	// the initial network name on which this node operates on.
	TargetNetworkName string `default:"iota-mainnet" usage:"the initial network name on which this node operates on"`
	// the amount of public keys in a milestone.
	MilestonePublicKeyCount int `default:"7" usage:"the amount of public keys in a milestone"`
	// the ed25519 public key of the coordinator in hex representation.
	PublicKeyRanges ConfigPublicKeyRanges `noflag:"true"`

	BaseToken BaseToken `usage:"the network base token properties"`
}

var ParamsProtocol = &ParametersProtocol{
	PublicKeyRanges: ConfigPublicKeyRanges{
		{
			Key:        "2fb1d7ec714adf365eefa343b66c0c459a9930276aff08cde482cb8050028624",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "8845cd560d66d50070c6e251d7a0a19f8de217fabf53a78ee15b41d85a489cc6",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "1d61aab6f7e52129b78fcdf9568def0baa9c71112964f5b4d86ffc406866a986",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "fa94be504dfb10876a449db5272f19393ded922cbe3b023b4e57b62a53835721",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "5fadfabe6944f5f0166ada11452c642010339f916e28187ecf8b4a207c8dba47",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "347e6892d72b71e0423bd14daaf61d2ac35e91852fa5b155b92ddda0e064f55f",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "0e403f526a66b4c0b18e8b0257671b07892a419e4b6e4540707d9a4793d1e3be",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "3af73a609696ff6fe63c36d060455cd83ec23edea2d2b87d5317004849cc0e9a",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "ee1bfa9e791a9f57ea72c6192b000d906f21479ba8f40bb20cdd8badb7ddcb78",
			StartIndex: 0,
			EndIndex:   0,
		}, {
			Key:        "083d7af99250a06d086b07bdd5bccd2bff406ee17e19332ccdb08d8be72218ce",
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
