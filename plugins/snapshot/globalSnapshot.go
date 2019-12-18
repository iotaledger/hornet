package snapshot

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

func loadSpentAddresses(filePathSpent string) error {
	log.Infof("Importing initial spent addresses from %v", filePathSpent)

	spentFile, err := os.OpenFile(filePathSpent, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer spentFile.Close()

	var line string

	ioReader := bufio.NewReader(spentFile)
	for err == nil {
		line, err = ioReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		address := line[:len(line)-1]
		err = trinary.ValidTrytes(address)
		if err != nil {
			return nil
		}

		tangle.MarkAddressAsSpent(address)
	}

	log.Infof("Finished loading spent addresses from %v", filePathSpent)

	return nil
}

func loadSnapshotFromTextfiles(filePathLedger string, filePathSpent []string, snapshotIndex milestone_index.MilestoneIndex) error {

	tangle.ResetSolidEntryPoints()

	// Genesis transaction
	tangle.SolidEntryPointsAdd(NullHash, snapshotIndex)
	tangle.StoreSolidEntryPoints()

	log.Infof("Importing initial ledger from %v", filePathLedger)

	ledgerFile, err := os.OpenFile(filePathLedger, os.O_RDONLY, 0666)
	if err != nil {
		return err
	}
	defer ledgerFile.Close()

	ledgerState := make(map[trinary.Hash]uint64)

	var line string
	var balance uint64

	ioReader := bufio.NewReader(ledgerFile)
	for err == nil {
		line, err = ioReader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		lineSplitted := strings.Split(line[:len(line)-1], ";")
		if len(lineSplitted) != 2 {
			return fmt.Errorf("Wrong format in %v", filePathLedger)
		}

		address := lineSplitted[0][:81]
		err = trinary.ValidTrytes(address)
		if err != nil {
			return nil
		}
		balance, err = strconv.ParseUint(lineSplitted[1], 10, 64)
		if err != nil {
			return err
		}

		ledgerState[address] = balance

		//log.Infof("Address: %v (%d)", address, balance)
	}

	err = tangle.StoreBalancesInDatabase(ledgerState, snapshotIndex)
	if err != nil {
		return fmt.Errorf("ledgerEntries: %s", err)
	}

	for _, spent := range filePathSpent {
		err := loadSpentAddresses(spent)
		if err != nil {
			return err
		}
	}

	tangle.SetSnapshotMilestone(NullHash, snapshotIndex, snapshotIndex, 0)

	log.Info("Finished loading snapshot")

	tanglePlugin.Events.SnapshotMilestoneIndexChanged.Trigger(snapshotIndex)

	return nil
}

func LoadEmptySnapshot(filePathLedger string) error {

	log.Info("Loading empty snapshot...")
	return loadSnapshotFromTextfiles(filePathLedger, []string{}, 0)
}

func LoadGlobalSnapshot(filePathLedger string, filePathSpent []string, snapshotIndex milestone_index.MilestoneIndex) error {

	log.Infof("Loading global snapshot with index %v...", snapshotIndex)
	return loadSnapshotFromTextfiles(filePathLedger, filePathSpent, snapshotIndex)
}
