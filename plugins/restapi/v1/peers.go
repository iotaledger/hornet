package v1

import (
	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p-core/peer"

	p2ppkg "github.com/gohornet/hornet/pkg/p2p"
)

func wrapInfoSnapshot(info *p2ppkg.PeerInfoSnapshot) *peerResponse {
	var alias *string

	if info.Alias != "" {
		alias = &info.Alias
	}

	multiAddresses := []string{}

	for _, multiAddress := range info.Addresses {
		multiAddresses = append(multiAddresses, multiAddress.String())
	}

	return &peerResponse{
		ID:             info.ID,
		MultiAddresses: multiAddresses,
		Alias:          alias,
		Relation:       info.Relation,
		Connected:      info.Connected,
		GossipMetrics: &peerGossipMetrics{
			DroppedSentPackets: info.DroppedSentPackets,
			SentPackets:        info.SentPackets,
		},
	}
}

func getPeer(c echo.Context) (*peerResponse, error) {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid peerID, error: %s", err)
	}

	info := deps.Manager.PeerInfoSnapshot(peerID)
	if info == nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "peer not found, peerID: %s", peerID.String())
	}

	return wrapInfoSnapshot(info), nil
}

func removePeer(c echo.Context) error {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return errors.WithMessagef(restapi.ErrInvalidParameter, "invalid peerID, error: %s", err)
	}
	return deps.Manager.DisconnectPeer(peerID, errors.New("peer was removed via API"))
}

func listPeers(c echo.Context) ([]*peerResponse, error) {
	var results []*peerResponse

	for _, info := range deps.Manager.PeerInfoSnapshots() {
		results = append(results, wrapInfoSnapshot(info))
	}

	return results, nil
}

func addPeer(c echo.Context) (*peerResponse, error) {

	request := &addPeerRequest{}

	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid addPeerRequest, error: %s", err)
	}

	multiAddr, err := multiaddr.NewMultiaddr(request.MultiAddress)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid multiAddress, error: %s", err)
	}

	addrInfo, err := peer.AddrInfoFromP2pAddr(multiAddr)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid multiAddress, error: %s", err)
	}

	var alias string
	if request.Alias != nil {
		alias = *request.Alias
	}

	// error is ignored, because the peer is added to the known peers and protected from trimming
	_ = deps.Manager.ConnectPeer(addrInfo, p2ppkg.PeerRelationKnown, alias)

	info := deps.Manager.PeerInfoSnapshot(addrInfo.ID)
	if info == nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "peer not found, peerID: %s", addrInfo.ID.String())
	}

	return wrapInfoSnapshot(info), nil
}
