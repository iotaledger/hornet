package snapshot

import (
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/dag"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
)

const (
	SolidEntryPointCheckThreshold = 50
)

// isSolidEntryPoint checks whether any direct approver of the given transaction was confirmed by a milestone which is above the target milestone.
func isSolidEntryPoint(txHash trinary.Hash, targetIndex milestone_index.MilestoneIndex) (bool, milestone_index.MilestoneIndex) {
	approvers, _ := tangle.GetApprovers(txHash)
	if approvers == nil {
		return false, 0
	}

	for _, approver := range approvers.GetHashes() {
		tx, _ := tangle.GetTransaction(approver)
		if tx == nil {
			log.Panicf("isSolidEntryPoint: Transaction not found: %v", approver)
		}

		// HINT: Check for orphaned Tx as solid entry points is skipped in HORNET, since this operation is heavy and not necessary, and
		//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
		//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

		confirmed, at := tx.GetConfirmed()
		if confirmed && (at > targetIndex) {
			// confirmed by a later milestone than targetIndex => solidEntryPoint
			return true, at
		}
	}

	return false, 0
}

// getMilestoneApprovees traverses a milestone and collects all tx that were confirmed by that milestone or higher
func getMilestoneApprovees(milestoneIndex milestone_index.MilestoneIndex, milestoneTail *hornet.Transaction) []trinary.Hash {
	ts := time.Now()

	txsToTraverse := make(map[string]struct{})
	txsChecked := make(map[string]struct{})
	var approvees []trinary.Hash
	txsToTraverse[milestoneTail.GetHash()] = struct{}{}

	// Collect all tx by traversing the tangle
	// Loop as long as new transactions are added in every loop cycle
	for len(txsToTraverse) != 0 {

		for txHash := range txsToTraverse {
			delete(txsToTraverse, txHash)

			if _, checked := txsChecked[txHash]; checked {
				// Tx was already checked => ignore
				continue
			}
			txsChecked[txHash] = struct{}{}

			if tangle.SolidEntryPointsContain(txHash) {
				// Ignore solid entry points (snapshot milestone included)
				continue
			}

			tx, _ := tangle.GetTransaction(txHash)
			if tx == nil {
				log.Panicf("getMilestoneApprovees: Transaction not found: %v", txHash)
			}

			confirmed, at := tx.GetConfirmed()
			if !confirmed {
				log.Panicf("getMilestoneApprovees: Transaction must be confirmed: %v", txHash)
			}

			if at < milestoneIndex {
				// Ignore Tx that were confirmed by older milestones
				continue
			}

			approvees = append(approvees, txHash)

			// Traverse the approvee
			txsToTraverse[tx.GetTrunk()] = struct{}{}
			txsToTraverse[tx.GetBranch()] = struct{}{}
		}
	}

	log.Debugf("Milestone walked (%d): approvees: %v, collect: %v", milestoneIndex, len(approvees), time.Since(ts))
	return approvees
}

func shouldTakeSnapshot(solidMilestoneIndex milestone_index.MilestoneIndex) bool {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	var snapshotInterval milestone_index.MilestoneIndex
	if tangle.IsNodeSynced() {
		snapshotInterval = snapshotIntervalSynced
	} else {
		snapshotInterval = snapshotIntervalUnsynced
	}

	return solidMilestoneIndex-(snapshotDepth+snapshotInterval) > snapshotInfo.SnapshotIndex
}

