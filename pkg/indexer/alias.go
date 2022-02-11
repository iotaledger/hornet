package indexer

import (
	"time"

	iotago "github.com/iotaledger/iota.go/v3"
)

type alias struct {
	AliasID          aliasIDBytes  `gorm:"primaryKey;notnull"`
	OutputID         outputIDBytes `gorm:"unique;notnull"`
	NativeTokenCount int           `gorm:"notnull"`
	StateController  addressBytes  `gorm:"notnull;index:alias_state_controller"`
	Governor         addressBytes  `gorm:"notnull;index:alias_governor"`
	Issuer           addressBytes  `gorm:"index:alias_issuer"`
	Sender           addressBytes  `gorm:"index:alias_sender"`
	CreatedAt        time.Time     `gorm:"notnull"`
}

type AliasFilterOptions struct {
	hasNativeTokens     *bool
	minNativeTokenCount *uint32
	maxNativeTokenCount *uint32
	stateController     *iotago.Address
	governor            *iotago.Address
	issuer              *iotago.Address
	sender              *iotago.Address
	pageSize            uint32
	cursor              *string
	createdBefore       *time.Time
	createdAfter        *time.Time
}

type AliasFilterOption func(*AliasFilterOptions)

func AliasHasNativeTokens(value bool) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.hasNativeTokens = &value
	}
}

func AliasMinNativeTokenCount(value uint32) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.minNativeTokenCount = &value
	}
}

func AliasMaxNativeTokenCount(value uint32) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.maxNativeTokenCount = &value
	}
}

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

func AliasPageSize(pageSize uint32) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.pageSize = pageSize
	}
}

func AliasCursor(cursor string) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.cursor = &cursor
	}
}

func AliasCreatedBefore(time time.Time) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.createdBefore = &time
	}
}

func AliasCreatedAfter(time time.Time) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.createdAfter = &time
	}
}

func aliasFilterOptions(optionalOptions []AliasFilterOption) *AliasFilterOptions {
	result := &AliasFilterOptions{}

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

	if opts.hasNativeTokens != nil {
		if *opts.hasNativeTokens {
			query = query.Where("native_token_count > 0")
		} else {
			query = query.Where("native_token_count == 0")
		}
	}

	if opts.minNativeTokenCount != nil {
		query = query.Where("native_token_count >= ?", *opts.minNativeTokenCount)
	}

	if opts.maxNativeTokenCount != nil {
		query = query.Where("native_token_count <= ?", *opts.maxNativeTokenCount)
	}

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

	if opts.createdBefore != nil {
		query = query.Where("created_at < ?", *opts.createdBefore)
	}

	if opts.createdAfter != nil {
		query = query.Where("created_at > ?", *opts.createdAfter)
	}

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.cursor)
}
