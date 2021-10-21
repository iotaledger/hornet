package toolset

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/core/database"
	"github.com/iotaledger/hive.go/configuration"
)

func databaseSplit(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	databasePath := fs.String("database", "", "the path to the p2p database folder")

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

	needsSplitting, err := database.NeedsSplitting(*databasePath)
	if err != nil {
		return err
	}
	if !needsSplitting {
		return fmt.Errorf("legacy database not found. Already migrated?")
	}

	err = database.SplitIntoTangleAndUTXO(*databasePath)
	if err == nil {
		fmt.Println("The split database might be larger. Run your node to compact the new database automatically")
	}
	return err
}
