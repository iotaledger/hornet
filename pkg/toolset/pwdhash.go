package toolset

import (
	"bytes"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/gohornet/hornet/pkg/basicauth"
)

func hashPasswordAndSalt(args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolPwdHash)
	}

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
	password, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}

	fmt.Print("\nRe-enter your password: ")
	passwordReenter, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return err
	}

	if !bytes.Equal(password, passwordReenter) {
		return errors.New("re-entered password doesn't match")
	}

	passwordSalt, err := basicauth.SaltGenerator(32)
	if err != nil {
		return err
	}

	passwordKey, err := basicauth.GetPasswordKey(password, passwordSalt)

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %x\n", passwordKey, passwordSalt)

	return nil
}
