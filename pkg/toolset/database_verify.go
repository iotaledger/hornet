package toolset

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/database"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
)

func databaseVerify(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	configFilePathFlag := fs.String(FlagToolConfigFilePath, "", "the path to the config file")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	genesisSnapshotFilePathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the genesis snapshot file")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseVerify)
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

	// TODO: adapt to new protocol parameter logic
	protoParas := &iotago.ProtocolParameters{}

	// we don't need to check the health of the source db.
	// it is fine as long as all blocks in the cone are found.
	tangleStoreSource, err := getTangleStorage(*databasePathSourceFlag, "source", string(database.EngineAuto), true, false, false, true)
	if err != nil {
		return err
	}
	defer func() {
		tangleStoreSource.ShutdownStorages()
		if err := tangleStoreSource.FlushAndCloseStores(); err != nil {
			panic(err)
		}
	}()

	milestoneManager, err := getMilestoneManagerFromConfigFile(*configFilePathFlag)
	if err != nil {
		return err
	}

	ts := time.Now()
	println(fmt.Sprintf("verifying source database... (path: %s)", *databasePathSourceFlag))

	if err := verifyDatabase(
		getGracefulStopContext(),
		protoParas,
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

// verifyDatabase checks if all blocks in the cones of the existing milestones in the database are found.
func verifyDatabase(
	ctx context.Context,
	protoParas *iotago.ProtocolParameters,
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
	defer func() {
		tangleStoreTemp.ShutdownStorages()
		if err := tangleStoreTemp.FlushAndCloseStores(); err != nil {
			panic(err)
		}
	}()

	// load the genesis ledger state into the temporary storage (SEP and ledger state only)
	println("loading genesis snapshot...")
	if err := loadGenesisSnapshot(tangleStoreTemp, genesisSnapshotFilePath, protoParas, true, tangleStoreSource.SnapshotInfo().NetworkID); err != nil {
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

	// checkMilestoneCone checks if all blocks in the current milestone cone are found.
	checkMilestoneCone := func(
		ctx context.Context,
		cachedBlockFunc storage.CachedBlockFunc,
		milestoneManager *milestonemanager.MilestoneManager,
		onNewMilestoneConeBlock func(*storage.CachedBlock),
		msIndex milestone.Index) error {

		// traversal stops if no more blocks pass the given condition
		// Caution: condition func is not in DFS order
		condition := func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
			defer cachedBlockMeta.Release(true) // meta -1

			// collect all blocks that were referenced by that milestone
			referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex()

			blockID := cachedBlockMeta.Metadata().BlockID()

			if !referenced {
				// all existing blocks in the database must be referenced by a milestone
				return false, fmt.Errorf("block was not referenced (msIndex: %d, blockID: %s)", msIndex, blockID.ToHex())
			}

			if at > msIndex {
				return false, fmt.Errorf("milestone cone inconsistent (msIndex: %d, referencedAt: %d)", msIndex, at)
			}

			if at < msIndex {
				// do not traverse blocks that were referenced by an older milestonee
				return false, nil
			}

			// check if the block exists
			cachedBlock, err := cachedBlockFunc(blockID) // block +1
			if err != nil {
				return false, err
			}
			if cachedBlock == nil {
				return false, fmt.Errorf("block not found: %s", blockID.ToHex())
			}
			defer cachedBlock.Release(true) // block -1

			if onNewMilestoneConeBlock != nil {
				onNewMilestoneConeBlock(cachedBlock.Retain()) // block pass +1
			}

			return true, nil
		}

		parentsTraverser := dag.NewConcurrentParentsTraverser(tangleStoreSource)

		milestonePayload, err := getMilestonePayloadFromStorage(tangleStoreSource, msIndex)
		if err != nil {
			return err
		}

		// traverse the milestone and collect all blocks that were referenced by this milestone or newer
		if err := parentsTraverser.Traverse(
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

	applyAndCompareLedgerStateChange := func(
		ctx context.Context,
		storeSource *storage.Storage,
		utxoManagerTemp *utxo.Manager,
		msIndex milestone.Index) error {

		milestonePayload, err := getMilestonePayloadFromStorage(storeSource, msIndex)
		if err != nil {
			return err
		}

		referencedBlocks := make(map[iotago.BlockID]struct{})

		// confirm the milestone with the help of a special walker condition.
		// we re-confirm the existing milestones in the source database, but apply the
		// ledger changes to the temporary UTXOManager.
		_, _, err = whiteflag.ConfirmMilestone(
			utxoManagerTemp,
			storeSource,
			storeSource.CachedBlock,
			protoParas,
			milestonePayload,
			// traversal stops if no more blocks pass the given condition
			// Caution: condition func is not in DFS order
			func(cachedBlockMeta *storage.CachedMetadata) (bool, error) { // meta +1
				defer cachedBlockMeta.Release(true) // meta -1

				// collect all blocks that were referenced by that milestone
				referenced, at := cachedBlockMeta.Metadata().ReferencedWithIndex()
				return referenced && at == msIndex, nil
			},
			func(meta *storage.BlockMetadata) bool {
				referenced, at := meta.ReferencedWithIndex()
				if referenced && at == msIndex {
					_, exists := referencedBlocks[meta.BlockID()]
					return exists
				}

				return meta.IsReferenced()
			},
			func(meta *storage.BlockMetadata, referenced bool, msIndex milestone.Index) {
				if _, exists := referencedBlocks[meta.BlockID()]; !exists {
					referencedBlocks[meta.BlockID()] = struct{}{}
					meta.SetReferenced(referenced, msIndex)
				}
			},
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

		// cleanup the state changes from the temporary UTXOManager to save memory
		if err := cleanupMilestoneFromUTXOManager(utxoManagerTemp, milestonePayload, msIndex); err != nil {
			return err
		}

		return nil
	}

	for msIndex := msIndexStart; msIndex <= msIndexEnd; msIndex++ {
		blocksCount := 0

		ts := time.Now()

		if err := checkMilestoneCone(
			ctx,
			tangleStoreSource.CachedBlock,
			milestoneManager,
			func(cachedBlock *storage.CachedBlock) {
				defer cachedBlock.Release(true) // block -1
				blocksCount++
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

		println(fmt.Sprintf("successfully verified milestone cone %d, blocks: %d, total: %v", msIndex, blocksCount, time.Since(ts).Truncate(time.Millisecond)))
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
	ledgerStateTemp, err := utxoManagerTemp.LedgerStateSHA256Sum()
	if err != nil {
		return err
	}
	if !bytes.Equal(ledgerStateSource, ledgerStateTemp) {
		return errors.New("ledger state of source database and temp database does not match")
	}

	return nil
}

func cleanupMilestoneFromUTXOManager(utxoManager *utxo.Manager, milestonePayload *iotago.Milestone, msIndex milestone.Index) error {

	var receiptMigratedAtIndex []uint32

	opts, err := milestonePayload.Opts.Set()
	if err == nil && opts != nil {
		if r := opts.Receipt(); r != nil {
			receiptMigratedAtIndex = append(receiptMigratedAtIndex, r.MigratedAt)
		}
	}

	if err := utxoManager.PruneMilestoneIndexWithoutLocking(msIndex, true, receiptMigratedAtIndex...); err != nil {
		return err
	}

	return nil
}
