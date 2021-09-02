package toolset

import (
	stded25519 "crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/iotaledger/hive.go/configuration"
)

func generateP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {

	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [P2P_DATABASE_PATH] [P2P_PRIVATE_KEY]", ToolP2PIdentityGen))
		println()
		println("	[P2P_DATABASE_PATH] - the path to the p2p database folder (optional)")
		println("	[P2P_PRIVATE_KEY]   - the p2p private key (optional)")
		println()
		println(fmt.Sprintf("example: %s %s", ToolP2PIdentityGen, "p2pstore"))
	}

	if len(args) > 2 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolP2PIdentityGen)
	}

	p2pDatabasePath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if len(args) > 0 {
		p2pDatabasePath = args[0]
	}
	privKeyFilePath := filepath.Join(p2pDatabasePath, p2p.PrivKeyFileName)

	if err := os.MkdirAll(p2pDatabasePath, 0700); err != nil {
		return fmt.Errorf("could not create peer store database dir '%s': %w", p2pDatabasePath, err)
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

	if len(args) > 1 {
		hivePrivKey, err := utils.ParseEd25519PrivateKeyFromString(args[1])
		if err != nil {
			return fmt.Errorf("invalid private key given '%s': %w", args[1], err)
		}

		stdPrvKey := stded25519.PrivateKey(hivePrivKey)
		privateKey, publicKey, err = crypto.KeyPairFromStdKey(&stdPrvKey)
		if err != nil {
			return fmt.Errorf("unable to convert given private key '%s': %w", args[1], err)
		}
	} else {
		// create identity
		privateKey, publicKey, err = crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return fmt.Errorf("unable to generate Ed25519 private key for peer identity: %w", err)
		}
	}

	// obtain Peer ID from public key
	peerID, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		return fmt.Errorf("unable to get peer identity from public key: %w", err)
	}

	privKeyBytes, err := privateKey.Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw private key bytes: %w", err)
	}

	pubKeyBytes, err := publicKey.Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw public key bytes: %w", err)
	}

	if err := p2p.WriteEd25519PrivateKeyToPEMFile(privKeyFilePath, privateKey); err != nil {
		return fmt.Errorf("writing private key file for peer identity failed: %w", err)
	}

	fmt.Println("Your p2p private key (hex):   ", hex.EncodeToString(privKeyBytes))
	fmt.Println("Your p2p public key (hex):    ", hex.EncodeToString(pubKeyBytes))
	fmt.Println("Your p2p public key (base58): ", base58.Encode(pubKeyBytes))
	fmt.Println("Your p2p PeerID:              ", peerID.String())

	return nil
}
