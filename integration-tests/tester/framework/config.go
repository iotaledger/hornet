package framework

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"

	"github.com/gohornet/hornet/plugins/autopeering"

	"github.com/gohornet/hornet/core/app"
	"github.com/gohornet/hornet/core/gossip"
	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/core/protocfg"
	"github.com/gohornet/hornet/core/snapshot"
	coopkg "github.com/gohornet/hornet/pkg/model/coordinator"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/plugins/coordinator"
	"github.com/gohornet/hornet/plugins/dashboard"
	"github.com/gohornet/hornet/plugins/migrator"
	"github.com/gohornet/hornet/plugins/profiling"
	"github.com/gohornet/hornet/plugins/receipt"
	"github.com/gohornet/hornet/plugins/restapi"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

const (
	// The default REST API port of every node.
	RestAPIPort = 14265

	GenesisAddressPublicKeyHex = "f7868ab6bb55800b77b8b74191ad8285a9bf428ace579d541fda47661803ff44"
	GenesisAddressHex          = "6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92"

	autopeeringMaxTries = 50

	containerNodeImage           = "hornet:dev"
	containerWhiteFlagMockServer = "wfmock:latest"
	containerPumbaImage          = "gaiaadm/pumba:0.7.4"
	containerIPRouteImage        = "gaiadocker/iproute2"

	containerNameTester      = "/tester"
	containerNameReplica     = "replica_"
	containerNameSuffixPumba = "_pumba" //nolint:deadcode,varcheck

	logsDir = "/tmp/logs/"

	assetsDir = "/assets"

	dockerLogsPrefixLen    = 8
	exitStatusSuccessful   = 0
	containerNameEntryNode = "entry_node"
)

var (
	disabledPluginsPeer      = []string{}
	disabledPluginsEntryNode = []string{}
	// The seed on which the total supply resides on per default.
	GenesisSeed    ed25519.PrivateKey
	GenesisAddress iotago.Ed25519Address
)

func init() {
	prvkey, err := utils.ParseEd25519PrivateKeyFromString("256a818b2aac458941f7274985a410e57fb750f3a3a67969ece5bd9ae7eef5b2f7868ab6bb55800b77b8b74191ad8285a9bf428ace579d541fda47661803ff44")
	if err != nil {
		panic(err)
	}
	GenesisSeed = prvkey
	GenesisAddress = iotago.AddressFromEd25519PubKey(GenesisSeed.Public().(ed25519.PublicKey))
}

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
		Protocol:    DefaultProtocolConfig(),
		RestAPI:     DefaultRestAPIConfig(),
		Plugins:     DefaultPluginConfig(),
		Profiling:   DefaultProfilingConfig(),
		Dashboard:   DefaultDashboardConfig(),
		Receipts:    DefaultReceiptValidatorConfig(),
		Migrator:    DefaultMigratorConfig(),
		Autopeering: DefaultAutopeeringConfig(),
	}
	cfg.ExposedPorts = nat.PortSet{
		nat.Port(fmt.Sprintf("%s/tcp", strings.Split(cfg.RestAPI.BindAddress, ":")[1])): {},
		"6060/tcp": {},
		"8081/tcp": {},
	}
	return cfg
}

// WhiteFlagMockServerConfig defines the config for a white-flag mock server instance.
type WhiteFlagMockServerConfig struct {
	// The name for this white-flag mock server.
	Name string
	// environment variables.
	Envs []string
	// Binds for the container.
	Binds []string
}

// DefaultWhiteFlagMockServerConfig returns the default WhiteFlagMockServerConfig.
func DefaultWhiteFlagMockServerConfig(configFileName string) *WhiteFlagMockServerConfig {
	return &WhiteFlagMockServerConfig{
		Name: "wfmock",
		Envs: []string{
			fmt.Sprintf("WHITE_FLAG_MOCK_CONFIG=%s/%s", assetsDir, configFileName),
		},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
	}
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
	// Protocol config.
	Protocol ProtocolConfig
	// Plugin config.
	Plugins PluginConfig
	// Profiling config.
	Profiling ProfilingConfig
	// Dashboard config.
	Dashboard DashboardConfig
	// Receipts config
	Receipts ReceiptsConfig
	// Migrator config.
	Migrator MigratorConfig
	// Autopeering config.
	Autopeering AutopeeringConfig
}

// AsCoo adjusts the config to make it usable as the Coordinator's config.
func (cfg *NodeConfig) AsCoo() {
	cfg.Coordinator.Bootstrap = true
	cfg.Coordinator.RunAsCoo = true
	cfg.Plugins.Enabled = append(cfg.Plugins.Enabled, "Coordinator")
	cfg.Envs = append(cfg.Envs, fmt.Sprintf("COO_PRV_KEYS=%s", strings.Join(cfg.Coordinator.PrivateKeys, ",")))
}

