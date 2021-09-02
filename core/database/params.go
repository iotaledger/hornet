package database

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/node"
)

const (
	// the used database engine (pebble/rocksdb).
	CfgDatabaseEngine = "db.engine"
	// the path to the database folder.
	CfgDatabasePath = "db.path"
	// whether to automatically start revalidation on startup if the database is corrupted.
	CfgDatabaseAutoRevalidation = "db.autoRevalidation"
	// ignore the check for corrupted databases (should only be used for debug reasons).
	CfgDatabaseDebug = "db.debug"
)

var params = &node.PluginParams{
	Params: map[string]*flag.FlagSet{
		"nodeConfig": func() *flag.FlagSet {
			fs := flag.NewFlagSet("", flag.ContinueOnError)
			fs.String(CfgDatabaseEngine, database.EngineRocksDB, "the used database engine (pebble/rocksdb)")
			fs.String(CfgDatabasePath, "mainnetdb", "the path to the database folder")
			fs.Bool(CfgDatabaseAutoRevalidation, false, "whether to automatically start revalidation on startup if the database is corrupted")
			fs.Bool(CfgDatabaseDebug, false, "ignore the check for corrupted databases (should only be used for debug reasons)")
			return fs
		}(),
	},
	Masked: nil,
}
