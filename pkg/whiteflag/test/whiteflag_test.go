package test

import (
	"bytes"
	"crypto"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"

	_ "golang.org/x/crypto/blake2b"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/kerl"
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
	hornet_pow "github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/profile"
	"github.com/gohornet/hornet/pkg/utils"
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
	coo               *coordinator.Coordinator
	nextTip           = hornet.NullHashBytes
	lastMilestoneHash = hornet.NullHashBytes

	// This is just used to clean up at the end of a test
	cachedBundles tangle.CachedBundles

	// This is to avoid panics when running multiple tests
	setupTangleOnce sync.Once
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
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hash)
		require.NotNil(t, cachedTxMeta)
		if cachedTxMeta.GetMetadata().IsTail() {
			tailTx = cachedTxMeta.GetMetadata().GetTxHash()
		}
		if !expectMilestone {
			cachedTxMeta.GetMetadata().SetSolid(true)
		}

		cachedTxMeta.Release()
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

// We don't need to care about the M-Bug in the spammer => much faster without
func finalizeInsecure(bundle bundle.Bundle) (bundle.Bundle, error) {
	var valueTrits = make([]trinary.Trits, len(bundle))
	var timestampTrits = make([]trinary.Trits, len(bundle))
	var currentIndexTrits = make([]trinary.Trits, len(bundle))
	var obsoleteTagTrits = make([]trinary.Trits, len(bundle))
	var lastIndexTrits = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[0].LastIndex)), 27)

	for i := range bundle {
		valueTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(bundle[i].Value), 81)
		timestampTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[i].Timestamp)), 27)
		currentIndexTrits[i] = trinary.MustPadTrits(trinary.IntToTrits(int64(bundle[i].CurrentIndex)), 27)
		obsoleteTagTrits[i] = trinary.MustPadTrits(trinary.MustTrytesToTrits(bundle[i].ObsoleteTag), 81)
	}

	var bundleHash trinary.Hash

	k := kerl.NewKerl()

	for i := 0; i < len(bundle); i++ {
		relevantTritsForBundleHash := trinary.MustTrytesToTrits(
			bundle[i].Address +
				trinary.MustTritsToTrytes(valueTrits[i]) +
				trinary.MustTritsToTrytes(obsoleteTagTrits[i]) +
				trinary.MustTritsToTrytes(timestampTrits[i]) +
				trinary.MustTritsToTrytes(currentIndexTrits[i]) +
				trinary.MustTritsToTrytes(lastIndexTrits),
		)
		k.Absorb(relevantTritsForBundleHash)
	}

	bundleHashTrits, err := k.Squeeze(consts.HashTrinarySize)
	if err != nil {
		return nil, err
	}
	bundleHash = trinary.MustTritsToTrytes(bundleHashTrits)

	// set the computed bundle hash on each tx in the bundle
	for i := range bundle {
		tx := &bundle[i]
		tx.Bundle = bundleHash
	}

	return bundle, nil
}

func zeroValueTx(t *testing.T, tag trinary.Trytes) []trinary.Trytes {

	var b bundle.Bundle
	entry := bundle.BundleEntry{
		Address:                   trinary.MustPad(utils.RandomTrytesInsecure(consts.AddressTrinarySize/3), consts.AddressTrinarySize/3),
		Value:                     0,
		Tag:                       tag,
		Timestamp:                 uint64(time.Now().UnixNano() / int64(time.Second)),
		Length:                    uint64(1),
		SignatureMessageFragments: []trinary.Trytes{trinary.MustPad("", consts.SignatureMessageFragmentSizeInTrytes)},
	}
	b, err := finalizeInsecure(bundle.AddEntry(b, entry))
	require.NoError(t, err)

	return transaction.MustFinalTransactionTrytes(b)
}

