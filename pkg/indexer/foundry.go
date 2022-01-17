package indexer

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"

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

func (i *Indexer) FoundryOutput(foundryID *iotago.FoundryID) (iotago.OutputID, error) {
	result := &queryResult{}
	if err := i.db.Take(&foundry{}, foundryID[:]).
		Limit(1).
		Find(&result).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NullOutputID, ErrNotFound
		}
		return NullOutputID, err
	}
	return result.OutputID.ID(), nil
}

func (i *Indexer) FoundryOutputsWithFilters(filters ...FoundryFilterOption) (iotago.OutputIDs, error) {
	var results queryResults
	opts := foundryFilterOptions(filters)
	query := i.db.Model(&foundry{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return nil, err
		}
		query = query.Where("address = ?", addr[:])
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results.IDs(), nil
}
