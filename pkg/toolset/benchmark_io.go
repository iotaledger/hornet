package toolset

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hornet/v2/pkg/tpkg"
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
		panic(fmt.Errorf("write operation failed: %w", err))
	}
}

func (bo *benchmarkObject) BatchWriteDone() {
	// do a read operation after the batchwrite is done,
	// so the write and read operations are equally distributed over the whole benchmark run.
	if _, err := bo.store.Has(tpkg.RandBytes(32)); err != nil {
		panic(fmt.Errorf("read operation failed: %w", err))
	}

	bo.writeDoneWaitGroup.Done()
}

func (bo *benchmarkObject) BatchWriteScheduled() bool {
	return false
}

func (bo *benchmarkObject) ResetBatchWriteScheduled() {
	// do nothing
}
