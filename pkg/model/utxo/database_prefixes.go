package utxo

const (
	StorePrefixUTXO byte = 8
)

const (
	UTXOStoreKeyPrefixLedgerMilestoneIndex byte = 0
	UTXOStoreKeyPrefixOutput               byte = 1
	UTXOStoreKeyPrefixUnspent              byte = 2
	UTXOStoreKeyPrefixSpent                byte = 3
	UTXOStoreKeyPrefixMilestoneDiffs       byte = 4
)
