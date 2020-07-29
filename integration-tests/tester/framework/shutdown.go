package framework

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Shutdowner can shutdown itself.
type Shutdowner interface {
	Shutdown() error
}

// ShutdownNetwork shuts down the network and reports errors.
func ShutdownNetwork(t *testing.T, n Shutdowner) {
	err := n.Shutdown()
	require.NoError(t, err)
}
