package database

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/utils"
)

const (
	storePrefixHealthDeprecated byte = 0
	storePrefixUTXODeprecated   byte = 8
)

func NeedsSplitting(dbPath string) (bool, error) {

	// check if the database exists
	dbExists, err := database.DatabaseExists(dbPath)
	if err != nil {
		return false, err
	}
	if !dbExists {
		// there is no database, so no need to even split
		return false, nil
	}

	tangleDatabasePath := filepath.Join(dbPath, TangleDatabaseDirectoryName)
	utxoDatabasePath := filepath.Join(dbPath, UTXODatabaseDirectoryName)

	tangleExists, err := database.DatabaseExists(tangleDatabasePath)
	if err != nil {
		return false, err
	}

	utxoExists, err := database.DatabaseExists(utxoDatabasePath)
	if err != nil {
		return false, err
	}

	return !tangleExists || !utxoExists, nil
}

func SplitIntoTangleAndUTXO(databasePath string, dbEngine ...database.Engine) error {

	needsSplitting, err := NeedsSplitting(databasePath)
	if err != nil {
		return err
	}
	if !needsSplitting {
		return nil
	}

	legacyDatabasePath := databasePath
	tangleDatabasePath := filepath.Join(legacyDatabasePath, TangleDatabaseDirectoryName)
	utxoDatabasePath := filepath.Join(legacyDatabasePath, UTXODatabaseDirectoryName)

	// Read the engine the current database is using
	dbEngineLegacy, err := database.CheckDatabaseEngine(legacyDatabasePath, false, dbEngine...)
	if err != nil {
		return err
	}

	if err := utils.CreateDirectory(tangleDatabasePath, 0700); err != nil {
		return err
	}

	if err := utils.CreateDirectory(utxoDatabasePath, 0700); err != nil {
		return err
	}

	// Move the legacy database into the tangle directory
	files, err := ioutil.ReadDir(legacyDatabasePath)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.IsDir() && (f.Name() == TangleDatabaseDirectoryName || f.Name() == UTXODatabaseDirectoryName) {
			continue
		}
		os.Rename(filepath.Join(legacyDatabasePath, f.Name()), filepath.Join(tangleDatabasePath, f.Name()))
	}

	tangleStore, err := database.StoreWithDefaultSettings(tangleDatabasePath, false, dbEngineLegacy)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", TangleDatabaseDirectoryName, err)
	}
	defer func() { _ = tangleStore.Close() }()

	utxoStore, err := database.StoreWithDefaultSettings(utxoDatabasePath, true, dbEngineLegacy)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", UTXODatabaseDirectoryName, err)
	}
	defer func() { _ = utxoStore.Close() }()

	fmt.Printf("Splitting database using %s...\n", dbEngineLegacy)

	// Migrate the UTXO data by removing the old 8 prefix
	legacyStorePrefixUTXO := kvstore.KeyPrefix{storePrefixUTXODeprecated}
	if err := databaseMigrateKeys(tangleStore, utxoStore, legacyStorePrefixUTXO, kvstore.EmptyPrefix); err != nil {
		return fmt.Errorf("error migrating data to utxo database: %w", err)
	}

	// Remove UTXO data from tangle database
	if err := tangleStore.DeletePrefix(legacyStorePrefixUTXO); err != nil {
		return fmt.Errorf("error deleting utxo data from tangle database: %w", err)
	}

	// Copy the DB health data to UTXO by replacing the old 0 prefix with the new 255 prefix
	if err := databaseMigrateKeys(tangleStore, utxoStore, kvstore.KeyPrefix{storePrefixHealthDeprecated}, kvstore.KeyPrefix{common.StorePrefixHealth}); err != nil {
		return fmt.Errorf("error copying health data to utxo database: %w", err)
	}

	// Migrate the DB health data in the tangle database to the new keys
	if err := databaseMigrateKeys(tangleStore, tangleStore, kvstore.KeyPrefix{storePrefixHealthDeprecated}, kvstore.KeyPrefix{common.StorePrefixHealth}); err != nil {
		return fmt.Errorf("error migrating tangle health database: %w", err)
	}

	// Remove old DB health data from tangle database
	if err := tangleStore.DeletePrefix(kvstore.KeyPrefix{storePrefixHealthDeprecated}); err != nil {
		return fmt.Errorf("error deleting legacy health data from tangle database: %w", err)
	}

	if err := tangleStore.Flush(); err != nil {
		return fmt.Errorf("error flushing tangle database: %w", err)
	}

	if err := utxoStore.Flush(); err != nil {
		return fmt.Errorf("error flushing utxo database: %w", err)
	}

	return nil
}

func databaseMigrateKeys(source kvstore.KVStore, target kvstore.KVStore, prefix kvstore.KeyPrefix, replacementPrefix kvstore.KeyPrefix) error {

	copyBytes := func(source []byte) []byte {
		cpy := make([]byte, len(source))
		copy(cpy, source)
		return cpy
	}

	copyBytesReplacingPrefix := func(source []byte, prefix []byte, replacementPrefix []byte) []byte {
		cpy := make([]byte, len(source)+(len(replacementPrefix)-len(prefix)))
		copy(cpy, replacementPrefix)
		copy(cpy[len(replacementPrefix):], source[len(prefix):])
		return cpy
	}

	var errDB error
	if err := source.Iterate(prefix, func(key []byte, value kvstore.Value) bool {
		dstKey := copyBytesReplacingPrefix(key, prefix, replacementPrefix)
		dstValue := copyBytes(value)

		if errDB = target.Set(dstKey, dstValue); errDB != nil {
			return false
		}

		return true
	}); err != nil {
		return fmt.Errorf("source database iteration failed: %w", err)
	}

	if errDB != nil {
		return fmt.Errorf("taget database set failed: %w", errDB)
	}
	return nil
}
