package indexer

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type foundry struct {
	FoundryID foundryIDBytes `gorm:"primaryKey;not null"`
	OutputID  outputIDBytes  `gorm:"unique;not null"`
	Amount    uint64         `gorm:"not null"`
	Address   addressBytes   `gorm:"not null;index:foundries_address"`
}

type FoundryFilterOptions struct {
	unlockableByAddress *iotago.Address
	maxResults          int
}

type FoundryFilterOption func(*FoundryFilterOptions)

func FoundryUnlockableByAddress(address iotago.Address) FoundryFilterOption {
	return func(args *FoundryFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func FoundryMaxResults(maxResults int) FoundryFilterOption {
	return func(args *FoundryFilterOptions) {
		args.maxResults = maxResults
	}
}

func foundryFilterOptions(optionalOptions []FoundryFilterOption) *FoundryFilterOptions {
	result := &FoundryFilterOptions{
		unlockableByAddress: nil,
		maxResults:          0,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}

func (i *Indexer) FoundryOutput(foundryID *iotago.FoundryID) (iotago.OutputID, milestone.Index, error) {
	query := i.db.Model(&foundry{}).
		Where("foundry_id = ?", foundryID[:]).
		Limit(1)

	outputIDs, ledgerIndex, err := i.combineOutputIDFilteredQuery(query)
	if err != nil {
		return NullOutputID, 0, err
	}
	if len(outputIDs) == 0 {
		return NullOutputID, 0, ErrNotFound
	}
	return outputIDs[0], ledgerIndex, nil
}

func (i *Indexer) FoundryOutputsWithFilters(filters ...FoundryFilterOption) (iotago.OutputIDs, milestone.Index, error) {
	opts := foundryFilterOptions(filters)
	query := i.db.Model(&foundry{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("address = ?", addr[:])
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	return i.combineOutputIDFilteredQuery(query)
}
