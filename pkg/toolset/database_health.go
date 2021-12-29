package toolset

import (
	"fmt"
	"os"
	"path"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"

	"github.com/iotaledger/hive.go/configuration"
)

func databaseHealth(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	databasePath := fs.String("database", "", "the path to the database folder that should be checked")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseHealth)
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

	checkDatabaseHealth := func(path string, name string) error {

		dbExists, err := database.DatabaseExists(path)
		if err != nil {
			return err
		}

		if !dbExists {
			print(fmt.Sprintf("database %s does not exist (%s)!\n", name, path))
			return nil
		}

		dbStore, err := database.StoreWithDefaultSettings(path, false)
		if err != nil {
			return fmt.Errorf("%s database initialization failed: %w", name, err)
		}
		defer func() { _ = dbStore.Close() }()

		healthTracker := storage.NewStoreHealthTracker(dbStore)

		dbVersion, err := healthTracker.DatabaseVersion()
		if err != nil {
			return err
		}

		isCorrupted, err := healthTracker.IsCorrupted()
		if err != nil {
			return err
		}

		isTainted, err := healthTracker.IsTainted()
		if err != nil {
			return err
		}

		print(fmt.Sprintf("Database: '%s', Version: %d, IsCorrupted: %t, IsTainted: %t\n", name, dbVersion, isCorrupted, isTainted))
		return nil
	}

	dbPath := *databasePath
	dbName := path.Base(dbPath)

	if err := checkDatabaseHealth(dbPath, dbName); err != nil {
		return err
	}

	return nil
}
