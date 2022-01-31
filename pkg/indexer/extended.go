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
	offset                    []byte
	createdBefore             *time.Time
	createdAfter              *time.Time
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

func ExtendedOutputHasExpirationCondition(value bool) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.hasExpirationCondition = &value
	}
}

func ExtendedOutputExpiresBefore(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.expiresBefore = &time
	}
}

func ExtendedOutputExpiresAfter(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.expiresAfter = &time
	}
}

func ExtendedOutputExpiresBeforeMilestone(index milestone.Index) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.expiresBeforeMilestone = &index
	}
}

func ExtendedOutputExpiresAfterMilestone(index milestone.Index) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.expiresAfterMilestone = &index
	}
}

func ExtendedOutputHasTimelockCondition(value bool) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.hasTimelockCondition = &value
	}
}

func ExtendedOutputTimelockedBefore(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.timelockedBefore = &time
	}
}

func ExtendedOutputTimelockedAfter(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.timelockedAfter = &time
	}
}

func ExtendedOutputTimelockedBeforeMilestone(index milestone.Index) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.timelockedBeforeMilestone = &index
	}
}

func ExtendedOutputTimelockedAfterMilestone(index milestone.Index) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.timelockedAfterMilestone = &index
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

func ExtendedOutputCreatedBefore(time time.Time) ExtendedOutputFilterOption {
	return func(args *ExtendedOutputFilterOptions) {
		args.createdBefore = &time
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

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.offset)
}
