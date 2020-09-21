package curl

import (
	"runtime"
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
		// TODO: probably >1 worker is not helping; using exactly one would reduce memory and overhead
		hasher = batcher.NewCurlP81(inputSize, timeout, runtime.NumCPU())
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
