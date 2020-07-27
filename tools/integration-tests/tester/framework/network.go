package framework

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

// Network represents a complete Hornet network within Docker.
// Including an entry node and arbitrary many peers.
type Network struct {
	// The ID of the network.
	id string

	// The name of the network.
	name string

	// Peers the peers within the network.
	peers []*Peer

	// The tester docker container executing the tests.
	tester *DockerContainer

	// The entry node docker container.
	entryNode *DockerContainer

	// The peer identity of the entry node.
	entryNodeIdentity *identity.Identity

	// The partitions of which this network is made up of.
	partitions []*Partition

	// The docker client used to communicate with the docker daemon.
	dockerClient *client.Client
}

// newNetwork returns a Network instance, creates its underlying Docker network and adds the tester container to the network.
func newNetwork(dockerClient *client.Client, name string, tester *DockerContainer) (*Network, error) {
	// create Docker network
	resp, err := dockerClient.NetworkCreate(context.Background(), name, types.NetworkCreate{})
	if err != nil {
		return nil, err
	}

	// the tester container needs to join the Docker network in order to communicate with the peers
	if err := tester.ConnectToNetwork(resp.ID); err != nil {
		return nil, err
	}

	return &Network{
		id:           resp.ID,
		name:         name,
		tester:       tester,
		dockerClient: dockerClient,
	}, nil
}

// createEntryNode creates the network's entry node.
func (n *Network) createEntryNode() error {
	// create identity
	publicKey, privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return err
	}

	n.entryNodeIdentity = identity.New(publicKey)
	seed := privateKey.Seed().String()

	// create entry node container
	n.entryNode = NewDockerContainer(n.dockerClient)
	if err = n.entryNode.CreateHornetEntryNode(n.namePrefix(containerNameEntryNode), seed); err != nil {
		return err
	}
	if err = n.entryNode.ConnectToNetwork(n.id); err != nil {
		return err
	}
	if err = n.entryNode.Start(); err != nil {
		return err
	}

	return nil
}

// CreatePeer creates a new Hornet node in the network and returns it.
func (n *Network) CreatePeer(c NodeConfig) (*Peer, error) {
	name := n.namePrefix(fmt.Sprintf("%s%d", containerNameReplica, len(n.peers)))

	// create identity
	publicKey, privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return nil, err
	}
	seed := privateKey.Seed().String()

	config := c
	config.Name = name
	config.AutopeeringSeed = seed
	config.EntryNodeHost = n.namePrefix(containerNameEntryNode)
	config.EntryNodePublicKey = n.entryNodePublicKey()
	config.DisabledPlugins = disabledPluginsPeer
	config.SnapshotFilePath = snapshotFilePath

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	err = container.CreateHornetPeer(config)
	if err != nil {
		return nil, err
	}
	err = container.ConnectToNetwork(n.id)
	if err != nil {
		return nil, err
	}
	err = container.Start()
	if err != nil {
		return nil, err
	}

	peer, err := newPeer(name, identity.New(publicKey), container, n)
	if err != nil {
		return nil, err
	}
	n.peers = append(n.peers, peer)
	return peer, nil
}

// Shutdown creates logs and removes network and containers.
// Should always be called when a network is not needed anymore!
func (n *Network) Shutdown() error {
	// stop containers
	err := n.entryNode.Stop()
	if err != nil {
		return err
	}
	for _, p := range n.peers {
		if err := p.Stop(); err != nil {
			return err
		}
	}

	// delete all partitions
	if err := n.DeletePartitions(); err != nil {
		return err
	}

	// retrieve logs
	logs, err := n.entryNode.Logs()
	if err != nil {
		return err
	}
	if err := createLogFile(n.namePrefix(containerNameEntryNode), logs); err != nil {
		return err
	}
	for _, p := range n.peers {
		logs, err = p.Logs()
		if err != nil {
			return err
		}
		err = createLogFile(p.name, logs)
		if err != nil {
			return err
		}
	}

	// save exit status of containers to check at end of shutdown process
	exitStatus := make(map[string]int, len(n.peers)+1)
	exitStatus[containerNameEntryNode], err = n.entryNode.ExitStatus()
	if err != nil {
		return err
	}
	for _, p := range n.peers {
		exitStatus[p.name], err = p.ExitStatus()
		if err != nil {
			return err
		}
	}

	// remove containers
	if err := n.entryNode.Remove(); err != nil {
		return err
	}
	for _, p := range n.peers {
		err = p.Remove()
		if err != nil {
			return err
		}
	}

	// disconnect tester from network otherwise the network can't be removed
	if err := n.tester.DisconnectFromNetwork(n.id); err != nil {
		return err
	}

	// remove network
	if err := n.dockerClient.NetworkRemove(context.Background(), n.id); err != nil {
		return err
	}

	// check exit codes of containers
	for name, status := range exitStatus {
		if status != exitStatusSuccessful {
			return fmt.Errorf("container %s exited with code %d", name, status)
		}
	}

	return nil
}

