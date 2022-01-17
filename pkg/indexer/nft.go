package indexer

import (
	"time"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type nft struct {
	NFTID               nftIDBytes    `gorm:"primaryKey;not null"`
	OutputID            outputIDBytes `gorm:"unique;not null"`
	Address             addressBytes  `gorm:"not null;index:nft_address"`
	Amount              uint64        `gorm:"not null"`
	Issuer              addressBytes  `gorm:"index:nft_issuer"`
	Sender              addressBytes  `gorm:"index:nft_sender_tag"`
	Tag                 []byte        `gorm:"index:nft_sender_tag"`
	DustReturn          *uint64
	TimelockMilestone   *milestone.Index
	TimelockTime        *time.Time
	ExpirationMilestone *milestone.Index
	ExpirationTime      *time.Time
}

type NFTFilterOptions struct {
	unlockableByAddress *iotago.Address
	requiresDustReturn  *bool
	issuer              *iotago.Address
	sender              *iotago.Address
	tag                 []byte
	maxResults          int
}

type NFTFilterOption func(*NFTFilterOptions)

func NFTUnlockableByAddress(address iotago.Address) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.unlockableByAddress = &address
	}
}

func NFTRequiresDustReturn(requiresDustReturn bool) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.requiresDustReturn = &requiresDustReturn
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

func NFTMaxResults(maxResults int) NFTFilterOption {
	return func(args *NFTFilterOptions) {
		args.maxResults = maxResults
	}
}

func nftFilterOptions(optionalOptions []NFTFilterOption) *NFTFilterOptions {
	result := &NFTFilterOptions{
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

func (i *Indexer) NFTOutput(nftID *iotago.NFTID) (iotago.OutputID, error) {
	result := &queryResult{}
	if err := i.db.Take(&nft{}, nftID[:]).
		Find(&result).
		Limit(1).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NullOutputID, ErrNotFound
		}
		return NullOutputID, err
	}
	return result.OutputID.ID(), nil
}

func (i *Indexer) NFTOutputsWithFilters(filters ...NFTFilterOption) (iotago.OutputIDs, error) {
	var results queryResults
	opts := nftFilterOptions(filters)
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
			query = query.Where("dust_return > 0")
		} else {
			query = query.Where("dust_return = 0")
		}
	}

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return nil, err
		}
		query = query.Where("issuer = ?", addr[:])
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
