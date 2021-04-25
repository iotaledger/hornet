package testsuite

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/metrics"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

var (
	// setupTangleOnce is used to avoid panics when running multiple tests.
	setupTangleOnce sync.Once
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

	// networkID is the network ID used for this test network
	networkID uint64

	// cooPrivateKey holds the coo private key
	cooPrivateKey ed25519.PrivateKey

	// coo holds the coordinator instance.
	coo *coordinator.Coordinator

	// lastMilestoneMessageID is the message ID of the last issued milestone.
	lastMilestoneMessageID hornet.MessageID

	// tempDir is the directory that contains the temporary files for the test.
	tempDir string

	// store is the temporary key value store for the test.
	store kvstore.KVStore

	// storage is the tangle storage for this test
	storage *storage.Storage

	// serverMetrics holds metrics about the tangle
	serverMetrics *metrics.ServerMetrics

	// GenesisOutput marks the initial output created when bootstrapping the tangle
	GenesisOutput *utxo.Output
}

// searchProjectRootFolder searches the hornet root directory.
// this is used to always find the "assets" folder if executed from different test cases.
func searchProjectRootFolder() string {
	wd, _ := os.Getwd()

	for !strings.HasSuffix(wd, "pkg") && !strings.HasSuffix(wd, "plugins") {
		wd = filepath.Dir(wd)
	}

	// one more time to get to the root dir
	wd = filepath.Dir(wd)

	return wd
}

// SetupTestEnvironment initializes a clean database with initial snapshot,
// configures a coordinator with a clean state, bootstraps the network and issues the first "numberOfMilestones" milestones.
func SetupTestEnvironment(testInterface testing.TB, genesisAddress *iotago.Ed25519Address, numberOfMilestones int, belowMaxDepth int, targetScore float64, showConfirmationGraphs bool) *TestEnvironment {

	te := &TestEnvironment{
		TestInterface:          testInterface,
		Milestones:             make(storage.CachedMilestones, 0),
		cachedMessages:         make(storage.CachedMessages, 0),
		showConfirmationGraphs: showConfirmationGraphs,
		PoWHandler:             pow.New(nil, targetScore, 5*time.Second, "", 30*time.Second),
		networkID:              iotago.NetworkIDFromString("alphanet1"),
		lastMilestoneMessageID: hornet.GetNullMessageID(),
		serverMetrics:          &metrics.ServerMetrics{},
	}

	cooPrvKey1, err := utils.ParseEd25519PrivateKeyFromString("651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c")
	require.NoError(testInterface, err)
	cooPrvKey2, err := utils.ParseEd25519PrivateKeyFromString("0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c")
	require.NoError(testInterface, err)

	tempDir, err := ioutil.TempDir("", fmt.Sprintf("test_%s", testInterface.Name()))
	require.NoError(te.TestInterface, err)
	te.tempDir = tempDir

	testInterface.Logf("Testdir: %s", tempDir)

	cooPrivateKeys := []ed25519.PrivateKey{cooPrvKey1, cooPrvKey2}

	keyManager := keymanager.New()
	for _, key := range cooPrivateKeys {
		keyManager.AddKeyRange(key.Public().(ed25519.PublicKey), 0, 0)
	}

	te.store = mapdb.NewMapDB()
	te.storage = storage.New(te.tempDir, te.store, TestProfileCaches, belowMaxDepth, keyManager, len(cooPrivateKeys))

	// Initialize SEP
	te.storage.SolidEntryPointsAdd(hornet.GetNullMessageID(), 0)

	// Initialize UTXO
	te.GenesisOutput = utxo.CreateOutput(&iotago.UTXOInputID{}, hornet.GetNullMessageID(), iotago.OutputSigLockedSingleOutput, genesisAddress, iotago.TokenSupply)
	te.storage.UTXO().AddUnspentOutput(te.GenesisOutput)
	te.storage.UTXO().StoreUnspentTreasuryOutput(&utxo.TreasuryOutput{MilestoneID: [32]byte{}, Amount: 0})

	te.AssertTotalSupplyStillValid()

	// Start up the coordinator
	te.configureCoordinator(cooPrivateKeys, keyManager)
	require.NotNil(testInterface, te.coo)

	te.VerifyCMI(1)

	for i := 1; i <= numberOfMilestones; i++ {
		_, confStats := te.IssueAndConfirmMilestoneOnTips(hornet.MessageIDs{hornet.GetNullMessageID()}, false)
		require.Equal(testInterface, 1, confStats.MessagesReferenced)                  // 1 for milestone
		require.Equal(testInterface, 1, confStats.MessagesExcludedWithoutTransactions) // 1 for milestone
		require.Equal(testInterface, 0, confStats.MessagesIncludedWithTransactions)
		require.Equal(testInterface, 0, confStats.MessagesExcludedWithConflictingTransactions)
	}

	return te
}

func (te *TestEnvironment) Storage() *storage.Storage {
	return te.storage
}

func (te *TestEnvironment) UTXO() *utxo.Manager {
	return te.storage.UTXO()
}

// CleanupTestEnvironment cleans up everything at the end of the test.
func (te *TestEnvironment) CleanupTestEnvironment(removeTempDir bool) {
	te.cachedMessages.Release(true)
	te.cachedMessages = nil

	te.Milestones.Release(true)
	te.cachedMessages = nil

	// this should not hang, i.e. all objects should be released
	te.storage.ShutdownStorages()

	te.store.Clear()

	if removeTempDir && te.tempDir != "" {
		os.RemoveAll(te.tempDir)
	}
}

func (te *TestEnvironment) NewTestMessage(index int, parents hornet.MessageIDs) *storage.MessageMetadata {
	msg := te.NewMessageBuilder(fmt.Sprintf("%d", index)).Parents(parents).BuildIndexation().Store()
	cachedMsgMeta := te.Storage().GetCachedMessageMetadataOrNil(msg.StoredMessageID()) // metadata +1
	defer cachedMsgMeta.Release(true)
	return cachedMsgMeta.GetMetadata()
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
			return hornet.MessageIDs{te.Milestones[0].GetMilestone().MessageID}
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
				parents = append(parents, te.Milestones[len(te.Milestones)-1-msIndex].GetMilestone().MessageID)
				continue
			}
			parents = append(parents, milestoneMessages[rand.Intn(len(milestoneMessages))])
		}

		return parents.RemoveDupsAndSortByLexicalOrder()
	}

	// build a tangle
	for msIndex := 2; msIndex < milestonesCount; msIndex++ {
		messagesPerMilestones = append(messagesPerMilestones, hornet.MessageIDs{})

		cmi := te.Storage().GetConfirmedMilestoneIndex()

		msgsCount := minMessagesPerMilestone + rand.Intn(maxMessagesPerMilestone-minMessagesPerMilestone)
		for msgCount := 0; msgCount < msgsCount; msgCount++ {
			messageTotalCount++
			msgMeta := te.NewTestMessage(messageTotalCount, getParents())

			messages = append(messages, msgMeta.GetMessageID())
			messagesPerMilestones[len(messagesPerMilestones)-1] = append(messagesPerMilestones[len(messagesPerMilestones)-1], msgMeta.GetMessageID())

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
