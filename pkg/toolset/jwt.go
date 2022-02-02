package toolset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hive.go/configuration"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/plugins/restapi"
)

func generateJWTApiToken(nodeConfig *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePath := fs.String(FlagToolDatabasePath, "", "the path to the p2p database folder (optional)")
	outputJSON := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolJWTApi)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	// Check if all parameters were parsed
	if fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
	}

	salt := nodeConfig.String(restapi.CfgRestAPIJWTAuthSalt)
	if len(salt) == 0 {
		return fmt.Errorf("'%s' should not be empty", restapi.CfgRestAPIJWTAuthSalt)
	}

	p2pDatabasePath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if len(*databasePath) > 0 {
		p2pDatabasePath = *databasePath
	}
	privKeyFilePath := filepath.Join(p2pDatabasePath, p2p.PrivKeyFileName)

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

	peerID, err := peer.IDFromPublicKey(privKey.GetPublic())
	if err != nil {
		return fmt.Errorf("unable to get peer identity from public key: %w", err)
	}

	// API tokens do not expire.
	jwtAuth, err := jwt.NewJWTAuth(salt,
		0,
		peerID.String(),
		privKey,
	)
	if err != nil {
		return fmt.Errorf("JWT auth initialization failed: %w", err)
	}

	jwtToken, err := jwtAuth.IssueJWT(true, false)
	if err != nil {
		return fmt.Errorf("issuing JWT token failed: %w", err)
	}

	if *outputJSON {

		result := struct {
			JWT string `json:"jwt"`
		}{
			JWT: jwtToken,
		}

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		fmt.Println(string(output))
		return nil
	}

	fmt.Println("Your API JWT token: ", jwtToken)
	return nil
}
