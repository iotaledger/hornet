package toolset

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"
)

const (
	FlagToolDatabasePath         = "databasePath"
	FlagToolDatabasePathSource   = "sourceDatabasePath"
	FlagToolDatabasePathTarget   = "targetDatabasePath"
	FlagToolDatabaseEngineTarget = "targetDatabaseEngine"

	FlagToolSnapshotPath      = "snapshotPath"
	FlagToolSnapshotPathFull  = "fullSnapshotPath"
	FlagToolSnapshotPathDelta = "deltaSnapshotPath"

	FlagToolPrivateKey = "privateKey"
	FlagToolPublicKey  = "publicKey"

	FlagToolHRP      = "hrp"
	FlagToolPassword = "password"

	FlagToolOutputJSON            = "json"
	FlagToolDescriptionOutputJSON = "format output as JSON"
)

const (
	ToolPwdHash                 = "pwd-hash"
	ToolP2PIdentityGen          = "p2pidentity-gen"
	ToolP2PExtractIdentity      = "p2pidentity-extract"
	ToolEd25519Key              = "ed25519-key"
	ToolEd25519Addr             = "ed25519-addr"
	ToolJWTApi                  = "jwt-api"
	ToolSnapGen                 = "snap-gen"
	ToolSnapMerge               = "snap-merge"
	ToolSnapInfo                = "snap-info"
	ToolSnapHash                = "snap-hash"
	ToolBenchmarkIO             = "bench-io"
	ToolBenchmarkCPU            = "bench-cpu"
	ToolDatabaseMigration       = "db-migration"
	ToolDatabaseLedgerHash      = "db-hash"
	ToolDatabaseHealth          = "db-health"
	ToolDatabaseSplit           = "db-split"
	ToolCoordinatorFixStateFile = "coo-fix-state"
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
func HandleTools(nodeConfig *configuration.Configuration) {

	args := os.Args[1:]
	if len(args) == 1 {
		listTools()
		os.Exit(1)
	}

	tools := map[string]func(*configuration.Configuration, []string) error{
		ToolPwdHash:                 hashPasswordAndSalt,
		ToolP2PIdentityGen:          generateP2PIdentity,
		ToolP2PExtractIdentity:      extractP2PIdentity,
		ToolEd25519Key:              generateEd25519Key,
		ToolEd25519Addr:             generateEd25519Address,
		ToolJWTApi:                  generateJWTApiToken,
		ToolSnapGen:                 snapshotGen,
		ToolSnapMerge:               snapshotMerge,
		ToolSnapInfo:                snapshotInfo,
		ToolSnapHash:                snapshotHash,
		ToolBenchmarkIO:             benchmarkIO,
		ToolBenchmarkCPU:            benchmarkCPU,
		ToolDatabaseMigration:       databaseMigration,
		ToolDatabaseLedgerHash:      databaseLedgerHash,
		ToolDatabaseHealth:          databaseHealth,
		ToolDatabaseSplit:           databaseSplit,
		ToolCoordinatorFixStateFile: coordinatorFixStateFile,
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools()
		os.Exit(1)
	}

	if err := tool(nodeConfig, args[2:]); err != nil {
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
	fmt.Printf("%-20s migrates the database to another engine\n", fmt.Sprintf("%s:", ToolDatabaseMigration))
	fmt.Printf("%-20s calculates the sha256 hash of the ledger state of a database\n", fmt.Sprintf("%s:", ToolDatabaseLedgerHash))
	fmt.Printf("%-20s checks the health status of the database\n", fmt.Sprintf("%s:", ToolDatabaseHealth))
	fmt.Printf("%-20s split a legacy database into `tangle` and `utxo`\n", fmt.Sprintf("%s:", ToolDatabaseSplit))
	fmt.Printf("%-20s applies the latest milestone in the database to the coordinator state file\n", fmt.Sprintf("%s:", ToolCoordinatorFixStateFile))
}

func yesOrNo(value bool) string {
	if value {
		return "YES"
	}
	return "NO"
}

func parseFlagSet(fs *flag.FlagSet, args []string, minArgsCount ...int) error {

	if err := fs.Parse(args); err != nil {
		return err
	}

	if len(minArgsCount) > 0 {
		// minimum amount of args must be checked
		if len(args) < minArgsCount[0] {
			fs.Usage()
			return errors.New("not enough arguments")
		}
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
