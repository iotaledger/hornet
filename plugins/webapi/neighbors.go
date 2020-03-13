package webapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("addNeighbors", addNeighbors, implementedAPIcalls)
	addEndpoint("removeNeighbors", removeNeighbors, implementedAPIcalls)
	addEndpoint("getNeighbors", getNeighbors, implementedAPIcalls)
}

func addNeighbors(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

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
	addedNeighbors := 0

	preferIPv6 := config.NodeConfig.GetBool(config.CfgNetPreferIPv6)

	if err := mapstructure.Decode(i, an); err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	added := false

	configNeighbors := []gossip.NeighborConfig{}
	if err := config.NeighborsConfig.UnmarshalKey(config.CfgNeighbors, &configNeighbors); err != nil {
		log.Error(err)
	}

	for _, uri := range an.Uris {

		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		} else if strings.Contains(uri, "://") {
			continue
		}

		contains := false
		for _, cn := range configNeighbors {
			if cn.Identity == uri {
				contains = true
				break
			}
		}

		if !contains {
			configNeighbors = append(configNeighbors, gossip.NeighborConfig{
				Identity:   uri,
				Alias:      uri,
				PreferIPv6: preferIPv6,
			})
			added = true
		}

		if err := gossip.AddNeighbor(uri, preferIPv6, uri); err != nil {
			log.Warnf("Can't add neighbor %s, Error: %s", uri, err)
		} else {
			addedNeighbors++
			log.Infof("Added neighbor: %s", uri)
		}
	}

	if added {
		config.DenyNeighborsConfigHotReload()
		config.NeighborsConfig.Set(config.CfgNeighbors, configNeighbors)
		config.NeighborsConfig.WriteConfig()
		config.AllowNeighborsConfigHotReload()
	}

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedNeighbors})
}

func addNeighborsWithAlias(s *AddNeighborsHornet, c *gin.Context) {
	addedNeighbors := 0

	added := false

	configNeighbors := []gossip.NeighborConfig{}
	if err := config.NeighborsConfig.UnmarshalKey(config.CfgNeighbors, &configNeighbors); err != nil {
		log.Error(err)
	}

	for _, neighbor := range s.Neighbors {

		if strings.Contains(neighbor.Identity, "tcp://") {
			neighbor.Identity = neighbor.Identity[6:]
		} else if strings.Contains(neighbor.Identity, "://") {
			continue
		}

		contains := false
		for _, cn := range configNeighbors {
			if cn.Identity == neighbor.Identity {
				contains = true
				break
			}
		}

		if !contains {
			configNeighbors = append(configNeighbors, gossip.NeighborConfig{
				Identity:   neighbor.Identity,
				Alias:      neighbor.Alias,
				PreferIPv6: neighbor.PreferIPv6,
			})
			added = true
		}

		if err := gossip.AddNeighbor(neighbor.Identity, neighbor.PreferIPv6, neighbor.Alias); err != nil {
			log.Warnf("Can't add neighbor %s, Error: %s", neighbor.Identity, err)
		} else {
			addedNeighbors++
			log.Infof("Added neighbor: %s", neighbor.Identity)
		}
	}

	if added {
		config.DenyNeighborsConfigHotReload()
		config.NeighborsConfig.Set(config.CfgNeighbors, configNeighbors)
		config.NeighborsConfig.WriteConfig()
		config.AllowNeighborsConfigHotReload()
	}

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedNeighbors})
}

func removeNeighbors(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

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

	configNeighbors := []gossip.NeighborConfig{}
	if err := config.NeighborsConfig.UnmarshalKey(config.CfgNeighbors, &configNeighbors); err != nil {
		log.Error(err)
	}

	nb := gossip.GetNeighbors()
	for _, uri := range rn.Uris {
		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		}
		for _, n := range nb {

			for i, cn := range configNeighbors {
				if strings.EqualFold(cn.Identity, uri) {
					removed = true

					// Delete item
					configNeighbors[i] = configNeighbors[len(configNeighbors)-1]
					configNeighbors = configNeighbors[:len(configNeighbors)-1]
					break
				}
			}

			// Remove connected neighbor
			if n.Neighbor != nil {
				if strings.EqualFold(n.Neighbor.Identity, uri) || strings.EqualFold(n.DomainWithPort, uri) {
					err := gossip.RemoveNeighbor(uri)
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
				if strings.EqualFold(n.Address, uri) {
					err := gossip.RemoveNeighbor(uri)
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
		config.DenyNeighborsConfigHotReload()
		config.NeighborsConfig.Set(config.CfgNeighbors, configNeighbors)
		config.NeighborsConfig.WriteConfig()
		config.AllowNeighborsConfigHotReload()
	}

	c.JSON(http.StatusOK, RemoveNeighborsReturn{RemovedNeighbors: uint(removedNeighbors)})
}

func getNeighbors(i interface{}, c *gin.Context, abortSignal <-chan struct{}) {

	nb := &GetNeighborsReturn{}
	nb.Neighbors = gossip.GetNeighbors()
	c.JSON(http.StatusOK, nb)
}
