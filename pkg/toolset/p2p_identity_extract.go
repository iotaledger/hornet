package toolset

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/gohornet/hornet/pkg/p2p"
)

func loadP2PPrivKeyAndIdentityFromStore(peerStorePath string) (crypto.PrivKey, peer.ID, error) {

	pubKeyFilePath := fmt.Sprintf("%s/%s", peerStorePath, p2p.PubKeyFileName)

	peerID, err := p2p.LoadIdentityFromFile(pubKeyFilePath)
	if err != nil {
		panic(err)
	}

	peerStore, err := p2p.NewPeerstore(peerStorePath)
	if err != nil {
		panic(err)
	}

	prvKey, err := p2p.LoadPrivateKeyFromStore(peerID, peerStore)
	if err != nil {
		panic(err)
	}

	return prvKey, peerID, nil
}

func extractP2PIdentity(args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [P2P_STORE_PATH]", ToolP2PExtractIdentity))
		println()
		println("	[P2P_STORE_PATH]	- the path to the p2p store")
		println()
		println(fmt.Sprintf("example: %s %s", ToolP2PExtractIdentity, "./p2pstore"))
	}

	if len(args) != 1 {
		printUsage()
		return fmt.Errorf("wrong argument count '%s'", ToolP2PExtractIdentity)
	}

	prvKey, pid, err := loadP2PPrivKeyAndIdentityFromStore(args[0])
	if err != nil {
		panic(err)
	}

	prvKeyBytes, err := prvKey.Raw()
	if err != nil {
		log.Panicf("unable to convert private key to bytes: %v", err)
	}

	pubKeyBytes, err := prvKey.GetPublic().Raw()
	if err != nil {
		log.Panicf("unable to convert public key to bytes: %v", err)
	}

	fmt.Println("Your p2p private key: ", hex.EncodeToString(prvKeyBytes))
	fmt.Println("Your p2p public key: ", hex.EncodeToString(pubKeyBytes))
	fmt.Println("Your p2p PeerID: ", pid.String())
	return nil
}
