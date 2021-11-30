package test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFaucetTestEnv verifies that our FaucetTestEnv is sane. This allows us to skip the assertions on the other tests to speed them up
func TestFaucetTestEnv(t *testing.T) {

	randomBalance := func() uint64 {
		return uint64(rand.Intn(256)) * 1_000_000
	}

	env := NewFaucetTestEnv(t,
		randomBalance(), // faucetBalance
		randomBalance(), // wallet1Balance
		randomBalance(), // wallet2Balance
		randomBalance(), // wallet3Balance
		10_000_000,      // faucetAmount:				10 Mi
		1_000_000,       // faucetSmallAmount: 		 	 1 Mi
		20_000_000,      // faucetMaxAddressBalance:	20 Mi
		true)
	defer env.Cleanup()
	require.NotNil(t, env)
}
