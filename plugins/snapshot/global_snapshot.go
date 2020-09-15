package snapshot

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func loadSnapshotFromTextfiles(filePathLedger string, snapshotIndex milestone.Index) error {

	latestMilestoneFromDatabase := tangle.SearchLatestMilestoneIndexInStore()
	if latestMilestoneFromDatabase > snapshotIndex {
		return errors.Wrapf(ErrSnapshotImportFailed, "Milestone in database (%d) newer than snapshot milestone (%d)", latestMilestoneFromDatabase, snapshotIndex)
	}

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()

	// Genesis transaction must be marked as SEP with snapshot index during loading a global snapshot,
	// because coordinator bootstraps the network by referencing the genesis tx
	tangle.SolidEntryPointsAdd(hornet.NullHashBytes, snapshotIndex)
	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	log.Infof("Importing initial ledger from %v", filePathLedger)

	ledgerFile, err := os.OpenFile(filePathLedger, os.O_RDONLY, 0666)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "OpenFile: %v", err)
	}
	defer ledgerFile.Close()

	ledgerState := make(map[string]uint64)
	scanner := bufio.NewScanner(ledgerFile)

	for scanner.Scan() {
		line := scanner.Text()
		lineSplitted := strings.Split(line, ";")
		if len(lineSplitted) != 2 {
			return errors.Wrapf(ErrSnapshotImportFailed, "Wrong format in %v", filePathLedger)
		}

		addr := lineSplitted[0]
		if err := address.ValidAddress(addr); err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ValidAddress: %v", err)
		}

		balance, err := strconv.ParseUint(lineSplitted[1], 10, 64)
		if err != nil {
			return errors.Wrapf(ErrSnapshotImportFailed, "ParseUint: %v", err)
		}

		ledgerState[string(hornet.HashFromAddressTrytes(addr))] = balance
	}
	if err := scanner.Err(); err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "Scanner: %v", err)
	}

	var total uint64
	for _, value := range ledgerState {
		total += value
	}

	if total != consts.TotalSupply {
		return errors.Wrapf(ErrInvalidBalance, "%d != %d", total, consts.TotalSupply)
	}

	err = tangle.StoreSnapshotBalancesInDatabase(ledgerState, snapshotIndex)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "snapshot ledgerEntries: %s", err)
	}

	err = tangle.StoreLedgerBalancesInDatabase(ledgerState, snapshotIndex)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %s", err)
	}

	coordinatorAddress := hornet.HashFromAddressTrytes(config.NodeConfig.GetString(config.CfgCoordinatorAddress))
	tangle.SetSnapshotMilestone(coordinatorAddress, hornet.NullHashBytes, snapshotIndex, snapshotIndex, snapshotIndex, 0)
	tangle.SetLatestSeenMilestoneIndexFromSnapshot(snapshotIndex)

	// set the solid milestone index based on the snapshot milestone
	tangle.SetSolidMilestoneIndex(snapshotIndex, false)

	log.Info("Finished loading snapshot")

	tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(snapshotIndex)

	return nil
}

func LoadGlobalSnapshot(filePathLedger string, snapshotIndex milestone.Index) error {

	log.Infof("Loading global snapshot with index %v...", snapshotIndex)
	return loadSnapshotFromTextfiles(filePathLedger, snapshotIndex)
}
