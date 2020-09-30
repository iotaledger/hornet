package test

import (
	"testing"

	_ "golang.org/x/crypto/blake2b"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/testsuite"
	"github.com/gohornet/hornet/pkg/testsuite/utils"
)

const (
	seed1 = "JBN9ZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWZ"
	seed2 = "JBNAZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWZ"
	seed3 = "JBNBZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWZ"
	seed4 = "DBNBZRCOH9YRUGSWIQNZWAIFEZUBDUGTFPVRKXWPAUCEQQFS9NHPQLXCKZKRHVCCUZNF9CZZWKXRZVCWZ"

	showConfirmationGraphs = false
)

func TestWhiteFlagWithMultipleConflicting(t *testing.T) {

	// Fill up the balances
	balances := make(map[string]uint64)
	balances[string(utils.GenerateAddress(t, seed1, 0))] = 1000

	te := testsuite.SetupTestEnvironment(t, balances, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	// Issue some transactions
	// Valid transfer 100 from seed1[0] to seed2[0]
	bundleA := te.AttachAndStoreBundle(te.Milestones[0].GetMessage().GetTailHash(), te.Milestones[1].GetMessage().GetTailHash(), utils.ValueTx(t, "A", seed1, 0, 1000, seed2, 0, 100))
	// Valid transfer 200 from seed1[1] to seed2[0]
	bundleB := te.AttachAndStoreBundle(bundleA.GetMessage().GetTailHash(), te.Milestones[0].GetMessage().GetTailHash(), utils.ValueTx(t, "B", seed1, 1, 900, seed2, 0, 200))
	// Invalid transfer 10 from seed3[0] to seed2[0] (insufficient funds)
	bundleC := te.AttachAndStoreBundle(te.Milestones[2].GetMessage().GetTailHash(), bundleB.GetMessage().GetTailHash(), utils.ValueTx(t, "C", seed3, 0, 99999, seed2, 0, 10))
	// Invalid transfer 150 from seed4[1] to seed2[0] (insufficient funds)
	bundleD := te.AttachAndStoreBundle(bundleA.GetMessage().GetTailHash(), bundleC.GetMessage().GetTailHash(), utils.ValueTx(t, "D", seed4, 1, 99999, seed2, 0, 150))
	// Valid transfer 150 from seed2[0] to seed4[0]
	bundleE := te.AttachAndStoreBundle(bundleB.GetMessage().GetTailHash(), bundleD.GetMessage().GetTailHash(), utils.ValueTx(t, "E", seed2, 0, 300, seed4, 0, 150))

	// Confirming milestone at bundle C (bundle D and E are not included)
	conf := te.IssueAndConfirmMilestoneOnTip(bundleC.GetMessage().GetTailHash(), true)

	require.Equal(t, 4+4+4+3, conf.MessagesConfirmed) // 3 are for the milestone itself
	require.Equal(t, 8, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 4, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3, conf.MessagesExcludedWithoutTransactions) // The milestone

	// Verify balances (seed, index, balance)
	te.AssertAddressBalance(seed1, 0, 0)
	te.AssertAddressBalance(seed1, 1, 0)
	te.AssertAddressBalance(seed1, 2, 700)
	te.AssertAddressBalance(seed2, 0, 300)
	te.AssertAddressBalance(seed2, 1, 0)
	te.AssertAddressBalance(seed3, 0, 0)
	te.AssertAddressBalance(seed3, 1, 0)
	te.AssertAddressBalance(seed4, 0, 0)
	te.AssertAddressBalance(seed4, 1, 0)

	// Confirming milestone at bundle E
	conf = te.IssueAndConfirmMilestoneOnTip(bundleE.GetMessage().GetTailHash(), true)
	require.Equal(t, 4+4+3, conf.MessagesConfirmed) // 3 are for the milestone itself
	require.Equal(t, 4, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 4, conf.MessagesExcludedWithConflictingTransactions)
	require.Equal(t, 3, conf.MessagesExcludedWithoutTransactions) // The milestone

	// Verify balances (seed, index, balance)
	te.AssertAddressBalance(seed1, 0, 0)
	te.AssertAddressBalance(seed1, 1, 0)
	te.AssertAddressBalance(seed1, 2, 700)
	te.AssertAddressBalance(seed2, 0, 0)
	te.AssertAddressBalance(seed2, 1, 150)
	te.AssertAddressBalance(seed3, 0, 0)
	te.AssertAddressBalance(seed3, 1, 0)
	te.AssertAddressBalance(seed4, 0, 150)
	te.AssertAddressBalance(seed4, 1, 0)
}

func TestWhiteFlagWithOnlyZeroTx(t *testing.T) {

	// Fill up the balances
	balances := make(map[string]uint64)
	te := testsuite.SetupTestEnvironment(t, balances, 3, showConfirmationGraphs)
	defer te.CleanupTestEnvironment(!showConfirmationGraphs)

	// Issue some transactions
	bundleA := te.AttachAndStoreBundle(te.Milestones[0].GetMessage().GetTailHash(), te.Milestones[1].GetMessage().GetTailHash(), utils.ZeroValueTx(t, "A"))
	bundleB := te.AttachAndStoreBundle(bundleA.GetMessage().GetTailHash(), te.Milestones[0].GetMessage().GetTailHash(), utils.ZeroValueTx(t, "B"))
	bundleC := te.AttachAndStoreBundle(te.Milestones[2].GetMessage().GetTailHash(), te.Milestones[0].GetMessage().GetTailHash(), utils.ZeroValueTx(t, "C"))
	bundleD := te.AttachAndStoreBundle(bundleB.GetMessage().GetTailHash(), bundleC.GetMessage().GetTailHash(), utils.ZeroValueTx(t, "D"))
	bundleE := te.AttachAndStoreBundle(bundleB.GetMessage().GetTailHash(), bundleA.GetMessage().GetTailHash(), utils.ZeroValueTx(t, "E"))

	// Confirming milestone include all msg up to bundle E. This should only include A, B and E
	conf := te.IssueAndConfirmMilestoneOnTip(bundleE.GetMessage().GetTailHash(), true)
	require.Equal(t, 3+3, conf.MessagesConfirmed)                   // A, B, E + 3 for Milestone
	require.Equal(t, 3+3, conf.MessagesExcludedWithoutTransactions) // 3 are for the milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)

	// Issue another bundle
	bundleF := te.AttachAndStoreBundle(bundleD.GetMessage().GetTailHash(), bundleE.GetMessage().GetTailHash(), utils.ZeroValueTx(t, "F"))

	// Confirming milestone at bundle F. This should confirm D, C and F
	conf = te.IssueAndConfirmMilestoneOnTip(bundleF.GetMessage().GetTailHash(), true)

	require.Equal(t, 3+3, conf.MessagesConfirmed)                   // D, C, F + 3 for Milestone
	require.Equal(t, 3+3, conf.MessagesExcludedWithoutTransactions) // 3 are for the milestone itself
	require.Equal(t, 0, conf.MessagesIncludedWithTransactions)
	require.Equal(t, 0, conf.MessagesExcludedWithConflictingTransactions)
}
