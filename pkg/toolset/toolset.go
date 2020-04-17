package toolset

import (
	"fmt"
	"os"
	"strings"
)

// HandleTools handles available tools
func HandleTools() {
	args := os.Args[1:]
	if len(args) == 0 {
		return
	}

	if strings.ToLower(args[0]) != "tool" {
		return
	}

	if len(args) == 1 {
		fmt.Println("no tool specified. enter 'tool list' to list available tools.\n")
		listTools([]string{})
		os.Exit(1)
	}

	// register tools
	tools := make(map[string]func([]string) error)
	tools["pwdhash"] = hashPasswordAndSalt
	tools["seedgen"] = seedGen
	tools["list"] = listTools
	tools["merkle"] = merkleTreeCreate

	for tool, f := range tools {
		if strings.ToLower(args[1]) == tool {
			if err := f(args[2:]); err != nil {
				fmt.Printf("\nError: %v\n", err.Error())
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	fmt.Println("Tool not found.\n")
	listTools([]string{})
	os.Exit(1)
}

func listTools(args []string) error {
	fmt.Println("pwdhash: generates an sha265 sum from your password and salt")
	fmt.Println("seedgen: generates an autopeering seed")
	fmt.Println("merkle: generates a merkle tree for coordinator")

	return nil
}
