package pow

import (
	"time"

	"github.com/iotaledger/hive.go/app"
)

// ParametersPoW contains the definition of the parameters used by PoW.
type ParametersPoW struct {
	// Defines the interval for refreshing tips during PoW for spammer messages and messages passed without parents via API.
	RefreshTipsInterval time.Duration `default:"5s" usage:"interval for refreshing tips during PoW for spammer messages and messages passed without parents via API"`
}

var ParamsPoW = &ParametersPoW{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"pow": ParamsPoW,
	},
	Masked: nil,
}
