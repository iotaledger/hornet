package toolset

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

const (
	FlagToolSnapshotHashFullSnapshotPath  = "fullSnapshotPath"
	FlagToolSnapshotHashDeltaSnapshotPath = "deltaSnapshotPath"
)

func snapshotHash(_ *configuration.Configuration, args []string) error {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fullSnapshotPathFlag := fs.String(FlagToolSnapshotHashFullSnapshotPath, "snapshots/mainnet/full_snapshot.bin", "the path to the full snapshot file")
	deltaSnapshotPathFlag := fs.String(FlagToolSnapshotHashDeltaSnapshotPath, "snapshots/mainnet/delta_snapshot.bin", "the path to the delta snapshot file (optional)")
	outputJSON := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapHash)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s", ToolSnapHash, FlagToolSnapshotHashFullSnapshotPath, "./snapshots/mainnet/full_snapshot.bin", FlagToolSnapshotHashDeltaSnapshotPath, "snapshots/mainnet/delta_snapshot.bin"))
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	fullPath := *fullSnapshotPathFlag
	deltaPath := *deltaSnapshotPathFlag

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

	_, _, err = snapshot.LoadSnapshotFilesToStorage(context.Background(), dbStorage, nil, fullPath, deltaPath)
	if err != nil {
		return err
	}

	return calculateDatabaseLedgerHash(dbStorage, *outputJSON)
}
