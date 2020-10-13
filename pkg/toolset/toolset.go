package toolset

import (
	"fmt"
	"os"
	"strings"
)

const (
	ToolPwdHash     = "pwdhash"
	ToolSeedGen     = "seedgen"
	ToolEd25519Key  = "ed25519key"
	ToolEd25519Addr = "ed25519addr"
	ToolSnapGen     = "snapgen"
)

var (
	tools = map[string]func([]string) error{
		ToolPwdHash:     hashPasswordAndSalt,
		ToolSeedGen:     seedGen,
		ToolEd25519Key:  generateEd25519Key,
		ToolEd25519Addr: generateEd25519Address,
		ToolSnapGen:     snapshotGen,
	}
)

// HandleTools handles available tools.
func HandleTools() {
	args := os.Args[1:]

	toolFound := false
	for i, arg := range args {
		if strings.ToLower(arg) == "tool" {
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

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("tool not found.\n\n")
		listTools([]string{})
		os.Exit(1)
	}

	if err := tool(args[2:]); err != nil {
		fmt.Printf("\nerror: %v\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func listTools(args []string) error {
	fmt.Println(fmt.Sprintf("%-15s generates a sha265 sum from your password and salt", fmt.Sprintf("%s:", ToolPwdHash)))
	fmt.Println(fmt.Sprintf("%-15s generates an autopeering seed", fmt.Sprintf("%s:", ToolSeedGen)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 key pair", fmt.Sprintf("%s:", ToolEd25519Key)))
	fmt.Println(fmt.Sprintf("%-15s generates an ed25519 address from a public key", fmt.Sprintf("%s:", ToolEd25519Addr)))
	fmt.Println(fmt.Sprintf("%-15s generates an initial snapshot for a private network", fmt.Sprintf("%s:", ToolSnapGen)))

	return nil
}
