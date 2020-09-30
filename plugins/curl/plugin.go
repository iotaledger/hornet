package curl

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/pkg/batcher"
	"github.com/gohornet/hornet/pkg/shutdown"
)

const (
	inputSize = consts.TransactionTrinarySize
	timeout   = 50 * time.Millisecond
)

var (
	PLUGIN     = node.NewPlugin("Curl", node.Enabled, configure, run)
	log        *logger.Logger
	hasher     *batcher.Curl
	hasherOnce sync.Once
)

// Hasher returns the batched Curl singleton.
func Hasher() *batcher.Curl {
	hasherOnce.Do(func() {
		// create a new batched Curl instance to compute transaction hashes
		// on average amd64 hardware, even a single worker can hash about 100Mb/s; this is sufficient for all scenarios
		// TODO: verify performance on arm (especially 32bit) that >1 worker is indeed not needed and beneficial
		hasher = batcher.NewCurlP81(inputSize, timeout, 1)
	})
	return hasher
}

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	// assure that the hasher is initialized
	Hasher()
}

func run(_ *node.Plugin) {
	// close the hasher on shutdown
	daemon.BackgroundWorker("Curl batched hashing", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Curl batched hashing ... done")
		<-shutdownSignal
		log.Info("Stopping Curl batched hashing ...")
		if err := Hasher().Close(); err != nil {
			log.Errorf("Stopping Curl batched hashing: %s", err)
		} else {
			log.Info("Stopping Curl batched hashing ... done")
		}
	}, shutdown.PriorityCurlHasher)
}
