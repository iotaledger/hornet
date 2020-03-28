package webapi

import (
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
	han := &AddNeighborsHornet{}
	if err := mapstructure.Decode(i, han); err == nil {
		if len(han.Neighbors) != 0 {
			addNeighborsWithAlias(han, c)
			return
		}
	}

	an := &AddNeighbors{}
	e := ErrorReturn{}
	addedPeers := 0

	preferIPv6 := config.NodeConfig.GetBool(config.CfgNetPreferIPv6)

	if err := mapstructure.Decode(i, an); err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		log.Error(err)
	}

	for _, uri := range an.Uris {

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
		} else {
			addedPeers++
			log.Infof("added peer: %s", uri)
		}
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
	addedNeighbors := 0

	added := false

	var configPeers []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configPeers); err != nil {
		log.Error(err)
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
		} else {
			addedNeighbors++
			log.Infof("Added peer: %s", peer.Identity)
		}
	}

	if added {
		config.DenyPeeringConfigHotReload()
		config.PeeringConfig.Set(config.CfgPeers, configPeers)
		config.PeeringConfig.WriteConfig()
		config.AllowPeeringConfigHotReload()
	}

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedNeighbors})
}

func removeNeighbors(i interface{}, c *gin.Context, _ <-chan struct{}) {

	rn := &RemoveNeighbors{}
	e := ErrorReturn{}
	removedNeighbors := 0
	err := mapstructure.Decode(i, rn)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	removed := false

	var configNeighbors []config.PeerConfig
	if err := config.PeeringConfig.UnmarshalKey(config.CfgPeers, &configNeighbors); err != nil {
		log.Error(err)
	}

	peers := peering.Manager().PeerInfos()
	for _, uri := range rn.Uris {
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
						log.Errorf("Can't remove neighbor, Error: %s", err.Error())
						e.Error = "Internal error"
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
						log.Errorf("Can't remove neighbor, Error: %s", err.Error())
						e.Error = "Internal error"
						c.JSON(http.StatusInternalServerError, e)
						return
					}
					removedNeighbors++
					log.Infof("Removed neighbor: %s", uri)
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
	nb := &GetNeighborsReturn{}
	nb.Neighbors = peering.Manager().PeerInfos()
	c.JSON(http.StatusOK, nb)
}
