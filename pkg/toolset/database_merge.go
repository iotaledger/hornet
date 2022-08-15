package toolset

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/contextutils"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
)

var (
	// ErrNoNewTangleData is returned when there is no new data in the source database.
	ErrNoNewTangleData = errors.New("no new tangle history available")
)

func databaseMerge(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	configFilePathFlag := fs.String(FlagToolConfigFilePath, "", "the path to the config file")
	genesisSnapshotFilePathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the genesis snapshot file (optional)")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	databasePathTargetFlag := fs.String(FlagToolDatabasePathTarget, "", "the path to the target database")
	databaseEngineSourceFlag := fs.String(FlagToolDatabaseEngineSource, string(database.EngineAuto), "the engine of the source database (optional, values: pebble, rocksdb, auto)")
	databaseEngineTargetFlag := fs.String(FlagToolDatabaseEngineTarget, string(DefaultValueDatabaseEngine), "the engine of the target database (values: pebble, rocksdb)")
	targetIndexFlag := fs.Uint32(FlagToolDatabaseTargetIndex, 0, "the target index (optional)")
	nodeURLFlag := fs.String(FlagToolDatabaseMergeNodeURL, "", "URL of the node (optional)")
	apiParallelismFlag := fs.Uint("apiParallelism", 50, "the amount of concurrent API requests")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseMerge)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s --%s %s --%s %s",
			ToolDatabaseMerge,
			FlagToolConfigFilePath,
			"config.json",
			FlagToolDatabasePathSource,
			DefaultValueMainnetDatabasePath,
			FlagToolDatabaseEngineSource,
			database.EnginePebble,
			FlagToolDatabasePathTarget,
			"database_new",
			FlagToolSnapshotPath,
			"genesis_snapshot.bin",
		))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*configFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolConfigFilePath)
	}
	if len(*databasePathSourceFlag) == 0 {
		if len(*nodeURLFlag) == 0 {
			return fmt.Errorf("either '%s' or '%s' must be specified", FlagToolDatabasePathSource, FlagToolDatabaseMergeNodeURL)
		}
	}
	if len(*databasePathTargetFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePathTarget)
	}
	if len(*databaseEngineSourceFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabaseEngineSource)
	}
	if len(*databaseEngineTargetFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabaseEngineTarget)
	}

	// TODO: needs to be adapted for when protocol parameters struct changes
	protoParams := &iotago.ProtocolParameters{}

	var tangleStoreSource *storage.Storage = nil
	if len(*databasePathSourceFlag) > 0 {
		var err error

		// we don't need to check the health of the source db.
		// it is fine as long as all blocks in the cone are found.
		tangleStoreSource, err = getTangleStorage(*databasePathSourceFlag, "source", *databaseEngineSourceFlag, true, false, false, true)
		if err != nil {
			return err
		}
		defer func() {
			println("\nshutdown source storage...")
			if err := tangleStoreSource.Shutdown(); err != nil {
				panic(err)
			}
		}()
	}

	// we need to check the health of the target db, since we don't want use tainted/corrupted dbs.
	tangleStoreTarget, err := getTangleStorage(*databasePathTargetFlag, "target", *databaseEngineTargetFlag, false, true, true, false)
	if err != nil {
		return err
	}
	defer func() {
		println("\nshutdown target storage...")
		if err := tangleStoreTarget.Shutdown(); err != nil {
			panic(err)
		}
	}()

	_, msIndexEndTarget := getStorageMilestoneRange(tangleStoreTarget)
	if msIndexEndTarget == 0 {
		// no ledger state in database available => we need to load the genesis snapshot
		if len(*genesisSnapshotFilePathFlag) == 0 {
			return fmt.Errorf("'%s' not specified", FlagToolSnapshotPath)
		}
	}

	milestoneManager, err := getMilestoneManagerFromConfigFile(*configFilePathFlag)
	if err != nil {
		return err
	}

	client := getNodeHTTPAPIClient(*nodeURLFlag)

	// mark the database as corrupted.
	// this flag will be cleared after the operation finished successfully.
	if err := tangleStoreTarget.MarkDatabasesCorrupted(); err != nil {
		return err
	}

	ts := time.Now()
	println(fmt.Sprintf("merging databases... (source: %s, target: %s)", *databasePathSourceFlag, *databasePathTargetFlag))

	errMerge := mergeDatabase(
		getGracefulStopContext(),
		protoParams,
		milestoneManager,
		tangleStoreSource,
		tangleStoreTarget,
		client,
		*targetIndexFlag,
		*genesisSnapshotFilePathFlag,
		int(*apiParallelismFlag),
	)
	if errMerge != nil && errors.Is(errMerge, ErrCritical) {
		// do not mark the database as healthy in case of critical errors
		return errMerge
	}

	// mark clean shutdown of the database
	if err := tangleStoreTarget.MarkDatabasesHealthy(); err != nil {
		return err
	}

	if errMerge != nil {
		return errMerge
	}

	msIndexStart, msIndexEnd := getStorageMilestoneRange(tangleStoreTarget)
	println(fmt.Sprintf("\nsuccessfully merged %d milestones, took: %v", msIndexEnd-msIndexEndTarget, time.Since(ts).Truncate(time.Millisecond)))
	println(fmt.Sprintf("milestone range in database: %d-%d (target)", msIndexStart, msIndexEnd))

	return nil
}

