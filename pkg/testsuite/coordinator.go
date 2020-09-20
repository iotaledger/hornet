package testsuite

import (
	"crypto"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

const (
	cooSeed    = "WMC9IZAXFW9WQHSJDFUROTNVZPSCDJAQJCTPPAIDFKHVOGPONPQUGDEGWNLSEPZYXOPKQKGKDDINIVOCY"
	cooAddress = "WZZQHXUDONRBBIUBCNGNCULQWMLHW9VWEESGFTMWVDVGDTO9EBFGSQXNYPAAFUOI9WIGALDNTSSGNW9ZC"

	mwm            = 1
	merkleHashFunc = crypto.BLAKE2b_512
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator() {

	storeMessageFunc := func(msg *tangle.Message, isMilestone bool) error {
		cachedMessage := te.StoreMessage(msg, true) // no need to release, since we remember all the messages for later cleanup

		if isMilestone {
			tangle.SetLatestMilestoneIndex(cachedMessage.GetMessage().GetMilestoneIndex())
		}

		return nil
	}

	var err error
	te.coo, err = coordinator.New(cooSeed, mwm, fmt.Sprintf("%s/coordinator.state", te.tempDir), 10, te.powHandler, storeMessageFunc, merkleHashFunc)
	require.NoError(te.testState, err)
	require.NotNil(te.testState, te.coo)

	te.coo.InitState(true, 0)

	// save snapshot info
	tangle.SetSnapshotMilestone(hornet.HashFromAddressTrytes(cooAddress), hornet.NullMessageID, 0, 0, 0, time.Now().Unix())

	// configure Milestones
	tangle.ConfigureMilestones(hornet.HashFromAddressTrytes(cooAddress), int(cooSecLevel), merkleTreeDepth, merkleHashFunc)

	milestoneMessageID, err := te.coo.Bootstrap()
	require.NoError(te.testState, err)

	te.lastMilestoneMessageID = milestoneMessageID

	ms := tangle.GetCachedMilestoneOrNil(1)
	require.NotNil(te.testState, ms)
	defer ms.Release(true)

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// all releases are forced since the cone is confirmed and not needed anymore

		// release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	conf, err := whiteflag.ConfirmMilestone(cachedMsgMetas, ms.GetMilestone().MessageID, func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime uint64) {}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(te.testState, err)
	require.Equal(te.testState, 3, conf.MessagesConfirmed)
}

// IssueAndConfirmMilestoneOnTip creates a milestone on top of a given tip.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTip(tip hornet.Hash, createConfirmationGraph bool) *whiteflag.ConfirmedMilestoneStats {

	currentIndex := tangle.GetSolidMilestoneIndex()
	te.VerifyLMI(currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)
	milestoneMessageID, noncriticalErr, criticalErr := te.coo.IssueMilestone(te.lastMilestoneMessageID, tip)
	require.NoError(te.testState, noncriticalErr)
	require.NoError(te.testState, criticalErr)
	te.lastMilestoneMessageID = milestoneMessageID

	te.VerifyLMI(currentIndex + 1)

	milestoneIndex := currentIndex + 1
	ms := tangle.GetCachedMilestoneOrNil(milestoneIndex)
	require.NotNil(te.testState, ms)

	cachedMsgMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all msg metadata at the end
		for _, cachedMsgMeta := range cachedMsgMetas {
			cachedMsgMeta.Release(true) // meta -1
		}
	}()

	var wfConf *whiteflag.Confirmation
	confStats, err := whiteflag.ConfirmMilestone(cachedMsgMetas, ms.GetMilestone().MessageID, func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime uint64) {}, func(confirmation *whiteflag.Confirmation) {
		wfConf = confirmation
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(te.testState, err)

	require.Equal(te.testState, currentIndex+1, confStats.Index)
	te.VerifyLSMI(confStats.Index)

	te.cachedMessages = append(te.cachedMessages, ms)

	te.AssertTotalSupplyStillValid()

	if createConfirmationGraph {
		dotFileContent := te.generateDotFileFromConfirmation(wfConf)
		if te.showConfirmationGraphs {
			dotFilePath := fmt.Sprintf("%s/%s_%d.png", te.tempDir, te.testState.Name(), confStats.Index)
			utils.ShowDotFile(te.testState, dotFileContent, dotFilePath)
		} else {
			fmt.Println(dotFileContent)
		}
	}

	te.Milestones = append(te.Milestones, ms)

	return confStats
}
