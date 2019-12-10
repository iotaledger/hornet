package tangle

import (
	"github.com/iotaledger/iota.go/trinary"

	"github.com/gohornet/hornet/packages/syncutils"
	"github.com/gohornet/hornet/packages/typeutils"
)

type Approvers struct {
	hash        trinary.Trytes
	hashes      map[trinary.Trytes]bool
	hashesMutex syncutils.RWMutex
}

func NewApprovers(hash trinary.Hash) *Approvers {
	return &Approvers{
		hash:   hash,
		hashes: make(map[trinary.Trytes]bool),
	}
}

func GetApprovers(hash trinary.Hash) (result *Approvers, err error) {
	if cacheResult := approversCache.ComputeIfAbsent(hash, func() interface{} {
		approvers, dbErr := readApproversForTransactionFromDatabase(hash)
		if dbErr == nil {
			return approvers
		}
		err = dbErr
		return nil
	}); !typeutils.IsInterfaceNil(cacheResult) {
		result = cacheResult.(*Approvers)
	}
	return
}

func DiscardApproversFromCache(hash trinary.Hash) {
	approversCache.DeleteWithoutEviction(hash)
}

func (approvers *Approvers) Add(transactionHash trinary.Hash) {
	approvers.hashesMutex.Lock()
	if _, exists := approvers.hashes[transactionHash]; !exists {
		approvers.hashes[transactionHash] = true
	}
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) Remove(approverHash trinary.Hash) {
	approvers.hashesMutex.Lock()
	delete(approvers.hashes, approverHash)
	approvers.hashesMutex.Unlock()
}

func (approvers *Approvers) GetHashes() (result []trinary.Hash) {
	approvers.hashesMutex.RLock()

	result = make([]trinary.Hash, len(approvers.hashes))

	counter := 0
	for hash := range approvers.hashes {
		result[counter] = hash
		counter++
	}

	approvers.hashesMutex.RUnlock()

	return
}

func (approvers *Approvers) GetHash() (result trinary.Hash) {
	approvers.hashesMutex.RLock()
	result = approvers.hash
	approvers.hashesMutex.RUnlock()

	return
}
