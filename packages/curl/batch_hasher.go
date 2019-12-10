package curl

import (
	"runtime"
	"strconv"

	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/batchworkerpool"
	"github.com/gohornet/hornet/packages/ternary"
)

const (
	BatchedHasherQueueSize = 500
)

var (
	BatchedHasherCount = runtime.NumCPU() * 2
)

type BatchHasher struct {
	hashLength int
	rounds     int
	workerPool *batchworkerpool.BatchWorkerPool
}

func NewBatchHasher(hashLength int, rounds int) (result *BatchHasher) {
	result = &BatchHasher{
		hashLength: hashLength,
		rounds:     rounds,
	}

	result.workerPool = batchworkerpool.New(result.processHashes, batchworkerpool.BatchSize(strconv.IntSize), batchworkerpool.WorkerCount(BatchedHasherCount), batchworkerpool.QueueSize(BatchedHasherQueueSize))
	result.workerPool.Start()

	return
}

func (this *BatchHasher) GetWorkerCount() int {
	return this.workerPool.GetWorkerCount()
}

func (this *BatchHasher) GetBatchSize() int {
	return this.workerPool.GetBatchSize()
}

func (this *BatchHasher) GetPendingQueueSize() int {
	return this.workerPool.GetPendingQueueSize()
}

func (this *BatchHasher) Hash(trits trinary.Trits) trinary.Trits {
	return (<-this.workerPool.Submit(trits)).(trinary.Trits)
}

func (this *BatchHasher) processHashes(tasks []batchworkerpool.Task) {
	if len(tasks) > 1 {
		// multiplex the requests
		multiplexer := ternary.NewBCTernaryMultiplexer()
		for _, hashRequest := range tasks {
			multiplexer.Add(hashRequest.Param(0).(trinary.Trits))
		}
		bcTrits, err := multiplexer.Extract()
		if err != nil {
			panic(err)
		}

		// calculate the hash
		bctCurl := NewBCTCurl(this.hashLength, this.rounds, strconv.IntSize)
		bctCurl.Reset()
		bctCurl.Absorb(bcTrits)

		// extract the results from the demultiplexer
		demux := ternary.NewBCTernaryDemultiplexer(bctCurl.Squeeze(243))
		for i, task := range tasks {
			task.Return(demux.Get(i))
		}
	} else {
		var resp = make(trinary.Trits, this.hashLength)

		trits := tasks[0].Param(0).(trinary.Trits)

		curl := NewCurl(this.hashLength, this.rounds)
		curl.Absorb(trits, 0, len(trits))
		curl.Squeeze(resp, 0, this.hashLength)

		tasks[0].Return(resp)
	}
}
