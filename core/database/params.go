package database

import (
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/iotaledger/hive.go/app"
)

const (
	// the used database engine (pebble/rocksdb/mapdb).
	CfgDatabaseEngine = "db.engine"
	// the path to the database folder.
	CfgDatabasePath = "db.path"
	// whether to automatically start revalidation on startup if the database is corrupted.
	CfgDatabaseAutoRevalidation = "db.autoRevalidation"
	// ignore the check for corrupted databases (should only be used for debug reasons).
	CfgDatabaseDebug = "db.debug"
)

var params = &app.ComponentParams{
	Params: func(fs *flag.FlagSet) {
		fs.String(CfgDatabaseEngine, string(database.EngineRocksDB), "the used database engine (pebble/rocksdb/mapdb)")
		fs.String(CfgDatabasePath, "mainnetdb", "the path to the database folder")
		fs.Bool(CfgDatabaseAutoRevalidation, false, "whether to automatically start revalidation on startup if the database is corrupted")
		fs.Bool(CfgDatabaseDebug, false, "ignore the check for corrupted databases (should only be used for debug reasons)")
	},
	Masked: nil,
}
