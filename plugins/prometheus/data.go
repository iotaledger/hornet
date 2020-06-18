package prometheus

import (
	"os"
	"path/filepath"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	dataSizes *prometheus.GaugeVec
)

func init() {
	dataSizes = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iota_data_sizes_bytes",
			Help: "Data sizes in bytes.",
		},
		[]string{"name"},
	)

	registry.MustRegister(dataSizes)

	addCollect(collectData)
}

func collectData() {
	dataSizes.Reset()
	dbSize, err := directorySize(config.NodeConfig.GetString(config.CfgDatabasePath))
	if err == nil {
		dataSizes.WithLabelValues("database").Set(float64(dbSize))
	}
}

func directorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
