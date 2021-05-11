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

	salt := nodeConfig.String(restapi.CfgRestAPIAuthSalt)
	if len(salt) == 0 {
		panic(fmt.Sprintf("'%s' should not be empty", restapi.CfgRestAPIAuthSalt))
	}

	prvKey, pid, err := loadP2PPrivKeyAndIdentityFromStore(nodeConfig.String(p2p.CfgP2PPeerStorePath))
	if err != nil {
		panic(err)
	}

	// API tokens do not expire.
	jwtAuth := jwt.NewJWTAuth(salt,
		0,
		pid.String(),
		prvKey,
	)

	token, err := jwtAuth.IssueJWT(true, false)
	if err != nil {
		panic(err)
	}

	fmt.Println("Your API JWT token: ", string(token))

	return nil
}
