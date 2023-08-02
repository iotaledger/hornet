package webapi

import (
	"io"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/plugins/peering"
)

func (s *WebAPIServer) rpcAddNeighbors(c echo.Context) (interface{}, error) {

	// Read the content of the body
	var bodyBytes []byte
	if c.Request().Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(c.Request().Body)
		if err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	}

	// we need to restore the body after reading it
	restoreBody(c, bodyBytes)

	// Check if HORNET style addNeighbors call was made
	requestHornet := &AddNeighborsHornet{}
	if err := c.Bind(requestHornet); err == nil {
		if len(requestHornet.Neighbors) != 0 {
			return s.rpcAddNeighborsWithAlias(c, requestHornet)
		}
	}

	// we need to restore the body after reading it
	restoreBody(c, bodyBytes)

	request := &AddNeighbors{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	addedPeers := 0

	preferIPv6 := config.NodeConfig.GetBool(config.CfgNetPreferIPv6)

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		s.logger.Warn(err)
	}

	for _, uri := range request.Uris {

		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		} else if strings.Contains(uri, "://") {
			continue
		}

		contains := false
		for _, cn := range configPeers {
			if cn.ID == uri {
				contains = true
				break
			}
		}

		if !contains {
			configPeers = append(configPeers, config.PeerConfig{
				ID:         uri,
				Alias:      uri,
				PreferIPv6: preferIPv6,
			})
			added = true
		}

		if err := peering.Manager().Add(uri, preferIPv6, uri); err != nil {
			s.logger.Warnf("can't add peer %s, Error: %s", uri, err)
			continue
		}
		addedPeers++
	}

	if added {
		config.DenyPeeringConfigHotReload()
		config.PeeringConfig.Set(config.CfgPeers, configPeers)
		config.PeeringConfig.WriteConfig()
		config.AllowPeeringConfigHotReload()
	}

	return &AddNeighborsResponse{AddedNeighbors: addedPeers}, nil
}

func (s *WebAPIServer) rpcAddNeighborsWithAlias(c echo.Context, request *AddNeighborsHornet) (interface{}, error) {
	addedPeers := 0

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		s.logger.Warn(err)
	}

	for _, peer := range request.Neighbors {

		if strings.Contains(peer.Identity, "tcp://") {
			peer.Identity = peer.Identity[6:]
		} else if strings.Contains(peer.Identity, "://") {
			continue
		}

		contains := false
		for _, cn := range configPeers {
			if cn.ID == peer.Identity {
				contains = true
				break
			}
		}

		if !contains {
			configPeers = append(configPeers, config.PeerConfig{
				ID:         peer.Identity,
				Alias:      peer.Alias,
				PreferIPv6: peer.PreferIPv6,
			})
			added = true
		}

		if err := peering.Manager().Add(peer.Identity, peer.PreferIPv6, peer.Alias); err != nil {
			s.logger.Warnf("Can't add peer %s, Error: %s", peer.Identity, err)
			continue
		}
		addedPeers++
	}

	if added {
		config.DenyPeeringConfigHotReload()
		config.PeeringConfig.Set(config.CfgPeers, configPeers)
		config.PeeringConfig.WriteConfig()
		config.AllowPeeringConfigHotReload()
	}

	return &AddNeighborsResponse{AddedNeighbors: addedPeers}, nil
}

func (s *WebAPIServer) rpcRemoveNeighbors(c echo.Context) (interface{}, error) {
	request := &RemoveNeighbors{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	removedNeighbors := 0
	removed := false

	var configNeighbors []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configNeighbors); err != nil {
		s.logger.Warn(err)
	}

	peers := peering.Manager().PeerInfos()
	for _, uri := range request.Uris {
		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		}
		for _, p := range peers {

			for i, cn := range configNeighbors {
				if strings.EqualFold(cn.ID, uri) {
					removed = true

					// Delete item
					configNeighbors[i] = configNeighbors[len(configNeighbors)-1]
					configNeighbors = configNeighbors[:len(configNeighbors)-1]
					break
				}
			}

			// Remove connected neighbor
			if p.Peer != nil {
				if strings.EqualFold(p.Peer.ID, uri) || strings.EqualFold(p.DomainWithPort, uri) {
					err := peering.Manager().Remove(uri)
					if err != nil {
						return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
					}
					removedNeighbors++
				}
			} else {
				// Remove unconnected neighbor
				if strings.EqualFold(p.Address, uri) {
					err := peering.Manager().Remove(uri)
					if err != nil {
						return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
					}
					removedNeighbors++
				}
			}
		}
	}

	if removed {
		config.DenyPeeringConfigHotReload()
		config.PeeringConfig.Set(config.CfgPeers, configNeighbors)
		config.PeeringConfig.WriteConfig()
		config.AllowPeeringConfigHotReload()
	}

	return &RemoveNeighborsResponse{RemovedNeighbors: uint(removedNeighbors)}, nil
}

func (s *WebAPIServer) rpcGetNeighbors(c echo.Context) (interface{}, error) {
	return &GetNeighborsResponse{Neighbors: peering.Manager().PeerInfos()}, nil
}
