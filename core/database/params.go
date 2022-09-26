package database

import (
	"github.com/iotaledger/hive.go/core/app"
)

// ParametersDatabase contains the definition of the parameters used by the ParametersDatabase.
type ParametersDatabase struct {
	// Engine defines the used database engine (pebble/rocksdb/mapdb).
	Engine string `default:"rocksdb" usage:"the used database engine (pebble/rocksdb/mapdb)"`
	// Path defines the path to the database folder.
	Path string `default:"shimmer/database" usage:"the path to the database folder"`
	// AutoRevalidation defines whether to automatically start revalidation on startup if the database is corrupted.
	AutoRevalidation bool `default:"false" usage:"whether to automatically start revalidation on startup if the database is corrupted"`
	// Debug defines whether to ignore the check for corrupted databases (should only be used for debug reasons).
	Debug bool `default:"false" usage:"ignore the check for corrupted databases (should only be used for debug reasons)"`
	// CheckLedgerStateOnStartup defines whether to check if the ledger state matches the total supply on startup
	CheckLedgerStateOnStartup bool `default:"false" usage:"whether to check if the ledger state matches the total supply on startup"`
}

var ParamsDatabase = &ParametersDatabase{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"db": ParamsDatabase,
	},
	Masked: nil,
}
