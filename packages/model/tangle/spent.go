package tangle

import (
	"github.com/iotaledger/iota.go/trinary"
	cuckoo "github.com/seiflotfy/cuckoofilter"
)

const (
	CuckooFilterSize = 50000000
)

var (
	SpentAddressesCuckooFilter *cuckoo.Filter
)

// Checks whether an address was persisted and might return a false-positive.
func WasAddressSpentFrom(address trinary.Hash) bool {
	spentAddressesLock.RLock()
	defer spentAddressesLock.RUnlock()
	return SpentAddressesCuckooFilter.Lookup(trinary.MustTrytesToBytes(address))
}

// Marks an address in the cuckoo filter as spent.
func MarkAddressAsSpent(address trinary.Hash) bool {
	spentAddressesLock.Lock()
	defer spentAddressesLock.Unlock()
	return SpentAddressesCuckooFilter.Insert(trinary.MustTrytesToBytes(address))
}

// Initializes the cuckoo filter by loading it from the database (if available) or initializing
// a new one with the default size.
func InitSpentAddressesCuckooFilter() {
	SpentAddressesCuckooFilter = loadSpentAddressesCuckooFilter()
}