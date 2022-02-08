package toolset

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

func snapshotGen(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	networkIDFlag := fs.String(FlagToolNetworkID, "", "the network ID for which this snapshot is meant for")
	mintAddressFlag := fs.String(FlagToolSnapGenMintAddress, "", "the initial ed25519 address all the tokens will be minted to")
	treasuryAllocationFlag := fs.Uint64(FlagToolSnapGenTreasuryAllocation, 0, "the amount of tokens to reside within the treasury, the delta from the supply will be allocated to 'mintAddress'")
	outputFilePathFlag := fs.String(FlagToolOutputPath, "", "the file path to the generated snapshot file")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapGen)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s --%s %s",
			ToolSnapGen,
			FlagToolNetworkID,
			"private_tangle@1",
			FlagToolSnapGenMintAddress,
			"[MINT_ADDRESS]",
			FlagToolSnapGenTreasuryAllocation,
			"500000000",
			FlagToolOutputPath,
			"snapshots/private_tangle/full_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*networkIDFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolNetworkID)
	}

	networkID := iotago.NetworkIDFromString(*networkIDFlag)

	// check mint address
	if len(*mintAddressFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapGenMintAddress)
	}
	addressBytes, err := hex.DecodeString(*mintAddressFlag)
	if err != nil {
		return fmt.Errorf("can't decode '%s': %w'", FlagToolSnapGenMintAddress, err)
	}
	if len(addressBytes) != iotago.Ed25519AddressBytesLength {
		return fmt.Errorf("incorrect '%s' length: %d != %d (%s)", FlagToolSnapGenMintAddress, len(addressBytes), iotago.Ed25519AddressBytesLength, *mintAddressFlag)
	}

	var address iotago.Ed25519Address
	copy(address[:], addressBytes)

	treasury := *treasuryAllocationFlag

	// check filepath
	if len(*outputFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolOutputPath)
	}

	outputFilePath := *outputFilePathFlag
	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("'%s' already exists", FlagToolOutputPath)
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
	outputProducerFunc := func() (*utxo.Output, error) {
		if outputAdded {
			return nil, nil
		}

		outputAdded = true

		return utxo.CreateOutput(&iotago.OutputID{}, hornet.NullMessageID(), 0, 0, &iotago.BasicOutput{
			Amount: iotago.TokenSupply - treasury,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{Address: &address},
			},
		}), nil
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
