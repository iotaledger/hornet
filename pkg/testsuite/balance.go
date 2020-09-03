package testsuite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

// GenerateAddress generates an address for the given seed and index with medium security.
func GenerateAddress(t *testing.T, seed trinary.Trytes, index uint64) hornet.Hash {
	seedAddress, err := address.GenerateAddress(seed, index, consts.SecurityLevelMedium, false)
	require.NoError(t, err)
	return hornet.HashFromAddressTrytes(seedAddress)
}

// AssertAddressBalance generates an address for the given seed and index and checks correct balance.
func AssertAddressBalance(t *testing.T, seed trinary.Trytes, index uint64, balance uint64) {
	address := GenerateAddress(t, seed, index)
	addrBalance, _, err := tangle.GetBalanceForAddress(address)
	require.NoError(t, err)
	require.Equal(t, balance, addrBalance)
}

// AssertTotalSupplyStillValid checks if the total supply in the database is still correct.
func AssertTotalSupplyStillValid(t *testing.T) {
	_, _, err := tangle.GetLedgerStateForLSMI(nil)
	require.NoError(t, err)
}
