package toolset

import (
	"encoding/hex"
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

const (
	FlagToolSnapshotMergeSnapshotPathTarget = "snapshotPathTarget"
)

func snapshotMerge(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	snapshotPathFullFlag := fs.String(FlagToolSnapshotPathFull, "", "the path to the full snapshot file")
	snapshotPathDeltaFlag := fs.String(FlagToolSnapshotPathDelta, "", "the path to the delta snapshot file")
	snapshotPathTargetFlag := fs.String(FlagToolSnapshotMergeSnapshotPathTarget, "", "the path to the target/merged snapshot file")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapMerge)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s",
			ToolSnapMerge,
			FlagToolSnapshotPathFull,
			"snapshots/mainnet/full_snapshot.bin",
			FlagToolSnapshotPathDelta,
			"snapshots/mainnet/delta_snapshot.bin",
			FlagToolSnapshotMergeSnapshotPathTarget,
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
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotMergeSnapshotPathTarget)
	}

	var fullPath, deltaPath, targetPath = *snapshotPathFullFlag, *snapshotPathDeltaFlag, *snapshotPathTargetFlag

	if !*outputJSONFlag {
		fmt.Println("merging snapshot files...")
	}

	ts := time.Now()

	mergeInfo, err := snapshot.MergeSnapshotsFiles(fullPath, deltaPath, targetPath, nil)
	if err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("metadata:\n")
	}

	_ = printSnapshotHeaderInfo("full", fullPath, mergeInfo.FullSnapshotHeader, *outputJSONFlag)
	_ = printSnapshotHeaderInfo("delta", deltaPath, mergeInfo.DeltaSnapshotHeader, *outputJSONFlag)
	_ = printSnapshotHeaderInfo("merged", targetPath, mergeInfo.MergedSnapshotHeader, *outputJSONFlag)

	if !*outputJSONFlag {
		fmt.Printf("successfully created merged full snapshot '%s', took %v\n", targetPath, time.Since(ts).Truncate(time.Millisecond))
	}

	return nil
}

// prints information about the given snapshot file header.
func printSnapshotHeaderInfo(name string, path string, header *snapshot.ReadFileHeader, outputJSON bool) error {

	if outputJSON {

		type treasuryStruct struct {
			MilestoneID string `json:"milestoneID"`
			Tokens      uint64 `json:"tokens"`
		}

		var treasury *treasuryStruct
		if header.TreasuryOutput != nil {
			treasury = &treasuryStruct{
				MilestoneID: hex.EncodeToString(header.TreasuryOutput.MilestoneID[:]),
				Tokens:      header.TreasuryOutput.Amount,
			}
		}

		result := struct {
			SnapshotName        string          `json:"snapshotName,omitempty"`
			FilePath            string          `json:"filePath"`
			SnapshotTime        time.Time       `json:"snapshotTime"`
			NetworkID           uint64          `json:"networkID"`
			Treasury            *treasuryStruct `json:"treasury"`
			LedgerIndex         milestone.Index `json:"ledgerIndex"`
			SnapshotIndex       milestone.Index `json:"snapshotIndex"`
			UTXOsCount          uint64          `json:"UTXOsCount"`
			SEPsCount           uint64          `json:"SEPsCount"`
			MilestoneDiffsCount uint64          `json:"milestoneDiffsCount"`
		}{
			SnapshotName:        name,
			FilePath:            path,
			SnapshotTime:        time.Unix(int64(header.Timestamp), 0),
			NetworkID:           header.NetworkID,
			Treasury:            treasury,
			LedgerIndex:         header.LedgerMilestoneIndex,
			SnapshotIndex:       header.SEPMilestoneIndex,
			UTXOsCount:          header.OutputCount,
			SEPsCount:           header.SEPCount,
			MilestoneDiffsCount: header.MilestoneDiffCount,
		}

		return printJSON(result)
	}

	snapshotNameString := ""
	if name != "" {
		snapshotNameString = fmt.Sprintf(`
         - Snapshot name:  %s\n`, name)
	}

	fmt.Printf(`    >%s
        - File path:      %s
        - Snapshot time:  %v
        - Network ID:     %d
        - Treasury:       %s
        - Ledger index:   %d
        - Snapshot index: %d
        - UTXOs count:    %d
        - SEPs count:     %d
        - Milestone diffs count: %d`+"\n",
		snapshotNameString,
		path,
		time.Unix(int64(header.Timestamp), 0),
		header.NetworkID,
		func() string {
			if header.TreasuryOutput == nil {
				return "no treasury output in header"
			}
			return fmt.Sprintf("milestone ID %s, tokens %d", hex.EncodeToString(header.TreasuryOutput.MilestoneID[:]), header.TreasuryOutput.Amount)
		}(),
		header.LedgerMilestoneIndex,
		header.SEPMilestoneIndex,
		header.OutputCount,
		header.SEPCount,
		header.MilestoneDiffCount,
	)

	return nil
}
