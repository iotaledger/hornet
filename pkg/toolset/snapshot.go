package toolset

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	iotago "github.com/iotaledger/iota.go"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/snapshot"
)

func snapshotGen(args []string) error {

	//
	// check arguments
	//
	if len(args) == 0 {
		return errors.New("input balances filepath missing")
	}

	if len(args) == 1 {
		return errors.New("output filepath missing")
	}

	if len(args) > 2 {
		return errors.New("too many arguments for 'snapshotgen'")
	}

	balancesFilePath := args[0]
	outputFilePath := args[1]
	targetIndex := 0

	//
	// check filepaths
	//
	if _, err := os.Stat(balancesFilePath); err != nil && os.IsNotExist(err) {
		return errors.New("input balances file does not exist")
	}

	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return errors.New("output file already exists")
	}

	//
	// open files
	//
	balancesFile, err := os.OpenFile(balancesFilePath, os.O_RDONLY, 0666)
	if err != nil {
		return fmt.Errorf("unable to open input balances file: %w", err)
	}
	defer balancesFile.Close()

	// build temp file path
	outputFilePathTmp := outputFilePath + "_tmp"
	_ = os.Remove(outputFilePathTmp)

	snapshotFile, err := os.OpenFile(outputFilePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create snapshot file: %w", err)
	}

	//
	// create snapshot file
	//
	header := &snapshot.FileHeader{
		Version:              snapshot.SupportedFormatVersion,
		Type:                 snapshot.Full,
		SEPMilestoneIndex:    milestone.Index(targetIndex),
		LedgerMilestoneIndex: milestone.Index(targetIndex),
	}
	copy(header.SEPMilestoneHash[:], hornet.NullMessageID)
	copy(header.LedgerMilestoneHash[:], hornet.NullMessageID)

	// solid entry points
	// add "NullMessageID" as sole entry point
	nullHashAdded := false
	solidEntryPointProducerFunc := func() (*[snapshot.SolidEntryPointHashLength]byte, error) {
		if !nullHashAdded {
			nullHashAdded = true

			var solidEntryPoint [snapshot.SolidEntryPointHashLength]byte
			copy(solidEntryPoint[:], hornet.NullMessageID)

			return &solidEntryPoint, nil
		}
		return nil, nil
	}

	// unspent outputs
	// read all addresses and balances from the input balances file and add a "NullOutputID" ("NullHash" as TransactionID and OutputIndex 0)
	var totalAmount uint64 = 0

	scanner := bufio.NewScanner(balancesFile)
	outputProducerFunc := func() (*snapshot.Output, error) {
		if !scanner.Scan() {
			// end of file reached
			return nil, nil
		}

		line := scanner.Text()
		lineSplitted := strings.Split(line, ";")

		if len(lineSplitted) != 2 {
			return nil, fmt.Errorf("Wrong format in %v", balancesFilePath)
		}

		addressBytes, err := hex.DecodeString(lineSplitted[0])
		if err != nil {
			return nil, fmt.Errorf("DecodeString: %v", err)
		}
		if len(addressBytes) != iotago.Ed25519AddressBytesLength {
			return nil, fmt.Errorf("incorrect address length: %d != %d (%s)", len(addressBytes), iotago.Ed25519AddressBytesLength, lineSplitted[0])
		}

		var address iotago.Ed25519Address
		copy(address[:], addressBytes)

		balance, err := strconv.ParseUint(lineSplitted[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("ParseUint: %v", err)
		}
		totalAmount += balance

		var nullOutputID [iotago.TransactionIDLength + iotago.UInt16ByteSize]byte

		return &snapshot.Output{OutputID: nullOutputID, Address: &address, Amount: balance}, nil
	}

	// milestone diffs
	// no milestone diffs needed
	milestoneDiffProducerFunc := func() (*snapshot.MilestoneDiff, error) {
		return nil, nil
	}

	if err := snapshot.StreamLocalSnapshotDataTo(snapshotFile, uint64(time.Now().Unix()), header, solidEntryPointProducerFunc, outputProducerFunc, milestoneDiffProducerFunc); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate snapshot file: %w", err)
	}

	if err := scanner.Err(); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't read input balances file: %w", err)
	}

	if totalAmount != iotago.TokenSupply {
		return fmt.Errorf("accumulated output balance is not equal to total supply: %d != %d", totalAmount, iotago.TokenSupply)
	}

	// rename tmp file to final file name
	if err := snapshotFile.Close(); err != nil {
		return fmt.Errorf("unable to close snapshot file: %w", err)
	}

	if err := os.Rename(outputFilePathTmp, outputFilePath); err != nil {
		return fmt.Errorf("unable to rename temp snapshot file: %w", err)
	}

	fmt.Println("Snapshot creation successful!")
	return nil
}
