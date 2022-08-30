package toolset

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"
	"github.com/wollac/iota-crypto-demo/pkg/bip32path"
	"github.com/wollac/iota-crypto-demo/pkg/bip39"
	"github.com/wollac/iota-crypto-demo/pkg/slip10"
	"github.com/wollac/iota-crypto-demo/pkg/slip10/eddsa"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto"
	iotago "github.com/iotaledger/iota.go/v3"
)

func printEd25519Info(mnemonic bip39.Mnemonic, path bip32path.Path, prvKey ed25519.PrivateKey, pubKey ed25519.PublicKey, hrp iotago.NetworkPrefix, outputJSON bool) error {

	addr := iotago.Ed25519AddressFromPubKey(pubKey)

	type keys struct {
		BIP39          string `json:"mnemonic,omitempty"`
		BIP32          string `json:"path,omitempty"`
		PrivateKey     string `json:"privateKey,omitempty"`
		PublicKey      string `json:"publicKey"`
		Ed25519Address string `json:"ed25519"`
		Bech32Address  string `json:"bech32"`
	}

	k := keys{
		PublicKey:      hex.EncodeToString(pubKey),
		Ed25519Address: hex.EncodeToString(addr[:]),
		Bech32Address:  addr.Bech32(hrp),
	}

	if prvKey != nil {
		k.PrivateKey = hex.EncodeToString(prvKey)
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

	if k.PrivateKey != "" {
		fmt.Println("Your ed25519 private key: ", k.PrivateKey)
	}

	fmt.Println("Your ed25519 public key:  ", k.PublicKey)
	fmt.Println("Your ed25519 address:     ", k.Ed25519Address)
	fmt.Println("Your bech32 address:      ", k.Bech32Address)

	return nil
}

func generateEd25519Key(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	hrpFlag := fs.String(FlagToolHRP, string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")
	bip32Path := fs.String(FlagToolBIP32Path, "m/44'/4218'/0'/0'/0'", "the BIP32 path that should be used to derive keys from seed")
	mnemonicFlag := fs.String(FlagToolMnemonic, "", "the BIP-39 mnemonic sentence that should be used to derive the seed from (optional)")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Key)
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

	var mnemonicSentence bip39.Mnemonic
	if len(*mnemonicFlag) == 0 {
		// Generate random entropy by using ed25519 key generation and using the private key seed (32 bytes)
		_, random, err := ed25519.GenerateKey(nil)
		if err != nil {
			return err
		}
		entropy := random.Seed()

		mnemonicSentence, err = bip39.EntropyToMnemonic(entropy)
		if err != nil {
			return err
		}
	} else {
		mnemonicSentence = bip39.ParseMnemonic(*mnemonicFlag)
		if len(mnemonicSentence) != 24 {
			return fmt.Errorf("'%s' contains an invalid sentence length. Mnemonic should be 24 words", FlagToolMnemonic)
		}
	}

	path, err := bip32path.ParsePath(*bip32Path)
	if err != nil {
		return err
	}

	seed, err := bip39.MnemonicToSeed(mnemonicSentence, "")
	if err != nil {
		return err
	}

	key, err := slip10.DeriveKeyFromPath(seed, eddsa.Ed25519(), path)
	if err != nil {
		return err
	}
	pubKey, prvKey := key.Key.(eddsa.Seed).Ed25519Key()

	return printEd25519Info(mnemonicSentence, path, ed25519.PrivateKey(prvKey), ed25519.PublicKey(pubKey), iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}

func generateEd25519Address(args []string) error {

	fs := configuration.NewUnsortedFlagSet("", flag.ContinueOnError)
	hrpFlag := fs.String(FlagToolHRP, string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")
	publicKeyFlag := fs.String(FlagToolPublicKey, "", "an ed25519 public key")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Addr)
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

	if len(*publicKeyFlag) == 0 {
		return fmt.Errorf("'%s' not specified", FlagToolPublicKey)
	}

	// parse pubkey
	pubKey, err := crypto.ParseEd25519PublicKeyFromString(*publicKeyFlag)
	if err != nil {
		return fmt.Errorf("can't decode '%s': %w", FlagToolPublicKey, err)
	}

	return printEd25519Info(nil, nil, nil, pubKey, iotago.NetworkPrefix(*hrpFlag), *outputJSONFlag)
}