// WaitForAutopeering waits until all peers have reached the minimum amount of neighbors.
// Returns error if this minimum is not reached after autopeeringMaxTries.
func (n *Network) WaitForAutopeering(minimumNeighbors int) error {
	log.Printf("Waiting for autopeering...\n")
	defer log.Printf("Waiting for autopeering... done\n")

	if minimumNeighbors == 0 {
		return nil
	}

	for i := autopeeringMaxTries; i > 0; i-- {

		for _, p := range n.peers {
			resp, err := p.GetNeighbors()
			if err != nil {
				log.Printf("request error: %v\n", err)
				continue
			}
			p.SetNeighbors(resp)
		}

		// verify neighbor requirement
		min := 100
		total := 0
		for _, p := range n.peers {
			neighbors := p.TotalNeighbors()
			if neighbors < min {
				min = neighbors
			}
			total += neighbors
		}
		if min >= minimumNeighbors {
			log.Printf("Neighbors: min=%d avg=%.2f\n", min, float64(total)/float64(len(n.peers)))
			return nil
		}

		log.Println("Not done yet. Try again in 5 seconds...")
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("autopeering not successful")
}

// namePrefix returns the suffix prefixed with the name.
func (n *Network) namePrefix(suffix string) string {
	return fmt.Sprintf("%s-%s", n.name, suffix)
}

// entryNodePublicKey returns the entry node's public key encoded as base58
func (n *Network) entryNodePublicKey() string {
	return n.entryNodeIdentity.PublicKey().String()
}

// Peers returns all available peers in the network.
func (n *Network) Peers() []*Peer {
	return n.peers
}

// RandomPeer returns a random peer out of the list of peers.
func (n *Network) RandomPeer() *Peer {
	return n.peers[rand.Intn(len(n.peers))]
}

// createPumba creates and starts a Pumba Docker container.
func (n *Network) createPumba(name string, containerName string, targetIPs []string) (*DockerContainer, error) {
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreatePumba(name, containerName, targetIPs); err != nil {
		return nil, err
	}
	if err := container.Start(); err != nil {
		return nil, err
	}

	return container, nil
}

// createPartition creates a partition with the given peers.
// It starts a Pumba container for every peer that blocks traffic to all other partitions.
func (n *Network) createPartition(peers []*Peer) (*Partition, error) {
	peersMap := make(map[string]*Peer)
	for _, peer := range peers {
		peersMap[peer.ID().String()] = peer
	}

	// block all traffic to all other peers except in the current partition
	var targetIPs []string
	for _, peer := range n.peers {
		if _, ok := peersMap[peer.ID().String()]; ok {
			continue
		}
		targetIPs = append(targetIPs, peer.ip)
	}

	partitionName := n.namePrefix(fmt.Sprintf("partition_%d-", len(n.partitions)))

	// create pumba container for every peer in the partition
	pumbas := make([]*DockerContainer, len(peers))
	for i, p := range peers {
		name := partitionName + p.name + containerNameSuffixPumba
		pumba, err := n.createPumba(name, p.name, targetIPs)
		if err != nil {
			return nil, err
		}
		pumbas[i] = pumba
		time.Sleep(1 * time.Second)
	}

	partition := &Partition{
		name:     partitionName,
		peers:    peers,
		peersMap: peersMap,
		pumbas:   pumbas,
	}
	n.partitions = append(n.partitions, partition)

	return partition, nil
}

// DeletePartitions deletes all partitions of the network.
// All nodes can communicate with the full network again.
func (n *Network) DeletePartitions() error {
	for _, p := range n.partitions {
		err := p.deletePartition()
		if err != nil {
			return err
		}
	}
	n.partitions = nil
	return nil
}

// Partitions returns the network's partitions.
func (n *Network) Partitions() []*Partition {
	return n.partitions
}

// Split splits the existing network in given partitions.
func (n *Network) Split(partitions ...[]*Peer) error {
	for _, peers := range partitions {
		_, err := n.createPartition(peers)
		if err != nil {
			return err
		}
	}
	// wait until pumba containers are started and block traffic between partitions
	time.Sleep(5 * time.Second)

	return nil
}

// Partition represents a network partition.
// It contains its peers and the corresponding Pumba instances that block all traffic to peers in other partitions.
type Partition struct {
	name     string
	peers    []*Peer
	peersMap map[string]*Peer
	pumbas   []*DockerContainer
}

// Peers returns the partition's peers.
func (p *Partition) Peers() []*Peer {
	return p.peers
}

// PeersMap returns the partition's peers map.
func (p *Partition) PeersMap() map[string]*Peer {
	return p.peersMap
}

func (p *Partition) String() string {
	return fmt.Sprintf("Partition{%s, %s}", p.name, p.peers)
}

// deletePartition deletes a partition, all its Pumba containers and creates logs for them.
func (p *Partition) deletePartition() error {
	// stop containers
	for _, pumba := range p.pumbas {
		if err := pumba.Stop(); err != nil {
			return err
		}
	}

	// retrieve logs
	for i, pumba := range p.pumbas {
		logs, err := pumba.Logs()
		if err != nil {
			return err
		}
		err = createLogFile(fmt.Sprintf("%s%s", p.name, p.peers[i].name), logs)
		if err != nil {
			return err
		}
	}

	for _, pumba := range p.pumbas {
		if err := pumba.Remove(); err != nil {
			return err
		}
	}

	return nil
}