// copyMilestoneCone copies all blocks of a milestone cone to the target storage.
func copyMilestoneCone(
	ctx context.Context,
	protoParams *iotago.ProtocolParameters,
	msIndex iotago.MilestoneIndex,
	milestonePayload *iotago.Milestone,
	parentsTraverserInterface dag.ParentsTraverserInterface,
	cachedBlockFuncSource storage.CachedBlockFunc,
	storeBlockTarget StoreBlockInterface,
	milestoneManager *milestonemanager.MilestoneManager) error {

	// traversal stops if no more blocks pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		// collect all blocks that were referenced by that milestone
		referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex()

		if referenced {
			if at > msIndex {
				return false, fmt.Errorf("milestone cone inconsistent (msIndex: %d, referencedAt: %d)", msIndex, at)
			}

			if at < msIndex {
				// do not traverse blocks that were referenced by an older milestonee
				return false, nil
			}
		}
		blockID := cachedBlockMeta.Metadata().BlockID()
		cachedBlock, err := cachedBlockFuncSource(blockID) // block +1
		if err != nil {
			return false, err
		}
		if cachedBlock == nil {
			return false, fmt.Errorf("block not found: %s", blockID.ToHex())
		}
		defer cachedBlock.Release(true) // block -1

		// store the block in the target storage
		cachedBlockNew, err := storeBlock(protoParams, storeBlockTarget, milestoneManager, cachedBlock.Block().Block()) // block +1
		if err != nil {
			return false, err
		}
		defer cachedBlockNew.Release(true) // block -1

		cachedBlockMetaNew := cachedBlockNew.CachedMetadata() // meta +1
		defer cachedBlockMetaNew.Release(true)                // meta -1

		// we need to mark all blocks that contain a milestone payload,
		// but we can not trust the metadata of the parentsTraverserInterface for correct info about milestones
		// because it could be a proxystorage, which doesn't know the correct milestones yet.
		if cachedBlockNew.Block().IsMilestone() {
			milestonePayload := milestoneManager.VerifyMilestonePayload(cachedBlockNew.Block().Milestone())
			if milestonePayload != nil {
				cachedBlockMetaNew.Metadata().SetMilestone(true)
			}
		}

		// set the new block as solid
		cachedBlockMetaNew.Metadata().SetSolid(true)

		return true, nil
	}

	// traverse the milestone and collect all blocks that were referenced by this milestone or newer
	if err := parentsTraverserInterface.Traverse(
		ctx,
		milestonePayload.Parents,
		condition,
		nil,
		// called on missing parents
		// return error on missing parents
		nil,
		// called on solid entry points
		// Ignore solid entry points (snapshot milestone included)
		nil,
		false); err != nil {
		return err
	}

	return nil
}

