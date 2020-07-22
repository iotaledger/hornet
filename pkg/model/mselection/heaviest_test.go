package mselection

/*

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
)

const (
	numTestTxs      = 32 * 100
	numBenchmarkTxs = 5000
)

func init() {
	rand.Seed(0)
}

func TestHeaviestSelector_SelectTipsChain(t *testing.T) {
	hps := New()
	// create a chain
	var lastHash = hornet.NullHashBytes
	for i := 1; i <= numTestTxs; i++ {
		bndl := newTestBundle(i, lastHash, lastHash)
		hps.OnNewSolidBundle(bndl)
		lastHash = bndl.GetTailHash()
	}

	tip, err := hps.selectTip(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, lastHash, tip)
}

func TestHeaviestSelector_SelectTipsChains(t *testing.T) {
	hps := New()

	var lastHash = [2]hornet.Hash{}
	for i := 0; i < 2; i++ {
		lastHash[i] = hornet.NullHashBytes
		for j := 1; j <= numTestTxs; j++ {
			bndl := newTestBundle(i*numTestTxs+j, lastHash[i], lastHash[i])
			hps.OnNewSolidBundle(bndl)
			lastHash[i] = bndl.GetTailHash()
		}
	}

	tip, err := hps.selectTip(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, lastHash, tip)
}

func TestHeaviestSelector_SelectTipsCancel(t *testing.T) {
	hps := New()
	// create a very large blow ball
	for i := 1; i <= 10000; i++ {
		bndl := newTestBundle(i, hornet.NullHashBytes, hornet.NullHashBytes)
		hps.OnNewSolidBundle(bndl)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err := hps.selectTip(ctx)
		assert.Truef(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()
}

func TestHeaviestSelector_Concurrent(t *testing.T) {
	hps := New()
	hashes := []hornet.Hash{hornet.NullHashBytes}
	for i := 0; i < 1000; i++ {
		bndl := newTestBundle(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidBundle(bndl)
		hashes = append(hashes, bndl.GetTailHash())
	}

	var wg sync.WaitGroup
	selector := func() {
		defer wg.Done()
		_, err := hps.selectTip(context.Background())
		assert.NoError(t, err)
	}

	wg.Add(2)
	go selector()
	go selector()

	for i := 1000; i < 2000; i++ {
		bndl := newTestBundle(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidBundle(bndl)
		hashes = append(hashes, bndl.GetTailHash())
	}
	wg.Wait()
}

func BenchmarkHeaviestSelector_OnNewSolidTransaction(b *testing.B) {
	hps := New()
	hashes := []hornet.Hash{hornet.NullHashBytes}
	data := make([]*tangle.Bundle, numBenchmarkTxs)
	for i := 0; i < numBenchmarkTxs; i++ {
		data[i] = newTestBundle(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hashes = append(hashes, data[i].GetTailHash())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hps.OnNewSolidTransaction(data[i%numBenchmarkTxs])
		if i%numBenchmarkTxs == numBenchmarkTxs-1 {
			hps.SetRoot(hornet.NullHashBytes)
		}
	}
}

func BenchmarkHeaviestSelector_SelectTips(b *testing.B) {
	hps := New()
	hashes := []hornet.Hash{hornet.NullHashBytes}
	for i := 0; i < numBenchmarkTxs; i++ {
		bndl := newTestBundle(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidBundle(bndl)
		hashes = append(hashes, bndl.GetTailHash())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = hps.selectTip(context.Background())
	}
}

func newTestBundle(idx int, trunk, branch hornet.Hash) *tangle.Bundle {
	bndl := tangle.Bundle{

	}
	tx := &transaction.Transaction{
		Hash:              trinary.IntToTrytes(int64(idx), consts.HashTrytesSize),
		Value:             0,
		Timestamp:         uint64(idx),
		TrunkTransaction:  trunk.Trytes(),
		BranchTransaction: branch.Trytes(),
	}
	return hornet.NewTransactionFromTx(tx, nil)
}
*/
