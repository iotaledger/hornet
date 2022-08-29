package framework

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"

	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/nodeclient"
)

// Node represents a HORNET node inside the Docker network.
type Node struct {
	// Name of the node derived from the container and hostname.
	Name string
	// the IP address of this node within the network.
	IP string
	// The configuration with which the node was started.
	Config *AppConfig
	// The libp2p identifier of the peer.
	ID peer.ID
	// The iota.go web API instance with additional information used to communicate with the node.
	DebugNodeAPIClient *DebugNodeAPIClient
	// The profiler instance.
	Profiler
	// The DockerContainer that this peer is running in
	*DockerContainer
	// The Peers of the peer.
	peers []*nodeclient.PeerResponse
}

type INXExtension struct {
	Name string
	IP   string
	*DockerContainer
}

// newNode creates a new instance of Node with the given information.
// dockerContainer needs to be started in order to determine the container's (and therefore peer's) IP correctly.
func newNode(name string, id peer.ID, cfg *AppConfig, dockerContainer *DockerContainer, network *Network) (*Node, error) {
	// after container is started we can get its IP
	ip, err := dockerContainer.IP(network.Name)
	if err != nil {
		return nil, err
	}

	debugNodeAPI := NewDebugNodeAPIClient(getNodeAPIBaseURL(ip))

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
		IP:                 ip,
		Config:             cfg,
		ID:                 id,
		DebugNodeAPIClient: debugNodeAPI,
		DockerContainer:    dockerContainer,
	}, nil
}

func (p *Node) String() string {
	return fmt.Sprintf("Node:{%s, %s, %s, %d}", p.Name, p.ID, p.APIURI(), p.TotalPeers())
}

// APIURI returns the URL under which this node's web API is accessible.
func (p *Node) APIURI() string {
	return getNodeAPIBaseURL(p.Name)
}

// TotalPeers returns the total number of peers the peer has.
func (p *Node) TotalPeers() int {
	return len(p.peers)
}

// SetPeers sets the peers of the peer accordingly.
func (p *Node) SetPeers(peers []*nodeclient.PeerResponse) {
	p.peers = make([]*nodeclient.PeerResponse, len(peers))
	copy(p.peers, peers)
}

// Spam spams zero value transactions on the node.
// Returns the number of spammed transactions.
func (p *Node) Spam(dur time.Duration, parallelism int) (int32, error) {

	start := time.Now()
	end := start.Add(dur)

	var wg sync.WaitGroup
	wg.Add(parallelism)

	var spamErr error
	var spammed int32
	for j := 0; j < parallelism; j++ {

		time.Sleep(250 * time.Millisecond)

		go func() {
			defer wg.Done()
			for time.Now().Before(end) {

				nodeInfo, err := p.DebugNodeAPIClient.Info(context.Background())
				if err != nil {
					spamErr = err

					return
				}
				protoParams := &nodeInfo.Protocol

				txCount := atomic.AddInt32(&spammed, 1)
				data := fmt.Sprintf("Count: %06d, Timestamp: %s", txCount, time.Now().Format(time.RFC3339))
				iotaBlock := &iotago.Block{
					ProtocolVersion: protoParams.Version,
					Payload: &iotago.TaggedData{
						Tag:  []byte("SPAM"),
						Data: []byte(data)},
				}

				if _, err := p.DebugNodeAPIClient.SubmitBlock(context.Background(), iotaBlock, protoParams); err != nil {
					spamErr = err

					return
				}
			}
		}()
	}
	wg.Wait()

	return spammed, spamErr
}
