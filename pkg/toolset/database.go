package toolset

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	"github.com/pkg/errors"

	databasecore "github.com/gohornet/hornet/core/database"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	// Returned when a critical error stops the execution of a task.
	ErrCritical = errors.New("critical error")
)

func getMilestoneManagerFromConfigFile(filePath string) (*milestonemanager.MilestoneManager, error) {

	_, err := loadConfigFile(filePath, map[string]any{
		"protocol": protocfg.ParamsProtocol,
	})
	if err != nil {
		return nil, err
	}

	keyManager, err := protocfg.KeyManagerWithConfigPublicKeyRanges(protocfg.ParamsProtocol.PublicKeyRanges)
	if err != nil {
		return nil, err
	}

	return milestonemanager.New(nil, nil, keyManager, protocfg.ParamsProtocol.MilestonePublicKeyCount), nil
}

func checkDatabaseHealth(storage *storage.Storage, markTainted bool) error {

	corrupted, err := storage.AreDatabasesCorrupted()
	if err != nil {
		return err
	}

	if corrupted {
		if markTainted {
			if err := storage.MarkDatabasesTainted(); err != nil {
				return err
			}
		}
		return errors.New("database is corrupted")
	}

	tainted, err := storage.AreDatabasesTainted()
	if err != nil {
		return err
	}

	if tainted {
		return errors.New("database is tainted")
	}

	return nil
}

// getMilestonePayloadFromStorage returns the milestone payload from the storage.
func getMilestonePayloadFromStorage(tangleStore *storage.Storage, msIndex milestone.Index) (*iotago.Milestone, error) {

	cachedMilestone := tangleStore.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, fmt.Errorf("milestone not found! %d", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Milestone(), nil
}

// getStorageMilestoneRange returns the range of milestones that are found in the storage.
func getStorageMilestoneRange(tangleStore *storage.Storage) (milestone.Index, milestone.Index) {
	var msIndexStart milestone.Index = math.MaxUint32
	var msIndexEnd milestone.Index = 0

	tangleStore.ForEachMilestoneIndex(func(msIndex milestone.Index) bool {
		if msIndexStart > msIndex {
			msIndexStart = msIndex
		}
		if msIndexEnd < msIndex {
			msIndexEnd = msIndex
		}
		return true
	})

	if msIndexStart == math.MaxUint32 {
		// no milestone found
		msIndexStart = 0
	}

	return msIndexStart, msIndexEnd
}

type StoreMessageInterface interface {
	StoreMessageIfAbsent(message *storage.Message) (cachedMsg *storage.CachedMessage, newlyAdded bool)
	StoreChild(parentMessageID hornet.MessageID, childMessageID hornet.MessageID) *storage.CachedChild
	StoreMilestoneIfAbsent(milestonePayload *iotago.Milestone, messageID hornet.MessageID) (*storage.CachedMilestone, bool)
}

// storeMessage adds a new message to the storage,
// including all additional information like
// metadata, children, indexation and milestone entries.
// message +1
func storeMessage(protoParas *iotago.ProtocolParameters, dbStorage StoreMessageInterface, milestoneManager *milestonemanager.MilestoneManager, msg *iotago.Message) (*storage.CachedMessage, error) {

	message, err := storage.NewMessage(msg, serializer.DeSeriModePerformValidation, protoParas)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid message, error: %s", err)
	}

	cachedMsg, isNew := dbStorage.StoreMessageIfAbsent(message) // message +1
	if !isNew {
		// no need to process known messages
		return cachedMsg, nil
	}

	for _, parent := range message.Parents() {
		dbStorage.StoreChild(parent, cachedMsg.Message().MessageID()).Release(true) // child +-0
	}

	if milestonePayload := milestoneManager.VerifyMilestoneMessage(message.Message()); milestonePayload != nil {
		cachedMilestone, _ := dbStorage.StoreMilestoneIfAbsent(milestonePayload, message.MessageID()) // milestone +1

		// Force release to store milestones without caching
		cachedMilestone.Release(true) // milestone -1
	}

	return cachedMsg, nil
}

// getTangleStorage returns a tangle storage. If specified, it checks if the database exists,
// splits old databases and checks for database health or marks it as tainted if not healthy.
func getTangleStorage(path string,
	name string,
	dbEngineStr string,
	checkExist bool,
	checkHealth bool,
	markTainted bool,
	checkSnapInfo bool) (*storage.Storage, error) {

	dbEngine, err := database.DatabaseEngineFromStringAllowed(dbEngineStr, database.EnginePebble, database.EngineRocksDB, database.EngineAuto)
	if err != nil {
		return nil, err
	}

	if checkExist {
		databaseExists, err := database.DatabaseExists(path)
		if err != nil {
			return nil, err
		}

		if !databaseExists {
			return nil, fmt.Errorf("%s database does not exist (%s)", name, path)
		}
	}

	tangleStore, err := createTangleStorage(
		name,
		filepath.Join(path, databasecore.TangleDatabaseDirectoryName),
		filepath.Join(path, databasecore.UTXODatabaseDirectoryName),
		dbEngine,
	)
	if err != nil {
		return nil, err
	}

	if checkHealth {
		if err := checkDatabaseHealth(tangleStore, markTainted); err != nil {
			return nil, fmt.Errorf("%s storage initialization failed: %w", name, err)
		}
	}

	if checkSnapInfo {
		if err := checkSnapshotInfo(tangleStore); err != nil {
			return nil, fmt.Errorf("%s storage initialization failed: %w", name, err)
		}
	}

	return tangleStore, nil
}

func checkSnapshotInfo(dbStorage *storage.Storage) error {
	if dbStorage.SnapshotInfo() == nil {
		return errors.New("snapshot info not found")
	}
	return nil
}

func createTangleStorage(name string, tangleDatabasePath string, utxoDatabasePath string, dbEngine ...database.Engine) (*storage.Storage, error) {

	storeTangle, err := database.StoreWithDefaultSettings(tangleDatabasePath, true, dbEngine...)
	if err != nil {
		return nil, fmt.Errorf("%s tangle database initialization failed: %w", name, err)
	}

	storeUTXO, err := database.StoreWithDefaultSettings(utxoDatabasePath, true, dbEngine...)
	if err != nil {
		return nil, fmt.Errorf("%s utxo database initialization failed: %w", name, err)
	}

	tangleStore, err := storage.New(storeTangle, storeUTXO)
	if err != nil {
		return nil, fmt.Errorf("%s storage initialization failed: %w", name, err)
	}

	return tangleStore, nil
}

// loadGenesisSnapshot loads the genesis snapshot to the storage and checks if the networkID fits.
func loadGenesisSnapshot(storage *storage.Storage, genesisSnapshotFilePath string, checkSourceNetworkID bool, sourceNetworkID uint64) error {

	fullHeader, err := snapshot.ReadSnapshotHeaderFromFile(genesisSnapshotFilePath)
	if err != nil {
		return err
	}

	if checkSourceNetworkID && sourceNetworkID != fullHeader.NetworkID {
		return fmt.Errorf("source storage networkID not equal to genesis snapshot networkID (%d != %d)", sourceNetworkID, fullHeader.NetworkID)
	}

	if _, _, err := snapshot.LoadSnapshotFilesToStorage(context.Background(), storage, nil, genesisSnapshotFilePath); err != nil {
		return err
	}

	return nil
}
