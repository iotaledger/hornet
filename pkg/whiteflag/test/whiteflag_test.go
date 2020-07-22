package test

import (
	"crypto"
	"fmt"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/pow"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/gohornet/hornet/pkg/compressed"
	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/whiteflag"
)

const (
	cooSeed    = "WMC9IZAXFW9WQHSJDFUROTNVZPSCDJAQJCTPPAIDFKHVOGPONPQUGDEGWNLSEPZYXOPKQKGKDDINIVOCY"
	cooAddress = "WZZQHXUDONRBBIUBCNGNCULQWMLHW9VWEESGFTMWVDVGDTO9EBFGSQXNYPAAFUOI9WIGALDNTSSGNW9ZC"

	seed1 = "JBN9ZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWQ"
	seed2 = "JBNAZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWQ"
	seed3 = "JBNBZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWQ"
	seed4 = "DBNBZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWQ"

	mwm             = 1
	merkleHashFunc  = crypto.BLAKE2b_512
	merkleTreeDepth = 10
	secLevel        = consts.SecurityLevelMedium
)

var (
	coo     *coordinator.Coordinator
	nextTip = hornet.NullHashBytes

	// This is just used to clean up at the end of a test
	cachedBundles tangle.CachedBundles
)

func setupTestEnvironment(t *testing.T, store kvstore.KVStore) {

	opts := profile.Profile2GB.Caches
	//opts.Bundles.LeakDetectionOptions.Enabled = true

	tangle.ConfigureStorages(
		store.WithRealm([]byte("tangle")),
		store.WithRealm([]byte("snapshot")),
		store.WithRealm([]byte("spent")),
		opts,
	)
}

func storeTransaction(t *testing.T, tx *transaction.Transaction) *tangle.CachedTransaction {

	txTrits, err := transaction.TransactionToTrits(tx)
	require.NoError(t, err)
	txBytesTruncated := compressed.TruncateTx(trinary.MustTritsToBytes(txTrits))
	hornetTx := hornet.NewTransactionFromTx(tx, txBytesTruncated)
	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()
	//fmt.Printf("Store tx: %s, isTail: %v\n", hornetTx.GetTxHash().Trytes(), hornetTx.IsTail())
	cachedTx, alreadyAdded := tangle.AddTransactionToStorage(hornetTx, latestMilestoneIndex, false, true, true)
	require.NotNil(t, cachedTx)
	require.False(t, alreadyAdded)
	return cachedTx
}

func storeBundle(t *testing.T, bndl bundle.Bundle, expectMilestone bool) *tangle.CachedBundle {

	var hashes hornet.Hashes
	// Store all tx in db
	for i := 0; i < len(bndl); i++ {
		cachedTx := storeTransaction(t, &bndl[i])
		require.NotNil(t, cachedTx)
		hashes = append(hashes, cachedTx.GetTransaction().GetTxHash())
		cachedTx.Release()
	}

	var tailTx hornet.Hash
	// Solidify tx if not a milestone
	for _, hash := range hashes {
		cachedTx := tangle.GetCachedTransactionOrNil(hash)
		require.NotNil(t, cachedTx)
		if cachedTx.GetTransaction().IsTail() {
			tailTx = cachedTx.GetTransaction().GetTxHash()
		}
		if !expectMilestone {
			cachedTx.GetMetadata().SetSolid(true)
		}

		cachedTx.Release()
	}

	// Trigger bundle construction due to solid tail
	if !expectMilestone {
		cachedTx := tangle.GetCachedTransactionOrNil(tailTx)
		require.NotNil(t, cachedTx)
		require.True(t, cachedTx.GetMetadata().IsSolid())
		tangle.OnTailTransactionSolid(cachedTx.Retain())
		cachedTx.Release()
	}

	cachedBundle := tangle.GetCachedBundleOrNil(tailTx)
	require.NotNil(t, cachedBundle)
	require.True(t, cachedBundle.GetBundle().IsValid())
	require.True(t, cachedBundle.GetBundle().ValidStrictSemantics())

	// Verify the bundle is solid if it is no milestone
	if !expectMilestone {
		require.True(t, cachedBundle.GetBundle().IsSolid())
	}

	cachedBundles = append(cachedBundles, cachedBundle)
	return cachedBundle
}

