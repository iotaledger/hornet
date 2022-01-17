package indexer

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type extendedOutput struct {
	OutputID            outputIDBytes `gorm:"primaryKey;not null"`
	Address             addressBytes  `gorm:"not null;index:extended_address"`
	Amount              uint64        `gorm:"not null"`
	Sender              addressBytes  `gorm:"index:extended_sender_tag"`
	Tag                 []byte        `gorm:"index:extended_sender_tag"`
	DustReturn          *uint64
	TimelockMilestone   *milestone.Index
	TimelockTime        *time.Time
	ExpirationMilestone *milestone.Index
	ExpirationTime      *time.Time
}

type ExtendedOutputFilterOptions struct {
	unlockableByAddress *iotago.Address
	requiresDustReturn  *bool
	sender              *iotago.Address
	tag                 []byte
	maxResults          int
}

type ExtendedOutputFilterOption func(*ExtendedOutputFilterOptions)

func ExtendedOutputUnlockableByAddress(address iotago.Address) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func ExtendedOutputRequiresDustReturn(requiresDustReturn bool) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.requiresDustReturn = &requiresDustReturn
	}
}

func ExtendedOutputSender(address iotago.Address) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.sender = &address
	}
}

func ExtendedOutputTag(tag []byte) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.tag = tag
	}
}

func ExtendedOutputMaxResults(maxResults int) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.maxResults = maxResults
	}
}

func extendedOutputFilterOptions(optionalOptions []ExtendedOutputFilterOption) *ExtendedOutputFilterOptions {
	result := &ExtendedOutputFilterOptions{
		unlockableByAddress: nil,
		requiresDustReturn:  nil,
		sender:              nil,
		tag:                 nil,
		maxResults:          0,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}

func (i *Indexer) ExtendedOutputsWithFilters(filters ...ExtendedOutputFilterOption) (iotago.OutputIDs, error) {
	var results queryResults
	opts := extendedOutputFilterOptions(filters)
	query := i.db.Model(&extendedOutput{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return nil, err
		}
		query = query.Where("address = ?", addr[:])
	}

	if opts.requiresDustReturn != nil {
		if *opts.requiresDustReturn {
			query = query.Where("dust_return IS NOT NULL")
		} else {
			query = query.Where("dust_return IS NULL")
		}
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return nil, err
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.tag != nil && len(opts.tag) > 0 {
		query = query.Where("tag = ?", opts.tag)
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results.IDs(), nil
}
