package toolset

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/iotaledger/iota.go/v3/ed25519"
)

type keys struct {
	PublicKey      string `json:"publicKey"`
	PrivateKey     string `json:"privateKey,omitempty"`
	Ed25519Address string `json:"ed25519"`
	Bech32Address  string `json:"bech32"`
}

func printEd25519Info(pubKey ed25519.PublicKey, privKey ed25519.PrivateKey, hrp iotago.NetworkPrefix, outputJSON bool) {

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
		output, err := json.MarshalIndent(k, "", "  ")
		if err != nil {
			fmt.Printf("Error: %s\n", err)
		}
		fmt.Println(string(output))
		return
	}

	if len(k.PrivateKey) > 0 {
		fmt.Println("Your ed25519 private key: ", k.PrivateKey)
	}
	fmt.Println("Your ed25519 public key:  ", k.PublicKey)
	fmt.Println("Your ed25519 address:     ", k.Ed25519Address)
	fmt.Println("Your bech32 address:      ", k.Bech32Address)
}

func generateEd25519Key(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	outputJSON := fs.Bool("json", false, "format output as JSON")
	hrp := fs.String("hrp", string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Key)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if all parameters were parsed
	if fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
	}

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	printEd25519Info(pubKey, privKey, iotago.NetworkPrefix(*hrp), *outputJSON)
	return nil
}

func generateEd25519Address(_ *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ExitOnError)
	publicKey := fs.String("publicKey", "", "an ed25519 public key")
	outputJSON := fs.Bool("json", false, "format output as JSON")
	hrp := fs.String("hrp", string(iotago.PrefixTestnet), "the HRP which should be used for the Bech32 address")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolEd25519Addr)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if all parameters were parsed
	if len(args) == 0 || fs.NArg() != 0 {
		fs.Usage()
		os.Exit(2)
	}

	// parse pubkey
	pubKey, err := utils.ParseEd25519PublicKeyFromString(*publicKey)
	if err != nil {
		return fmt.Errorf("can't decode publicKey: %w", err)
	}

	printEd25519Info(pubKey, nil, iotago.NetworkPrefix(*hrp), *outputJSON)

	return nil
}
