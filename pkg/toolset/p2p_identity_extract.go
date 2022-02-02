package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/configuration"
)

func extractP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	p2pDatabasePath := fs.String("p2pDatabasePath", "", "the path to the p2p database folder (optional)")
	outputJSON := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolP2PExtractIdentity)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	dbPath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if p2pDatabasePath != nil && len(*p2pDatabasePath) > 0 {
		dbPath = *p2pDatabasePath
	}
	privKeyFilePath := filepath.Join(dbPath, p2p.PrivKeyFileName)

	_, err := os.Stat(privKeyFilePath)
	switch {
	case os.IsNotExist(err):
		// private key does not exist
		return fmt.Errorf("private key file (%s) does not exist", privKeyFilePath)

	case err == nil || os.IsExist(err):
		// private key file exists

	default:
		return fmt.Errorf("unable to check private key file (%s): %w", privKeyFilePath, err)
	}

	privKey, err := p2p.ReadEd25519PrivateKeyFromPEMFile(privKeyFilePath)
	if err != nil {
		return fmt.Errorf("reading private key file for peer identity failed: %w", err)
	}

	return printP2PIdentity(privKey, privKey.GetPublic(), *outputJSON)
}