func getSolidEntryPoints(targetIndex milestone_index.MilestoneIndex) map[string]milestone_index.MilestoneIndex {

	solidEntryPoints := make(map[string]milestone_index.MilestoneIndex)
	solidEntryPoints[NullHash] = targetIndex

	// HINT: Check if "old solid entry points are still valid" is skipped in HORNET,
	//		 since they should all be found by iterating the milestones to a certain depth under targetIndex, because the tipselection for COO was changed.
	//		 When local snapshots were introduced in IRI, there was the problem that COO approved really old tx as valid tips, which is not the case anymore.

	// Iterate from a reasonable old milestone to the target index to check for solid entry points
	for milestoneIndex := targetIndex - SolidEntryPointCheckThreshold; milestoneIndex <= targetIndex; milestoneIndex++ {
		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			log.Panicf("CreateLocalSnapshot: Milestone (%d) not found!", milestoneIndex)
		}

		// Get all approvees of that milestone
		approvees := getMilestoneApprovees(milestoneIndex, ms.GetTail())
		for _, approvee := range approvees {

			if isEntryPoint, at := isSolidEntryPoint(approvee, targetIndex); isEntryPoint {
				// A solid entry point should only be a tail transaction, otherwise the whole bundle can't be reproduced with a snapshot file
				tails, err := dag.FindAllTails(approvee)
				if err != nil {
					log.Panicf("CreateLocalSnapshot: %v", err)
				}

				for tailHash := range tails {
					solidEntryPoints[tailHash] = at
				}
			}
		}
	}

	return solidEntryPoints
}

func getSeenMilestones(targetIndex milestone_index.MilestoneIndex) map[string]milestone_index.MilestoneIndex {

	// Fill the list with seen milestones
	seenMilestones := make(map[string]milestone_index.MilestoneIndex)
	lastestMilestone := tangle.GetLatestMilestoneIndex()
	for milestoneIndex := targetIndex + 1; milestoneIndex <= lastestMilestone; milestoneIndex++ {
		ms, _ := tangle.GetMilestone(milestoneIndex)
		if ms == nil {
			continue
		}
		seenMilestones[ms.GetTailHash()] = milestoneIndex
	}
	return seenMilestones
}

func getLedgerStateAtMilestone(balances map[trinary.Hash]uint64, targetIndex milestone_index.MilestoneIndex, solidMilestoneIndex milestone_index.MilestoneIndex) map[trinary.Hash]uint64 {

	// Calculate balances for targetIndex
	for milestoneIndex := solidMilestoneIndex; milestoneIndex > targetIndex; milestoneIndex-- {
		diff, err := tangle.GetLedgerDiffForMilestone(milestoneIndex)
		if err != nil {
			log.Panicf("CreateLocalSnapshot: %v", err)
		}

		for address, change := range diff {
			newBalance := int64(balances[address]) - change

			if newBalance < 0 {
				panic(fmt.Sprintf("CreateLocalSnapshot: Ledger diff for milestone %d creates negative balance for address %s: current %d, diff %d", milestoneIndex, address, balances[address], change))
			} else if newBalance == 0 {
				delete(balances, address)
			} else {
				balances[address] = uint64(newBalance)
			}
		}
	}
	return balances
}

func checkSnapshotLimits(targetIndex milestone_index.MilestoneIndex) error {

	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo == nil {
		log.Panic("No snapshotInfo found!")
	}

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()
	if targetIndex > (solidMilestoneIndex - snapshotDepth) {
		return fmt.Errorf("The snapshot target %d is too new. Should be older than %d", targetIndex, solidMilestoneIndex-snapshotDepth)
	}

	if targetIndex <= snapshotInfo.SnapshotIndex {
		return fmt.Errorf("The snapshot target %d is too old. Should be newer than %d", targetIndex, snapshotInfo.SnapshotIndex)
	}

	if targetIndex-SolidEntryPointCheckThreshold < snapshotInfo.PruningIndex+1 {
		return fmt.Errorf("The snapshot target %d is too old. Should be newer than %d", targetIndex, snapshotInfo.PruningIndex+1+SolidEntryPointCheckThreshold)
	}

	return nil
}

