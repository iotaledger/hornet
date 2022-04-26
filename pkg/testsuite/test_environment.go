package testsuite

import (
	"crypto/ed25519"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/milestonemanager"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/logger"
	iotago "github.com/iotaledger/iota.go/v3"
)

// TestEnvironment holds the state of the test environment.
type TestEnvironment struct {
	// TestInterface is the common interface for tests and benchmarks.
	TestInterface testing.TB

	// Milestones are the created milestones by the coordinator during the test.
	Milestones storage.CachedMilestones

	// cachedMessages is used to cleanup all messages at the end of a test.
	cachedMessages storage.CachedMessages

	// showConfirmationGraphs is set if pictures of the confirmation graph should be externally opened during the test.
	showConfirmationGraphs bool

	// PoWHandler holds the PoWHandler instance.
	PoWHandler *pow.Handler

	// PoWMinScore used in the PoWHandler instance.
	PoWMinScore float64

	// protoParas are the protocol parameters of the network.
	protoParas *iotago.ProtocolParameters

	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given message in relation to the current CMI before it gets lazy.
	belowMaxDepth milestone.Index

	// coo holds the coordinator instance.
	coo *MockCoo

	// TempDir is the directory that contains the temporary files for the test.
	TempDir string

	// tangleStore is the temporary key value store for the test holding the tangle.
	tangleStore kvstore.KVStore

	// utxoStore is the temporary key value store for the test holding the utxo ledger.
	utxoStore kvstore.KVStore

	// storage is the tangle storage for this test.
	storage *storage.Storage

	// syncManager is used to determine the sync status of the node in this test.
	syncManager *syncmanager.SyncManager

	// milestoneManager is used to retrieve, verify and store milestones.
	milestoneManager *milestonemanager.MilestoneManager

	// serverMetrics holds metrics about the tangle.
	serverMetrics *metrics.ServerMetrics

	// GenesisOutput marks the initial output created when bootstrapping the tangle.
	GenesisOutput *utxo.Output

	// OnLedgerUpdatedFunc callback that will be called after the ledger gets updated during confirmation. This is equivalent to the tangle.LedgerUpdated event.
	OnLedgerUpdatedFunc OnLedgerUpdatedFunc
}

type OnMilestoneConfirmedFunc func(confirmation *whiteflag.Confirmation)
type OnLedgerUpdatedFunc func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents)

