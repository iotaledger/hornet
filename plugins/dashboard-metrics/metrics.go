package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/tangle"
)

var (
	nodeStartupTimestamp = time.Now()

	lastGossipMetrics = &tangle.BPSMetrics{
		Incoming: 0,
		New:      0,
		Outgoing: 0,
	}
	lastGossipMetricsLock = &sync.RWMutex{}
)

func nodeInfoExtended() *NodeInfoExtended {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := &NodeInfoExtended{
		Version:       deps.AppInfo.Version,
		LatestVersion: deps.AppInfo.LatestGitHubVersion,
		Uptime:        time.Since(nodeStartupTimestamp).Milliseconds(),
		NodeID:        deps.Host.ID().String(),
		NodeAlias:     deps.NodeAlias,
		MemoryUsage:   int64(m.HeapAlloc + m.StackSys + m.MSpanSys + m.MCacheSys + m.BuckHashSys + m.GCSys + m.OtherSys),
	}

	return status
}

func databaseSizesMetrics() (*DatabaseSizesMetric, error) {

	tangleDatabaseSize, err := deps.TangleDatabase.Size()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "calculating tangle database size failed, error: %s", err)
	}

	utxoDatabaseSize, err := deps.UTXODatabase.Size()
	if err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "calculating UTXO database size failed, error: %s", err)
	}

	return &DatabaseSizesMetric{
		Tangle: tangleDatabaseSize,
		UTXO:   utxoDatabaseSize,
		Total:  tangleDatabaseSize + utxoDatabaseSize,
		Time:   time.Now().Unix(),
	}, nil
}

func gossipMetrics() *tangle.BPSMetrics {
	lastGossipMetricsLock.RLock()
	defer lastGossipMetricsLock.RUnlock()

	return lastGossipMetrics
}
