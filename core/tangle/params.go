package tangle

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgTangleMilestoneTimeout is the interval milestone timeout events are fired if no new milestones are received.
	CfgTangleMilestoneTimeout = "tangle.milestoneTimeout"
	// CfgTangleMaxDeltaMsgYoungestConeRootIndexToCMI is the maximum allowed delta
	// value for the YCRI of a given message in relation to the current CMI before it gets lazy.
	CfgTangleMaxDeltaMsgYoungestConeRootIndexToCMI = "tangle.maxDeltaMsgYoungestConeRootIndexToCMI"
	// CfgTangleMaxDeltaMsgOldestConeRootIndexToCMI is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets semi-lazy.
	CfgTangleMaxDeltaMsgOldestConeRootIndexToCMI = "tangle.maxDeltaMsgOldestConeRootIndexToCMI"
	// CfgTangleWhiteFlagParentsSolidTimeout is the maximum duration for the parents to become solid during white flag confirmation API or INX call.
	CfgTangleWhiteFlagParentsSolidTimeout = "tangle.whiteFlagParentsSolidTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgTangleMilestoneTimeout, 30*time.Second, "the interval milestone timeout events are fired if no new milestones are received.")
			fs.Int(CfgTangleMaxDeltaMsgYoungestConeRootIndexToCMI, 8, "the maximum allowed delta "+
				"value for the YCRI of a given message in relation to the current CMI before it gets lazy")
			fs.Int(CfgTangleMaxDeltaMsgOldestConeRootIndexToCMI, 13, "the maximum allowed delta "+
				"value between OCRI of a given message in relation to the current CMI before it gets semi-lazy")
			fs.Duration(CfgTangleWhiteFlagParentsSolidTimeout, 2*time.Second, "defines the the maximum duration for the parents to become solid during white flag confirmation API or INX call")
			return fs
		}(),
	},
	Masked: nil,
}