// SetupTestEnvironment initializes a clean database with initial snapshot,
// configures a coordinator with a clean state, bootstraps the network and issues the first "numberOfMilestones" milestones.
func SetupTestEnvironment(testInterface testing.TB, genesisAddress *iotago.Ed25519Address, numberOfMilestones int, belowMaxDepth int, targetScore float64, showConfirmationGraphs bool) *TestEnvironment {

	te := &TestEnvironment{
		TestInterface:          testInterface,
		Milestones:             make(storage.CachedMilestones, 0),
		cachedMessages:         make(storage.CachedMessages, 0),
		showConfirmationGraphs: showConfirmationGraphs,
		PoWHandler:             pow.New(targetScore, 5*time.Second),
		protoParas: &iotago.ProtocolParameters{
			Version:     2,
			NetworkName: "alphapnet1",
			Bech32HRP:   iotago.PrefixTestnet,
			MinPowScore: targetScore,
			RentStructure: iotago.RentStructure{
				VByteCost:    500,
				VBFactorData: 1,
				VBFactorKey:  10,
			},
			TokenSupply: 2_779_530_283_277_761,
		},
		belowMaxDepth: milestone.Index(belowMaxDepth),
		serverMetrics: &metrics.ServerMetrics{},
	}

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(testInterface, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	cooPrvKey1, err := utils.ParseEd25519PrivateKeyFromString("651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c")
	require.NoError(te.TestInterface, err)
	cooPrvKey2, err := utils.ParseEd25519PrivateKeyFromString("0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c")
	require.NoError(te.TestInterface, err)

	tempDir, err := ioutil.TempDir("", fmt.Sprintf("test_%s", te.TestInterface.Name()))
	require.NoError(te.TestInterface, err)
	te.TempDir = tempDir

	te.TestInterface.Logf("Testdir: %s", tempDir)

	cooPrivateKeys := []ed25519.PrivateKey{cooPrvKey1, cooPrvKey2}

	keyManager := keymanager.New()
	for _, key := range cooPrivateKeys {
		keyManager.AddKeyRange(key.Public().(ed25519.PublicKey), 0, 0)
	}

	te.tangleStore = mapdb.NewMapDB()
	te.utxoStore = mapdb.NewMapDB()
	te.storage, err = storage.New(te.tangleStore, te.utxoStore, TestProfileCaches)
	require.NoError(te.TestInterface, err)

	// Initialize SEP
	te.storage.SolidEntryPointsAddWithoutLocking(hornet.NullMessageID(), 0)

	// Initialize SyncManager
	te.syncManager, err = syncmanager.New(te.storage.UTXOManager(), belowMaxDepth)
	require.NoError(te.TestInterface, err)

	// Initialize MilestoneManager
	te.milestoneManager = milestonemanager.New(te.storage, te.syncManager, keyManager, len(cooPrivateKeys))

	// Initialize UTXO
	output := &iotago.BasicOutput{
		Amount: te.protoParas.TokenSupply,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: genesisAddress,
			},
		},
	}
	te.GenesisOutput = utxo.CreateOutput(&iotago.OutputID{}, hornet.NullMessageID(), 0, 0, output)
	err = te.storage.UTXOManager().AddUnspentOutput(te.GenesisOutput)
	require.NoError(te.TestInterface, err)

	err = te.storage.UTXOManager().StoreUnspentTreasuryOutput(&utxo.TreasuryOutput{MilestoneID: [32]byte{}, Amount: 0})
	require.NoError(te.TestInterface, err)

	te.AssertTotalSupplyStillValid()

	// Start up the coordinator
	te.configureCoordinator(cooPrivateKeys, keyManager)
	require.NotNil(te.TestInterface, te.coo)

	te.VerifyCMI(1)

	for i := 1; i <= numberOfMilestones; i++ {
		_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{hornet.NullMessageID()}, false)
		require.Equal(te.TestInterface, 1, confStats.MessagesReferenced)                  // 1 for previous milestone
		require.Equal(te.TestInterface, 1, confStats.MessagesExcludedWithoutTransactions) // 1 for previous milestone
		require.Equal(te.TestInterface, 0, confStats.MessagesIncludedWithTransactions)
		require.Equal(te.TestInterface, 0, confStats.MessagesExcludedWithConflictingTransactions)
	}

	return te
}

func (te *TestEnvironment) ConfigureUTXOCallbacks(onLedgerUpdatedFunc OnLedgerUpdatedFunc) {
	te.OnLedgerUpdatedFunc = onLedgerUpdatedFunc
}

func (te *TestEnvironment) ProtocolParameters() *iotago.ProtocolParameters {
	return te.protoParas
}

func (te *TestEnvironment) Storage() *storage.Storage {
	return te.storage
}

func (te *TestEnvironment) UTXOManager() *utxo.Manager {
	return te.storage.UTXOManager()
}

func (te *TestEnvironment) SyncManager() *syncmanager.SyncManager {
	return te.syncManager
}

func (te *TestEnvironment) BelowMaxDepth() milestone.Index {
	return te.belowMaxDepth
}

// LastMilestoneMessageID is the message ID of the last issued milestone.
func (te *TestEnvironment) LastMilestoneMessageID() hornet.MessageID {
	return te.coo.LastMilestoneMessageID
}

// LastMilestoneIndex is the index of the last issued milestone.
func (te *TestEnvironment) LastMilestoneIndex() milestone.Index {
	return te.coo.LastMilestoneIndex
}

