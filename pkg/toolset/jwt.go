package toolset

import (
	"encoding/hex"
	"fmt"
	"log"

	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/jwt"
	"github.com/iotaledger/hornet/plugins/webapi"
)

func generateJWTApiToken(args []string) error {
	// get nodes private key
	privKey := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthPrivateKey)
	privKeyFilePath := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthPrivateKeyPath)

	// load up the previously generated identity or create a new one
	jwtPrivateKey, _, err := webapi.LoadOrCreateIdentityPrivateKey(privKeyFilePath, privKey)
	if err != nil {
		log.Panic(err)
	}

	// create an ID by hashing the public key of the JWT private key
	jwtPublicKeyBytes, err := jwtPrivateKey.GetPublic().Raw()
	if err != nil {
		log.Panic(err)
	}
	jwtIDBytes := blake2b.Sum256(jwtPublicKeyBytes)
	jwtID := hex.EncodeToString(jwtIDBytes[:])

	// configure JWT auth
	salt := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthSalt)
	if len(salt) == 0 {
		log.Fatalf("'%s' should not be empty", config.CfgWebAPIJWTAuthSalt)
	}

	// API tokens do not expire.
	jwtAuth, err := jwt.NewAuth(salt,
		0,
		jwtID,
		jwtPrivateKey,
	)
	if err != nil {
		log.Fatalf("JWT auth initialization failed: %s", err)
	}

	jwtToken, err := jwtAuth.IssueJWT()
	if err != nil {
		return fmt.Errorf("issuing JWT token failed: %w", err)
	}

	fmt.Println("Your API JWT token: ", jwtToken)

	return nil
}