func sendFrom(t *testing.T, tag trinary.Trytes, fromSeed trinary.Trytes, fromIndex uint64, balance uint64, toSeed trinary.Trytes, toIndex uint64, value uint64) []trinary.Trytes {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
		LocalProofOfWorkFunc: powFunc,
	})
	require.NoError(t, err)

	fromAddress, err := address.GenerateAddresses(fromSeed, fromIndex, 2, consts.SecurityLevelMedium, true)
	toAddress, err := address.GenerateAddress(toSeed, toIndex, consts.SecurityLevelMedium, true)

	fmt.Println("Send", value, "from", fromAddress[0], "to", toAddress, "and remaining", balance-value, "to", fromAddress[1])

	require.NoError(t, err)

	transfers := bundle.Transfers{
		{
			Address: toAddress,
			Value:   value,
			Tag:     tag,
		},
	}

	inputs := []api.Input{
		{
			Address:  fromAddress[0],
			Security: consts.SecurityLevelMedium,
			KeyIndex: fromIndex,
			Balance:  balance,
		},
	}

	prepTransferOpts := api.PrepareTransfersOptions{Inputs: inputs, RemainderAddress: &fromAddress[1]}

	trytes, err := iotaAPI.PrepareTransfers(fromSeed, transfers, prepTransferOpts)
	require.NoError(t, err)
	return trytes
}

func attachTo(t *testing.T, trunk hornet.Hash, branch hornet.Hash, trytes []trinary.Trytes) bundle.Bundle {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	powed, err := pow.DoPoW(trunk.Trytes(), branch.Trytes(), trytes, mwm, powFunc)
	require.NoError(t, err)

	txs, err := transaction.AsTransactionObjects(powed, nil)
	require.NoError(t, err)
	return txs
}

func configureCoordinator(t *testing.T) *coordinator.Coordinator {

	storeBundleFunc := func(b coordinator.Bundle) error {
		var bndl = make(bundle.Bundle, 0)
		// Insert it the reverse way
		for i := len(b) - 1; i >= 0; i-- {
			bndl = append(bndl, *b[i])
		}
		ms := storeBundle(t, bndl, true) // No need to release, since we store all the bundles for later cleanup
		tangle.SetLatestMilestoneIndex(ms.GetBundle().GetMilestoneIndex())
		return nil
	}

	tipSelFunc := func() (hornet.Hash, error) {
		return nextTip, nil
	}

	_, powFunc := pow.GetFastestProofOfWorkImpl()

	dir, err := ioutil.TempDir("", "coo.test")
	require.NoError(t, err)
	dirAndFile := fmt.Sprintf("%s/coordinator.state", dir)

	coo = coordinator.New(cooSeed, secLevel, merkleTreeDepth, mwm, dirAndFile, 10, 0, powFunc, tipSelFunc, storeBundleFunc, merkleHashFunc)
	require.NotNil(t, coo)

	err = coo.InitMerkleTree("coordinator.tree", cooAddress)
	require.NoError(t, err)

	coo.InitState(true, 0)

	// Save snapshot info
	tangle.SetSnapshotMilestone(hornet.HashFromAddressTrytes(cooAddress), hornet.NullHashBytes, 0, 0, 0, time.Now().Unix(), false)

	// Configure Milestones
	tangle.ConfigureMilestones(hornet.HashFromAddressTrytes(cooAddress), int(secLevel), merkleTreeDepth, merkleHashFunc)

	return coo
}

func verifyLSMI(t *testing.T, index milestone.Index) {
	lsmi := tangle.GetSolidMilestoneIndex()
	require.Equal(t, index, lsmi)
}

func verifyLMI(t *testing.T, index milestone.Index) {
	lmi := tangle.GetLatestMilestoneIndex()
	require.Equal(t, index, lmi)
}

func issueAndConfirmMilestoneAndVerifyIfLSMI(t *testing.T, index int) (*tangle.CachedBundle, *whiteflag.ConfirmedMilestoneStats) {

	currentIndex := tangle.GetSolidMilestoneIndex()
	verifyLMI(t, currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)
	noncriticalErr, criticalErr := coo.IssueNextCheckpointOrMilestone()
	require.NoError(t, noncriticalErr)
	require.NoError(t, criticalErr)

	verifyLMI(t, currentIndex+1)

	milestoneIndex := milestone.Index(index)
	ms := tangle.GetMilestoneOrNil(milestoneIndex)
	require.NotNil(t, ms)

	conf, err := whiteflag.ConfirmMilestone(ms.Retain(), func(tx *tangle.CachedTransaction, index milestone.Index, confTime int64) {}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex)
	})
	require.NoError(t, err)

	require.Equal(t, currentIndex+1, conf.Index)
	verifyLSMI(t, conf.Index)

	cachedBundles = append(cachedBundles, ms)
	return ms, conf
}

func generateAddress(t *testing.T, seed trinary.Trytes, index uint64) hornet.Hash {
	seedAddress, err := address.GenerateAddress(seed, index, consts.SecurityLevelMedium, false)
	require.NoError(t, err)
	return hornet.HashFromAddressTrytes(seedAddress)
}

