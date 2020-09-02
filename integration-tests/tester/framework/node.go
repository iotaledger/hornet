package framework

import (
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gohornet/hornet/pkg/peering/peer"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/checksum"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/trinary"
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

// Spam spams zero value transactions on the node.
// Returns the number of spammed transactions.
func (p *Node) Spam(dur time.Duration, depth int, parallelism int, batchSize ...int) (int32, error) {
	targetAddr, err := checksum.AddChecksum(consts.NullHashTrytes, true, consts.AddressChecksumTrytesSize)
	if err != nil {
		return 0, err
	}
	spamTransfer := []bundle.Transfer{{Address: targetAddr}}
	bndl, err := p.WebAPI.PrepareTransfers(consts.NullHashTrytes, spamTransfer, api.PrepareTransfersOptions{})
	if err != nil {
		return 0, err
	}

	batch := 20
	if len(batchSize) > 0 {
		batch = batchSize[0]
	}

	s := time.Now()
	end := s.Add(dur)
	var spammed int32
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for j := 0; j < parallelism; j++ {
		time.Sleep(250 * time.Millisecond)
		go func() {
			defer wg.Done()
			for time.Now().Before(end) {

				// we use batching for increased throughput
				toSend := make([]trinary.Trytes, batch)
				for i := 0; i < batch; i++ {
					tips, err := p.WebAPI.GetTransactionsToApprove(uint64(depth))
					if err != nil {
						return
					}
					readyTx, err := p.WebAPI.AttachToTangle(tips.TrunkTransaction, tips.BranchTransaction, uint64(p.Config.Coordinator.MWM), bndl)
					if err != nil {
						return
					}
					toSend[i] = readyTx[0]
				}

				if _, err = p.WebAPI.BroadcastTransactions(toSend...); err != nil {
					return
				}

				atomic.AddInt32(&spammed, int32(batch))
			}
		}()
	}
	wg.Wait()
	return spammed, nil
}
