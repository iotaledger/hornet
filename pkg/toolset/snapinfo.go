package toolset

import (
	"fmt"

	"github.com/gohornet/hornet/pkg/snapshot"
)

func snapshotInfo(args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [SNAPSHOT_PATH]", ToolSnapInfo))
		println()
		println("	[SNAPSHOT_PATH]	- the path to the snapshot file")
		println()
		println(fmt.Sprintf("example: %s %s", ToolSnapGen, "./snapshot.bin"))
	}

	if len(args) != 1 {
		printUsage()
		return fmt.Errorf("wrong argument count '%s'", ToolSnapInfo)
	}

	filePath := args[0]
	readFileHeader, err := snapshot.ReadSnapshotHeader(filePath)
	if err != nil {
		return err
	}

	printSnapshotHeaderInfo("", filePath, readFileHeader)
	return nil
}