func assertAddressBalance(t *testing.T, seed trinary.Trytes, index uint64, balance uint64) {

	address := generateAddress(t, seed, index)
	addrBalance, _, err := tangle.GetBalanceForAddress(address)
	require.NoError(t, err)
	require.Equal(t, balance, addrBalance)
}

func assertTotalSupplyStillValid(t *testing.T) {
	_, _, err := tangle.GetLedgerStateForLSMI(nil)
	require.NoError(t, err)
}

func setupCoordinatorAndIssueInitialMilestones(t *testing.T, initialBalances map[string]uint64, numberOfMilestones int) tangle.CachedBundles {

	balances := initialBalances
	var sum uint64
	for _, value := range balances {
		sum += value
	}
	// Move remaining supply to 999..999
	balances[string(hornet.NullHashBytes)] = consts.TotalSupply - sum

	store := mapdb.NewMapDB()
	setupTestEnvironment(t, store)

	tangle.LoadInitialValuesFromDatabase()

	snapshotIndex := milestone.Index(0)

	tangle.StoreSnapshotBalancesInDatabase(balances, snapshotIndex)
	tangle.StoreLedgerBalancesInDatabase(balances, snapshotIndex)

	assertTotalSupplyStillValid(t)

	// Start up the coordinator
	coo := configureCoordinator(t)
	require.NotNil(t, coo)

	verifyLSMI(t, 0)

	var milestones tangle.CachedBundles
	for i := 1; i <= numberOfMilestones; i++ {
		// 1st milestone
		ms, conf := issueAndConfirmMilestoneAndVerifyIfLSMI(t, i)
		require.Equal(t, 3, conf.TxsConfirmed)
		require.Equal(t, 0, conf.TxsConflicting)
		milestones = append(milestones, ms)
	}

	return milestones
}

func shortenedHash(hash hornet.Hash) string {
	trytes := hash.Trytes()
	return trytes[0:4] + "..." + trytes[77:81]
}

func shortened(bundle *tangle.CachedBundle) string {
	if bundle.GetBundle().IsMilestone() {
		return fmt.Sprintf("%d", bundle.GetBundle().GetMilestoneIndex())
	}
	tail := bundle.GetBundle().GetTail()
	defer tail.Release()
	tag := tail.GetTransaction().Tx.Tag
	return tag[0:strings.IndexByte(tag, '9')]
}

func generateDotFileFromTangle(t *testing.T) string {

	visitedBundles := make(map[string]tangle.CachedBundles)

	bundleTxs := tangle.GetAllBundleTransactionHashes(100)
	for _, hash := range bundleTxs {
		cachedTx := tangle.GetCachedTransactionOrNil(hash)
		if _, visited := visitedBundles[string(cachedTx.GetTransaction().GetBundleHash())]; visited == false {
			bndls := tangle.GetBundlesOfTransactionOrNil(cachedTx.GetTransaction().GetTxHash(), false)
			visitedBundles[string(cachedTx.GetTransaction().GetBundleHash())] = bndls
		}
		cachedTx.Release(true)
	}

	var milestones []string
	var included []string
	var ignored []string
	var conflicting []string

	dotFile := fmt.Sprintf("digraph %s\n{\n", t.Name())
	for _, bndls := range visitedBundles {
		//singleBundle := len(bndls) == 1
		for _, bndl := range bndls {
			shortBundle := shortened(bndl)

			bundleHead := bndl.GetBundle().GetHead()
			//if singleBundle {
			trunk := bndl.GetBundle().GetTrunk(true)
			if tangle.SolidEntryPointsContain(trunk) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\";\n", shortBundle, shortenedHash(trunk))
			} else {
				trunkBundles := tangle.GetBundlesOfTransactionOrNil(trunk, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\";\n", shortBundle, shortened(trunkBundles[0]))
				trunkBundles.Release()
			}

			branch := bndl.GetBundle().GetBranch(true)
			if tangle.SolidEntryPointsContain(branch) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\";\n", shortBundle, shortenedHash(branch))
			} else {
				branchBundles := tangle.GetBundlesOfTransactionOrNil(branch, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\";\n", shortBundle, shortened(branchBundles[0]))
				branchBundles.Release()
			}

			if bndl.GetBundle().IsMilestone() {
				milestones = append(milestones, shortBundle)
			} else if bndl.GetBundle().IsConflicting() {
				conflicting = append(conflicting, shortBundle)
			} else if bndl.GetBundle().IsValueSpam() {
				ignored = append(ignored, shortBundle)
			} else if bndl.GetBundle().IsConfirmed() {
				included = append(included, shortBundle)
			}

			bundleHead.Release()
		}
		bndls.Release()
	}

	for _, milestone := range milestones {
		dotFile += fmt.Sprintf("\"%s\" [shape=Msquare];\n", milestone)
	}
	for _, conf := range conflicting {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=red];\n", conf)
	}
	for _, conf := range ignored {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gray];\n", conf)
	}
	for _, conf := range included {
		dotFile += fmt.Sprintf("\"%s\" [style=filled,color=green];\n", conf)
	}

	dotFile += "}\n"
	return dotFile
}

