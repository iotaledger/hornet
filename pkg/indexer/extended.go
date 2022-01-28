package indexer

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type extendedOutput struct {
	OutputID                outputIDBytes `gorm:"primaryKey;notnull"`
	Amount                  uint64        `gorm:"notnull"`
	Sender                  addressBytes  `gorm:"index:extended_sender_tag"`
	Tag                     []byte        `gorm:"index:extended_sender_tag"`
	Address                 addressBytes  `gorm:"notnull;index:extended_address"`
	DustReturn              *uint64
	DustReturnAddress       addressBytes
	TimelockMilestone       *milestone.Index
	TimelockTime            *time.Time
	ExpirationMilestone     *milestone.Index
	ExpirationTime          *time.Time
	ExpirationReturnAddress addressBytes
	CreatedAt               time.Time `gorm:"notnull"`
}

type ExtendedOutputFilterOptions struct {
	dustReturnAddress       *iotago.Address
	expirationReturnAddress *iotago.Address
	sender                  *iotago.Address
	tag                     []byte
	pageSize                int
	offset                  []byte
	createdAfter            *time.Time
	unlockableByAddress       *iotago.Address
	hasDustReturnCondition    *bool
}

type ExtendedOutputFilterOption func(*ExtendedOutputFilterOptions)

func ExtendedOutputUnlockableByAddress(address iotago.Address) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func ExtendedOutputHasDustReturnCondition(value bool) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.hasDustReturnCondition = &value
	}
}

func ExtendedOutputDustReturnAddress(address iotago.Address) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.dustReturnAddress = &address
	}
}

func ExtendedOutputExpirationReturnAddress(address iotago.Address) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.expirationReturnAddress = &address
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

func ExtendedOutputPageSize(pageSize int) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.pageSize = pageSize
	}
}

func ExtendedOutputOffset(offset []byte) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.offset = offset
	}
}

func ExtendedOutputCreatedAfter(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.createdAfter = &time
	}
}

func extendedOutputFilterOptions(optionalOptions []ExtendedOutputFilterOption) *ExtendedOutputFilterOptions {
	result := &ExtendedOutputFilterOptions{}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}
func (i *Indexer) ExtendedOutputsWithFilters(filters ...ExtendedOutputFilterOption) *IndexerResult {
	opts := extendedOutputFilterOptions(filters)
	query := i.db.Model(&extendedOutput{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("address = ?", addr[:])
	}

	if opts.hasDustReturnCondition != nil {
		if *opts.hasDustReturnCondition {
			query = query.Where("dust_return IS NOT NULL")
		} else {
			query = query.Where("dust_return IS NULL")
		}
	}

	if opts.dustReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.dustReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("dust_return_address = ?", addr[:])
	}

	if opts.expirationReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.expirationReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("expiration_return_address = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.tag != nil && len(opts.tag) > 0 {
		query = query.Where("tag = ?", opts.tag)
	}

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.offset)
}
