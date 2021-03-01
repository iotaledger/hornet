package testsuite

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/v2/ed25519"

	"github.com/gohornet/hornet/pkg/keymanager"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator(cooPrivateKeys []ed25519.PrivateKey) {

	storeMessageFunc := func(msg *storage.Message, msIndex ...milestone.Index) error {
		cachedMessage := te.StoreMessage(msg) // no need to release, since we remember all the messages for later cleanup

		ms := cachedMessage.GetMessage().GetMilestone()
		if ms != nil {
			te.storage.SetLatestMilestoneIndex(milestone.Index(ms.Index))
		}

		return nil
	}

	keyManager := keymanager.New()
	for _, key := range cooPrivateKeys {
		keyManager.AddKeyRange(key.Public().(ed25519.PublicKey), 0, 0)
	}

	inMemoryEd25519MilestoneSignerProvider := coordinator.NewInMemoryEd25519MilestoneSignerProvider(cooPrivateKeys, keyManager, len(cooPrivateKeys))

	coo, err := coordinator.New(te.storage, te.networkID, inMemoryEd25519MilestoneSignerProvider, fmt.Sprintf("%s/coordinator.state", te.tempDir), 10, 1, te.PowHandler, nil, nil, storeMessageFunc)
	require.NoError(te.TestState, err)
	require.NotNil(te.TestState, coo)
	te.coo = coo

	te.coo.InitState(true, 0)

	// save snapshot info
	te.storage.SetSnapshotMilestone(te.networkID, 0, 0, 0, time.Now())

	// configure Milestones
	te.storage.ConfigureMilestones(keyManager, len(cooPrivateKeys))

	milestoneMessageID, err := te.coo.Bootstrap()
	require.NoError(te.TestState, err)

	te.lastMilestoneMessageID = milestoneMessageID

	ms := te.storage.GetCachedMilestoneOrNil(1)
	require.NotNil(te.TestState, ms)

	te.Milestones = append(te.Milestones, ms)

	cachedMsgMetas := make(map[string]*storage.CachedMetadata)

	defer func() {
		// all releases are forced since the cone is referenced and not needed anymore

		// release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	conf, err := whiteflag.ConfirmMilestone(te.storage, te.serverMetrics, cachedMsgMetas, ms.GetMilestone().MessageID,
		func(txMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {},
		func(confirmation *whiteflag.Confirmation) {
			te.storage.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
		},
		func(output *utxo.Output) {},
		func(spent *utxo.Spent) {},
		nil,
	)
	require.NoError(te.TestState, err)
	require.Equal(te.TestState, 1, conf.MessagesReferenced)
}

// IssueAndConfirmMilestoneOnTip creates a milestone on top of a given tip.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTip(tip hornet.MessageID, createConfirmationGraph bool) (*whiteflag.Confirmation, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := te.storage.GetSolidMilestoneIndex()
	te.VerifyLMI(currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)

	milestoneMessageID, noncriticalErr, criticalErr := te.coo.IssueMilestone(hornet.MessageIDs{te.lastMilestoneMessageID, tip})
	require.NoError(te.TestState, noncriticalErr)
	require.NoError(te.TestState, criticalErr)
	te.lastMilestoneMessageID = milestoneMessageID

	te.VerifyLMI(currentIndex + 1)

	milestoneIndex := currentIndex + 1
	ms := te.storage.GetCachedMilestoneOrNil(milestoneIndex)
	require.NotNil(te.TestState, ms)

	cachedMsgMetas := make(map[string]*storage.CachedMetadata)

	defer func() {
		// All releases are forced since the cone is referenced and not needed anymore

		// Release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	var wfConf *whiteflag.Confirmation
	confStats, err := whiteflag.ConfirmMilestone(te.storage, te.serverMetrics, cachedMsgMetas, ms.GetMilestone().MessageID,
		func(txMeta *storage.CachedMetadata, index milestone.Index, confTime uint64) {},
		func(confirmation *whiteflag.Confirmation) {
			wfConf = confirmation
			te.storage.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
		},
		func(output *utxo.Output) {},
		func(spent *utxo.Spent) {},
		nil,
	)
	require.NoError(te.TestState, err)

	require.Equal(te.TestState, currentIndex+1, confStats.Index)
	te.VerifyLSMI(confStats.Index)

	te.AssertTotalSupplyStillValid()

	if createConfirmationGraph {
		dotFileContent := te.generateDotFileFromConfirmation(wfConf)
		if te.showConfirmationGraphs {
			dotFilePath := fmt.Sprintf("%s/%s_%d.png", te.tempDir, te.TestState.Name(), confStats.Index)
			utils.ShowDotFile(te.TestState, dotFileContent, dotFilePath)
		} else {
			fmt.Println(dotFileContent)
		}
	}

	te.Milestones = append(te.Milestones, ms)

	return wfConf, confStats
}
