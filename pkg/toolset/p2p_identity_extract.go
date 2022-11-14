package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/core/certificate"
	"github.com/iotaledger/hive.go/core/configuration"
	hivep2p "github.com/iotaledger/hive.go/core/p2p"
	"github.com/iotaledger/hornet/v2/pkg/p2p"
)

func extractP2PIdentity(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, DefaultValueP2PDatabasePath, "the path to the p2p database folder")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolP2PExtractIdentity)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolP2PExtractIdentity,
			FlagToolDatabasePath,
			DefaultValueP2PDatabasePath))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}

	databasePath := *databasePathFlag
	privKeyFilePath := filepath.Join(databasePath, p2p.PrivKeyFileName)

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

	privKey, err := certificate.ReadEd25519PrivateKeyFromPEMFile(privKeyFilePath)
	if err != nil {
		return fmt.Errorf("reading private key file for peer identity failed: %w", err)
	}

	libp2pPrivKey, err := hivep2p.Ed25519PrivateKeyToLibp2pPrivateKey(privKey)
	if err != nil {
		return err
	}

	return printP2PIdentity(libp2pPrivKey, libp2pPrivKey.GetPublic(), *outputJSONFlag)
}
