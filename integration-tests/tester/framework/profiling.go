package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/go-echarts/go-echarts/charts"
	"github.com/gorilla/websocket"

	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/iotaledger/hive.go/websockethub"
)

const (
	cpuProfilePrefix   = "cpu_profile"
	heapProfilePrefix  = "heap_profile"
	metricsChartPrefix = "metrics_charts"

	timeoutProfilingQuery = 1 * time.Minute

	byteMBDivider = 1000000
)

// Profiler profiles a node for metrics.
type Profiler struct {
	pprofURI     string
	websocketURI string
	// the name of the target of this profiler. used to determine the profile file names.
	targetName string
	http.Client
}

// registerWSTopics registers the given topics on the publisher.
func registerWSTopics(wsConn *websocket.Conn, topics ...byte) error {
	for _, b := range topics {
		topicRegCmd := []byte{dashboard.WebsocketCmdRegister, b}
		if err := wsConn.WriteMessage(websockethub.BinaryMessage, topicRegCmd); err != nil {
			return err
		}
	}
	return nil
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

// GraphMetrics graphs metrics about MPS, memory consumption, confirmation rate of the node and saves it into the log dir.
func (n *Profiler) GraphMetrics(dur time.Duration) error {
	var err error

	// MPS
	var mpsChartXAxis []string
	var newMPS, incomingMPS, outgoingMPS []int32
	mpsChart := charts.NewLine()
	mpsChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Messages Per Second"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)

	// conf. and issuance rate
	referencedRateChart := charts.NewLine()
	referencedRateChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Referenced Rate"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	issuanceRateChart := charts.NewBar()
	issuanceRateChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Milestone Issuance Delta"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	var confIssXAxis []string
	var referencedRate []float64
	var issuanceInterval []float64

	// memory
	memChart := charts.NewLine()
	memObjsChart := charts.NewLine()
	memChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Memory"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	memObjsChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Heap Objects"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	var memChartXAxis []string
	var memSys, memHeapSys, memHeapInUse, memHeapIdle, memHeapReleased, memHeapObjects, memMSpanInUse, memMCacheInUse, memStackSys []uint64

	// db size
	dbSizeChart := charts.NewLine()
	dbSizeChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Database Size"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	var dbSizeXAxis []string
	var dbSizeTotal []int64

	// tip selection
	tipSelChart := charts.NewLine()
	tipSelChart.SetGlobalOptions(
		charts.TitleOpts{Title: "Tip-Selection"},
		charts.DataZoomOpts{XAxisIndex: []int{0}, Start: 0, End: 100},
	)
	var tipSelXAxis []string
	var tipSelDur []int64

	conn, _, err := websocket.DefaultDialer.Dial(n.websocketURI, nil)
	if err != nil {
		return err
	}

	if err := registerWSTopics(conn, dashboard.MsgTypeMPSMetric, dashboard.MsgTypeTipSelMetric, dashboard.MsgTypeConfirmedMsMetrics,
		dashboard.MsgTypeDatabaseSizeMetric, dashboard.MsgTypeNodeStatus); err != nil {
		return err
	}

	s := time.Now()
	end := s.Add(dur)
	for time.Now().Before(end) {
		if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return err
		}
		_, msgRaw, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		m := &dashboard.Msg{}
		if err := json.Unmarshal(msgRaw, m); err != nil {
			return err
		}
		if m.Data == nil {
			continue
		}
		switch m.Type {

		case dashboard.MsgTypeMPSMetric:
			mpsMetric := &tangle.MPSMetrics{}
			if err := json.Unmarshal(msgRaw, &dashboard.Msg{Data: mpsMetric}); err != nil {
				return err
			}
			mpsChartXAxis = append(mpsChartXAxis, fmt.Sprintf("%s sec", strconv.Itoa(int(time.Since(s).Seconds()))))
			incomingMPS = append(incomingMPS, int32(mpsMetric.Incoming))
			outgoingMPS = append(outgoingMPS, -int32(mpsMetric.Outgoing))
			newMPS = append(newMPS, int32(mpsMetric.New))

		case dashboard.MsgTypeTipSelMetric:
			tipSelMetric := &tipselect.TipSelStats{}
			if err := json.Unmarshal(msgRaw, &dashboard.Msg{Data: tipSelMetric}); err != nil {
				return err
			}

			tipSelXAxis = append(mpsChartXAxis, fmt.Sprintf("%s sec", strconv.Itoa(int(time.Since(s).Seconds()))))
			tipSelDur = append(tipSelDur, int64(tipSelMetric.Duration)/int64(time.Millisecond))

		case dashboard.MsgTypeConfirmedMsMetrics:
			confMetrics := []*tangle.ConfirmedMilestoneMetric{}
			if err := json.Unmarshal(msgRaw, &dashboard.Msg{Data: &confMetrics}); err != nil {
				return err
			}
			if len(confMetrics) == 0 {
				continue
			}
			confMetric := confMetrics[len(confMetrics)-1]
			confIssXAxis = append(confIssXAxis, fmt.Sprintf("Ms %s", strconv.Itoa(int(confMetric.MilestoneIndex))))
			referencedRate = append(referencedRate, confMetric.ReferencedRate)
			issuanceInterval = append(issuanceInterval, confMetric.TimeSinceLastMilestone)

		case dashboard.MsgTypeDatabaseSizeMetric:
			dbSizeMetrics := []*dashboard.DBSizeMetric{}
			if err := json.Unmarshal(msgRaw, &dashboard.Msg{Data: &dbSizeMetrics}); err != nil {
				return err
			}
			if len(dbSizeMetrics) == 0 {
				continue
			}
			dbSizeMetric := dbSizeMetrics[len(dbSizeMetrics)-1]
			dbSizeXAxis = append(mpsChartXAxis, fmt.Sprintf("%s sec", strconv.Itoa(int(time.Since(s).Seconds()))))
			dbSizeTotal = append(dbSizeTotal, dbSizeMetric.Total/byteMBDivider)

		case dashboard.MsgTypeNodeStatus:
			nodeStatus := &dashboard.NodeStatus{
				Mem: &dashboard.MemMetrics{},
			}
			if err := json.Unmarshal(msgRaw, &dashboard.Msg{Data: nodeStatus}); err != nil {
				return err
			}

			memMetrics := nodeStatus.Mem
			memChartXAxis = append(memChartXAxis, fmt.Sprintf("%s sec", strconv.Itoa(int(time.Since(s).Seconds()))))
			memSys = append(memSys, memMetrics.Sys/byteMBDivider)
			memHeapSys = append(memHeapSys, memMetrics.HeapSys/byteMBDivider)
			memHeapInUse = append(memHeapInUse, memMetrics.HeapInuse/byteMBDivider)
			memHeapIdle = append(memHeapIdle, memMetrics.HeapIdle/byteMBDivider)
			memHeapReleased = append(memHeapReleased, memMetrics.HeapReleased/byteMBDivider)
			memHeapObjects = append(memHeapObjects, memMetrics.HeapObjects)
			memMSpanInUse = append(memMSpanInUse, memMetrics.MSpanInuse/byteMBDivider)
			memMCacheInUse = append(memMCacheInUse, memMetrics.MCacheInuse/byteMBDivider)
			memStackSys = append(memStackSys, memMetrics.StackSys/byteMBDivider)
		}
	}

	mpsChart.AddXAxis(mpsChartXAxis).
		AddYAxis("New", newMPS).
		AddYAxis("Incoming", incomingMPS).
		AddYAxis("Outgoing", outgoingMPS)

	referencedRateChart.AddXAxis(confIssXAxis).
		AddYAxis("Ref. Rate %", referencedRate)
	issuanceRateChart.AddXAxis(confIssXAxis).
		AddYAxis("Issuance Delta", issuanceInterval)

	dbSizeChart.AddXAxis(dbSizeXAxis).AddYAxis("Total", dbSizeTotal)

	memChart.AddXAxis(memChartXAxis).
		AddYAxis("Sys", memSys).
		AddYAxis("Heap Sys", memHeapSys).
		AddYAxis("Heap In Use", memHeapInUse).
		AddYAxis("Heap Idle", memHeapIdle).
		AddYAxis("Heap Released", memHeapReleased).
		AddYAxis("M Span In Use", memMSpanInUse).
		AddYAxis("M Cache In Use", memMCacheInUse).
		AddYAxis("Stack Sys", memStackSys)

	memObjsChart.AddXAxis(memChartXAxis).AddYAxis("Objects", memHeapObjects)

	tipSelChart.AddXAxis(tipSelXAxis).AddYAxis("Duration", tipSelDur)

	chartPage := charts.NewPage()
	chartPage.PageTitle = n.targetName
	chartPage.Add(mpsChart, memChart, dbSizeChart, memObjsChart, tipSelChart, referencedRateChart, issuanceRateChart)

	var buf bytes.Buffer
	if err := chartPage.Render(&buf); err != nil {
		return fmt.Errorf("unable to render metrics charts: %w", err)
	}

	return writeFileInLogDir(fmt.Sprintf("%s_%s_%d.html", n.targetName, metricsChartPrefix, time.Now().Unix()), ioutil.NopCloser(&buf))
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

	profileBytes, err := ioutil.ReadAll(resp.Body)
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
