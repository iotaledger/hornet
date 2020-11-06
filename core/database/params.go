package database

import (
	"github.com/gohornet/hornet/pkg/node"
	flag "github.com/spf13/pflag"
)

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
	// ignore the check for corrupted databases (should only be used for debug reasons)
	CfgDatabaseDebug = "db.debug"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgDatabasePath, "mainnetdb", "the path to the database folder")
			fs.Bool(CfgDatabaseDebug, false, "ignore the check for corrupted databases (should only be used for debug reasons)")
			return fs
		}(),
	},
	Hide: nil,
}
