package toolset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/configuration"
	hivedb "github.com/iotaledger/hive.go/core/database"
)

const (
	FlagToolDatabaseEngine       = "databaseEngine"
	FlagToolDatabaseEngineSource = "sourceDatabaseEngine"
	FlagToolDatabaseEngineTarget = "targetDatabaseEngine"

	FlagToolConfigFilePath = "configFile"

	FlagToolDatabasePath       = "databasePath"
	FlagToolDatabasePathSource = "sourceDatabasePath"
	FlagToolDatabasePathTarget = "targetDatabasePath"

	FlagToolProtocolParametersPath = "protocolParametersPath"
	FlagToolCoordinatorStatePath   = "cooStatePath"
	FlagToolGenesisAddressesPath   = "genesisAddressesPath"
	FlagToolGenesisAddresses       = "genesisAddresses"

	FlagToolSnapshotPath       = "snapshotPath"
	FlagToolSnapshotPathFull   = "fullSnapshotPath"
	FlagToolSnapshotPathDelta  = "deltaSnapshotPath"
	FlagToolSnapshotPathTarget = "targetSnapshotPath"
	FlagToolSnapshotGlobal     = "global"

	FlagToolOutputPath = "outputPath"

	FlagToolPrivateKey = "privateKey"
	FlagToolPublicKey  = "publicKey"

	FlagToolHRP       = "hrp"
	FlagToolBIP32Path = "bip32Path"
	FlagToolMnemonic  = "mnemonic"
	FlagToolPassword  = "password"
	FlagToolSalt      = "salt"

	FlagToolNodeURL = "nodeURL"

	FlagToolOutputJSON            = "json"
	FlagToolDescriptionOutputJSON = "format output as JSON"

	FlagToolBenchmarkCount    = "count"
	FlagToolBenchmarkSize     = "size"
	FlagToolBenchmarkThreads  = "threads"
	FlagToolBenchmarkDuration = "duration"

	FlagToolSnapGenMintAddress        = "mintAddress"
	FlagToolSnapGenTreasuryAllocation = "treasuryAllocation"

	FlagToolDatabaseTargetIndex = "targetIndex"
)

const (
	ToolPwdHash                = "pwd-hash"
	ToolP2PIdentityGen         = "p2pidentity-gen"
	ToolP2PExtractIdentity     = "p2pidentity-extract"
	ToolEd25519Key             = "ed25519-key"
	ToolEd25519Addr            = "ed25519-addr"
	ToolJWTApi                 = "jwt-api"
	ToolSnapGen                = "snap-gen"
	ToolSnapMerge              = "snap-merge"
	ToolSnapInfo               = "snap-info"
	ToolSnapHash               = "snap-hash"
	ToolBenchmarkIO            = "bench-io"
	ToolBenchmarkCPU           = "bench-cpu"
	ToolDatabaseLedgerHash     = "db-hash"
	ToolDatabaseHealth         = "db-health"
	ToolDatabaseMerge          = "db-merge"
	ToolDatabaseMigration      = "db-migration"
	ToolDatabaseSnapshot       = "db-snapshot"
	ToolDatabaseVerify         = "db-verify"
	ToolBootstrapPrivateTangle = "bootstrap-private-tangle"
	ToolNodeInfo               = "node-info"
)

const (
	DefaultValueAPIJWTTokenSalt     = "HORNET"
	DefaultValueMainnetDatabasePath = "mainnetdb"
	DefaultValueP2PDatabasePath     = "p2pstore"
	DefaultValueDatabaseEngine      = hivedb.EngineRocksDB
)

const (
	passwordEnvKey = "HORNET_TOOL_PASSWORD"

	// printStatusInterval is the interval for printing status messages.
	printStatusInterval = 2 * time.Second
)

// ShouldHandleTools checks if tools were requested.
func ShouldHandleTools() bool {
	args := os.Args[1:]

	for _, arg := range args {
		if strings.ToLower(arg) == "tool" || strings.ToLower(arg) == "tools" {
			return true
		}
	}

	return false
}

