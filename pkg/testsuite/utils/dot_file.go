package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"testing"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// ShortenedHash returns a shortened hex encoded hash for the given hash.
// this is used for the dot file.
func ShortenedHash(hash *hornet.MessageID) string {
	hexHash := hash.Hex()
	return hexHash[0:4] + "..." + hexHash[len(hexHash)-4:]
}

// ShortenedIndex returns a shortened index or milestone index for the given message.
// this is used for the dot file.
func ShortenedIndex(cachedMessage *tangle.CachedMessage) string {
	defer cachedMessage.Release(true)

	ms := cachedMessage.GetMessage().GetMilestone()
	if ms != nil {
		return fmt.Sprintf("%d", ms.Index)
	}

	indexation := cachedMessage.GetMessage().GetIndexation()

	index := indexation.Index
	if len(index) > 4 {
		index = index[:4]
	}
	return index
}

// ShowDotFile creates a png file with dot and shows it in an external application.
func ShowDotFile(t *testing.T, dotCommand string, outFilePath string) {

	cmd := exec.Command("dot", "-Tpng", "-o"+outFilePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	stdin.Write([]byte(dotCommand))
	stdin.Close()

	cmd.Wait()

	switch os := runtime.GOOS; os {
	case "darwin":
		if err := exec.Command("open", outFilePath).Start(); err != nil {
			t.Fatal(err)
		}
	case "linux":
		if err := exec.Command("xdg-open", outFilePath).Start(); err != nil {
			t.Fatal(err)
		}
	default:
		// freebsd, openbsd, plan9, windows...
		t.Fatalf("OS %s not supported", os)
	}
}
