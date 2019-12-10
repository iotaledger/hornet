package snapshot

import (
	"strings"

	"github.com/iotaledger/hive.go/parameter"
	"github.com/iotaledger/iota.go/consts"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/compressed"
	"github.com/gohornet/hornet/packages/logger"
	"github.com/gohornet/hornet/packages/model/hornet"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/node"
)

var (
	PLUGIN = node.NewPlugin("Snapshot", node.Enabled, configure, run)
	log    = logger.NewLogger("Snapshot")

	NullHash = strings.Repeat("9", 81)
)

func configure(plugin *node.Plugin) {
	installGenesisTransaction()
}

func run(plugin *node.Plugin) {
	//ls, sa, err := LoadSnapshotFromFile(plugin, parameter.NodeConfig.GetString("localSnapshots.path"))

	localSnapshotsFile := parameter.NodeConfig.GetString("localSnapshots.path")
	loadGlobalSnapshot := parameter.NodeConfig.GetBool("loadGlobalSnapshot")
	if tangle.GetSnapshotInfo() == nil {
		var err error
		if loadGlobalSnapshot {
			err = LoadGlobalSnapshot("snapshotMainnet.txt", []string{"previousEpochsSpentAddresses1.txt", "previousEpochsSpentAddresses2.txt", "previousEpochsSpentAddresses3.txt"}, 1050000)
		} else if localSnapshotsFile != "" {
			err = LoadSnapshotFromFile(localSnapshotsFile)
		} else {
			err = LoadEmptySnapshot("snapshot.txt")
		}

		if err != nil {
			log.Panic(err.Error())
			return
		}
	} else {
		// Check the ledger state
		tangle.GetAllBalances()
	}
}

func installGenesisTransaction() {
	// ensure genesis transaction exists
	genesisTxTrits := make(trinary.Trits, consts.TransactionTrinarySize)
	genesis, _ := transaction.ParseTransaction(genesisTxTrits, true)
	genesis.Hash = NullHash
	txBytesTruncated := compressed.TruncateTx(trinary.TritsToBytes(genesisTxTrits))
	genesisTx := hornet.NewTransactionFromAPI(genesis, txBytesTruncated)
	tangle.StoreTransactionInCache(genesisTx)

	// ensure the bundle is also existent for the genesis tx
	genesisBundleBucket, err := tangle.GetBundleBucket(genesis.Bundle)
	if err != nil {
		log.Panic(err)
	}
	genesisBundleBucket.AddTransaction(genesisTx)
}
