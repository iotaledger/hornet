package indexer

import iotago "github.com/iotaledger/iota.go/v3"

func (i *Indexer) OutputsByAddress(address iotago.Address, filterType *iotago.OutputType) (iotago.OutputIDs, error) {
	return iotago.OutputIDs{}, nil
}

func (i *Indexer) AliasOutput(aliasID *iotago.AliasID) (*iotago.OutputID, error) {
	outputID := &outputID{}
	if err := i.db.Find(&alias{}, aliasID[:]).Limit(1).Find(&outputID).Error; err != nil {
		return nil, err
	}
	return outputID.ID(), nil
}

func (i *Indexer) NFTOutput(nftID *iotago.NFTID) (*iotago.OutputID, error) {
	outputID := &outputID{}
	if err := i.db.Find(&nft{}, nftID[:]).Limit(1).Find(&outputID).Error; err != nil {
		return nil, err
	}
	return outputID.ID(), nil
}

func (i *Indexer) FoundryOutput(foundryID *iotago.FoundryID) (*iotago.OutputID, error) {
	outputID := &outputID{}
	if err := i.db.Find(&foundry{}, foundryID[:]).Limit(1).Find(&outputID).Error; err != nil {
		return nil, err
	}
	return outputID.ID(), nil
}
