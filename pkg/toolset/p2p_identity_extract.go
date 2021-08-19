package toolset

import (
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/mr-tron/base58"

	"github.com/iotaledger/hive.go/configuration"

	"github.com/gohornet/hornet/pkg/database"
	"github.com/gohornet/hornet/pkg/p2p"
)

func loadP2PPrivKeyAndIdentityFromStore(p2pDatabasePath string) (crypto.PrivKey, peer.ID, error) {

	pubKeyFilePath := filepath.Join(p2pDatabasePath, p2p.PubKeyFileName)

	peerID, err := p2p.LoadIdentityFromFile(pubKeyFilePath)
	if err != nil {
		return nil, "", err
	}

	peerStorePath := filepath.Join(p2pDatabasePath, "peers")

	dbInfoFilePath := filepath.Join(peerStorePath, "dbinfo")
	engine, err := database.LoadDatabaseEngineFromFile(dbInfoFilePath)
	if err != nil {
		return nil, "", err
	}

	peerStore, err := p2p.NewPeerStoreContainer(peerStorePath, engine, false)
	if err != nil {
		return nil, "", err
	}

	prvKey, err := p2p.LoadPrivateKeyFromStore(peerID, peerStore.Peerstore())
	if err != nil {
		return nil, "", err
	}

	return prvKey, peerID, nil
}

func extractP2PIdentity(_ *configuration.Configuration, args []string) error {
	printUsage := func() {
		println("Usage:")
		println(fmt.Sprintf("	%s [P2P_DATABASE_PATH]", ToolP2PExtractIdentity))
		println()
		println("	[P2P_DATABASE_PATH]	- the path to the p2p database folder")
		println()
		println(fmt.Sprintf("example: %s %s", ToolP2PExtractIdentity, "p2pstore"))
	}

	if len(args) != 1 {
		printUsage()
		return fmt.Errorf("wrong argument count for '%s'", ToolP2PExtractIdentity)
	}

	prvKey, pid, err := loadP2PPrivKeyAndIdentityFromStore(args[0])
	if err != nil {
		panic(err)
	}

	prvKeyBytes, err := prvKey.Raw()
	if err != nil {
		log.Panicf("unable to convert private key to bytes: %s", err)
	}

	pubKeyBytes, err := prvKey.GetPublic().Raw()
	if err != nil {
		log.Panicf("unable to convert public key to bytes: %s", err)
	}

	fmt.Println("Your p2p private key (hex):   ", hex.EncodeToString(prvKeyBytes))
	fmt.Println("Your p2p public key (hex):    ", hex.EncodeToString(pubKeyBytes))
	fmt.Println("Your p2p public key (base58): ", base58.Encode(pubKeyBytes))
	fmt.Println("Your p2p PeerID:              ", pid.String())
	return nil
}
