package snapshot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/dustin/go-humanize"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	timeoutDownloadSnapshotHeader = 5 * time.Second
	timeoutDownloadSnapshotFile   = 10 * time.Minute
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

func (s *Snapshot) filterTargets(wantedNetworkID uint64, targets []*DownloadTarget) []*DownloadTarget {

	// check if the remote snapshot files fit the network ID and if delta fits the full snapshot.
	checkTargetConsistency := func(wantedNetworkID uint64, fullHeader *ReadFileHeader, deltaHeader *ReadFileHeader) error {
		if fullHeader == nil {
			return errors.New("full snapshot header not found")
		}

		if fullHeader.NetworkID != wantedNetworkID {
			return fmt.Errorf("full snapshot networkID does not match (%d != %d): %w", fullHeader.NetworkID, wantedNetworkID, ErrInvalidSnapshotAvailabilityState)
		}

		if deltaHeader == nil {
			return nil
		}

		if deltaHeader.NetworkID != wantedNetworkID {
			return fmt.Errorf("delta snapshot networkID does not match (%d != %d): %w", deltaHeader.NetworkID, wantedNetworkID, ErrInvalidSnapshotAvailabilityState)
		}

		if fullHeader.SEPMilestoneIndex > deltaHeader.SEPMilestoneIndex {
			return fmt.Errorf("full snapshot SEP index is bigger than delta snapshot SEP index (%d > %d): %w", fullHeader.SEPMilestoneIndex, deltaHeader.SEPMilestoneIndex, ErrInvalidSnapshotAvailabilityState)
		}

		if fullHeader.SEPMilestoneIndex != deltaHeader.LedgerMilestoneIndex {
			// delta snapshot file doesn't fit the full snapshot file
			return fmt.Errorf("full snapshot SEP index does not match the delta snapshot ledger index (%d != %d): %w", fullHeader.SEPMilestoneIndex, deltaHeader.LedgerMilestoneIndex, ErrInvalidSnapshotAvailabilityState)
		}

		return nil
	}

	type downloadTargetWithIndex struct {
		target *DownloadTarget
		index  milestone.Index
	}

	filteredTargets := []*downloadTargetWithIndex{}

	// search the latest snapshot by scanning all target headers
	for _, target := range targets {
		s.log.Debugf("downloading full snapshot header from %s", target.Full)

		fullHeader, err := s.downloadHeader(target.Full)
		if err != nil {
			// as the full snapshot URL failed to download, we commence further with our targets
			s.log.Debugf("downloading full snapshot header from %s failed: %s", target.Full, err)
			continue
		}

		var deltaHeader *ReadFileHeader
		if len(target.Delta) > 0 {
			s.log.Debugf("downloading delta snapshot header from %s", target.Delta)
			deltaHeader, err = s.downloadHeader(target.Delta)
			if err != nil {
				// it is valid that no delta snapshot file is available on the target.
				s.log.Debugf("downloading delta snapshot header from %s failed: %s", target.Delta, err)
			}
		}

		if err = checkTargetConsistency(wantedNetworkID, fullHeader, deltaHeader); err != nil {
			// the snapshots on the target do not seem to be consistent
			s.log.Infof("snapshot consistency check failed (full: %s, delta: %s): %s", target.Full, target.Delta, err)
			continue
		}

		filteredTargets = append(filteredTargets, &downloadTargetWithIndex{
			target: target,
			index:  getSnapshotFilesLedgerIndex(fullHeader, deltaHeader),
		})
	}

	// sort by snapshot index, latest index first
	sort.Slice(filteredTargets, func(i int, j int) bool {
		return filteredTargets[i].index > filteredTargets[j].index
	})

	results := make([]*DownloadTarget, len(filteredTargets))
	for i := 0; i < len(filteredTargets); i++ {
		results[i] = filteredTargets[i].target
	}

	return results
}

// DownloadSnapshotFiles tries to download snapshots files from the given targets.
func (s *Snapshot) DownloadSnapshotFiles(wantedNetworkID uint64, fullPath string, deltaPath string, targets []*DownloadTarget) error {

	for _, target := range s.filterTargets(wantedNetworkID, targets) {

		s.log.Infof("downloading full snapshot file from %s", target.Full)
		if err := s.downloadFile(fullPath, target.Full); err != nil {
			s.log.Warn(err)
			// as the full snapshot URL failed to download, we commence further with our targets
			continue
		}

		if len(target.Delta) > 0 {
			s.log.Infof("downloading delta snapshot file from %s", target.Delta)
			if err := s.downloadFile(deltaPath, target.Delta); err != nil {
				// it is valid that no delta snapshot file is available on the target.
				s.log.Warn(err)
			}
		}
		return nil
	}

	return ErrSnapshotDownloadNoValidSource
}

// downloads a snapshot header from the given url.
func (s *Snapshot) downloadHeader(url string) (*ReadFileHeader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDownloadSnapshotHeader)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed, server returned status code %d", resp.StatusCode)
	}

	return ReadSnapshotHeader(resp.Body)
}

// downloads a snapshot file from the given url to the specified path.
func (s *Snapshot) downloadFile(path string, url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutDownloadSnapshotFile)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, server returned status code %d", resp.StatusCode)
	}

	tempFileName := path + ".tmp"
	out, err := os.Create(tempFileName)
	if err != nil {
		return err
	}

	var ok bool
	defer func() {
		if !ok {
			// we don't need to check the error, maybe the file doesn't exist
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
	if err = os.Rename(tempFileName, path); err != nil {
		return fmt.Errorf("unable to rename downloaded snapshot file: %w", err)
	}

	ok = true
	return nil
}
