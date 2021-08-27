package toolset

import (
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

func generateEd25519Key(_ *configuration.Configuration, args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolEd25519Key)
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	addr := iotago.AddressFromEd25519PubKey(pubKey)

	fmt.Println("Your ed25519 private key: ", hex.EncodeToString(privKey))
	fmt.Println("Your ed25519 public key:  ", hex.EncodeToString(pubKey))
	fmt.Println("Your ed25519 address:     ", hex.EncodeToString(addr[:]))
	fmt.Println("Your bech32 address:      ", addr.Bech32(iotago.PrefixTestnet))

	return nil
}

func generateEd25519Address(_ *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [ED25519_PUB_KEY]", ToolEd25519Addr))
		println()
		println("	[ED25519_PUB_KEY] - an ed25519 public key")
		println()
		println(fmt.Sprintf("example: %s %s", ToolEd25519Addr, "88457c9836b9b2c3cdf363ab9516aa5a9786f5c6ceff426efe25147c58b57839"))
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
		return fmt.Errorf("can't decode ED25519_PUB_KEY: %w", err)
	}

	addr := iotago.AddressFromEd25519PubKey(pubKey)

	fmt.Println("Your ed25519 address: ", hex.EncodeToString(addr[:]))
	fmt.Println("Your bech32 address:  ", addr.Bech32(iotago.PrefixTestnet))
	return nil
}
