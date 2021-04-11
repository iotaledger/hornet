package tangle

import (
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// CfgTangleMilestoneTimeout is the interval milestone timeout events are fired if no new milestones are received.
	CfgTangleMilestoneTimeout = "tangle.milestoneTimeout"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.Duration(CfgTangleMilestoneTimeout, 30*time.Second, "the interval milestone timeout events are fired if no new milestones are received.")
			return fs
		}(),
	},
	Masked: nil,
}
