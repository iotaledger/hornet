package warpsync

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersWarpSync contains the definition of the parameters used by WarpSync.
type ParametersWarpSync struct {
	// Enabled defines whether the warpsync plugin is enabled.
	Enabled bool `default:"true" usage:"whether the warpsync plugin is enabled"`
	// Defines the used advancement range per warpsync checkpoint
	AdvancementRange int `default:"150" usage:"the used advancement range per warpsync checkpoint"`
}

var ParamsWarpSync = &ParametersWarpSync{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"warpsync": ParamsWarpSync,
	},
	Masked: nil,
}
