package migrator_test

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/gohornet/hornet/pkg/model/migrator"
	"github.com/iotaledger/iota.go/v2"
	"github.com/stretchr/testify/require"
)

var stateFileName string

func init() {
	dir, err := ioutil.TempDir("", "migrator_test")
	if err != nil {
		log.Fatalf("failed to create temp dir: %s", err)
	}
	stateFileName = filepath.Join(dir, "migrator.state")
}

func TestReceiptFull(t *testing.T) {
	migrator.MaxReceipts = len(serviceTests.entries)
	s, teardown := newTestService(t, 1)
	defer teardown()

	receipt1 := s.Receipt()
	require.EqualValues(t, serviceTests.migratedAt, receipt1.MigratedAt)
	require.True(t, receipt1.Final)
	require.ElementsMatch(t, serviceTests.entries, receipt1.Funds)

	receipt2 := s.Receipt()
	require.Nil(t, receipt2)
}

func TestReceiptAfterClose(t *testing.T) {
	migrator.MaxReceipts = len(serviceTests.entries)
	s, teardown := newTestService(t, 1)

	receipt := s.Receipt()
	require.NotNil(t, receipt)

	teardown()
	require.Nil(t, s.Receipt())
}

func TestReceiptBatch(t *testing.T) {
	migrator.MaxReceipts = 2
	s, teardown := newTestService(t, 1)
	defer teardown()

	receipt1 := s.Receipt()
	require.EqualValues(t, serviceTests.migratedAt, receipt1.MigratedAt)
	require.False(t, receipt1.Final)
	require.Len(t, receipt1.Funds, 2)
	require.Subset(t, serviceTests.entries, receipt1.Funds)

	receipt2 := s.Receipt()
	require.EqualValues(t, serviceTests.migratedAt, receipt2.MigratedAt)
	require.True(t, receipt2.Final)
	require.Len(t, receipt2.Funds, len(serviceTests.entries)-2)
	require.Subset(t, serviceTests.entries, receipt2.Funds)

	receipt3 := s.Receipt()
	require.Nil(t, receipt3)
}

func TestRestoreState(t *testing.T) {
	migrator.MaxReceipts = 2
	s1, teardown1 := newTestService(t, 1)
	defer teardown1()

	receipt1 := s1.Receipt()
	require.EqualValues(t, serviceTests.migratedAt, receipt1.MigratedAt)
	require.False(t, receipt1.Final)
	require.Len(t, receipt1.Funds, 2)
	require.Subset(t, serviceTests.entries, receipt1.Funds)

	err := s1.PersistState()
	require.NoError(t, err)

	// initialize state from file
	s2, teardown2 := newTestService(t, 0)
	defer teardown2()

	receipt2 := s2.Receipt()
	require.EqualValues(t, 2, receipt2.MigratedAt)
	require.True(t, receipt2.Final)
	require.Len(t, receipt2.Funds, len(serviceTests.entries)-2)
	require.Subset(t, serviceTests.entries, receipt2.Funds)
}

func newTestService(t *testing.T, msIndex uint32) (*migrator.MigratorService, func()) {
	s := migrator.NewService(&mockQueryer{}, stateFileName)

	if msIndex > 0 {
		// bootstrap
		err := s.InitState(&msIndex, nil)
		require.NoError(t, err)
	} else {
		// load from state
		err := s.InitState(nil, nil)
		require.NoError(t, err)
	}

	closing := make(chan struct{})
	started := make(chan struct{})
	go func() {
		close(started)
		s.Start(closing, nil)
	}()

	<-started
	return s, func() {
		close(closing)
		os.Remove(stateFileName)
	}
}

type mockQueryer struct{}

func (mockQueryer) QueryMigratedFunds(msIndex uint32) ([]*iota.MigratedFundsEntry, error) {
	if msIndex == serviceTests.migratedAt {
		return serviceTests.entries, nil
	}
	return nil, nil
}

func (mockQueryer) QueryNextMigratedFunds(startIndex uint32) (uint32, []*iota.MigratedFundsEntry, error) {
	if startIndex <= serviceTests.migratedAt {
		return serviceTests.migratedAt, serviceTests.entries, nil
	}
	return serviceTests.migratedAt, nil, nil
}

var serviceTests = struct {
	migratedAt uint32
	entries    []*iota.MigratedFundsEntry
}{
	migratedAt: 2,
	entries: []*iota.MigratedFundsEntry{
		{
			TailTransactionHash: iota.LegacyTailTransactionHash{0},
			Address:             &iota.Ed25519Address{0},
			Deposit:             1_000_000,
		},
		{
			TailTransactionHash: iota.LegacyTailTransactionHash{1},
			Address:             &iota.Ed25519Address{1},
			Deposit:             1_000_000,
		},
		{
			TailTransactionHash: iota.LegacyTailTransactionHash{2},
			Address:             &iota.Ed25519Address{2},
			Deposit:             1_000_000,
		},
	},
}
