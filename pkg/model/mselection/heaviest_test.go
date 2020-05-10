package mselection

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/gohornet/hornet/pkg/model/hornet"
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
	hps := HPS(hornet.NullHashBytes)
	// create a chain
	var lastHash = hornet.NullHashBytes
	for i := 1; i <= numTestTxs; i++ {
		tx := newTestTransaction(i, lastHash, lastHash)
		hps.OnNewSolidTransaction(tx)
		lastHash = tx.GetTxHash()
	}

	tips, err := hps.SelectTips(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, []hornet.Hash{lastHash, lastHash}, tips)
}

func TestHeaviestSelector_SelectTipsChains(t *testing.T) {
	hps := HPS(hornet.NullHashBytes)

	var lastHash = [2]hornet.Hash{}
	for i := 0; i < 2; i++ {
		lastHash[i] = hornet.NullHashBytes
		for j := 1; j <= numTestTxs; j++ {
			tx := newTestTransaction(i*numTestTxs+j, lastHash[i], lastHash[i])
			hps.OnNewSolidTransaction(tx)
			lastHash[i] = tx.GetTxHash()
		}
	}

	tips, err := hps.SelectTips(context.Background())
	assert.NoError(t, err)
	assert.ElementsMatch(t, lastHash, tips)
}

func TestHeaviestSelector_SelectTipsCancel(t *testing.T) {
	hps := HPS(hornet.NullHashBytes)
	// create a very large blow ball
	for i := 1; i <= 10000; i++ {
		tx := newTestTransaction(i, hornet.NullHashBytes, hornet.NullHashBytes)
		hps.OnNewSolidTransaction(tx)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tips, err := hps.SelectTips(ctx)
		assert.Len(t, tips, 2)
		assert.Truef(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()
}

func TestHeaviestSelector_Concurrent(t *testing.T) {
	hps := HPS(hornet.NullHashBytes)
	hashes := []hornet.Hash{hornet.NullHashBytes}
	for i := 0; i < 1000; i++ {
		tx := newTestTransaction(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidTransaction(tx)
		hashes = append(hashes, tx.GetTxHash())
	}

	var wg sync.WaitGroup
	selector := func() {
		defer wg.Done()
		tips, err := hps.SelectTips(context.Background())
		assert.NoError(t, err)
		assert.Len(t, tips, 2)
	}

	wg.Add(2)
	go selector()
	go selector()

	for i := 1000; i < 2000; i++ {
		tx := newTestTransaction(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidTransaction(tx)
		hashes = append(hashes, tx.GetTxHash())
	}
	wg.Wait()
}

func BenchmarkHeaviestSelector_OnNewSolidTransaction(b *testing.B) {
	hps := HPS(hornet.NullHashBytes)
	hashes := []hornet.Hash{hornet.NullHashBytes}
	data := make([]*hornet.Transaction, numBenchmarkTxs)
	for i := 0; i < numBenchmarkTxs; i++ {
		data[i] = newTestTransaction(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hashes = append(hashes, data[i].GetTxHash())
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
	hps := HPS(hornet.NullHashBytes)
	hashes := []hornet.Hash{hornet.NullHashBytes}
	for i := 0; i < numBenchmarkTxs; i++ {
		tx := newTestTransaction(i, hashes[rand.Intn(len(hashes))], hashes[rand.Intn(len(hashes))])
		hps.OnNewSolidTransaction(tx)
		hashes = append(hashes, tx.GetTxHash())
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = hps.SelectTips(context.Background())
	}
}

func newTestTransaction(idx int, trunk, branch hornet.Hash) *hornet.Transaction {
	tx := &transaction.Transaction{
		Hash:              trinary.IntToTrytes(int64(idx), consts.HashTrytesSize),
		Value:             0,
		Timestamp:         uint64(idx),
		TrunkTransaction:  trunk.Trytes(),
		BranchTransaction: branch.Trytes(),
	}
	return hornet.NewTransactionFromTx(tx, nil)
}