// WithMigration adjusts the config to activate the migrator plugin.
func (cfg *NodeConfig) WithMigration() {
	cfg.Migrator.Bootstrap = true
	cfg.Plugins.Enabled = append(cfg.Plugins.Enabled, "Migrator", "Receipts")
}

// CLIFlags returns the config as CLI flags.
func (cfg *NodeConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, cfg.Network.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Snapshot.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Coordinator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Protocol.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.RestAPI.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Plugins.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Profiling.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Dashboard.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Receipts.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Migrator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Autopeering.CLIFlags()...)
	return cliFlags
}

// NetworkConfig defines the network specific configuration.
type NetworkConfig struct {
	// the private key used to derive the node identity.
	IdentityPrivKey string
	// the bind addresses of this node.
	BindMultiAddresses []string
	// the path to the p2p database.
	DatabasePath string
	// the high watermark to use within the connection manager.
	ConnMngHighWatermark int
	// the low watermark to use within the connection manager.
	ConnMngLowWatermark int
	// the static peers this node should retain a connection to.
	Peers []string
	// aliases of the static peers.
	PeerAliases []string
	// time to wait before trying to reconnect to a disconnected peer.
	ReconnectInterval time.Duration
	// the maximum amount of unknown peers a gossip protocol connection is established to
	GossipUnknownPeersLimit int
}

// CLIFlags returns the config as CLI flags.
func (netConfig *NetworkConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", p2p.CfgP2PIdentityPrivKey, netConfig.IdentityPrivKey),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PBindMultiAddresses, strings.Join(netConfig.BindMultiAddresses, ",")),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PDatabasePath, netConfig.DatabasePath),
		fmt.Sprintf("--%s=%d", p2p.CfgP2PConnMngHighWatermark, netConfig.ConnMngHighWatermark),
		fmt.Sprintf("--%s=%d", p2p.CfgP2PConnMngLowWatermark, netConfig.ConnMngLowWatermark),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PPeers, strings.Join(netConfig.Peers, ",")),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PPeerAliases, strings.Join(netConfig.PeerAliases, ",")),
		fmt.Sprintf("--%s=%s", p2p.CfgP2PReconnectInterval, netConfig.ReconnectInterval),
		fmt.Sprintf("--%s=%d", gossip.CfgP2PGossipUnknownPeersLimit, netConfig.GossipUnknownPeersLimit),
	}
}

// DefaultNetworkConfig returns the default network config.
func DefaultNetworkConfig() NetworkConfig {
	return NetworkConfig{
		IdentityPrivKey:         "",
		BindMultiAddresses:      []string{"/ip4/0.0.0.0/tcp/15600"},
		DatabasePath:            "p2pstore",
		ConnMngHighWatermark:    8,
		ConnMngLowWatermark:     4,
		Peers:                   []string{},
		PeerAliases:             []string{},
		ReconnectInterval:       1 * time.Second,
		GossipUnknownPeersLimit: 4,
	}
}

// AutopeeringConfig defines the autopeering specific configuration.
type AutopeeringConfig struct {
	// The ist of autopeering entry nodes to use.
	EntryNodes []string
	// BindAddr bind address for autopeering.
	BindAddr string
	// Whether the node should act as an autopeering entry node.
	RunAsEntryNode bool
	// The max number of inbound autopeers.
	InboundPeers int
	// The max the number of outbound autopeers.
	OutboundPeers int
	// The lifetime of the private and public local salt.
	SaltLifetime time.Duration
}

// CLIFlags returns the config as CLI flags.
func (autoConfig *AutopeeringConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", autopeering.CfgNetAutopeeringEntryNodes, strings.Join(autoConfig.EntryNodes, ",")),
		fmt.Sprintf("--%s=%s", autopeering.CfgNetAutopeeringBindAddr, autoConfig.BindAddr),
		fmt.Sprintf("--%s=%v", autopeering.CfgNetAutopeeringRunAsEntryNode, autoConfig.RunAsEntryNode),
		fmt.Sprintf("--%s=%d", autopeering.CfgNetAutopeeringInboundPeers, autoConfig.InboundPeers),
		fmt.Sprintf("--%s=%d", autopeering.CfgNetAutopeeringOutboundPeers, autoConfig.OutboundPeers),
		fmt.Sprintf("--%s=%s", autopeering.CfgNetAutopeeringSaltLifetime, autoConfig.SaltLifetime),
	}
}

