package toolset

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/iotaledger/hive.go/core/basicauth"
	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hornet/v2/pkg/utils"
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
	originalTerminalState, err := term.GetState(syscall.Stdin)
	if err != nil {
		return nil, errors.New("failed to get terminal state")
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		// reset the terminal to the original state if we receive an interrupt
		_ = term.Restore(syscall.Stdin, originalTerminalState)
		fmt.Println("\naborted ... Bye!")
		os.Exit(1)
	}()

	fmt.Print("Enter a password: ")
	password, err = term.ReadPassword(syscall.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read password failed: %w", err)
	}

	fmt.Print("\nRe-enter your password: ")
	passwordReenter, err := term.ReadPassword(syscall.Stdin)
	if err != nil {
		return nil, fmt.Errorf("read password failed: %w", err)
	}

	if !bytes.Equal(password, passwordReenter) {
		return nil, errors.New("re-entered password doesn't match")
	}
	fmt.Println()

	return password, nil
}

func hashPasswordAndSalt(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	saltFlag := fs.String(FlagToolSalt, "", "salt used to hash the password (optional)")
	passwordFlag := fs.String(FlagToolPassword, "", fmt.Sprintf("password to hash. Can also be passed as %s environment variable.", passwordEnvKey))
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolPwdHash)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolPwdHash,
			FlagToolPassword,
			"[PASSWORD]",
		))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	var err error
	var passwordSalt []byte
	if len(*saltFlag) > 0 {
		// Salt passed over flag
		if len(*saltFlag) != 64 {
			return errors.New("the given salt must be 64 (hex encoded) in length")
		}
		passwordSalt, err = hex.DecodeString(*saltFlag)
		if err != nil {
			return fmt.Errorf("parsing given salt failed: %w", err)
		}
	} else {
		passwordSalt, err = basicauth.SaltGenerator(32)
		if err != nil {
			return fmt.Errorf("generating random salt failed: %w", err)
		}
	}

	var password []byte
	if p, err := readPasswordFromEnv(); err == nil {
		// Password passed over the environment
		password = p
	} else if len(*passwordFlag) > 0 {
		// Password passed over flag
		password = []byte(*passwordFlag)
	} else {
		// Read from stdin
		p, err := readPasswordFromStdin()
		if err != nil {
			return err
		}
		password = p
	}

	passwordKey, err := basicauth.DerivePasswordKey(password, passwordSalt)
	if err != nil {
		return fmt.Errorf("deriving password key failed: %w", err)
	}

	if *outputJSONFlag {

		result := struct {
			Password string `json:"passwordHash"`
			Salt     string `json:"passwordSalt"`
		}{
			Password: hex.EncodeToString(passwordKey),
			Salt:     hex.EncodeToString(passwordSalt),
		}

		return printJSON(result)
	}

	fmt.Printf("\nSuccess!\nYour hash: %x\nYour salt: %x\n", passwordKey, passwordSalt)

	return nil
}
