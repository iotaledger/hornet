package value

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
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
	target1 := ed25519.NewKeyFromSeed(randSeed())
	target1Addr := iotago.Ed25519AddressFromPubKey(target1.Public().(ed25519.PublicKey))

	target2 := ed25519.NewKeyFromSeed(randSeed())
	target2Addr := iotago.Ed25519AddressFromPubKey(target2.Public().(ed25519.PublicKey))

	var target1Deposit, target2Deposit uint64 = 10_000_000, protoParas.TokenSupply - 10_000_000

	genesisAddrKey := iotago.AddressKeys{Address: &framework.GenesisAddress, Keys: framework.GenesisSeed}
	genesisInputID := &iotago.UTXOInput{TransactionID: [32]byte{}, TransactionOutputIndex: 0}

	// build and sign transaction spending the total supply
	tx, err := builder.NewTransactionBuilder(protoParas.NetworkID()).
		AddInput(&builder.ToBeSignedUTXOInput{
			Address:  &framework.GenesisAddress,
			OutputID: genesisInputID.ID(),
			Output: &iotago.BasicOutput{
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
		Build(protoParas, iotago.NewInMemoryAddressSigner(genesisAddrKey))
	require.NoError(t, err)

	// build block
	block, err := builder.NewBlockBuilder(protoParas.Version).Payload(tx).Build()
	require.NoError(t, err)

	// broadcast to a node
	log.Println("submitting transaction...")
	submittedMsg, err := n.Nodes[2].DebugNodeAPIClient.SubmitBlock(context.Background(), block, protoParas)
	require.NoError(t, err)

	// eventually the message should be confirmed
	submittedMsgID, err := submittedMsg.ID()
	require.NoError(t, err)

	log.Println("checking that the transaction gets confirmed...")
	require.Eventually(t, func() bool {
		msgMeta, err := n.Coordinator().DebugNodeAPIClient.BlockMetadataByBlockID(context.Background(), *submittedMsgID)
		if err != nil {
			return false
		}
		if msgMeta.LedgerInclusionState == nil {
			return false
		}
		return *msgMeta.LedgerInclusionState == "included"
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

const seedLength = ed25519.SeedSize

func randSeed() []byte {
	var b [seedLength]byte
	_, err := rand.Read(b[:])
	if err != nil {
		panic(err)
	}
	return b[:]
}
