package database

import (
	"github.com/iotaledger/hive.go/app"
)

// ParametersDatabase contains the definition of the parameters used by the ParametersDatabase.
type ParametersDatabase struct {
	// the used database engine (pebble/rocksdb/mapdb).
	Engine string `default:"rocksdb" usage:"the used database engine (pebble/rocksdb/mapdb)"`
	// the path to the database folder.
	Path string `default:"mainnetdb" usage:"the path to the database folder"`
	// whether to automatically start revalidation on startup if the database is corrupted.
	AutoRevalidation bool `default:"false" usage:"whether to automatically start revalidation on startup if the database is corrupted"`
	// ignore the check for corrupted databases (should only be used for debug reasons).
	Debug bool `default:"false" usage:"ignore the check for corrupted databases (should only be used for debug reasons)"`
}

var ParamsDatabase = &ParametersDatabase{}

var params = &app.ComponentParams{
	Params: map[string]any{
		"db": ParamsDatabase,
	},
	Masked: nil,
}
