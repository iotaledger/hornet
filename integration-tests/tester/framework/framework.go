// Package framework provides integration test functionality for Hornet with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

var (
	once     sync.Once
	instance *Framework

	ErrNodeMissingInLayout   = errors.New("node is missing in layout")
	ErrSelfPeering           = errors.New("a node can not peer to itself")
	ErrNoStaticPeers         = errors.New("nodes must have static nodes")
	ErrLayoutEmpty           = errors.New("layout must not be empty")
	ErrNodesDidNotPeerInTime = errors.New("nodes did not peer in time")
	ErrNodesDidNotSyncInTime = errors.New("nodes did not sync in time")
	ErrNodesNotOnlineInTime  = errors.New("nodes did not become online in time")
)

// Framework is a wrapper that provides the integration testing functionality.
type Framework struct {
	tester       *DockerContainer
	dockerClient *client.Client
}

// Instance returns the singleton Framework instance.
func Instance() (f *Framework, err error) {
	once.Do(func() {
		f, err = newFramework()
		instance = f
	})

	return instance, err
}

// newFramework creates a new instance of Framework, creates a DockerClient
// and creates a DockerContainer for the tester container where the tests are running in.
func newFramework() (*Framework, error) {
	dockerClient, err := newDockerClient()
	if err != nil {
		return nil, err
	}

	tester, err := NewDockerContainerFromExisting(dockerClient, containerNameTester)
	if err != nil {
		return nil, err
	}

	f := &Framework{
		dockerClient: dockerClient,
		tester:       tester,
	}

	return f, nil
}

// CfgOverrideFunc is a function which overrides configuration values.
type CfgOverrideFunc func(index int, cfg *NodeConfig)

// CreateStaticNetwork creates a network made out of statically peered nodes by the given layout.
// The first node is initialized with the Coordinator plugin enabled.
func (f *Framework) CreateStaticNetwork(name string, layout StaticPeeringLayout, cfgOverrideF ...CfgOverrideFunc) (*StaticNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeStatic, f.tester)
	if err != nil {
		return nil, err
	}

	if err := layout.Validate(); err != nil {
		return nil, err
	}

	for i := 0; i < len(layout); i++ {
		cfg := DefaultConfig()
		if i == 0 {
			cfg.AsCoo()
		}
		// since we use addNeighbors to peer the nodes with each other, we need to let
		// them accept any connection. we don't define the neighbors on startup to prevent
		// nodes from canceling out each other's connection.
		cfg.Network.AcceptAnyConnection = true
		cfg.Network.MaxPeers = len(layout)
		cfg.Plugins.Disabled = append(cfg.Plugins.Disabled, "autopeering")
		if len(cfgOverrideF) > 0 && cfgOverrideF[0] != nil {
			cfgOverrideF[0](i, cfg)
		}
		if _, err = network.CreateNode(cfg); err != nil {
			return nil, err
		}
	}

	staticNet := &StaticNetwork{Network: network, layout: layout}
	if err := staticNet.ConnectNodes(); err != nil {
		return nil, err
	}
	return staticNet, nil
}

// CreateAutopeeredNetwork creates a network consisting out of peersCount nodes.
// It waits for the nodes to autopeer until the minimum neighbors criteria is met for every node.
func (f *Framework) CreateAutopeeredNetwork(name string, peerCount int, minimumNeighbors int, cfgOverrideF ...CfgOverrideFunc) (*AutopeeredNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeAutopeered, f.tester)
	if err != nil {
		return nil, err
	}

	autoNetwork := &AutopeeredNetwork{Network: network}
	if err := autoNetwork.createEntryNode(); err != nil {
		return nil, err
	}

	for i := 0; i < peerCount; i++ {
		cfg := DefaultConfig()
		if i == 0 {
			cfg.AsCoo()
		}
		if len(cfgOverrideF) > 0 && cfgOverrideF[0] != nil {
			cfgOverrideF[0](i, cfg)
		}
		if _, err = autoNetwork.CreatePeer(cfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := network.AwaitOnline(ctx); err != nil {
		return nil, err
	}

	// await minimum auto. peers
	if err := autoNetwork.AwaitPeering(minimumNeighbors); err != nil {
		return nil, err
	}

	return autoNetwork, nil
}

// CreateNetworkWithPartitions creates a network consisting out of partitions that contain peerCount nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The entry node is reachable by all nodes at all times.
func (f *Framework) CreateNetworkWithPartitions(name string, peerCount, partitions, minimumNeighbors int, cfgOverrideF ...CfgOverrideFunc) (*AutopeeredNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeAutopeered, f.tester)
	if err != nil {
		return nil, err
	}

	autoNetwork := &AutopeeredNetwork{Network: network}
	if err := autoNetwork.createEntryNode(); err != nil {
		return nil, err
	}

	// block all traffic from/to entry node
	log.Println("blocking traffic to entry node...")
	pumbaEntryNodeName := network.PrefixName(containerNameEntryNode) + containerNameSuffixPumba
	pumbaEntryNode, err := autoNetwork.createPumba(
		pumbaEntryNodeName,
		network.PrefixName(containerNameEntryNode),
		strslice.StrSlice{},
	)
	if err != nil {
		return nil, err
	}

	// wait until pumba is started and blocks all traffic
	time.Sleep(5 * time.Second)

	// create peers
	for i := 0; i < peerCount; i++ {
		cfg := DefaultConfig()
		if i == 0 {
			cfg.AsCoo()
		}
		if len(cfgOverrideF) > 0 && cfgOverrideF[0] != nil {
			cfgOverrideF[0](i, cfg)
		}
		if _, err = autoNetwork.CreatePeer(cfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := network.AwaitOnline(ctx); err != nil {
		return nil, err
	}

	// create partitions
	chunkSize := peerCount / partitions
	var end int
	log.Printf("partitioning nodes from each other (%d partitions, %d nodes per partition)...", partitions, chunkSize)
	for i := 0; end < peerCount; i += chunkSize {
		end = i + chunkSize
		// last partitions takes the rest
		if i/chunkSize == partitions-1 {
			end = peerCount
		}
		if _, err = autoNetwork.createPartition(network.Nodes[i:end]); err != nil {
			return nil, err
		}
	}

	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(5 * time.Second)

	// delete pumba for entry node
	log.Println("unblocking traffic to entry node...")
	if err := pumbaEntryNode.Stop(); err != nil {
		return nil, err
	}

	logs, err := pumbaEntryNode.Logs()
	if err != nil {
		return nil, err
	}

	if err := createContainerLogFile(pumbaEntryNodeName, logs); err != nil {
		return nil, err
	}

	if err := pumbaEntryNode.Remove(); err != nil {
		return nil, err
	}

	if err := autoNetwork.AwaitPeering(minimumNeighbors); err != nil {
		return nil, err
	}

	return autoNetwork, nil
}
