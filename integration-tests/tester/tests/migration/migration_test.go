//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package migration_test

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/integration-tests/tester/framework"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	FullSnapshotPath   = "/assets/migration_full_snapshot.bin"
	DeltaSnapshotPath  = "/assets/migration_delta_snapshot.bin"
	CoordinatorAddress = "QYO9OXGLVLUKMCEONVAPEWXUFQTGTTHPZZOTOFHYUFVPJJLLFAYBIOFMTUSVXVRQFSUIQXJUGZQDDDULY"
)

// TestMigration boots up a statically peered network which runs with a white-flag mock server
// in order to validate an entire migration flow.
// The migration full snapshot used to bootstrap the C2 network has 10000000000 allocated on the treasury.
func TestMigration(t *testing.T) {
	const initialTreasuryTokens uint64 = 10_000_000_000

	type tuple struct {
		//nolint:structcheck // milestoneIndex not used in the tests yet
		milestoneIndex int
		amount         uint64
	}

	migrations := map[string]tuple{
		"0x2c2bb061de51f09ce2ccee44a626762bbb766997e1c8098eaec2e3a089c65843": {1, 1000000},
		"0x0be076ce68461235e6c43241160a62faeff1536f1a79d816e054f7c7e0c68661": {3, 2000000},
		"0x583e0b3a19e15c1ad09f53cc152cafde9109cd56486107249feb77d1ff16ce61": {3, 4000000},
		"0x0a9a5b39438f3fe9107facd9bf6df747573a8c5050c467f4dfcc32d82e3560f8": {5, 10000000},
	}

	var totalMigration uint64
	for _, tuple := range migrations {
		totalMigration += tuple.amount
	}

	n, err := f.CreateStaticNetwork("test_migration", &framework.IntegrationNetworkConfig{
		SpawnWhiteFlagMockServer:  true,
		WhiteFlagMockServerConfig: framework.DefaultWhiteFlagMockServerConfig("wfmock", "wfmock_config.json"),
	}, framework.DefaultStaticPeeringLayout(), func(index int, cfg *framework.AppConfig) {
		cfg.Receipts.IgnoreSoftErrors = false
		cfg.Receipts.Validate = true
		cfg.Receipts.Validator.APIAddress = "http://wfmock:14265"
		cfg.Receipts.Validator.APITimeout = 5 * time.Second
		cfg.Receipts.Validator.CoordinatorAddress = CoordinatorAddress
		cfg.Receipts.Validator.CoordinatorMerkleTreeDepth = 8

		switch {
		case index == 0:
			cfg.WithReceipts()
			cfg.INXCoo.Migrator.StartIndex = 1
			cfg.INXCoo.Validator = cfg.Receipts.Validator
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

		return amount == initialTreasuryTokens-totalMigration
	}, 2*time.Minute, time.Second)

	// checking that funds were migrated in appropriate receipts
	log.Println("checking receipts ...")
	receiptTuples, err := n.Coordinator().DebugNodeAPIClient.Receipts(context.Background())
	require.NoError(t, err)

	require.Len(t, receiptTuples, 3, "expected 3 receipts in total")

	for _, tuple := range receiptTuples {
		r := tuple.Receipt
		var entriesFound int
		for _, migEntry := range r.Funds {
			for addr, balance := range migrations {
				if addr == migEntry.Address.(*iotago.Ed25519Address).String() {
					entriesFound++
					require.EqualValues(t, migEntry.Deposit, balance.amount)

					break
				}
			}
		}
		require.EqualValues(t, entriesFound, len(r.Funds))
	}

	// check that indeed the funds were correctly minted
	log.Println("checking that migrated funds are available ...")
	for addr, tuple := range migrations {
		balance, err := n.Coordinator().DebugNodeAPIClient.BalanceByAddress(context.Background(), iotago.MustParseEd25519AddressFromHexString(addr))
		require.NoError(t, err)
		require.EqualValues(t, tuple.amount, balance)
	}
}

// TestAPIError boots up a statically peered network without a white-flag mock server in order
// to validate that API errors are ignored.
func TestAPIError(t *testing.T) {
	// start a network without a mock
	n, err := f.CreateStaticNetwork("test_migration_api_error", nil, framework.DefaultStaticPeeringLayout(),
		func(index int, cfg *framework.AppConfig) {
			cfg.Receipts.IgnoreSoftErrors = true
			cfg.Receipts.Validate = true
			cfg.Receipts.Validator.APIAddress = "http://localhost:14265"
			cfg.Receipts.Validator.APITimeout = 5 * time.Second
			cfg.Receipts.Validator.CoordinatorAddress = CoordinatorAddress
			cfg.Receipts.Validator.CoordinatorMerkleTreeDepth = 8

			switch {
			case index == 0:
				cfg.WithReceipts()
				cfg.INXCoo.Migrator.StartIndex = 1
				cfg.INXCoo.Validator = cfg.Receipts.Validator
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
}
