package toolset

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
)

type keys struct {
	PublicKey      string `json:"publicKey"`
	PrivateKey     string `json:"privateKey,omitempty"`
	Ed25519Address string `json:"ed25519"`
	Bech32Address  string `json:"bech32"`
}

func printEd25519Info(pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, hrp iotago.NetworkPrefix, outputJSON bool) error {

	addr := iotago.Ed25519AddressFromPubKey(pubKey)

	k := keys{
		PublicKey:      hex.EncodeToString(pubKey),
		Ed25519Address: hex.EncodeToString(addr[:]),
		Bech32Address:  addr.Bech32(hrp),
	}

	if privKey != nil {
		k.PrivateKey = hex.EncodeToString(privKey)
	}

	if outputJSON {
		return printJSON(k)
	}

	if len(k.PrivateKey) > 0 {
		fmt.Println("Your ed25519 private key: ", k.PrivateKey)
	}
	fmt.Println("Your ed25519 public key:  ", k.PublicKey)
	fmt.Println("Your ed25519 address:     ", k.Ed25519Address)
	fmt.Println("Your bech32 address:      ", k.Bech32Address)

	return nil
}

func generateEd25519Key(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	hrpFlag := fs.String(FlagToolHRP, string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Key)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s",
			ToolEd25519Key,
			FlagToolHRP,
			string(iotago.PrefixTestnet)))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*hrpFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolHRP)
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	return printEd25519Info(pubKey, privKey, iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}

func generateEd25519Address(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	hrpFlag := fs.String(FlagToolHRP, string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")
	publicKeyFlag := fs.String(FlagToolPublicKey, "", "an ed25519 public key")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Addr)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolEd25519Addr,
			FlagToolHRP,
			string(iotago.PrefixTestnet),
			FlagToolPublicKey,
			"[PUB_KEY]",
		))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	if len(*hrpFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolHRP)
	}

	// parse pubkey
	pubKey, err := utils.ParseEd25519PublicKeyFromString(*publicKeyFlag)
	if err != nil {
		return fmt.Errorf("can't decode '%s': %w", FlagToolPublicKey, err)
	}

	return printEd25519Info(pubKey, nil, iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}
