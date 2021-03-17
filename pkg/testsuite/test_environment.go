package testsuite

import (
	"fmt"
	"io/ioutil"
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
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/utils"
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
	// TestState is the state of the current test case.
	TestState *testing.T

	// Milestones are the created milestones by the coordinator during the test.
	Milestones storage.CachedMilestones

	// cachedMessages is used to cleanup all messages at the end of a test.
	cachedMessages storage.CachedMessages

	// showConfirmationGraphs is set if pictures of the confirmation graph should be externally opened during the test.
	showConfirmationGraphs bool

	// PowHandler holds the PowHandler instance.
	PowHandler *pow.Handler

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
func SetupTestEnvironment(testState *testing.T, genesisAddress *iotago.Ed25519Address, numberOfMilestones int, belowMaxDepth int, targetScore float64, showConfirmationGraphs bool) *TestEnvironment {

	te := &TestEnvironment{
		TestState:              testState,
		Milestones:             make(storage.CachedMilestones, 0),
		cachedMessages:         make(storage.CachedMessages, 0),
		showConfirmationGraphs: showConfirmationGraphs,
		PowHandler:             pow.New(nil, targetScore, "", 30*time.Second),
		networkID:              iotago.NetworkIDFromString("alphanet1"),
		lastMilestoneMessageID: hornet.GetNullMessageID(),
		serverMetrics:          &metrics.ServerMetrics{},
	}

	cooPrvKey1, err := utils.ParseEd25519PrivateKeyFromString("651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c")
	require.NoError(testState, err)
	cooPrvKey2, err := utils.ParseEd25519PrivateKeyFromString("0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c")
	require.NoError(testState, err)

	tempDir, err := ioutil.TempDir("", fmt.Sprintf("test_%s", testState.Name()))
	require.NoError(te.TestState, err)
	te.tempDir = tempDir

	testState.Logf("Testdir: %s", tempDir)

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
	require.NotNil(testState, te.coo)

	te.VerifyCMI(1)

	for i := 1; i <= numberOfMilestones; i++ {
		_, confStats := te.IssueAndConfirmMilestoneOnTip(hornet.GetNullMessageID(), false)
		require.Equal(testState, 1, confStats.MessagesReferenced)                  // 1 for milestone
		require.Equal(testState, 1, confStats.MessagesExcludedWithoutTransactions) // 1 for milestone
		require.Equal(testState, 0, confStats.MessagesIncludedWithTransactions)
		require.Equal(testState, 0, confStats.MessagesExcludedWithConflictingTransactions)
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
