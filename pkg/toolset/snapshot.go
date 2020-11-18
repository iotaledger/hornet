package toolset

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go"
	"github.com/pkg/errors"
)

func snapshotGen(args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [NETWORK_ID_STR] [MINT_ADDRESS] [OUTPUT_FILE_PATH]", ToolSnapGen))
		println()
		println("	[NETWORK_ID_STR]	- the network ID for which this snapshot is meant for")
		println("	[MINT_ADDRESS]		- the initial ed25519 address all the tokens will be minted to")
		println("	[OUTPUT_FILE_PATH]	- the file path to the generated snapshot file")
		println()
		println(fmt.Sprintf("example: %s %s %s %s", ToolSnapGen, "alphanet@1", "6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92", "snapshots/alphanet/export.bin"))
	}

	// check arguments
	if len(args) != 3 {
		printUsage()
		return fmt.Errorf("wrong argument count '%s'", ToolSnapGen)
	}

	// check network ID
	networkID := iotago.NetworkIDFromString(args[0])

	// check mint address
	mintAddress := args[1]
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
	outputFilePath := args[2]
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

	// create snapshot file
	targetIndex := 0
	header := &snapshot.FileHeader{
		Version:              snapshot.SupportedFormatVersion,
		Type:                 snapshot.Full,
		NetworkID:            networkID,
		SEPMilestoneIndex:    milestone.Index(targetIndex),
		LedgerMilestoneIndex: milestone.Index(targetIndex),
	}

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

	if err := snapshot.StreamSnapshotDataTo(snapshotFile, uint64(time.Now().Unix()), header, solidEntryPointProducerFunc, outputProducerFunc, milestoneDiffProducerFunc); err != nil {
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
