package toolset

import (
	"fmt"
	"os"
	"strings"
)

var (
	tools = map[string]func([]string) error{
		"pwdhash": hashPasswordAndSalt,
		"seedgen": seedGen,
		"list":    listTools,
		"merkle":  merkleTreeCreate,
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
		fmt.Print("no tool specified. enter 'tool list' to list available tools.\n\n")
		listTools([]string{})
		os.Exit(1)
	}

	tool, exists := tools[strings.ToLower(args[1])]
	if !exists {
		fmt.Print("Tool not found.\n\n")
		listTools([]string{})
		os.Exit(1)
	}

	if err := tool(args[2:]); err != nil {
		fmt.Printf("\nError: %v\n", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func listTools(args []string) error {
	fmt.Println("pwdhash: generates a sha265 sum from your password and salt")
	fmt.Println("seedgen: generates an autopeering seed")
	fmt.Println("merkle: generates a Merkle tree for coordinator plugin")

	return nil
}