type confStats struct {
	msIndex              iotago.MilestoneIndex
	blocksReferenced     int
	durationCopy         time.Duration
	durationConfirmation time.Duration
}

// copyAndVerifyMilestoneCone verifies the milestone, copies the milestone cone to the
// target storage, confirms the milestone and applies the ledger changes.
func copyAndVerifyMilestoneCone(
	ctx context.Context,
	protoParams *iotago.ProtocolParameters,
	genesisMilestoneIndex iotago.MilestoneIndex,
	msIndex iotago.MilestoneIndex,
	getMilestonePayload func(msIndex iotago.MilestoneIndex) (*iotago.Milestone, error),
	parentsTraverserInterfaceSource dag.ParentsTraverserInterface,
	cachedBlockFuncSource storage.CachedBlockFunc,
	cachedBlockFuncTarget storage.CachedBlockFunc,
	utxoManagerTarget *utxo.Manager,
	storeBlockTarget StoreBlockInterface,
	parentsTraverserStorageTarget dag.ParentsTraverserStorage,
	milestoneManager *milestonemanager.MilestoneManager) (*confStats, error) {

	if err := contextutils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		return nil, err
	}

	milestonePayloadUnverified, err := getMilestonePayload(msIndex)
	if err != nil {
		return nil, err
	}

	milestonePayload := milestoneManager.VerifyMilestonePayload(milestonePayloadUnverified)
	if milestonePayload == nil {
		return nil, fmt.Errorf("source milestone not valid! %d", msIndex)
	}

	ts := time.Now()

	if err := copyMilestoneCone(
		context.Background(), // we do not want abort the copying of the blocks itself
		protoParams,
		msIndex,
		milestonePayload,
		parentsTraverserInterfaceSource,
		cachedBlockFuncSource,
		storeBlockTarget,
		milestoneManager); err != nil {
		return nil, err
	}

	timeCopyMilestoneCone := time.Now()

	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		utxoManagerTarget,
		parentsTraverserStorageTarget,
		cachedBlockFuncTarget,
		protoParams,
		genesisMilestoneIndex,
		milestonePayload,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		nil,
		// Hint: Ledger is write locked
		nil,
		// Hint: Ledger is write locked
		nil,
		// Hint: Ledger is not locked
		nil,
		// Hint: Ledger is not locked
		nil,
		// Hint: Ledger is not locked
		nil,
	)
	if err != nil {
		return nil, err
	}

	timeConfirmMilestone := time.Now()

	return &confStats{
		msIndex:              confirmedMilestoneStats.Index,
		blocksReferenced:     confirmedMilestoneStats.BlocksReferenced,
		durationCopy:         timeCopyMilestoneCone.Sub(ts).Truncate(time.Millisecond),
		durationConfirmation: timeConfirmMilestone.Sub(timeCopyMilestoneCone).Truncate(time.Millisecond),
	}, nil
}

