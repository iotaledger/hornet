package protocfg

import (
	"github.com/iotaledger/hive.go/app"
)

const (
	// CfgProtocolPublicKeyRangesJSON defines the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"
)

type ConfigPublicKeyRange struct {
	Key        string `default:"0000000000000000000000000000000000000000000000000000000000000000" usage:"the ed25519 public key of the coordinator in hex representation" json:"key" koanf:"key"`
	StartIndex uint32 `default:"0" usage:"the start milestone index of the public key" json:"start" koanf:"start"`
	EndIndex   uint32 `default:"0" usage:"the end milestone index of the public key" json:"end" koanf:"end"`
}

type ConfigPublicKeyRanges []*ConfigPublicKeyRange

type BaseToken struct {
	// the base token name
	Name string `default:"IOTA" usage:"the base token name" json:"name"`
	// the base token ticker symbol
	TickerSymbol string `default:"MIOTA" usage:"the base token ticker symbol" json:"tickerSymbol"`
	// the base token unit
	Unit string `default:"i" usage:"the base token unit" json:"unit"`
	// the base token subunit
	Subunit string `default:"" usage:"the base token subunit" json:"subunit,omitempty"`
	// the base token amount of decimals
	Decimals uint32 `default:"0" usage:"the base token amount of decimals" json:"decimals"`
	// the base token uses the metric prefix
	UseMetricPrefix bool `default:"true" usage:"the base token uses the metric prefix" json:"useMetricPrefix"`
}

// SupportedProtocolVersions is a slice of supported protocol versions.
type SupportedProtocolVersions []uint32

// Highest returns the highest version.
func (versions SupportedProtocolVersions) Highest() uint32 {
	return versions[len(versions)-1]
}

// Lowest returns the lowest version.
func (versions SupportedProtocolVersions) Lowest() uint32 {
	return versions[0]
}

// ParametersProtocol contains the definition of the parameters used by protocol.
type ParametersProtocol struct {
	// The supported protocol versions by this node.
	SupportedProtocolVersions SupportedProtocolVersions `noflag:"true"`

	Parameters struct {
		// the protocol version this node supports
		Version byte `default:"2" usage:"the protocol version this node supports"`
		// the network ID on which this node operates on.
		NetworkName string `default:"chrysalis-mainnet" usage:"the network ID on which this node operates on"`
		// the HRP which should be used for Bech32 addresses.
		Bech32HRP string `default:"iota" usage:"the HRP which should be used for Bech32 addresses"`
		// the minimum PoW score required by the network.
		MinPoWScore uint32 `default:"4000" usage:"the minimum PoW score required by the network"`
		// BelowMaxDepth is the maximum allowed delta
		// value between OCRI of a given block in relation to the current CMI before it gets lazy.
		BelowMaxDepth uint8 `default:"15" usage:"the maximum allowed delta value for the OCRI of a given block in relation to the current CMI before it gets lazy"`
		// the vByte cost used for the storage deposit
		RentStructureVByteCost uint32 `name:"vByteCost" default:"500" usage:"the vByte cost used for the storage deposit"`
		// the vByte factor used for data fields
		RentStructureVByteFactorData uint8 `name:"vByteFactorData" default:"1" usage:"the vByte factor used for data fields"`
		// the vByte factor used for key fields
		RentStructureVByteFactorKey uint8 `name:"vByteFactorKey" default:"10" usage:"the vByte factor used for key fields"`
		// the token supply of the base token
		TokenSupply uint64 `default:"2779530283277761" usage:"the token supply of the native protocol token"`
	}

	// the amount of public keys in a milestone.
	MilestonePublicKeyCount int `default:"2" usage:"the amount of public keys in a milestone"`
	// the ed25519 public key of the coordinator in hex representation.
	PublicKeyRanges ConfigPublicKeyRanges `noflag:"true"`

	BaseToken BaseToken `usage:"the network base token properties"`
}

var ParamsProtocol = &ParametersProtocol{
	SupportedProtocolVersions: SupportedProtocolVersions{2}, // make sure to add the versions sorted asc
	PublicKeyRanges: ConfigPublicKeyRanges{
		{
			Key:        "a9b46fe743df783dedd00c954612428b34241f5913cf249d75bed3aafd65e4cd",
			StartIndex: 0,
			EndIndex:   777600,
		}, {
			Key:        "365fb85e7568b9b32f7359d6cbafa9814472ad0ecbad32d77beaf5dd9e84c6ba",
			StartIndex: 0,
			EndIndex:   1555200,
		}, {
			Key:        "ba6d07d1a1aea969e7e435f9f7d1b736ea9e0fcb8de400bf855dba7f2a57e947",
			StartIndex: 552960,
			EndIndex:   2108160,
		}, {
			Key:        "760d88e112c0fd210cf16a3dce3443ecf7e18c456c2fb9646cabb2e13e367569",
			StartIndex: 1333460,
			EndIndex:   2888660,
		}, {
			Key:        "7bac2209b576ea2235539358c7df8ca4d2f2fc35a663c760449e65eba9f8a6e7",
			StartIndex: 2108160,
			EndIndex:   3359999,
		}, {
			Key:        "edd9c639a719325e465346b84133bf94740b7d476dd87fc949c0e8df516f9954",
			StartIndex: 2888660,
			EndIndex:   3359999,
		}, {
			Key:        "47a5098c696e0fb53e6339edac574be4172cb4701a8210c2ae7469b536fd2c59",
			StartIndex: 3360000,
			EndIndex:   0,
		}, {
			Key:        "ae4e03072b4869e87dd4cd59315291a034493a8c202b43b257f9c07bc86a2f3e",
			StartIndex: 3360000,
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
