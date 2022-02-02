package indexer

import (
	"time"

	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type nft struct {
	NFTID                   nftIDBytes    `gorm:"primaryKey;notnull"`
	OutputID                outputIDBytes `gorm:"unique;notnull"`
	Amount                  uint64        `gorm:"notnull"`
	Issuer                  addressBytes  `gorm:"index:nft_issuer"`
	Sender                  addressBytes  `gorm:"index:nft_sender_tag"`
	Tag                     []byte        `gorm:"index:nft_sender_tag"`
	Address                 addressBytes  `gorm:"notnull;index:nft_address"`
	DustReturn              *uint64
	DustReturnAddress       addressBytes
	TimelockMilestone       *milestone.Index
	TimelockTime            *time.Time
	ExpirationMilestone     *milestone.Index
	ExpirationTime          *time.Time
	ExpirationReturnAddress addressBytes
	CreatedAt               time.Time `gorm:"notnull"`
}

type NFTFilterOptions struct {
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
	issuer                    *iotago.Address
	sender                    *iotago.Address
	tag                       []byte
	pageSize                  int
	cursor                    []byte
	createdBefore             *time.Time
	createdAfter              *time.Time
}

type NFTFilterOption func(*NFTFilterOptions)

func NFTUnlockableByAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func NFTHasDustReturnCondition(requiresDustReturn bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasDustReturnCondition = &requiresDustReturn
	}
}

func NFTDustReturnAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.dustReturnAddress = &address
	}
}

func NFTExpirationReturnAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expirationReturnAddress = &address
	}
}

func NFTHasExpirationCondition(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasExpirationCondition = &value
	}
}

func NFTExpiresBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresBefore = &time
	}
}

func NFTExpiresAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresAfter = &time
	}
}

func NFTExpiresBeforeMilestone(index milestone.Index) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresBeforeMilestone = &index
	}
}

func NFTExpiresAfterMilestone(index milestone.Index) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.expiresAfterMilestone = &index
	}
}

func NFTHasTimelockCondition(value bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.hasTimelockCondition = &value
	}
}

func NFTTimelockedBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedBefore = &time
	}
}

func NFTTimelockedAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedAfter = &time
	}
}

func NFTTimelockedBeforeMilestone(index milestone.Index) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedBeforeMilestone = &index
	}
}

func NFTTimelockedAfterMilestone(index milestone.Index) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.timelockedAfterMilestone = &index
	}
}

func NFTIssuer(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.issuer = &address
	}
}

func NFTSender(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.sender = &address
	}
}

func NFTTag(tag []byte) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.tag = tag
	}
}

func NFTPageSize(pageSize int) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.pageSize = pageSize
	}
}

func NFTCursor(cursor []byte) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.cursor = cursor
	}
}

func NFTCreatedBefore(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.createdBefore = &time
	}
}

func NFTCreatedAfter(time time.Time) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.createdAfter = &time
	}
}

func nftFilterOptions(optionalOptions []NFTFilterOption) *NFTFilterOptions {
	result := &NFTFilterOptions{}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}
	return result
}

func (i *Indexer) NFTOutput(nftID *iotago.NFTID) *IndexerResult {
	query := i.db.Model(&nft{}).
		Where("nft_id = ?", nftID[:]).
		Limit(1)

	return i.combineOutputIDFilteredQuery(query, 0, nil)
}

func (i *Indexer) NFTOutputsWithFilters(filters ...NFTFilterOption) *IndexerResult {
	opts := nftFilterOptions(filters)
	query := i.db.Model(&nft{})

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

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("issuer = ?", addr[:])
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
