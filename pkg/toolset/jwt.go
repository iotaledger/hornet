package toolset

import (
	"fmt"

	"github.com/iotaledger/hive.go/configuration"

	"github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/jwt"
	"github.com/gohornet/hornet/plugins/restapi"
)

func generateJWTApiToken(nodeConfig *configuration.Configuration, args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolJWTApi)
	}

	salt := nodeConfig.String(restapi.CfgRestAPIJWTAuthSalt)
	if len(salt) == 0 {
		return fmt.Errorf("'%s' should not be empty", restapi.CfgRestAPIJWTAuthSalt)
	}

	prvKey, pid, err := loadP2PPrivKeyAndIdentityFromStore(nodeConfig.String(p2p.CfgP2PPeerStorePath))
	if err != nil {
		return fmt.Errorf("loading private key from p2p store failed: %w", err)
	}

	// API tokens do not expire.
	jwtAuth, err := jwt.NewJWTAuth(salt,
		0,
		pid.String(),
		prvKey,
	)
	if err != nil {
		return fmt.Errorf("JWT auth initialization failed: %w", err)
	}

	token, err := jwtAuth.IssueJWT(true, false)
	if err != nil {
		return fmt.Errorf("issuing JWT token failed: %w", err)
	}

	fmt.Println("Your API JWT token: ", string(token))

	return nil
}
