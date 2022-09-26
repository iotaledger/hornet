//nolint:forcetypeassert,varnamelen,revive,exhaustruct // we don't care about these linters in test cases
package framework

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"

	"github.com/iotaledger/hive.go/core/crypto"
	"github.com/iotaledger/hornet/v2/core/protocfg"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// RestAPIPort defines the default REST API port of every node.
	RestAPIPort = 14265

	/*
		// values used in the snapshot
		GenesisAddressPublicKeyHex = "f7868ab6bb55800b77b8b74191ad8285a9bf428ace579d541fda47661803ff44"
		GenesisAddressHex          = "6920b176f613ec7be59e68fc68f597eb3393af80f74c7c3db78198147d5f1f92"
		GenesisAddressBech32       = "atoi1qp5jpvtk7cf7c7l9ne50c684jl4n8ya0srm5clpak7qes9ratu0eysflmsz"
	*/

	autopeeringMaxTries = 50

	containerNodeImage           = "hornet:dev"
	coordinatorImage             = "iotaledger/inx-coordinator:1.0-rc"
	indexerImage                 = "iotaledger/inx-indexer:1.0-rc"
	containerWhiteFlagMockServer = "wfmock:latest"

	containerNameTester  = "/tester"
	containerNameReplica = "replica_"
	containerNameINX     = "inx_"

	logsDir = "/tmp/logs/"

	assetsDir = "/assets"

	dockerLogsPrefixLen    = 8
	exitStatusSuccessful   = 0
	containerNameEntryNode = "entry_node"
)

var (
	// GenesisSeed defines the seed on which the total supply resides on per default.
	GenesisSeed ed25519.PrivateKey
	// GenesisAddress defines the address computed from the GenesisSeed.
	GenesisAddress iotago.Ed25519Address
)

func init() {
	prvkey, err := crypto.ParseEd25519PrivateKeyFromString("256a818b2aac458941f7274985a410e57fb750f3a3a67969ece5bd9ae7eef5b2f7868ab6bb55800b77b8b74191ad8285a9bf428ace579d541fda47661803ff44")
	if err != nil {
		panic(err)
	}
	GenesisSeed = prvkey
	GenesisAddress = iotago.Ed25519AddressFromPubKey(GenesisSeed.Public().(ed25519.PublicKey))
}

// DefaultConfig returns the default AppConfig.
func DefaultConfig() *AppConfig {
	cfg := &AppConfig{
		Name: "",
		Envs: []string{"LOGGER_LEVEL=debug"},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
		Network:     DefaultNetworkConfig(),
		Snapshot:    DefaultSnapshotConfig(),
		Protocol:    DefaultProtocolConfig(),
		RestAPI:     DefaultRestAPIConfig(),
		INX:         DefaultINXConfig(),
		Profiling:   DefaultProfilingConfig(),
		Receipts:    DefaultNodeReceiptValidatorConfig(),
		Autopeering: DefaultAutopeeringConfig(),
		INXCoo:      DefaultINXCoordinatorConfig(),
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
func DefaultWhiteFlagMockServerConfig(name string, configFileName string) *WhiteFlagMockServerConfig {
	return &WhiteFlagMockServerConfig{
		Name: name,
		Envs: []string{
			fmt.Sprintf("WHITE_FLAG_MOCK_CONFIG=%s/%s", assetsDir, configFileName),
		},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
	}
}

type INXCoordinatorConfig struct {
	// Whether to let the node run as the coordinator.
	RunAsCoo bool
	// The name of this node.
	Name string
	// Environment variables.
	Envs []string
	// Binds for the container.
	Binds []string
	// INXAddress is the INX address of the node.
	INXAddress string
	// Coordinator config.
	Coordinator CoordinatorConfig
	// Migrator config.
	Migrator MigratorConfig
	// Receipt validator config.
	Validator ReceiptValidatorConfig
}

// DefaultINXCoordinatorConfig returns the default INX coordinator config.
func DefaultINXCoordinatorConfig() *INXCoordinatorConfig {
	return &INXCoordinatorConfig{
		RunAsCoo: false,
		Name:     "",
		Envs:     []string{"LOGGER_LEVEL=debug"},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
		Coordinator: DefaultCoordinatorConfig(),
		Migrator:    DefaultMigratorConfig(),
		Validator:   DefaultReceiptValidatorConfig(),
	}
}

// WithMigration adjusts the config to activate the migrator plugin.
func (cfg *INXCoordinatorConfig) WithMigration() {
	cfg.Migrator.Enabled = true
	cfg.Migrator.Bootstrap = true
}

// CLIFlags returns the config as CLI flags.
func (cfg *INXCoordinatorConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, fmt.Sprintf("--inx.address=%s", cfg.INXAddress))
	cliFlags = append(cliFlags, cfg.Coordinator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Migrator.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Validator.CLIFlags()...)

	return cliFlags
}

// INXIndexerConfig defines the config of an INX-Indexer.
type INXIndexerConfig struct {
	// The name of this node.
	Name string
	// Environment variables.
	Envs []string
	// Binds for the container.
	Binds       []string
	INXAddress  string
	BindAddress string
}

func DefaultINXIndexerConfig() *INXIndexerConfig {
	return &INXIndexerConfig{
		Name: "",
		Envs: []string{"LOGGER_LEVEL=debug"},
		Binds: []string{
			fmt.Sprintf("hornet-testing-assets:%s:rw", assetsDir),
		},
	}
}

func (cfg *INXIndexerConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, fmt.Sprintf("--inx.address=%s", cfg.INXAddress))
	cliFlags = append(cliFlags, fmt.Sprintf("--restAPI.bindAddress=%s", cfg.BindAddress))

	return cliFlags
}

