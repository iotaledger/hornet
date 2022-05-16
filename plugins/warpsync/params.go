package warpsync

import (
	"github.com/iotaledger/hive.go/app"
)

// ParametersWarpSync contains the definition of the parameters used by WarpSync.
type ParametersWarpSync struct {
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
