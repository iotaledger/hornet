package toolset

import (
	"encoding/hex"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/ioutils"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

func snapshotGen(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	protocolParametersPathFlag := fs.String(FlagToolProtocolParametersPath, "", "the path to the initial protocol parameters file")
	mintAddressFlag := fs.String(FlagToolSnapGenMintAddress, "", "the initial ed25519 address all the tokens will be minted to")
	treasuryAllocationFlag := fs.Uint64(FlagToolSnapGenTreasuryAllocation, 0, "the amount of tokens to reside within the treasury, the delta from the supply will be allocated to 'mintAddress'")
	outputFilePathFlag := fs.String(FlagToolOutputPath, "", "the file path to the generated snapshot file")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapGen)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s --%s %s",
			ToolSnapGen,
			FlagToolProtocolParametersPath,
			"protocol_parameters.json",
			FlagToolSnapGenMintAddress,
			"[MINT_ADDRESS]",
			FlagToolSnapGenTreasuryAllocation,
			"500000000",
			FlagToolOutputPath,
			"genesis_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*protocolParametersPathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolProtocolParametersPath)
	}
	if len(*mintAddressFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapGenMintAddress)
	}
	if len(*outputFilePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolOutputPath)
	}

	protocolParametersPath := *protocolParametersPathFlag
	if _, err := os.Stat(protocolParametersPath); err != nil || os.IsNotExist(err) {
		return fmt.Errorf("'%s' (%s) does not exist", FlagToolProtocolParametersPath, protocolParametersPath)
	}

	outputFilePath := *outputFilePathFlag
	if _, err := os.Stat(outputFilePath); err == nil || !os.IsNotExist(err) {
		return fmt.Errorf("'%s' already exists", FlagToolOutputPath)
	}

	println("loading protocol parameters...")
	// TODO: needs to be adapted for when protocol parameters struct changes
	protoParams := &iotago.ProtocolParameters{}
	if err := ioutils.ReadJSONFromFile(protocolParametersPath, protoParams); err != nil {
		return fmt.Errorf("failed to load protocol parameters: %w", err)
	}

	protoParamsBytes, err := protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return fmt.Errorf("failed to serialize protocol parameters: %w", err)
	}

	// check mint address
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

	// build temp file path
	outputFilePathTmp := outputFilePath + "_tmp"

	// we don't need to check the error, maybe the file doesn't exist
	_ = os.Remove(outputFilePathTmp)

	fileHandle, err := os.OpenFile(outputFilePathTmp, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return fmt.Errorf("unable to create snapshot file: %w", err)
	}

	// create snapshot file
	var targetIndex iotago.MilestoneIndex = 0
	fullHeader := &snapshot.FullSnapshotHeader{
		Version:                  snapshot.SupportedFormatVersion,
		Type:                     snapshot.Full,
		GenesisMilestoneIndex:    0,
		TargetMilestoneIndex:     targetIndex,
		TargetMilestoneTimestamp: 0,
		TargetMilestoneID:        iotago.MilestoneID{},
		LedgerMilestoneIndex:     targetIndex,
		TreasuryOutput: &utxo.TreasuryOutput{
			MilestoneID: iotago.MilestoneID{},
			Amount:      treasury,
		},
		ProtocolParamsMilestoneOpt: &iotago.ProtocolParamsMilestoneOpt{
			TargetMilestoneIndex: targetIndex,
			ProtocolVersion:      protoParams.Version,
			Params:               protoParamsBytes,
		},
		OutputCount:        0,
		MilestoneDiffCount: 0,
		SEPCount:           0,
	}

	// solid entry points
	// add "EmptyBlockID" as sole entry point
	nullHashAdded := false
	solidEntryPointProducerFunc := func() (iotago.BlockID, error) {
		if nullHashAdded {
			return iotago.EmptyBlockID(), snapshot.ErrNoMoreSEPToProduce
		}
		nullHashAdded = true
		return iotago.EmptyBlockID(), nil
	}

	// unspent transaction outputs
	outputAdded := false
	outputProducerFunc := func() (*utxo.Output, error) {
		if outputAdded {
			return nil, nil
		}

		outputAdded = true

		return utxo.CreateOutput(iotago.OutputID{}, iotago.EmptyBlockID(), 0, 0, &iotago.BasicOutput{
			Amount: protoParams.TokenSupply - treasury,
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

	if _, err := snapshot.StreamFullSnapshotDataTo(
		fileHandle,
		fullHeader,
		outputProducerFunc,
		milestoneDiffProducerFunc,
		solidEntryPointProducerFunc,
	); err != nil {
		_ = fileHandle.Close()
		return fmt.Errorf("couldn't generate snapshot file: %w", err)
	}

	if err := fileHandle.Close(); err != nil {
		return fmt.Errorf("unable to close snapshot file: %w", err)
	}

	// rename tmp file to final file name
	if err := os.Rename(outputFilePathTmp, outputFilePath); err != nil {
		return fmt.Errorf("unable to rename temp snapshot file: %w", err)
	}

	fmt.Println("Snapshot creation successful!")
	return nil
}
