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

	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/contextutils"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	timeoutDownloadSnapshotHeader = 5 * time.Second
	timeoutDownloadSnapshotFile   = 10 * time.Minute
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	// context that is done when the node is shutting down.
	ctx      context.Context
	Expected uint64

	total            uint64
	last             uint64
	lastProgressTime time.Time
}

// NewWriteCounter creates a new WriteCounter.
func NewWriteCounter(ctx context.Context, expected uint64) *WriteCounter {
	return &WriteCounter{
		ctx:      ctx,
		Expected: expected,
	}
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.total += uint64(n)

	if err := contextutils.ReturnErrIfCtxDone(wc.ctx, ErrSnapshotDownloadWasAborted); err != nil {
		return n, err
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
	fmt.Printf("\rDownloading ... %s/%s (%s/s)", humanize.Bytes(wc.total), humanize.Bytes(wc.Expected), humanize.Bytes(bytesPerSecond))
}

// DownloadTarget holds URLs to a full and delta snapshot.
type DownloadTarget struct {
	// URL of the full snapshot file.
	Full string `usage:"URL of the full snapshot file" json:"full"`
	// URL of the delta snapshot file.
	Delta string `usage:"URL of the delta snapshot file" json:"delta"`
}

func (s *Importer) filterTargets(ctx context.Context, targetNetworkID uint64, targets []*DownloadTarget) []*DownloadTarget {

	// check if the remote snapshot files fit the network ID and if delta fits the full snapshot.
	checkTargetConsistency := func(targetNetworkID uint64, fullHeader *FullSnapshotHeader, deltaHeader *DeltaSnapshotHeader) error {
		if fullHeader == nil {
			return errors.New("full snapshot header not found")
		}

		fullHeaderProtoParams, err := fullHeader.ProtocolParameters()
		if err != nil {
			return err
		}

		if fullHeaderProtoParams.NetworkID() != targetNetworkID {
			return fmt.Errorf("full snapshot networkID does not match (%d != %d): %w", fullHeaderProtoParams.NetworkID(), targetNetworkID, ErrInvalidSnapshotAvailabilityState)
		}

		if deltaHeader == nil {
			return nil
		}

		if deltaHeader.FullSnapshotTargetMilestoneID != fullHeader.TargetMilestoneID {
			// delta snapshot file doesn't fit the full snapshot file
			return fmt.Errorf("full snapshot target milestone ID of the delta snapshot does not fit the actual full snapshot target milestone ID (%s != %s): %w", deltaHeader.FullSnapshotTargetMilestoneID.ToHex(), fullHeader.TargetMilestoneID.ToHex(), ErrDeltaSnapshotIncompatible)
		}

		return nil
	}

	type downloadTargetWithIndex struct {
		target *DownloadTarget
		index  iotago.MilestoneIndex
	}

	filteredTargets := []*downloadTargetWithIndex{}

	// search the latest snapshot by scanning all target headers
	for _, target := range targets {
		s.LogDebugf("downloading full snapshot header from %s", target.Full)

		var fullHeader *FullSnapshotHeader
		if err := s.downloadHeader(ctx, target.Full, func(readCloser io.ReadCloser) error {
			fullSnapshotHeader, err := ReadFullSnapshotHeader(readCloser)
			if err != nil {
				return err
			}

			fullHeader = fullSnapshotHeader

			return nil
		}); err != nil {
			// as the full snapshot URL failed to download, we commence further with our targets
			s.LogDebugf("downloading full snapshot header from %s failed: %s", target.Full, err)

			continue
		}

		var deltaHeader *DeltaSnapshotHeader
		if len(target.Delta) > 0 {
			s.LogDebugf("downloading delta snapshot header from %s", target.Delta)
			if err := s.downloadHeader(ctx, target.Delta, func(readCloser io.ReadCloser) error {
				deltaSnapshotHeader, err := ReadDeltaSnapshotHeader(readCloser)
				if err != nil {
					return err
				}

				deltaHeader = deltaSnapshotHeader

				return nil
			}); err != nil {
				// it is valid that no delta snapshot file is available on the target.
				s.LogDebugf("downloading delta snapshot header from %s failed: %s", target.Delta, err)
			}
		}

		if err := checkTargetConsistency(targetNetworkID, fullHeader, deltaHeader); err != nil {
			// the snapshots on the target do not seem to be consistent
			if !errors.Is(err, ErrDeltaSnapshotIncompatible) {
				s.LogInfof("snapshot consistency check failed (full: %s, delta: %s): %s", target.Full, target.Delta, err)

				continue
			}

			// the delta snapshot file does not fit the full snapshot file
			// we will not use the delta snapshot file
			deltaHeader = nil
			target.Delta = ""
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
func (s *Importer) DownloadSnapshotFiles(ctx context.Context, targetNetworkID uint64, fullPath string, deltaPath string, targets []*DownloadTarget) error {

	for _, target := range s.filterTargets(ctx, targetNetworkID, targets) {

		s.LogInfof("downloading full snapshot file from %s", target.Full)
		if err := s.downloadFile(ctx, fullPath, target.Full); err != nil {
			s.LogWarn(err)
			// as the full snapshot URL failed to download, we commence further with our targets
			continue
		}

		if len(target.Delta) > 0 {
			s.LogInfof("downloading delta snapshot file from %s", target.Delta)
			if err := s.downloadFile(ctx, deltaPath, target.Delta); err != nil {
				// it is valid that no delta snapshot file is available on the target.
				s.LogWarn(err)
			}
		}

		return nil
	}

	return ErrSnapshotDownloadNoValidSource
}

// downloads a snapshot header from the given url.
func (s *Importer) downloadHeader(ctx context.Context, url string, headerConsumer func(readCloser io.ReadCloser) error) error {
	ctxHeader, cancelHeader := context.WithTimeout(ctx, timeoutDownloadSnapshotHeader)
	defer cancelHeader()

	req, err := http.NewRequestWithContext(ctxHeader, http.MethodGet, url, nil)
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

	return headerConsumer(resp.Body)
}

// downloads a snapshot file from the given url to the specified path.
func (s *Importer) downloadFile(ctx context.Context, path string, url string) error {
	downloadCtx, downloadCtxCancel := context.WithTimeout(ctx, timeoutDownloadSnapshotFile)
	defer downloadCtxCancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, url, nil)
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
	counter := NewWriteCounter(ctx, uint64(resp.ContentLength))
	if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
		_ = out.Close()

		return fmt.Errorf("download failed: %w", err)
	}

	// the progress indicator uses the same line so print a new line once it's finished downloading
	fmt.Print("\n")

	if err = out.Close(); err != nil {
		return fmt.Errorf("unable to close downloaded snapshot file: %w", err)
	}

	if err = os.Rename(tempFileName, path); err != nil {
		return fmt.Errorf("unable to rename downloaded snapshot file: %w", err)
	}

	ok = true

	return nil
}
