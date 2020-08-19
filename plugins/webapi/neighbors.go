package webapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/peering"
)

func init() {
	addEndpoint("addNeighbors", addNeighbors, implementedAPIcalls)
	addEndpoint("removeNeighbors", removeNeighbors, implementedAPIcalls)
	addEndpoint("getNeighbors", getNeighbors, implementedAPIcalls)
}

func addNeighbors(i interface{}, c *gin.Context, _ <-chan struct{}) {

	// Check if HORNET style addNeighbors call was made
	queryHornet := &AddNeighborsHornet{}
	if err := mapstructure.Decode(i, queryHornet); err == nil {
		if len(queryHornet.Neighbors) != 0 {
			addNeighborsWithAlias(queryHornet, c)
			return
		}
	}

	e := ErrorReturn{}
	query := &AddNeighbors{}
	addedPeers := 0

	preferIPv6 := config.NodeConfig.GetBool(config.CfgNetPreferIPv6)

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		log.Warn(err)
	}

	for _, uri := range query.Uris {

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
			log.Warnf("can't add peer %s, Error: %s", uri, err)
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

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedPeers})
}

func addNeighborsWithAlias(s *AddNeighborsHornet, c *gin.Context) {
	addedPeers := 0

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		log.Warn(err)
	}

	for _, peer := range s.Neighbors {

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
			log.Warnf("Can't add peer %s, Error: %s", peer.Identity, err)
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

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedPeers})
}

func removeNeighbors(i interface{}, c *gin.Context, _ <-chan struct{}) {
	e := ErrorReturn{}
	query := &RemoveNeighbors{}

	removedNeighbors := 0

	if err := mapstructure.Decode(i, query); err != nil {
		e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	removed := false

	var configNeighbors []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configNeighbors); err != nil {
		log.Warn(err)
	}

	peers := peering.Manager().PeerInfos()
	for _, uri := range query.Uris {
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
						e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
						c.JSON(http.StatusInternalServerError, e)
						return
					}
					removedNeighbors++
				}
			} else {
				// Remove unconnected neighbor
				if strings.EqualFold(p.Address, uri) {
					err := peering.Manager().Remove(uri)
					if err != nil {
						e.Error = fmt.Sprintf("%v: %v", ErrInternalError, err)
						c.JSON(http.StatusInternalServerError, e)
						return
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

	c.JSON(http.StatusOK, RemoveNeighborsReturn{RemovedNeighbors: uint(removedNeighbors)})
}

func getNeighbors(i interface{}, c *gin.Context, _ <-chan struct{}) {
	c.JSON(http.StatusOK, GetNeighborsReturn{Neighbors: peering.Manager().PeerInfos()})
}