// mergeViaAPI copies a milestone from a remote node to the target database via API.
func mergeViaAPI(
	ctx context.Context,
	protoParams *iotago.ProtocolParameters,
	msIndex iotago.MilestoneIndex,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	client *nodeclient.Client,
	apiParallelism int) error {

	getBlockViaAPI := func(blockID iotago.BlockID) (*iotago.Block, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		block, err := client.BlockByBlockID(ctx, blockID, protoParams)
		if err != nil {
			return nil, err
		}

		return block, nil
	}

	getMilestonePayloadViaAPI := func(client *nodeclient.Client, msIndex iotago.MilestoneIndex) (*iotago.Milestone, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ms, err := client.MilestoneByIndex(ctx, msIndex)
		if err != nil {
			return nil, err
		}

		return ms, nil
	}

	if err := checkSnapshotInfo(storeTarget); err != nil {
		return err
	}
	snapshotInfoTarget := storeTarget.SnapshotInfo()

	proxyStorage, err := NewProxyStorage(protoParams, storeTarget, milestoneManager, getBlockViaAPI)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protoParams,
		snapshotInfoTarget.GenesisMilestoneIndex(),
		msIndex,
		func(msIndex iotago.MilestoneIndex) (*iotago.Milestone, error) {
			return getMilestonePayloadViaAPI(client, msIndex)
		},
		dag.NewConcurrentParentsTraverser(proxyStorage, apiParallelism),
		proxyStorage.CachedBlock,
		proxyStorage.CachedBlock,
		storeTarget.UTXOManager(),
		proxyStorage,
		proxyStorage,
		milestoneManager)
	if err != nil {
		return err
	}

	timeMergeStoragesStart := time.Now()

	if err := proxyStorage.MergeStorages(); err != nil {
		return fmt.Errorf("merge storages failed: %w", err)
	}

	te := time.Now()

	println(fmt.Sprintf("confirmed milestone %d, blocks: %d, duration copy: %v, duration conf.: %v, duration merge: %v, total: %v",
		confStats.msIndex,
		confStats.blocksReferenced,
		confStats.durationCopy,
		confStats.durationConfirmation,
		te.Sub(timeMergeStoragesStart).Truncate(time.Millisecond),
		te.Sub(ts).Truncate(time.Millisecond)))

	return nil
}

// mergeViaSourceDatabase copies a milestone from the source database to the target database.
func mergeViaSourceDatabase(
	ctx context.Context,
	protoParams *iotago.ProtocolParameters,
	msIndex iotago.MilestoneIndex,
	storeSource *storage.Storage,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	if err := checkSnapshotInfo(storeTarget); err != nil {
		return err
	}
	snapshotInfoTarget := storeTarget.SnapshotInfo()

	proxyStorage, err := NewProxyStorage(protoParams, storeTarget, milestoneManager, storeSource.Block)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protoParams,
		snapshotInfoTarget.GenesisMilestoneIndex(),
		msIndex,
		func(msIndex iotago.MilestoneIndex) (*iotago.Milestone, error) {
			return getMilestonePayloadFromStorage(storeSource, msIndex)
		},
		dag.NewConcurrentParentsTraverser(storeSource),
		storeSource.CachedBlock,
		proxyStorage.CachedBlock,
		storeTarget.UTXOManager(),
		proxyStorage,
		proxyStorage,
		milestoneManager)
	if err != nil {
		return err
	}

	timeMergeStoragesStart := time.Now()

	if err := proxyStorage.MergeStorages(); err != nil {
		return fmt.Errorf("merge storages failed: %w", err)
	}

	te := time.Now()

	println(fmt.Sprintf("confirmed milestone %d, blocks: %d, duration copy: %v, duration conf.: %v, duration merge: %v, total: %v",
		confStats.msIndex,
		confStats.blocksReferenced,
		confStats.durationCopy,
		confStats.durationConfirmation,
		te.Sub(timeMergeStoragesStart).Truncate(time.Millisecond),
		te.Sub(ts).Truncate(time.Millisecond)))

	return nil
}