func createLocalSnapshotWithoutLocking(targetIndex milestone_index.MilestoneIndex, filePath string) error {

	log.Infof("Creating local snapshot for targetIndex %d", targetIndex)

	ts := time.Now()

	if err := checkSnapshotLimits(targetIndex); err != nil {
		return err
	}

	tangle.ReadLockLedger()
	defer tangle.ReadUnlockLedger()

	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	targetMilestone, _ := tangle.GetMilestone(targetIndex)
	if targetMilestone == nil {
		log.Panicf("CreateLocalSnapshot: Milestone (%d) not found!", targetIndex)
	}

	balances, ledgerMilestone, err := tangle.GetAllBalances()
	if err != nil {
		return err
	}

	if ledgerMilestone != solidMilestoneIndex {
		log.Panicf("CreateLocalSnapshot: LedgerMilestone wrong! %d/%d", ledgerMilestone, solidMilestoneIndex)
	}

	newBalances := getLedgerStateAtMilestone(balances, targetIndex, solidMilestoneIndex)
	solidEntryPoints := getSolidEntryPoints(targetIndex)
	seenMilestones := getSeenMilestones(targetIndex)

	tangle.ReadLockSpentAddresses()
	defer tangle.ReadUnlockSpentAddresses()

	spentAddressesCount, err := tangle.CountSpentAddressesEntries()
	if err != nil {
		return err
	}

	filePathTmp := filePath + "_tmp"

	// Remove old temp file
	os.Remove(filePathTmp)

	exportFile, err := os.OpenFile(filePathTmp, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer exportFile.Close()

	gzipWriter := gzip.NewWriter(exportFile)
	defer gzipWriter.Close()

	lsh := &localSnapshotHeader{
		msHash:              targetMilestone.GetTailHash(),
		msIndex:             targetIndex,
		msTimestamp:         targetMilestone.GetTail().GetTimestamp(),
		solidEntryPoints:    solidEntryPoints,
		seenMilestones:      seenMilestones,
		balances:            newBalances,
		spentAddressesCount: spentAddressesCount,
	}

	if err := lsh.WriteToBuffer(gzipWriter); err != nil {
		return err
	}

	/*
		balancesWritten, err := tangle.StreamBalancesToWriter(gzipWriter, balancesCount, totalBalanceDiffs)
		if err != nil {
			return err
		}
	*/

	if err := tangle.StreamSpentAddressesToWriter(gzipWriter, spentAddressesCount); err != nil {
		return err
	}

	if err := os.Rename(filePathTmp, filePath); err != nil {
		return err
	}

	log.Infof("Creating local snapshot for targetIndex %d done, took %v", targetIndex, time.Since(ts))

	return nil
}

func CreateLocalSnapshot(targetIndex milestone_index.MilestoneIndex, filePath string) error {
	localSnapshotLock.Lock()
	defer localSnapshotLock.Unlock()
	return createLocalSnapshotWithoutLocking(targetIndex, filePath)
}

type localSnapshotHeader struct {
	msHash              string
	msIndex             milestone_index.MilestoneIndex
	msTimestamp         int64
	solidEntryPoints    map[string]milestone_index.MilestoneIndex
	seenMilestones      map[string]milestone_index.MilestoneIndex
	balances            map[string]uint64
	spentAddressesCount int32
}

func (ls *localSnapshotHeader) WriteToBuffer(buf io.Writer) error {
	var err error

	msHashBytes, err := trinary.TrytesToBytes(ls.msHash)
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, msHashBytes[:49])
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, ls.msIndex)
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, ls.msTimestamp)
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, int32(len(ls.solidEntryPoints)))
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, int32(len(ls.seenMilestones)))
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, int32(len(ls.balances)))
	if err != nil {
		return err
	}

	err = binary.Write(buf, binary.BigEndian, ls.spentAddressesCount)
	if err != nil {
		return err
	}

	for hash, val := range ls.solidEntryPoints {
		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, addrBytes[:49])
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, val)
		if err != nil {
			return err
		}
	}

	for hash, val := range ls.seenMilestones {
		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, addrBytes[:49])
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, val)
		if err != nil {
			return err
		}
	}

	// ToDo: Don't convert to trinary at all
	for hash, val := range ls.balances {
		addrBytes, err := trinary.TrytesToBytes(hash)
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, addrBytes[:49])
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, val)
		if err != nil {
			return err
		}
	}

	return nil
}

