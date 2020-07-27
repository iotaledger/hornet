// Package framework provides integration test functionality for Hornet with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
)

var (
	once     sync.Once
	instance *Framework
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

// CreateNetwork creates and returns a (Docker) Network that contains peerCount Hornet nodes.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
func (f *Framework) CreateNetwork(name string, peerCount int, minimumNeighbors int) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	if err := network.createEntryNode(); err != nil {
		return nil, err
	}

	// create Hornet nodes
	for i := 0; i < peerCount; i++ {
		config := NodeConfig{Coordinator: i == 0}
		if _, err = network.CreatePeer(config); err != nil {
			return nil, err
		}
	}

	// wait until containers are fully started
	time.Sleep(1 * time.Second)
	if err := network.WaitForAutopeering(minimumNeighbors); err != nil {
		return nil, err
	}

	return network, nil
}

// CreateNetworkWithPartitions creates and returns a partitioned network that contains peerCount Hornet nodes per partition.
// It waits for the peers to autopeer until the minimum neighbors criteria is met for every peer.
func (f *Framework) CreateNetworkWithPartitions(name string, peerCount, partitions, minimumNeighbors int) (*Network, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), f.tester)
	if err != nil {
		return nil, err
	}

	if err = network.createEntryNode(); err != nil {
		return nil, err
	}

	// block all traffic from/to entry node
	pumbaEntryNodeName := network.namePrefix(containerNameEntryNode) + containerNameSuffixPumba
	pumbaEntryNode, err := network.createPumba(
		pumbaEntryNodeName,
		network.namePrefix(containerNameEntryNode),
		strslice.StrSlice{},
	)
	if err != nil {
		return nil, err
	}

	// wait until pumba is started and blocks all traffic
	time.Sleep(5 * time.Second)

	// create peers
	for i := 0; i < peerCount; i++ {
		config := NodeConfig{Coordinator: i == 0}
		if _, err = network.CreatePeer(config); err != nil {
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
		_, err = network.createPartition(network.peers[i:end])
		if err != nil {
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

	if err := network.WaitForAutopeering(minimumNeighbors); err != nil {
		return nil, err
	}

	return network, nil
}
