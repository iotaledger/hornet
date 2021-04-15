package toolset

import (
	"fmt"
	"os"
	"strings"
)

const (
	ToolPwdHash      = "pwdhash"
	ToolP2PIdentity  = "p2pidentity"
	ToolEd25519Key   = "ed25519key"
	ToolEd25519Addr  = "ed25519addr"
	ToolSnapGen      = "snapgen"
	ToolSnapMerge    = "snapmerge"
	ToolSnapInfo     = "snapinfo"
	ToolBenchmarkIO  = "bench-io"
	ToolBenchmarkCPU = "bench-cpu"
)

// HandleTools handles available tools.
func HandleTools() {
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

	tools := map[string]func([]string) error{
		ToolPwdHash:      hashPasswordAndSalt,
		ToolEd25519Key:   generateEd25519Key,
		ToolEd25519Addr:  generateEd25519Address,
		ToolSnapGen:      snapshotGen,
		ToolSnapMerge:    snapshotMerge,
		ToolSnapInfo:     snapshotInfo,
		ToolP2PIdentity:  generateP2PIdentity,
		ToolBenchmarkIO:  benchmarkIO,
		ToolBenchmarkCPU: benchmarkCPU,
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools([]string{})
		os.Exit(1)
	}

	if err := tool(args[2:]); err != nil {
		fmt.Printf("\nerror: %s\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func listTools(args []string) error {
	fmt.Println(fmt.Sprintf("%-15s generates a scrypt hash from your password and salt", fmt.Sprintf("%s:", ToolPwdHash)))
	fmt.Println(fmt.Sprintf("%-15s generates an p2p identity", fmt.Sprintf("%s:", ToolP2PIdentity)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 key pair", fmt.Sprintf("%s:", ToolEd25519Key)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 address from a public key", fmt.Sprintf("%s:", ToolEd25519Addr)))
	fmt.Println(fmt.Sprintf("%-15s generates an initial snapshot for a private network", fmt.Sprintf("%s:", ToolSnapGen)))
	fmt.Println(fmt.Sprintf("%-15s merges a full and delta snapshot into an updated full snapshot", fmt.Sprintf("%s:", ToolSnapMerge)))
	fmt.Println(fmt.Sprintf("%-15s outputs information about a snapshot file", fmt.Sprintf("%s:", ToolSnapInfo)))
	fmt.Println(fmt.Sprintf("%-15s benchmarks the IO throughput", fmt.Sprintf("%s:", ToolBenchmarkIO)))
	fmt.Println(fmt.Sprintf("%-15s benchmarks the CPU performance", fmt.Sprintf("%s:", ToolBenchmarkCPU)))

	return nil
}
