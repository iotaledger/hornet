package framework

import (
	"fmt"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/core/snapshot"
	coopkg "github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/restapi"
)

const (
	// The default REST API port of every node.
	RestAPIPort = 14265

	autopeeringMaxTries = 50

	containerNodeImage    = "hornet:dev"
	containerPumbaImage   = "gaiaadm/pumba:0.7.4"
	containerIPRouteImage = "gaiadocker/iproute2"

	containerNameTester      = "/tester"
	containerNameEntryNode   = "entry_node"
	containerNameReplica     = "replica_"
	containerNameSuffixPumba = "_pumba"

	logsDir = "/tmp/logs/"

	assetsDir = "/assets"

	dockerLogsPrefixLen  = 8
	exitStatusSuccessful = 0
)

var (
	disabledPluginsEntryNode = []string{"dashboard", "profiling", "gossip", "snapshot", "metrics", "tangle", "warpsync", "restapi"}
	disabledPluginsPeer      = []string{}
)

// DefaultConfig returns the default NodeConfig.
func DefaultConfig() *NodeConfig {
	cfg := &NodeConfig{
		Name: "",
		Envs: []string{"LOGGER_LEVEL=debug"},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
		Network:     DefaultNetworkConfig(),
		Snapshot:    DefaultSnapshotConfig(),
		Coordinator: DefaultCoordinatorConfig(),
		RestAPI:     DefaultRestAPIConfig(),
		Plugins:     DefaultPluginConfig(),
		Profiling:   DefaultProfilingConfig(),
		Dashboard:   DefaultDashboardConfig(),
	}
	cfg.ExposedPorts = nat.PortSet{
		nat.Port(fmt.Sprintf("%s/tcp", strings.Split(cfg.RestAPI.BindAddress, ":")[1])): {},
		"6060/tcp": {},
		"8081/tcp": {},
	}
	return cfg
}

// NodeConfig defines the config of a HORNET node.
type NodeConfig struct {
	// The name of this node.
	Name string
	// Environment variables.
	Envs []string
	// Binds for the container.
	Binds []string
	// Exposed ports of this container.
	ExposedPorts nat.PortSet
	// Network config.
	Network NetworkConfig
	// Web API config.
	RestAPI RestAPIConfig
	// Snapshot config.
	Snapshot SnapshotConfig
	// Coordinator config.
	Coordinator CoordinatorConfig
	// Plugin config.
	Plugins PluginConfig
	// Profiling config.
	Profiling ProfilingConfig
	// Dashboard config.
	Dashboard DashboardConfig
}

// AsCoo adjusts the config to make it usable as the Coordinator's config.
func (cfg *NodeConfig) AsCoo() {
	cfg.Coordinator.Bootstrap = true
	cfg.Coordinator.RunAsCoo = true
	cfg.Plugins.Enabled = append(cfg.Plugins.Enabled, "Coordinator")
	cfg.Envs = append(cfg.Envs, fmt.Sprintf("COO_PRV_KEYS=%s", strings.Join(cfg.Coordinator.PrivateKeys, ",")))
}

// CLIFlags returns the config as CLI flags.
func (cfg *NodeConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, cfg.Network.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Snapshot.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Coordinator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.RestAPI.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Plugins.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Profiling.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Dashboard.CLIFlags()...)
	return cliFlags
}

// NetworkConfig defines the network specific configuration.
type NetworkConfig struct {
	// the private key used to derive the node identity.
	IdentityPrivKey string
	// the bind addresses of this node.
	BindMultiAddresses []string
	// the path to the peerstore.
	PeerStorePath string
	// the high watermark to use within the connection manager.
	ConnMngHighWatermark int
	// the low watermark to use within the connection manager.
	ConnMngLowWatermark int
	// the static peers this node should retain a connection to.
	Peers []string
	// aliases of the static peers.
	PeerAliases []string
	// number of seconds to wait before trying to reconnect to a disconnected peer.
	ReconnectIntervalSeconds int
	// the maximum amount of unknown peers a gossip protocol connection is established to
	GossipUnknownPeersLimit int
}

// CLIFlags returns the config as CLI flags.
func (netConfig *NetworkConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", p2p.CfgP2PIdentityPrivKey, netConfig.IdentityPrivKey),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PBindMultiAddresses, strings.Join(netConfig.BindMultiAddresses, ",")),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PPeerStorePath, netConfig.PeerStorePath),
		fmt.Sprintf("--%s=%d", p2p.CfgP2PConnMngHighWatermark, netConfig.ConnMngHighWatermark),
		fmt.Sprintf("--%s=%d", p2p.CfgP2PConnMngLowWatermark, netConfig.ConnMngLowWatermark),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PPeers, strings.Join(netConfig.Peers, ",")),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PPeerAliases, strings.Join(netConfig.PeerAliases, ",")),
		fmt.Sprintf("--%s=%d", p2p.CfgP2PReconnectIntervalSeconds, netConfig.ReconnectIntervalSeconds),
		fmt.Sprintf("--%s=%d", gossip.CfgP2PGossipUnknownPeersLimit, netConfig.GossipUnknownPeersLimit),
	}
}

// DefaultNetworkConfig returns the default network config.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		IdentityPrivKey:          "",
		BindMultiAddresses:       []string{"/ip4/127.0.0.1/tcp/15600"},
		PeerStorePath:            "./p2pstore",
		ConnMngHighWatermark:     10,
		ConnMngLowWatermark:      5,
		Peers:                    []string{},
		PeerAliases:              []string{},
		ReconnectIntervalSeconds: 10,
		GossipUnknownPeersLimit:  4,
	}
}

// RestAPIConfig defines the REST API specific configuration.
type RestAPIConfig struct {
	// The bind address for the REST API.
	BindAddress string
	// Explicit permitted REST API routes.
	PermittedRoutes []string
}

