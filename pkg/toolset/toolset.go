package toolset

import (
	"fmt"
	"os"
	"strings"
)

var (
	tools map[string]func([]string)
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
		fmt.Println("No tool specified. Enter 'tool list' to list available tools")
		os.Exit(1)
	}

	// register tools
	tools = make(map[string]func([]string))
	tools["pwdhash"] = hashPasswordAndSalt
	tools["seedgen"] = seedGen
	tools["list"] = listTools
	tools["merkle"] = merkleTree

	for tool, f := range tools {
		if strings.ToLower(args[1]) == tool {
			f(args[2:])
		}
	}

	fmt.Println("Tool not found")
	os.Exit(1)
}

func listTools(args []string) {
	fmt.Println("pwdhash: Generates an sha265 sum from your password and salt")
	fmt.Println("seedgen: Generates an autopeering seed")
	fmt.Println("merkle: Tools for coordinator merkle tree")
	os.Exit(0)
}
