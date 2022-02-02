package toolset

import (
	stded25519 "crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"
	flag "github.com/spf13/pflag"

	"github.com/libp2p/go-libp2p-core/crypto"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
)

func generateP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	databasePathFlag := fs.String(FlagToolDatabasePath, "", "the path to the p2p database folder (optional)")
	privateKeyFlag := fs.String(FlagToolPrivateKey, "", "the p2p private key (optional)")
	outputJSONFlag := fs.Bool(FlagToolOutputJSON, false, FlagToolDescriptionOutputJSON)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", ToolP2PIdentityGen)
		fs.PrintDefaults()
		println(fmt.Sprintf("\nexample: %s --%s %s --%s %s",
			ToolP2PIdentityGen,
			FlagToolDatabasePath,
			"p2pstore",
			FlagToolPrivateKey,
			"[PRIVATE_KEY]",
		))
	}

	if err := parseFlagSet(fs, args); err != nil {
		return err
	}

	dbPath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if databasePathFlag != nil && len(*databasePathFlag) > 0 {
		dbPath = *databasePathFlag
	}

	privKeyFilePath := filepath.Join(dbPath, p2p.PrivKeyFileName)

	if err := os.MkdirAll(dbPath, 0700); err != nil {
		return fmt.Errorf("could not create peer store database dir '%s': %w", dbPath, err)
	}

	_, err := os.Stat(privKeyFilePath)
	switch {
	case err == nil || os.IsExist(err):
		// private key file already exists
		return fmt.Errorf("private key file (%s) already exists", privKeyFilePath)

	case os.IsNotExist(err):
		// private key file does not exist, create a new one

	default:
		return fmt.Errorf("unable to check private key file (%s): %w", privKeyFilePath, err)
	}

	var privateKey crypto.PrivKey
	var publicKey crypto.PubKey

	if privateKeyFlag != nil && len(*privateKeyFlag) > 0 {
		hivePrivKey, err := utils.ParseEd25519PrivateKeyFromString(*privateKeyFlag)
		if err != nil {
			return fmt.Errorf("invalid private key given '%s': %w", *privateKeyFlag, err)
		}

		stdPrvKey := stded25519.PrivateKey(hivePrivKey)
		privateKey, publicKey, err = crypto.KeyPairFromStdKey(&stdPrvKey)
		if err != nil {
			return fmt.Errorf("unable to convert given private key '%s': %w", *privateKeyFlag, err)
		}
	} else {
		// create identity
		privateKey, publicKey, err = crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return fmt.Errorf("unable to generate Ed25519 private key for peer identity: %w", err)
		}
	}

	if err := p2p.WriteEd25519PrivateKeyToPEMFile(privKeyFilePath, privateKey); err != nil {
		return fmt.Errorf("writing private key file for peer identity failed: %w", err)
	}

	return printP2PIdentity(privateKey, publicKey, *outputJSONFlag)
}

func printP2PIdentity(privateKey crypto.PrivKey, publicKey crypto.PubKey, outputJSON bool) error {

	type P2PIdentity struct {
		PrivateKey      string `json:"privateKey"`
		PublicKey       string `json:"publicKey"`
		PublicKeyBase58 string `json:"publicKeyBase58"`
		PeerID          string `json:"peerId"`
	}

	privKeyBytes, err := privateKey.Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw private key bytes: %w", err)
	}

	pubKeyBytes, err := publicKey.Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw public key bytes: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("unable to get peer identity from public key: %w", err)
	}

	identity := P2PIdentity{
		PrivateKey:      hex.EncodeToString(privKeyBytes),
		PublicKey:       hex.EncodeToString(pubKeyBytes),
		PublicKeyBase58: base58.Encode(pubKeyBytes),
		PeerID:          peerID.String(),
	}

	if outputJSON {
		output, err := json.MarshalIndent(identity, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(output))
		return nil
	}

	fmt.Println("Your p2p private key (hex):   ", identity.PrivateKey)
	fmt.Println("Your p2p public key (hex):    ", identity.PublicKey)
	fmt.Println("Your p2p public key (base58): ", identity.PublicKeyBase58)
	fmt.Println("Your p2p PeerID:              ", identity.PeerID)
	return nil
}
