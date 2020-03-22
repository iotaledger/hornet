package toolset

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh/terminal"
)

var (
	tools map[string]func()
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

	if len(args) > 2 {
		fmt.Println("Too many arguments")
		os.Exit(0)
	}

	// register tools
	tools = make(map[string]func())
	tools["pwdhash"] = hashPasswordAndSalt
	tools["seedgen"] = seedGen
	tools["list"] = listTools

	for tool, f := range tools {
		if strings.ToLower(args[1]) == tool {
			f()
		}
	}

	fmt.Println("Tool not found")
	os.Exit(1)
}

func hashPasswordAndSalt() {
	reader := bufio.NewReader(os.Stdin)

	// Get terminal state to be able to restore it in case of an interrupt
	originalTerminalState, err := terminal.GetState(int(syscall.Stdin))
	if err != nil {
		fmt.Println("Failed to get terminal state")
		os.Exit(1)
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		// Reset the terminal to the original state if we receive an interrupt
		terminal.Restore(int(syscall.Stdin), originalTerminalState)
		fmt.Println("\nAborted... Bye!")
		os.Exit(1)
	}()

	fmt.Print("Enter a password: ")
	bytePassword, err := terminal.ReadPassword(0)
	if err != nil {
		panic(err)
	}
	password := string(bytePassword)

	fmt.Print("\nRe-Enter your password: ")
	bytePasswordReenter, err := terminal.ReadPassword(0)
	if err != nil {
		panic(err)
	}
	if password != string(bytePasswordReenter) {
		fmt.Println("\nRe-Entered password doesn't match")
		os.Exit(1)
	}

	fmt.Print("\nEnter a salt (lower cased): ")
	salt, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	salt = strings.TrimSuffix(salt, "\n")

	for _, rune := range []rune(salt) {
		if unicode.IsUpper(rune) {
			fmt.Printf("\nSalt (%s) contains upper cased characters\n", salt)
			os.Exit(1)
		}
	}

	hash := sha256.Sum256(append([]byte(password), []byte(salt)...))

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %s\n", hash, salt)
	os.Exit(0)
}

func seedGen() {
	rand.Seed(time.Now().UnixNano())

	var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, 32)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	fmt.Println("Your autopeering seed: ", base64.StdEncoding.EncodeToString([]byte(string(b))))
	os.Exit(0)
}

func listTools() {
	fmt.Println("pwdhash: Generates an sha265 sum from your password and salt")
	fmt.Println("seedgen: Generates an autopeering seed")
	os.Exit(0)
}
