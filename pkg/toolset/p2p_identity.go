package toolset

import (
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/configuration"
)

func generateP2PIdentity(nodeConfig *configuration.Configuration, args []string) error {

	if len(args) > 0 {
		return fmt.Errorf("too many arguments for '%s'", ToolP2PIdentity)
	}

	// create identity
	privateKey, publicKey, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
	if err != nil {
		panic(err)
	}

	// obtain Peer ID from public key
	pid, err := peer.IDFromPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	privateKeyBytes, err := privateKey.Raw()
	if err != nil {
		panic(err)
	}

	publicKeyBytes, err := publicKey.Raw()
	if err != nil {
		panic(err)
	}

	fmt.Println("Your p2p private key (hex): ", hex.EncodeToString(privateKeyBytes))
	fmt.Println("Your p2p public key (hex): ", hex.EncodeToString(publicKeyBytes))
	fmt.Println("Your p2p public key (base58): ", base58.Encode(publicKeyBytes))
	fmt.Println("Your p2p PeerID: ", pid.String())
	fmt.Println()
	fmt.Println("Make sure to specify the private key within the 'p2p.identityPrivateKey' config option to use it for your node's identity")

	return nil
}
