package indexer

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
)

type alias struct {
	AliasID         aliasIDBytes  `gorm:"primaryKey;notnull"`
	OutputID        outputIDBytes `gorm:"unique;notnull"`
	Amount          uint64        `gorm:"notnull"`
	StateController addressBytes  `gorm:"notnull;index:alias_state_controller"`
	Governor        addressBytes  `gorm:"notnull;index:alias_governor"`
	Issuer          addressBytes  `gorm:"index:alias_issuer"`
	Sender          addressBytes  `gorm:"index:alias_sender"`
	CreatedAt       time.Time     `gorm:"notnull"`
}

type AliasFilterOptions struct {
	stateController *iotago.Address
	governor        *iotago.Address
	issuer          *iotago.Address
	sender          *iotago.Address
	pageSize        int
	offset          []byte
	createdAfter    *time.Time
}

type AliasFilterOption func(*AliasFilterOptions)

func AliasStateController(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.stateController = &address
	}
}

func AliasGovernor(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.governor = &address
	}
}

func AliasSender(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.sender = &address
	}
}

func AliasIssuer(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.issuer = &address
	}
}

func AliasPageSize(pageSize int) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.pageSize = pageSize
	}
}

func AliasOffset(offset []byte) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.offset = offset
	}
}

func AliasCreatedAfter(time time.Time) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.createdAfter = &time
	}
}

func aliasFilterOptions(optionalOptions []AliasFilterOption) *AliasFilterOptions {
	result := &AliasFilterOptions{
		stateController: nil,
		governor:        nil,
		issuer:          nil,
		sender:          nil,
		pageSize:        0,
		offset:          nil,
		createdAfter:    nil,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

func (i *Indexer) AliasOutput(aliasID *iotago.AliasID) *IndexerResult {
	query := i.db.Model(&alias{}).
		Where("alias_id = ?", aliasID[:]).
		Limit(1)

	return i.combineOutputIDFilteredQuery(query, 0, nil)
}

func (i *Indexer) AliasOutputsWithFilters(filter ...AliasFilterOption) *IndexerResult {
	opts := aliasFilterOptions(filter)
	query := i.db.Model(&alias{})

	if opts.stateController != nil {
		addr, err := addressBytesForAddress(*opts.stateController)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("state_controller = ?", addr[:])
	}

	if opts.governor != nil {
		addr, err := addressBytesForAddress(*opts.governor)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("governor = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("issuer = ?", addr[:])
	}

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.offset)
}
