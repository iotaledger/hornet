package framework

import (
	"fmt"
	"strings"

	"github.com/docker/go-connections/nat"
	"github.com/gohornet/hornet/pkg/config"
)

const (
	// The seed of the genesis wallet.
	GenesisSeed = "BCPCHOJNIRZDMNDFSEEBIBGFQRGZL9PRXJQBSGHQAKCTYXBSDQEQIJ9STDFHFKMVSAZOHKSVIUBSZYBUC"
	// The first address of the genesis wallet.
	GenesisAddress = "9QJKPJPYTNPF9AFCLGLMAGXOR9ZIPYTRISKOGJPM9ZKKDXGRXWFJZMQTETDJJOGYEVRMLAOECBPWTUZ9B"

	// The default web API port of every node.
	WebAPIPort = 14265

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
	disabledPluginsEntryNode = []string{"dashboard", "profiling", "gossip", "snapshot", "metrics", "tangle", "warpsync", "webapi"}
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
		WebAPI:      DefaultWebAPIConfig(),
		Plugins:     DefaultPluginConfig(),
		Profiling:   DefaultProfilingConfig(),
		Dashboard:   DefaultDashboardConfig(),
	}
	cfg.ExposedPorts = nat.PortSet{
		nat.Port(fmt.Sprintf("%s/tcp", strings.Split(cfg.WebAPI.BindAddress, ":")[1])): {},
		"6060/tcp": {},
		"8081/tcp": {},
	}
	return cfg
}

// NodeConfig defines the config of a Hornet node.
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
	WebAPI WebAPIConfig
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
	cfg.Envs = append(cfg.Envs, fmt.Sprintf("COO_SEED=%s", cfg.Coordinator.Seed))
}

// CLIFlags returns the config as CLI flags.
func (cfg *NodeConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, cfg.Network.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Snapshot.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Coordinator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.WebAPI.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Plugins.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Profiling.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Dashboard.CLIFlags()...)
	return cliFlags
}

// NetworkConfig defines the network specific configuration.
type NetworkConfig struct {
	// The seed for the autopeering identity.
	AutopeeringSeed string
	// The list of entry nodes.
	EntryNodes []string
	// Whether to run the node as entry node.
	RunAsEntryNode bool
	// The static peers for this node.
	StaticPeers []string
	// Whether to accept any connection.
	AcceptAnyConnection bool
	// Max. amount of connected peers via AcceptAnyConnection.
	MaxPeers int
}

// CLIFlags returns the config as CLI flags.
func (netConfig *NetworkConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", config.CfgNetAutopeeringSeed, netConfig.AutopeeringSeed),
		fmt.Sprintf("--%s=%s", config.CfgNetAutopeeringEntryNodes, strings.Join(netConfig.EntryNodes, ",")),
		fmt.Sprintf("--%s=%v", config.CfgNetAutopeeringRunAsEntryNode, netConfig.RunAsEntryNode),
		fmt.Sprintf("--%s=%v", config.CfgPeeringAcceptAnyConnection, netConfig.AcceptAnyConnection),
		fmt.Sprintf("--%s=%d", config.CfgPeeringMaxPeers, netConfig.MaxPeers),
		fmt.Sprintf("--%s=%s", config.CfgPeersList, strings.Join(netConfig.StaticPeers, ",")),
	}
}

// DefaultNetworkConfig returns the default network config.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		AutopeeringSeed: "",
		EntryNodes:      []string{},
	}
}

// WebAPIConfig defines the web API specific configuration.
type WebAPIConfig struct {
	// The bind address for the web API.
	BindAddress string
	// Explicit permitted API calls.
	PermittedAPICalls []string
}

// CLIFlags returns the config as CLI flags.
func (webAPIConfig *WebAPIConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", config.CfgWebAPIBindAddress, webAPIConfig.BindAddress),
		fmt.Sprintf("--%s=%s", config.CfgWebAPIPermitRemoteAccess, strings.Join(webAPIConfig.PermittedAPICalls, ",")),
	}
}