func sendFrom(t *testing.T, tag trinary.Trytes, fromSeed trinary.Trytes, fromIndex uint64, balance uint64, toSeed trinary.Trytes, toIndex uint64, value uint64) []trinary.Trytes {

	_, powFunc := pow.GetFastestProofOfWorkImpl()
	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{
		LocalProofOfWorkFunc: powFunc,
	})
	require.NoError(t, err)

	fromAddress, err := address.GenerateAddresses(fromSeed, fromIndex, 2, consts.SecurityLevelMedium, true)
	require.NoError(t, err)

	toAddress, err := address.GenerateAddress(toSeed, toIndex, consts.SecurityLevelMedium, true)
	require.NoError(t, err)

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

	storeBundleFunc := func(b coordinator.Bundle, isMilestone bool) error {
		var bndl = make(bundle.Bundle, 0)
		// Insert it the reverse way
		for i := len(b) - 1; i >= 0; i-- {
			bndl = append(bndl, *b[i])
		}
		ms := storeBundle(t, bndl, true) // No need to release, since we store all the bundles for later cleanup

		tangle.SetLatestMilestoneIndex(ms.GetBundle().GetMilestoneIndex())

		return nil
	}

	dir, err := ioutil.TempDir("", "coo.test")
	require.NoError(t, err)
	dirAndFile := fmt.Sprintf("%s/coordinator.state", dir)

	// init pow handler
	powHandler := hornet_pow.New(nil, "", 30*time.Second)

	coo = coordinator.New(cooSeed, secLevel, merkleTreeDepth, mwm, dirAndFile, 10, powHandler, storeBundleFunc, merkleHashFunc)
	require.NotNil(t, coo)

	err = coo.InitMerkleTree("coordinator.tree", cooAddress)
	require.NoError(t, err)

	coo.InitState(true, 0)

	// Save snapshot info
	tangle.SetSnapshotMilestone(hornet.HashFromAddressTrytes(cooAddress), hornet.NullHashBytes, 0, 0, 0, time.Now().Unix(), false)

	// Configure Milestones
	tangle.ConfigureMilestones(hornet.HashFromAddressTrytes(cooAddress), int(secLevel), merkleTreeDepth, merkleHashFunc)

	milestoneHash, err := coo.Bootstrap()
	require.NoError(t, err)

	nextTip = milestoneHash
	lastMilestoneHash = milestoneHash

	ms := tangle.GetMilestoneOrNil(1)
	require.NotNil(t, ms)

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	conf, err := whiteflag.ConfirmMilestone(cachedTxMetas, ms.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {}, func(confirmation *whiteflag.Confirmation) {
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(t, err)
	require.Equal(t, 3, conf.TxsConfirmed)
	ms.Release(true)

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

func issueAndConfirmMilestoneOnTip(t *testing.T, tip hornet.Hash, printTangle bool) (*tangle.CachedBundle, *whiteflag.ConfirmedMilestoneStats) {

	nextTip = tip

	currentIndex := tangle.GetSolidMilestoneIndex()
	verifyLMI(t, currentIndex)

	fmt.Printf("Issue milestone %v\n", currentIndex+1)
	milestoneHash, noncriticalErr, criticalErr := coo.IssueMilestone(lastMilestoneHash, nextTip)
	require.NoError(t, noncriticalErr)
	require.NoError(t, criticalErr)
	lastMilestoneHash = milestoneHash

	verifyLMI(t, currentIndex+1)

	milestoneIndex := currentIndex + 1
	ms := tangle.GetMilestoneOrNil(milestoneIndex)
	require.NotNil(t, ms)

	var wfConf *whiteflag.Confirmation

	cachedTxMetas := make(map[string]*tangle.CachedMetadata)

	defer func() {
		// All releases are forced since the cone is confirmed and not needed anymore

		// Release all tx metadata at the end
		for _, cachedTxMeta := range cachedTxMetas {
			cachedTxMeta.Release(true) // meta -1
		}
	}()

	conf, err := whiteflag.ConfirmMilestone(cachedTxMetas, ms.Retain(), func(txMeta *tangle.CachedMetadata, index milestone.Index, confTime int64) {}, func(confirmation *whiteflag.Confirmation) {
		wfConf = confirmation
		tangle.SetSolidMilestoneIndex(confirmation.MilestoneIndex, true)
	})
	require.NoError(t, err)

	require.Equal(t, currentIndex+1, conf.Index)
	verifyLSMI(t, conf.Index)

	cachedBundles = append(cachedBundles, ms)

	assertTotalSupplyStillValid(t)

	if printTangle {
		fmt.Print(generateDotFileFromTangle(t, wfConf))
	}

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

	setupTangleOnce.Do(func() {
		tangle.LoadInitialValuesFromDatabase()
	})
	tangle.ResetSolidEntryPoints()
	tangle.ResetMilestoneIndexes()

	snapshotIndex := milestone.Index(0)

	tangle.StoreSnapshotBalancesInDatabase(balances, snapshotIndex)
	tangle.StoreLedgerBalancesInDatabase(balances, snapshotIndex)

	assertTotalSupplyStillValid(t)

	// Start up the coordinator
	coo := configureCoordinator(t)
	require.NotNil(t, coo)

	verifyLSMI(t, 1)

	var milestones tangle.CachedBundles
	for i := 1; i <= numberOfMilestones; i++ {
		// 2nd milestone
		ms, conf := issueAndConfirmMilestoneOnTip(t, hornet.NullHashBytes, true)
		require.Equal(t, 3, conf.TxsConfirmed) // 3 for milestone
		require.Equal(t, 3, conf.TxsZeroValue) // 3 for milestone
		require.Equal(t, 0, conf.TxsValue)
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
	// Cut the tags at the first 9 or at max length 4
	tagLength := strings.IndexByte(tag, '9')
	if tagLength == -1 || tagLength > 4 || tagLength == 0 {
		tagLength = 4
	}
	return tag[0:tagLength]
}

func generateDotFileFromTangle(t *testing.T, conf *whiteflag.Confirmation) string {

	indexOf := func(hash hornet.Hash) int {
		if conf == nil {
			return -1
		}
		for i := 0; i < len(conf.Mutations.TailsReferenced)-1; i++ {
			if bytes.Equal(conf.Mutations.TailsReferenced[i], hash) {
				return i
			}
		}
		return -1
	}

	visitedBundles := make(map[string]tangle.CachedBundles)

	bundleTxs := tangle.GetAllBundleTransactionHashes(100)
	for _, hash := range bundleTxs {
		cachedTxMeta := tangle.GetCachedTxMetadataOrNil(hash)
		if _, visited := visitedBundles[string(cachedTxMeta.GetMetadata().GetBundleHash())]; visited == false {
			bndls := tangle.GetBundlesOfTransactionOrNil(cachedTxMeta.GetMetadata().GetTxHash(), false)
			visitedBundles[string(cachedTxMeta.GetMetadata().GetBundleHash())] = bndls
		}
		cachedTxMeta.Release(true)
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

			tailHash := bndl.GetBundle().GetTailHash()
			if index := indexOf(tailHash); index != -1 {
				dotFile += fmt.Sprintf("\"%s\" [ label=\"[%d] %s\" ];\n", shortBundle, index, shortBundle)
			}

			trunk := bndl.GetBundle().GetTrunkHash(true)
			if tangle.SolidEntryPointsContain(trunk) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, shortenedHash(trunk))
			} else {
				trunkBundles := tangle.GetBundlesOfTransactionOrNil(trunk, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Trunk\" ];\n", shortBundle, shortened(trunkBundles[0]))
				trunkBundles.Release()
			}

			branch := bndl.GetBundle().GetBranchHash(true)
			if tangle.SolidEntryPointsContain(branch) {
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, shortenedHash(branch))
			} else {
				branchBundles := tangle.GetBundlesOfTransactionOrNil(branch, false)
				dotFile += fmt.Sprintf("\"%s\" -> \"%s\" [ label=\"Branch\" ];\n", shortBundle, shortened(branchBundles[0]))
				branchBundles.Release()
			}

			if bndl.GetBundle().IsMilestone() {
				if conf != nil && bndl.GetBundle().GetMilestoneIndex() == conf.MilestoneIndex {
					dotFile += fmt.Sprintf("\"%s\" [style=filled,color=gold];\n", shortBundle)
				}
				milestones = append(milestones, shortBundle)
			} else if bndl.GetBundle().IsConfirmed() {
				if bndl.GetBundle().IsConflicting() {
					conflicting = append(conflicting, shortBundle)
				} else if bndl.GetBundle().IsValueSpam() {
					ignored = append(ignored, shortBundle)
				} else {
					included = append(included, shortBundle)
				}
			}
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
	// Valid transfer 100 from seed1[0] to seed2[0]
	bundleA := storeBundle(t, attachTo(t, milestones[0].GetBundle().GetTailHash(), milestones[1].GetBundle().GetTailHash(), sendFrom(t, "A", seed1, 0, 1000, seed2, 0, 100)), false)
	// Valid transfer 200 from seed1[1] to seed2[0]
	bundleB := storeBundle(t, attachTo(t, bundleA.GetBundle().GetTailHash(), milestones[0].GetBundle().GetTailHash(), sendFrom(t, "B", seed1, 1, 900, seed2, 0, 200)), false)
	// Invalid transfer 10 from seed3[0] to seed2[0] (insufficient funds)
	bundleC := storeBundle(t, attachTo(t, milestones[2].GetBundle().GetTailHash(), bundleB.GetBundle().GetTailHash(), sendFrom(t, "C", seed3, 0, 99999, seed2, 0, 10)), false)
	// Invalid transfer 150 from seed4[1] to seed2[0] (insufficient funds)
	bundleD := storeBundle(t, attachTo(t, bundleA.GetBundle().GetTailHash(), bundleC.GetBundle().GetTailHash(), sendFrom(t, "D", seed4, 1, 99999, seed2, 0, 150)), false)
	// Valid transfer 150 from seed2[0] to seed4[0]
	bundleE := storeBundle(t, attachTo(t, bundleB.GetBundle().GetTailHash(), bundleD.GetBundle().GetTailHash(), sendFrom(t, "E", seed2, 0, 300, seed4, 0, 150)), false)

	// Confirming milestone at bundle C (bundle D and E are not included)
	_, conf := issueAndConfirmMilestoneOnTip(t, bundleC.GetBundle().GetTailHash(), true)
	require.Equal(t, 4+4+4+3, conf.TxsConfirmed) // 3 are for the milestone itself
	require.Equal(t, 8, conf.TxsValue)
	require.Equal(t, 4, conf.TxsConflicting)
	require.Equal(t, 3, conf.TxsZeroValue) // The milestone

	// Verify balances (seed, index, balance)
	assertAddressBalance(t, seed1, 0, 0)
	assertAddressBalance(t, seed1, 1, 0)
	assertAddressBalance(t, seed1, 2, 700)
	assertAddressBalance(t, seed2, 0, 300)
	assertAddressBalance(t, seed2, 1, 0)
	assertAddressBalance(t, seed3, 0, 0)
	assertAddressBalance(t, seed3, 1, 0)
	assertAddressBalance(t, seed4, 0, 0)
	assertAddressBalance(t, seed4, 1, 0)

	// Confirming milestone at bundle E
	_, conf = issueAndConfirmMilestoneOnTip(t, bundleE.GetBundle().GetTailHash(), true)
	require.Equal(t, 4+4+3, conf.TxsConfirmed) // 3 are for the milestone itself
	require.Equal(t, 4, conf.TxsValue)
	require.Equal(t, 4, conf.TxsConflicting)
	require.Equal(t, 3, conf.TxsZeroValue) // The milestone

	// Verify balances (seed, index, balance)
	assertAddressBalance(t, seed1, 0, 0)
	assertAddressBalance(t, seed1, 1, 0)
	assertAddressBalance(t, seed1, 2, 700)
	assertAddressBalance(t, seed2, 0, 0)
	assertAddressBalance(t, seed2, 1, 150)
	assertAddressBalance(t, seed3, 0, 0)
	assertAddressBalance(t, seed3, 1, 0)
	assertAddressBalance(t, seed4, 0, 150)
	assertAddressBalance(t, seed4, 1, 0)

	// Clean up all the bundles we created
	cachedBundles.Release()
	cachedBundles = nil

	// This should not hang, i.e. all objects should be released
	tangle.ShutdownStorages()
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	// Fill up the balances
	balances := make(map[string]uint64)
	milestones := setupCoordinatorAndIssueInitialMilestones(t, balances, 3)

	// Issue some transactions
	bundleA := storeBundle(t, attachTo(t, milestones[0].GetBundle().GetTailHash(), milestones[1].GetBundle().GetTailHash(), zeroValueTx(t, "A")), false)
	bundleB := storeBundle(t, attachTo(t, bundleA.GetBundle().GetTailHash(), milestones[0].GetBundle().GetTailHash(), zeroValueTx(t, "B")), false)
	bundleC := storeBundle(t, attachTo(t, milestones[2].GetBundle().GetTailHash(), milestones[0].GetBundle().GetTailHash(), zeroValueTx(t, "C")), false)
	bundleD := storeBundle(t, attachTo(t, bundleB.GetBundle().GetTailHash(), bundleC.GetBundle().GetTailHash(), zeroValueTx(t, "D")), false)
	bundleE := storeBundle(t, attachTo(t, bundleB.GetBundle().GetTailHash(), bundleA.GetBundle().GetTailHash(), zeroValueTx(t, "E")), false)

	// Confirming milestone include all tx up to bundle E. This should only include A, B and E
	_, conf := issueAndConfirmMilestoneOnTip(t, bundleE.GetBundle().GetTailHash(), true)
	require.Equal(t, 3+3, conf.TxsConfirmed) // A, B, E + 3 for Milestone
	require.Equal(t, 3+3, conf.TxsZeroValue) // 3 are for the milestone itself
	require.Equal(t, 0, conf.TxsValue)
	require.Equal(t, 0, conf.TxsConflicting)

	// Issue another bundle
	bundleF := storeBundle(t, attachTo(t, bundleD.GetBundle().GetTailHash(), bundleE.GetBundle().GetTailHash(), zeroValueTx(t, "F")), false)

	// Confirming milestone at bundle F. This should confirm D, C and F
	_, conf = issueAndConfirmMilestoneOnTip(t, bundleF.GetBundle().GetTailHash(), true)

	require.Equal(t, 3+3, conf.TxsConfirmed) // D, C, F + 3 for Milestone
	require.Equal(t, 3+3, conf.TxsZeroValue) // 3 are for the milestone itself
	require.Equal(t, 0, conf.TxsValue)
	require.Equal(t, 0, conf.TxsConflicting)

	// Clean up all the bundles we created
	cachedBundles.Release()
	cachedBundles = nil

	// This should not hang, i.e. all objects should be released
	tangle.ShutdownStorages()
}
