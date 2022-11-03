package toolset

import (
	"fmt"
	"os"
	"path"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/database"
)

func databaseHealth(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "mainnetdb/tangle", "the path to the database folder that should be checked")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseHealth)
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

		dbExists, err := hivedb.Exists(path)
		if err != nil {
			return err
		}

		if !dbExists {
			print(fmt.Sprintf("database %s does not exist (%s)!\n", name, path))

			return nil
		}

		dbStore, err := database.StoreWithDefaultSettings(path, false, hivedb.EngineAuto, database.AllowedEnginesStorageAuto...)
		if err != nil {
			return fmt.Errorf("%s database initialization failed: %w", name, err)
		}
		defer func() { _ = dbStore.Close() }()

		healthTracker, err := kvstore.NewStoreHealthTracker(dbStore, []byte{common.StorePrefixHealth}, kvstore.StoreVersionNone, nil)
		if err != nil {
			return err
		}

		storeVersion, err := healthTracker.StoreVersion()
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
				Version:  int(storeVersion),
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
			storeVersion,
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
