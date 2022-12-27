package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/peer"
	flag "github.com/spf13/pflag"

	"github.com/iotaledger/hornet/pkg/jwt"
	"github.com/iotaledger/hornet/pkg/p2p"
)

func generateJWTApiToken(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, DefaultValueP2PDatabasePath, "the path to the p2p database folder")
	apiJWTSaltFlag := fs.String(FlagToolSalt, DefaultValueAPIJWTTokenSalt, "salt used inside the JWT tokens for the REST API")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolJWTApi)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolJWTApi,
			FlagToolDatabasePath,
			DefaultValueP2PDatabasePath,
			FlagToolSalt,
			DefaultValueAPIJWTTokenSalt))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*databasePathFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolDatabasePath)
	}
	if len(*apiJWTSaltFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolSalt)
	}

	databasePath := *databasePathFlag
	privKeyFilePath := filepath.Join(databasePath, p2p.PrivKeyFileName)

	salt := *apiJWTSaltFlag

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

	if *outputJSONFlag {

		result := struct {
			JWT string `json:"jwt"`
		}{
			JWT: jwtToken,
		}

		return printJSON(result)
	}

	fmt.Println("Your API JWT token: ", jwtToken)
	return nil
}