// AppConfig defines the config of a HORNET node.
type AppConfig struct {
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
	// INX config.
	INX INXConfig
	// Snapshot config.
	Snapshot SnapshotConfig
	// Protocol config.
	Protocol ProtocolConfig
	// Profiling config.
	Profiling ProfilingConfig
	// Receipts config
	Receipts ReceiptsConfig
	// Autopeering config.
	Autopeering AutopeeringConfig
	// INXCoo inx-coordinator config.
	INXCoo *INXCoordinatorConfig
}

// AsCoo adjusts the config to make it usable as the Coordinator's config.
func (cfg *AppConfig) AsCoo() {
	cfg.INX.Enabled = true
	cfg.INXCoo.RunAsCoo = true
	cfg.INXCoo.Envs = append(cfg.INXCoo.Envs, fmt.Sprintf("COO_PRV_KEYS=%s", strings.Join(cfg.INXCoo.Coordinator.PrivateKeys, ",")))
}

// WithReceipts adjusts the config to activate the receipts plugin.
func (cfg *AppConfig) WithReceipts() {
	cfg.Receipts.Enabled = true
	cfg.INXCoo.WithMigration()
}

// CLIFlags returns the config as CLI flags.
func (cfg *AppConfig) CLIFlags() []string {
	var cliFlags []string
	cliFlags = append(cliFlags, cfg.Network.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Snapshot.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Protocol.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.RestAPI.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.INX.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Profiling.CLIFlags()...)
	cliFlags = append(cliFlags, cfg.Receipts.CLIFlags()...)
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
		fmt.Sprintf("--%s=%s", "p2p.identityPrivateKey", netConfig.IdentityPrivKey),
		fmt.Sprintf("--%s=%s", "p2p.bindMultiAddresses", strings.Join(netConfig.BindMultiAddresses, ",")),
		fmt.Sprintf("--%s=%s", "p2p.db.path", netConfig.DatabasePath),
		fmt.Sprintf("--%s=%d", "p2p.connectionManager.highWatermark", netConfig.ConnMngHighWatermark),
		fmt.Sprintf("--%s=%d", "p2p.connectionManager.lowWatermark", netConfig.ConnMngLowWatermark),
		fmt.Sprintf("--%s=%s", "p2p.peers", strings.Join(netConfig.Peers, ",")),
		fmt.Sprintf("--%s=%s", "p2p.peerAliases", strings.Join(netConfig.PeerAliases, ",")),
		fmt.Sprintf("--%s=%s", "p2p.reconnectInterval", netConfig.ReconnectInterval),
		fmt.Sprintf("--%s=%d", "p2p.gossip.unknownPeersLimit", netConfig.GossipUnknownPeersLimit),
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
	// Enabled defines whether the autopeering plugin is enabled.
	Enabled bool
	// The list of autopeering entry nodes to use.
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
		fmt.Sprintf("--%s=%v", "p2p.autopeering.enabled", autoConfig.Enabled),
		fmt.Sprintf("--%s=%s", "p2p.autopeering.entryNodes", strings.Join(autoConfig.EntryNodes, ",")),
		fmt.Sprintf("--%s=%s", "p2p.autopeering.bindAddress", autoConfig.BindAddr),
		fmt.Sprintf("--%s=%v", "p2p.autopeering.runAsEntryNode", autoConfig.RunAsEntryNode),
		fmt.Sprintf("--%s=%d", "p2p.autopeering.inboundPeers", autoConfig.InboundPeers),
		fmt.Sprintf("--%s=%d", "p2p.autopeering.outboundPeers", autoConfig.OutboundPeers),
		fmt.Sprintf("--%s=%s", "p2p.autopeering.saltLifetime", autoConfig.SaltLifetime),
	}
}

