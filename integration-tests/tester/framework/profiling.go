package framework

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/go-echarts/go-echarts/charts"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gorilla/websocket"
)

const (
	cpuProfilePrefix  = "cpu_profile"
	heapProfilePrefix = "heap_profile"
	tpsChartPrefix    = "tps_chart"
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
func (n *Profiler) TakeCPUProfile(durSec int) error {
	profileBytes, err := n.query(fmt.Sprintf("/debug/pprof/profile?seconds=%d", durSec))
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

// GraphTPS graphs the TPS by consuming the TPS metric from the node's websocket.
func (n *Profiler) GraphTPS(dur time.Duration) error {
	line := charts.NewLine()

	conn, _, err := websocket.DefaultDialer.Dial(n.websocketURI, nil)
	if err != nil {
		return err
	}

	// read tps ws msgs
	var xAxis []string
	var newTPS, incomingTPS, outgoingTPS []int
	s := time.Now()
	end := s.Add(dur)
	for time.Now().Before(end) {
		m := &dashboard.Msg{}
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		if err := conn.ReadJSON(m); err != nil {
			return err
		}
		if m.Type != dashboard.MsgTypeTPSMetric {
			continue
		}
		tpsMetrics := m.Data.(map[string]interface{})
		xAxis = append(xAxis, fmt.Sprintf("%s sec", strconv.Itoa(int(time.Since(s).Seconds()))))
		incomingTPS = append(incomingTPS, int(tpsMetrics["incoming"].(float64)))
		outgoingTPS = append(outgoingTPS, int(tpsMetrics["outgoing"].(float64)))
		newTPS = append(newTPS, int(tpsMetrics["new"].(float64)))
	}
	line.AddXAxis(xAxis).
		AddYAxis("New", newTPS).
		AddYAxis("Incoming", incomingTPS).
		AddYAxis("Outgoing", outgoingTPS)

	var buf bytes.Buffer
	if err := line.Render(&buf); err != nil {
		return fmt.Errorf("unable to render TPS chart: %w", err)
	}

	return writeFileInLogDir(fmt.Sprintf("%s_%s_%d.html", n.targetName, tpsChartPrefix, time.Now().Unix()), ioutil.NopCloser(&buf))
}

// queries the given pprof URI and returns the profile data.
func (n *Profiler) query(path string) ([]byte, error) {
	target := fmt.Sprintf("%s%s", n.pprofURI, path)
	res, err := n.Get(target)
	if err != nil {
		return nil, fmt.Errorf("unable to take profile: %w", err)
	}
	defer res.Body.Close()
	profileBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read profile from response: %w", err)
	}
	return profileBytes, err
}

// writeProfile writes the given profile data to the given file in the log directory.
func (n *Profiler) writeProfile(fileName string, profileBytes []byte) error {
	profileReader := ioutil.NopCloser(bytes.NewReader(profileBytes))
	return writeFileInLogDir(fileName, profileReader)
}
