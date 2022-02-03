package toolset

import (
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/snapshot"
	"github.com/iotaledger/hive.go/configuration"
)

func snapshotInfo(_ *configuration.Configuration, args []string) error {
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	snapshotPathFlag := fs.String(FlagToolSnapshotPath, "", "the path to the snapshot file")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolSnapInfo)
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
	readFileHeader, err := snapshot.ReadSnapshotHeaderFromFile(filePath)
	if err != nil {
		return err
	}

	return printSnapshotHeaderInfo("", filePath, readFileHeader, *outputJSONFlag)
}
