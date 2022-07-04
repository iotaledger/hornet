package value

import (
	"context"
	"crypto/ed25519"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/integration-tests/tester/framework"
	"github.com/iotaledger/hornet/pkg/tpkg"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/builder"
)

// TestValue boots up a statically peered network and then checks that spending
// the genesis output to create multiple new output works.
func TestValue(t *testing.T) {
	n, err := f.CreateStaticNetwork("test_value", nil, framework.DefaultStaticPeeringLayout())
	require.NoError(t, err)
	defer framework.ShutdownNetwork(t, n)

	syncCtx, syncCtxCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer syncCtxCancel()
	assert.NoError(t, n.AwaitAllSync(syncCtx))

	infoRes, err := n.Coordinator().DebugNodeAPIClient.Info(context.Background())
	require.NoError(t, err)
	protoParas := &infoRes.Protocol

	// create two targets
	target1 := ed25519.NewKeyFromSeed(tpkg.RandSeed())
	target1Addr := iotago.Ed25519AddressFromPubKey(target1.Public().(ed25519.PublicKey))

	target2 := ed25519.NewKeyFromSeed(tpkg.RandSeed())
	target2Addr := iotago.Ed25519AddressFromPubKey(target2.Public().(ed25519.PublicKey))

	var target1Deposit, target2Deposit uint64 = 10_000_000, protoParas.TokenSupply - 10_000_000

	genesisAddrKey := iotago.AddressKeys{Address: &framework.GenesisAddress, Keys: framework.GenesisSeed}
	genesisInputID := &iotago.UTXOInput{TransactionID: [32]byte{}, TransactionOutputIndex: 0}

	// build and sign transaction spending the total supply and create block
	block, err := builder.NewTransactionBuilder(protoParas.NetworkID()).
		AddInput(&builder.TxInput{
			UnlockTarget: &framework.GenesisAddress,
			InputID:      genesisInputID.ID(),
			Input: &iotago.BasicOutput{
				Amount: protoParas.TokenSupply,
				Conditions: iotago.UnlockConditions{
					&iotago.AddressUnlockCondition{
						Address: &framework.GenesisAddress,
					},
				},
			},
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: target1Deposit,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: &target1Addr,
				},
			},
		}).
		AddOutput(&iotago.BasicOutput{
			Amount: target2Deposit,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: &target2Addr,
				},
			},
		}).
		BuildAndSwapToBlockBuilder(protoParas, iotago.NewInMemoryAddressSigner(genesisAddrKey), nil).
		ProtocolVersion(protoParas.Version).
		Build()
	require.NoError(t, err)

	// broadcast to a node
	log.Println("submitting transaction...")
	submittedBlock, err := n.Nodes[2].DebugNodeAPIClient.SubmitBlock(context.Background(), block, protoParas)
	require.NoError(t, err)

	// eventually the block should be confirmed
	submittedBlockID, err := submittedBlock.ID()
	require.NoError(t, err)

	log.Println("checking that the transaction gets confirmed...")
	require.Eventually(t, func() bool {
		blockMeta, err := n.Coordinator().DebugNodeAPIClient.BlockMetadataByBlockID(context.Background(), submittedBlockID)
		if err != nil {
			return false
		}
		if blockMeta.LedgerInclusionState == nil {
			return false
		}
		return *blockMeta.LedgerInclusionState == "included"
	}, 30*time.Second, 100*time.Millisecond)

	// check that indeed the balances are available
	balance, err := n.Coordinator().DebugNodeAPIClient.BalanceByAddress(context.Background(), &framework.GenesisAddress)
	require.NoError(t, err)
	require.Zero(t, balance)

	balance, err = n.Coordinator().DebugNodeAPIClient.BalanceByAddress(context.Background(), &target1Addr)
	require.NoError(t, err)
	require.EqualValues(t, target1Deposit, balance)

	balance, err = n.Coordinator().DebugNodeAPIClient.BalanceByAddress(context.Background(), &target2Addr)
	require.NoError(t, err)
	require.EqualValues(t, target2Deposit, balance)

	// the genesis output should be spent
	outputRes, err := n.Coordinator().DebugNodeAPIClient.OutputByID(context.Background(), genesisInputID.ID())
	require.NoError(t, err)
	require.True(t, outputRes.Metadata.Spent)
}
