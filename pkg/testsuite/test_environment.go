//nolint:forcetypeassert,varnamelen,revive,exhaustruct,gosec // we don't care about these linters in test cases
package testsuite

import (
	"crypto/ed25519"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore/mapdb"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/v2/pkg/metrics"
	"github.com/iotaledger/hornet/v2/pkg/model/milestonemanager"
	"github.com/iotaledger/hornet/v2/pkg/model/storage"
	"github.com/iotaledger/hornet/v2/pkg/model/syncmanager"
	"github.com/iotaledger/hornet/v2/pkg/model/utxo"
	"github.com/iotaledger/hornet/v2/pkg/pow"
	"github.com/iotaledger/hornet/v2/pkg/protocol"
	"github.com/iotaledger/hornet/v2/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

// TestEnvironment holds the state of the test environment.
type TestEnvironment struct {
	// TestInterface is the common interface for tests and benchmarks.
	TestInterface testing.TB

	// Milestones are the created milestones by the coordinator during the test.
	Milestones storage.CachedMilestones

	// cachedBlocks is used to cleanup all blocks at the end of a test.
	cachedBlocks storage.CachedBlocks

	// showConfirmationGraphs is set if pictures of the confirmation graph should be externally opened during the test.
	showConfirmationGraphs bool

	// PoWHandler holds the PoWHandler instance.
	PoWHandler *pow.Handler

	// protoParams are the protocol parameters of the network.
	protoParams *iotago.ProtocolParameters

	// belowMaxDepth is the maximum allowed delta
	// value between OCRI of a given block in relation to the current CMI before it gets lazy.
	belowMaxDepth syncmanager.MilestoneIndexDelta

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

	// protocolManager is used to determine the current protocol parameters in this test.
	protocolManager *protocol.Manager

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
type OnLedgerUpdatedFunc func(index iotago.MilestoneIndex, newOutputs utxo.Outputs, newSpents utxo.Spents)

// SetupTestEnvironment initializes a clean database with initial snapshot,
// configures a coordinator with a clean state, bootstraps the network and issues the first "numberOfMilestones" milestones.
func SetupTestEnvironment(testInterface testing.TB, genesisAddress *iotago.Ed25519Address, numberOfMilestones int, protocolVersion byte, belowMaxDepth uint8, targetScore uint32, showConfirmationGraphs bool) *TestEnvironment {

	te := &TestEnvironment{
		TestInterface:          testInterface,
		Milestones:             make(storage.CachedMilestones, 0),
		cachedBlocks:           make(storage.CachedBlocks, 0),
		showConfirmationGraphs: showConfirmationGraphs,
		PoWHandler:             pow.New(5 * time.Second),
		protoParams: &iotago.ProtocolParameters{
			Version:       protocolVersion,
			NetworkName:   "alphapnet1",
			Bech32HRP:     iotago.PrefixTestnet,
			MinPoWScore:   targetScore,
			BelowMaxDepth: belowMaxDepth,
			RentStructure: iotago.RentStructure{
				VByteCost:    500,
				VBFactorData: 1,
				VBFactorKey:  10,
			},
			TokenSupply: 2_779_530_283_277_761,
		},
		belowMaxDepth: syncmanager.MilestoneIndexDelta(belowMaxDepth),
		serverMetrics: &metrics.ServerMetrics{},
	}

	cfg := configuration.New()
	err := cfg.Set("logger.disableStacktrace", true)
	require.NoError(testInterface, err)

	// no need to check the error, since the global logger could already be initialized
	_ = logger.InitGlobalLogger(cfg)

	cooPrvKey1, err := crypto.ParseEd25519PrivateKeyFromString("651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c")
	require.NoError(te.TestInterface, err)
	cooPrvKey2, err := crypto.ParseEd25519PrivateKeyFromString("0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c")
	require.NoError(te.TestInterface, err)

	tempDir, err := os.MkdirTemp("", fmt.Sprintf("test_%s", te.TestInterface.Name()))
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
	te.storage.SolidEntryPointsAddWithoutLocking(iotago.EmptyBlockID(), 0)

	// Initialize ProtocolManager
	ledgerIndex, err := te.storage.UTXOManager().ReadLedgerIndex()
	require.NoError(te.TestInterface, err)

	protoParamsBytes, err := te.protoParams.Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(te.TestInterface, err)

	err = te.Storage().StoreProtocolParametersMilestoneOption(&iotago.ProtocolParamsMilestoneOpt{
		TargetMilestoneIndex: 0,
		ProtocolVersion:      te.protoParams.Version,
		Params:               protoParamsBytes,
	})
	require.NoError(te.TestInterface, err)

	te.protocolManager, err = protocol.NewManager(te.Storage(), ledgerIndex)
	require.NoError(te.TestInterface, err)

	// Initialize SyncManager
	te.syncManager, err = syncmanager.New(ledgerIndex, te.protocolManager)
	require.NoError(te.TestInterface, err)

	// Initialize MilestoneManager
	te.milestoneManager = milestonemanager.New(te.storage, te.syncManager, keyManager, len(cooPrivateKeys))

	// Initialize UTXO
	output := &iotago.BasicOutput{
		Amount: te.protoParams.TokenSupply,
		Conditions: iotago.UnlockConditions{
			&iotago.AddressUnlockCondition{
				Address: genesisAddress,
			},
		},
	}
	te.GenesisOutput = utxo.CreateOutput(iotago.OutputID{}, iotago.EmptyBlockID(), 0, 0, output)
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
		_, confStats := te.IssueAndConfirmMilestoneOnTips(iotago.BlockIDs{}, false)
		require.Equal(te.TestInterface, 1, confStats.BlocksReferenced)                  // 1 for previous milestone
		require.Equal(te.TestInterface, 1, confStats.BlocksExcludedWithoutTransactions) // 1 for previous milestone
		require.Equal(te.TestInterface, 0, confStats.BlocksIncludedWithTransactions)
		require.Equal(te.TestInterface, 0, confStats.BlocksExcludedWithConflictingTransactions)
	}

	return te
}

func (te *TestEnvironment) ConfigureUTXOCallbacks(onLedgerUpdatedFunc OnLedgerUpdatedFunc) {
	te.OnLedgerUpdatedFunc = onLedgerUpdatedFunc
}

func (te *TestEnvironment) ProtocolParameters() *iotago.ProtocolParameters {
	return te.protoParams
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

func (te *TestEnvironment) ProtocolManager() *protocol.Manager {
	return te.protocolManager
}

func (te *TestEnvironment) BelowMaxDepth() iotago.MilestoneIndex {
	return te.belowMaxDepth
}

func (te *TestEnvironment) LastMilestonePayload() *iotago.Milestone {
	return te.coo.LastMilestonePayload()
}

func (te *TestEnvironment) LastMilestoneIndex() iotago.MilestoneIndex {
	return te.coo.LastMilestoneIndex()
}

func (te *TestEnvironment) LastMilestoneID() iotago.MilestoneID {
	return te.coo.LastMilestoneID()
}

func (te *TestEnvironment) LastPreviousMilestoneID() iotago.MilestoneID {
	return te.coo.LastPreviousMilestoneID()
}

func (te *TestEnvironment) LastMilestoneBlockID() iotago.BlockID {
	return te.coo.LastMilestoneBlockID()
}

func (te *TestEnvironment) LastMilestoneParents() iotago.BlockIDs {
	return te.coo.LastMilestoneParents()
}

// CleanupTestEnvironment cleans up everything at the end of the test.
func (te *TestEnvironment) CleanupTestEnvironment(removeTempDir bool) {
	te.cachedBlocks.Release(true) // block -1
	te.cachedBlocks = nil

	te.Milestones.Release(true) // milestone -1
	te.Milestones = nil

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

func (te *TestEnvironment) NewTestBlock(index int, parents iotago.BlockIDs) *storage.BlockMetadata {
	block := te.NewBlockBuilder(fmt.Sprintf("%d", index)).Parents(parents).BuildTaggedData().Store()
	cachedBlockMeta := te.Storage().CachedBlockMetadataOrNil(block.StoredBlockID()) // meta +1
	defer cachedBlockMeta.Release(true)                                             // meta -1

	return cachedBlockMeta.Metadata()
}

// BuildTangle builds a tangle structure without a tipselection algorithm, but random tips from the last
// blocks in the last belowMaxDepth milestones.
func (te *TestEnvironment) BuildTangle(initBlocksCount int,
	belowMaxDepth int,
	milestonesCount int,
	minBlocksPerMilestone int,
	maxBlocksPerMilestone int,
	onNewBlock func(cmi iotago.MilestoneIndex, blockMetadata *storage.BlockMetadata),
	milestoneTipSelectFunc func(blocksIDs iotago.BlockIDs, blockIDsPerMilestones []iotago.BlockIDs) iotago.BlockIDs,
	onNewMilestone func(msIndex iotago.MilestoneIndex, blockIDs iotago.BlockIDs, conf *whiteflag.Confirmation, confStats *whiteflag.ConfirmedMilestoneStats)) (blockIDs iotago.BlockIDs, blockIDsPerMilestones []iotago.BlockIDs) {

	blockTotalCount := 0
	blockIDs = iotago.BlockIDs{}
	blockIDsPerMilestones = make([]iotago.BlockIDs, 0)

	getParents := func() iotago.BlockIDs {

		if len(blockIDs) < initBlocksCount {
			// reference the first milestone at the beginning
			return iotago.BlockIDs{te.LastMilestoneBlockID()}
		}

		parents := iotago.BlockIDs{}
		for j := 2; j <= 2+rand.Intn(7); j++ {
			msIndex := rand.Intn(belowMaxDepth)
			if msIndex > len(blockIDsPerMilestones)-1 {
				msIndex = rand.Intn(len(blockIDsPerMilestones))
			}
			milestoneBlocks := blockIDsPerMilestones[len(blockIDsPerMilestones)-1-msIndex]
			if len(milestoneBlocks) == 0 {
				// use the last milestone block id
				parents = append(parents, te.LastMilestoneBlockID())

				continue
			}
			parents = append(parents, milestoneBlocks[rand.Intn(len(milestoneBlocks))])
		}

		return parents.RemoveDupsAndSort()
	}

	// build a tangle
	for msIndex := 2; msIndex < milestonesCount; msIndex++ {
		blockIDsPerMilestones = append(blockIDsPerMilestones, iotago.BlockIDs{})

		cmi := te.SyncManager().ConfirmedMilestoneIndex()

		blocksCount := minBlocksPerMilestone + rand.Intn(maxBlocksPerMilestone-minBlocksPerMilestone)
		for c := 0; c < blocksCount; c++ {
			blockTotalCount++
			blockMeta := te.NewTestBlock(blockTotalCount, getParents())

			blockIDs = append(blockIDs, blockMeta.BlockID())
			blockIDsPerMilestones[len(blockIDsPerMilestones)-1] = append(blockIDsPerMilestones[len(blockIDsPerMilestones)-1], blockMeta.BlockID())

			if onNewBlock != nil {
				onNewBlock(cmi, blockMeta)
			}
		}

		// confirm the new cone
		conf, confStats := te.IssueAndConfirmMilestoneOnTips(milestoneTipSelectFunc(blockIDs, blockIDsPerMilestones), false)
		if onNewMilestone != nil {
			onNewMilestone(conf.MilestoneIndex, blockIDs, conf, confStats)
		}
	}

	return blockIDs, blockIDsPerMilestones
}
