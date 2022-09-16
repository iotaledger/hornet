package framework

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

type NetworkType byte

const (
	// NetworkTypeAutopeered defines a network which consists out of autopeered nodes.
	NetworkTypeAutopeered NetworkType = iota
	// NetworkTypeStatic defines a network which consists out of statically peered nodes.
	NetworkTypeStatic
)

// Network is a network consisting out of HORNET nodes.
type Network struct {
	// The ID of the network.
	ID string
	// the type of the network.
	NetworkType NetworkType
	// The name of the network.
	Name string
	// The nodes within the network in the order in which they were spawned.
	Nodes []*Node
	// The containers running INX extensions in the network.
	INXExtensions []*INXExtension
	// The white-flag mock server if one was started.
	WhiteFlagMockServer *DockerContainer
	// The tester docker container executing the tests.
	tester *DockerContainer
	// The docker client used to communicate with the docker daemon.
	dockerClient *client.Client
}

// PrefixName returns the suffix prefixed with the name.
func (n *Network) PrefixName(suffix string) string {
	return fmt.Sprintf("%s-%s", n.Name, suffix)
}

// AwaitOnline awaits until all nodes are online or the given context is canceled.
func (n *Network) AwaitOnline(ctx context.Context) error {
	log.Println("waiting for nodes to become online ...")
	for _, node := range n.Nodes {
		for {
			if err := returnErrIfCtxDone(ctx, ErrNodesNotOnlineInTime); err != nil {
				return err
			}

			ctxInfo, ctxInfoCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer ctxInfoCancel()

			if _, err := node.DebugNodeAPIClient.Info(ctxInfo); err != nil {
				time.Sleep(500 * time.Millisecond)

				continue
			}

			break
		}
	}

	return nil
}

