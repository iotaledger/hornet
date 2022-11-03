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
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/common"
	"github.com/iotaledger/hornet/v2/pkg/dag"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
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
	databaseEngineSourceFlag := fs.String(FlagToolDatabaseEngineSource, string(hivedb.EngineAuto), "the engine of the source database (optional, values: pebble, rocksdb, auto)")
	databaseEngineTargetFlag := fs.String(FlagToolDatabaseEngineTarget, string(DefaultValueDatabaseEngine), "the engine of the target database (values: pebble, rocksdb)")
	targetIndexFlag := fs.Uint32(FlagToolDatabaseTargetIndex, 0, "the target index (optional)")
	nodeURLFlag := fs.String(FlagToolNodeURL, "", "URL of the node (optional)")
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
			hivedb.EnginePebble,
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
			return fmt.Errorf("either '%s' or '%s' must be specified", FlagToolDatabasePathSource, FlagToolNodeURL)
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

	var tangleStoreSource *storage.Storage
	if len(*databasePathSourceFlag) > 0 {
		var err error

		// we don't need to check the health of the source db.
		// it is fine as long as all blocks in the cone are found.
		tangleStoreSource, err = getTangleStorage(*databasePathSourceFlag, "source", *databaseEngineSourceFlag, true, false, false, true)
		if err != nil {
			return err
		}
		defer func() {
			println("\nshutdown source storage ...")
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
		println("\nshutdown target storage ...")
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
	if err := tangleStoreTarget.MarkStoresCorrupted(); err != nil {
		return err
	}

	ts := time.Now()
	if len(*databasePathSourceFlag) > 0 {
		println(fmt.Sprintf("merging databases ... (source: %s, target: %s)", *databasePathSourceFlag, *databasePathTargetFlag))
	} else {
		println(fmt.Sprintf("merging databases ... (nodeURL: %s, target: %s)", *nodeURLFlag, *databasePathTargetFlag))
	}

	errMerge := mergeDatabase(
		getGracefulStopContext(),
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
	if err := tangleStoreTarget.MarkStoresHealthy(); err != nil {
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
	protocolManager *protocol.Manager,
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

	// we need to store every milestone payload here as well,
	// otherwise the ledger changes might be applied,
	// but the corresponding milestone payload is unknown in the database.
	cachedMilestone, _ := storeBlockTarget.StoreMilestoneIfAbsent(milestonePayload) // milestone +1
	cachedMilestone.Release(true)                                                   // milestone -1

	ts := time.Now()

	//nolint:contextcheck // we don't want abort the copying of the blocks itself
	if err := copyMilestoneCone(
		context.Background(),
		protocolManager.Current(),
		msIndex,
		milestonePayload,
		parentsTraverserInterfaceSource,
		cachedBlockFuncSource,
		storeBlockTarget,
		milestoneManager); err != nil {
		return nil, err
	}

	timeCopyMilestoneCone := time.Now()

	//nolint:contextcheck // we don't pass a context here to not cancel the whiteflag computation!
	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		utxoManagerTarget,
		parentsTraverserStorageTarget,
		cachedBlockFuncTarget,
		protocolManager.Current(),
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

	// handle protocol parameter updates
	protocolManager.HandleConfirmedMilestone(milestonePayload)

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
	protocolManager *protocol.Manager,
	msIndex iotago.MilestoneIndex,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	client *nodeclient.Client,
	apiParallelism int) error {

	getBlockViaAPI := func(blockID iotago.BlockID) (*iotago.Block, error) {
		ctxBlock, cancelBlock := context.WithTimeout(ctx, 5*time.Second)
		defer cancelBlock()

		block, err := client.BlockByBlockID(ctxBlock, blockID, protocolManager.Current())
		if err != nil {
			return nil, err
		}

		return block, nil
	}

	getMilestonePayloadViaAPI := func(client *nodeclient.Client, msIndex iotago.MilestoneIndex) (*iotago.Milestone, error) {
		ctxMilestone, cancelMilestone := context.WithTimeout(ctx, 5*time.Second)
		defer cancelMilestone()

		ms, err := client.MilestoneByIndex(ctxMilestone, msIndex)
		if err != nil {
			return nil, err
		}

		return ms, nil
	}

	if err := checkSnapshotInfo(storeTarget); err != nil {
		return err
	}
	snapshotInfoTarget := storeTarget.SnapshotInfo()

	//nolint:contextcheck // false positive
	proxyStorage, err := NewProxyStorage(protocolManager.Current(), storeTarget, milestoneManager, getBlockViaAPI)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protocolManager,
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
	protocolManager *protocol.Manager,
	msIndex iotago.MilestoneIndex,
	storeSource *storage.Storage,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	if err := checkSnapshotInfo(storeTarget); err != nil {
		return err
	}
	snapshotInfoTarget := storeTarget.SnapshotInfo()

	//nolint:contextcheck // false positive
	proxyStorage, err := NewProxyStorage(protocolManager.Current(), storeTarget, milestoneManager, storeSource.Block)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protocolManager,
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
	milestoneManager *milestonemanager.MilestoneManager,
	tangleStoreSource *storage.Storage,
	tangleStoreTarget *storage.Storage,
	client *nodeclient.Client,
	targetIndex iotago.MilestoneIndex,
	genesisSnapshotFilePath string,
	apiParallelism int) error {

	tangleStoreSourceAvailable := tangleStoreSource != nil

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
		println("loading genesis snapshot ...")
		if err := loadGenesisSnapshot(ctx, tangleStoreTarget, genesisSnapshotFilePath, tangleStoreSourceAvailable, sourceNetworkID); err != nil {
			return errors.Wrapf(ErrCritical, "loading genesis snapshot failed: %s", err.Error())
		}

		if err := checkSnapshotInfo(tangleStoreTarget); err != nil {
			return err
		}
		snapshotInfoTarget := tangleStoreTarget.SnapshotInfo()

		// set the new start and end indexes after applying the genesis snapshot
		msIndexStartTarget, msIndexEndTarget = snapshotInfoTarget.EntryPointIndex(), snapshotInfoTarget.EntryPointIndex()
	} else {
		if err := checkSnapshotInfo(tangleStoreTarget); err != nil {
			return err
		}
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

	ledgerIndexTarget, err := tangleStoreTarget.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return errors.Wrapf(ErrCritical, "loading target ledger index failed: %s", err.Error())
	}

	protocolManagerTarget, err := protocol.NewManager(tangleStoreTarget, ledgerIndexTarget)
	if err != nil {
		return errors.Wrapf(ErrCritical, "initializing target protocol manager failed: %s", err.Error())
	}

	for msIndex := msIndexStart; msIndex <= msIndexEnd; msIndex++ {
		if !tangleStoreSourceAvailable || !indexAvailableInSource(msIndex) {
			if client == nil {
				return fmt.Errorf("history is missing (milestone range in source database: %d-%d, target index: %d)", msIndexStartSource, msIndexEndSource, msIndex)
			}

			print(fmt.Sprintf("get milestone %d via API ... ", msIndex))
			if err := mergeViaAPI(
				ctx,
				protocolManagerTarget,
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

		print(fmt.Sprintf("get milestone %d via source database (source range: %d-%d) ... ", msIndex, msIndexStartSource, msIndexEndSource))
		if err := mergeViaSourceDatabase(
			ctx,
			protocolManagerTarget,
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

	storeProxy, err := createTangleStorage("proxy", "", "", hivedb.EngineMapDB)
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
	_ = s.storeProxy.Shutdown()
}

// StoreBlockInterface

func (s *ProxyStorage) StoreBlockIfAbsent(block *storage.Block) (cachedBlock *storage.CachedBlock, newlyAdded bool) {
	return s.storeProxy.StoreBlockIfAbsent(block)
}

func (s *ProxyStorage) StoreChild(parentBlockID iotago.BlockID, childBlockID iotago.BlockID) *storage.CachedChild {
	return s.storeProxy.StoreChild(parentBlockID, childBlockID)
}

func (s *ProxyStorage) StoreMilestoneIfAbsent(milestone *iotago.Milestone) (*storage.CachedMilestone, bool) {
	return s.storeProxy.StoreMilestoneIfAbsent(milestone)
}
