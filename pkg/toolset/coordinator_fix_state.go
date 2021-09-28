package toolset

import (
	"fmt"
	"os"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
)

func coordinatorFixStateFile(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [DATABASE_PATH] [COO_STATE_FILE_PATH]", ToolCoordinatorFixStateFile))
		println()
		println("   [DATABASE_PATH]       - the path to the database")
		println("   [COO_STATE_FILE_PATH] - the path to the coordinator state file")
		println()
		println(fmt.Sprintf("example: %s %s %s", ToolCoordinatorFixStateFile, "mainnetdb", "./coordinator.state"))
	}

	if len(args) != 2 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolCoordinatorFixStateFile)
	}

	databasePath := args[0]
	if _, err := os.Stat(databasePath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("DATABASE_PATH (%s) does not exist", databasePath)
	}

	coordinatorStateFilePath := args[1]
	if coordinatorStateFilePath == "" {
		return errors.New("COO_STATE_FILE_PATH is missing")
	}

	store, err := database.StoreWithDefaultSettings(databasePath, false)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	// clean up store
	defer func() {
		store.Shutdown()
		_ = store.Close()
	}()

	dbStorage, err := storage.New(store)
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

	cachedMs := dbStorage.CachedMilestoneOrNil(ledgerIndex) // milestone +1
	if cachedMs == nil {
		return fmt.Errorf("milestone %d not found", ledgerIndex)
	}
	defer cachedMs.Release(true) // milestone -1

	_, err = os.Stat(coordinatorStateFilePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to check COO_STATE_FILE_PATH (%s), error: %w", coordinatorStateFilePath, err)
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
		LatestMilestoneMessageID: cachedMs.Milestone().MessageID,
		LatestMilestoneTime:      cachedMs.Milestone().Timestamp,
	}

	if err := utils.WriteJSONToFile(coordinatorStateFilePath, state, 0660); err != nil {
		return fmt.Errorf("failed to write coordinator state file (%s), error: %w", coordinatorStateFilePath, err)
	}

	return nil
}