// DefaultWebAPIConfig returns the default web API config.
func DefaultWebAPIConfig() WebAPIConfig {
	return WebAPIConfig{
		BindAddress: "0.0.0.0:14265",
		PermittedAPICalls: []string{
			"getNodeInfo",
			"attachToTangle",
			"getBalances",
			"checkConsistency",
			"getTransactionsToApprove",
			"getInclusionStates",
			"getNodeInfo",
			"getLedgerDiff",
			"getLedgerDiffExt",
			"getLedgerState",
			"addNeighbors",
			"removeNeighbors",
			"getNeighbors",
			"attachToTangle",
			"pruneDatabase",
			"createSnapshotFile",
			"getNodeAPIConfiguration",
			"wereAddressesSpentFrom",
			"broadcastTransactions",
			"findTransactions",
			"storeTransactions",
			"getTrytes",
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
	// The load type of the snapshot.
	LoadType string
	// The path to the global snapshot file.
	GlobalSnapshotFilePath string
	// The index of the global snapshot.
	GlobalSnapshotIndex int
	// The path to the local snapshot file.
	LocalSnapshotFilePath string
	// The file paths to the epoch spent address files.
	EpochSpentAddressesFilePath []string
}

// CLIFlags returns the config as CLI flags.
func (snapshotConfig *SnapshotConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", config.CfgSnapshotLoadType, snapshotConfig.LoadType),
		fmt.Sprintf("--%s=%s", config.CfgGlobalSnapshotPath, snapshotConfig.GlobalSnapshotFilePath),
		fmt.Sprintf("--%s=%s", config.CfgGlobalSnapshotSpentAddressesPaths, strings.Join(snapshotConfig.EpochSpentAddressesFilePath, ",")),
		fmt.Sprintf("--%s=%d", config.CfgGlobalSnapshotIndex, snapshotConfig.GlobalSnapshotIndex),
		fmt.Sprintf("--%s=%s", config.CfgLocalSnapshotsPath, snapshotConfig.LocalSnapshotFilePath),
	}
}

// DefaultSnapshotConfig returns the default snapshot config.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		LoadType:               "global",
		GlobalSnapshotFilePath: "/assets/snapshot.csv",
		GlobalSnapshotIndex:    0,
		LocalSnapshotFilePath:  "",
		EpochSpentAddressesFilePath: []string{
			"/assets/previousEpochsSpentAddresses.txt",
		},
	}
}

// CoordinatorConfig defines coordinator specific configuration.
type CoordinatorConfig struct {
	// Whether to let the node run as the coordinator.
	RunAsCoo bool
	// Whether to run the coordinator in bootstrap node.
	Bootstrap bool
	// The coo Merkle root address.
	Address string
	// The coo seed.
	Seed string
	// The MWM/PoW difficulty to use.
	MWM int
	// The security level used for milestones.
	SecurityLevel int
	// The interval in which to issue new milestones.
	IssuanceIntervalSeconds int
	// The depth of the coo merkle tree.
	MerkleTreeDepth int
	// The path to the Merkle tree file.
	MerkleTreeFilePath string
}

// CLIFlags returns the config as CLI flags.
func (cooConfig *CoordinatorConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--cooBootstrap=%v", cooConfig.Bootstrap),
		fmt.Sprintf("--%s=%d", config.CfgCoordinatorMWM, cooConfig.MWM),
		fmt.Sprintf("--%s=%s", config.CfgCoordinatorAddress, cooConfig.Address),
		fmt.Sprintf("--%s=%s", config.CfgCoordinatorMerkleTreeFilePath, cooConfig.MerkleTreeFilePath),
		fmt.Sprintf("--%s=%d", config.CfgCoordinatorIntervalSeconds, cooConfig.IssuanceIntervalSeconds),
		fmt.Sprintf("--%s=%d", config.CfgCoordinatorSecurityLevel, cooConfig.SecurityLevel),
		fmt.Sprintf("--%s=%d", config.CfgCoordinatorMerkleTreeDepth, cooConfig.MerkleTreeDepth),
	}
}

// DefaultCoordinatorConfig returns the default coordinator config.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		RunAsCoo:                false,
		Bootstrap:               false,
		Address:                 "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ",
		Seed:                    "MFKHXRCTTRSPATPMAGFCSSIBJXDWNCLBQJPMHQGMGJEWYJNUXSXAGVSUJK9BMCAFEDNTXHDWWVVNILDRG",
		MWM:                     1,
		SecurityLevel:           2,
		IssuanceIntervalSeconds: 10,
		MerkleTreeDepth:         18,
		MerkleTreeFilePath:      "/assets/coordinator.tree",
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
		fmt.Sprintf("--%s=%s", config.CfgProfilingBindAddress, profilingConfig.BindAddress),
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
		fmt.Sprintf("--%s=%s", config.CfgDashboardBindAddress, dashboardConfig.BindAddress),
	}
}

// DefaultDashboardConfig returns the default profiling config.
func DefaultDashboardConfig() DashboardConfig {
	return DashboardConfig{
		BindAddress: "0.0.0.0:8081",
	}
}
