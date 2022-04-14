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
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/gohornet/inx-coordinator/pkg/coordinator"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator(cooPrivateKeys []ed25519.PrivateKey, keyManager *keymanager.KeyManager) {

	storeMessageFunc := func(message *iotago.Message, _ ...milestone.Index) error {
		msg, err := storage.NewMessage(message, serializer.DeSeriModeNoValidation, iotago.ZeroRentParas) // no need to validate bytes, they come pre-validated from the coo
		if err != nil {
			return err
		}
		cachedMsg := te.StoreMessage(msg) // message +1, no need to release, since we remember all the messages for later cleanup

		ms := cachedMsg.Message().Milestone()
		if ms != nil {
			te.syncManager.SetLatestMilestoneIndex(milestone.Index(ms.Index))
		}

		return nil
	}

	inMemoryEd25519MilestoneSignerProvider := coordinator.NewInMemoryEd25519MilestoneSignerProvider(cooPrivateKeys, keyManager, len(cooPrivateKeys))

	computeWhiteFlag := func(ctx context.Context, index milestone.Index, timestamp uint64, parents hornet.MessageIDs, lastMilestoneID iotago.MilestoneID) (*coordinator.MilestoneMerkleProof, error) {
		messagesMemcache := storage.NewMessagesMemcache(te.storage.CachedMessage)
		metadataMemcache := storage.NewMetadataMemcache(te.storage.CachedMessageMetadata)
		memcachedTraverserStorage := dag.NewMemcachedTraverserStorage(te.storage, metadataMemcache)

		defer func() {
			// all releases are forced since the cone is referenced and not needed anymore
			memcachedTraverserStorage.Cleanup(true)

			// release all messages at the end
			messagesMemcache.Cleanup(true)

			// Release all message metadata at the end
			metadataMemcache.Cleanup(true)
		}()

		parentsTraverser := dag.NewParentsTraverser(memcachedTraverserStorage)

		// compute merkle tree root
		mutations, err := whiteflag.ComputeWhiteFlagMutations(ctx, te.UTXOManager(), parentsTraverser, messagesMemcache.CachedMessage, te.NetworkID(), index, timestamp, parents, lastMilestoneID, whiteflag.DefaultWhiteFlagTraversalCondition)
		if err != nil {
			return nil, err
		}
		merkleTreeHash := &coordinator.MilestoneMerkleProof{
			PastConeMerkleProof:  &coordinator.MerkleTreeHash{},
			InclusionMerkleProof: &coordinator.MerkleTreeHash{},
		}
		copy(merkleTreeHash.PastConeMerkleProof[:], mutations.PastConeMerkleProof[:])
		copy(merkleTreeHash.InclusionMerkleProof[:], mutations.InclusionMerkleProof[:])
		return merkleTreeHash, nil
	}

	nodeSync := func() bool {
		return true
	}

	coo, err := coordinator.New(
		computeWhiteFlag,
		nodeSync,
		te.networkID,
		DeSerializationParameters,
		inMemoryEd25519MilestoneSignerProvider,
		nil,
		nil,
		te.PoWHandler,
		storeMessageFunc,
		coordinator.WithStateFilePath(fmt.Sprintf("%s/coordinator.state", te.TempDir)),
		coordinator.WithMilestoneInterval(time.Duration(10)*time.Second),
	)
	require.NoError(te.TestInterface, err)
	require.NotNil(te.TestInterface, coo)
	te.coo = coo

	err = te.coo.InitState(true, 0, &coordinator.LatestMilestone{
		Index:     0,
		Timestamp: 0,
		MessageID: hornet.NullMessageID(),
	})
	require.NoError(te.TestInterface, err)

	// save snapshot info
	err = te.storage.SetSnapshotMilestone(te.networkID, 0, 0, 0, time.Now())
	require.NoError(te.TestInterface, err)

	milestoneMessageID, err := te.coo.Bootstrap()
	require.NoError(te.TestInterface, err)

	te.LastMilestoneMessageID = milestoneMessageID

	cachedMilestone := te.storage.CachedMilestoneOrNil(1) // milestone +1
	require.NotNil(te.TestInterface, cachedMilestone)

	te.Milestones = append(te.Milestones, cachedMilestone)

	messagesMemcache := storage.NewMessagesMemcache(te.storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(te.storage.CachedMessageMetadata)
	memcachedParentsTraverserStorage := dag.NewMemcachedParentsTraverserStorage(te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		te.UTXOManager(),
		memcachedParentsTraverserStorage,
		messagesMemcache.CachedMessage,
		te.networkID,
		cachedMilestone.Milestone().MessageID,
		iotago.MilestoneID{}, // first milestone does not have a last milestone ID
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckMessageReferencedFunc,
		whiteflag.DefaultSetMessageReferencedFunc,
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
	require.Equal(te.TestInterface, 1, confirmedMilestoneStats.MessagesReferenced)
}

func (te *TestEnvironment) milestoneIDForIndex(msIndex milestone.Index) iotago.MilestoneID {
	msgMilestone := te.storage.MilestoneCachedMessageOrNil(msIndex)
	require.NotNil(te.TestInterface, msgMilestone)
	defer msgMilestone.Release(true)

	milestoneID, err := msgMilestone.Message().Milestone().ID()
	require.NoError(te.TestInterface, err)
	return *milestoneID
}

func (te *TestEnvironment) milestoneForIndex(msIndex milestone.Index) *storage.Milestone {
	ms := te.storage.CachedMilestoneOrNil(msIndex)
	require.NotNil(te.TestInterface, ms)
	defer ms.Release(true)
	return ms.Milestone()
}

// IssueMilestoneOnTips creates a milestone on top of the given tips.
func (te *TestEnvironment) IssueMilestoneOnTips(tips hornet.MessageIDs, addLastMilestoneAsParent bool) milestone.Index {

	currentIndex := te.syncManager.LatestMilestoneIndex()
	te.VerifyLMI(currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)

	if addLastMilestoneAsParent {
		tips = append(tips, te.LastMilestoneMessageID)
	}

	milestoneMessageID, err := te.coo.IssueMilestone(tips)
	require.NoError(te.TestInterface, err)
	te.LastMilestoneMessageID = milestoneMessageID

	te.VerifyLMI(currentIndex + 1)

	milestoneIndex := currentIndex + 1
	cachedMilestone := te.storage.CachedMilestoneOrNil(milestoneIndex) // milestone +1
	require.NotNil(te.TestInterface, cachedMilestone)

	te.Milestones = append(te.Milestones, cachedMilestone)

	return cachedMilestone.Milestone().Index
}

func (te *TestEnvironment) PerformWhiteFlagConfirmation(msIndex milestone.Index) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats, error) {

	milestoneForIndex := te.milestoneForIndex(msIndex)

	messagesMemcache := storage.NewMessagesMemcache(te.storage.CachedMessage)
	metadataMemcache := storage.NewMetadataMemcache(te.storage.CachedMessageMetadata)
	memcachedParentsTraverserStorage := dag.NewMemcachedParentsTraverserStorage(te.storage, metadataMemcache)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore
		memcachedParentsTraverserStorage.Cleanup(true)

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	var wfConf *whiteflag.Confirmation
	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(
		te.UTXOManager(),
		memcachedParentsTraverserStorage,
		messagesMemcache.CachedMessage,
		te.networkID,
		milestoneForIndex.MessageID,
		te.milestoneIDForIndex(msIndex-1),
		whiteflag.DefaultWhiteFlagTraversalCondition,
		whiteflag.DefaultCheckMessageReferencedFunc,
		whiteflag.DefaultSetMessageReferencedFunc,
		te.serverMetrics,
		nil,
		func(confirmation *whiteflag.Confirmation) {
			wfConf = confirmation
			err := te.syncManager.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
			require.NoError(te.TestInterface, err)
			if te.OnMilestoneConfirmed != nil {
				te.OnMilestoneConfirmed(confirmation)
			}
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
func (te *TestEnvironment) ConfirmMilestone(msIndex milestone.Index, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	// Verify that we are properly synced and confirming the next milestone
	currentIndex := te.syncManager.LatestMilestoneIndex()
	require.GreaterOrEqual(te.TestInterface, msIndex, currentIndex)
	confirmedIndex := te.syncManager.ConfirmedMilestoneIndex()
	require.Equal(te.TestInterface, msIndex, confirmedIndex+1)

	wfConf, confirmedMilestoneStats, err := te.PerformWhiteFlagConfirmation(msIndex)
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

// IssueAndConfirmMilestoneOnTips creates a milestone on top of the given tips and confirms it.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTips(tips hornet.MessageIDs, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := te.syncManager.ConfirmedMilestoneIndex()
	te.VerifyLMI(currentIndex)

	msIndex := te.IssueMilestoneOnTips(tips, true)
	return te.ConfirmMilestone(msIndex, createConfirmationGraph)
}
