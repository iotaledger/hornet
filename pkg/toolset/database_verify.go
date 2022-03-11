package toolset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	iotago "github.com/iotaledger/iota.go/v3"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

func databaseVerify(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	configFilePathFlag := fs.String(FlagToolConfigFilePath, "", "the path to the config file")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	genesisSnapshotFilePathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the genesis snapshot file")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseVerify)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s",
			ToolDatabaseVerify,
			FlagToolConfigFilePath,
			"config.json",
			FlagToolDatabasePathSource,
			DefaultValueMainnetDatabasePath,
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
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePathSource)
	}
	if len(*genesisSnapshotFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPath)
	}

	// we don't need to check the health of the source db.
	// it is fine as long as all messages in the cone are found.
	tangleStoreSource, err := getTangleStorage(*databasePathSourceFlag, "source", string(database.EngineAuto), true, true, false, false, true)
	if err != nil {
		return err
	}

	defer func() {
		println("shutdown storages...")
		tangleStoreSource.ShutdownStorages()

		println("flush and close stores...")
		tangleStoreSource.FlushAndCloseStores()
	}()

	milestoneManager, err := getMilestoneManagerFromConfigFile(*configFilePathFlag)
	if err != nil {
		return err
	}

	ts := time.Now()
	println(fmt.Sprintf("verifying source database... (path: %s)", *databasePathSourceFlag))

	if err := verifyDatabase(
		getGracefulStopContext(),
		milestoneManager,
		tangleStoreSource,
		*genesisSnapshotFilePathFlag,
	); err != nil {
		return err
	}

	msIndexStart, msIndexEnd := getStorageMilestoneRange(tangleStoreSource)
	println(fmt.Sprintf("\nsuccessfully verified %d milestones, took: %v", msIndexEnd-msIndexStart, time.Since(ts).Truncate(time.Millisecond)))
	println(fmt.Sprintf("milestone range in database: %d-%d (source)", msIndexStart, msIndexEnd))

	return nil
}

