package toolset

import (
	"context"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

func snapshotMerge(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	snapshotPathFullFlag := fs.String(FlagToolSnapshotPathFull, "", "the path to the full snapshot file")
	snapshotPathDeltaFlag := fs.String(FlagToolSnapshotPathDelta, "", "the path to the delta snapshot file")
	snapshotPathTargetFlag := fs.String(FlagToolSnapshotPathTarget, "", "the path to the target/merged snapshot file")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapMerge)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s",
			ToolSnapMerge,
			FlagToolSnapshotPathFull,
			"snapshots/mainnet/full_snapshot.bin",
			FlagToolSnapshotPathDelta,
			"snapshots/mainnet/delta_snapshot.bin",
			FlagToolSnapshotPathTarget,
			"merged_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*snapshotPathFullFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPathFull)
	}
	if len(*snapshotPathDeltaFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPathDelta)
	}
	if len(*snapshotPathTargetFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPathTarget)
	}

	var fullPath, deltaPath, targetPath = *snapshotPathFullFlag, *snapshotPathDeltaFlag, *snapshotPathTargetFlag

	if !*outputJSONFlag {
		fmt.Println("merging snapshot files ...")
	}

	ts := time.Now()

	mergeInfo, err := snapshot.MergeSnapshotsFiles(context.Background(), fullPath, deltaPath, targetPath)
	if err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("metadata:\n")
	}

	_ = printFullSnapshotHeaderInfo("full", fullPath, mergeInfo.FullSnapshotHeader)
	_ = printDeltaSnapshotHeaderInfo("delta", deltaPath, mergeInfo.DeltaSnapshotHeader)
	_ = printFullSnapshotHeaderInfo("merged", targetPath, mergeInfo.MergedSnapshotHeader)

	if !*outputJSONFlag {
		fmt.Printf("successfully created merged full snapshot '%s', took %v\n", targetPath, time.Since(ts).Truncate(time.Millisecond))
	}

	return nil
}

// prints information about the given full snapshot file header.
func printFullSnapshotHeaderInfo(name string, path string, fullHeader *snapshot.FullSnapshotHeader) error {

	fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
	if err != nil {
		return err
	}

	result := struct {
		SnapshotName             string                     `json:"snapshotName,omitempty"`
		FilePath                 string                     `json:"filePath"`
		Version                  byte                       `json:"version"`
		Type                     string                     `json:"type"`
		GenesisMilestoneIndex    iotago.MilestoneIndex      `json:"genesisMilestoneIndex"`
		TargetMilestoneIndex     iotago.MilestoneIndex      `json:"targetMilestoneIndex"`
		TargetMilestoneTimestamp time.Time                  `json:"targetMilestoneTimestamp"`
		TargetMilestoneID        string                     `json:"targetMilestoneId"`
		LedgerMilestoneIndex     iotago.MilestoneIndex      `json:"ledgerMilestoneIndex"`
		TreasuryOutput           *utxo.TreasuryOutput       `json:"treasuryOutput"`
		ProtocolParameters       *iotago.ProtocolParameters `json:"protocolParameters"`
		OutputCount              uint64                     `json:"outputCount"`
		MilestoneDiffCount       uint32                     `json:"milestoneDiffCount"`
		SolidEntryPointsCount    uint16                     `json:"solidEntryPointsCount"`
	}{
		SnapshotName:             name,
		FilePath:                 path,
		Version:                  fullHeader.Version,
		Type:                     "full",
		GenesisMilestoneIndex:    fullHeader.GenesisMilestoneIndex,
		TargetMilestoneIndex:     fullHeader.TargetMilestoneIndex,
		TargetMilestoneTimestamp: time.Unix(int64(fullHeader.TargetMilestoneTimestamp), 0),
		TargetMilestoneID:        fullHeader.TargetMilestoneID.ToHex(),
		LedgerMilestoneIndex:     fullHeader.LedgerMilestoneIndex,
		TreasuryOutput:           fullHeader.TreasuryOutput,
		ProtocolParameters:       fullHeaderProtoParams,
		OutputCount:              fullHeader.OutputCount,
		MilestoneDiffCount:       fullHeader.MilestoneDiffCount,
		SolidEntryPointsCount:    fullHeader.SEPCount,
	}

	return printJSON(result)
}

// prints information about the given delta snapshot file header.
func printDeltaSnapshotHeaderInfo(name string, path string, deltaHeader *snapshot.DeltaSnapshotHeader) error {

	result := struct {
		SnapshotName                  string                `json:"snapshotName,omitempty"`
		FilePath                      string                `json:"filePath"`
		Version                       byte                  `json:"version"`
		Type                          string                `json:"type"`
		TargetMilestoneIndex          iotago.MilestoneIndex `json:"targetMilestoneIndex"`
		TargetMilestoneTimestamp      time.Time             `json:"targetMilestoneTimestamp"`
		FullSnapshotTargetMilestoneID string                `json:"fullSnapshotTargetMilestoneId"`
		SolidEntryPointsFileOffset    int64                 `json:"solidEntryPointsFileOffset"`
		MilestoneDiffCount            uint32                `json:"milestoneDiffCount"`
		SolidEntryPointsCount         uint16                `json:"solidEntryPointsCount"`
	}{
		SnapshotName:                  name,
		FilePath:                      path,
		Version:                       deltaHeader.Version,
		Type:                          "delta",
		TargetMilestoneIndex:          deltaHeader.TargetMilestoneIndex,
		TargetMilestoneTimestamp:      time.Unix(int64(deltaHeader.TargetMilestoneTimestamp), 0),
		FullSnapshotTargetMilestoneID: deltaHeader.FullSnapshotTargetMilestoneID.ToHex(),
		SolidEntryPointsFileOffset:    deltaHeader.SEPFileOffset,
		MilestoneDiffCount:            deltaHeader.MilestoneDiffCount,
		SolidEntryPointsCount:         deltaHeader.SEPCount,
	}

	return printJSON(result)
}
