package toolset

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	"github.com/pkg/errors"

	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/serializer/v2"
	databasecore "github.com/iotaledger/hornet/v2/core/database"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	// ErrCritical is returned when a critical error stops the execution of a task.
	ErrCritical = errors.New("critical error")
)

func getMilestoneManagerFromConfigFile(filePath string) (*milestonemanager.MilestoneManager, error) {

	if err := loadConfigFile(filePath, map[string]any{
		"protocol": protocfg.ParamsProtocol,
	}); err != nil {
		return nil, err
	}

	keyManager, err := protocfg.KeyManagerWithConfigPublicKeyRanges(protocfg.ParamsProtocol.PublicKeyRanges)
	if err != nil {
		return nil, err
	}

	return milestonemanager.New(nil, nil, keyManager, protocfg.ParamsProtocol.MilestonePublicKeyCount), nil
}

func checkDatabaseHealth(storage *storage.Storage, markTainted bool) error {

	corrupted, err := storage.AreStoresCorrupted()
	if err != nil {
		return err
	}

	if corrupted {
		if markTainted {
			if err := storage.MarkStoresTainted(); err != nil {
				return err
			}
		}

		return errors.New("database is corrupted")
	}

	tainted, err := storage.AreStoresTainted()
	if err != nil {
		return err
	}

	if tainted {
		return errors.New("database is tainted")
	}

	return nil
}

// getMilestonePayloadFromStorage returns the milestone payload from the storage.
func getMilestonePayloadFromStorage(tangleStore *storage.Storage, msIndex iotago.MilestoneIndex) (*iotago.Milestone, error) {

	cachedMilestone := tangleStore.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	if cachedMilestone == nil {
		return nil, fmt.Errorf("milestone not found! %d", msIndex)
	}
	defer cachedMilestone.Release(true) // milestone -1

	return cachedMilestone.Milestone().Milestone(), nil
}

// getStorageMilestoneRange returns the range of milestones that are found in the storage.
func getStorageMilestoneRange(tangleStore *storage.Storage) (iotago.MilestoneIndex, iotago.MilestoneIndex) {
	var msIndexStart iotago.MilestoneIndex = math.MaxUint32
	var msIndexEnd iotago.MilestoneIndex

	tangleStore.ForEachMilestoneIndex(func(msIndex iotago.MilestoneIndex) bool {
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

type StoreBlockInterface interface {
	StoreBlockIfAbsent(block *storage.Block) (cachedBlock *storage.CachedBlock, newlyAdded bool)
	StoreChild(parentBlockID iotago.BlockID, childBlockID iotago.BlockID) *storage.CachedChild
	StoreMilestoneIfAbsent(milestonePayload *iotago.Milestone) (*storage.CachedMilestone, bool)
}

// storeBlock adds a new block to the storage,
// including all additional information like
// metadata, children, indexation and milestone entries.
// block +1.
func storeBlock(protoParams *iotago.ProtocolParameters, dbStorage StoreBlockInterface, milestoneManager *milestonemanager.MilestoneManager, blk *iotago.Block) (*storage.CachedBlock, error) {

	block, err := storage.NewBlock(blk, serializer.DeSeriModePerformValidation, protoParams)
	if err != nil {
		return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid block, error: %s", err)
	}

	cachedBlock, isNew := dbStorage.StoreBlockIfAbsent(block) // block +1
	if !isNew {
		// no need to process known blocks
		return cachedBlock, nil
	}

	for _, parent := range block.Parents() {
		dbStorage.StoreChild(parent, cachedBlock.Block().BlockID()).Release(true) // child +-0
	}

	if milestonePayload := milestoneManager.VerifyMilestoneBlock(block.Block()); milestonePayload != nil {
		cachedMilestone, _ := dbStorage.StoreMilestoneIfAbsent(milestonePayload) // milestone +1

		// Force release to store milestones without caching
		cachedMilestone.Release(true) // milestone -1
	}

	return cachedBlock, nil
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

	dbEngine, err := hivedb.EngineFromStringAllowed(dbEngineStr, database.AllowedEnginesStorageAuto...)
	if err != nil {
		return nil, err
	}

	if checkExist {
		databaseExists, err := hivedb.Exists(path)
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
		return common.ErrSnapshotInfoNotFound
	}

	return nil
}

func createTangleStorage(name string, tangleDatabasePath string, utxoDatabasePath string, dbEngine hivedb.Engine) (*storage.Storage, error) {

	allowedEngines := database.AllowedEnginesDefault

	storeTangle, err := database.StoreWithDefaultSettings(tangleDatabasePath, true, dbEngine, allowedEngines...)
	if err != nil {
		return nil, fmt.Errorf("%s tangle database initialization failed: %w", name, err)
	}

	storeUTXO, err := database.StoreWithDefaultSettings(utxoDatabasePath, true, dbEngine, allowedEngines...)
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
func loadGenesisSnapshot(ctx context.Context, storage *storage.Storage, genesisSnapshotFilePath string, checkSourceNetworkID bool, sourceNetworkID uint64) error {

	fullHeader, err := snapshot.ReadFullSnapshotHeaderFromFile(genesisSnapshotFilePath)
	if err != nil {
		return err
	}

	fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
	if err != nil {
		return err
	}

	if checkSourceNetworkID && sourceNetworkID != fullHeaderProtoParams.NetworkID() {
		return fmt.Errorf("source storage networkID not equal to genesis snapshot networkID (%d != %d)", sourceNetworkID, fullHeaderProtoParams.NetworkID())
	}

	if _, _, err := snapshot.LoadSnapshotFilesToStorage(ctx, storage, false, genesisSnapshotFilePath); err != nil {
		return err
	}

	return nil
}
