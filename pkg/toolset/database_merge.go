package toolset

import (
	"context"
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	iotago "github.com/iotaledger/iota.go/v2"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

var (
	// ErrNoNewTangleData is returned when there is no new data in the source database.
	ErrNoNewTangleData = errors.New("no new tangle history available")
)

func databaseMerge(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	configFilePathFlag := fs.String(FlagToolConfigFilePath, "", "the path to the config file")
	genesisSnapshotFilePathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the genesis snapshot file (optional)")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	databasePathTargetFlag := fs.String(FlagToolDatabasePathTarget, "", "the path to the target database")
	databaseEngineSourceFlag := fs.String(FlagToolDatabaseEngineSource, string(DefaultValueDatabaseEngine), "the engine of the source database (values: pebble, rocksdb)")
	databaseEngineTargetFlag := fs.String(FlagToolDatabaseEngineTarget, string(DefaultValueDatabaseEngine), "the engine of the target database (values: pebble, rocksdb)")
	targetIndexFlag := fs.Uint32("targetIndex", 0, "the target index (optional)")
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
			"mainnetdb",
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

	var tangleStoreSource *storage.Storage = nil
	if len(*databasePathSourceFlag) > 0 {
		var err error

		// we don't need to check the health of the source db.
		// it is fine as long as all messages in the cone are found.
		tangleStoreSource, err = getTangleStorage(*databasePathSourceFlag, "source", *databaseEngineSourceFlag, true, true, false, false, true)
		if err != nil {
			return err
		}
	}

	// we need to check the health of the target db, since we don't want use tainted/corrupted dbs.
	tangleStoreTarget, err := getTangleStorage(*databasePathTargetFlag, "target", *databaseEngineTargetFlag, false, false, true, true, false)
	if err != nil {
		return err
	}

	defer func() {
		println("\nshutdown storages...")
		if tangleStoreSource != nil {
			tangleStoreSource.ShutdownStorages()
		}
		tangleStoreTarget.ShutdownStorages()

		println("flush and close stores...")
		if tangleStoreSource != nil {
			tangleStoreSource.FlushAndCloseStores()
		}
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

	ctx, cancel := context.WithCancel(context.Background())

	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop
		cancel()
	}()

	// mark the database as corrupted.
	// this flag will be cleared after the operation finished successfully.
	if err := tangleStoreTarget.MarkDatabasesCorrupted(); err != nil {
		return err
	}

	ts := time.Now()
	println(fmt.Sprintf("merging databases... (source: %s, target: %s)", *databasePathSourceFlag, *databasePathTargetFlag))

	if err := mergeDatabase(
		ctx,
		milestoneManager,
		tangleStoreSource,
		tangleStoreTarget,
		client,
		milestone.Index(*targetIndexFlag),
		*genesisSnapshotFilePathFlag,
		*chronicleFlag,
		int(*apiParallelismFlag),
	); err != nil && !errors.Is(err, common.ErrOperationAborted) && !errors.Is(err, ErrNoNewTangleData) {
		// ignore errors due to node shutdown
		return err
	}

	// mark clean shutdown of the database
	if err := tangleStoreTarget.MarkDatabasesHealthy(); err != nil {
		return err
	}

	msIndexStart, msIndexEnd := getStorageMilestoneRange(tangleStoreTarget)
	println(fmt.Sprintf("\nsuccessfully merged %d milestones, took: %v", msIndexEnd-msIndexEndTarget, time.Since(ts).Truncate(time.Millisecond)))
	println(fmt.Sprintf("milestone range in database: %d-%d (target)", msIndexStart, msIndexEnd))

	return nil
}

// copyMilestoneCone copies all messages of a milestone cone to the target storage.
func copyMilestoneCone(ctx context.Context,
	msIndex milestone.Index,
	milestoneMessageID hornet.MessageID,
	parentsTraverserInterface dag.ParentsTraverserInterface,
	cachedMessageFuncSource storage.CachedMessageFunc,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	// traversal stops if no more messages pass the given condition
	// Caution: condition func is not in DFS order
	condition := func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
		defer cachedMsgMeta.Release(true) // meta -1

		// collect all msgs that were referenced by that milestone
		referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()

		if referenced {
			if at > msIndex {
				return false, fmt.Errorf("milestone cone inconsistent (msIndex: %d, referencedAt: %d)", msIndex, at)
			}

			if at < msIndex {
				// do not traverse messages that were referenced by an older milestonee
				return false, nil
			}
		}

		cachedMsg, err := cachedMessageFuncSource(cachedMsgMeta.Metadata().MessageID()) // message +1
		if err != nil {
			return false, err
		}
		if cachedMsg == nil {
			return false, fmt.Errorf("message not found: %s", cachedMsgMeta.Metadata().MessageID().ToHex())
		}
		defer cachedMsg.Release(true) // message -1

		// store the message in the target storage
		cachedMsgNew, err := storeMessage(storeTarget, milestoneManager, cachedMsg.Message().Message()) // message +1
		if err != nil {
			return false, err
		}
		defer cachedMsgNew.Release(true) // message -1

		return true, nil
	}

	// traverse the milestone and collect all messages that were referenced by this milestone or newer
	if err := parentsTraverserInterface.Traverse(
		ctx,
		hornet.MessageIDs{milestoneMessageID},
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

// copyAndVerifyMilestoneCone verifies the milestone, copies the milestone cone to the
// target storage, confirms the milestone and applies the ledger changes.
func copyAndVerifyMilestoneCone(
	ctx context.Context,
	msIndex milestone.Index,
	getMilestoneAndMessageID func(msIndex milestone.Index) (*storage.Message, hornet.MessageID, error),
	parentsTraverserInterfaceSource dag.ParentsTraverserInterface,
	cachedMessageFuncSource storage.CachedMessageFunc,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	if err := utils.ReturnErrIfCtxDone(ctx, common.ErrOperationAborted); err != nil {
		return err
	}

	msMsg, milestoneMessageID, err := getMilestoneAndMessageID(msIndex)
	if err != nil {
		return err
	}

	if ms := milestoneManager.VerifyMilestone(msMsg); ms == nil {
		return fmt.Errorf("source milestone not valid! %d", msIndex)
	}

	ts := time.Now()

	if err := copyMilestoneCone(
		context.Background(), // we do not want abort the copying of the messages itself
		msIndex,
		milestoneMessageID,
		parentsTraverserInterfaceSource,
		cachedMessageFuncSource,
		storeTarget,
		milestoneManager); err != nil {
		return err
	}

	timeCopyMilestoneCone := time.Now()

	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		storeTarget.UTXOManager(),
		storeTarget,
		storeTarget.CachedMessage,
		milestoneMessageID,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	timeConfirmMilestone := time.Now()
	println(fmt.Sprintf("confirmed milestone %d, messages: %d, duration copy: %v, duration conf.: %v, total: %v",
		confirmedMilestoneStats.Index,
		confirmedMilestoneStats.MessagesReferenced,
		timeCopyMilestoneCone.Sub(ts).Truncate(time.Millisecond),
		timeConfirmMilestone.Sub(timeCopyMilestoneCone).Truncate(time.Millisecond),
		timeConfirmMilestone.Sub(ts).Truncate(time.Millisecond)))
	return nil
}

// mergeViaAPI copies a milestone from a remote node to the target database via API.
func mergeViaAPI(
	ctx context.Context,
	msIndex milestone.Index,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	client *iotago.NodeHTTPAPIClient,
	chronicleMode bool,
	apiParallelism int) error {

	getMessageViaAPI := func(client *iotago.NodeHTTPAPIClient, messageID hornet.MessageID) (*iotago.Message, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		var msg *iotago.Message
		if !chronicleMode {
			msg, err = client.MessageByMessageID(ctx, messageID.ToArray())
		} else {
			msg, err = client.MessageJSONByMessageID(ctx, messageID.ToArray())
		}
		if err != nil {
			return nil, err
		}

		return msg, nil
	}

	getMilestoneAndMessageIDViaAPI := func(client *iotago.NodeHTTPAPIClient, getCachedMessageViaAPI storage.CachedMessageFunc, msIndex milestone.Index) (*storage.Message, hornet.MessageID, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		ms, err := client.MilestoneByIndex(ctx, uint32(msIndex))
		if err != nil {
			return nil, nil, err
		}

		messageID, err := hornet.MessageIDFromHex(ms.MessageID)
		if err != nil {
			return nil, nil, err
		}

		cachedMsg, err := getCachedMessageViaAPI(messageID) // message +1
		if err != nil {
			return nil, nil, err
		}
		if cachedMsg == nil {
			return nil, nil, fmt.Errorf("message not found: %s", messageID.ToHex())
		}
		defer cachedMsg.Release(true) // message -1

		return cachedMsg.Message(), cachedMsg.Message().MessageID(), nil
	}

	parentsTraverserStorageAPI := NewAPIStorage(storeTarget, milestoneManager, client, getMessageViaAPI)

	if err := copyAndVerifyMilestoneCone(
		ctx,
		msIndex,
		func(msIndex milestone.Index) (*storage.Message, hornet.MessageID, error) {
			return getMilestoneAndMessageIDViaAPI(client, parentsTraverserStorageAPI.CachedMessage, msIndex)
		},
		dag.NewConcurrentParentsTraverser(parentsTraverserStorageAPI, apiParallelism),
		parentsTraverserStorageAPI.CachedMessage,
		storeTarget,
		milestoneManager); err != nil {
		return err
	}

	return nil
}

// mergeViaSourceDatabase copies a milestone from the source database to the target database.
func mergeViaSourceDatabase(
	ctx context.Context,
	msIndex milestone.Index,
	storeSource *storage.Storage,
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager) error {

	return copyAndVerifyMilestoneCone(
		ctx,
		msIndex,
		func(msIndex milestone.Index) (*storage.Message, hornet.MessageID, error) {
			milestoneMessageID, err := getMilestoneMessageIDFromStorage(storeSource, msIndex)
			if err != nil {
				return nil, nil, err
			}

			msMsg, err := getMilestoneMessageFromStorage(storeSource, milestoneMessageID)
			if err != nil {
				return nil, nil, err
			}

			return msMsg, milestoneMessageID, nil
		},
		dag.NewConcurrentParentsTraverser(storeSource),
		storeSource.CachedMessage,
		storeTarget,
		milestoneManager)
}

// mergeDatabase copies milestone after milestone from source to target database.
// if a node client is given, missing history in the source database is fetched via API.
// if the target database has no history at all, a genesis snapshot is loaded.
func mergeDatabase(
	ctx context.Context,
	milestoneManager *milestonemanager.MilestoneManager,
	tangleStoreSource *storage.Storage,
	tangleStoreTarget *storage.Storage,
	client *iotago.NodeHTTPAPIClient,
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
			return fmt.Errorf("loading genesis snapshot failed: %w", err)
		}

		// set the new start and end indexes after applying the genesis snapshot
		msIndexStartTarget, msIndexEndTarget = tangleStoreTarget.SnapshotInfo().EntryPointIndex, tangleStoreTarget.SnapshotInfo().EntryPointIndex
	}

	println(fmt.Sprintf("milestone range in database: %d-%d (source)", msIndexStartSource, msIndexEndSource))
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
func getNodeHTTPAPIClient(nodeURL string, chronicleMode bool, chronicleKeyspace string) *iotago.NodeHTTPAPIClient {

	var client *iotago.NodeHTTPAPIClient
	if nodeURL != "" {
		var requestURLHook func(url string) string = nil
		if chronicleMode {
			requestURLHook = func(url string) string {
				return strings.Replace(url, fmt.Sprintf("api/%s/api/v1/", chronicleKeyspace), fmt.Sprintf("api/%s/", chronicleKeyspace), 1)
			}
		}
		client = iotago.NewNodeHTTPAPIClient(nodeURL, iotago.WithNodeHTTPAPIClientRequestURLHook(requestURLHook))
	}

	return client
}

type GetMessageViaAPIFunc func(client *iotago.NodeHTTPAPIClient, messageID hornet.MessageID) (*iotago.Message, error)

// APIStorage is used to get messages via remote node API
// if they do not exist in the target storage already.
type APIStorage struct {
	storeTarget          *storage.Storage
	milestoneManager     *milestonemanager.MilestoneManager
	client               *iotago.NodeHTTPAPIClient
	getMessageViaAPIFunc GetMessageViaAPIFunc
}

func NewAPIStorage(
	storeTarget *storage.Storage,
	milestoneManager *milestonemanager.MilestoneManager,
	client *iotago.NodeHTTPAPIClient,
	getMessageViaAPIFunc GetMessageViaAPIFunc) *APIStorage {

	return &APIStorage{
		storeTarget:          storeTarget,
		milestoneManager:     milestoneManager,
		client:               client,
		getMessageViaAPIFunc: getMessageViaAPIFunc,
	}
}

// message +1
func (s *APIStorage) CachedMessage(messageID hornet.MessageID) (*storage.CachedMessage, error) {
	if !s.storeTarget.ContainsMessage(messageID) {
		msg, err := s.getMessageViaAPIFunc(s.client, messageID)
		if err != nil {
			return nil, err
		}

		// store the message in the target storage
		// Caution: this may not be the correct place here, but this way we avoid requesting
		//          messages multiple times during the traversal of the milestone cone.
		//			the message is requested via API because it would get stored anyway.
		cachedMsg, err := storeMessage(s.storeTarget, s.milestoneManager, msg) // message +1
		if err != nil {
			return nil, err
		}

		return cachedMsg, nil
	}
	return s.storeTarget.CachedMessage(messageID) // message +1
}

// meta +1
func (s *APIStorage) CachedMessageMetadata(messageID hornet.MessageID) (*storage.CachedMetadata, error) {
	cachedMsg, err := s.CachedMessage(messageID) // message +1
	if err != nil {
		return nil, err
	}
	if cachedMsg == nil {
		return nil, nil
	}
	defer cachedMsg.Release(true)          // message -1
	return cachedMsg.CachedMetadata(), nil // meta +1
}

func (s *APIStorage) SolidEntryPointsContain(messageID hornet.MessageID) (bool, error) {
	return s.storeTarget.SolidEntryPointsContain(messageID)
}

func (s *APIStorage) SolidEntryPointsIndex(messageID hornet.MessageID) (milestone.Index, bool, error) {
	return s.storeTarget.SolidEntryPointsIndex(messageID)
}
