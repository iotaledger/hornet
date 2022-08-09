package inx

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersINX contains the definition of the parameters used by INX.
type ParametersINX struct {
	// Enabled defines whether the INX plugin is enabled.
	Enabled bool `default:"false" usage:"whether the INX plugin is enabled"`
	// the bind address on which the INX can be accessed from
	BindAddress string `default:"localhost:9029" usage:"the bind address on which the INX can be accessed from"`

	PoW struct {
		// the amount of workers used for calculating PoW when issuing blocks via INX
		WorkerCount int `default:"0" usage:"the amount of workers used for calculating PoW when issuing blocks via INX. (use 0 to use the maximum possible)"`
	} `name:"pow"`
}

var ParamsINX = &ParametersINX{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"inx": ParamsINX,
	},
	Masked: nil,
}
