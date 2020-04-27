package toolset

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

func hashPasswordAndSalt(args []string) error {

	if len(args) > 0 {
		return errors.New("too many arguments for 'pwdhash'")
	}

	reader := bufio.NewReader(os.Stdin)

	// get terminal state to be able to restore it in case of an interrupt
	originalTerminalState, err := terminal.GetState(int(syscall.Stdin))
	if err != nil {
		return errors.New("failed to get terminal state")
	}

	signalChan := make(chan os.Signal)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		// reset the terminal to the original state if we receive an interrupt
		terminal.Restore(int(syscall.Stdin), originalTerminalState)
		fmt.Println("\naborted... Bye!")
		os.Exit(1)
	}()

	fmt.Print("Enter a password: ")
	bytePassword, err := terminal.ReadPassword(0)
	if err != nil {
		return err
	}
	password := string(bytePassword)

	fmt.Print("\nRe-enter your password: ")
	bytePasswordReenter, err := terminal.ReadPassword(0)
	if err != nil {
		return err
	}
	if password != string(bytePasswordReenter) {
		return errors.New("re-entered password doesn't match")
	}

	fmt.Print("\nEnter a salt: ")
	salt, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	salt = strings.TrimSuffix(salt, "\n")

	hash := sha256.Sum256(append([]byte(password), []byte(salt)...))

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %s\n", hash, salt)

	return nil
}
