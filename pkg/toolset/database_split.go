package toolset

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/core/database"
	"github.com/iotaledger/hive.go/configuration"
)

func databaseSplit(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "", "the path to the database folder that should be split")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseSplit)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolDatabaseSplit,
			FlagToolDatabasePath,
			"mainnetdb"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	needsSplitting, err := database.NeedsSplitting(*databasePathFlag)
	if err != nil {
		return err
	}
	if !needsSplitting {
		return fmt.Errorf("legacy database not found. Already migrated?")
	}

	err = database.SplitIntoTangleAndUTXO(*databasePathFlag)
	if err == nil {
		fmt.Println("The split database might be larger. Run your node to compact the new database automatically")
	}
	return err
}
