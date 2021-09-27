package toolset

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func snapshotMerge(_ *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [FULL_SNAPSHOT_PATH] [DELTA_SNAPSHOT_PATH] [TARGET_SNAPSHOT_PATH]", ToolSnapMerge))
		println()
		println("	[FULL_SNAPSHOT_PATH]	- the path to the full snapshot file")
		println("	[DELTA_SNAPSHOT_PATH]	- the path to the delta snapshot file")
		println("	[TARGET_SNAPSHOT_PATH]	- the path to the target/merged snapshot file")
		println()
		println(fmt.Sprintf("example: %s %s %s %s", ToolSnapMerge, "./full_snapshot.bin", "./delta_snapshot.bin", "./merged_snapshot.bin"))
	}

	if len(args) != 3 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolSnapMerge)
	}

	ts := time.Now()
	fmt.Println("merging snapshot files...")

	var fullPath, deltaPath, targetPath = args[0], args[1], args[2]
	mergeInfo, err := snapshot.MergeSnapshotsFiles(fullPath, deltaPath, targetPath)
	if err != nil {
		return err
	}

	fmt.Printf("metadata:\n")
	printSnapshotHeaderInfo("full", fullPath, mergeInfo.FullSnapshotHeader)
	printSnapshotHeaderInfo("delta", deltaPath, mergeInfo.DeltaSnapshotHeader)
	printSnapshotHeaderInfo("merged", targetPath, mergeInfo.MergedSnapshotHeader)
	fmt.Printf("successfully created merged full snapshot '%s', took %v\n", args[2], time.Since(ts).Truncate(time.Millisecond))

	return nil
}

// prints information about the given snapshot file header.
func printSnapshotHeaderInfo(name string, path string, header *snapshot.ReadFileHeader) {
	fmt.Printf(`> %s snapshot, file %s:
	- Snapshot time %v
	- Network ID %d
	- Treasury %s
	- Ledger index %d
	- Snapshot index %d
	- UTXOs count %d
	- SEPs count %d
	- Milestone diffs count %d`+"\n", name, path,
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
}
