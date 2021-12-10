package toolset

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func snapshotHash(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("   %s [FULL_SNAPSHOT_PATH] [DELTA_SNAPSHOT_PATH]", ToolSnapHash))
		println()
		println("   [FULL_SNAPSHOT_PATH]  - the path to the full snapshot file")
		println("   [DELTA_SNAPSHOT_PATH] - the path to the delta snapshot file (optional)")
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
		return errors.New("FULL_SNAPSHOT_PATH is missing")
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

	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.TangleDatabaseDirectoryName), true, targetEngine)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.UTXODatabaseDirectoryName), true, targetEngine)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up temp db
	defer func() {
		tangleStore.Shutdown()
		_ = tangleStore.Close()

		utxoStore.Shutdown()
		_ = utxoStore.Close()

		_ = os.RemoveAll(tempDir)
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return err
	}

	_, _, err = snapshot.LoadSnapshotFilesToStorage(context.Background(), dbStorage, fullPath, deltaPath)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage)
}
