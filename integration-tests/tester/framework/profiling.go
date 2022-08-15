package framework

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	cpuProfilePrefix  = "cpu_profile"
	heapProfilePrefix = "heap_profile"

	timeoutProfilingQuery = 1 * time.Minute
)

// Profiler profiles a node for metrics.
type Profiler struct {
	pprofURI     string
	websocketURI string
	// the name of the target of this profiler. used to determine the profile file names.
	targetName string
	http.Client
}

// TakeCPUProfile takes a CPU profile for the given duration and then saves it to the log directory.
func (n *Profiler) TakeCPUProfile(dur time.Duration) error {
	profileBytes, err := n.query(fmt.Sprintf("/debug/pprof/profile?seconds=%d", dur.Truncate(time.Second)))
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("%s_%s_%d.profile", n.targetName, cpuProfilePrefix, time.Now().Unix())

	return n.writeProfile(fileName, profileBytes)
}

// TakeHeapSnapshot takes a snapshot of the heap memory and then saves it to the log directory.
func (n *Profiler) TakeHeapSnapshot() error {
	profileBytes, err := n.query("/debug/pprof/heap")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("%s_%s_%d.profile", n.targetName, heapProfilePrefix, time.Now().Unix())

	return n.writeProfile(fileName, profileBytes)
}

// queries the given pprof URI and returns the profile data.
func (n *Profiler) query(path string) ([]byte, error) {
	target := fmt.Sprintf("%s%s", n.pprofURI, path)

	ctx, cancel := context.WithTimeout(context.Background(), timeoutProfilingQuery)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	resp, err := n.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to take profile: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	profileBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read profile from response: %w", err)
	}

	return profileBytes, err
}

// writeProfile writes the given profile data to the given file in the log directory.
func (n *Profiler) writeProfile(fileName string, profileBytes []byte) error {
	profileReader := io.NopCloser(bytes.NewReader(profileBytes))

	return writeFileInLogDir(fileName, profileReader)
}
