package toolset

import (
	"fmt"
	"os"
	"strings"

	"github.com/iotaledger/hive.go/configuration"
)

const (
	ToolPwdHash            = "pwdhash"
	ToolP2PIdentity        = "p2pidentity"
	ToolP2PExtractIdentity = "p2pidentityextract"
	ToolEd25519Key         = "ed25519key"
	ToolEd25519Addr        = "ed25519addr"
	ToolJWTApi             = "jwt-api"
	ToolSnapGen            = "snapgen"
	ToolSnapMerge          = "snapmerge"
	ToolSnapInfo           = "snapinfo"
	ToolBenchmarkIO        = "bench-io"
	ToolBenchmarkCPU       = "bench-cpu"
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
		listTools([]string{})
		os.Exit(1)
	}

	tools := map[string]func(*configuration.Configuration, []string) error{
		ToolPwdHash:            hashPasswordAndSalt,
		ToolP2PIdentity:        generateP2PIdentity,
		ToolP2PExtractIdentity: extractP2PIdentity,
		ToolEd25519Key:         generateEd25519Key,
		ToolEd25519Addr:        generateEd25519Address,
		ToolJWTApi:             generateJWTApiToken,
		ToolSnapGen:            snapshotGen,
		ToolSnapMerge:          snapshotMerge,
		ToolSnapInfo:           snapshotInfo,
		ToolBenchmarkIO:        benchmarkIO,
		ToolBenchmarkCPU:       benchmarkCPU,
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools([]string{})
		os.Exit(1)
	}

	if err := tool(nodeConfig, args[2:]); err != nil {
		fmt.Printf("\nerror: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func listTools(args []string) error {
	fmt.Println(fmt.Sprintf("%-15s generates a scrypt hash from your password and salt", fmt.Sprintf("%s:", ToolPwdHash)))
	fmt.Println(fmt.Sprintf("%-15s generates an p2p identity", fmt.Sprintf("%s:", ToolP2PIdentity)))
	fmt.Println(fmt.Sprintf("%-15s extracts the p2p identity from the given store", fmt.Sprintf("%s:", ToolP2PExtractIdentity)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 key pair", fmt.Sprintf("%s:", ToolEd25519Key)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 address from a public key", fmt.Sprintf("%s:", ToolEd25519Addr)))
	fmt.Println(fmt.Sprintf("%-15s generates a JWT token for REST-API access", fmt.Sprintf("%s:", ToolJWTApi)))
	fmt.Println(fmt.Sprintf("%-15s generates an initial snapshot for a private network", fmt.Sprintf("%s:", ToolSnapGen)))
	fmt.Println(fmt.Sprintf("%-15s merges a full and delta snapshot into an updated full snapshot", fmt.Sprintf("%s:", ToolSnapMerge)))
	fmt.Println(fmt.Sprintf("%-15s outputs information about a snapshot file", fmt.Sprintf("%s:", ToolSnapInfo)))
	fmt.Println(fmt.Sprintf("%-15s benchmarks the IO throughput", fmt.Sprintf("%s:", ToolBenchmarkIO)))
	fmt.Println(fmt.Sprintf("%-15s benchmarks the CPU performance", fmt.Sprintf("%s:", ToolBenchmarkCPU)))

	return nil
}
