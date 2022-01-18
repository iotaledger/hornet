package indexer

import (
	"time"

	"gorm.io/gorm"

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

func (i *Indexer) ExtendedOutputsWithFilters(filters ...ExtendedOutputFilterOption) (iotago.OutputIDs, milestone.Index, error) {

	opts := extendedOutputFilterOptions(filters)
	query := i.db.Model(&extendedOutput{})

	if opts.unlockableByAddress != nil {
		addr, err := addressBytesForAddress(*opts.unlockableByAddress)
		if err != nil {
			return nil, 0, err
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

func (i *Indexer) combineOutputIDFilteredQuery(query *gorm.DB) (iotago.OutputIDs, milestone.Index, error) {

	// This combines the query with a second query that checks for the current ledger_index.
	// This way we do not need to lock anything and we know the index matches the results.
	//TODO: measure performance for big datasets
	ledgerIndexQuery := i.db.Model(&status{}).Select("ledger_index")
	joinedQuery := i.db.Table("(?), (?)", query.Select("output_id"), ledgerIndexQuery)

	var results queryResults
	if err := joinedQuery.Find(&results).Error; err != nil {
		return nil, 0, err
	}
	ledgerIndex := milestone.Index(0)
	if len(results) > 0 {
		ledgerIndex = results[0].LedgerIndex
	}
	return results.IDs(), ledgerIndex, nil
}
