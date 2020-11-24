package toolset

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/pebble"

	"github.com/gohornet/hornet/pkg/database"
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
		println(fmt.Sprintf("	%s [COUNT] [SIZE]", ToolBenchmarkIO))
		println()
		println("	[COUNT] - objects count (optional)")
		println("	[SIZE]  - objects size  (optional)")
	}

	loopCnt := 500000
	size := 1000

	if len(args) > 2 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolBenchmarkIO)
	}

	if len(args) >= 1 {
		var err error
		loopCnt, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("can't parse COUNT: %v", err)
		}
	}

	if len(args) == 2 {
		var err error
		size, err = strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("can't parse SIZE: %v", err)
		}
	}

	tempDir, err := ioutil.TempDir("", "benchmarkIO")
	if err != nil {
		return fmt.Errorf("can't create temp dir: %v", err)
	}

	defer os.RemoveAll(tempDir)

	pebbleInstance := database.GetPebbleDB(tempDir, false)
	pebbleStore := pebble.New(pebbleInstance)
	batchWriter := kvstore.NewBatchedWriter(pebbleStore)
	writeDoneWaitGroup := &sync.WaitGroup{}
	writeDoneWaitGroup.Add(loopCnt)

	ts := time.Now()

	for i := 0; i < loopCnt; i++ {
		// one read operation and one write operation per cycle
		batchWriter.Enqueue(newBenchmarkObject(pebbleStore, writeDoneWaitGroup, randBytes(32), randBytes(size)))
	}

	writeDoneWaitGroup.Wait()

	if err := pebbleInstance.Flush(); err != nil {
		return fmt.Errorf("flush database failed: %v", err)
	}

	if err := pebbleInstance.Close(); err != nil {
		return fmt.Errorf("close database failed: %v", err)
	}

	te := time.Now()
	duration := te.Sub(ts)
	totalBytes := uint64(loopCnt * (32 + size))
	bytesPerSecond := uint64(float64(totalBytes) / duration.Seconds())

	fmt.Println(fmt.Sprintf("Average speed: %s/s (%dx 32+%d byte chunks, total %s, took %v)", humanize.Bytes(bytesPerSecond), loopCnt, size, humanize.Bytes(totalBytes), duration.Truncate(time.Millisecond)))

	return nil
}
