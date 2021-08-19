package toolset

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v2"
)

func snapshotGen(_ *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [NETWORK_ID_STR] [MINT_ADDRESS] [TREASURY_ALLOCATION] [OUTPUT_FILE_PATH]", ToolSnapGen))
		println()
		println("	[NETWORK_ID_STR]		- the network ID for which this snapshot is meant for")
		println("	[MINT_ADDRESS]			- the initial ed25519 address all the tokens will be minted to")
		println("	[TREASURY_ALLOCATION]	- the amount of tokens to reside within the treasury, the delta from the supply will be allocated to MINT_ADDRESS")
		println("	[OUTPUT_FILE_PATH]		- the file path to the generated snapshot file")
		println()
		println(fmt.Sprintf("example: %s %s %s %s %s", ToolSnapGen, "private_tangle@1", "6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92", "500000000", "snapshots/private_tangle/full_snapshot.bin"))
	}

	// check arguments
	if len(args) != 4 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolSnapGen)
	}

	// check network ID
	networkID := iotago.NetworkIDFromString(args[0])

	// check mint address
	mintAddress := args[1]
	addressBytes, err := hex.DecodeString(mintAddress)
	if err != nil {
		return fmt.Errorf("can't decode MINT_ADDRESS: %w", err)
	}
	if len(addressBytes) != iotago.Ed25519AddressBytesLength {
		return fmt.Errorf("incorrect MINT_ADDRESS length: %d != %d (%s)", len(addressBytes), iotago.Ed25519AddressBytesLength, mintAddress)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	// check treasury
	treasury, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return fmt.Errorf("unable to decode TREASURY_ALLOCATION: %w", err)
	}

	// check filepath
	outputFilePath := args[3]
	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return errors.New("OUTPUT_FILE_PATH already exists")
	}

	// build temp file path
	outputFilePathTmp := outputFilePath + "_tmp"

	// we don't need to check the error, maybe the file doesn't exist
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
		TreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      treasury,
		},
	}

	// solid entry points
	// add "NullMessageID" as sole entry point
	nullHashAdded := false
	solidEntryPointProducerFunc := func() (hornet.MessageID, error) {
		if nullHashAdded {
			return nil, nil
		}

		nullHashAdded = true

		return hornet.NullMessageID(), nil
	}

	// unspent transaction outputs
	outputAdded := false
	outputProducerFunc := func() (*snapshot.Output, error) {
		if outputAdded {
			return nil, nil
		}

		outputAdded = true

		var nullMessageID [iotago.MessageIDLength]byte
		var nullOutputID [utxo.OutputIDLength]byte

		return &snapshot.Output{
			MessageID:  nullMessageID,
			OutputID:   nullOutputID,
			OutputType: iotago.OutputSigLockedSingleOutput,
			Address:    &address,
			Amount:     iotago.TokenSupply - treasury,
		}, nil
	}

	// milestone diffs
	milestoneDiffProducerFunc := func() (*snapshot.MilestoneDiff, error) {
		// no milestone diffs needed
		return nil, nil
	}

	if _, err := snapshot.StreamSnapshotDataTo(snapshotFile, uint64(time.Now().Unix()), header, solidEntryPointProducerFunc, outputProducerFunc, milestoneDiffProducerFunc); err != nil {
		_ = snapshotFile.Close()
		return fmt.Errorf("couldn't generate snapshot file: %w", err)
	}

	if err := snapshotFile.Close(); err != nil {
		return fmt.Errorf("unable to close snapshot file: %w", err)
	}

	// rename tmp file to final file name
	if err := os.Rename(outputFilePathTmp, outputFilePath); err != nil {
		return fmt.Errorf("unable to rename temp snapshot file: %w", err)
	}

	fmt.Println("Snapshot creation successful!")
	return nil
}
