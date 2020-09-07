package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// ShortenedHash returns a shortened trinary hash for the given hash.
// this is used for the dot file.
func ShortenedHash(hash hornet.Hash) string {
	trytes := hash.Trytes()
	return trytes[0:4] + "..." + trytes[77:81]
}

// ShortenedTag returns a shortened tag or milestone index for the given bundle.
// this is used for the dot file.
func ShortenedTag(bundle *tangle.CachedBundle) string {
	if bundle.GetBundle().IsMilestone() {
		return fmt.Sprintf("%d", bundle.GetBundle().GetMilestoneIndex())
	}

	tail := bundle.GetBundle().GetTail()
	defer tail.Release(true)

	tag := tail.GetTransaction().Tx.Tag

	// Cut the tags at the first 9 or at max length 4
	tagLength := strings.IndexByte(tag, '9')
	if tagLength == -1 || tagLength > 4 || tagLength == 0 {
		tagLength = 4
	}
	return tag[0:tagLength]
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
