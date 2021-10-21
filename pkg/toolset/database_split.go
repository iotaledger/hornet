package toolset

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	flag "github.com/spf13/pflag"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/kvstore"
	storeUtils "github.com/iotaledger/hive.go/kvstore/utils"
)

func databaseSplit(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	databasePath := fs.String("database", "", "the path to the p2p database folder")
	databaseEngine := fs.String("engine", "rocksdb", "the engine of the target database (values: pebble, rocksdb [default])")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseSplit)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if all parameters were parsed
	if len(args) == 0 || fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
	}

	legacyDatabasePath := *databasePath
	tangleDatabasePath := filepath.Join(legacyDatabasePath, coreDatabase.TangleDatabaseDirectoryName)
	utxoDatabasePath := filepath.Join(legacyDatabasePath, coreDatabase.UTXODatabaseDirectoryName)

	exists, err := storeUtils.PathExists(legacyDatabasePath)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%s does not exist\n", legacyDatabasePath)
	}

	exists, err = storeUtils.PathExists(tangleDatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error %s:\n", err)
		os.Exit(1)
	}
	if exists {
		fmt.Fprint(os.Stderr, "database already migrated?\n")
		os.Exit(1)
	}

	if err := storeUtils.CreateDirectory(tangleDatabasePath, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "error %s:\n", err)
		os.Exit(1)
	}

	if err := storeUtils.CreateDirectory(utxoDatabasePath, 0700); err != nil {
		fmt.Fprintf(os.Stderr, "error %s:\n", err)
		os.Exit(1)
	}

	// Move the legacy database into the tangle directory
	files, err := ioutil.ReadDir(legacyDatabasePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error %s:\n", err)
		os.Exit(1)
	}
	for _, f := range files {
		if f.IsDir() && (f.Name() == coreDatabase.TangleDatabaseDirectoryName || f.Name() == coreDatabase.UTXODatabaseDirectoryName) {
			continue
		}
		os.Rename(filepath.Join(legacyDatabasePath, f.Name()), filepath.Join(tangleDatabasePath, f.Name()))
	}

	dbEngine, err := database.DatabaseEngine(strings.ToLower(*databaseEngine))
	if err != nil {
		return err
	}

	tangleStore, err := database.StoreWithDefaultSettings(tangleDatabasePath, false, dbEngine)
	if err != nil {
		return fmt.Errorf("tangle database initialization failed: %w", err)
	}
	defer func() { _ = tangleStore.Close() }()

	utxoStore, err := database.StoreWithDefaultSettings(utxoDatabasePath, true, dbEngine)
	if err != nil {
		return fmt.Errorf("utxo database initialization failed: %w", err)
	}
	defer func() { _ = utxoStore.Close() }()

	fmt.Println("Splitting database... ")

	// Migrate the UTXO data by removing the old 8 prefix
	legacyStorePrefixUTXO := kvstore.KeyPrefix{byte(8)}
	if err := databaseMigrateKeys(tangleStore, utxoStore, legacyStorePrefixUTXO, kvstore.EmptyPrefix); err != nil {
		return fmt.Errorf("error migrating data to utxo database: %w", err)
	}

	// Remove UTXO data from tangle database
	if err := tangleStore.DeletePrefix(legacyStorePrefixUTXO); err != nil {
		return fmt.Errorf("error deleting utxo data from tangle database: %w", err)
	}

	// Copy the DB health data to UTXO by replacing the old 0 prefix with the new 255 prefix
	if err := databaseMigrateKeys(tangleStore, utxoStore, kvstore.KeyPrefix{common.StorePrefixHealthDeprecated}, kvstore.KeyPrefix{common.StorePrefixHealth}); err != nil {
		return fmt.Errorf("error copying health data to utxo database: %w", err)
	}

	// Migrate the DB health data in the tangle database to the new keys
	if err := databaseMigrateKeys(tangleStore, tangleStore, kvstore.KeyPrefix{common.StorePrefixHealthDeprecated}, kvstore.KeyPrefix{common.StorePrefixHealth}); err != nil {
		return fmt.Errorf("error migrating tangle health database: %w", err)
	}

	// Remove old DB health data from tangle database
	if err := tangleStore.DeletePrefix(kvstore.KeyPrefix{common.StorePrefixHealthDeprecated}); err != nil {
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
