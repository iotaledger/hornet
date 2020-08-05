package framework

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/iota.go/api"
)

// Node represents a Hornet node inside the Docker network.
type Node struct {
	// Name of the node derived from the container and hostname.
	Name string
	// the IP address of this node within the network.
	IP string
	// The configuration with which the node was started.
	Config *NodeConfig
	// The autopeering identity of the peer.
	*identity.Identity
	// The iota.go web API instance used to communicate with the node.
	WebAPI *api.API
	// The more specific web API providing more information for debugging purposes.
	DebugWebAPI *WebAPI
	// The profiler instance.
	Profiler
	// The DockerContainer that this peer is running in
	*DockerContainer
	// The Neighbors of the peer.
	neighbors []*peer.Info
}

// newNode creates a new instance of Node with the given information.
// dockerContainer needs to be started in order to determine the container's (and therefore peer's) IP correctly.
func newNode(name string, identity *identity.Identity, cfg *NodeConfig, dockerContainer *DockerContainer, network *Network) (*Node, error) {
	// after container is started we can get its IP
	ip, err := dockerContainer.IP(network.Name)
	if err != nil {
		return nil, err
	}

	iotaAPI, err := api.ComposeAPI(api.HTTPClientSettings{URI: getWebAPIBaseURL(ip)})
	if err != nil {
		return nil, fmt.Errorf("can't instantiate Web API: %w", err)
	}

	testWebAPI := NewWebAPI(getWebAPIBaseURL(ip))

	return &Node{
		Name: name,
		Profiler: Profiler{
			pprofURI:     fmt.Sprintf("http://%s:6060", ip),
			websocketURI: fmt.Sprintf("ws://%s:8081/ws", ip),
			targetName:   name,
			Client: http.Client{
				Timeout: 2 * time.Minute,
			},
		},
		IP:              ip,
		Config:          cfg,
		Identity:        identity,
		WebAPI:          iotaAPI,
		DebugWebAPI:     testWebAPI,
		DockerContainer: dockerContainer,
	}, nil
}

func (p *Node) String() string {
	return fmt.Sprintf("Node:{%s, %s, %s, %d}", p.Name, p.ID().String(), p.APIURI(), p.TotalNeighbors())
}

// APIURI returns the URL under which this node's web API is accessible.
func (p *Node) APIURI() string {
	return getWebAPIBaseURL(p.Name)
}

// TotalNeighbors returns the total number of neighbors the peer has.
func (p *Node) TotalNeighbors() int {
	return len(p.neighbors)
}

// SetNeighbors sets the neighbors of the peer accordingly.
func (p *Node) SetNeighbors(peers []*peer.Info) {
	p.neighbors = make([]*peer.Info, len(peers))
	copy(p.neighbors, peers)
}
