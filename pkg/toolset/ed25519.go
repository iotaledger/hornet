package toolset

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/utils"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

func generateEd25519Key(args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolEd25519Key)
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	fmt.Println("Your ed25519 private key: ", hex.EncodeToString(privKey))
	fmt.Println("Your ed25519 public key: ", hex.EncodeToString(pubKey))
	return nil
}

func generateEd25519Address(args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [ED25519_PUB_KEY]", ToolEd25519Addr))
		println()
		println("	[ED25519_PUB_KEY] - an ed25519 public key")
	}

	// check arguments
	if len(args) == 0 {
		printUsage()
		return errors.New("ED25519_PUB_KEY missing")
	}

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolEd25519Addr)
	}

	// parse pubkey
	pubKey, err := utils.ParseEd25519PublicKeyFromString(args[0])
	if err != nil {
		return fmt.Errorf("can't decode ED25519_PUB_KEY: %v", err)
	}

	addr := iotago.AddressFromEd25519PubKey(pubKey)

	fmt.Println("Your ed25519 address: ", hex.EncodeToString(addr[:]))
	return nil
}
