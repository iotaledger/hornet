package framework

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

type NetworkType byte

const (
	// Defines a network which consists out of autopeered nodes.
	NetworkTypeAutopeered NetworkType = iota
	// Defines a network which consists out of statically peered nodes.
	NetworkTypeStatic
)

// Network is a network consisting out of Hornet nodes.
type Network struct {
	// The ID of the network.
	ID string
	// the type of the network.
	NetworkType NetworkType
	// The name of the network.
	Name string
	// The nodes within the network in the order in which they were spawned.
	Nodes []*Node
	// The tester docker container executing the tests.
	tester *DockerContainer
	// The docker client used to communicate with the docker daemon.
	dockerClient *client.Client
}

// PrefixName returns the suffix prefixed with the name.
func (n *Network) PrefixName(suffix string) string {
	return fmt.Sprintf("%s-%s", n.Name, suffix)
}

// AwaitOnline awaits until all nodes are online or the given context is cancelled.
func (n *Network) AwaitOnline(ctx context.Context) error {
	for _, node := range n.Nodes {
		for {
			select {
			case <-ctx.Done():
				return ErrNodesNotOnlineInTime
			default:
			}
			if _, err := node.WebAPI.GetNodeInfo(); err != nil {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			break
		}
	}
	return nil
}

// AwaitAllSync awaits until all nodes see themselves as synced.
func (n *Network) AwaitAllSync(ctx context.Context) error {
	for _, node := range n.Nodes {
		for {
			select {
			case <-ctx.Done():
				return ErrNodesDidNotSyncInTime
			default:
			}
			info, err := node.DebugWebAPI.Info()
			if err != nil {
				time.Sleep(250 * time.Millisecond)
				continue
			}
			if info.IsSynced {
				break
			}
		}
	}
	return nil
}

// CreatePeer creates a new Hornet node in the network and returns it.
func (n *Network) CreatePeer(cfg *NodeConfig) (*Node, error) {
	name := n.PrefixName(fmt.Sprintf("%s%d", containerNameReplica, len(n.Nodes)))

	// create identity
	publicKey, privateKey, err := ed25519.GenerateKey()
	if err != nil {
		return nil, err
	}
	seed := privateKey.Seed().String()

	cfg.Name = name
	cfg.AutopeeringSeed = seed

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	if err = container.CreateNode(cfg); err != nil {
		return nil, err
	}
	if err = container.ConnectToNetwork(n.ID); err != nil {
		return nil, err
	}
	if err = container.Start(); err != nil {
		return nil, err
	}

	peer, err := newNode(name, identity.New(publicKey), container, n)
	if err != nil {
		return nil, err
	}
	n.Nodes = append(n.Nodes, peer)
	return peer, nil
}

// newNetwork returns a AutopeeredNetwork instance, creates its underlying Docker network and adds the tester container to the network.
func newNetwork(dockerClient *client.Client, name string, netType NetworkType, tester *DockerContainer) (*Network, error) {
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
		ID:           resp.ID,
		NetworkType:  netType,
		Name:         name,
		tester:       tester,
		dockerClient: dockerClient,
	}, nil
}

// Shutdown stops all nodes, persists their container logs and removes them from Docker.
func (n *Network) Shutdown() error {
	for _, p := range n.Nodes {
		if err := p.Stop(); err != nil {
			return err
		}
	}

	for _, p := range n.Nodes {
		logs, err := p.Logs()
		if err != nil {
			return err
		}
		if err = createLogFile(p.Name, logs); err != nil {
			return err
		}
	}

	// save exit status of containers to check at end of shutdown process
	exitStatus := make(map[string]int, len(n.Nodes))
	for _, p := range n.Nodes {
		var err error
		exitStatus[p.Name], err = p.ExitStatus()
		if err != nil {
			return err
		}
	}

	// remove containers
	for _, p := range n.Nodes {
		if err := p.Remove(); err != nil {
			return err
		}
	}

	// disconnect tester from network otherwise the network can't be removed
	if err := n.tester.DisconnectFromNetwork(n.ID); err != nil {
		return err
	}

	// remove network
	if err := n.dockerClient.NetworkRemove(context.Background(), n.ID); err != nil {
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

// RandomPeer returns a random peer out of the list of peers.
func (n *Network) RandomPeer() *Node {
	return n.Nodes[rand.Intn(len(n.Nodes))]
}
