package snapshot

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/model/milestone_index"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/plugins/gossip"
	tanglePlugin "github.com/gohornet/hornet/plugins/tangle"
)

type localSnapshot struct {
	msHash           string
	msIndex          milestone_index.MilestoneIndex
	msTimestamp      int64
	solidEntryPoints map[string]milestone_index.MilestoneIndex
	seenMilestones   map[string]milestone_index.MilestoneIndex
	ledgerState      map[string]uint64
}

func (ls *localSnapshot) SizeInBytes() int {
	return 49 + 24 + (len(ls.solidEntryPoints) * (49 + 4)) + (len(ls.seenMilestones) * (49 + 4)) + (len(ls.ledgerState) * (49 + 8))
}

func (ls *localSnapshot) WriteToBuffer(buf io.Writer, useFileFormat bool, spentAddrCnt int32) error {
	var err error

	if useFileFormat {
		msHashBytes, err := trinary.TrytesToBytes(ls.msHash)
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, msHashBytes[:49])
		if err != nil {
			return err
		}
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

	if useFileFormat {
		err = binary.Write(buf, binary.BigEndian, int32(len(ls.ledgerState)))
		if err != nil {
			return err
		}

		err = binary.Write(buf, binary.BigEndian, spentAddrCnt)
		if err != nil {
			return err
		}
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

	for hash, val := range ls.ledgerState {
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

func (ls *localSnapshot) Bytes() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, ls.SizeInBytes()))

	err := ls.WriteToBuffer(buf, false, 0)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type spentAddresses struct {
	addresses [][]byte
}

func (sa *spentAddresses) WriteToBuffer(buf io.Writer) error {
	var err error

	for _, val := range sa.addresses {
		err = binary.Write(buf, binary.BigEndian, val)
		if err != nil {
			return err
		}
	}

	return nil
}

func (sa *spentAddresses) SizeInBytes() int {
	return (len(sa.addresses) * 49)
}

func (sa *spentAddresses) Bytes() ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, sa.SizeInBytes()))

	err := sa.WriteToBuffer(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func printLocalSnapshotFileSsInfo(ls *localSnapshot, sa *spentAddresses) {
	fmt.Printf("ms index: %d\n", ls.msIndex)
	fmt.Printf("hash: %s\n", ls.msHash)
	fmt.Printf("timestamp: %d\n", ls.msTimestamp)
	fmt.Printf("solid entry points: %d\n", len(ls.solidEntryPoints))
	fmt.Printf("seen milestones: %d\n", len(ls.seenMilestones))
	fmt.Printf("ledger entries: %d\n", len(ls.ledgerState))
	fmt.Printf("spent addresses: %d\n", len(sa.addresses))

	var total int64
	for _, val := range ls.ledgerState {
		total += int64(val)
	}
	lsSizeInBytes := ls.SizeInBytes()
	saSizeInBytes := sa.SizeInBytes()

	lsBytes, err := ls.Bytes()
	if err != nil {
		panic(err)
	}

	saBytes, err := sa.Bytes()
	if err != nil {
		panic(err)
	}

	lsActualSizeInBytes := len(lsBytes)
	saActualSizeInBytes := len(saBytes)

	fmt.Printf("max supply correct: %v\n", uint64(total) == compressed.TOTAL_SUPPLY)
	fmt.Printf("local snapshot  bytes size correct: %v (%d = %d (MBs))\n", lsSizeInBytes == lsActualSizeInBytes, lsSizeInBytes/1024/2024, lsActualSizeInBytes/1024/2024)
	fmt.Printf("spent addresses bytes size correct: %v (%d = %d (MBs))\n", saSizeInBytes == saActualSizeInBytes, saSizeInBytes/1024/2024, saActualSizeInBytes/1024/2024)
}

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

func LoadEmptySnapshot(filePathLedger string) error {

	log.Info("Loading empty snapshot...")
	return loadSnapshotFromTextfiles(filePathLedger, []string{}, 0)
}

func LoadGlobalSnapshot(filePathLedger string, filePathSpent []string, snapshotIndex milestone_index.MilestoneIndex) error {

	log.Infof("Loading global snapshot with index %v...", snapshotIndex)
	return loadSnapshotFromTextfiles(filePathLedger, filePathSpent, snapshotIndex)
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

	/*
		ls := &localSnapshot{
			solidEntryPoints: make(map[string]int32),
			seenMilestones:   make(map[string]int32),
			ledgerState:      make(map[string]uint64),
		}
	*/

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

	log.Info("Importing current ledger")

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

	if parameter.NodeConfig.GetBool("localSnapshots.importSpentAddresses") {
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
	} else {
		log.Warningf("Skipping importing %d spent addresses\n", spentAddrsCount)
	}

	log.Info("Finished loading snapshot")

	return nil
}

func SaveSnapshotToFile(filePath string, ls *localSnapshot, sa *spentAddresses) error {
	ts := time.Now()

	os.Remove(filePath)

	fmt.Printf("writing gzipped stream to file %s\n", filePath)

	exportFile, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	defer exportFile.Close()

	gzipWriter := gzip.NewWriter(exportFile)
	defer gzipWriter.Close()

	err = ls.WriteToBuffer(gzipWriter, true, int32(len(sa.addresses)))
	if err != nil {
		return err
	}

	err = sa.WriteToBuffer(gzipWriter)
	if err != nil {
		return err
	}

	fmt.Printf("finished, took %v\n", time.Now().Sub(ts))

	return nil

}
