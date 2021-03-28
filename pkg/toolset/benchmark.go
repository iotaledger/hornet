package toolset

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/kvstore/pebble"
	"github.com/iotaledger/hive.go/kvstore/rocksdb"

	"github.com/iotaledger/iota.go/v2/pow"
)

const (
	// printStatusInterval is the interval for printing status messages
	printStatusInterval = 2 * time.Second
)

type benchmarkObject struct {
	store              kvstore.KVStore
	writeDoneWaitGroup *sync.WaitGroup
	key                []byte
	value              []byte
}

func newBenchmarkObject(store kvstore.KVStore, writeDoneWaitGroup *sync.WaitGroup, key []byte, value []byte) *benchmarkObject {
	return &benchmarkObject{
		store:              store,
		writeDoneWaitGroup: writeDoneWaitGroup,
		key:                key,
		value:              value,
	}
}

func (bo *benchmarkObject) BatchWrite(batchedMuts kvstore.BatchedMutations) {
	if err := batchedMuts.Set(bo.key, bo.value); err != nil {
		panic(fmt.Errorf("write operation failed: %v", err))
	}
}

func (bo *benchmarkObject) BatchWriteDone() {
	// do a read operation after the batchwrite is done,
	// so the write and read operations are equally distributed over the whole benchmark run.
	if _, err := bo.store.Has(randBytes(32)); err != nil {
		panic(fmt.Errorf("read operation failed: %v", err))
	}

	bo.writeDoneWaitGroup.Done()
}

func (bo *benchmarkObject) BatchWriteScheduled() bool {
	return false
}

func (bo *benchmarkObject) ResetBatchWriteScheduled() {
	// do nothing
}

// returns length amount random bytes
func randBytes(length int) []byte {
	b := make([]byte, length)
	for i := 0; i < length; i++ {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

func benchmarkIO(args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [COUNT] [SIZE] [DB_ENGINE]", ToolBenchmarkIO))
		println()
		println("	[COUNT] 	- objects count (optional)")
		println("	[SIZE]  	- objects size  (optional)")
		println("	[DB_ENGINE] - database engine (optional, values: bolt, pebble, rocksdb)")
	}

	objectCnt := 500000
	size := 1000
	dbEngine := "pebble"

	if len(args) > 3 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolBenchmarkIO)
	}

	if len(args) >= 1 {
		var err error
		objectCnt, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("can't parse COUNT: %v", err)
		}
	}

	if len(args) >= 2 {
		var err error
		size, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("can't parse SIZE: %v", err)
		}
	}

	if len(args) == 3 {
		dbEngine = strings.ToLower(args[2])
	}

	tempDir, err := ioutil.TempDir("", "benchmarkIO")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %v", err)
	}

	defer os.RemoveAll(tempDir)

	var store kvstore.KVStore

	switch dbEngine {
	case "pebble":
		store = pebble.New(database.NewPebbleDB(tempDir, nil, true))
	case "bolt":
		store = bolt.New(database.NewBoltDB(tempDir, "bolt.db"))
	case "rocksdb":
		store = rocksdb.New(database.NewRocksDB(tempDir))
	default:
		return fmt.Errorf("unknown database engine: %s, supported engines: pebble/bolt/rocksdb", dbEngine)
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
			fmt.Println(fmt.Sprintf("Average speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, %0.2f%%. %v left...)", humanize.Bytes(bytesPerSecond), i, size, dbEngine, humanize.Bytes(bytes), humanize.Bytes(totalBytes), objectsPerSecond, percentage, remaining.Truncate(time.Second)))
		}
	}

	writeDoneWaitGroup.Wait()

	if err := store.Flush(); err != nil {
		return fmt.Errorf("flush database failed: %v", err)
	}

	if err := store.Close(); err != nil {
		return fmt.Errorf("close database failed: %v", err)
	}

	te := time.Now()
	duration := te.Sub(ts)
	totalBytes := uint64(objectCnt * (32 + size))
	bytesPerSecond := uint64(float64(totalBytes) / duration.Seconds())
	objectsPerSecond := uint64(float64(objectCnt) / duration.Seconds())

	fmt.Println(fmt.Sprintf("Average speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, took %v)", humanize.Bytes(bytesPerSecond), objectCnt, size, dbEngine, humanize.Bytes(totalBytes), humanize.Bytes(totalBytes), objectsPerSecond, duration.Truncate(time.Millisecond)))

	return nil
}

func benchmarkCPU(args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [COUNT]", ToolBenchmarkCPU))
		println()
		println("	[COUNT] 	- iteration count (optional)")
	}

	threads := runtime.NumCPU()
	count := 1000
	size := 347
	score := 500.0

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolBenchmarkCPU)
	}

	if len(args) == 1 {
		var err error
		count, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("can't parse COUNT: %v", err)
		}
	}

	ts := time.Now()

	lastStatusTime := time.Now()
	for i := 0; i < count; i++ {
		data := randBytes(size)

		pow.New(threads).Mine(context.Background(), data, score)

		if time.Since(lastStatusTime) >= printStatusInterval {
			lastStatusTime = time.Now()

			duration := time.Since(ts)
			powPerSecond := float64(i) / duration.Seconds()
			percentage, remaining := utils.EstimateRemainingTime(ts, int64(i), int64(count))
			fmt.Println(fmt.Sprintf("Average PoW/s: %0.2fPoW/s (%dx, %0.2f%%. %v left...)", powPerSecond, i, percentage, remaining.Truncate(time.Second)))
		}
	}

	te := time.Now()
	duration := te.Sub(ts)
	powPerSecond := float64(count) / duration.Seconds()

	fmt.Println(fmt.Sprintf("Average PoW/s: %0.2fPOW/s (%dx with a size of %s and a PoW score of %0.1f, took %v)", powPerSecond, count, humanize.Bytes(uint64(size)), score, duration.Truncate(time.Millisecond)))

	return nil
}
