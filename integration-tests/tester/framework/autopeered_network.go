package framework

import (
	"fmt"
	"log"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

// AutopeeredNetwork is a network consisting out of autopeered nodes.
// It contains additionally an entry node.
type AutopeeredNetwork struct {
	*Network
	// The entry node docker container.
	entryNode *DockerContainer
	// The peer identity of the entry node.
	entryNodeIdentity *identity.Identity
	// The partitions of which this network is made up of.
	partitions []*Partition
}

// createEntryNode creates the network's entry node.
func (n *AutopeeredNetwork) createEntryNode() error {
	// create identity
	publicKey, privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return err
	}

	n.entryNodeIdentity = identity.New(publicKey)
	seed := privateKey.Seed().String()

	// create entry node container
	n.entryNode = NewDockerContainer(n.dockerClient)

	cfg := DefaultConfig()
	cfg.Name = n.PrefixName(containerNameEntryNode)
	cfg.Network.RunAsEntryNode = true
	cfg.Network.AutopeeringSeed = seed
	cfg.Plugins.Disabled = disabledPluginsEntryNode

	if err = n.entryNode.CreateNodeContainer(cfg); err != nil {
		return err
	}
	if err = n.entryNode.ConnectToNetwork(n.ID); err != nil {
		return err
	}
	if err = n.entryNode.Start(); err != nil {
		return err
	}

	return nil
}

// AwaitPeering waits until all peers have reached the minimum amount of neighbors.
// Returns error if this minimum is not reached after autopeeringMaxTries.
func (n *AutopeeredNetwork) AwaitPeering(minimumNeighbors int) error {
	log.Printf("waiting for nodes to fulfill min. autopeer criteria (%d)...", minimumNeighbors)

	if minimumNeighbors == 0 {
		return nil
	}

	for i := autopeeringMaxTries; i > 0; i-- {

		for _, p := range n.Nodes {
			resp, err := p.DebugWebAPI.Neighbors()
			if err != nil {
				log.Printf("request error: %v\n", err)
				continue
			}
			p.SetNeighbors(resp)
		}

		min := 100
		total := 0
		for _, p := range n.Nodes {
			neighbors := p.TotalNeighbors()
			if neighbors < min {
				min = neighbors
			}
			total += neighbors
		}

		if min >= minimumNeighbors {
			log.Printf("neighbors: min=%d avg=%.2f\n", min, float64(total)/float64(len(n.Nodes)))
			return nil
		}

		log.Printf("criteria (%d) not fulfilled yet. trying again in 5 seconds...", minimumNeighbors)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("autopeering not successful")
}

// CreatePeer creates a new Hornet node initialized with the right entry node.
func (n *AutopeeredNetwork) CreatePeer(cfg *NodeConfig) (*Node, error) {
	ip, err := n.entryNode.IP(n.Name)
	if err != nil {
		return nil, err
	}
	cfg.Network.EntryNodes = []string{
		fmt.Sprintf("%s@%s:14626", n.entryNodePublicKey(), ip),
	}
	return n.Network.CreateNode(cfg)
}

// Shutdown shuts down the network.
func (n *AutopeeredNetwork) Shutdown() error {
	if err := n.entryNode.Stop(); err != nil {
		return err
	}

	// delete all partitions
	if err := n.DeletePartitions(); err != nil {
		return err
	}

	// persist entry node log, stop it and remove it from the network
	logs, err := n.entryNode.Logs()
	if err != nil {
		return err
	}
	if err := createContainerLogFile(n.PrefixName(containerNameEntryNode), logs); err != nil {
		return err
	}

	entryNodeExitStatus, err := n.entryNode.ExitStatus()
	if err != nil {
		return err
	}

	if err := n.entryNode.Remove(); err != nil {
		return err
	}

	// shutdown the actual network
	if err := n.Network.Shutdown(); err != nil {
		return err
	}

	// check whether the entry node was successfully exited
	if entryNodeExitStatus != exitStatusSuccessful {
		return fmt.Errorf("container %s exited with code %d", containerNameEntryNode, entryNodeExitStatus)
	}

	return nil
}

// entryNodePublicKey returns the entry node's public key encoded as base58
func (n *AutopeeredNetwork) entryNodePublicKey() string {
	return n.entryNodeIdentity.PublicKey().String()
}

// createPumba creates and starts a Pumba Docker container.
func (n *AutopeeredNetwork) createPumba(pumbaContainerName string, targetContainerName string, targetIPs []string) (*DockerContainer, error) {
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreatePumbaContainer(pumbaContainerName, targetContainerName, targetIPs); err != nil {
		return nil, err
	}
	if err := container.Start(); err != nil {
		return nil, err
	}

	return container, nil
}

// createPartition creates a partition with the given peers.
// It starts a Pumba container for every peer that blocks traffic to all other partitions.
func (n *AutopeeredNetwork) createPartition(peers []*Node) (*Partition, error) {
	peersMap := make(map[string]*Node)
	for _, peer := range peers {
		peersMap[peer.ID().String()] = peer
	}

	// block all traffic to all other peers except in the current partition
	var targetIPs []string
	for _, peer := range n.Nodes {
		if _, ok := peersMap[peer.ID().String()]; ok {
			continue
		}
		targetIPs = append(targetIPs, peer.IP)
	}

	partitionName := n.PrefixName(fmt.Sprintf("partition_%d-", len(n.partitions)))

	// create pumba container for every peer in the partition
	pumbas := make([]*DockerContainer, len(peers))
	for i, p := range peers {
		name := partitionName + p.Name + containerNameSuffixPumba
		pumba, err := n.createPumba(name, p.Name, targetIPs)
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
func (n *AutopeeredNetwork) DeletePartitions() error {
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
func (n *AutopeeredNetwork) Partitions() []*Partition {
	return n.partitions
}

// Split splits the existing network in given partitions.
func (n *AutopeeredNetwork) Split(partitions ...[]*Node) error {
	for _, peers := range partitions {
		if _, err := n.createPartition(peers); err != nil {
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
	peers    []*Node
	peersMap map[string]*Node
	pumbas   []*DockerContainer
}

// Nodes returns the partition's peers.
func (p *Partition) Peers() []*Node {
	return p.peers
}

// PeersMap returns the partition's peers map.
func (p *Partition) PeersMap() map[string]*Node {
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
		err = createContainerLogFile(fmt.Sprintf("%s%s", p.name, p.peers[i].Name), logs)
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
