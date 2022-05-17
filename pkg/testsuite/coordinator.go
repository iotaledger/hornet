package testsuite

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/dag"
	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator(cooPrivateKeys []ed25519.PrivateKey, keyManager *keymanager.KeyManager) {

	te.coo = &MockCoo{
		te:             te,
		cooPrivateKeys: cooPrivateKeys,
		keyManager:     keyManager,
	}

	// save snapshot info
	err := te.storage.SetSnapshotMilestone(te.protoParas.NetworkID(), 0, 0, 0, time.Now())
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
		te.protoParas,
		te.LastMilestonePayload(),
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		te.serverMetrics,
		nil,
		func(confirmation *whiteflag.Confirmation) {
			err = te.syncManager.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
			require.NoError(te.TestInterface, err)
		},
		nil,
		nil,
		nil,
	)
	require.NoError(te.TestInterface, err)
	require.Equal(te.TestInterface, 0, confirmedMilestoneStats.BlocksReferenced)
}

func (te *TestEnvironment) milestoneIDForIndex(msIndex milestone.Index) iotago.MilestoneID {
	cachedMilestone := te.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	require.NotNil(te.TestInterface, cachedMilestone)
	defer cachedMilestone.Release(true) // milestone -1
	return cachedMilestone.Milestone().MilestoneID()
}

func (te *TestEnvironment) milestoneForIndex(msIndex milestone.Index) *storage.Milestone {
	ms := te.storage.CachedMilestoneByIndexOrNil(msIndex) // milestone +1
	require.NotNil(te.TestInterface, ms)
	defer ms.Release(true) // milestone -1
	return ms.Milestone()
}

func (te *TestEnvironment) ReattachBlock(blockID hornet.BlockID, parents ...hornet.BlockID) hornet.BlockID {
	block := te.storage.CachedBlockOrNil(blockID)
	require.NotNil(te.TestInterface, block)
	defer block.Release(true)

	iotaBlock := block.Block().Block()

	newParents := iotaBlock.Parents
	if len(parents) > 0 {
		newParents = hornet.BlockIDs(parents).RemoveDupsAndSortByLexicalOrder().ToSliceOfArrays()
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
	powMinScore := te.protoParas.MinPoWScore
	for newBlock.Nonce == iotaBlock.Nonce {
		powMinScore += 10.0
		// Use a higher PowScore on every iteration to force a different nonce
		handler := pow.New(powMinScore, 5*time.Minute)
		_, err := handler.DoPoW(context.Background(), newBlock, 1)
		require.NoError(te.TestInterface, err)
	}

	storedBlock, err := storage.NewBlock(newBlock, serializer.DeSeriModePerformValidation, te.protoParas)
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
		te.protoParas,
		milestonePayload,
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckBlockReferencedFunc,
		whiteflag.DefaultSetBlockReferencedFunc,
		te.serverMetrics,
		nil,
		func(confirmation *whiteflag.Confirmation) {
			wfConf = confirmation
			err := te.syncManager.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
			require.NoError(te.TestInterface, err)
		},
		func(index milestone.Index, newOutputs utxo.Outputs, newSpents utxo.Spents) {
			if te.OnLedgerUpdatedFunc != nil {
				te.OnLedgerUpdatedFunc(index, newOutputs, newSpents)
			}
		},
		nil,
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
		} else {
			fmt.Println(dotFileContent)
		}
	}

	return wfConf, confirmedMilestoneStats
}

// IssueMilestoneOnTips creates a milestone on top of the given tips.
func (te *TestEnvironment) IssueMilestoneOnTips(tips hornet.BlockIDs, addLastMilestoneAsParent bool) (*storage.Milestone, hornet.BlockID, error) {
	return te.coo.issueMilestoneOnTips(tips, addLastMilestoneAsParent)
}

// IssueAndConfirmMilestoneOnTips creates a milestone on top of the given tips and confirms it.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTips(tips hornet.BlockIDs, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := te.syncManager.ConfirmedMilestoneIndex()
	te.VerifyLMI(currentIndex)

	milestone, _, err := te.coo.issueMilestoneOnTips(tips, true)
	require.NoError(te.TestInterface, err)
	return te.ConfirmMilestone(milestone, createConfirmationGraph)
}
