package tangle

import "github.com/iotaledger/iota.go/trinary"

func databaseKeyForHashPrefix(hash trinary.Hash) []byte {
	return trinary.MustTrytesToBytes(hash)[:49]
}
