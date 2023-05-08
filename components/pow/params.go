package pow

import (
	"time"

	"github.com/iotaledger/hive.go/app"
)

// ParametersPoW contains the definition of the parameters used by PoW.
type ParametersPoW struct {
	// RefreshTipsInterval defines the interval for refreshing tips during PoW for blocks passed without parents via API.
	RefreshTipsInterval time.Duration `default:"5s" usage:"interval for refreshing tips during PoW for blocks passed without parents via API"`

	// RemotePoWHost defines the host of a remote PoW provider.
	RemotePoWHost string `default:"localhost:19180" usage:"host of a remote PoW provider"`
}

var ParamsPoW = &ParametersPoW{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"pow": ParamsPoW,
	},
	Masked: nil,
}