func LoadSnapshotFromFile(filePath string) error {
	log.Info("Loading snapshot file...")

	file, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	hashBuf := make([]byte, 49)
	_, err = gzipReader.Read(hashBuf)
	if err != nil {
		return err
	}

	tangle.ResetSolidEntryPoints()

	// Genesis transaction
	tangle.SolidEntryPointsAdd(NullHash, 0)

	var msIndex int32
	var msTimestamp int64
	var solidEntryPointsCount, seenMilestonesCount, ledgerEntriesCount, spentAddrsCount int32

	msHash, err := trinary.BytesToTrytes(hashBuf)
	if err != nil {
		return err
	}

	err = binary.Read(gzipReader, binary.BigEndian, &msIndex)
	if err != nil {
		return err
	}

	err = binary.Read(gzipReader, binary.BigEndian, &msTimestamp)
	if err != nil {
		return err
	}

	tangle.SetSnapshotMilestone(msHash[:81], milestone_index.MilestoneIndex(msIndex), milestone_index.MilestoneIndex(msIndex), msTimestamp)
	tangle.SolidEntryPointsAdd(msHash[:81], milestone_index.MilestoneIndex(msIndex))

	err = binary.Read(gzipReader, binary.BigEndian, &solidEntryPointsCount)
	if err != nil {
		return err
	}

	err = binary.Read(gzipReader, binary.BigEndian, &seenMilestonesCount)
	if err != nil {
		return err
	}

	err = binary.Read(gzipReader, binary.BigEndian, &ledgerEntriesCount)
	if err != nil {
		return err
	}

	err = binary.Read(gzipReader, binary.BigEndian, &spentAddrsCount)
	if err != nil {
		return err
	}

	log.Info("Importing solid entry points")

	for i := 0; i < int(solidEntryPointsCount); i++ {
		var val int32

		err = binary.Read(gzipReader, binary.BigEndian, hashBuf)
		if err != nil {
			return fmt.Errorf("solidEntryPoints: %s", err)
		}

		err = binary.Read(gzipReader, binary.BigEndian, &val)
		if err != nil {
			return fmt.Errorf("solidEntryPoints: %s", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return fmt.Errorf("solidEntryPoints: %s", err)
		}
		//ls.solidEntryPoints[hash[:81]] = val

		tangle.SolidEntryPointsAdd(hash[:81], milestone_index.MilestoneIndex(val))
	}

	tangle.StoreSolidEntryPoints()

	log.Info("Importing seen milestones")

	for i := 0; i < int(seenMilestonesCount); i++ {
		var val int32

		err = binary.Read(gzipReader, binary.BigEndian, hashBuf)
		if err != nil {
			return fmt.Errorf("seenMilestones: %s", err)
		}

		err = binary.Read(gzipReader, binary.BigEndian, &val)
		if err != nil {
			return fmt.Errorf("seenMilestones: %s", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return fmt.Errorf("seenMilestones: %s", err)
		}

		tangle.SetLatestSeenMilestoneIndexFromSnapshot(milestone_index.MilestoneIndex(val))
		gossip.Request([]trinary.Hash{hash[:81]}, milestone_index.MilestoneIndex(val))
	}

	log.Info("Importing ledger state")

	ledgerState := make(map[trinary.Hash]uint64)
	for i := 0; i < int(ledgerEntriesCount); i++ {
		var val uint64

		err = binary.Read(gzipReader, binary.BigEndian, hashBuf)
		if err != nil {
			return fmt.Errorf("ledgerEntries: %s", err)
		}

		err = binary.Read(gzipReader, binary.BigEndian, &val)
		if err != nil {
			return fmt.Errorf("ledgerEntries: %s", err)
		}

		hash, err := trinary.BytesToTrytes(hashBuf)
		if err != nil {
			return fmt.Errorf("ledgerEntries: %s", err)
		}
		ledgerState[hash[:81]] = val
	}

	err = tangle.StoreBalancesInDatabase(ledgerState, milestone_index.MilestoneIndex(msIndex))
	if err != nil {
		return fmt.Errorf("ledgerEntries: %s", err)
	}

	log.Infof("Importing %d spent addresses\n", spentAddrsCount)
	for i := 0; i < int(spentAddrsCount); i++ {
		spentAddrBuf := make([]byte, 49)

		err = binary.Read(gzipReader, binary.BigEndian, spentAddrBuf)
		if err != nil {
			return fmt.Errorf("spentAddrs: %s", err)
		}

		hash, err := trinary.BytesToTrytes(spentAddrBuf)
		if err != nil {
			return fmt.Errorf("spentAddrs: %s", err)
		}

		tangle.MarkAddressAsSpent(hash[:81])
	}

	log.Info("Finished loading snapshot")

	return nil
}
