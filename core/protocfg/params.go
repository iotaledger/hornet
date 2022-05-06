package protocfg

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/node"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// the protocol version this node supports
	CfgProtocolParametersVersion = "protocol.parameters.version"
	// the network ID on which this node operates on.
	CfgProtocolParametersNetworkName = "protocol.parameters.networkName"
	// the HRP which should be used for Bech32 addresses.
	CfgProtocolParametersBech32HRP = "protocol.parameters.bech32HRP"
	// the minimum PoW score required by the network.
	CfgProtocolParametersMinPoWScore = "protocol.parameters.minPoWScore"
	// CfgProtocolParametersBelowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	CfgProtocolParametersBelowMaxDepth = "protocol.parameters.belowMaxDepth"
	// the vByte cost used for the dust protection
	CfgProtocolParametersRentStructureVByteCost = "protocol.parameters.vByteCost"
	// the vByte factor used for data fields
	CfgProtocolParametersRentStructureVByteFactorData = "protocol.parameters.vByteFactorData"
	// the vByte factor used for key fields
	CfgProtocolParametersRentStructureVByteFactorKey = "protocol.parameters.vByteFactorKey"
	// the token supply of the base token
	CfgProtocolParametersTokenSupply = "protocol.parameters.tokenSupply"

	// the amount of public keys in a milestone.
	CfgProtocolMilestonePublicKeyCount = "protocol.milestonePublicKeyCount"
	// the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRanges = "protocol.publicKeyRanges"
	// the ed25519 public key of the coordinator in hex representation.
	CfgProtocolPublicKeyRangesJSON = "publicKeyRanges"

	// the base token name
	CfgProtocolBaseTokenName = "protocol.baseToken.name"
	// the base token ticker symbol
	CfgProtocolBaseTokenTickerSymbol = "protocol.baseToken.tickerSymbol"
	// the base token unit
	CfgProtocolBaseTokenUnit = "protocol.baseToken.unit"
	// the base token subunit
	CfgProtocolBaseTokenSubunit = "protocol.baseToken.subunit"
	// the base token amount of decimals
	CfgProtocolBaseTokenDecimals = "protocol.baseToken.decimals"
	// the base token uses the metric prefix
	CfgProtocolBaseTokenUseMetricPrefix = "protocol.baseToken.useMetricPrefix"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Uint8(CfgProtocolParametersVersion, 2, "the protocol version this node supports")
			fs.String(CfgProtocolParametersNetworkName, "chrysalis-mainnet", "the network ID on which this node operates on.")
			fs.String(CfgProtocolParametersBech32HRP, string(iotago.PrefixMainnet), "the HRP which should be used for Bech32 addresses.")
			fs.Float64(CfgProtocolParametersMinPoWScore, 4000, "the minimum PoW score required by the network.")
			fs.Uint16(CfgProtocolParametersBelowMaxDepth, 15, "the maximum allowed delta value for the OCRI of a given message in relation to the current CMI before it gets lazy")
			fs.Uint64(CfgProtocolParametersRentStructureVByteCost, 500, "the vByte cost used for the dust protection")
			fs.Uint64(CfgProtocolParametersRentStructureVByteFactorData, 1, "the vByte factor used for data fields")
			fs.Uint64(CfgProtocolParametersRentStructureVByteFactorKey, 10, "the vByte factor used for key fields")
			fs.Uint64(CfgProtocolParametersTokenSupply, 2_779_530_283_277_761, "the token supply of the native protocol token")

			fs.Int(CfgProtocolMilestonePublicKeyCount, 2, "the amount of public keys in a milestone")

			fs.String(CfgProtocolBaseTokenName, "IOTA", "the base token name")
			fs.String(CfgProtocolBaseTokenTickerSymbol, "MIOTA", "the base token ticker symbol")
			fs.String(CfgProtocolBaseTokenUnit, "IOTA", "the base token unit")
			fs.String(CfgProtocolBaseTokenSubunit, "", "the base token subunit")
			fs.Uint32(CfgProtocolBaseTokenDecimals, 0, "the base token amount of decimals")
			fs.Bool(CfgProtocolBaseTokenUseMetricPrefix, true, "the base token uses the metric prefix")
			return fs
		}(),
	},
	Masked: nil,
}

type ConfigPublicKeyRange struct {
	Key        string          `json:"key" koanf:"key"`
	StartIndex milestone.Index `json:"start" koanf:"start"`
	EndIndex   milestone.Index `json:"end" koanf:"end"`
}

type ConfigPublicKeyRanges []*ConfigPublicKeyRange

type BaseToken struct {
	Name            string `json:"name"`
	TickerSymbol    string `json:"tickerSymbol"`
	Unit            string `json:"unit"`
	Subunit         string `json:"subunit,omitempty"`
	Decimals        uint32 `json:"decimals"`
	UseMetricPrefix bool   `json:"useMetricPrefix"`
}
