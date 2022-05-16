package profile

import (
	"github.com/iotaledger/hive.go/app"
)

const (
	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

// ParametersNode contains the definition of the parameters used by the node.
type ParametersNode struct {
	// Profile is the key to set the profile to use.
	Profile string `default:"auto" usage:"the profile the node runs with" shorthand:"p"`
}

var ParamsNode = &ParametersNode{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"node": ParamsNode,
	},
	Masked: nil,
}
