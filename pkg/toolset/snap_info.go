package toolset

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hornet/v2/pkg/snapshot"
)

func snapshotInfo(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	snapshotPathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the snapshot file")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapInfo)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolSnapInfo,
			FlagToolSnapshotPath,
			"snapshots/mainnet/full_snapshot.bin"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*snapshotPathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSnapshotPath)
	}

	filePath := *snapshotPathFlag

	snapshotType, err := snapshot.ReadSnapshotTypeFromFile(filePath)
	if err != nil {
		return err
	}

	switch snapshotType {
	case snapshot.Full:
		fullHeader, err := snapshot.ReadFullSnapshotHeaderFromFile(filePath)
		if err != nil {
			return err
		}

		return printFullSnapshotHeaderInfo("", filePath, fullHeader)

	case snapshot.Delta:
		deltaHeader, err := snapshot.ReadDeltaSnapshotHeaderFromFile(filePath)
		if err != nil {
			return err
		}

		return printDeltaSnapshotHeaderInfo("", filePath, deltaHeader)

	default:
		return fmt.Errorf("unknown snapshot type: %d", snapshotType)
	}
}
