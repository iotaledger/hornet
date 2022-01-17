package indexer

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

type alias struct {
	AliasID              aliasIDBytes  `gorm:"primaryKey;not null"`
	OutputID             outputIDBytes `gorm:"unique;not null"`
	Amount               uint64        `gorm:"not null"`
	StateController      addressBytes  `gorm:"not null;index:alias_state_controller""`
	GovernanceController addressBytes  `gorm:"not null;index:alias_governance_controller"`
	Issuer               addressBytes  `gorm:"index:alias_issuer"`
	Sender               addressBytes  `gorm:"index:alias_sender"`
}

type AliasFilterOptions struct {
	stateController      *iotago.Address
	governanceController *iotago.Address
	issuer               *iotago.Address
	sender               *iotago.Address
	maxResults           int
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

func AliasMaxResults(maxResults int) AliasFilterOption {
	return func(args *AliasFilterOptions) {
		args.maxResults = maxResults
	}
}

func aliasFilterOptions(optionalOptions []AliasFilterOption) *AliasFilterOptions {
	result := &AliasFilterOptions{
		stateController:      nil,
		governanceController: nil,
		issuer:               nil,
		sender:               nil,
		maxResults:           0,
	}

	for _, optionalOption := range optionalOptions {
		optionalOption(result)
	}

	return result
}

func (i *Indexer) AliasOutput(aliasID *iotago.AliasID) (iotago.OutputID, milestone.Index, error) {
	query := i.db.Model(&alias{}).
		Where("alias_id = ?", aliasID[:]).
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

func (i *Indexer) AliasOutputsWithFilters(filter ...AliasFilterOption) (iotago.OutputIDs, milestone.Index, error) {
	opts := aliasFilterOptions(filter)
	query := i.db.Model(&alias{})

	if opts.stateController != nil {
		addr, err := addressBytesForAddress(*opts.stateController)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("state_controller = ?", addr[:])
	}

	if opts.governanceController != nil {
		addr, err := addressBytesForAddress(*opts.governanceController)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("governance_controller = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return nil, 0, err
		}
		query = query.Where("issuer = ?", addr[:])
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	return i.combineOutputIDFilteredQuery(query)
}
