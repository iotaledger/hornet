package value

import (
	"context"
	"crypto/rand"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/integration-tests/tester/framework"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
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

	// create two targets
	target1 := ed25519.NewKeyFromSeed(randSeed())
	target1Addr := iotago.AddressFromEd25519PubKey(target1.Public().(ed25519.PublicKey))

	target2 := ed25519.NewKeyFromSeed(randSeed())
	target2Addr := iotago.AddressFromEd25519PubKey(target2.Public().(ed25519.PublicKey))

	var target1Deposit, target2Deposit uint64 = 10_000_000, iotago.TokenSupply - 10_000_000

	genesisAddrKey := iotago.AddressKeys{Address: &framework.GenesisAddress, Keys: framework.GenesisSeed}
	genesisInputID := &iotago.UTXOInput{TransactionID: [32]byte{}, TransactionOutputIndex: 0}

	// build and sign transaction spending the total supply
	tx, err := iotago.NewTransactionBuilder().
		AddInput(&iotago.ToBeSignedUTXOInput{
			Address: &framework.GenesisAddress,
			Input:   genesisInputID,
		}).
		AddOutput(&iotago.SigLockedSingleOutput{
			Address: &target1Addr,
			Amount:  target1Deposit,
		}).
		AddOutput(&iotago.SigLockedSingleOutput{
			Address: &target2Addr,
			Amount:  target2Deposit,
		}).
		Build(iotago.NewInMemoryAddressSigner(genesisAddrKey))
	require.NoError(t, err)

	// build message
	msg, err := iotago.NewMessageBuilder().Payload(tx).Build()
	require.NoError(t, err)

	// broadcast to a node
	log.Println("submitting transaction...")
	submittedMsg, err := n.Nodes[0].DebugNodeAPIClient.SubmitMessage(msg)
	require.NoError(t, err)

	// eventually the message should be confirmed
	submittedMsgID, err := submittedMsg.ID()
	require.NoError(t, err)

	log.Println("checking that the transaction gets confirmed...")
	require.Eventually(t, func() bool {
		msgMeta, err := n.Coordinator().DebugNodeAPIClient.MessageMetadataByMessageID(*submittedMsgID)
		if err != nil {
			return false
		}
		if msgMeta.LedgerInclusionState == nil {
			return false
		}
		return *msgMeta.LedgerInclusionState == "included"
	}, 30*time.Second, 100*time.Millisecond)

	// check that indeed the balances are available
	res, err := n.Coordinator().DebugNodeAPIClient.BalanceByEd25519Address(&framework.GenesisAddress)
	require.NoError(t, err)
	require.Zero(t, res.Balance)

	res, err = n.Coordinator().DebugNodeAPIClient.BalanceByEd25519Address(&target1Addr)
	require.NoError(t, err)
	require.EqualValues(t, target1Deposit, res.Balance)

	res, err = n.Coordinator().DebugNodeAPIClient.BalanceByEd25519Address(&target2Addr)
	require.NoError(t, err)
	require.EqualValues(t, target2Deposit, res.Balance)

	// the genesis output should be spent
	outputRes, err := n.Coordinator().DebugNodeAPIClient.OutputByID(genesisInputID.ID())
	require.NoError(t, err)
	require.True(t, outputRes.Spent)
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
