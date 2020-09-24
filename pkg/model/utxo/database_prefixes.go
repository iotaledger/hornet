package utxo

const (
	StorePrefixUTXO byte = 8
)

const (
	UTXOStoreKeyPrefixOutput         byte = 0
	UTXOStoreKeyPrefixUnspent        byte = 1
	UTXOStoreKeyPrefixSpent          byte = 2
	UTXOStoreKeyPrefixMilestoneDiffs byte = 3
)
