package toolset

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/contextutils"
	"github.com/iotaledger/hive.go/kvstore"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/whiteflag"
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
	chronicleFlag := fs.Bool(FlagToolDatabaseMergeChronicle, false, "use chronicle compatibility mode for API sync")
	chronicleKeyspaceFlag := fs.String(FlagToolDatabaseMergeChronicleKeyspace, "mainnet", "key space for chronicle compatibility mode")
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
	if *chronicleFlag {
		if len(*nodeURLFlag) == 0 {
			return fmt.Errorf("'%s' not specified", FlagToolDatabaseMergeNodeURL)
		}
		if len(*chronicleKeyspaceFlag) == 0 {
			return fmt.Errorf("'%s' not specified", FlagToolDatabaseMergeChronicleKeyspace)
		}
	}

	// TODO: adapt to new protocol parameter logic
	protoParas := &iotago.ProtocolParameters{}

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
			tangleStoreSource.ShutdownStorages()
			tangleStoreSource.FlushAndCloseStores()
		}()
	}

	// we need to check the health of the target db, since we don't want use tainted/corrupted dbs.
	tangleStoreTarget, err := getTangleStorage(*databasePathTargetFlag, "target", *databaseEngineTargetFlag, false, true, true, false)
	if err != nil {
		return err
	}
	defer func() {
		println("\nshutdown storages...")
		tangleStoreTarget.ShutdownStorages()

		println("flush and close stores...")
		tangleStoreTarget.FlushAndCloseStores()
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

	client := getNodeHTTPAPIClient(*nodeURLFlag, *chronicleFlag, *chronicleKeyspaceFlag)

	// mark the database as corrupted.
	// this flag will be cleared after the operation finished successfully.
	if err := tangleStoreTarget.MarkDatabasesCorrupted(); err != nil {
		return err
	}

	ts := time.Now()
	println(fmt.Sprintf("merging databases... (source: %s, target: %s)", *databasePathSourceFlag, *databasePathTargetFlag))

	errMerge := mergeDatabase(
		getGracefulStopContext(),
		protoParas,
		milestoneManager,
		tangleStoreSource,
		tangleStoreTarget,
		client,
		milestone.Index(*targetIndexFlag),
		*genesisSnapshotFilePathFlag,
		*chronicleFlag,
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
	protoParas *iotago.ProtocolParameters,
	msIndex milestone.Index,
	milestonePayload *iotago.Milestone,
	parentsTraverserInterface dag.ParentsTraverserInterface,
	cachedBlockFuncSource storage.CachedBlockFunc,
	storeBlockTarget StoreBlockInterface,
	milestoneManager *milestonemanager.MilestoneManager) error {

	// traversal stops if no more blocks pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedBlockMeta.Release(true) // meta -1

		// collect all msgs that were referenced by that milestone
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

		cachedBlock, err := cachedBlockFuncSource(cachedBlockMeta.Metadata().BlockID()) // block +1
		if err != nil {
			return false, err
		}
		if cachedBlock == nil {
			return false, fmt.Errorf("block not found: %s", cachedBlockMeta.Metadata().BlockID().ToHex())
		}
		defer cachedBlock.Release(true) // block -1

		// store the block in the target storage
		cachedBlockNew, err := storeBlock(protoParas, storeBlockTarget, milestoneManager, cachedBlock.Block().Block()) // block +1
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
		hornet.BlockIDsFromSliceOfArrays(milestonePayload.Parents),
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
	msIndex              milestone.Index
	blocksReferenced     int
	durationCopy         time.Duration
	durationConfirmation time.Duration
}

// copyAndVerifyMilestoneCone verifies the milestone, copies the milestone cone to the
// target storage, confirms the milestone and applies the ledger changes.
func copyAndVerifyMilestoneCone(
	ctx context.Context,
	protoParas *iotago.ProtocolParameters,
	msIndex milestone.Index,
	getMilestonePayload func(msIndex milestone.Index) (*iotago.Milestone, error),
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
		protoParas,
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
		protoParas,
		milestonePayload,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		nil,
		nil,
		nil,
		nil,
		nil,
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
	protoParas *iotago.ProtocolParameters,
	msIndex milestone.Index,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	client *nodeclient.Client,
	chronicleMode bool,
	apiParallelism int) error {

	getBlockViaAPI := func(blockID hornet.BlockID) (*iotago.Block, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		msg, err := client.BlockByBlockID(ctx, blockID.ToArray(), protoParas)
		if err != nil {
			return nil, err
		}

		return msg, nil
	}

	getMilestonePayloadViaAPI := func(client *nodeclient.Client, msIndex milestone.Index) (*iotago.Milestone, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		milestone, err := client.MilestoneByIndex(ctx, uint32(msIndex))
		if err != nil {
			return nil, err
		}

		return milestone, nil
	}

	proxyStorage, err := NewProxyStorage(protoParas, storeTarget, milestoneManager, getBlockViaAPI)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protoParas,
		msIndex,
		func(msIndex milestone.Index) (*iotago.Milestone, error) {
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
	protoParas *iotago.ProtocolParameters,
	msIndex milestone.Index,
	storeSource *storage.Storage,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	proxyStorage, err := NewProxyStorage(protoParas, storeTarget, milestoneManager, storeSource.Block)
	if err != nil {
		return err
	}
	defer proxyStorage.Cleanup()

	ts := time.Now()

	confStats, err := copyAndVerifyMilestoneCone(
		ctx,
		protoParas,
		msIndex,
		func(msIndex milestone.Index) (*iotago.Milestone, error) {
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
	protoParas *iotago.ProtocolParameters,
	milestoneManager *milestonemanager.MilestoneManager,
	tangleStoreSource *storage.Storage,
	tangleStoreTarget *storage.Storage,
	client *nodeclient.Client,
	targetIndex milestone.Index,
	genesisSnapshotFilePath string,
	chronicleMode bool,
	apiParallelism int) error {

	tangleStoreSourceAvailable := tangleStoreSource != nil

	var sourceNetworkID uint64
	var msIndexStartSource, msIndexEndSource milestone.Index = 0, 0
	msIndexStartTarget, msIndexEndTarget := getStorageMilestoneRange(tangleStoreTarget)
	if tangleStoreSourceAvailable {
		sourceNetworkID = tangleStoreSource.SnapshotInfo().NetworkID
		msIndexStartSource, msIndexEndSource = getStorageMilestoneRange(tangleStoreSource)
	}

	if msIndexEndTarget == 0 {
		// no ledger state in database available => load the genesis snapshot
		println("loading genesis snapshot...")
		if err := loadGenesisSnapshot(tangleStoreTarget, genesisSnapshotFilePath, tangleStoreSourceAvailable, sourceNetworkID); err != nil {
			return errors.Wrapf(ErrCritical, "loading genesis snapshot failed: %s", err.Error())
		}

		// set the new start and end indexes after applying the genesis snapshot
		msIndexStartTarget, msIndexEndTarget = tangleStoreTarget.SnapshotInfo().EntryPointIndex, tangleStoreTarget.SnapshotInfo().EntryPointIndex
	}

	if tangleStoreSourceAvailable {
		println(fmt.Sprintf("milestone range in database: %d-%d (source)", msIndexStartSource, msIndexEndSource))
	}
	println(fmt.Sprintf("milestone range in database: %d-%d (target)", msIndexStartTarget, msIndexEndTarget))

	// check network ID
	targetNetworkID := tangleStoreTarget.SnapshotInfo().NetworkID
	if tangleStoreSourceAvailable && sourceNetworkID != targetNetworkID {
		return fmt.Errorf("source storage networkID not equal to target storage networkID (%d != %d)", sourceNetworkID, targetNetworkID)
	}

	msIndexStart := msIndexEndTarget + 1
	msIndexEnd := msIndexEndSource

	if targetIndex != 0 {
		msIndexEnd = targetIndex
	}

	if msIndexEnd <= msIndexStart {
		return fmt.Errorf("%w (start index: %d, target index: %d)", ErrNoNewTangleData, msIndexStart, msIndexEnd)
	}

	indexAvailableInSource := func(msIndex milestone.Index) bool {
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
				protoParas,
				msIndex,
				tangleStoreTarget,
				milestoneManager,
				client,
				chronicleMode,
				apiParallelism,
			); err != nil {
				return err
			}

			continue
		}

		print(fmt.Sprintf("get milestone %d via source database (source range: %d-%d)... ", msIndex, msIndexStartSource, msIndexEndSource))
		if err := mergeViaSourceDatabase(
			ctx,
			protoParas,
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
func getNodeHTTPAPIClient(nodeURL string, chronicleMode bool, chronicleKeyspace string) *nodeclient.Client {

	var client *nodeclient.Client
	if nodeURL != "" {
		var requestURLHook func(url string) string = nil
		if chronicleMode {
			requestURLHook = func(url string) string {
				return strings.Replace(url, fmt.Sprintf("api/%s/api/v2/", chronicleKeyspace), fmt.Sprintf("api/%s/", chronicleKeyspace), 1)
			}
		}
		client = nodeclient.New(nodeURL, nodeclient.WithRequestURLHook(requestURLHook))
	}

	return client
}

type GetBlockFunc func(blockID hornet.BlockID) (*iotago.Block, error)

// ProxyStorage is used to temporarily store changes to an intermediate storage,
// which then can be merged with the target store in a single commit.
type ProxyStorage struct {
	protoParas       *iotago.ProtocolParameters
	storeTarget      *storage.Storage
	storeProxy       *storage.Storage
	milestoneManager *milestonemanager.MilestoneManager
	getBlockFunc     GetBlockFunc
}

func NewProxyStorage(
	protoParas *iotago.ProtocolParameters,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	getBlockFunc GetBlockFunc) (*ProxyStorage, error) {

	storeProxy, err := createTangleStorage("proxy", "", "", database.EngineMapDB)
	if err != nil {
		return nil, err
	}

	return &ProxyStorage{
		protoParas:       protoParas,
		storeTarget:      storeTarget,
		storeProxy:       storeProxy,
		milestoneManager: milestoneManager,
		getBlockFunc:     getBlockFunc,
	}, nil
}

// block +1
func (s *ProxyStorage) CachedBlock(blockID hornet.BlockID) (*storage.CachedBlock, error) {
	if !s.storeTarget.ContainsBlock(blockID) {
		if !s.storeProxy.ContainsBlock(blockID) {
			msg, err := s.getBlockFunc(blockID)
			if err != nil {
				return nil, err
			}

			cachedBlock, err := storeBlock(s.protoParas, s.storeProxy, s.milestoneManager, msg) // block +1
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

// meta +1
func (s *ProxyStorage) CachedBlockMetadata(blockID hornet.BlockID) (*storage.CachedMetadata, error) {
	cachedBlock, err := s.CachedBlock(blockID) // block +1
	if err != nil {
		return nil, err
	}
	if cachedBlock == nil {
		return nil, nil
	}
	defer cachedBlock.Release(true)          // block -1
	return cachedBlock.CachedMetadata(), nil // meta +1
}

func (s *ProxyStorage) SolidEntryPointsContain(blockID hornet.BlockID) (bool, error) {
	return s.storeTarget.SolidEntryPointsContain(blockID)
}

func (s *ProxyStorage) SolidEntryPointsIndex(blockID hornet.BlockID) (milestone.Index, bool, error) {
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
	s.storeProxy.ShutdownStorages()
	s.storeProxy.FlushAndCloseStores()
}

// StoreBlockInterface
func (s *ProxyStorage) StoreBlockIfAbsent(block *storage.Block) (cachedBlock *storage.CachedBlock, newlyAdded bool) {
	return s.storeProxy.StoreBlockIfAbsent(block)
}

func (s *ProxyStorage) StoreChild(parentBlockID hornet.BlockID, childBlockID hornet.BlockID) *storage.CachedChild {
	return s.storeProxy.StoreChild(parentBlockID, childBlockID)
}

func (s *ProxyStorage) StoreMilestoneIfAbsent(milestone *iotago.Milestone, blockID hornet.BlockID) (*storage.CachedMilestone, bool) {
	return s.storeProxy.StoreMilestoneIfAbsent(milestone, blockID)
}