// CleanupTestEnvironment cleans up everything at the end of the test.
func (te *TestEnvironment) CleanupTestEnvironment(removeTempDir bool) {
	te.cachedMessages.Release(true) // message -1
	te.cachedMessages = nil

	te.Milestones.Release(true) // milestone -1
	te.cachedMessages = nil

	// this should not hang, i.e. all objects should be released
	te.storage.ShutdownStorages()

	err := te.tangleStore.Clear()
	require.NoError(te.TestInterface, err)

	err = te.utxoStore.Clear()
	require.NoError(te.TestInterface, err)

	if removeTempDir && te.TempDir != "" {
		_ = os.RemoveAll(te.TempDir)
	}
}

func (te *TestEnvironment) NewTestMessage(index int, parents hornet.MessageIDs) *storage.MessageMetadata {
	msg := te.NewMessageBuilder(fmt.Sprintf("%d", index)).Parents(parents).BuildTaggedData().Store()
	cachedMsgMeta := te.Storage().CachedMessageMetadataOrNil(msg.StoredMessageID()) // meta +1
	defer cachedMsgMeta.Release(true)                                               // meta -1
	return cachedMsgMeta.Metadata()
}

// BuildTangle builds a tangle structure without a tipselection algorithm, but random tips from the last
// messages in the last belowMaxDepth milestones.
func (te *TestEnvironment) BuildTangle(initMessagesCount int,
	belowMaxDepth int,
	milestonesCount int,
	minMessagesPerMilestone int,
	maxMessagesPerMilestone int,
	onNewMessage func(cmi milestone.Index, msgMeta *storage.MessageMetadata),
	milestoneTipSelectFunc func(messages hornet.MessageIDs, messagesPerMilestones []hornet.MessageIDs) hornet.MessageIDs,
	onNewMilestone func(msIndex milestone.Index, messages hornet.MessageIDs, conf *whiteflag.Confirmation, confStats *whiteflag.ConfirmedMilestoneStats)) (messages hornet.MessageIDs, messagesPerMilestones []hornet.MessageIDs) {

	messageTotalCount := 0
	messages = hornet.MessageIDs{}
	messagesPerMilestones = make([]hornet.MessageIDs, 0)

	getParents := func() hornet.MessageIDs {

		if len(messages) < initMessagesCount {
			// reference the first milestone at the beginning
			return hornet.MessageIDs{te.Milestones[0].Milestone().MessageID}
		}

		parents := hornet.MessageIDs{}
		for j := 2; j <= 2+rand.Intn(7); j++ {
			msIndex := rand.Intn(belowMaxDepth)
			if msIndex > len(messagesPerMilestones)-1 {
				msIndex = rand.Intn(len(messagesPerMilestones))
			}
			milestoneMessages := messagesPerMilestones[len(messagesPerMilestones)-1-msIndex]
			if len(milestoneMessages) == 0 {
				// use the milestone hash
				parents = append(parents, te.Milestones[len(te.Milestones)-1-msIndex].Milestone().MessageID)
				continue
			}
			parents = append(parents, milestoneMessages[rand.Intn(len(milestoneMessages))])
		}

		return parents.RemoveDupsAndSortByLexicalOrder()
	}

	// build a tangle
	for msIndex := 2; msIndex < milestonesCount; msIndex++ {
		messagesPerMilestones = append(messagesPerMilestones, hornet.MessageIDs{})

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		msgsCount := minMessagesPerMilestone + rand.Intn(maxMessagesPerMilestone-minMessagesPerMilestone)
		for msgCount := 0; msgCount < msgsCount; msgCount++ {
			messageTotalCount++
			msgMeta := te.NewTestMessage(messageTotalCount, getParents())

			messages = append(messages, msgMeta.MessageID())
			messagesPerMilestones[len(messagesPerMilestones)-1] = append(messagesPerMilestones[len(messagesPerMilestones)-1], msgMeta.MessageID())

			if onNewMessage != nil {
				onNewMessage(cmi, msgMeta)
			}
		}

		// confirm the new cone
		conf, confStats := te.IssueAndConfirmMilestoneOnTips(milestoneTipSelectFunc(messages, messagesPerMilestones), false)
		if onNewMilestone != nil {
			onNewMilestone(conf.MilestoneIndex, messages, conf, confStats)
		}
	}

	return messages, messagesPerMilestones
}
