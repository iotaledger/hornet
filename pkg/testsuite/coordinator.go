package testsuite

import (
	"crypto"
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

const (
	cooSeed     = "WMC9IZAXFW9WQHSJDFUROTNVZPSCDJAQJCTPPAIDFKHVOGPONPQUGDEGWNLSEPZYXOPKQKGKDDINIVOCY"
	cooAddress  = "WZZQHXUDONRBBIUBCNGNCULQWMLHW9VWEESGFTMWVDVGDTO9EBFGSQXNYPAAFUOI9WIGALDNTSSGNW9ZC"
	cooSecLevel = consts.SecurityLevelMedium

	mwm             = 1
	merkleHashFunc  = crypto.BLAKE2b_512
	merkleTreeDepth = 10
)

// configureCoordinator configures a new coordinator with clean state for the tests.
// the node is initialized, the network is bootstrapped and the first milestone is confirmed.
func (te *TestEnvironment) configureCoordinator() {

	storeBundleFunc := func(b coordinator.Bundle, isMilestone bool) error {
		var bndl = make(bundle.Bundle, 0)

		// insert it the reverse way
		for i := len(b) - 1; i >= 0; i-- {
			bndl = append(bndl, *b[i])
		}

		ms := te.StoreBundle(bndl, true) // no need to release, since we store all the bundles for later cleanup

		if isMilestone {
			tangle.SetLatestMilestoneIndex(ms.GetBundle().GetMilestoneIndex())
		}

		return nil
	}

	te.coo = coordinator.New(cooSeed, cooSecLevel, merkleTreeDepth, mwm, fmt.Sprintf("%s/coordinator.state", te.tempDir), 10, te.powHandler, storeBundleFunc, merkleHashFunc)
	require.NotNil(te.testState, te.coo)

	err := te.coo.InitMerkleTree(fmt.Sprintf("%s/pkg/testsuite/assets/coordinator.tree", searchProjectRootFolder()), cooAddress)
	require.NoError(te.testState, err)

	te.coo.InitState(true, 0)

	// save snapshot info
	tangle.SetSnapshotMilestone(hornet.HashFromAddressTrytes(cooAddress), hornet.NullHashBytes, 0, 0, 0, time.Now().Unix(), false)

	// configure Milestones
	tangle.ConfigureMilestones(hornet.HashFromAddressTrytes(cooAddress), int(cooSecLevel), merkleTreeDepth, merkleHashFunc)

	milestoneHash, err := te.coo.Bootstrap()
	require.NoError(te.testState, err)

	te.lastMilestoneHash = milestoneHash

	ms := tangle.GetMilestoneOrNil(1)
	require.NotNil(te.testState, ms)
	defer ms.Release(true)

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// all releases are forced since the cone is confirmed and not needed anymore

		// release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	conf, err := whiteflag.ConfirmMilestone(cachedTxMetas, ms.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(te.testState, err)
	require.Equal(te.testState, 3, conf.TxsConfirmed)
}

// IssueAndConfirmMilestoneOnTip creates a milestone on top of a given tip.
func (te *TestEnvironment) IssueAndConfirmMilestoneOnTip(tip hornet.Hash, createConfirmationGraph bool) *whiteflag.ConfirmedMilestoneStats {

	currentIndex := tangle.GetSolidMilestoneIndex()
	te.VerifyLMI(currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)
	milestoneHash, noncriticalErr, criticalErr := te.coo.IssueMilestone(te.lastMilestoneHash, tip)
	require.NoError(te.testState, noncriticalErr)
	require.NoError(te.testState, criticalErr)
	te.lastMilestoneHash = milestoneHash

	te.VerifyLMI(currentIndex + 1)

	milestoneIndex := currentIndex + 1
	ms := tangle.GetMilestoneOrNil(milestoneIndex)
	require.NotNil(te.testState, ms)

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	var wfConf *whiteflag.Confirmation
	confStats, err := whiteflag.ConfirmMilestone(cachedTxMetas, ms.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {}, func(confirmation *whiteflag.Confirmation) {
		wfConf = confirmation
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(te.testState, err)

	require.Equal(te.testState, currentIndex+1, confStats.Index)
	te.VerifyLSMI(confStats.Index)

	te.cachedBundles = append(te.cachedBundles, ms)

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
