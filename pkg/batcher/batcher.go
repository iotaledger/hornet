package batcher

import (
	"errors"
	"sync"
	"time"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/curl"
	"github.com/iotaledger/iota.go/curl/bct"
	"github.com/iotaledger/iota.go/trinary"
)

var (
	// ErrClosed is returned when a job is submitted after Close was called.
	ErrClosed = errors.New("use of closed batcher")
	// ErrCanceled is returned when a submitted job could not be processed before Close was called.
	ErrCanceled = errors.New("processing canceled")
)

const (
	// batch size limit for which curl/bct should be used
	bctThreshold = 3
)

// Curl defines an instance of a batched Curl hasher.
type Curl struct {
	rounds      curl.CurlRounds
	inputSize   int
	timeout     time.Duration
	workerCount int

	jobs    chan job
	batches chan []job

	wg        sync.WaitGroup
	closeOnce sync.Once
	closing   chan struct{}
}

// CurlResult defines the result to a submitted hash computation.
type CurlResult struct {
	Hash trinary.Trits
	Err  error
}

type job struct {
	trits  trinary.Trits
	result chan CurlResult
}

// NewCurl initializes a new batched Curl instance.
func NewCurl(rounds curl.CurlRounds, inputSize int, timeout time.Duration, workerCount int) *Curl {
	c := &Curl{
		rounds:      rounds,
		inputSize:   inputSize,
		timeout:     timeout,
		workerCount: workerCount,
		jobs:        make(chan job),
		batches:     make(chan []job),
		closing:     make(chan struct{}),
	}
	c.start()
	return c
}

// NewCurlP81 returns a new batched Curl-P-81.
func NewCurlP81(inputSize int, timeout time.Duration, workerCount int) *Curl {
	return NewCurl(curl.CurlP81, inputSize, timeout, workerCount)
}

// WorkerCount returns the number of parallel workers computing the hashes.
func (c *Curl) WorkerCount() int {
	return c.workerCount
}

// BatchSize returns the maximum batch size.
func (c *Curl) BatchSize() int {
	return bct.MaxBatchSize
}

// Close stops the batched Curl hasher.
// It blocks until all remaining jobs have been processed or canceled. Successive calls of Close are ignored.
func (c *Curl) Close() error {
	c.closeOnce.Do(func() {
		close(c.closing)
		c.wg.Wait()
	})
	return nil
}

// SubmitHash submits the Curl hash computation of trits to the batcher.
// It does not wait for the computation to finish but returns a channel for the result.
func (c *Curl) SubmitHash(trits trinary.Trits) <-chan CurlResult {
	result := make(chan CurlResult, 1)
	if len(trits) != c.inputSize {
		result <- CurlResult{nil, consts.ErrInvalidTritsLength}
		return result
	}

	select {
	case c.jobs <- job{trits, result}:
		return result
	case <-c.closing:
		result <- CurlResult{nil, ErrClosed}
		return result
	}
}

// Hash returns the Curl hash of trits.
// It blocks until the hash has been computed or an error occurred.
func (c *Curl) Hash(trits trinary.Trits) (trinary.Trits, error) {
	result := <-c.SubmitHash(trits)
	return result.Hash, result.Err
}

func (c *Curl) start() {
	// start the main loop
	c.wg.Add(1)
	go c.loop()

	// start workers
	for i := 0; i < c.workerCount; i++ {
		c.wg.Add(1)
		go c.worker()
	}
}

func (c *Curl) loop() {
	defer c.wg.Done()

	for {
		select {
		// on job received, start a new batch
		case j := <-c.jobs:
			c.startBatch(j)

		// on close, terminate
		case <-c.closing:
			// close the batches channel as we are done sending to it
			close(c.batches)
			return
		}
	}
}

// startBatch batches several jobs within the timeout together and starts the processing
func (c *Curl) startBatch(j job) {
	batch := append(make([]job, 0, c.BatchSize()), j)

	timeout := time.NewTimer(c.timeout)
	defer timeout.Stop()

Loop:
	for i := 1; i < cap(batch); i++ {
		select {
		// on timeout, start processing the current batch
		case <-timeout.C:
			break Loop

		// on job received, add it to the batch
		case c := <-c.jobs:
			batch = append(batch, c)

		// on close, cancel all the jobs in the batch
		case <-c.closing:
			c.cancel(batch)
			return
		}
	}

	// try submitting the batch to a worker
	select {
	case c.batches <- batch:
	case <-c.closing:
		c.cancel(batch)
	}
}

func (c *Curl) cancel(batch []job) {
	for i := range batch {
		batch[i].result <- CurlResult{nil, ErrCanceled}
	}
}

func (c *Curl) worker() {
	defer c.wg.Done()

	for batch := range c.batches {
		c.process(batch)
	}
}

func (c *Curl) process(batch []job) {
	// if below threshold, compute the hashes in serial
	if len(batch) < bctThreshold {
		for i := range batch {
			hash, err := curl.HashTrits(batch[i].trits, c.rounds)
			batch[i].result <- CurlResult{hash, err}
		}
		return
	}

	batchBuf := make([]trinary.Trits, len(batch))
	for i := range batch {
		batchBuf[i] = batch[i].trits
	}

	err := c.hashBatchedTrits(batchBuf)
	for i := range batch {
		batch[i].result <- CurlResult{batchBuf[i], err}
	}
}

func (c *Curl) hashBatchedTrits(batchBuf []trinary.Trits) error {
	h := bct.NewCurl(c.rounds)
	if err := h.Absorb(batchBuf, c.inputSize); err != nil {
		return err
	}
	if err := h.Squeeze(batchBuf, consts.HashTrinarySize); err != nil {
		return err
	}
	return nil
}
