package utils

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/storage"
)

// ShortenedHash returns a shortened hex encoded hash for the given hash.
// this is used for the dot file.
func ShortenedHash(hash hornet.MessageID) string {
	hexHash := hash.ToHex()
	return hexHash[0:4] + "..." + hexHash[len(hexHash)-4:]
}

// ShortenedTag returns a shortened tag or milestone index for the given message.
// this is used for the dot file.
func ShortenedTag(cachedMessage *storage.CachedMessage) string {
	defer cachedMessage.Release(true)

	ms := cachedMessage.Message().Milestone()
	if ms != nil {
		return fmt.Sprintf("%d", ms.Index)
	}

	taggedData := cachedMessage.Message().TransactionEssenceTaggedData()
	if taggedData == nil {
		taggedData = cachedMessage.Message().TaggedData()
	}
	if taggedData == nil {
		panic("no taggedData found")
	}

	tag := taggedData.Tag
	if len(tag) > 4 {
		tag = tag[:4]
	}
	tagHex := hex.EncodeToString(tag)

	if cachedMessage.Metadata().IsConflictingTx() {
		conflict := cachedMessage.Metadata().Conflict()
		return fmt.Sprintf("%s (%d)", tagHex, conflict)
	}

	return tagHex
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
