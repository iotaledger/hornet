//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package autopeering_test

import (
	"fmt"
	"testing"

	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hornet/v2/pkg/p2p/autopeering"
)

func TestMultiAddrAutopeeringProtocol(t *testing.T) {
	require.NoError(t, autopeering.RegisterAutopeeringProtocolInMultiAddresses())
	base58PubKey := "HmKTkSd9F6nnERBvVbr55FvL1hM5WfcLvsc9bc3hWxWc"
	smpl := fmt.Sprintf("/ip4/127.0.0.1/udp/14626/autopeering/%s", base58PubKey)
	ma, err := multiaddr.NewMultiaddr(smpl)
	require.NoError(t, err)

	extractedBase58PubKey, err := ma.ValueForProtocol(autopeering.ProtocolCode)
	require.NoError(t, err)

	require.Equal(t, base58PubKey, extractedBase58PubKey)
}