// DefaultAutopeeringConfig returns the default autopeering config.
func DefaultAutopeeringConfig() AutopeeringConfig {
	return AutopeeringConfig{
		Enabled:        false,
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
	// Public REST API routes.
	PublicRoutes []string
	// Protected REST API routes.
	ProtectedRoutes []string
	// Whether the node does proof-of-work for submitted blocks.
	PoWEnabled bool
}

// CLIFlags returns the config as CLI flags.
func (restAPIConfig *RestAPIConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", "restAPI.bindAddress", restAPIConfig.BindAddress),
		fmt.Sprintf("--%s=%s", "restAPI.publicRoutes", strings.Join(restAPIConfig.PublicRoutes, ",")),
		fmt.Sprintf("--%s=%s", "restAPI.protectedRoutes", strings.Join(restAPIConfig.ProtectedRoutes, ",")),
		fmt.Sprintf("--%s=%v", "restAPI.pow.enabled", restAPIConfig.PoWEnabled),
	}
}

// DefaultRestAPIConfig returns the default REST API config.
func DefaultRestAPIConfig() RestAPIConfig {
	return RestAPIConfig{
		BindAddress: "0.0.0.0:14265",
		PublicRoutes: []string{
			"/health",
			"/api/*",
		},
		ProtectedRoutes: []string{},
		PoWEnabled:      true,
	}
}

// INXConfig defines the INX specific configuration.
type INXConfig struct {
	// Enabled defines whether the INX plugin is enabled.
	Enabled bool
	// The bind address for the INX.
	BindAddress string
}

// CLIFlags returns the config as CLI flags.
func (inxConfig *INXConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%v", "inx.enabled", inxConfig.Enabled),
		fmt.Sprintf("--%s=%s", "inx.bindAddress", inxConfig.BindAddress),
	}
}

// DefaultINXConfig returns the default INX config.
func DefaultINXConfig() INXConfig {
	return INXConfig{
		Enabled:     false,
		BindAddress: "0.0.0.0:9029",
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
		fmt.Sprintf("--%s=%s", "snapshots.fullPath", snapshotConfig.FullSnapshotFilePath),
		fmt.Sprintf("--%s=%s", "snapshots.deltaPath", snapshotConfig.DeltaSnapshotFilePath),
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
	// The coo private keys.
	PrivateKeys []string
	// Whether to run the coordinator in bootstrap node.
	Bootstrap bool
	// The interval in which to issue new milestones.
	IssuanceInterval time.Duration
	// Block Backups Enabled.
	BlockBackupsEnabled bool
}

// CLIFlags returns the config as CLI flags.
func (cooConfig *CoordinatorConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--cooBootstrap=%v", cooConfig.Bootstrap),
		fmt.Sprintf("--coordinator.interval=%s", cooConfig.IssuanceInterval),
		fmt.Sprintf("--coordinator.blockBackups.enabled=%v", cooConfig.BlockBackupsEnabled),
	}
}

// DefaultCoordinatorConfig returns the default coordinator config.
func DefaultCoordinatorConfig() CoordinatorConfig {
	return CoordinatorConfig{
		Bootstrap: true,
		PrivateKeys: []string{"651941eddb3e68cb1f6ef4ef5b04625dcf5c70de1fdc4b1c9eadb2c219c074e0ed3c3f1a319ff4e909cf2771d79fece0ac9bd9fd2ee49ea6c0885c9cb3b1248c",
			"0e324c6ff069f31890d496e9004636fd73d8e8b5bea08ec58a4178ca85462325f6752f5f46a53364e2ee9c4d662d762a81efd51010282a75cd6bd03f28ef349c"},
		IssuanceInterval:    10 * time.Second,
		BlockBackupsEnabled: false,
	}
}

// ReceiptsConfig defines the receipt validator plugin specific configuration.
type ReceiptsConfig struct {
	// Enabled defines whether the receipts plugin is enabled.
	Enabled bool
	// Whether receipt backups are enabled.
	BackupEnabled bool
	// The path to the receipts folder.
	BackupFolder string
	// Whether the receipts plugin should validate receipts
	Validate bool
	// Whether to ignore soft errors or not.
	IgnoreSoftErrors bool
	// The validator config
	Validator ReceiptValidatorConfig
}

func (receiptsConfig *ReceiptsConfig) CLIFlags() []string {
	flags := []string{
		fmt.Sprintf("--%s=%v", "receipts.enabled", receiptsConfig.Enabled),
		fmt.Sprintf("--%s=%v", "receipts.backup.enabled", receiptsConfig.BackupEnabled),
		fmt.Sprintf("--%s=%s", "receipts.backup.path", receiptsConfig.BackupFolder),
		fmt.Sprintf("--%s=%v", "receipts.validator.validate", receiptsConfig.Validate),
		fmt.Sprintf("--%s=%v", "receipts.validator.ignoreSoftErrors", receiptsConfig.IgnoreSoftErrors),
	}
	flags = append(flags, receiptsConfig.Validator.CLIFlags()...)

	return flags
}

