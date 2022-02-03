package toolset

import (
	"fmt"
	"os"
	"path"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"
)

func databaseHealth(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "mainnetdb/tangle", "the path to the database folder that should be checked")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseHealth)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolDatabaseHealth,
			FlagToolDatabasePath,
			"mainnetdb/tangle"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	checkDatabaseHealth := func(path string, name string, outputJSON bool) error {

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

		corrupted, err := healthTracker.IsCorrupted()
		if err != nil {
			return err
		}

		tainted, err := healthTracker.IsTainted()
		if err != nil {
			return err
		}

		if outputJSON {

			result := struct {
				Database string `json:"database"`
				Version  int    `json:"version"`
				Healthy  bool   `json:"healthy"`
				Tainted  bool   `json:"tainted"`
			}{
				Database: name,
				Version:  dbVersion,
				Healthy:  !corrupted,
				Tainted:  tainted,
			}

			return printJSON(result)
		}

		fmt.Printf(`    >
        - Database:       %s
        - Version:        %d
        - Healthy:        %s
        - Tainted:        %s`+"\n\n",
			name,
			dbVersion,
			yesOrNo(!corrupted),
			yesOrNo(tainted),
		)

		return nil
	}

	dbPath := *databasePathFlag
	dbName := path.Base(dbPath)

	if err := checkDatabaseHealth(dbPath, dbName, *outputJSONFlag); err != nil {
		return err
	}

	return nil
}