// AwaitAllSync awaits until all nodes see themselves as synced.
func (n *Network) AwaitAllSync(ctx context.Context) error {
	log.Println("waiting for nodes to become synced ...")
	for _, node := range n.Nodes {
		for {
			if err := returnErrIfCtxDone(ctx, ErrNodesDidNotSyncInTime); err != nil {
				return err
			}

			ctxInfo, ctxInfoCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			defer ctxInfoCancel()

			info, err := node.DebugNodeAPIClient.Info(ctxInfo)
			if err != nil {
				time.Sleep(500 * time.Millisecond)

				continue
			}

			if info.Status.IsHealthy {
				break
			}

			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

// CreateWhiteFlagMockServer creates a new white-flag moc kserver in the network.
func (n *Network) CreateWhiteFlagMockServer(cfg *WhiteFlagMockServerConfig) error {
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreateWhiteFlagMockContainer(cfg); err != nil {
		return err
	}

	n.WhiteFlagMockServer = container

	if err := container.ConnectToNetwork(n.ID); err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		return err
	}

	return nil
}

// generates a new private key or returns the one from the opt parameter.
func generatePrivateKey(optPrvKey ...crypto.PrivKey) (crypto.PrivKey, error) {
	if len(optPrvKey) > 0 && optPrvKey[0] != nil {
		return optPrvKey[0], nil
	}

	privateKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// CreateNode creates a new HORNET node in the network and returns it.
func (n *Network) CreateNode(cfg *AppConfig, optPrvKey ...crypto.PrivKey) (*Node, error) {
	name := n.PrefixName(fmt.Sprintf("%s%d", containerNameReplica, len(n.Nodes)))

	privateKey, err := generatePrivateKey(optPrvKey...)
	if err != nil {
		return nil, err
	}

	privateKeyBytes, err := privateKey.Raw()
	if err != nil {
		return nil, err
	}

	cfg.Network.IdentityPrivKey = hex.EncodeToString(privateKeyBytes)
	cfg.Name = name

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreateNodeContainer(cfg); err != nil {
		return nil, err
	}

	if err := container.ConnectToNetwork(n.ID); err != nil {
		return nil, err
	}

	if err := container.Start(); err != nil {
		return nil, err
	}

	// Obtain Peer ID from public key
	pid, err := peer.IDFromPublicKey(privateKey.GetPublic())
	if err != nil {
		return nil, err
	}

	node, err := newNode(name, pid, cfg, container, n)
	if err != nil {
		return nil, err
	}

	n.Nodes = append(n.Nodes, node)

	return node, nil
}

// CreateCoordinator creates a new INX-Coordinator in the network.
func (n *Network) CreateCoordinator(cfg *INXCoordinatorConfig) (*INXExtension, error) {
	name := n.PrefixName(fmt.Sprintf("%s%d", containerNameINX, len(n.INXExtensions)))

	cfg.Name = name

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreateCoordinatorContainer(cfg); err != nil {
		return nil, err
	}

	if err := container.ConnectToNetwork(n.ID); err != nil {
		return nil, err
	}

	if err := container.Start(); err != nil {
		return nil, err
	}

	ip, err := container.IP(n.Name)
	if err != nil {
		return nil, err
	}

	ext := &INXExtension{
		Name:            cfg.Name,
		IP:              ip,
		DockerContainer: container,
	}
	n.INXExtensions = append(n.INXExtensions, ext)

	return ext, nil
}

// CreateIndexer creates a new INX-Indexer in the network.
func (n *Network) CreateIndexer(cfg *INXIndexerConfig) (*INXExtension, error) {
	name := n.PrefixName(fmt.Sprintf("%s%d", containerNameINX, len(n.INXExtensions)))

	cfg.Name = name
	cfg.BindAddress = fmt.Sprintf("%s:9091", name)

	// create Docker container
	container := NewDockerContainer(n.dockerClient)
	if err := container.CreateIndexerContainer(cfg); err != nil {
		return nil, err
	}

	if err := container.ConnectToNetwork(n.ID); err != nil {
		return nil, err
	}

	if err := container.Start(); err != nil {
		return nil, err
	}

	ip, err := container.IP(n.Name)
	if err != nil {
		return nil, err
	}

	ext := &INXExtension{
		Name:            cfg.Name,
		IP:              ip,
		DockerContainer: container,
	}
	n.INXExtensions = append(n.INXExtensions, ext)

	return ext, nil
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

		if err = createContainerLogFile(p.Name, logs); err != nil {
			return err
		}
	}

	for _, p := range n.INXExtensions {
		logs, err := p.Logs()
		if err != nil {
			return err
		}

		if err = createContainerLogFile(p.Name, logs); err != nil {
			return err
		}
	}

	// save exit status of containers to check at end of shutdown process.
	// we ignore the INXExtensions here, since they will exit with errors if INX disconnects.
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
	for _, p := range n.INXExtensions {
		if err := p.Remove(); err != nil {
			return err
		}
	}

	// shutdown mock server in case it runs
	if n.WhiteFlagMockServer != nil {
		if err := n.WhiteFlagMockServer.Remove(); err != nil {
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

// RandomNode returns a random peer out of the list of peers.
func (n *Network) RandomNode() *Node {
	//nolint:gosec // we don't care about weak random numbers here
	return n.Nodes[rand.Intn(len(n.Nodes))]
}

// Coordinator returns the node with the coordinator plugin enabled.
func (n *Network) Coordinator() *Node {
	return n.Nodes[0]
}

// TakeCPUProfiles takes a CPU profile on all nodes within the network.
func (n *Network) TakeCPUProfiles(dur time.Duration) error {
	log.Printf("taking CPU profile (%v) on all nodes", dur)

	var wg sync.WaitGroup
	wg.Add(len(n.Nodes))

	var profErr error
	for _, n := range n.Nodes {
		go func(node *Node) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Println(r)
				}
			}()

			defer wg.Done()
			if err := node.TakeCPUProfile(dur); err != nil {
				profErr = err
			}
		}(n)
	}
	wg.Wait()

	return profErr
}

// TakeHeapSnapshots takes a heap snapshot on all nodes within the network.
func (n *Network) TakeHeapSnapshots() error {
	log.Printf("taking heap snapshot on all nodes")

	var wg sync.WaitGroup
	wg.Add(len(n.Nodes))

	var profErr error
	for _, n := range n.Nodes {
		go func(n *Node) {
			defer wg.Done()
			if err := n.TakeHeapSnapshot(); err != nil {
				profErr = err
			}
		}(n)
	}

	return profErr
}

// SpamZeroVal starts spamming zero value blocks on all nodes for the given duration.
func (n *Network) SpamZeroVal(dur time.Duration, parallelism int) error {
	log.Printf("spamming zero value blocks on all nodes")

	var wg sync.WaitGroup
	wg.Add(len(n.Nodes))

	var spamErr error
	for _, n := range n.Nodes {
		go func(n *Node) {
			defer wg.Done()
			if _, err := n.Spam(dur, parallelism); err != nil {
				spamErr = err
			}
		}(n)
	}
	wg.Wait()

	return spamErr
}
