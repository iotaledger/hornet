package metrics

// NodeInfoExtended represents extended information about the node.
type NodeInfoExtended struct {
	Version       string `json:"version"`
	LatestVersion string `json:"latestVersion"`
	Uptime        int64  `json:"uptime"`
	NodeID        string `json:"nodeId"`
	NodeAlias     string `json:"nodeAlias"`
	MemoryUsage   int64  `json:"memUsage"`
}

// DatabaseSizesMetric represents database size metrics.
type DatabaseSizesMetric struct {
	Tangle int64 `json:"tangle"`
	UTXO   int64 `json:"utxo"`
	Total  int64 `json:"total"`
	Time   int64 `json:"ts"`
}
