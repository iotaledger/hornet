package indexer

import (
	"time"

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

func (i *Indexer) NFTOutput(nftID *iotago.NFTID) (iotago.OutputID, milestone.Index, error) {
	query := i.db.Model(&nft{}).
		Where("nft_id = ?", nftID[:]).
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

func (i *Indexer) NFTOutputsWithFilters(filters ...NFTFilterOption) (iotago.OutputIDs, milestone.Index, error) {
	opts := nftFilterOptions(filters)
	query := i.db.Model(&nft{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return nil, 0, err
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
			return nil, 0, err
		}
		query = query.Where("issuer = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.tag != nil && len(opts.tag) > 0 {
		query = query.Where("tag = ?", opts.tag)
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	return i.combineOutputIDFilteredQuery(query)
}
