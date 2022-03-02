package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	coreDatabase "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
)

func coordinatorFixStateFile(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, DefaultValueMainnetDatabasePath, "the path to the database")
	cooStateFilePathFlag := fs.String(FlagToolCoordinatorFixStateCooStateFilePath, DefaultValueCoordinatorStateFilePath, "the path to the coordinator state file")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolCoordinatorFixStateFile)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolCoordinatorFixStateFile,
			FlagToolDatabasePath,
			DefaultValueMainnetDatabasePath,
			FlagToolCoordinatorFixStateCooStateFilePath,
			DefaultValueCoordinatorStateFilePath))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	databasePath := *databasePathFlag
	if _, err := os.Stat(databasePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolDatabasePath, databasePath)
	}

	coordinatorStateFilePath := *cooStateFilePathFlag
	if coordinatorStateFilePath == "" {
		return fmt.Errorf("'%s' is missing", FlagToolCoordinatorFixStateCooStateFilePath)
	}

	tangleStore, err := database.StoreWithDefaultSettings(filepath.Join(databasePath, coreDatabase.TangleDatabaseDirectoryName), false)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.TangleDatabaseDirectoryName, err)
	}

	utxoStore, err := database.StoreWithDefaultSettings(filepath.Join(databasePath, coreDatabase.UTXODatabaseDirectoryName), false)
	if err != nil {
		return fmt.Errorf("%s database initialization failed: %w", coreDatabase.UTXODatabaseDirectoryName, err)
	}

	// clean up store
	defer func() {
		tangleStore.Shutdown()
		_ = tangleStore.Close()

		utxoStore.Shutdown()
		_ = utxoStore.Close()
	}()

	dbStorage, err := storage.New(tangleStore, utxoStore)
	if err != nil {
		return err
	}

	ledgerIndex, err := dbStorage.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return err
	}

	latestMilestoneFromDatabase := dbStorage.SearchLatestMilestoneIndexInStore()

	if ledgerIndex != latestMilestoneFromDatabase {
		return fmt.Errorf("node is not synchronized (solid milestone index: %d, latest milestone index: %d)", ledgerIndex, latestMilestoneFromDatabase)
	}

	cachedMilestone := dbStorage.CachedMilestoneOrNil(ledgerIndex) // milestone +1
	if cachedMilestone == nil {
		return fmt.Errorf("milestone %d not found", ledgerIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	_, err = os.Stat(coordinatorStateFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to check '%s' (%s), error: %w", FlagToolCoordinatorFixStateCooStateFilePath, coordinatorStateFilePath, err)
	}

	if err == nil {
		// coordinator state file exists => rename it to not overwrite the original
		backupFilePath := fmt.Sprintf("%s_backup", coordinatorStateFilePath)

		_, err = os.Stat(backupFilePath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("unable to check backup file path (%s), error: %w", backupFilePath, err)
		}

		if err == nil {
			return fmt.Errorf("backup file path already exists (%s), will not proceed to overwrite old backup file", backupFilePath)
		}

		if err := os.Rename(coordinatorStateFilePath, backupFilePath); err != nil {
			return fmt.Errorf("unable to rename coordinator state file, error: %w", err)
		}
	}

	// state of the coordinator holds information about the last issued milestones.
	state := &coordinator.State{
		LatestMilestoneIndex:     ledgerIndex,
		LatestMilestoneMessageID: cachedMilestone.Milestone().MessageID,
		LatestMilestoneTime:      cachedMilestone.Milestone().Timestamp,
	}

	if err := utils.WriteJSONToFile(coordinatorStateFilePath, state, 0660); err != nil {
		return fmt.Errorf("failed to write coordinator state file (%s), error: %w", coordinatorStateFilePath, err)
	}

	return nil
}
