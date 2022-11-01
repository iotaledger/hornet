package toolset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	hivedb "github.com/iotaledger/hive.go/core/database"
	coreDatabase "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
)

func snapshotHash(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	fullSnapshotPathFlag := fs.String(FlagToolSnapshotPathFull, "", "the path to the full snapshot file")
	deltaSnapshotPathFlag := fs.String(FlagToolSnapshotPathDelta, "", "the path to the delta snapshot file (optional)")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapHash)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolSnapHash,
			FlagToolSnapshotPathFull,
			"snapshots/mainnet/full_snapshot.bin",
			FlagToolSnapshotPathDelta,
			"snapshots/mainnet/delta_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*fullSnapshotPathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPathFull)
	}

	fullPath := *fullSnapshotPathFlag
	deltaPath := *deltaSnapshotPathFlag

	allowedEngines := database.AllowedEnginesStorage

	targetEngine, err := hivedb.EngineAllowed(hivedb.EnginePebble, allowedEngines...)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "snapHash")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %w", err)
	}

	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.TangleDatabaseDirectoryName), true, targetEngine, allowedEngines...)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(tempDir, coreDatabase.UTXODatabaseDirectoryName), true, targetEngine, allowedEngines...)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up temp db
	defer func() {
		_ = tangleStore.Close()
		_ = utxoStore.Close()

		_ = os.RemoveAll(tempDir)
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return err
	}

	_, _, err = snapshot.LoadSnapshotFilesToStorage(context.Background(), dbStorage, false, fullPath, deltaPath)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage, *outputJSONFlag)
}
