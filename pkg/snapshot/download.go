package snapshot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/utils"
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Expected    uint64
	shutdownCtx context.Context

	total            uint64
	last             uint64
	lastProgressTime time.Time
}

// NewWriteCounter creates a new WriteCounter.
func NewWriteCounter(expected uint64, shutdownCtx context.Context) *WriteCounter {
	return &WriteCounter{
		Expected:    expected,
		shutdownCtx: shutdownCtx,
	}
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.total += uint64(n)

	if err := utils.ReturnErrIfCtxDone(wc.shutdownCtx, common.ErrOperationAborted); err != nil {
		return n, ErrSnapshotDownloadWasAborted
	}

	wc.PrintProgress()
	return n, nil
}

// PrintProgress prints the current progress.
func (wc *WriteCounter) PrintProgress() {
	if time.Since(wc.lastProgressTime) < 1*time.Second {
		return
	}

	bytesPerSecond := uint64(float64(wc.total-wc.last) / time.Since(wc.lastProgressTime).Seconds())
	wc.lastProgressTime = time.Now()
	wc.last = wc.total

	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 60))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s/%s (%s/s)", humanize.Bytes(wc.total), humanize.Bytes(wc.Expected), humanize.Bytes(bytesPerSecond))
}

// DownloadSnapshotFile downloads a snapshot file by examining the given URLs.
func (s *Snapshot) DownloadSnapshotFile(filepath string, urls []string) error {

	// Try to download a snapshot from one of the provided sources, break if download was successful
	downloadOK := false
	for _, url := range urls {
		s.log.Infof("Downloading snapshot from %s", url)

		// Create the file, but give it a tmp file extension, this means we won't overwrite a
		// file until it's downloaded, but we'll remove the tmp extension once downloaded.
		out, err := os.Create(filepath + ".tmp")
		if err != nil {
			return err
		}

		// Get the data
		resp, err := http.Get(url)
		if err != nil {
			s.log.Warnf("Downloading snapshot from %s failed with %v", url, err)
			out.Close()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			s.log.Warnf("Downloading snapshot from %s failed. Server returned %d", url, resp.StatusCode)
			out.Close()
			continue
		}

		defer resp.Body.Close()

		// Create our progress reporter and pass it to be used alongside our writer
		counter := NewWriteCounter(uint64(resp.ContentLength), s.shutdownCtx)
		if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
			s.log.Warnf("Downloading snapshot from %s failed with %v", url, err)
			out.Close()
			continue
		}

		// The progress use the same line so print a new line once it's finished downloading
		fmt.Print("\n")

		downloadOK = true

		// Close the file without defer so it can happen before Rename()
		out.Close()
		break
	}

	// No download possible
	if !downloadOK {
		return ErrSnapshotDownloadNoValidSource
	}

	if err := os.Rename(filepath+".tmp", filepath); err != nil {
		return err
	}
	return nil
}