type ReceiptValidatorConfig struct {
	// The API to query.
	APIAddress string
	// The API timeout.
	APITimeout time.Duration
	// Legacy Coordinator address
	CoordinatorAddress string
	// The merkle tree depth.
	CoordinatorMerkleTreeDepth int
}

func DefaultNodeReceiptValidatorConfig() ReceiptsConfig {
	return ReceiptsConfig{
		Enabled:          false,
		BackupEnabled:    false,
		BackupFolder:     "receipts",
		Validate:         false,
		IgnoreSoftErrors: false,
		Validator:        DefaultReceiptValidatorConfig(),
	}
}

func (validatorConfig *ReceiptValidatorConfig) CLIFlags() []string {
	return []string{
		fmt.Sprintf("--%s=%s", "receipts.validator.api.address", validatorConfig.APIAddress),
		fmt.Sprintf("--%s=%s", "receipts.validator.api.timeout", validatorConfig.APITimeout),
		fmt.Sprintf("--%s=%s", "receipts.validator.coordinator.address", validatorConfig.CoordinatorAddress),
		fmt.Sprintf("--%s=%d", "receipts.validator.coordinator.merkleTreeDepth", validatorConfig.CoordinatorMerkleTreeDepth),
	}
}

// DefaultReceiptValidatorConfig returns the default receipt validator config.
func DefaultReceiptValidatorConfig() ReceiptValidatorConfig {
	return ReceiptValidatorConfig{
		APIAddress:                 "http://localhost:14265",
		APITimeout:                 5 * time.Second,
		CoordinatorAddress:         "JFQ999DVN9CBBQX9DSAIQRAFRALIHJMYOXAQSTCJLGA9DLOKIWHJIFQKMCQ9QHWW9RXQMDBVUIQNIY9GZ",
		CoordinatorMerkleTreeDepth: 18,
	}
}

// MigratorConfig defines migrator plugin specific configuration.
type MigratorConfig struct {
	// Enabled defines whether the migrator plugin is enabled.
	Enabled bool
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
		fmt.Sprintf("--migrator.enabled=%v", migConfig.Enabled),
		fmt.Sprintf("--migratorBootstrap=%v", migConfig.Bootstrap),
		fmt.Sprintf("--migrator.receiptMaxEntries=%v", migConfig.MaxEntries),
		fmt.Sprintf("--migratorStartIndex=%d", migConfig.StartIndex),
		fmt.Sprintf("--migrator.stateFilePath=%s", migConfig.StateFilePath),
	}
}

// DefaultMigratorConfig returns the default migrator plugin config.
func DefaultMigratorConfig() MigratorConfig {
	return MigratorConfig{
		Enabled:       false,
		Bootstrap:     false,
		MaxEntries:    iotago.MaxMigratedFundsEntryCount,
		StartIndex:    1,
		StateFilePath: "migrator.state",
	}
}

// ProtocolConfig defines protocol specific configuration.
type ProtocolConfig struct {
	// The initial network name on which this node operates on.
	TargetNetworkName string
	// The amount of public keys in a milestone.
	MilestonePublicKeyCount int
	// The coo public key ranges.
	PublicKeyRanges []protocfg.ConfigPublicKeyRange
}

// CLIFlags returns the config as CLI flags.
func (protoConfig *ProtocolConfig) CLIFlags() []string {

	keyRangesJSON, err := json.Marshal(protoConfig.PublicKeyRanges)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal COO public key ranges: %s", err))
	}

	return []string{
		fmt.Sprintf("--protocol.targetNetworkName=%s", protoConfig.TargetNetworkName),
		fmt.Sprintf("--protocol.milestonePublicKeyCount=%d", protoConfig.MilestonePublicKeyCount),
		fmt.Sprintf("--%s=%s", protocfg.CfgProtocolPublicKeyRangesJSON, string(keyRangesJSON)),
	}
}

// DefaultProtocolConfig returns the default protocol config.
func DefaultProtocolConfig() ProtocolConfig {
	return ProtocolConfig{
		TargetNetworkName:       "alphanet1",
		MilestonePublicKeyCount: 2,
		PublicKeyRanges: []protocfg.ConfigPublicKeyRange{
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
		fmt.Sprintf("--%s=%s", "profiling.bindAddress", profilingConfig.BindAddress),
	}
}

// DefaultProfilingConfig returns the default profiling config.
func DefaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		BindAddress: "0.0.0.0:6060",
	}
}
