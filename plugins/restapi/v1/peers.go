package v1

import (
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p-core/peer"

	p2ppkg "github.com/gohornet/hornet/pkg/p2p"
	p2pplug "github.com/gohornet/hornet/plugins/p2p"
	"github.com/gohornet/hornet/plugins/restapi/common"
)

func wrapInfoSnapshot(info *p2ppkg.PeerInfoSnapshot) *peerResponse {
	var alias *string

	if info.Alias != "" {
		alias = &info.Alias
	}

	return &peerResponse{
		ID:           info.ID,
		MultiAddress: info.Addresses,
		Alias:        alias,
		Relation:     "not implemented",
		Connected:    info.Connected,
		GossipMetrics: &peerGossipMetrics{
			DroppedSentPackets: info.DroppedSentPackets,
			SentPackets:        info.SentPackets,
		},
	}
}

func getPeer(c echo.Context) (*peerResponse, error) {
	peerID, err := peer.IDFromString(c.Param(ParameterPeerID))
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid peerID, error: %w", err)
	}

	info := p2pplug.Manager().PeerInfoSnapshot(peerID)
	if info == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "peer not found, peerID: %s", peerID.String())
	}

	return wrapInfoSnapshot(info), nil
}

func removePeer(c echo.Context) error {
	peerID, err := peer.IDFromString(c.Param(ParameterPeerID))
	if err != nil {
		return errors.WithMessagef(common.ErrInvalidParameter, "invalid peerID, error: %w", err)
	}
	return p2pplug.Manager().DisconnectPeer(peerID)
}

func listPeers(c echo.Context) ([]*peerResponse, error) {
	var results []*peerResponse

	for _, info := range p2pplug.Manager().PeerInfoSnapshots() {
		results = append(results, wrapInfoSnapshot(info))
	}

	return results, nil
}

func addPeer(c echo.Context) (*peerResponse, error) {

	request := &addPeerRequest{}

	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid addPeerRequest, error: %w", err)
	}

	multiAddr, err := multiaddr.NewMultiaddr(request.MultiAddress)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid multiAddress, error: %w", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "invalid multiAddress, error: %w", err)
	}

	var alias string
	if request.Alias != nil {
		alias = *request.Alias
	}

	// error is ignored, because the peer is added to the known peers and protected from trimming
	_ = p2pplug.Manager().ConnectPeer(addrInfo, p2ppkg.PeerRelationKnown, alias)

	info := p2pplug.Manager().PeerInfoSnapshot(addrInfo.ID)
	if info == nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "peer not found, peerID: %s", addrInfo.ID.String())
	}

	return wrapInfoSnapshot(info), nil
}
