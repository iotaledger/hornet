package toolset

import (
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	snapCore "github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/snapshot"
)

const (
	belowMaxDepth milestone.Index = 15
)

func databaseSnapshot(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	snapshotPathTargetFlag := fs.String(FlagToolSnapshotPathTarget, "", "the path to the target snapshot file")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	targetIndexFlag := fs.Uint32(FlagToolDatabaseTargetIndex, 0, "the target index")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseSnapshot)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s --%s %s",
			ToolDatabaseSnapshot,
			FlagToolSnapshotPathTarget,
			"full_snapshot.bin",
			FlagToolDatabasePathSource,
			DefaultValueMainnetDatabasePath,
			FlagToolDatabaseTargetIndex,
			"100",
		))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*snapshotPathTargetFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPathTarget)
	}
	if len(*databasePathSourceFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePathSource)
	}

	solidEntryPointCheckThresholdPast := milestone.Index(belowMaxDepth + snapCore.SolidEntryPointCheckAdditionalThresholdPast)
	solidEntryPointCheckThresholdFuture := milestone.Index(belowMaxDepth + snapCore.SolidEntryPointCheckAdditionalThresholdFuture)

	tangleStoreSource, err := getTangleStorage(*databasePathSourceFlag, "source", string(database.EngineAuto), true, true, true, true, true)
	if err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Println("creating full snapshot file...")
	}

	ts := time.Now()

	readFileHeader, err := snapshot.CreateSnapshotFromStorage(
		getGracefulStopContext(),
		tangleStoreSource,
		tangleStoreSource.UTXOManager(),
		*snapshotPathTargetFlag,
		milestone.Index(*targetIndexFlag),
		solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture,
	)
	if err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("metadata:\n")
	}

	if err := printSnapshotHeaderInfo("", *snapshotPathTargetFlag, readFileHeader, *outputJSONFlag); err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("successfully created full snapshot '%s', took %v\n", *snapshotPathTargetFlag, time.Since(ts).Truncate(time.Millisecond))
	}

	return nil
}
