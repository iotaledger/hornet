package webapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/gohornet/hornet/plugins/gossip"
)

func init() {
	addEndpoint("addNeighbors", addNeighbors, implementedAPIcalls)
	addEndpoint("removeNeighbors", removeNeighbors, implementedAPIcalls)
	addEndpoint("getNeighbors", getNeighbors, implementedAPIcalls)
}

func addNeighbors(i interface{}, c *gin.Context) {
	an := &AddNeighbors{}
	e := ErrorReturn{}
	addedNeighbors := 0

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

		if err := gossip.AddNeighbor(uri); err != nil {
			log.Warningf("Can't add neighbor %s, Error: %s", uri, err)
		} else {
			addedNeighbors++
			log.Infof("Added neighbor: %s", uri)
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
				if strings.ToLower(n.Neighbor.Identity) == strings.ToLower(uri) || strings.ToLower(n.Address) == strings.ToLower(uri) {
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
				if strings.ToLower(n.Address) == strings.ToLower(uri) {
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
