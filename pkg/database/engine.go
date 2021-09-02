package database

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gohornet/hornet/pkg/utils"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"
)

type databaseInfo struct {
	Engine string `toml:"databaseEngine"`
}

// DatabaseEngine parses a string and returns an engine.
// Returns an error if the engine is unknown.
func DatabaseEngine(engine string) (Engine, error) {

	switch engine {
	case EngineRocksDB:
	case EnginePebble:
	default:
		return "", fmt.Errorf("unknown database engine: %s, supported engines: pebble/rocksdb", engine)
	}

	return Engine(engine), nil
}

// CheckDatabaseEngine checks if the correct database engine is used.
// This function stores a so called "database info file" in the database folder or
// checks if an existing "database info file" contains the correct engine.
// Otherwise the files in the database folder are not compatible.
func CheckDatabaseEngine(dbPath string, createDatabaseIfNotExists bool, dbEngine ...Engine) (Engine, error) {

	if createDatabaseIfNotExists && len(dbEngine) == 0 {
		return EngineRocksDB, errors.New("the database engine must be specified if the database should be newly created")
	}

	// check if the database exists and if it should be created
	_, err := os.Stat(dbPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return EngineUnknown, fmt.Errorf("unable to check database path (%s): %w", dbPath, err)
		}

		if !createDatabaseIfNotExists {
			return EngineUnknown, fmt.Errorf("database not found (%s)", dbPath)
		}

		// database will be newly created
	}

	var targetEngine Engine

	// check if the database info file exists and if it should be created
	dbInfoFilePath := filepath.Join(dbPath, "dbinfo")
	_, err = os.Stat(dbInfoFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return EngineUnknown, fmt.Errorf("unable to check database info file (%s): %w", dbInfoFilePath, err)
		}

		if len(dbEngine) == 0 {
			return EngineUnknown, fmt.Errorf("database info file not found (%s)", dbInfoFilePath)
		}

		// if the dbInfo file does not exist and the dbEngine is given, create the dbInfo file.
		if err := storeDatabaseInfoToFile(dbInfoFilePath, dbEngine[0]); err != nil {
			return EngineUnknown, err
		}

		targetEngine = dbEngine[0]
	} else {
		dbEngineFromInfoFile, err := LoadDatabaseEngineFromFile(dbInfoFilePath)
		if err != nil {
			return EngineUnknown, err
		}

		// if the dbInfo file exists and the dbEngine is given, compare the engines.
		if len(dbEngine) > 0 {

			if dbEngineFromInfoFile != dbEngine[0] {
				return EngineUnknown, fmt.Errorf(`database engine does not match the configuration: '%v' != '%v'

If you want to use another database engine, you can use the tool './hornet tool db-migration' to convert the current database.`, dbEngineFromInfoFile, dbEngine[0])
			}
		}

		targetEngine = dbEngineFromInfoFile
	}

	return targetEngine, nil
}

// LoadDatabaseEngineFromFile returns the engine from the "database info file".
func LoadDatabaseEngineFromFile(path string) (Engine, error) {

	var info databaseInfo

	if err := utils.ReadTOMLFromFile(path, &info); err != nil {
		return "", fmt.Errorf("unable to read database info file: %w", err)
	}

	return DatabaseEngine(info.Engine)
}

// storeDatabaseInfoToFile stores the used engine in a "database info file".
func storeDatabaseInfoToFile(filePath string, engine Engine) error {
	dirPath := filepath.Dir(filePath)

	if err := os.MkdirAll(dirPath, 0700); err != nil {
		return fmt.Errorf("could not create database dir '%s': %w", dirPath, err)
	}

	info := &databaseInfo{
		Engine: string(engine),
	}

	return utils.WriteTOMLToFile(filePath, info, 0660, "# auto-generated\n# !!! do not modify this file !!!")
}

// StoreWithDefaultSettings returns a kvstore with default settings.
// It also checks if the database engine is correct.
func StoreWithDefaultSettings(path string, createDatabaseIfNotExists bool, dbEngine ...Engine) (kvstore.KVStore, error) {

	targetEngine, err := CheckDatabaseEngine(path, createDatabaseIfNotExists, dbEngine...)
	if err != nil {
		return nil, err
	}

	switch targetEngine {
	case EnginePebble:
		db, err := NewPebbleDB(path, nil, false)
		if err != nil {
			return nil, err
		}
		return pebble.New(db), nil

	case EngineRocksDB:
		db, err := NewRocksDB(path)
		if err != nil {
			return nil, err
		}
		return rocksdb.New(db), nil

	default:
		return nil, fmt.Errorf("unknown database engine: %s, supported engines: pebble/rocksdb", dbEngine)
	}
}
