package v1

import (
	"github.com/labstack/echo/v4"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	"github.com/gohornet/hornet/pkg/restapi"
)

// WrapInfoSnapshot wraps the given peer info snapshot with additional metadata, such as gossip protocol information.
func WrapInfoSnapshot(info *p2p.PeerInfoSnapshot) *PeerResponse {
	var alias *string

	if info.Alias != "" {
		alias = &info.Alias
	}

	multiAddresses := make([]string, len(info.Addresses))
	for i, multiAddress := range info.Addresses {
		multiAddresses[i] = multiAddress.String()
	}

	gossipProto := deps.Service.Protocol(info.Peer.ID)
	var gossipInfo *gossip.Info
	if gossipProto != nil {
		gossipInfo = gossipProto.Info()
	}

	return &PeerResponse{
		ID:             info.ID,
		MultiAddresses: multiAddresses,
		Alias:          alias,
		Relation:       info.Relation,
		Connected:      info.Connected,
		Gossip:         gossipInfo,
	}
}

func getPeer(c echo.Context) (*PeerResponse, error) {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid peerID, error: %s", err)
	}

	info := deps.Manager.PeerInfoSnapshot(peerID)
	if info == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "peer not found, peerID: %s", peerID.String())
	}

	return WrapInfoSnapshot(info), nil
}

func removePeer(c echo.Context) error {
	peerID, err := peer.Decode(c.Param(ParameterPeerID))
	if err != nil {
		return errors.WithMessagef(restapi.ErrInvalidParameter, "invalid peerID, error: %s", err)
	}

	// error is ignored because we don't care about the config here
	_ = deps.PeeringConfigManager.RemovePeer(peerID)

	return deps.Manager.DisconnectPeer(peerID, errors.New("peer was removed via API"))
}

//nolint:unparam // even if the error is never used, the structure of all routes should be the same
func listPeers(_ echo.Context) ([]*PeerResponse, error) {
	peerInfos := deps.Manager.PeerInfoSnapshots()
	results := make([]*PeerResponse, len(peerInfos))
	for i, info := range peerInfos {
		results[i] = WrapInfoSnapshot(info)
	}
	return results, nil
}

func addPeer(c echo.Context) (*PeerResponse, error) {

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

	// error is ignored because the peer is added to the known peers and protected from trimming
	_ = deps.Manager.ConnectPeer(addrInfo, p2p.PeerRelationKnown, alias)

	info := deps.Manager.PeerInfoSnapshot(addrInfo.ID)
	if info == nil {
		return nil, errors.WithMessagef(echo.ErrNotFound, "peer not found, peerID: %s", addrInfo.ID.String())
	}

	// error is ignored because we don't care about the config here
	_ = deps.PeeringConfigManager.AddPeer(multiAddr, alias)

	return WrapInfoSnapshot(info), nil
}
