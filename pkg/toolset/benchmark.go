package toolset

import (
	"context"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dustin/go-humanize"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	hivedb "github.com/iotaledger/hive.go/core/database"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/database"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
	"github.com/iotaledger/hornet/v2/pkg/utils"
)

func benchmarkIO(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	objectsCountFlag := fs.Int(FlagToolBenchmarkCount, 500000, "objects count")
	objectsSizeFlag := fs.Int(FlagToolBenchmarkSize, 1000, "objects size in bytes")
	databaseEngineFlag := fs.String(FlagToolDatabaseEngine, string(DefaultValueDatabaseEngine), "database engine (optional, values: pebble, rocksdb)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolBenchmarkIO)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %d --%s %d --%s %s",
			ToolBenchmarkIO,
			FlagToolBenchmarkCount,
			500000,
			FlagToolBenchmarkSize,
			1000,
			FlagToolDatabaseEngine,
			DefaultValueDatabaseEngine))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	objectCnt := *objectsCountFlag
	size := *objectsSizeFlag

	allowedEngines := database.AllowedEnginesStorage

	dbEngine, err := hivedb.EngineFromStringAllowed(*databaseEngineFlag, allowedEngines...)
	if err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "benchmarkIO")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(tempDir) }()

	store, err := database.StoreWithDefaultSettings(tempDir, true, dbEngine, allowedEngines...)
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
		batchWriter.Enqueue(newBenchmarkObject(store, writeDoneWaitGroup, tpkg.RandBytes(32), tpkg.RandBytes(size)))

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			duration := time.Since(ts)
			bytes := uint64(i * (32 + size))
			totalBytes := uint64(objectCnt * (32 + size))
			bytesPerSecond := uint64(float64(bytes) / duration.Seconds())
			objectsPerSecond := uint64(float64(i) / duration.Seconds())
			percentage, remaining := utils.EstimateRemainingTime(ts, int64(i), int64(objectCnt))
			fmt.Printf("Average IO speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, %0.2f%%. %v left ...)\n", humanize.Bytes(bytesPerSecond), i, size, dbEngine, humanize.Bytes(bytes), humanize.Bytes(totalBytes), objectsPerSecond, percentage, remaining.Truncate(time.Second))
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

func benchmarkCPU(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	cpuThreadsFlag := fs.Int(FlagToolBenchmarkThreads, runtime.NumCPU(), "thread count")
	durationFlag := fs.Duration(FlagToolBenchmarkDuration, 1*time.Minute, "duration")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolBenchmarkCPU)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %d --%s 1m0s",
			ToolBenchmarkCPU,
			FlagToolBenchmarkThreads,
			2,
			FlagToolBenchmarkDuration))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	threads := *cpuThreadsFlag
	duration := *durationFlag

	benchmarkCtx, benchmarkCtxCancel := context.WithTimeout(context.Background(), duration)
	defer benchmarkCtxCancel()

	ts := time.Now()

	// doBenchmarkCPU mines with CURL until the context has been canceled.
	// it returns the number of calculated hashes.
	doBenchmarkCPU := func(ctx context.Context, numWorkers int) uint64 {
		var (
			done                         uint32
			counter                      uint64
			wg                           sync.WaitGroup
			closingCtx, closingCtxCancel = context.WithCancel(context.Background())
		)

		// random digest
		powDigest := tpkg.RandBytes(32)

		// stop when the context has been canceled
		go func() {
			select {
			case <-ctx.Done():
				atomic.StoreUint32(&done, 1)
			case <-closingCtx.Done():
				return
			}
		}()

		go func() {
			for atomic.LoadUint32(&done) == 0 {
				time.Sleep(printStatusInterval)

				elapsed := time.Since(ts)
				percentage, remaining := utils.EstimateRemainingTime(ts, elapsed.Milliseconds(), duration.Milliseconds())
				megahashesPerSecond := float64(counter) / (elapsed.Seconds() * 1000000)
				fmt.Printf("Average CPU speed: %0.2fMH/s (%d thread(s), %0.2f%%. %v left ...)\n", megahashesPerSecond, threads, percentage, remaining.Truncate(time.Second))
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
		closingCtxCancel()

		return counter
	}

	hashes := doBenchmarkCPU(benchmarkCtx, threads)
	megahashesPerSecond := float64(hashes) / (duration.Seconds() * 1000000)
	fmt.Printf("Average CPU speed: %0.2fMH/s (%d thread(s), took %v)\n", megahashesPerSecond, threads, duration.Truncate(time.Millisecond))

	return nil
}
