package indexer

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type alias struct {
	AliasID              aliasIDBytes  `gorm:"primaryKey;notnull"`
	OutputID             outputIDBytes `gorm:"unique;notnull"`
	Amount               uint64        `gorm:"notnull"`
	StateController      addressBytes  `gorm:"notnull;index:alias_state_controller""`
	GovernanceController addressBytes  `gorm:"notnull;index:alias_governance_controller"`
	Issuer               addressBytes  `gorm:"index:alias_issuer"`
	Sender               addressBytes  `gorm:"index:alias_sender"`
	MilestoneIndex       milestone.Index
}

type AliasFilterOptions struct {
	stateController      *iotago.Address
	governanceController *iotago.Address
	issuer               *iotago.Address
	sender               *iotago.Address
	pageSize             int
	offset               []byte
}

type AliasFilterOption func(*AliasFilterOptions)

func AliasStateController(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.stateController = &address
	}
}

func AliasGovernanceController(address iotago.Address) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.governanceController = &address
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

func aliasFilterOptions(optionalOptions []AliasFilterOption) *AliasFilterOptions {
	result := &AliasFilterOptions{
		stateController:      nil,
		governanceController: nil,
		issuer:               nil,
		sender:               nil,
		pageSize:             0,
		offset:               nil,
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

	if opts.governanceController != nil {
		addr, err := addressBytesForAddress(*opts.governanceController)
		if err != nil {
			return errorResult(err)
		}
		query = query.Where("governance_controller = ?", addr[:])
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

	return i.combineOutputIDFilteredQuery(query, opts.pageSize, opts.offset)
}
