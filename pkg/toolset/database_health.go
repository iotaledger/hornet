package toolset

import (
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"

	"github.com/iotaledger/hive.go/configuration"
)

func databaseHealth(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePath := fs.String(FlagToolDatabasePath, "", "the path to the database folder that should be checked")
	outputJSON := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseHealth)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Check if all parameters were parsed
	if len(args) == 0 || fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
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

			output, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			}
			fmt.Println(string(output))
			return nil
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

	dbPath := *databasePath
	dbName := path.Base(dbPath)

	if err := checkDatabaseHealth(dbPath, dbName, *outputJSON); err != nil {
		return err
	}

	return nil
}
