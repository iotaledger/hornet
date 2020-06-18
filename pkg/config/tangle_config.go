package config

import (
	flag "github.com/spf13/pflag"
)

const (
	// the path to the database folder
	CfgDatabasePath = "db.path"
	// ignore the check for corrupted databases (should only be used for debug reasons)
	CfgDatabaseDebug = "db.debug"
)

func init() {
	flag.String(CfgDatabasePath, "mainnetdb", "the path to the database folder")
	flag.Bool(CfgDatabaseDebug, false, "ignore the check for corrupted databases (should only be used for debug reasons)")
}