// HandleTools handles available tools.
func HandleTools() {

	args := os.Args[1:]
	if len(args) == 1 {
		listTools()
		os.Exit(1)
	}

	tools := map[string]func([]string) error{
		ToolPwdHash:                hashPasswordAndSalt,
		ToolP2PIdentityGen:         generateP2PIdentity,
		ToolP2PExtractIdentity:     extractP2PIdentity,
		ToolEd25519Key:             generateEd25519Key,
		ToolEd25519Addr:            generateEd25519Address,
		ToolJWTApi:                 generateJWTApiToken,
		ToolSnapGen:                snapshotGen,
		ToolSnapMerge:              snapshotMerge,
		ToolSnapInfo:               snapshotInfo,
		ToolSnapHash:               snapshotHash,
		ToolBenchmarkIO:            benchmarkIO,
		ToolBenchmarkCPU:           benchmarkCPU,
		ToolDatabaseLedgerHash:     databaseLedgerHash,
		ToolDatabaseHealth:         databaseHealth,
		ToolDatabaseMerge:          databaseMerge,
		ToolDatabaseMigration:      databaseMigration,
		ToolDatabaseSnapshot:       databaseSnapshot,
		ToolDatabaseVerify:         databaseVerify,
		ToolBootstrapPrivateTangle: networkBootstrap,
		ToolNodeInfo:               nodeInfo,
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools()
		os.Exit(1)
	}

	if err := tool(args[2:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// help text was requested
			os.Exit(0)
		}

		fmt.Printf("\nerror: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func listTools() {
	fmt.Printf("%-20s generates a scrypt hash from your password and salt\n", fmt.Sprintf("%s:", ToolPwdHash))
	fmt.Printf("%-20s generates a p2p identity private key file\n", fmt.Sprintf("%s:", ToolP2PIdentityGen))
	fmt.Printf("%-20s extracts the p2p identity from the private key file\n", fmt.Sprintf("%s:", ToolP2PExtractIdentity))
	fmt.Printf("%-20s generates an ed25519 key pair\n", fmt.Sprintf("%s:", ToolEd25519Key))
	fmt.Printf("%-20s generates an ed25519 address from a public key\n", fmt.Sprintf("%s:", ToolEd25519Addr))
	fmt.Printf("%-20s generates a JWT token for REST-API access\n", fmt.Sprintf("%s:", ToolJWTApi))
	fmt.Printf("%-20s generates an initial snapshot for a private network\n", fmt.Sprintf("%s:", ToolSnapGen))
	fmt.Printf("%-20s merges a full and delta snapshot into an updated full snapshot\n", fmt.Sprintf("%s:", ToolSnapMerge))
	fmt.Printf("%-20s outputs information about a snapshot file\n", fmt.Sprintf("%s:", ToolSnapInfo))
	fmt.Printf("%-20s calculates the sha256 hash of the ledger state inside a snapshot file\n", fmt.Sprintf("%s:", ToolSnapHash))
	fmt.Printf("%-20s benchmarks the IO throughput\n", fmt.Sprintf("%s:", ToolBenchmarkIO))
	fmt.Printf("%-20s benchmarks the CPU performance\n", fmt.Sprintf("%s:", ToolBenchmarkCPU))
	fmt.Printf("%-20s calculates the sha256 hash of the ledger state of a database\n", fmt.Sprintf("%s:", ToolDatabaseLedgerHash))
	fmt.Printf("%-20s checks the health status of the database\n", fmt.Sprintf("%s:", ToolDatabaseHealth))
	fmt.Printf("%-20s merges missing tangle data from a database to another one\n", fmt.Sprintf("%s:", ToolDatabaseMerge))
	fmt.Printf("%-20s migrates the database to another engine\n", fmt.Sprintf("%s:", ToolDatabaseMigration))
	fmt.Printf("%-20s creates a full snapshot from a database\n", fmt.Sprintf("%s:", ToolDatabaseSnapshot))
	fmt.Printf("%-20s verifies a valid ledger state and the existence of all blocks\n", fmt.Sprintf("%s:", ToolDatabaseVerify))
	fmt.Printf("%-20s bootstraps a private tangle by creating a snapshot, database and coordinator state file\n", fmt.Sprintf("%s:", ToolBootstrapPrivateTangle))
	fmt.Printf("%-20s queries the info endpoint of a node\n", fmt.Sprintf("%s:", ToolNodeInfo))
}

func yesOrNo(value bool) string {
	if value {
		return "YES"
	}

	return "NO"
}

func parseFlagSet(fs *flag.FlagSet, args []string) error {

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if all parameters were parsed
	if fs.NArg() != 0 {
		return errors.New("too much arguments")
	}

	return nil
}

func printJSON(obj interface{}) error {
	output, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(output))

	return nil
}

func loadConfigFile(filePath string, parameters map[string]any) error {
	config := configuration.New()
	flagset := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)

	for namespace, pointerToStruct := range parameters {
		config.BindParameters(flagset, namespace, pointerToStruct)
	}

	if err := config.LoadFile(filePath); err != nil {
		return fmt.Errorf("loading config file failed: %w", err)
	}

	config.UpdateBoundParameters()

	return nil
}

func getGracefulStopContext() context.Context {

	ctx, cancel := context.WithCancel(context.Background())

	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		<-gracefulStop
		cancel()
	}()

	return ctx
}
