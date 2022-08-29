// Package framework provides integration test functionality for HORNET with a Docker network.
// It effectively abstracts away all complexity with creating a custom Docker network per test,
// discovering peers, waiting for them to autopeer and offers easy access to the peers' web API and logs.
package framework

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/v2/pkg/p2p/autopeering"
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
type CfgOverrideFunc func(index int, cfg *AppConfig)

// IntegrationNetworkConfig holds configuration for a network.
type IntegrationNetworkConfig struct {
	// Whether the network should have a white-flag mock server running.
	SpawnWhiteFlagMockServer bool
	// The config to use for the white-flag mock server.
	WhiteFlagMockServerConfig *WhiteFlagMockServerConfig
}

// CreateStaticNetwork creates a network made out of statically peered nodes by the given layout.
// The first node is initialized with the Coordinator plugin enabled.
func (f *Framework) CreateStaticNetwork(name string, intNetCfg *IntegrationNetworkConfig, layout StaticPeeringLayout, cfgOverrideF ...CfgOverrideFunc) (*StaticNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeStatic, f.tester)
	if err != nil {
		return nil, err
	}

	if err := layout.Validate(); err != nil {
		return nil, err
	}

	if intNetCfg != nil {
		if err := network.CreateWhiteFlagMockServer(intNetCfg.WhiteFlagMockServerConfig); err != nil {
			return nil, err
		}
		// give the mock server some grace time to boot up
		time.Sleep(10 * time.Second)
	}

	for i := 0; i < len(layout); i++ {
		cfg := DefaultConfig()
		if i == 0 {
			cfg.AsCoo()
		}
		// since we use addPeers to peer the nodes with each other, we need to let
		// them accept any connection. we don't define the peers on startup to prevent
		// nodes from canceling out each other's connection.
		cfg.Network.GossipUnknownPeersLimit = len(layout)
		cfg.Network.ConnMngLowWatermark = len(layout)
		cfg.Network.ConnMngHighWatermark = len(layout) + 1
		cfg.Autopeering.Enabled = false
		if len(cfgOverrideF) > 0 && cfgOverrideF[0] != nil {
			cfgOverrideF[0](i, cfg)
		}
		if _, err = network.CreateNode(cfg); err != nil {
			return nil, err
		}
		if err := setupINX(network, cfg); err != nil {
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
// It waits for the nodes to autopeer until the minimum peers criteria is met for every node.
func (f *Framework) CreateAutopeeredNetwork(name string, peerCount int, minimumPeers int, cfgOverrideF ...CfgOverrideFunc) (*AutopeeredNetwork, error) {
	network, err := newNetwork(f.dockerClient, strings.ToLower(name), NetworkTypeAutopeered, f.tester)
	if err != nil {
		return nil, err
	}

	entryNodePrvKey, err := generatePrivateKey()
	if err != nil {
		return nil, err
	}

	hivePrvKey, err := autopeering.ConvertLibP2PPrivateKeyToHive(entryNodePrvKey.(*crypto.Ed25519PrivateKey))
	if err != nil {
		return nil, err
	}

	autoNetwork := &AutopeeredNetwork{Network: network}
	if err := autoNetwork.createEntryNode(*hivePrvKey); err != nil {
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
		if err := setupINX(network, cfg); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := network.AwaitOnline(ctx); err != nil {
		return nil, err
	}

	// await minimum auto. peers
	if err := autoNetwork.AwaitPeering(minimumPeers); err != nil {
		return nil, err
	}

	return autoNetwork, nil
}

func setupINX(network *Network, cfg *AppConfig) error {
	if cfg.INX.Enabled {
		inxAddress := fmt.Sprintf("%s:9029", cfg.Name)
		if cfg.INXCoo.RunAsCoo {
			cfg.INXCoo.INXAddress = inxAddress
			if _, err := network.CreateCoordinator(cfg.INXCoo); err != nil {
				return err
			}
		}

		// Setup an indexer container for this node
		indexerCfg := DefaultINXIndexerConfig()
		indexerCfg.INXAddress = inxAddress
		if _, err := network.CreateIndexer(indexerCfg); err != nil {
			return err
		}
	}

	return nil
}