// CLIFlags returns the config as CLI flags.
func (restAPIConfig *RestAPIConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", restapi.CfgRestAPIBindAddress, restAPIConfig.BindAddress),
		fmt.Sprintf("--%s=%s", restapi.CfgRestAPIPermittedRoutes, strings.Join(restAPIConfig.PermittedRoutes, ",")),
	}
}

// DefaultRestAPIConfig returns the default REST API config.
func DefaultRestAPIConfig() RestAPIConfig {
	return RestAPIConfig{
		BindAddress: "0.0.0.0:14265",
		PermittedRoutes: []string{
			"/health",
			"/api/v1/info",
			"/api/v1/tips",
			"/api/v1/messages/*",
			"/api/v1/messages/*/metadata",
			"/api/v1/messages/*/raw",
			"/api/v1/messages/*/children",
			"/api/v1/messages",
			"/api/v1/milestones/*",
			"/api/v1/outputs/*",
			"/api/v1/addresses/*",
			"/api/v1/addresses/*/outputs",
			"/api/v1/peers/*",
			"/api/v1/peers",
			"/api/v1/debug/outputs",
			"/api/v1/debug/outputs/unspent",
			"/api/v1/debug/outputs/spent",
			"/api/v1/debug/ms-diff/*",
			"/api/v1/debug/requests",
			"/api/v1/debug/message-cones/*",
		},
	}
}

// PluginConfig defines plugin specific configuration.
type PluginConfig struct {
	// Holds explicitly enabled plugins.
	Enabled []string
	// Holds explicitly disabled plugins.
	Disabled []string
}

// CLIFlags returns the config as CLI flags.
func (pluginConfig *PluginConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--node.enablePlugins=%s", strings.Join(pluginConfig.Enabled, ",")),
		fmt.Sprintf("--node.disablePlugins=%s", strings.Join(pluginConfig.Disabled, ",")),
	}
}

// DefaultPluginConfig returns the default plugin config.
func DefaultPluginConfig() PluginConfig {
	disabled := make([]string, len(disabledPluginsPeer))
	copy(disabled, disabledPluginsPeer)
	return PluginConfig{
		Enabled:  []string{},
		Disabled: disabled,
	}
}

// SnapshotConfig defines snapshot specific configuration.
type SnapshotConfig struct {
	// The path to the snapshot file.
	SnapshotFilePath string
}

// CLIFlags returns the config as CLI flags.
func (snapshotConfig *SnapshotConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", snapshot.CfgSnapshotsPath, snapshotConfig.SnapshotFilePath),
	}
}

// DefaultSnapshotConfig returns the default snapshot config.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		SnapshotFilePath: "/assets/snapshot.bin",
	}
}

// CoordinatorConfig defines coordinator specific configuration.
type CoordinatorConfig struct {
	// Whether to let the node run as the coordinator.
	RunAsCoo bool
	// Whether to run the coordinator in bootstrap node.
	Bootstrap bool
	// The minimum PoW score needed.
	MinPoWScore float64
	// The coo private keys.
	PrivateKeys []string
	// The coo public key ranges.
	PublicKeyRanges []coopkg.PublicKeyRange
	// The interval in which to issue new milestones.
	IssuanceIntervalSeconds int
}

// CLIFlags returns the config as CLI flags.
func (cooConfig *CoordinatorConfig) CLIFlags() []string {
	keyRanges := []string{}

	for _, keyRange := range cooConfig.PublicKeyRanges {
		keyRanges = append(keyRanges, fmt.Sprintf("{\"key\":\"%v\",\"start\":%d,\"end\":%d}", keyRange.Key, keyRange.StartIndex, keyRange.EndIndex))
	}

	return []string{
		fmt.Sprintf("--cooBootstrap=%v", cooConfig.Bootstrap),
		fmt.Sprintf("--publicKeyRanges=[%v]", strings.Join(keyRanges, ",")),
		fmt.Sprintf("--%s=%d", coordinator.CfgCoordinatorIntervalSeconds, cooConfig.IssuanceIntervalSeconds),
		fmt.Sprintf("--%s=%0.0f", coordinator.CfgCoordinatorMinPoWScore, cooConfig.MinPoWScore),
	}
}

// DefaultCoordinatorConfig returns the default coordinator config.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		RunAsCoo:    false,
		Bootstrap:   false,
		MinPoWScore: 100,
		PrivateKeys: []string{"651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
			"0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c"},
		PublicKeyRanges: []coopkg.PublicKeyRange{
			{
				Key:        "ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
				StartIndex: 0,
				EndIndex:   0,
			},
			{
				Key:        "f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c",
				StartIndex: 0,
				EndIndex:   0,
			},
		},
		IssuanceIntervalSeconds: 10,
	}
}

// ProfilingConfig defines the profiling specific configuration.
type ProfilingConfig struct {
	// The bind address of the pprof server.
	BindAddress string
}

// CLIFlags returns the config as CLI flags.
func (profilingConfig *ProfilingConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", profiling.CfgProfilingBindAddress, profilingConfig.BindAddress),
	}
}

// DefaultProfilingConfig returns the default profiling config.
func DefaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		BindAddress: "0.0.0.0:6060",
	}
}

// DashboardConfig holds the dashboard specific configuration.
type DashboardConfig struct {
	// The bind address of the dashboard
	BindAddress string
}

// CLIFlags returns the config as CLI flags.
func (dashboardConfig *DashboardConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", dashboard.CfgDashboardBindAddress, dashboardConfig.BindAddress),
	}
}

// DefaultDashboardConfig returns the default profiling config.
func DefaultDashboardConfig() DashboardConfig {
	return DashboardConfig{
		BindAddress: "0.0.0.0:8081",
	}
}
