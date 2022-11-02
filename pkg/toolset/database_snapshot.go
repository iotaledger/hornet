package toolset

import (
	"fmt"
	"os"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	hivedb "github.com/iotaledger/hive.go/core/database"
	snapCore "github.com/iotaledger/hornet/v2/core/snapshot"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	belowMaxDepth iotago.MilestoneIndex = 15
)

func databaseSnapshot(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	snapshotPathTargetFlag := fs.String(FlagToolSnapshotPathTarget, "", "the path to the target snapshot file")
	databasePathSourceFlag := fs.String(FlagToolDatabasePathSource, "", "the path to the source database")
	targetIndexFlag := fs.Uint32(FlagToolDatabaseTargetIndex, 0, "the target index")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)
	globalSnapshotFlag := fs.Bool(FlagToolSnapshotGlobal, false, "create a global snapshot (SEP equal to milestone parents)")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolDatabaseSnapshot)
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

	solidEntryPointCheckThresholdPast := belowMaxDepth + snapCore.SolidEntryPointCheckAdditionalThresholdPast
	solidEntryPointCheckThresholdFuture := belowMaxDepth + snapCore.SolidEntryPointCheckAdditionalThresholdFuture

	tangleStoreSource, err := getTangleStorage(*databasePathSourceFlag, "source", string(hivedb.EngineAuto), true, true, true, true)
	if err != nil {
		return err
	}
	defer func() {
		println("\nshutdown source storage ...")
		if err := tangleStoreSource.Shutdown(); err != nil {
			panic(err)
		}
	}()

	if !*outputJSONFlag {
		fmt.Println("creating full snapshot file ...")
	}

	ts := time.Now()

	fullHeader, err := snapshot.CreateSnapshotFromStorage(
		getGracefulStopContext(),
		tangleStoreSource,
		tangleStoreSource.UTXOManager(),
		*snapshotPathTargetFlag,
		*targetIndexFlag,
		*globalSnapshotFlag,
		solidEntryPointCheckThresholdPast,
		solidEntryPointCheckThresholdFuture,
	)
	if err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("metadata:\n")
	}

	if err := printFullSnapshotHeaderInfo("", *snapshotPathTargetFlag, fullHeader); err != nil {
		return err
	}

	if !*outputJSONFlag {
		fmt.Printf("successfully created full snapshot '%s', took %v\n", *snapshotPathTargetFlag, time.Since(ts).Truncate(time.Millisecond))
	}

	return nil
}