// mergeDatabase copies milestone after milestone from source to target database.
// if a node client is given, missing history in the source database is fetched via API.
// if the target database has no history at all, a genesis snapshot is loaded.
func mergeDatabase(
	ctx context.Context,
	protoParams *iotago.ProtocolParameters,
	milestoneManager *milestonemanager.MilestoneManager,
	tangleStoreSource *storage.Storage,
	tangleStoreTarget *storage.Storage,
	client *nodeclient.Client,
	targetIndex iotago.MilestoneIndex,
	genesisSnapshotFilePath string,
	apiParallelism int) error {

	tangleStoreSourceAvailable := tangleStoreSource != nil

	if err := checkSnapshotInfo(tangleStoreTarget); err != nil {
		return err
	}
	snapshotInfoTarget := tangleStoreTarget.SnapshotInfo()

	var sourceNetworkID uint64
	var msIndexStartSource, msIndexEndSource iotago.MilestoneIndex = 0, 0
	msIndexStartTarget, msIndexEndTarget := getStorageMilestoneRange(tangleStoreTarget)
	if tangleStoreSourceAvailable {
		protoParamsSource, err := tangleStoreSource.CurrentProtocolParameters()
		if err != nil {
			return errors.Wrapf(ErrCritical, "loading source protocol parameters failed: %s", err.Error())
		}
		sourceNetworkID = protoParamsSource.NetworkID()
		msIndexStartSource, msIndexEndSource = getStorageMilestoneRange(tangleStoreSource)
	}

	if msIndexEndTarget == 0 {
		// no ledger state in database available => load the genesis snapshot
		println("loading genesis snapshot...")
		if err := loadGenesisSnapshot(tangleStoreTarget, genesisSnapshotFilePath, tangleStoreSourceAvailable, sourceNetworkID); err != nil {
			return errors.Wrapf(ErrCritical, "loading genesis snapshot failed: %s", err.Error())
		}

		// set the new start and end indexes after applying the genesis snapshot
		msIndexStartTarget, msIndexEndTarget = snapshotInfoTarget.EntryPointIndex(), snapshotInfoTarget.EntryPointIndex()
	}

	if tangleStoreSourceAvailable {
		println(fmt.Sprintf("milestone range in database: %d-%d (source)", msIndexStartSource, msIndexEndSource))

		// check network ID
		protoParamsTarget, err := tangleStoreTarget.CurrentProtocolParameters()
		if err != nil {
			return errors.Wrapf(ErrCritical, "loading target protocol parameters failed: %s", err.Error())
		}

		targetNetworkID := protoParamsTarget.NetworkID()
		if sourceNetworkID != targetNetworkID {
			return fmt.Errorf("source storage networkID not equal to target storage networkID (%d != %d)", sourceNetworkID, targetNetworkID)
		}
	}
	println(fmt.Sprintf("milestone range in database: %d-%d (target)", msIndexStartTarget, msIndexEndTarget))

	msIndexStart := msIndexEndTarget + 1
	msIndexEnd := msIndexEndSource

	if targetIndex != 0 {
		msIndexEnd = targetIndex
	}

	if msIndexEnd <= msIndexStart {
		return fmt.Errorf("%w (start index: %d, target index: %d)", ErrNoNewTangleData, msIndexStart, msIndexEnd)
	}

	indexAvailableInSource := func(msIndex iotago.MilestoneIndex) bool {
		return (msIndex >= msIndexStartSource) && (msIndex <= msIndexEndSource)
	}

	for msIndex := msIndexStart; msIndex <= msIndexEnd; msIndex++ {
		if !tangleStoreSourceAvailable || !indexAvailableInSource(msIndex) {
			if client == nil {
				return fmt.Errorf("history is missing (oldest source index: %d, target index: %d)", msIndexStartSource, msIndex)
			}

			print(fmt.Sprintf("get milestone %d via API... ", msIndex))
			if err := mergeViaAPI(
				ctx,
				protoParams,
				msIndex,
				tangleStoreTarget,
				milestoneManager,
				client,
				apiParallelism,
			); err != nil {
				return err
			}

			continue
		}

		print(fmt.Sprintf("get milestone %d via source database (source range: %d-%d)... ", msIndex, msIndexStartSource, msIndexEndSource))
		if err := mergeViaSourceDatabase(
			ctx,
			protoParams,
			msIndex,
			tangleStoreSource,
			tangleStoreTarget,
			milestoneManager,
		); err != nil {
			return err
		}
	}

	return nil
}

// getNodeHTTPAPIClient returns a node client.
// we don't need to check for the correct networkID,
// because the node would be missing the history if the
// network is not correct.
func getNodeHTTPAPIClient(nodeURL string) *nodeclient.Client {

	var client *nodeclient.Client
	if nodeURL != "" {
		client = nodeclient.New(nodeURL)
	}

	return client
}

type GetBlockFunc func(blockID iotago.BlockID) (*iotago.Block, error)

