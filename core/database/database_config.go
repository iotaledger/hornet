package database

import (
	"github.com/gohornet/hornet/core/cli"
)

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
	// ignore the check for corrupted databases (should only be used for debug reasons)
	CfgDatabaseDebug = "db.debug"
)

func init() {
	cli.ConfigFlagSet.String(CfgDatabasePath, "mainnetdb", "the path to the database folder")
	cli.ConfigFlagSet.Bool(CfgDatabaseDebug, false, "ignore the check for corrupted databases (should only be used for debug reasons)")
}
