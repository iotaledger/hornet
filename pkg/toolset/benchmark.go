package toolset

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	// printStatusInterval is the interval for printing status messages
	printStatusInterval = 2 * time.Second
)

func benchmarkIO(_ *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [COUNT] [SIZE] [DB_ENGINE]", ToolBenchmarkIO))
		println()
		println("	[COUNT] 	- objects count (optional)")
		println("	[SIZE]  	- objects size  (optional)")
		println("	[DB_ENGINE] - database engine (optional, values: pebble, rocksdb)")
		println()
		println(fmt.Sprintf("example: %s %d %d %s", ToolBenchmarkIO, 500000, 1000, "rocksdb"))
	}

	objectCnt := 500000
	size := 1000
	dbEngine := database.EngineRocksDB

	if len(args) > 3 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolBenchmarkIO)
	}

	if len(args) >= 1 {
		var err error
		objectCnt, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("can't parse COUNT: %w", err)
		}
	}

	if len(args) >= 2 {
		var err error
		size, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("can't parse SIZE: %w", err)
		}
	}

	if len(args) == 3 {
		dbEngine = strings.ToLower(args[2])
	}

	engine, err := database.DatabaseEngine(dbEngine)
	if err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir("", "benchmarkIO")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(tempDir) }()

	store, err := database.StoreWithDefaultSettings(tempDir, true, engine)
	if err != nil {
		return fmt.Errorf("database initialization failed: %w", err)
	}

	batchWriter := kvstore.NewBatchedWriter(store)
	writeDoneWaitGroup := &sync.WaitGroup{}
	writeDoneWaitGroup.Add(objectCnt)

	ts := time.Now()

	lastStatusTime := time.Now()
	for i := 0; i < objectCnt; i++ {
		// one read operation and one write operation per cycle
		batchWriter.Enqueue(newBenchmarkObject(store, writeDoneWaitGroup, randBytes(32), randBytes(size)))

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			duration := time.Since(ts)
			bytes := uint64(i * (32 + size))
			totalBytes := uint64(objectCnt * (32 + size))
			bytesPerSecond := uint64(float64(bytes) / duration.Seconds())
			objectsPerSecond := uint64(float64(i) / duration.Seconds())
			percentage, remaining := utils.EstimateRemainingTime(ts, int64(i), int64(objectCnt))
			fmt.Printf("Average IO speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, %0.2f%%. %v left...)\n", humanize.Bytes(bytesPerSecond), i, size, dbEngine, humanize.Bytes(bytes), humanize.Bytes(totalBytes), objectsPerSecond, percentage, remaining.Truncate(time.Second))
		}
	}

	writeDoneWaitGroup.Wait()

	if err := store.Flush(); err != nil {
		return fmt.Errorf("flush database failed: %w", err)
	}

	if err := store.Close(); err != nil {
		return fmt.Errorf("close database failed: %w", err)
	}

	te := time.Now()
	duration := te.Sub(ts)
	totalBytes := uint64(objectCnt * (32 + size))
	bytesPerSecond := uint64(float64(totalBytes) / duration.Seconds())
	objectsPerSecond := uint64(float64(objectCnt) / duration.Seconds())

	fmt.Printf("Average IO speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, took %v)\n", humanize.Bytes(bytesPerSecond), objectCnt, size, dbEngine, humanize.Bytes(totalBytes), humanize.Bytes(totalBytes), objectsPerSecond, duration.Truncate(time.Millisecond))

	return nil
}

func benchmarkCPU(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [THREADS]", ToolBenchmarkCPU))
		println()
		println("	[THREADS]  	- thread count (optional)")
		println()
		println(fmt.Sprintf("example: %s %d", ToolBenchmarkCPU, 2))
	}

	threads := runtime.NumCPU()
	duration := 1 * time.Minute

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolBenchmarkCPU)
	}

	if len(args) == 1 {
		var err error
		threads, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("can't parse THREADS: %w", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	ts := time.Now()

	// doBenchmarkCPU mines with CURL until the context has been canceled.
	// it returns the number of calculated hashes.
	doBenchmarkCPU := func(ctx context.Context, numWorkers int) uint64 {
		var (
			done    uint32
			counter uint64
			wg      sync.WaitGroup
			closing = make(chan struct{})
		)

		// random digest
		powDigest := randBytes(32)

		// stop when the context has been canceled
		go func() {
			select {
			case <-ctx.Done():
				atomic.StoreUint32(&done, 1)
			case <-closing:
				return
			}
		}()

		go func() {
			for atomic.LoadUint32(&done) == 0 {
				time.Sleep(printStatusInterval)

				elapsed := time.Since(ts)
				percentage, remaining := utils.EstimateRemainingTime(ts, int64(elapsed.Milliseconds()), int64(duration.Milliseconds()))
				megahashesPerSecond := float64(counter) / (elapsed.Seconds() * 1000000)
				fmt.Printf("Average CPU speed: %0.2fMH/s (%d thread(s), %0.2f%%. %v left...)\n", megahashesPerSecond, threads, percentage, remaining.Truncate(time.Second))
			}
		}()

		workerWidth := math.MaxUint64 / uint64(numWorkers)
		for i := 0; i < numWorkers; i++ {
			startNonce := uint64(i) * workerWidth
			wg.Add(1)
			go func() {
				defer wg.Done()

				if err := cpuBenchmarkWorker(powDigest, startNonce, &done, &counter); err != nil {
					return
				}
				atomic.StoreUint32(&done, 1)
			}()
		}
		wg.Wait()
		close(closing)

		return counter
	}

	hashes := doBenchmarkCPU(ctx, threads)
	megahashesPerSecond := float64(hashes) / (duration.Seconds() * 1000000)
	fmt.Printf("Average CPU speed: %0.2fMH/s (%d thread(s), took %v)\n", megahashesPerSecond, threads, duration.Truncate(time.Millisecond))

	return nil
}
