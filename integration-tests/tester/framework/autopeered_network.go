package framework

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/identity"
)

// AutopeeredNetwork is a network consisting out of autopeered nodes.
// It contains additionally an entry node.
type AutopeeredNetwork struct {
	*Network
	// The entry node docker container.
	entryNode *DockerContainer
	// The peer identity of the entry node.
	entryNodeIdentity *identity.Identity
}

// entryNodePublicKey returns the entry node's public key encoded as base58.
func (n *AutopeeredNetwork) entryNodePublicKey() string {
	return n.entryNodeIdentity.PublicKey().String()
}

// createEntryNode creates the network's entry node.
func (n *AutopeeredNetwork) createEntryNode(privKey ed25519.PrivateKey) error {
	n.entryNodeIdentity = identity.New(privKey.Public())

	// create entry node container
	n.entryNode = NewDockerContainer(n.dockerClient)

	cfg := DefaultConfig()
	cfg.Name = n.PrefixName(containerNameEntryNode)
	cfg.Autopeering.Enabled = true
	cfg.Autopeering.RunAsEntryNode = true

	if err := n.entryNode.CreateNodeContainer(cfg); err != nil {
		return err
	}

	if err := n.entryNode.ConnectToNetwork(n.ID); err != nil {
		return err
	}

	if err := n.entryNode.Start(); err != nil {
		return err
	}

	return nil
}

// AwaitPeering waits until all peers have reached the minimum amount of peers.
// Returns error if this minimum is not reached after autopeeringMaxTries.
func (n *AutopeeredNetwork) AwaitPeering(minimumPeers int) error {
	log.Printf("waiting for nodes to fulfill min. autopeer criteria (%d)...", minimumPeers)

	if minimumPeers == 0 {
		return nil
	}

	for i := autopeeringMaxTries; i > 0; i-- {

		for _, p := range n.Nodes {
			peersResponse, err := p.DebugNodeAPIClient.Peers(context.Background())
			if err != nil {
				log.Printf("request error: %s\n", err)

				continue
			}
			p.SetPeers(peersResponse)
		}

		min := 100
		total := 0
		for _, p := range n.Nodes {
			totalPeers := p.TotalPeers()
			if totalPeers < min {
				min = totalPeers
			}
			total += totalPeers
		}

		if min >= minimumPeers {
			log.Printf("peers: min=%d avg=%.2f\n", min, float64(total)/float64(len(n.Nodes)))

			return nil
		}

		log.Printf("criteria (%d) not fulfilled yet. trying again in 5 seconds ...", minimumPeers)
		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("autopeering not successful")
}

// CreatePeer creates a new HORNET node initialized with the right entry node.
func (n *AutopeeredNetwork) CreatePeer(cfg *AppConfig) (*Node, error) {
	ip, err := n.entryNode.IP(n.Name)
	if err != nil {
		return nil, err
	}

	cfg.Autopeering.EntryNodes = []string{
		fmt.Sprintf("/ip4/%s/udp/14626/autopeering/%s", ip, n.entryNodePublicKey()),
	}

	return n.Network.CreateNode(cfg)
}

// Shutdown shuts down the network.
func (n *AutopeeredNetwork) Shutdown() error {
	if err := n.entryNode.Stop(); err != nil {
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

	// check whether the entry node was successfully shutdown
	if entryNodeExitStatus != exitStatusSuccessful {
		return fmt.Errorf("container %s exited with code %d", containerNameEntryNode, entryNodeExitStatus)
	}

	return nil
}