// DefaultAutopeeringConfig returns the default autopeering config.
func DefaultAutopeeringConfig() AutopeeringConfig {
	return AutopeeringConfig{
		EntryNodes:     nil,
		BindAddr:       "0.0.0.0:14626",
		RunAsEntryNode: false,
		InboundPeers:   2,
		OutboundPeers:  2,
		SaltLifetime:   30 * time.Minute,
	}
}

// RestAPIConfig defines the REST API specific configuration.
type RestAPIConfig struct {
	// The bind address for the REST API.
	BindAddress string
	// Explicit permitted REST API routes.
	PermittedRoutes []string
	// Whether the node does proof-of-work for submitted messages.
	PoWEnabled bool
}

// CLIFlags returns the config as CLI flags.
func (restAPIConfig *RestAPIConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", restapi.CfgRestAPIBindAddress, restAPIConfig.BindAddress),
		fmt.Sprintf("--%s=%s", restapi.CfgRestAPIPermittedRoutes, strings.Join(restAPIConfig.PermittedRoutes, ",")),
		fmt.Sprintf("--%s=%v", restapi.CfgRestAPIPoWEnabled, restAPIConfig.PoWEnabled),
	}
}

// DefaultRestAPIConfig returns the default REST API config.
func DefaultRestAPIConfig() RestAPIConfig {
	return RestAPIConfig{
		BindAddress: "0.0.0.0:14265",
		PermittedRoutes: []string{
			"/health",
			"/mqtt",
			"/api/v1/info",
			"/api/v1/tips",
			"/api/v1/messages/:messageID",
			"/api/v1/messages/:messageID/metadata",
			"/api/v1/messages/:messageID/raw",
			"/api/v1/messages/:messageID/children",
			"/api/v1/messages",
			"/api/v1/transactions/:transactionID/included-message",
			"/api/v1/milestones/:milestoneIndex",
			"/api/v1/outputs/:outputID",
			"/api/v1/addresses/:address",
			"/api/v1/addresses/:address/outputs",
			"/api/v1/addresses/ed25519/:address",
			"/api/v1/addresses/ed25519/:address/outputs",
			"/api/v1/treasury",
			"/api/v1/receipts",
			"/api/v1/receipts/:milestoneIndex",
			"/api/v1/peers/:peerID",
			"/api/v1/peers",
			"/api/plugins/*",
		},
		PoWEnabled: true,
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
		fmt.Sprintf("--%s=%s", app.CfgNodeEnablePlugins, strings.Join(pluginConfig.Enabled, ",")),
		fmt.Sprintf("--%s=%s", app.CfgNodeDisablePlugins, strings.Join(pluginConfig.Disabled, ",")),
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
	// The path to the full snapshot file.
	FullSnapshotFilePath string
	// the path to the delta snapshot file.
	DeltaSnapshotFilePath string
}

// CLIFlags returns the config as CLI flags.
func (snapshotConfig *SnapshotConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", snapshot.CfgSnapshotsFullPath, snapshotConfig.FullSnapshotFilePath),
		fmt.Sprintf("--%s=%s", snapshot.CfgSnapshotsDeltaPath, snapshotConfig.DeltaSnapshotFilePath),
	}
}

// DefaultSnapshotConfig returns the default snapshot config.
func DefaultSnapshotConfig() SnapshotConfig {
	return SnapshotConfig{
		FullSnapshotFilePath:  "/assets/full_snapshot.bin",
		DeltaSnapshotFilePath: "/assets/delta_snapshot.bin",
	}
}

// CoordinatorConfig defines coordinator specific configuration.
type CoordinatorConfig struct {
	// Whether to let the node run as the coordinator.
	RunAsCoo bool
	// The coo private keys.
	PrivateKeys []string
	// Whether to run the coordinator in bootstrap node.
	Bootstrap bool
	// The interval in which to issue new milestones.
	IssuanceInterval time.Duration
}

// CLIFlags returns the config as CLI flags.
func (cooConfig *CoordinatorConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--cooBootstrap=%v", cooConfig.Bootstrap),
		fmt.Sprintf("--%s=%s", coordinator.CfgCoordinatorInterval, cooConfig.IssuanceInterval),
	}
}

// DefaultCoordinatorConfig returns the default coordinator config.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		RunAsCoo:  false,
		Bootstrap: false,
		PrivateKeys: []string{"651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
			"0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c"},
		IssuanceInterval: 10 * time.Second,
	}
}

