package toolset

import (
	"encoding/hex"
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

func generateP2PIdentity(args []string) error {

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

	fmt.Println("Your p2p private key: ", hex.EncodeToString(privateKeyBytes))
	fmt.Println("Your p2p public key: ", hex.EncodeToString(publicKeyBytes))
	fmt.Println("Your p2p PeerID: ", pid.String())

	return nil
}
