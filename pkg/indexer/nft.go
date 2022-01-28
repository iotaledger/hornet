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
	dustReturnAddress       *iotago.Address
	expirationReturnAddress *iotago.Address
	issuer                  *iotago.Address
	sender                  *iotago.Address
	tag                     []byte
	pageSize                int
	offset                  []byte
	createdAfter            *time.Time
	unlockableByAddress       *iotago.Address
	hasDustReturnCondition    *bool
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

func NFTOffset(offset []byte) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.offset = offset
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

	if opts.expirationReturnAddress != nil {
		addr, err := addressBytesForAddress(*opts.expirationReturnAddress)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("expiration_return_address = ?", addr[:])
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

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.offset)
}