/// Testcases

func TestWhiteFlagWithMultipleConflicting(t *testing.T) {

	// Fill up the balances
	balances := make(map[string]uint64)
	balances[string(generateAddress(t, seed1, 0))] = 1000

	milestones := setupCoordinatorAndIssueInitialMilestones(t, balances, 3)

	// Issue some transactions
	bundleA := storeBundle(t, attachTo(t, milestones[0].GetBundle().GetTailHash(), milestones[1].GetBundle().GetTailHash(), sendFrom(t, "A", seed1, 0, 1000, seed2, 0, 100)), false)
	require.NotNil(t, bundleA)

	bundleB := storeBundle(t, attachTo(t, bundleA.GetBundle().GetTailHash(), milestones[0].GetBundle().GetTailHash(), sendFrom(t, "B", seed1, 1, 900, seed2, 0, 200)), false)
	require.NotNil(t, bundleB)

	bundleC := storeBundle(t, attachTo(t, milestones[2].GetBundle().GetTailHash(), bundleB.GetBundle().GetTailHash(), sendFrom(t, "C", seed3, 0, 99999, seed2, 0, 10)), false)
	require.NotNil(t, bundleC)

	bundleD := storeBundle(t, attachTo(t, bundleA.GetBundle().GetTailHash(), bundleC.GetBundle().GetTailHash(), sendFrom(t, "D", seed4, 1, 99999, seed2, 0, 150)), false)
	require.NotNil(t, bundleD)

	bundleE := storeBundle(t, attachTo(t, bundleB.GetBundle().GetTailHash(), bundleD.GetBundle().GetTailHash(), sendFrom(t, "E", seed2, 0, 300, seed4, 0, 150)), false)
	require.NotNil(t, bundleD)

	// Confirming milestone
	nextTip = bundleC.GetBundle().GetTailHash()
	_, conf := issueAndConfirmMilestoneAndVerifyIfLSMI(t, int(tangle.GetSolidMilestoneIndex())+1)
	require.Equal(t, 4+4+4+3, conf.TxsConfirmed) // 3 are for the milestone itself
	require.Equal(t, 8, conf.TxsValue)
	require.Equal(t, 4, conf.TxsConflicting)
	require.Equal(t, 3, conf.TxsZeroValue) // The milestone

	// Verify balances
	assertAddressBalance(t, seed1, 0, 0)
	assertAddressBalance(t, seed1, 1, 0)
	assertAddressBalance(t, seed1, 2, 700)
	assertAddressBalance(t, seed2, 0, 300)
	assertAddressBalance(t, seed2, 1, 0)
	assertAddressBalance(t, seed3, 0, 0)
	assertAddressBalance(t, seed3, 1, 0)
	assertAddressBalance(t, seed4, 0, 0)
	assertAddressBalance(t, seed4, 1, 0)

	assertTotalSupplyStillValid(t)
	fmt.Print(generateDotFileFromTangle(t))

	// Confirming milestone
	nextTip = bundleE.GetBundle().GetTailHash()
	_, conf = issueAndConfirmMilestoneAndVerifyIfLSMI(t, int(tangle.GetSolidMilestoneIndex())+1)
	require.Equal(t, 4+4+3, conf.TxsConfirmed) // 3 are for the milestone itself
	require.Equal(t, 4, conf.TxsValue)
	require.Equal(t, 4, conf.TxsConflicting)
	require.Equal(t, 3, conf.TxsZeroValue) // The milestone

	// Verify balances
	assertAddressBalance(t, seed1, 0, 0)
	assertAddressBalance(t, seed1, 1, 0)
	assertAddressBalance(t, seed1, 2, 700)
	assertAddressBalance(t, seed2, 0, 0)
	assertAddressBalance(t, seed2, 1, 150)
	assertAddressBalance(t, seed3, 0, 0)
	assertAddressBalance(t, seed3, 1, 0)
	assertAddressBalance(t, seed4, 0, 150)
	assertAddressBalance(t, seed4, 1, 0)

	assertTotalSupplyStillValid(t)

	fmt.Print(generateDotFileFromTangle(t))

	// Clean up all the bundles we created
	cachedBundles.Release()

	// This should not hang, i.e. all objects should be released
	tangle.ShutdownStorages()
}
