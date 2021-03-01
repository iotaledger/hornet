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
	shutdownCtx context.Context
	Expected    uint64

	total            uint64
	last             uint64
	lastProgressTime time.Time
}

// NewWriteCounter creates a new WriteCounter.
func NewWriteCounter(shutdownCtx context.Context, expected uint64) *WriteCounter {
	return &WriteCounter{
		shutdownCtx: shutdownCtx,
		Expected:    expected,
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

	// clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 60))

	// return again and print current status of download
	// we use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s/%s (%s/s)", humanize.Bytes(wc.total), humanize.Bytes(wc.Expected), humanize.Bytes(bytesPerSecond))
}

// DownloadTarget holds URLs to a full and delta snapshot.
type DownloadTarget struct {
	// URL of the full snapshot file.
	Full string `json:"full"`
	// URL of the delta snapshot file.
	Delta string `json:"delta"`
}

// DownloadSnapshotFiles tries to download snapshots file from the given targets.
func (s *Snapshot) DownloadSnapshotFiles(fullPath string, deltaPath string, targets []DownloadTarget) error {

	for _, target := range targets {

		s.log.Infof("downloading full snapshot file from %s", target.Full)
		if err := s.downloadFile(fullPath, target.Full); err != nil {
			s.log.Warn(err.Error())
			continue
		}

		if len(target.Delta) > 0 {
			s.log.Infof("downloading delta snapshot file from %s", target.Delta)
			if err := s.downloadFile(deltaPath, target.Delta); err != nil {
				s.log.Warn(err.Error())
				// as the delta snapshot URL was defined but it failed to download,
				// we delete the downloaded full snapshot and commence further with our targets
				if err := os.Remove(fullPath); err != nil {
					return fmt.Errorf("unable to remove full snapshot file, after failed companion delta snapshot file download: %w", err)
				}
				continue
			}
		}
		return nil
	}

	return ErrSnapshotDownloadNoValidSource
}

// downloads a snapshot file from the given url to the specified path.
func (s *Snapshot) downloadFile(path string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, server returned status code %d", resp.StatusCode)
	}

	defer resp.Body.Close()

	tempFileName := path + ".tmp"
	out, err := os.Create(tempFileName)
	if err != nil {
		return err
	}

	var ok bool
	defer func() {
		if !ok {
			_ = os.Remove(tempFileName)
		}
	}()

	// create our progress reporter and pass it to be used alongside our writer
	counter := NewWriteCounter(s.shutdownCtx, uint64(resp.ContentLength))
	if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
		_ = out.Close()
		return fmt.Errorf("download failed: %w", err)
	}

	// the progress indicator uses the same line so print a new line once it's finished downloading
	fmt.Print("\n")

	_ = out.Close()
	if err := os.Rename(tempFileName, path); err != nil {
		return fmt.Errorf("unable to rename downloaded snapshot file: %w", err)
	}

	ok = true
	return nil
}
