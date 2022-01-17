package indexer

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"

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

func (i *Indexer) AliasOutput(aliasID *iotago.AliasID) (iotago.OutputID, error) {
	result := &queryResult{}
	if err := i.db.Take(&alias{}, aliasID[:]).
		Limit(1).
		Find(&result).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NullOutputID, ErrNotFound
		}
		return NullOutputID, err
	}
	return result.OutputID.ID(), nil
}

func (i *Indexer) AliasOutputsWithFilters(filter ...AliasFilterOption) (iotago.OutputIDs, error) {
	var results queryResults
	opts := aliasFilterOptions(filter)
	query := i.db.Model(&alias{})

	if opts.stateController != nil {
		addr, err := addressBytesForAddress(*opts.stateController)
		if err != nil {
			return nil, err
		}
		query = query.Where("state_controller = ?", addr[:])
	}

	if opts.governanceController != nil {
		addr, err := addressBytesForAddress(*opts.governanceController)
		if err != nil {
			return nil, err
		}
		query = query.Where("governance_controller = ?", addr[:])
	}

	if opts.sender != nil {
		addr, err := addressBytesForAddress(*opts.sender)
		if err != nil {
			return nil, err
		}
		query = query.Where("sender = ?", addr[:])
	}

	if opts.issuer != nil {
		addr, err := addressBytesForAddress(*opts.issuer)
		if err != nil {
			return nil, err
		}
		query = query.Where("issuer = ?", addr[:])
	}

	if opts.maxResults > 0 {
		query = query.Limit(opts.maxResults)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, err
	}
	return results.IDs(), nil
}
