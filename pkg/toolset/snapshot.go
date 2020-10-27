package toolset

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	iotago "github.com/iotaledger/iota.go"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/snapshot"
)

func snapshotGen(args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [MINT_ADDRESS] [OUTPUT_FILE_PATH]", ToolSnapGen))
		println()
		println("	[MINT_ADDRESS] 	   - the initial ed25519 address all the tokens will be minted to")
		println("	[OUTPUT_FILE_PATH] - the file path to the generated snapshot file")
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("MINT_ADDRESS missing")
	}

	if len(args) == 1 {
		printUsage()
		return errors.New("OUTPUT_FILE_PATH missing")
	}

	if len(args) > 2 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolSnapGen)
	}

	// check mint address
	mintAddress := args[0]

	addressBytes, err := hex.DecodeString(mintAddress)
	if err != nil {
		return fmt.Errorf("can't decode MINT_ADDRESS: %v", err)
	}
	if len(addressBytes) != iotago.Ed25519AddressBytesLength {
		return fmt.Errorf("incorrect MINT_ADDRESS length: %d != %d (%s)", len(addressBytes), iotago.Ed25519AddressBytesLength, mintAddress)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	// check filepath
	outputFilePath := args[1]
	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return errors.New("OUTPUT_FILE_PATH already exists")
	}

	// build temp file path
	outputFilePathTmp := outputFilePath + "_tmp"
	_ = os.Remove(outputFilePathTmp)

	snapshotFile, err := os.OpenFile(outputFilePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create snapshot file: %w", err)
	}

	networkID := make([]byte, 1)

	// 1 is reserved for mainnet
	for networkID[0] <= 1 {
		if _, err := rand.Read(networkID); err != nil {
			return fmt.Errorf("unable to create network ID: %w", err)
		}
	}

	// create snapshot file
	targetIndex := 0
	header := &snapshot.FileHeader{
		Version:              snapshot.SupportedFormatVersion,
		Type:                 snapshot.Full,
		NetworkID:            networkID[0],
		SEPMilestoneIndex:    milestone.Index(targetIndex),
		LedgerMilestoneIndex: milestone.Index(targetIndex),
	}

	header.SEPMilestoneID = &iotago.MilestoneID{}
	header.LedgerMilestoneID = &iotago.MilestoneID{}

	// solid entry points
	// add "NullMessageID" as sole entry point
	nullHashAdded := false
	solidEntryPointProducerFunc := func() (*hornet.MessageID, error) {
		if nullHashAdded {
			return nil, nil
		}

		nullHashAdded = true

		return hornet.GetNullMessageID(), nil
	}

	// unspent transaction outputs
	outputAdded := false
	outputProducerFunc := func() (*snapshot.Output, error) {
		if outputAdded {
			return nil, nil
		}

		outputAdded = true

		var nullMessageID [iotago.MessageIDLength]byte
		var nullOutputID [iotago.TransactionIDLength + iotago.UInt16ByteSize]byte

		return &snapshot.Output{MessageID: nullMessageID, OutputID: nullOutputID, Address: &address, Amount: iotago.TokenSupply}, nil
	}

	// milestone diffs
	milestoneDiffProducerFunc := func() (*snapshot.MilestoneDiff, error) {
		// no milestone diffs needed
		return nil, nil
	}

	if err := snapshot.StreamLocalSnapshotDataTo(snapshotFile, uint64(time.Now().Unix()), header, solidEntryPointProducerFunc, outputProducerFunc, milestoneDiffProducerFunc); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate snapshot file: %w", err)
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