// verifyDatabase checks if all messages in the cones of the existing milestones in the database are found.
func verifyDatabase(
	ctx context.Context,
	milestoneManager *milestonemanager.MilestoneManager,
	tangleStoreSource *storage.Storage,
	genesisSnapshotFilePath string) error {

	msIndexStart, msIndexEnd := getStorageMilestoneRange(tangleStoreSource)
	if msIndexStart == msIndexEnd {
		return fmt.Errorf("no source database entries %d-%d", msIndexStart, msIndexEnd)
	}

	println(fmt.Sprintf("existing milestone range source database: %d-%d", msIndexStart, msIndexEnd))

	tangleStoreTemp, err := createTangleStorage("temp", "", "", database.EngineMapDB)
	if err != nil {
		return err
	}

	// load the genesis ledger state into the temporary storage (SEP and ledger state only)
	println("loading genesis snapshot...")
	if err := loadGenesisSnapshot(tangleStoreTemp, genesisSnapshotFilePath, true, tangleStoreSource.SnapshotInfo().NetworkID); err != nil {
		return fmt.Errorf("loading genesis snapshot failed: %w", err)
	}

	if err := checkSnapshotInfo(tangleStoreTemp); err != nil {
		return err
	}

	// compare source database index and genesis snapshot index
	if tangleStoreSource.SnapshotInfo().EntryPointIndex != tangleStoreTemp.SnapshotInfo().EntryPointIndex {
		return fmt.Errorf("entry point index does not match genesis snapshot index: (%d != %d)", tangleStoreSource.SnapshotInfo().EntryPointIndex, tangleStoreTemp.SnapshotInfo().EntryPointIndex)
	}

	// compare solid entry points in source database and genesis snapshot
	if err := compareSolidEntryPoints(tangleStoreSource, tangleStoreTemp); err != nil {
		return nil
	}

	// checkMilestoneCone checks if all messages in the current milestone cone are found.
	checkMilestoneCone := func(
		ctx context.Context,
		cachedMessageFunc storage.CachedMessageFunc,
		milestoneManager *milestonemanager.MilestoneManager,
		onNewMilestoneConeMsg func(*storage.CachedMessage),
		msIndex milestone.Index) error {

		// traversal stops if no more messages pass the given condition
		// Caution: condition func is not in DFS order
		condition := func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedMsgMeta.Release(true) // meta -1

			// collect all msgs that were referenced by that milestone
			referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()

			if !referenced {
				// all existing messages in the database must be referenced by a milestone
				return false, fmt.Errorf("message was not referenced (msIndex: %d, msgID: %s)", msIndex, cachedMsgMeta.Metadata().MessageID().ToHex())
			}

			if at > msIndex {
				return false, fmt.Errorf("milestone cone inconsistent (msIndex: %d, referencedAt: %d)", msIndex, at)
			}

			if at < msIndex {
				// do not traverse messages that were referenced by an older milestonee
				return false, nil
			}

			// check if the message exists
			cachedMsg, err := cachedMessageFunc(cachedMsgMeta.Metadata().MessageID()) // message +1
			if err != nil {
				return false, err
			}
			if cachedMsg == nil {
				return false, fmt.Errorf("message not found: %s", cachedMsgMeta.Metadata().MessageID().ToHex())
			}
			defer cachedMsg.Release(true) // message -1

			if onNewMilestoneConeMsg != nil {
				onNewMilestoneConeMsg(cachedMsg.Retain()) // message pass +1
			}

			return true, nil
		}

		parentsTraverser := dag.NewConcurrentParentsTraverser(tangleStoreSource)

		milestoneMessageID, err := getMilestoneMessageIDFromStorage(tangleStoreSource, msIndex)
		if err != nil {
			return err
		}

		// traverse the milestone and collect all messages that were referenced by this milestone or newer
		if err := parentsTraverser.Traverse(
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

	applyAndCompareLedgerStateChange := func(
		ctx context.Context,
		storeSource *storage.Storage,
		utxoManagerTemp *utxo.Manager,
		msIndex milestone.Index) error {

		milestoneMessageID, err := getMilestoneMessageIDFromStorage(storeSource, msIndex)
		if err != nil {
			return err
		}

		referencedMessages := make(map[string]struct{})

		// confirm the milestone with the help of a special walker condition.
		// we re-confirm the existing milestones in the source database, but apply the
		// ledger changes to the temporary UTXOManager.
		_, _, err = whiteflag.ConfirmMilestone(
			utxoManagerTemp,
			storeSource,
			storeSource.CachedMessage,
			storeSource.SnapshotInfo().NetworkID,
			milestoneMessageID,
			// traversal stops if no more messages pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedMsgMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedMsgMeta.Release(true) // meta -1

				// collect all msgs that were referenced by that milestone
				referenced, at := cachedMsgMeta.Metadata().ReferencedWithIndex()
				return referenced && at == msIndex, nil
			},
			func(meta *storage.MessageMetadata) bool {
				referenced, at := meta.ReferencedWithIndex()
				if referenced && at == msIndex {
					_, exists := referencedMessages[meta.MessageID().ToMapKey()]
					return exists
				}

				return meta.IsReferenced()
			},
			func(meta *storage.MessageMetadata, referenced bool, msIndex milestone.Index) {
				if _, exists := referencedMessages[meta.MessageID().ToMapKey()]; !exists {
					referencedMessages[meta.MessageID().ToMapKey()] = struct{}{}
					meta.SetReferenced(referenced, msIndex)
				}
			},
			nil,
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

		// compare the temporary results of the ledger state changes of this milestone with the source database
		if err := compareMilestoneDiff(storeSource.UTXOManager(), utxoManagerTemp, msIndex); err != nil {
			return err
		}

		msMsg, err := getMilestoneMessageFromStorage(storeSource, milestoneMessageID)
		if err != nil {
			return err
		}

		// cleanup the state changes from the temporary UTXOManager to save memory
		if err := cleanupMilestoneFromUTXOManager(utxoManagerTemp, msMsg, msIndex); err != nil {
			return err
		}

		return nil
	}

	for msIndex := msIndexStart; msIndex <= msIndexEnd; msIndex++ {
		msgsCount := 0

		ts := time.Now()

		if err := checkMilestoneCone(
			ctx,
			tangleStoreSource.CachedMessage,
			milestoneManager,
			func(cachedMsg *storage.CachedMessage) {
				defer cachedMsg.Release(true) // message -1
				msgsCount++
			}, msIndex); err != nil {
			return err
		}

		if err := applyAndCompareLedgerStateChange(
			ctx,
			tangleStoreSource,
			tangleStoreTemp.UTXOManager(),
			msIndex); err != nil {
			return err
		}

		println(fmt.Sprintf("successfully verified milestone cone %d, msgs: %d, total: %v", msIndex, msgsCount, time.Since(ts).Truncate(time.Millisecond)))
	}

	println("verifying final ledger state...")
	if err := compareLedgerState(tangleStoreSource.UTXOManager(), tangleStoreTemp.UTXOManager()); err != nil {
		return err
	}

	return nil
}

func getSolidEntryPointsSHA256Sum(dbStorage *storage.Storage) ([]byte, error) {

	if dbStorage.SolidEntryPoints() == nil {
		return nil, errors.New("solid entry points not initialized")
	}

	return dbStorage.SolidEntryPoints().SHA256Sum()
}

func compareSolidEntryPoints(tangleStoreSource *storage.Storage, tangleStoreTemp *storage.Storage) error {

	sepSHA256Source, err := getSolidEntryPointsSHA256Sum(tangleStoreSource)
	if err != nil {
		return err
	}
	sepSHA256Temp, err := getSolidEntryPointsSHA256Sum(tangleStoreTemp)
	if err != nil {
		return err
	}
	if !bytes.Equal(sepSHA256Source, sepSHA256Temp) {
		return errors.New("solid entry points of source database and genesis snapshot do not match")
	}

	return nil
}

func getMilestoneDiffSHA256Sum(utxoManager *utxo.Manager, msIndex milestone.Index) ([]byte, error) {

	msDiff, err := utxoManager.MilestoneDiff(msIndex)
	if err != nil {
		return nil, err
	}

	return msDiff.SHA256Sum()
}

func compareMilestoneDiff(utxoManagerSource *utxo.Manager, utxoManagerTemp *utxo.Manager, msIndex milestone.Index) error {

	msDiffSHA256Source, err := getMilestoneDiffSHA256Sum(utxoManagerSource, msIndex)
	if err != nil {
		return err
	}
	msDiffSHA256Temp, err := getMilestoneDiffSHA256Sum(utxoManagerTemp, msIndex)
	if err != nil {
		return err
	}
	if !bytes.Equal(msDiffSHA256Source, msDiffSHA256Temp) {
		return errors.New("milestone diff of source database and temp database do not match")
	}

	return nil
}

func compareLedgerState(utxoManagerSource *utxo.Manager, utxoManagerTemp *utxo.Manager) error {

	ledgerStateSource, err := utxoManagerSource.LedgerStateSHA256Sum()
	if err != nil {
		return err
	}
	ledgerStateTemp, err := utxoManagerSource.LedgerStateSHA256Sum()
	if err != nil {
		return err
	}
	if !bytes.Equal(ledgerStateSource, ledgerStateTemp) {
		return errors.New("ledger state of source database and temp database does not match")
	}

	return nil
}

func cleanupMilestoneFromUTXOManager(utxoManager *utxo.Manager, msMsg *storage.Message, msIndex milestone.Index) error {

	var receiptMigratedAtIndex []uint32
	if r, ok := msMsg.Milestone().Receipt.(*iotago.Receipt); ok {
		receiptMigratedAtIndex = append(receiptMigratedAtIndex, r.MigratedAt)
	}

	if err := utxoManager.PruneMilestoneIndexWithoutLocking(msIndex, true, receiptMigratedAtIndex...); err != nil {
		return err
	}

	return nil
}
