package v1

import (
	"strings"

	"github.com/labstack/echo/v4"
)

func getPeer(c echo.Context) (*peerResponse, error) {

	//peerID := strings.ToLower(c.Param(ParameterPeerID))

	return nil, nil
}

func removePeer(c echo.Context) error {

	peerID := strings.ToLower(c.Param(ParameterPeerID))
	_ = peerID
	return nil
	//return p2pplug.Manager().DisconnectPeer(peerID)
}

func listPeers(c echo.Context) ([]*peerResponse, error) {

	// peering.Manager().PeerInfos()
	return nil, nil
}

func addPeer(c echo.Context) (*peerResponse, error) {

	/*
		if err := peering.Manager().Add(uri, preferIPv6, uri); err != nil {
		}
	*/

	return nil, nil
}
