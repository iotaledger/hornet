package test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestTestEnv verifies that our ReferendumTestEnv is sane. This allows us to skip the assertions on the other tests to speed them up
func TestReferendumTestEnv(t *testing.T) {

	randomBalance := func() uint64 {
		return uint64(rand.Intn(256)) * 1_000_000
	}

	env := NewReferendumTestEnv(t, randomBalance(), randomBalance(), randomBalance(), randomBalance(), true)
	defer env.Cleanup()
	require.NotNil(t, env)
}
