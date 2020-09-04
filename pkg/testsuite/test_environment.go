package testsuite

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/iota.go/consts"

	"github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/profile"
)

var (
	// setupTangleOnce is used to avoid panics when running multiple tests.
	setupTangleOnce sync.Once
)

// TestEnvironment holds the state of the test environment.
type TestEnvironment struct {
	// testState is the state of the current test case.
	testState *testing.T

	// Milestones are the created milestones by the coordinator during the test.
	Milestones tangle.CachedBundles

	// cachedBundles is used to cleanup all bundles at the end of a test.
	cachedBundles tangle.CachedBundles

	// showConfirmationGraphs is set if pictures of the confirmation graph should be externally opened during the test.
	showConfirmationGraphs bool

	// powHandler holds the powHandler instance.
	powHandler *pow.Handler

	// coo holds the coordinator instance.
	coo *coordinator.Coordinator

	// lastMilestoneHash is the tail transaction hash of the last issued milestone.
	lastMilestoneHash hornet.Hash

	// tempDir is the directory that contains the temporary files for the test.
	tempDir string

	// store is the temporary key value store for the test.
	store kvstore.KVStore
}

// searchProjectRootFolder searches the hornet root directory.
// this is used to always find the "assets" folder if executed from different test cases.
func searchProjectRootFolder() string {
	wd, _ := os.Getwd()

	for !strings.HasSuffix(wd, "pkg") && !strings.HasSuffix(wd, "plugins") {
		wd = filepath.Dir(wd)
	}

	// one more time to get to the root dir
	wd = filepath.Dir(wd)

	return wd
}

// SetupTestEnvironment initializes a clean database with initial balances,
// configures a coordinator with a clean state, bootstraps the network and issues the first "numberOfMilestones" milestones.
func SetupTestEnvironment(testState *testing.T, initialBalances map[string]uint64, numberOfMilestones int, showConfirmationGraphs bool) *TestEnvironment {

	te := &TestEnvironment{
		testState:              testState,
		Milestones:             make(tangle.CachedBundles, 0),
		cachedBundles:          make(tangle.CachedBundles, 0),
		showConfirmationGraphs: showConfirmationGraphs,
		powHandler:             pow.New(nil, "", 30*time.Second),
		lastMilestoneHash:      hornet.NullHashBytes,
	}

	tempDir, err := ioutil.TempDir("", fmt.Sprintf("test_%s", testState.Name()))
	require.NoError(te.testState, err)
	te.tempDir = tempDir

	balances := initialBalances
	var sum uint64
	for _, value := range balances {
		sum += value
	}

	// Move remaining supply to 999..999
	balances[string(hornet.NullHashBytes)] = consts.TotalSupply - sum

	te.store = mapdb.NewMapDB()
	te.configureStorages(te.store)

	tangle.ResetSolidEntryPoints()
	tangle.ResetMilestoneIndexes()

	snapshotIndex := milestone.Index(0)

	tangle.StoreSnapshotBalancesInDatabase(balances, snapshotIndex)
	tangle.StoreLedgerBalancesInDatabase(balances, snapshotIndex)

	te.AssertTotalSupplyStillValid()

	// Start up the coordinator
	te.configureCoordinator()
	require.NotNil(testState, te.coo)

	te.VerifyLSMI(1)

	for i := 1; i <= numberOfMilestones; i++ {
		conf := te.IssueAndConfirmMilestoneOnTip(hornet.NullHashBytes, false)
		require.Equal(testState, 3, conf.TxsConfirmed) // 3 for milestone
		require.Equal(testState, 3, conf.TxsZeroValue) // 3 for milestone
		require.Equal(testState, 0, conf.TxsValue)
		require.Equal(testState, 0, conf.TxsConflicting)
	}

	return te
}

// configureStorages initializes the storage layer.
func (te *TestEnvironment) configureStorages(store kvstore.KVStore) {

	tangle.ConfigureStorages(
		store.WithRealm([]byte("tangle")),
		store.WithRealm([]byte("snapshot")),
		store.WithRealm([]byte("spent")),
		profile.Profile2GB.Caches,
	)

	setupTangleOnce.Do(func() {
		tangle.LoadInitialValuesFromDatabase()
	})
}

// CleanupTestEnvironment cleans up everything at the end of the test.
func (te *TestEnvironment) CleanupTestEnvironment(removeTempDir bool) {
	te.cachedBundles.Release()
	te.cachedBundles = nil

	// this should not hang, i.e. all objects should be released
	tangle.ShutdownStorages()

	te.store.Clear()

	if removeTempDir && te.tempDir != "" {
		os.RemoveAll(te.tempDir)
	}
}
