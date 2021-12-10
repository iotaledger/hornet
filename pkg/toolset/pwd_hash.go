package toolset

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	flag "github.com/spf13/pflag"

	"github.com/pkg/errors"
	"golang.org/x/term"

	"github.com/iotaledger/hive.go/configuration"

	"github.com/gohornet/hornet/pkg/basicauth"
	"github.com/gohornet/hornet/pkg/utils"
)

const (
	passwordEnvKey = "HORNET_TOOL_PASSWORD"
)

func readPasswordFromEnv() ([]byte, error) {
	passwordEnv, err := utils.LoadStringFromEnvironment(passwordEnvKey)
	if err != nil {
		return nil, err
	}
	return []byte(passwordEnv), nil
}

func readPasswordFromStdin() ([]byte, error) {
	var password []byte

	// get terminal state to be able to restore it in case of an interrupt
	originalTerminalState, err := term.GetState(int(syscall.Stdin))
	if err != nil {
		return nil, errors.New("failed to get terminal state")
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		// reset the terminal to the original state if we receive an interrupt
		_ = term.Restore(int(syscall.Stdin), originalTerminalState)
		fmt.Println("\naborted... Bye!")
		os.Exit(1)
	}()

	fmt.Print("Enter a password: ")
	password, err = term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("read password failed: %w", err)
	}

	fmt.Print("\nRe-enter your password: ")
	passwordReenter, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("read password failed: %w", err)
	}

	if !bytes.Equal(password, passwordReenter) {
		return nil, errors.New("re-entered password doesn't match")
	}
	fmt.Println()
	return password, nil
}

func hashPasswordAndSalt(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	passwordFlag := fs.String("password", "", fmt.Sprintf("password to hash (optional). Can also be passed as %s environment variable.", passwordEnvKey))
	outputJSON := fs.Bool("json", false, "format output as JSON")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolPwdHash)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if all parameters were parsed
	if fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
	}

	var password []byte

	if p, err := readPasswordFromEnv(); err == nil {
		// Password passed over the environment
		password = p
	} else if len(*passwordFlag) > 0 {
		// Password passed ofer flag
		password = []byte(*passwordFlag)
	} else {
		// Read from stdin
		p, err := readPasswordFromStdin()
		if err != nil {
			return err
		}
		password = p
	}

	passwordSalt, err := basicauth.SaltGenerator(32)
	if err != nil {
		return fmt.Errorf("generating random salt failed: %w", err)
	}

	passwordKey, err := basicauth.DerivePasswordKey(password, passwordSalt)
	if err != nil {
		return fmt.Errorf("deriving password key failed: %w", err)
	}

	if *outputJSON {

		result := struct {
			Password string `json:"passwordHash"`
			Salt     string `json:"passwordSalt"`
		}{
			Password: hex.EncodeToString(passwordKey),
			Salt:     hex.EncodeToString(passwordSalt),
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %x\n", passwordKey, passwordSalt)

	return nil
}
