package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/iotaledger/hive.go/configuration"
)

func extractP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "", "the path to the p2p database folder (optional)")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolP2PExtractIdentity)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolP2PExtractIdentity,
			FlagToolDatabasePath,
			"p2pstore"))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	dbPath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if databasePathFlag != nil && len(*databasePathFlag) > 0 {
		dbPath = *databasePathFlag
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

	return printP2PIdentity(privKey, privKey.GetPublic(), *outputJSONFlag)
}
