package referendum

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the path to the referendum database.
	CfgReferendumDatabasePath = "referendum.db.path"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgReferendumDatabasePath, "referendumStore", "the path to the referendum database")
			return fs
		}(),
	},
	Masked: nil,
}
