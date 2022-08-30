//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package migration_test

import (
	"context"
	"encoding/binary"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/integration-tests/tester/framework"
	iotago "github.com/iotaledger/iota.go/v3"
)

// TestBatch boots up a statically peered network which runs with a white-flag mock server in order
// to validate that the migrated funds entry limit is respected correctly.
func TestBatch(t *testing.T) {
	const (
		initialTreasuryTokens uint64 = 10_000_000_000
		migratedFundsCount           = 127 + 128 + 1
		migrationTokens       uint64 = 1_000_000
		totalMigrationTokens         = migratedFundsCount * migrationTokens
	)

	// receipts per migrated at index
	receipts := map[uint32]int{1: 1, 2: 2, 3: 1}
	var totalReceipts int
	for _, n := range receipts {
		totalReceipts += n
	}

	n, err := f.CreateStaticNetwork("test_migration_batch", &framework.IntegrationNetworkConfig{
		SpawnWhiteFlagMockServer:  true,
		WhiteFlagMockServerConfig: framework.DefaultWhiteFlagMockServerConfig("wfmock_batch", "wfmock_config_batch.json"),
	}, framework.DefaultStaticPeeringLayout(), func(index int, cfg *framework.AppConfig) {

		cfg.Receipts.IgnoreSoftErrors = false
		cfg.Receipts.Validate = true
		cfg.Receipts.Validator.APIAddress = "http://wfmock_batch:14265"
		cfg.Receipts.Validator.APITimeout = 5 * time.Second
		cfg.Receipts.Validator.CoordinatorAddress = CoordinatorAddress
		cfg.Receipts.Validator.CoordinatorMerkleTreeDepth = 8

		switch {
		case index == 0:
			cfg.WithReceipts()
			cfg.INXCoo.Validator = cfg.Receipts.Validator
			cfg.INXCoo.Migrator.StartIndex = 1
		default:
			cfg.Receipts.Enabled = true
		}

		cfg.Snapshot.FullSnapshotFilePath = FullSnapshotPath
		cfg.Snapshot.DeltaSnapshotFilePath = DeltaSnapshotPath // doesn't exist so the node will only load the full one
	})
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer syncCtxCancel()

	assert.NoError(t, n.AwaitAllSync(syncCtx))

	// eventually all migrations should have happened
	log.Println("waiting for treasury to be reduced to correct amount after migrations ...")
	require.Eventually(t, func() bool {
		treasury, err := n.Coordinator().DebugNodeAPIClient.Treasury(context.Background())
		if err != nil {
			log.Printf("failed to get current treasury: %s", err)

			return false
		}
		amount, err := iotago.DecodeUint64(treasury.Amount)
		if err != nil {
			log.Printf("failed to decode treasury amount: %s", err)

			return false
		}

		return amount == initialTreasuryTokens-totalMigrationTokens
	}, 2*time.Minute, time.Second)

	// checking that funds were migrated in appropriate receipts
	log.Println("checking receipts ...")
	receiptTuples, err := n.Coordinator().DebugNodeAPIClient.Receipts(context.Background())
	require.NoError(t, err)
	require.Lenf(t, receiptTuples, totalReceipts, "expected %d receipts in total", totalReceipts)
	for migratedAt, numReceipts := range receipts {
		receiptTuples, err := n.Coordinator().DebugNodeAPIClient.ReceiptsByMigratedAtIndex(context.Background(), migratedAt)
		require.NoError(t, err)
		require.Lenf(t, receiptTuples, numReceipts, "expected %d receipts for index %d", totalReceipts, migratedAt)
	}

	// check that indeed the funds were correctly minted
	log.Println("checking that migrated funds are available ...")
	for i := uint32(0); i < migratedFundsCount; i++ {
		var addr iotago.Ed25519Address
		binary.LittleEndian.PutUint32(addr[:], i)
		balance, err := n.Coordinator().DebugNodeAPIClient.BalanceByAddress(context.Background(), &addr)
		require.NoError(t, err)
		require.EqualValues(t, migrationTokens, balance)
	}
}
