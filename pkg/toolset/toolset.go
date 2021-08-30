package toolset

import (
	"fmt"
	"os"
	"strings"

	"github.com/iotaledger/hive.go/configuration"
)

const (
	ToolPwdHash            = "pwd-hash"
	ToolP2PIdentityGen     = "p2pidentity-gen"
	ToolP2PExtractIdentity = "p2pidentity-extract"
	ToolEd25519Key         = "ed25519-key"
	ToolEd25519Addr        = "ed25519-addr"
	ToolJWTApi             = "jwt-api"
	ToolSnapGen            = "snap-gen"
	ToolSnapMerge          = "snap-merge"
	ToolSnapInfo           = "snap-info"
	ToolBenchmarkIO        = "bench-io"
	ToolBenchmarkCPU       = "bench-cpu"
	ToolDatabaseMigration  = "db-migration"
)

// HandleTools handles available tools.
func HandleTools(nodeConfig *configuration.Configuration) {
	args := os.Args[1:]

	toolFound := false
	for i, arg := range args {
		if strings.ToLower(arg) == "tool" || strings.ToLower(arg) == "tools" {
			args = args[i:]
			toolFound = true
			break
		}
	}

	if !toolFound {
		// 'tool' was not found
		return
	}

	if len(args) == 1 {
		listTools()
		os.Exit(1)
	}

	tools := map[string]func(*configuration.Configuration, []string) error{
		ToolPwdHash:            hashPasswordAndSalt,
		ToolP2PIdentityGen:     generateP2PIdentity,
		ToolP2PExtractIdentity: extractP2PIdentity,
		ToolEd25519Key:         generateEd25519Key,
		ToolEd25519Addr:        generateEd25519Address,
		ToolJWTApi:             generateJWTApiToken,
		ToolSnapGen:            snapshotGen,
		ToolSnapMerge:          snapshotMerge,
		ToolSnapInfo:           snapshotInfo,
		ToolBenchmarkIO:        benchmarkIO,
		ToolBenchmarkCPU:       benchmarkCPU,
		ToolDatabaseMigration:  databaseMigration,
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools()
		os.Exit(1)
	}

	if err := tool(nodeConfig, args[2:]); err != nil {
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
	fmt.Printf("%-20s benchmarks the IO throughput\n", fmt.Sprintf("%s:", ToolBenchmarkIO))
	fmt.Printf("%-20s benchmarks the CPU performance\n", fmt.Sprintf("%s:", ToolBenchmarkCPU))
	fmt.Printf("%-20s migrates the database to another engine\n", fmt.Sprintf("%s:", ToolDatabaseMigration))
}
