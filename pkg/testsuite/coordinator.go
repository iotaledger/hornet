package testsuite

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/dag"
	"github.com/iotaledger/hornet/pkg/model/storage"
	"github.com/iotaledger/hornet/pkg/model/utxo"
	"github.com/iotaledger/hornet/pkg/pow"
	"github.com/iotaledger/hornet/pkg/testsuite/utils"
	"github.com/iotaledger/hornet/pkg/whiteflag"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/keymanager"
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator(cooPrivateKeys []ed25519.PrivateKey, keyManager *keymanager.KeyManager) {

	te.coo = &MockCoo{
		te:                    te,
		cooPrivateKeys:        cooPrivateKeys,
		genesisMilestoneIndex: 0,
		keyManager:            keyManager,
	}

	// save snapshot info
	err := te.storage.SetInitialSnapshotInfo(0, 0, 0, 0, time.Now())
	require.NoError(te.TestInterface, err)

	te.coo.bootstrap()

	blocksMemcache := storage.NewBlocksMemcache(te.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(te.storage.CachedBlockMetadata)
	memcachedParentsTraverserStorage := dag.NewMemcachedParentsTraverserStorage(te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)

		// release all blocks at the end
		blocksMemcache.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		te.UTXOManager(),
		memcachedParentsTraverserStorage,
		blocksMemcache.CachedBlock,
		te.protoParams,
		te.coo.genesisMilestoneIndex,
		te.LastMilestonePayload(),
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		te.serverMetrics,
		// Hint: Ledger is write locked
		nil,
		// Hint: Ledger is write locked
		func(confirmation *whiteflag.Confirmation) {
			err = te.syncManager.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
			require.NoError(te.TestInterface, err)
		},
		// Hint: Ledger is not locked
		nil,
		// Hint: Ledger is not locked
		nil,
		// Hint: Ledger is not locked
		nil,
	)
	require.NoError(te.TestInterface, err)
	require.Equal(te.TestInterface, 0, confirmedMilestoneStats.BlocksReferenced)
}

func (te *TestEnvironment) milestoneIDForIndex(msIndex iotago.MilestoneIndex) iotago.MilestoneID {
	cachedMilestone := te.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	require.NotNil(te.TestInterface, cachedMilestone)
	defer cachedMilestone.Release(true) // milestone -1
	return cachedMilestone.Milestone().MilestoneID()
}

func (te *TestEnvironment) milestoneForIndex(msIndex iotago.MilestoneIndex) *storage.Milestone {
	ms := te.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	require.NotNil(te.TestInterface, ms)
	defer ms.Release(true) // milestone -1
	return ms.Milestone()
}

func (te *TestEnvironment) ReattachBlock(blockID iotago.BlockID, parents ...iotago.BlockID) iotago.BlockID {
	block := te.storage.CachedBlockOrNil(blockID)
	require.NotNil(te.TestInterface, block)
	defer block.Release(true)

	iotaBlock := block.Block().Block()

	newParents := iotaBlock.Parents
	if len(parents) > 0 {
		newParents = iotago.BlockIDs(parents).RemoveDupsAndSort()
	}

	newBlock := &iotago.Block{
		ProtocolVersion: iotaBlock.ProtocolVersion,
		Parents:         newParents,
		Payload:         iotaBlock.Payload,
		Nonce:           iotaBlock.Nonce,
	}

	_, err := te.PoWHandler.DoPoW(context.Background(), newBlock, 1)
	require.NoError(te.TestInterface, err)

	// We brute-force a new nonce until it is different than the original one (this is important when reattaching valid milestones)
	powMinScore := te.protoParams.MinPoWScore
	for newBlock.Nonce == iotaBlock.Nonce {
		powMinScore += 10.0
		// Use a higher PowScore on every iteration to force a different nonce
		handler := pow.New(powMinScore, 5*time.Minute)
		_, err := handler.DoPoW(context.Background(), newBlock, 1)
		require.NoError(te.TestInterface, err)
	}

	storedBlock, err := storage.NewBlock(newBlock, serializer.DeSeriModePerformValidation, te.protoParams)
	require.NoError(te.TestInterface, err)

	cachedBlock := te.StoreBlock(storedBlock)
	require.NotNil(te.TestInterface, cachedBlock)

	return storedBlock.BlockID()
}

