package toolset

import (
	"encoding/hex"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/wollac/iota-crypto-demo/pkg/bip32path"
	"github.com/wollac/iota-crypto-demo/pkg/bip39"
	"github.com/wollac/iota-crypto-demo/pkg/slip10"

	"github.com/gohornet/hornet/pkg/utils"
	iotago "github.com/iotaledger/iota.go/v2"
	"github.com/iotaledger/iota.go/v2/ed25519"
)

func printEd25519Info(mnemonic bip39.Mnemonic, path bip32path.Path, pubKey ed25519.PublicKey, hrp iotago.NetworkPrefix, outputJSON bool) error {

	addr := iotago.AddressFromEd25519PubKey(pubKey)

	type keys struct {
		BIP39          string `json:"mnemonic,omitempty"`
		BIP32          string `json:"path,omitempty"`
		PublicKey      string `json:"publicKey"`
		Ed25519Address string `json:"ed25519"`
		Bech32Address  string `json:"bech32"`
	}

	k := keys{
		PublicKey:      hex.EncodeToString(pubKey),
		Ed25519Address: hex.EncodeToString(addr[:]),
		Bech32Address:  addr.Bech32(hrp),
	}

	if mnemonic != nil {
		k.BIP39 = mnemonic.String()
		k.BIP32 = path.String()
	}

	if outputJSON {
		return printJSON(k)
	}

	if len(k.BIP39) > 0 {
		fmt.Println("Your seed BIP39 mnemonic: ", k.BIP39)
		fmt.Println()
		fmt.Println("Your BIP32 path:          ", k.BIP32)
	}
	fmt.Println("Your ed25519 public key:  ", k.PublicKey)
	fmt.Println("Your ed25519 address:     ", k.Ed25519Address)
	fmt.Println("Your bech32 address:      ", k.Bech32Address)

	return nil
}

func generateEd25519Key(args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	hrpFlag := fs.String(FlagToolHRP, string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")
	bip32Path := fs.String(FlagToolBIP32Path, "m/44'/4218'/0'/0'/0'", "the BIP32 path that should be used to derive keys from seed")
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

	if len(*bip32Path) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolBIP32Path)
	}

	path, err := bip32path.ParsePath(*bip32Path)
	if err != nil {
		return err
	}

	// Generate random entropy by using ed25519 key generation and using the private key seed (32 bytes)
	_, random, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}
	entropy := random.Seed()

	mnemonic, err := bip39.EntropyToMnemonic(entropy)
	if err != nil {
		return err
	}

	seed, err := bip39.MnemonicToSeed(mnemonic, "")
	if err != nil {
		return err
	}

	key, err := slip10.DeriveKeyFromPath(seed, slip10.Ed25519(), path)
	if err != nil {
		return err
	}
	pubKey, _ := slip10.Ed25519Key(key)
	return printEd25519Info(mnemonic, path, ed25519.PublicKey(pubKey), iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}

func generateEd25519Address(args []string) error {

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

	return printEd25519Info(nil, nil, pubKey, iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}
