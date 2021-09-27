package toolset

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func snapshotHash(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [FULL_SNAPSHOT_PATH] [DELTA_SNAPSHOT_PATH]", ToolSnapHash))
		println()
		println("	[FULL_SNAPSHOT_PATH]  - the path to the full snapshot file")
		println("	[DELTA_SNAPSHOT_PATH] - the path to the delta snapshot file (optional)")
		println()
		println(fmt.Sprintf("example: %s %s", ToolSnapHash, "./snapshot.bin"))
	}

	if len(args) > 2 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolSnapHash)
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("FULL_SNAPSHOT_PATH missing")
	}

	fullPath := args[0]
	deltaPath := ""

	if len(args) == 2 {
		deltaPath = args[1]
	}

	targetEngine, err := database.DatabaseEngine(database.EnginePebble)
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "snapHash")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %w", err)
	}

	store, err := database.StoreWithDefaultSettings(tempDir, true, targetEngine)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// clean up temp db
	defer func() {
		store.Shutdown()
		_ = store.Close()
		_ = os.RemoveAll(tempDir)
	}()

	dbStorage, err := storage.New(store)
	if err != nil {
		return err
	}

	_, _, err = snapshot.LoadSnapshotFilesToStorage(context.Background(), dbStorage, fullPath, deltaPath)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage)
}
