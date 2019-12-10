package hornet

import "github.com/iotaledger/iota.go/trinary"

// Generated using: https://github.com/drgrib/maps
// `mapper -types "trinary.Hash:*Transaction"`

func ContainsKeyTrinaryHashTransaction(m map[trinary.Hash]*Transaction, k trinary.Hash) bool {
	_, ok := m[k]
	return ok
}

func ContainsValueTrinaryHashTransaction(m map[trinary.Hash]*Transaction, v *Transaction) bool {
	for _, mValue := range m {
		if mValue == v {
			return true
		}
	}

	return false
}

func GetKeysTrinaryHashTransaction(m map[trinary.Hash]*Transaction) []trinary.Hash {
	var keys []trinary.Hash

	for k, _ := range m {
		keys = append(keys, k)
	}

	return keys
}

func GetValuesTrinaryHashTransaction(m map[trinary.Hash]*Transaction) []*Transaction {
	var values []*Transaction

	for _, v := range m {
		values = append(values, v)
	}

	return values
}

func CopyTrinaryHashTransaction(m map[trinary.Hash]*Transaction) map[trinary.Hash]*Transaction {
	copyMap := map[trinary.Hash]*Transaction{}

	for k, v := range m {
		copyMap[k] = v
	}

	return copyMap
}
