package toolset

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/bolt"
	"github.com/iotaledger/hive.go/kvstore/pebble"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/utils"
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
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func benchmarkIO(args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [COUNT] [SIZE] [DATABASE]", ToolBenchmarkIO))
		println()
		println("	[COUNT] 	- objects count (optional)")
		println("	[SIZE]  	- objects size  (optional)")
		println("	[DATABASE]  - database implementation (optional, values: bolt, pebble)")
	}

	objectCnt := 500000
	size := 1000
	dbImplementation := "pebble"

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
		dbImplementation = strings.ToLower(args[2])
	}

	tempDir, err := ioutil.TempDir("", "benchmarkIO")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %v", err)
	}

	defer os.RemoveAll(tempDir)

	var store kvstore.KVStore

	switch dbImplementation {
	//case "badger":
	//	store = badger.New(database.NewBadgerDB(tempDir))
	case "pebble":
		store = pebble.New(database.NewPebbleDB(tempDir, false))
	case "bolt":
		store = bolt.New(database.NewBoltDB(tempDir, "bolt.db"))
	default:
		return fmt.Errorf("unknown database implementation: %s", dbImplementation)
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
			fmt.Println(fmt.Sprintf("Average speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, %0.2f%%. %v left...)", humanize.Bytes(bytesPerSecond), i, size, dbImplementation, humanize.Bytes(bytes), humanize.Bytes(totalBytes), objectsPerSecond, percentage, remaining.Truncate(time.Second)))
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

	fmt.Println(fmt.Sprintf("Average speed: %s/s (%dx 32+%d byte chunks with %s database, total %s/%s, %d objects/s, took %v)", humanize.Bytes(bytesPerSecond), objectCnt, size, dbImplementation, humanize.Bytes(totalBytes), humanize.Bytes(totalBytes), objectsPerSecond, duration.Truncate(time.Millisecond)))

	return nil
}
