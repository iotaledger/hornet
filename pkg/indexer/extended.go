package indexer

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type basicOutput struct {
	OutputID                outputIDBytes `gorm:"primaryKey;notnull"`
	Amount                  uint64        `gorm:"notnull"`
	Sender                  addressBytes  `gorm:"index:basic_outputs_sender_tag"`
	Tag                     []byte        `gorm:"index:basic_outputs_sender_tag"`
	Address                 addressBytes  `gorm:"notnull;index:basic_outputs_address"`
	DustReturn              *uint64
	DustReturnAddress       addressBytes
	TimelockMilestone       *milestone.Index
	TimelockTime            *time.Time
	ExpirationMilestone     *milestone.Index
	ExpirationTime          *time.Time
	ExpirationReturnAddress addressBytes
	CreatedAt               time.Time `gorm:"notnull"`
}

type BasicOutputFilterOptions struct {
	unlockableByAddress       *iotago.Address
	hasDustReturnCondition    *bool
	dustReturnAddress         *iotago.Address
	hasExpirationCondition    *bool
	expirationReturnAddress   *iotago.Address
	expiresBefore             *time.Time
	expiresAfter              *time.Time
	expiresBeforeMilestone    *milestone.Index
	expiresAfterMilestone     *milestone.Index
	hasTimelockCondition      *bool
	timelockedBefore          *time.Time
	timelockedAfter           *time.Time
	timelockedBeforeMilestone *milestone.Index
	timelockedAfterMilestone  *milestone.Index
	sender                    *iotago.Address
	tag                       []byte
	pageSize                  int
	cursor                    *string
	createdBefore             *time.Time
	createdAfter              *time.Time
}

type BasicOutputFilterOption func(*BasicOutputFilterOptions)

func BasicOutputUnlockableByAddress(address iotago.Address) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func BasicOutputHasDustReturnCondition(value bool) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.hasDustReturnCondition = &value
	}
}

func BasicOutputDustReturnAddress(address iotago.Address) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.dustReturnAddress = &address
	}
}

func BasicOutputHasExpirationCondition(value bool) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.hasExpirationCondition = &value
	}
}

func BasicOutputExpiresBefore(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.expiresBefore = &time
	}
}

func BasicOutputExpiresAfter(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.expiresAfter = &time
	}
}

func BasicOutputExpiresBeforeMilestone(index milestone.Index) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.expiresBeforeMilestone = &index
	}
}

func BasicOutputExpiresAfterMilestone(index milestone.Index) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.expiresAfterMilestone = &index
	}
}

func BasicOutputHasTimelockCondition(value bool) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.hasTimelockCondition = &value
	}
}

func BasicOutputTimelockedBefore(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.timelockedBefore = &time
	}
}

func BasicOutputTimelockedAfter(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.timelockedAfter = &time
	}
}

func BasicOutputTimelockedBeforeMilestone(index milestone.Index) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.timelockedBeforeMilestone = &index
	}
}

func BasicOutputTimelockedAfterMilestone(index milestone.Index) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.timelockedAfterMilestone = &index
	}
}

func BasicOutputExpirationReturnAddress(address iotago.Address) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.expirationReturnAddress = &address
	}
}

func BasicOutputSender(address iotago.Address) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.sender = &address
	}
}

func BasicOutputTag(tag []byte) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.tag = tag
	}
}

func BasicOutputPageSize(pageSize int) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.pageSize = pageSize
	}
}

func BasicOutputCursor(cursor string) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.cursor = &cursor
	}
}

func BasicOutputCreatedBefore(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.createdBefore = &time
	}
}

func BasicOutputCreatedAfter(time time.Time) BasicOutputFilterOption {
	return func(args *BasicOutputFilterOptions) {
		args.createdAfter = &time
	}
}

func basicOutputFilterOptions(optionalOptions []BasicOutputFilterOption) *BasicOutputFilterOptions {
	result := &BasicOutputFilterOptions{}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}
func (i *Indexer) BasicOutputsWithFilters(filters ...BasicOutputFilterOption) *IndexerResult {
	opts := basicOutputFilterOptions(filters)
	query := i.db.Model(&basicOutput{})

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

	if opts.hasExpirationCondition != nil {
		if *opts.hasExpirationCondition {
			query = query.Where("expiration_return_address IS NOT NULL")
		} else {
			query = query.Where("expiration_return_address IS NULL")
		}
	}

	if opts.expirationReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.expirationReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("expiration_return_address = ?", addr[:])
	}

	if opts.expiresBefore != nil {
		query = query.Where("expiration_time < ?", *opts.expiresBefore)
	}

	if opts.expiresAfter != nil {
		query = query.Where("expiration_time > ?", *opts.expiresAfter)
	}

	if opts.expiresBeforeMilestone != nil {
		query = query.Where("expiration_milestone < ?", *opts.expiresBeforeMilestone)
	}

	if opts.expiresAfterMilestone != nil {
		query = query.Where("expiration_milestone > ?", *opts.expiresAfterMilestone)
	}

	if opts.hasTimelockCondition != nil {
		if *opts.hasTimelockCondition {
			query = query.Where("(timelock_time IS NOT NULL OR timelock_milestone IS NOT NULL)")
		} else {
			query = query.Where("timelock_time IS NULL").Where("timelock_milestone IS NULL")
		}
	}

	if opts.timelockedBefore != nil {
		query = query.Where("timelock_time < ?", *opts.timelockedBefore)
	}

	if opts.timelockedAfter != nil {
		query = query.Where("timelock_time > ?", *opts.timelockedAfter)
	}

	if opts.timelockedBeforeMilestone != nil {
		query = query.Where("timelock_milestone < ?", *opts.timelockedBeforeMilestone)
	}

	if opts.timelockedAfterMilestone != nil {
		query = query.Where("timelock_milestone > ?", *opts.timelockedAfterMilestone)
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

	if opts.createdBefore != nil {
		query = query.Where("created_at < ?", *opts.createdBefore)
	}

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.cursor)
}
