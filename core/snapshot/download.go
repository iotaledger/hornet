package snapshot

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/iotaledger/hive.go/daemon"
)

// WriteCounter counts the number of bytes written to it. It implements to the io.Writer interface
// and we can pass this into io.TeeReader() which will report progress on each write cycle.
type WriteCounter struct {
	Expected         uint64
	Total            uint64
	Last             uint64
	LastProgressTime time.Time
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)

	if daemon.IsStopped() {
		return n, ErrSnapshotDownloadWasAborted
	}

	wc.PrintProgress()
	return n, nil
}

func (wc *WriteCounter) PrintProgress() {
	if time.Since(wc.LastProgressTime) < 1*time.Second {
		return
	}

	bytesPerSecond := uint64(float64(wc.Total-wc.Last) / time.Since(wc.LastProgressTime).Seconds())
	wc.LastProgressTime = time.Now()
	wc.Last = wc.Total

	// Clear the line by using a character return to go back to the start and remove
	// the remaining characters by filling it with spaces
	fmt.Printf("\r%s", strings.Repeat(" ", 60))

	// Return again and print current status of download
	// We use the humanize package to print the bytes in a meaningful way (e.g. 10 MB)
	fmt.Printf("\rDownloading... %s/%s (%s/s)", humanize.Bytes(wc.Total), humanize.Bytes(wc.Expected), humanize.Bytes(bytesPerSecond))
}

func downloadSnapshotFile(filepath string, urls []string) error {

	// Try to download a snapshot from one of the provided sources, break if download was successful
	downloadOK := false
	for _, url := range urls {
		log.Infof("Downloading snapshot from %s", url)

		// Create the file, but give it a tmp file extension, this means we won't overwrite a
		// file until it's downloaded, but we'll remove the tmp extension once downloaded.
		out, err := os.Create(filepath + ".tmp")
		if err != nil {
			return err
		}

		// Get the data
		resp, err := http.Get(url)
		if err != nil {
			log.Warnf("Downloading snapshot from %s failed with %v", url, err)
			out.Close()
			continue
		}
		if resp.StatusCode != http.StatusOK {
			log.Warnf("Downloading snapshot from %s failed. Server returned %d", url, resp.StatusCode)
			out.Close()
			continue
		}

		defer resp.Body.Close()

		// Create our progress reporter and pass it to be used alongside our writer
		counter := &WriteCounter{
			Expected: uint64(resp.ContentLength),
		}
		if _, err = io.Copy(out, io.TeeReader(resp.Body, counter)); err != nil {
			log.Warnf("Downloading snapshot from %s failed with %v", url, err)
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
		return fmt.Errorf(ErrSnapshotDownloadNoValidSource.Error())
	}

	if err := os.Rename(filepath+".tmp", filepath); err != nil {
		return err
	}
	return nil
}
