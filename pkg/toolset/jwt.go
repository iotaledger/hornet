package toolset

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/iotaledger/hive.go/configuration"

	"github.com/libp2p/go-libp2p-core/peer"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/plugins/restapi"
)

func generateJWTApiToken(nodeConfig *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [P2P_DATABASE_PATH]", ToolJWTApi))
		println()
		println("	[P2P_DATABASE_PATH] - the path to the p2p database folder (optional)")
		println()
		println(fmt.Sprintf("example: %s %s", ToolJWTApi, "p2pstore"))
	}

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolJWTApi)
	}

	salt := nodeConfig.String(restapi.CfgRestAPIJWTAuthSalt)
	if len(salt) == 0 {
		return fmt.Errorf("'%s' should not be empty", restapi.CfgRestAPIJWTAuthSalt)
	}

	p2pDatabasePath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if len(args) > 0 {
		p2pDatabasePath = args[0]
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

	fmt.Println("Your API JWT token: ", jwtToken)

	return nil
}
