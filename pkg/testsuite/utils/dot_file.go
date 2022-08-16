//nolint:forcetypeassert,varnamelen,revive,exhaustruct,gosec // we don't care about these linters in test cases
package utils

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ShortenedHash returns a shortened hex encoded hash for the given hash.
// this is used for the dot file.
func ShortenedHash(hash iotago.BlockID) string {
	hexHash := hash.ToHex()

	return hexHash[0:4] + "..." + hexHash[len(hexHash)-4:]
}

// ShortenedTag returns a shortened tag or milestone index for the given block.
// this is used for the dot file.
func ShortenedTag(cachedBlock *storage.CachedBlock) string {
	defer cachedBlock.Release(true) // block -1

	hash := ShortenedHash(cachedBlock.Block().BlockID())

	milestonePayload := cachedBlock.Block().Milestone()
	if milestonePayload != nil {
		return fmt.Sprintf("%d-%s", milestonePayload.Index, hash)
	}

	taggedData := cachedBlock.Block().TransactionEssenceTaggedData()
	if taggedData == nil {
		taggedData = cachedBlock.Block().TaggedData()
	}
	if taggedData == nil {
		panic("no taggedData found")
	}

	tag := taggedData.Tag
	tagString := string(tag)
	if len(tag) > 4 {
		tagString = hex.EncodeToString(tag[:4])
	}

	if cachedBlock.Metadata().IsConflictingTx() {
		conflict := cachedBlock.Metadata().Conflict()

		return fmt.Sprintf("%s (%d)", tagString, conflict)
	}

	return fmt.Sprintf("%s-%s", tagString, hash)
}

// ShowDotFile creates a png file with dot and shows it in an external application.
func ShowDotFile(testInterface testing.TB, dotCommand string, outFilePath string) {

	cmd := exec.Command("dot", "-Tpng", "-o"+outFilePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		testInterface.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		testInterface.Fatal(err)
	}

	if _, err := stdin.Write([]byte(dotCommand)); err != nil {
		testInterface.Fatal(err)
	}

	if err := stdin.Close(); err != nil {
		testInterface.Fatal(err)
	}

	if err := cmd.Wait(); err != nil {
		testInterface.Fatal(err)
	}

	switch os := runtime.GOOS; os {
	case "darwin":
		if err := exec.Command("open", outFilePath).Start(); err != nil {
			testInterface.Fatal(err)
		}
	case "linux":
		if err := exec.Command("xdg-open", outFilePath).Start(); err != nil {
			testInterface.Fatal(err)
		}
	default:
		// freebsd, openbsd, plan9, windows...
		testInterface.Fatalf("OS %s not supported", os)
	}
}
