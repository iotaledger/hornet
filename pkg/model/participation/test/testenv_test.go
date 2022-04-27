package test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParticipationTestEnv verifies that our ParticipationTestEnv is sane. This allows us to skip the assertions on the other tests to speed them up
func TestParticipationTestEnv(t *testing.T) {

	randomBalance := func() uint64 {
		return uint64(1+rand.Intn(255)) * 1_000_000
	}

	env := NewParticipationTestEnv(t, randomBalance(), randomBalance(), randomBalance(), randomBalance(), true)
	defer env.Cleanup()
	require.NotNil(t, env)
}
