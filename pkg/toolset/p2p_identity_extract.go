package toolset

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/configuration"

	p2pCore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/p2p"
)

func extractP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [P2P_DATABASE_PATH]", ToolP2PExtractIdentity))
		println()
		println("	[P2P_DATABASE_PATH] - the path to the p2p database folder (optional)")
		println()
		println(fmt.Sprintf("example: %s %s", ToolP2PExtractIdentity, "p2pstore"))
	}

	if len(args) > 1 {
		printUsage()
		return fmt.Errorf("too many arguments for '%s'", ToolP2PExtractIdentity)
	}

	p2pDatabasePath := nodeConfig.String(p2pCore.CfgP2PDatabasePath)
	if len(args) > 0 {
		p2pDatabasePath = args[0]
	}
	privKeyFilePath := filepath.Join(p2pDatabasePath, p2p.PrivKeyFileName)

	_, err := os.Stat(privKeyFilePath)
	switch {
	case os.IsNotExist(err):
		// private key does not exist
		return fmt.Errorf("private key file (%s) does not exist", privKeyFilePath)

	case err == nil || os.IsExist(err):
		// private key file exists

	default:
		return fmt.Errorf("unable to check private key file (%s): %w", privKeyFilePath, err)
	}

	privKey, err := p2p.ReadEd25519PrivateKeyFromPEMFile(privKeyFilePath)
	if err != nil {
		return fmt.Errorf("reading private key file for peer identity failed: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(privKey.GetPublic())
	if err != nil {
		return fmt.Errorf("unable to get peer identity from public key: %w", err)
	}

	privKeyBytes, err := privKey.Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw private key bytes: %w", err)
	}

	pubKeyBytes, err := privKey.GetPublic().Raw()
	if err != nil {
		return fmt.Errorf("unable to get raw public key bytes: %w", err)
	}

	fmt.Println("Your p2p private key (hex):   ", hex.EncodeToString(privKeyBytes))
	fmt.Println("Your p2p public key (hex):    ", hex.EncodeToString(pubKeyBytes))
	fmt.Println("Your p2p public key (base58): ", base58.Encode(pubKeyBytes))
	fmt.Println("Your p2p PeerID:              ", peerID.String())

	return nil
}