func (te *TestEnvironment) PerformWhiteFlagConfirmation(milestonePayload *iotago.Milestone) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats, error) {

	blocksMemcache := storage.NewBlocksMemcache(te.storage.CachedBlock)
	metadataMemcache := storage.NewMetadataMemcache(te.storage.CachedBlockMetadata)
	memcachedParentsTraverserStorage := dag.NewMemcachedParentsTraverserStorage(te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)

		// release all blocks at the end
		blocksMemcache.Cleanup(true)

		// Release all block metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	var wfConf *whiteflag.Confirmation
	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		te.UTXOManager(),
		memcachedParentsTraverserStorage,
		blocksMemcache.CachedBlock,
		te.protoParams,
		te.coo.genesisMilestoneIndex,
		milestonePayload,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		te.serverMetrics,
		// Hint: Ledger is write locked
		nil,
		// Hint: Ledger is write locked
		func(confirmation *whiteflag.Confirmation) {
			wfConf = confirmation
			err := te.syncManager.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
			require.NoError(te.TestInterface, err)
		},
		// Hint: Ledger is not locked
		nil,
		// Hint: Ledger is not locked
		func(index iotago.MilestoneIndex, newOutputs utxo.Outputs, newSpents utxo.Spents) {
			if te.OnLedgerUpdatedFunc != nil {
				te.OnLedgerUpdatedFunc(index, newOutputs, newSpents)
			}
		},
		// Hint: Ledger is not locked
		nil,
	)
	return wfConf, confirmedMilestoneStats, err
}

// ConfirmMilestone confirms the milestone for the given index.
func (te *TestEnvironment) ConfirmMilestone(ms *storage.Milestone, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	// Verify that we are properly synced and confirming the next milestone
	currentIndex := te.syncManager.LatestMilestoneIndex()
	require.GreaterOrEqual(te.TestInterface, ms.Index(), currentIndex)
	confirmedIndex := te.syncManager.ConfirmedMilestoneIndex()
	require.Equal(te.TestInterface, ms.Index(), confirmedIndex+1)

	wfConf, confirmedMilestoneStats, err := te.PerformWhiteFlagConfirmation(ms.Milestone())
	require.NoError(te.TestInterface, err)

	require.Equal(te.TestInterface, confirmedIndex+1, confirmedMilestoneStats.Index)
	te.VerifyCMI(confirmedMilestoneStats.Index)

	te.AssertTotalSupplyStillValid()

	if createConfirmationGraph {
		dotFileContent := te.generateDotFileFromConfirmation(wfConf)
		if te.showConfirmationGraphs {
			dotFilePath := fmt.Sprintf("%s/%s_%d.png", te.TempDir, te.TestInterface.Name(), confirmedMilestoneStats.Index)
			utils.ShowDotFile(te.TestInterface, dotFileContent, dotFilePath)
		}
	}

	return wfConf, confirmedMilestoneStats
}

// IssueMilestoneOnTips creates a milestone on top of the given tips.
func (te *TestEnvironment) IssueMilestoneOnTips(tips iotago.BlockIDs, addLastMilestoneAsParent bool) (*storage.Milestone, iotago.BlockID, error) {
	return te.coo.issueMilestoneOnTips(tips, addLastMilestoneAsParent)
}

// IssueAndConfirmMilestoneOnTips creates a milestone on top of the given tips and confirms it.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTips(tips iotago.BlockIDs, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := te.syncManager.ConfirmedMilestoneIndex()
	te.VerifyLMI(currentIndex)

	ms, _, err := te.coo.issueMilestoneOnTips(tips, true)
	require.NoError(te.TestInterface, err)
	return te.ConfirmMilestone(ms, createConfirmationGraph)
}

func (te *TestEnvironment) UnspentAliasOutputsInLedger() utxo.Outputs {
	outputs, err := te.UTXOManager().UnspentOutputs()
	require.NoError(te.TestInterface, err)

	var aliasOutputs utxo.Outputs
	for _, output := range outputs {
		switch output.OutputType() {
		case iotago.OutputAlias:
			aliasOutputs = append(aliasOutputs, output)
		}
	}
	return aliasOutputs
}
