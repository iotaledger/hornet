package toolset

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"unicode"

	"golang.org/x/crypto/ssh/terminal"
)

func hashPasswordAndSalt(args []string) {

	if len(args) > 0 {
		fmt.Println("Too many arguments for 'pwdhash'")
		os.Exit(0)
	}

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

	for _, r := range salt {
		if unicode.IsUpper(r) {
			fmt.Printf("\nSalt (%s) contains upper cased characters\n", salt)
			os.Exit(1)
		}
	}

	hash := sha256.Sum256(append([]byte(password), []byte(salt)...))

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %s\n", hash, salt)
	os.Exit(0)
}
