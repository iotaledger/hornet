package profile

import (
	"github.com/iotaledger/hive.go/core/app"
)

const (
	// AutoProfileName is the name of the automatic profile.
	AutoProfileName = "auto"
)

// ParametersNode contains the definition of the parameters used by the node.
type ParametersNode struct {
	// Profile is the key to set the profile to use.
	Profile string `default:"auto" usage:"the profile the node runs with" shorthand:"p"`
	// Alias is used to set an alias to identify a node
	Alias string `default:"HORNET node" usage:"set an alias to identify a node"`
}

var ParamsNode = &ParametersNode{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"node": ParamsNode,
	},
	Masked: nil,
}