// ProxyStorage is used to temporarily store changes to an intermediate storage,
// which then can be merged with the target store in a single commit.
type ProxyStorage struct {
	protoParams      *iotago.ProtocolParameters
	storeTarget      *storage.Storage
	storeProxy       *storage.Storage
	milestoneManager *milestonemanager.MilestoneManager
	getBlockFunc     GetBlockFunc
}

func NewProxyStorage(
	protoParams *iotago.ProtocolParameters,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	getBlockFunc GetBlockFunc) (*ProxyStorage, error) {

	storeProxy, err := createTangleStorage("proxy", "", "", database.EngineMapDB)
	if err != nil {
		return nil, err
	}

	return &ProxyStorage{
		protoParams:      protoParams,
		storeTarget:      storeTarget,
		storeProxy:       storeProxy,
		milestoneManager: milestoneManager,
		getBlockFunc:     getBlockFunc,
	}, nil
}

// CachedBlock returns a cached block object.
// block +1.
func (s *ProxyStorage) CachedBlock(blockID iotago.BlockID) (*storage.CachedBlock, error) {
	if !s.storeTarget.ContainsBlock(blockID) {
		if !s.storeProxy.ContainsBlock(blockID) {
			block, err := s.getBlockFunc(blockID)
			if err != nil {
				return nil, err
			}

			cachedBlock, err := storeBlock(s.protoParams, s.storeProxy, s.milestoneManager, block) // block +1
			if err != nil {
				return nil, err
			}

			// set the new block as solid
			cachedBlockMeta := cachedBlock.CachedMetadata() // meta +1
			defer cachedBlockMeta.Release(true)             // meta -1

			cachedBlockMeta.Metadata().SetSolid(true)

			return cachedBlock, nil
		}

		return s.storeProxy.CachedBlock(blockID) // block +1
	}

	return s.storeTarget.CachedBlock(blockID) // block +1
}

// CachedBlockMetadata returns a cached block metadata object.
// meta +1.
func (s *ProxyStorage) CachedBlockMetadata(blockID iotago.BlockID) (*storage.CachedMetadata, error) {
	cachedBlock, err := s.CachedBlock(blockID) // block +1
	if err != nil {
		return nil, err
	}
	if cachedBlock == nil {
		//nolint:nilnil // nil, nil is ok in this context, even if it is not go idiomatic
		return nil, nil
	}
	defer cachedBlock.Release(true) // block -1

	return cachedBlock.CachedMetadata(), nil // meta +1
}

func (s *ProxyStorage) SolidEntryPointsContain(blockID iotago.BlockID) (bool, error) {
	return s.storeTarget.SolidEntryPointsContain(blockID)
}

func (s *ProxyStorage) SolidEntryPointsIndex(blockID iotago.BlockID) (iotago.MilestoneIndex, bool, error) {
	return s.storeTarget.SolidEntryPointsIndex(blockID)
}

func (s *ProxyStorage) MergeStorages() error {

	// first flush both storages
	s.storeProxy.FlushStorages()
	s.storeTarget.FlushStorages()

	// copy all existing keys with values from the proxy storage to the target storage
	return kvstore.CopyBatched(s.storeProxy.TangleStore(), s.storeTarget.TangleStore(), 10000)
}

// Cleanup shuts down, flushes and closes the proxy store.
func (s *ProxyStorage) Cleanup() {
	s.storeProxy.Shutdown()
}

// StoreBlockInterface

func (s *ProxyStorage) StoreBlockIfAbsent(block *storage.Block) (cachedBlock *storage.CachedBlock, newlyAdded bool) {
	return s.storeProxy.StoreBlockIfAbsent(block)
}

func (s *ProxyStorage) StoreChild(parentBlockID iotago.BlockID, childBlockID iotago.BlockID) *storage.CachedChild {
	return s.storeProxy.StoreChild(parentBlockID, childBlockID)
}

func (s *ProxyStorage) StoreMilestoneIfAbsent(milestone *iotago.Milestone, blockID iotago.BlockID) (*storage.CachedMilestone, bool) {
	return s.storeProxy.StoreMilestoneIfAbsent(milestone, blockID)
}
