package snapshot

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func loadSpentAddresses(filePathSpent string) (int, error) {
	log.Infof("Importing initial spent addresses from %v", filePathSpent)

	spentAddressesCount := 0

	spentFile, err := os.OpenFile(filePathSpent, os.O_RDONLY, 0666)
	if err != nil {
		return 0, err
	}
	defer spentFile.Close()

	var line string

	ioReader := bufio.NewReader(spentFile)
	for {
		line, err = ioReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}

		address := line[:len(line)-1]
		if err := trinary.ValidTrytes(address); err != nil {
			return 0, err
		}

		if tangle.MarkAddressAsSpent(address) {
			spentAddressesCount++
		}
	}

	log.Infof("Finished loading spent addresses from %v", filePathSpent)

	return spentAddressesCount, nil
}

func loadSnapshotFromTextfiles(filePathLedger string, filePathsSpent []string, snapshotIndex milestone_index.MilestoneIndex) error {

	tangle.WriteLockSolidEntryPoints()
	tangle.ResetSolidEntryPoints()

	// Genesis transaction
	tangle.SolidEntryPointsAdd(consts.NullHashTrytes, snapshotIndex)
	tangle.StoreSolidEntryPoints()
	tangle.WriteUnlockSolidEntryPoints()

	log.Infof("Importing initial ledger from %v", filePathLedger)

	ledgerFile, err := os.OpenFile(filePathLedger, os.O_RDONLY, 0666)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "OpenFile: %v", err)
	}
	defer ledgerFile.Close()

	ledgerState := make(map[trinary.Hash]uint64)
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

		ledgerState[addr] = balance
		//log.Infof("Address: %v (%d)", addr, balance)
	}
	if scanner.Err() != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "Scanner: %v", scanner.Err())
	}

	var total uint64
	for _, value := range ledgerState {
		total += value
	}

	if total != compressed.TOTAL_SUPPLY {
		return errors.Wrapf(ErrInvalidBalance, "%d != %d", total, compressed.TOTAL_SUPPLY)
	}

	err = tangle.StoreSnapshotBalancesInDatabase(ledgerState, snapshotIndex)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "snapshot ledgerEntries: %s", err)
	}

	err = tangle.StoreLedgerBalancesInDatabase(ledgerState, snapshotIndex)
	if err != nil {
		return errors.Wrapf(ErrSnapshotImportFailed, "ledgerEntries: %s", err)
	}

	spentAddressesSum := 0
	if config.NodeConfig.GetBool(config.CfgSpentAddressesEnabled) {
		for _, spent := range filePathsSpent {
			spentAddressesCount, err := loadSpentAddresses(spent)
			if err != nil {
				return errors.Wrapf(ErrSnapshotImportFailed, "loadSpentAddresses: %v", err)
			}
			spentAddressesSum += spentAddressesCount
		}
	}

	tangle.SetSnapshotMilestone(consts.NullHashTrytes, snapshotIndex, snapshotIndex, 0, spentAddressesSum != 0)

	log.Info("Finished loading snapshot")

	tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(snapshotIndex)

	return nil
}

func LoadGlobalSnapshot(filePathLedger string, filePathsSpent []string, snapshotIndex milestone_index.MilestoneIndex) error {

	log.Infof("Loading global snapshot with index %v...", snapshotIndex)
	return loadSnapshotFromTextfiles(filePathLedger, filePathsSpent, snapshotIndex)
}
