// Package framework provides integration test functionality for Hornet with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"errors"
	"fmt"
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

// CreateStaticNetwork creates a network made out of statically peered nodes by the given layout.
// The first node is initialized with the Coordinator plugin enabled.
func (f *Framework) CreateStaticNetwork(name string, layout StaticPeeringLayout) (*StaticNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeStatic, f.tester)
	if err != nil {
		return nil, err
	}

	if len(layout) == 0 {
		return nil, ErrLayoutEmpty
	}

	/*
		// precompute the container names to define the static peers within the config
		// at startup without having to rely on addNeighbors. this ensures that we
		// don't encounter any race condition if we addNeighbors() too fast.
		var containerNames []string
		for i := 0; i < len(layout); i++ {
			containerNames = append(containerNames, fmt.Sprintf("%s-%s%d", name, containerNameReplica, i))
		}
	*/

	for i := 0; i < len(layout); i++ {
		cfg := DefaultConfig()
		cfg.Network.AcceptAnyConnection = true
		cfg.Network.MaxPeers = len(layout)
		// make the first node the coordinator
		if i == 0 {
			cfg.Coordinator.Bootstrap = true
			cfg.Coordinator.RunAsCoo = true
			cfg.Plugins.Enabled = append(cfg.Plugins.Enabled, "Coordinator")
			cfg.Envs = append(cfg.Envs, fmt.Sprintf("COO_SEED=%s", cfg.Coordinator.Seed))
		}
		// disable autopeering
		cfg.Plugins.Disabled = append(cfg.Plugins.Disabled, "autopeering")
		// define static peers
		peers, has := layout[i]
		if !has {
			return nil, fmt.Errorf("%w: %d", ErrNodeMissingInLayout, i)
		}
		if len(peers) == 0 {
			return nil, fmt.Errorf("%w: %d", ErrNoStaticPeers, i)
		}
		//var staticPeers []string
		for peerID := range peers {
			if peerID == i {
				return nil, fmt.Errorf("%w: id %d", ErrSelfPeering, i)
			}
			if _, has := layout[peerID]; !has {
				return nil, fmt.Errorf("%w: %d can't peer to undefined peer %d", ErrNodeMissingInLayout, i, peerID)
			}
			//staticPeers = append(staticPeers, fmt.Sprintf("%s:15600", containerNames[peerID]))
		}
		//cfg.Network.StaticPeers = staticPeers
		if _, err = network.CreatePeer(cfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := network.AwaitOnline(ctx); err != nil {
		return nil, err
	}

	time.Sleep(1 * time.Second)

	// manually connect peers to each other
	for i := 0; i < len(layout); i++ {
		peers := layout[i]
		node := network.Nodes[i]
		var uris []string
		for peerIndex := range peers {
			peer := network.Nodes[peerIndex]
			alreadyPeered := layout[peerIndex][i]
			if alreadyPeered {
				continue
			}
			uris = append(uris, fmt.Sprintf("tcp://%s:15600", peer.IP))
			layout[peerIndex][i] = true
		}
		if _, err := node.WebAPI.AddNeighbors(uris...); err != nil {
			return nil, fmt.Errorf("%w: couldn't add peers %v", err, uris)
		}
	}

	staticNet := &StaticNetwork{Network: network, layout: layout}
	return staticNet, nil
}

// CreateAutopeeredNetwork creates a network consisting out of peersCount nodes.
// It waits for the nodes to autopeer until the minimum neighbors criteria is met for every node.
func (f *Framework) CreateAutopeeredNetwork(name string, peerCount int, minimumNeighbors int) (*AutopeeredNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeAutopeered, f.tester)
	if err != nil {
		return nil, err
	}

	autoNetwork := &AutopeeredNetwork{Network: network}
	if err := autoNetwork.createEntryNode(); err != nil {
		return nil, err
	}

	// create Hornet nodes
	for i := 0; i < peerCount; i++ {
		cfg := DefaultConfig()
		if i == 0 {
			cfg.Coordinator.Bootstrap = true
			cfg.Coordinator.RunAsCoo = true
		}
		if _, err = network.CreatePeer(cfg); err != nil {
			return nil, err
		}
	}

	// wait until containers are fully started
	time.Sleep(1 * time.Second)
	if err := autoNetwork.AwaitPeering(minimumNeighbors); err != nil {
		return nil, err
	}

	return autoNetwork, nil
}

// CreateNetworkWithPartitions creates a network consisting out of partitions that contain peerCount nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
// The entry node is reachable by all nodes at all times.
func (f *Framework) CreateNetworkWithPartitions(name string, peerCount, partitions, minimumNeighbors int) (*AutopeeredNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeAutopeered, f.tester)
	if err != nil {
		return nil, err
	}

	autoNetwork := &AutopeeredNetwork{Network: network}
	if err := autoNetwork.createEntryNode(); err != nil {
		return nil, err
	}

	if err = autoNetwork.createEntryNode(); err != nil {
		return nil, err
	}

	// block all traffic from/to entry node
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
			cfg.Coordinator.Bootstrap = true
			cfg.Coordinator.RunAsCoo = true
		}
		if _, err = network.CreatePeer(cfg); err != nil {
			return nil, err
		}
	}

	// wait until containers are fully started
	time.Sleep(2 * time.Second)

	// create partitions
	chunkSize := peerCount / partitions
	var end int
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
	if err := pumbaEntryNode.Stop(); err != nil {
		return nil, err
	}

	logs, err := pumbaEntryNode.Logs()
	if err != nil {
		return nil, err
	}

	if err := createLogFile(pumbaEntryNodeName, logs); err != nil {
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
