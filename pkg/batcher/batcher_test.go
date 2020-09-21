package batcher_test

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/batcher"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/curl"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/stretchr/testify/assert"
)

const (
	testInputSize   = consts.TransactionTrinarySize
	testTimeout     = 100 * time.Millisecond
	testWorkerCount = 1
)

var nullTransactionTrits = make(trinary.Trits, testInputSize)

func TestClose(t *testing.T) {
	b := newTestBatcher()
	assert.NoError(t, b.Close())
}

func TestCloseTwice(t *testing.T) {
	b := newTestBatcher()
	assert.NoError(t, b.Close())
	assert.NoError(t, b.Close())
}

func TestClosed(t *testing.T) {
	b := newTestBatcher()
	assert.NoError(t, b.Close())

	// try multiple times to check for race conditions
	for i := 0; i < 10; i++ {
		_, err := b.Hash(nullTransactionTrits)
		assert.Equal(t, batcher.ErrClosed, err)
	}
}

func TestCancelJob(t *testing.T) {
	b := newTestBatcher()

	resultChan := b.SubmitHash(nullTransactionTrits)
	assert.NoError(t, b.Close())
	r := <-resultChan
	assert.Equal(t, batcher.ErrCanceled, r.Err)
}

func TestCancelBatch(t *testing.T) {
	b := newTestBatcher()

	hashCount := b.WorkerCount()*b.BatchSize() + b.BatchSize()
	results := make([]<-chan batcher.CurlResult, hashCount)
	for i := range results {
		results[i] = b.SubmitHash(nullTransactionTrits)
	}
	assert.NoError(t, b.Close())

	// each worker should have processed its batch
	for i := 0; i < b.WorkerCount()*b.BatchSize(); i++ {
		assert.NoError(t, (<-results[i]).Err)
	}
	// the final batch should be canceled
	for i := b.WorkerCount() * b.BatchSize(); i < hashCount; i++ {
		assert.Equal(t, batcher.ErrCanceled, (<-results[i]).Err)
	}
}

func TestInvalidInputLength(t *testing.T) {
	b := newTestBatcher()
	defer b.Close()

	result := <-b.SubmitHash(trinary.Trits{})
	assert.Error(t, result.Err)

	_, err := b.Hash(trinary.Trits{})
	assert.Error(t, err)
}

func TestSingleHash(t *testing.T) {
	b := newTestBatcher()
	defer b.Close()

	hash, err := b.Hash(nullTransactionTrits)
	assert.NoError(t, err)

	expHash, err := curl.HashTrits(nullTransactionTrits, curl.CurlP81)
	assert.NoError(t, err)
	assert.Equal(t, expHash, hash)
}

func TestHash(t *testing.T) {
	b := newTestBatcher()
	defer b.Close()

	// create different inputs for each hash call
	trits := testTrits(8, testInputSize)

	var wg sync.WaitGroup
	wg.Add(len(trits))

	for i := range trits {
		tmp := trits[i]
		go func() {
			defer wg.Done()
			h, err := b.Hash(tmp)
			assert.NoError(t, err)

			expHash, err := curl.HashTrits(tmp, curl.CurlP81)
			assert.NoError(t, err)
			assert.Equal(t, expHash, h)
		}()
	}
	wg.Wait()
}

func testTrits(size int, tritsCount int) []trinary.Trits {
	s := consts.TryteAlphabet
	trytesCount := tritsCount / consts.TritsPerTryte

	src := make([]trinary.Trits, size)
	for i := range src {
		trytes := strings.Repeat(s, trytesCount/len(s)+1)[:trytesCount-2] + trinary.IntToTrytes(int64(i), 2)
		src[i] = trinary.MustTrytesToTrits(trytes)
	}
	return src
}

func newTestBatcher() *batcher.Curl {
	return batcher.NewCurlP81(testInputSize, testTimeout, testWorkerCount)
}

func BenchmarkHash(b *testing.B) {
	batchHasher := batcher.NewCurlP81(testInputSize, time.Millisecond, runtime.GOMAXPROCS(0))
	defer batchHasher.Close()

	var wg sync.WaitGroup
	wg.Add(b.N)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		go func() {
			defer wg.Done()
			_, _ = batchHasher.Hash(nullTransactionTrits)
		}()
	}
	wg.Wait()
}
