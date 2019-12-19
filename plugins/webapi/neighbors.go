package webapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/plugins/gossip"
	"github.com/iotaledger/hive.go/parameter"
	"github.com/mitchellh/mapstructure"
)

func init() {
	addEndpoint("addNeighbors", addNeighbors, implementedAPIcalls)
	addEndpoint("removeNeighbors", removeNeighbors, implementedAPIcalls)
	addEndpoint("getNeighbors", getNeighbors, implementedAPIcalls)
}

func addNeighbors(i interface{}, c *gin.Context) {

	// Check if HORNET style addNeighbors call was made
	han := &AddNeighborsHornet{}
	if err := mapstructure.Decode(i, han); err == nil {
		addNeighborsWithAlias(han, c)
		return
	}

	an := &AddNeighbors{}
	e := ErrorReturn{}
	addedNeighbors := 0

	preferIPv6 := parameter.NodeConfig.GetBool("network.preferIPv6")

	if err := mapstructure.Decode(i, an); err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	for _, uri := range an.Uris {

		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		} else if strings.Contains(uri, "://") {
			continue
		}

		if err := gossip.AddNeighbor(uri, preferIPv6); err != nil {
			log.Warningf("Can't add neighbor %s, Error: %s", uri, err)
		} else {
			addedNeighbors++
			log.Infof("Added neighbor: %s", uri)
		}
	}

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedNeighbors})
}

func addNeighborsWithAlias(s *AddNeighborsHornet, c *gin.Context) {
	addedNeighbors := 0

	for _, neighbor := range s.Neighbors {

		if strings.Contains(neighbor.Identity, "tcp://") {
			neighbor.Identity = neighbor.Identity[6:]
		} else if strings.Contains(neighbor.Identity, "://") {
			continue
		}

		// TODO: Add alias (uri.Alias)

		if err := gossip.AddNeighbor(neighbor.Identity, neighbor.PreferIPv6); err != nil {
			log.Warningf("Can't add neighbor %s, Error: %s", neighbor.Identity, err)
		} else {
			addedNeighbors++
			log.Infof("Added neighbor: %s", neighbor.Identity)
		}
	}

	c.JSON(http.StatusOK, AddNeighborsResponse{AddedNeighbors: addedNeighbors})
}

func removeNeighbors(i interface{}, c *gin.Context) {

	rn := &RemoveNeighbors{}
	e := ErrorReturn{}
	removedNeighbors := 0
	err := mapstructure.Decode(i, rn)
	if err != nil {
		e.Error = "Internal error"
		c.JSON(http.StatusInternalServerError, e)
		return
	}

	nb := gossip.GetNeighbors()
	for _, uri := range rn.Uris {
		if strings.Contains(uri, "tcp://") {
			uri = uri[6:]
		}
		for _, n := range nb {
			// Remove connected neighbor
			if n.Neighbor != nil {
				if strings.EqualFold(n.Neighbor.Identity, uri) || strings.EqualFold(n.Address, uri) {
					err := gossip.RemoveNeighbor(uri)
					if err != nil {
						log.Errorf("Can't remove neighbor, Error: %s", err.Error())
						e.Error = "Internal error"
						c.JSON(http.StatusInternalServerError, e)
						return
					}
					removedNeighbors++
				}
				// Remove unconnected neighbor
			} else {
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

	c.JSON(http.StatusOK, RemoveNeighborsReturn{RemovedNeighbors: uint(removedNeighbors)})
}

func getNeighbors(i interface{}, c *gin.Context) {

	nb := &GetNeighborsReturn{}
	nb.Neighbors = gossip.GetNeighbors()
	c.JSON(http.StatusOK, nb)
}
