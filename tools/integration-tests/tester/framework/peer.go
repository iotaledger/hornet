package framework

import (
	"fmt"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/iota.go/api"
)

// Peer represents a Hornet node inside the Docker network.
type Peer struct {
	// Name of the node derived from the container and hostname.
	name string

	// the IP address of this node within the network.
	ip string

	// The autopeering identity of the peer.
	*identity.Identity

	// Web API instance used to communicate with the node.
	*api.API

	// the DockerContainer that this peer is running in
	*DockerContainer

	neighbors []api.Neighbor
}

// newPeer creates a new instance of Peer with the given information.
// dockerContainer needs to be started in order to determine the container's (and therefore peer's) IP correctly.
func newPeer(name string, identity *identity.Identity, dockerContainer *DockerContainer, network *Network) (*Peer, error) {
	// after container is started we can get its IP
	ip, err := dockerContainer.IP(network.name)
	if err != nil {
		return nil, err
	}

	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{URI: getWebAPIBaseURL(name)})
	if err != nil {
		return nil, fmt.Errorf("can't instantiate Web API: %w", err)
	}

	return &Peer{
		name:            name,
		ip:              ip,
		Identity:        identity,
		API:             iotaAPI,
		DockerContainer: dockerContainer,
	}, nil
}

func (p *Peer) String() string {
	return fmt.Sprintf("Peer:{%s, %s, %s, %d}", p.name, p.ID().String(), p.APIURI(), p.TotalNeighbors())
}

// APIURI returns the URL under which this peer's web API is accessible.
func (p *Peer) APIURI() string {
	return getWebAPIBaseURL(p.name)
}

// TotalNeighbors returns the total number of neighbors the peer has.
func (p *Peer) TotalNeighbors() int {
	return len(p.neighbors)
}

// SetNeighbors sets the neighbors of the peer accordingly.
func (p *Peer) SetNeighbors(neighbors []api.Neighbor) {
	p.neighbors = make([]api.Neighbor, len(neighbors))
	copy(p.neighbors, neighbors)
}
