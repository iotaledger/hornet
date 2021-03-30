package testsuite

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator(cooPrivateKeys []ed25519.PrivateKey, keyManager *keymanager.KeyManager) {

	storeMessageFunc := func(msg *storage.Message, msIndex ...milestone.Index) error {
		cachedMessage := te.StoreMessage(msg) // no need to release, since we remember all the messages for later cleanup

		ms := cachedMessage.GetMessage().GetMilestone()
		if ms != nil {
			te.storage.SetLatestMilestoneIndex(milestone.Index(ms.Index))
		}

		return nil
	}

	inMemoryEd25519MilestoneSignerProvider := coordinator.NewInMemoryEd25519MilestoneSignerProvider(cooPrivateKeys, keyManager, len(cooPrivateKeys))

	coo, err := coordinator.New(
		te.storage,
		te.networkID,
		inMemoryEd25519MilestoneSignerProvider,
		nil,
		nil,
		te.PowHandler,
		storeMessageFunc,
		coordinator.WithStateFilePath(fmt.Sprintf("%s/coordinator.state", te.tempDir)),
		coordinator.WithMilestoneInterval(time.Duration(10)*time.Second),
	)
	require.NoError(te.TestState, err)
	require.NotNil(te.TestState, coo)
	te.coo = coo

	te.coo.InitState(true, 0)

	// save snapshot info
	te.storage.SetSnapshotMilestone(te.networkID, 0, 0, 0, time.Now())

	milestoneMessageID, err := te.coo.Bootstrap()
	require.NoError(te.TestState, err)

	te.lastMilestoneMessageID = milestoneMessageID

	ms := te.storage.GetCachedMilestoneOrNil(1)
	require.NotNil(te.TestState, ms)

	te.Milestones = append(te.Milestones, ms)

	messagesMemcache := storage.NewMessagesMemcache(te.storage)
	metadataMemcache := storage.NewMetadataMemcache(te.storage)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(te.storage, te.serverMetrics, messagesMemcache, metadataMemcache, ms.GetMilestone().MessageID,
		func(txMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {},
		func(confirmation *whiteflag.Confirmation) {
			te.storage.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
		},
		func(output *utxo.Output) {},
		func(spent *utxo.Spent) {},
		nil,
	)
	require.NoError(te.TestState, err)
	require.Equal(te.TestState, 1, confirmedMilestoneStats.MessagesReferenced)
}

// IssueAndConfirmMilestoneOnTip creates a milestone on top of a given tip.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTip(tip hornet.MessageID, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := te.storage.GetConfirmedMilestoneIndex()
	te.VerifyLMI(currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)

	milestoneMessageID, err := te.coo.IssueMilestone(hornet.MessageIDs{te.lastMilestoneMessageID, tip})
	require.NoError(te.TestState, err)
	te.lastMilestoneMessageID = milestoneMessageID

	te.VerifyLMI(currentIndex + 1)

	milestoneIndex := currentIndex + 1
	ms := te.storage.GetCachedMilestoneOrNil(milestoneIndex)
	require.NotNil(te.TestState, ms)

	messagesMemcache := storage.NewMessagesMemcache(te.storage)
	metadataMemcache := storage.NewMetadataMemcache(te.storage)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore

		// release all messages at the end
		messagesMemcache.Cleanup(true)

		// Release all message metadata at the end
		metadataMemcache.Cleanup(true)
	}()

	var wfConf *whiteflag.Confirmation
	confirmedMilestoneStats, _, err := whiteflag.ConfirmMilestone(te.storage, te.serverMetrics, messagesMemcache, metadataMemcache, ms.GetMilestone().MessageID,
		func(txMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {},
		func(confirmation *whiteflag.Confirmation) {
			wfConf = confirmation
			te.storage.SetConfirmedMilestoneIndex(confirmation.MilestoneIndex, true)
		},
		func(output *utxo.Output) {},
		func(spent *utxo.Spent) {},
		nil,
	)
	require.NoError(te.TestState, err)

	require.Equal(te.TestState, currentIndex+1, confirmedMilestoneStats.Index)
	te.VerifyCMI(confirmedMilestoneStats.Index)

	te.AssertTotalSupplyStillValid()

	if createConfirmationGraph {
		dotFileContent := te.generateDotFileFromConfirmation(wfConf)
		if te.showConfirmationGraphs {
			dotFilePath := fmt.Sprintf("%s/%s_%d.png", te.tempDir, te.TestState.Name(), confirmedMilestoneStats.Index)
			utils.ShowDotFile(te.TestState, dotFileContent, dotFilePath)
		} else {
			fmt.Println(dotFileContent)
		}
	}

	te.Milestones = append(te.Milestones, ms)

	return wfConf, confirmedMilestoneStats
}