// ReceiptsConfig defines the receipt validator plugin specific configuration.
type ReceiptsConfig struct {
	// Whether receipt backups are enabled.
	BackupEnabled bool
	// The path to the receipts folder.
	BackupFolder string
	// Whether the receipts plugin should validate receipts
	Validate bool
	// Whether to ignore soft errors or not.
	IgnoreSoftErrors bool
	// The API to query.
	APIAddress string
	// The API timeout.
	APITimeout time.Duration
	// Legacy Coordinator address
	CoordinatorAddress string
	// The merkle tree depth.
	CoordinatorMerkleTreeDepth int
}

func (receiptsConfig *ReceiptsConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%v", receipt.CfgReceiptsBackupEnabled, receiptsConfig.BackupEnabled),
		fmt.Sprintf("--%s=%s", receipt.CfgReceiptsBackupPath, receiptsConfig.BackupFolder),
		fmt.Sprintf("--%s=%v", receipt.CfgReceiptsValidatorValidate, receiptsConfig.Validate),
		fmt.Sprintf("--%s=%v", receipt.CfgReceiptsValidatorIgnoreSoftErrors, receiptsConfig.IgnoreSoftErrors),
		fmt.Sprintf("--%s=%s", receipt.CfgReceiptsValidatorAPIAddress, receiptsConfig.APIAddress),
		fmt.Sprintf("--%s=%s", receipt.CfgReceiptsValidatorAPITimeout, receiptsConfig.APITimeout),
		fmt.Sprintf("--%s=%s", receipt.CfgReceiptsValidatorCoordinatorAddress, receiptsConfig.CoordinatorAddress),
		fmt.Sprintf("--%s=%d", receipt.CfgReceiptsValidatorCoordinatorMerkleTreeDepth, receiptsConfig.CoordinatorMerkleTreeDepth),
	}
}

// DefaultReceiptValidatorConfig returns the default migrator plugin config.
func DefaultReceiptValidatorConfig() ReceiptsConfig {
	return ReceiptsConfig{
		BackupEnabled:              false,
		BackupFolder:               "receipts",
		Validate:                   false,
		IgnoreSoftErrors:           false,
		APIAddress:                 "http://localhost:14265",
		APITimeout:                 5 * time.Second,
		CoordinatorAddress:         "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ",
		CoordinatorMerkleTreeDepth: 18,
	}
}

// MigratorConfig defines migrator plugin specific configuration.
type MigratorConfig struct {
	// The max amount of entries to include in a receipt.
	MaxEntries int
	// Whether to run the migrator plugin in bootstrap mode.
	Bootstrap bool
	// The index of the first legacy milestone to migrate.
	StartIndex int
	// The state file path.
	StateFilePath string
}

// CLIFlags returns the config as CLI flags.
func (migConfig *MigratorConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%v", migrator.CfgMigratorBootstrap, migConfig.Bootstrap),
		fmt.Sprintf("--%s=%v", migrator.CfgMigratorReceiptMaxEntries, migConfig.MaxEntries),
		fmt.Sprintf("--%s=%d", migrator.CfgMigratorStartIndex, migConfig.StartIndex),
		fmt.Sprintf("--%s=%s", migrator.CfgMigratorStateFilePath, migConfig.StateFilePath),
	}
}

// DefaultMigratorConfig returns the default migrator plugin config.
func DefaultMigratorConfig() MigratorConfig {
	return MigratorConfig{
		Bootstrap:     false,
		MaxEntries:    iotago.MaxMigratedFundsEntryCount,
		StartIndex:    1,
		StateFilePath: "migrator.state",
	}
}

// ProtocolConfig defines protocol specific configuration.
type ProtocolConfig struct {
	// The minimum PoW score needed.
	MinPoWScore float64
	// The coo public key ranges.
	PublicKeyRanges []coopkg.PublicKeyRange
	// The network ID on which this node operates on.
	NetworkIDName string
}

// CLIFlags returns the config as CLI flags.
func (protoConfig *ProtocolConfig) CLIFlags() []string {

	keyRangesJSON, err := json.Marshal(protoConfig.PublicKeyRanges)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal COO public key ranges: %s", err))
	}

	return []string{
		fmt.Sprintf("--%s=%0.0f", protocfg.CfgProtocolMinPoWScore, protoConfig.MinPoWScore),
		fmt.Sprintf("--%s=%s", protocfg.CfgProtocolPublicKeyRangesJSON, string(keyRangesJSON)),
		fmt.Sprintf("--%s=%s", protocfg.CfgProtocolNetworkIDName, protoConfig.NetworkIDName),
	}
}

// DefaultProtocolConfig returns the default protocol config.
func DefaultProtocolConfig() ProtocolConfig {
	return ProtocolConfig{
		MinPoWScore: 100,
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
		NetworkIDName: "alphanet1",
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
