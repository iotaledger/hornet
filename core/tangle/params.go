package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/core/app"
)

// ParametersTangle contains the definition of the parameters used by tangle.
type ParametersTangle struct {
	// MilestoneTimeout is the interval milestone timeout events are fired if no new milestones are received.
	MilestoneTimeout time.Duration `default:"30s" usage:"the interval milestone timeout events are fired if no new milestones are received"`
	// MaxDeltaBlockYoungestConeRootIndexToCMI is the maximum allowed delta
	// value for the YCRI of a given block in relation to the current CMI before it gets lazy.
	MaxDeltaBlockYoungestConeRootIndexToCMI int `default:"8" usage:"the maximum allowed delta value for the YCRI of a given block in relation to the current CMI before it gets lazy"`
	// MaxDeltaBlockOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets semi-lazy.
	MaxDeltaBlockOldestConeRootIndexToCMI int `default:"13" usage:"the maximum allowed delta value between OCRI of a given block in relation to the current CMI before it gets semi-lazy"`
	// WhiteFlagParentsSolidTimeout is the maximum duration for the parents to become solid during white flag confirmation API or INX call.
	WhiteFlagParentsSolidTimeout time.Duration `default:"2s" usage:"defines the the maximum duration for the parents to become solid during white flag confirmation API or INX call"`
}

var ParamsTangle = &ParametersTangle{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"tangle": ParamsTangle,
	},
	Masked: nil,
}
