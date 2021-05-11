package toolset

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"

	badger "github.com/ipfs/go-ds-badger"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-peerstore/pstoreds"
)

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

	dirPath := args[0]

	badgerStore, err := badger.NewDatastore(dirPath, &badger.DefaultOptions)
	if err != nil {
		log.Panicf("unable to initialize data store for peer store: %s", err)
	}

	peerStore, err := pstoreds.NewPeerstore(context.Background(), badgerStore, pstoreds.DefaultOpts())
	if err != nil {
		log.Panicf("unable to initialize peer store: %s", err)
	}

	pubKeyFilePath := fmt.Sprintf("%s/key.pub", dirPath)
	log.Printf("retrieving existing peer identity from %s", pubKeyFilePath)
	existingPubKeyBytes, err := ioutil.ReadFile(pubKeyFilePath)
	if err != nil {
		log.Panicf("unable to read public key identity file: %v", err)
	}

	pubKey, err := crypto.UnmarshalPublicKey(existingPubKeyBytes)
	if err != nil {
		log.Panicf("unable to unmarshal public key from public key identity file: %v", err)
	}
	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		log.Panicf("unable to convert read public key to peer ID: %v", err)
	}

	// retrieve this node's private key from the peer store
	prvKey := peerStore.PrivKey(peerID)

	prvKeyBytes, err := prvKey.Raw()
	if err != nil {
		log.Panicf("unable to convert private key to bytes: %v", err)
	}

	pubKeyBytes, err := pubKey.Raw()
	if err != nil {
		log.Panicf("unable to convert public key to bytes: %v", err)
	}

	pid, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		log.Panicf("unable to obtain peer identity from public key: %v", err)
	}

	fmt.Println("Your p2p private key: ", hex.EncodeToString(prvKeyBytes))
	fmt.Println("Your p2p public key: ", hex.EncodeToString(pubKeyBytes))
	fmt.Println("Your p2p PeerID: ", pid.String())
	return nil
}
