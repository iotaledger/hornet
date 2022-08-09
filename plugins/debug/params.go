package debug

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersDebug contains the definition of the parameters used by the debug plugin.
type ParametersDebug struct {
	// Enabled defines whether the debug plugin is enabled.
	Enabled bool `default:"false" usage:"whether the debug plugin is enabled"`
}

var ParamsDebug = &ParametersDebug{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"debug": ParamsDebug,
	},
	Masked: nil,
}
