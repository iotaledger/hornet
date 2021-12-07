package utxo

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func TestUTXOComputeBalance(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	initialOutput := RandOutputOnAddressWithAmount(iotago.OutputExtended, RandAddress(iotago.AddressEd25519), 2_134_656_365)
	require.NoError(t, utxo.AddUnspentOutput(initialOutput))
	require.NoError(t, utxo.AddUnspentOutput(RandOutputOnAddressWithAmount(iotago.OutputAlias, RandAddress(iotago.AddressAlias), 56_549_524)))
	require.NoError(t, utxo.AddUnspentOutput(RandOutputOnAddressWithAmount(iotago.OutputFoundry, RandAddress(iotago.AddressAlias), 25_548_858)))
	require.NoError(t, utxo.AddUnspentOutput(RandOutputOnAddressWithAmount(iotago.OutputNFT, RandAddress(iotago.AddressEd25519), 545_699_656)))
	require.NoError(t, utxo.AddUnspentOutput(RandOutputOnAddressWithAmount(iotago.OutputExtended, RandAddress(iotago.AddressAlias), 626_659_696)))

	msIndex := milestone.Index(756)

	outputs := Outputs{
		RandOutputOnAddressWithAmount(iotago.OutputExtended, RandAddress(iotago.AddressNFT), 2_134_656_365),
	}

	spents := Spents{
		RandSpent(initialOutput, msIndex),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	spent, err := utxo.SpentOutputs()
	require.NoError(t, err)
	require.Equal(t, 1, len(spent))

	unspentExtended, err := utxo.UnspentExtendedOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(unspentExtended))

	unspentNFT, err := utxo.UnspentNFTOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(unspentNFT))

	unspentAlias, err := utxo.UnspentAliasOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(unspentAlias))

	unspentFoundry, err := utxo.UnspentFoundryOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(unspentFoundry))

	balance, count, err := utxo.ComputeLedgerBalance()
	require.NoError(t, err)
	require.Equal(t, 5, count)
	require.Equal(t, uint64(2_134_656_365+56_549_524+25_548_858+545_699_656+626_659_696), balance)
}

func TestUTXOIterationWithoutFilters(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	extendedOutputs := Outputs{
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressEd25519)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressNFT)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressAlias)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressEd25519)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressNFT)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressAlias)),
		RandOutputOnAddress(iotago.OutputExtended, RandAddress(iotago.AddressEd25519)),
	}
	nftOutputs := Outputs{
		RandOutputOnAddress(iotago.OutputNFT, RandAddress(iotago.AddressEd25519)),
		RandOutputOnAddress(iotago.OutputNFT, RandAddress(iotago.AddressAlias)),
		RandOutputOnAddress(iotago.OutputNFT, RandAddress(iotago.AddressNFT)),
		RandOutputOnAddress(iotago.OutputNFT, RandAddress(iotago.AddressAlias)),
	}
	aliasOutputs := Outputs{
		RandOutputOnAddress(iotago.OutputAlias, RandAddress(iotago.AddressEd25519)),
	}
	foundryOutputs := Outputs{
		RandOutputOnAddress(iotago.OutputFoundry, RandAddress(iotago.AddressAlias)),
		RandOutputOnAddress(iotago.OutputFoundry, RandAddress(iotago.AddressAlias)),
		RandOutputOnAddress(iotago.OutputFoundry, RandAddress(iotago.AddressAlias)),
	}

	msIndex := milestone.Index(756)

	spents := Spents{
		RandSpent(extendedOutputs[3], msIndex),
		RandSpent(extendedOutputs[2], msIndex),
		RandSpent(nftOutputs[2], msIndex),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, append(append(append(extendedOutputs, nftOutputs...), aliasOutputs...), foundryOutputs...), spents, nil, nil))

	// Prepare values to check
	outputByID := make(map[string]struct{})
	unspentExtendedByID := make(map[string]struct{})
	unspentNFTByID := make(map[string]struct{})
	unspentAliasByID := make(map[string]struct{})
	unspentFoundryByID := make(map[string]struct{})
	spentByID := make(map[string]struct{})

	for _, output := range extendedOutputs {
		outputByID[output.mapKey()] = struct{}{}
		unspentExtendedByID[output.mapKey()] = struct{}{}
	}
	for _, output := range nftOutputs {
		outputByID[output.mapKey()] = struct{}{}
		unspentNFTByID[output.mapKey()] = struct{}{}
	}
	for _, output := range aliasOutputs {
		outputByID[output.mapKey()] = struct{}{}
		unspentAliasByID[output.mapKey()] = struct{}{}
	}
	for _, output := range foundryOutputs {
		outputByID[output.mapKey()] = struct{}{}
		unspentFoundryByID[output.mapKey()] = struct{}{}
	}
	for _, spent := range spents {
		spentByID[spent.mapKey()] = struct{}{}
		delete(unspentExtendedByID, spent.mapKey())
		delete(unspentNFTByID, spent.mapKey())
		delete(unspentAliasByID, spent.mapKey())
		delete(unspentFoundryByID, spent.mapKey())
	}

	// Test iteration without filters
	require.NoError(t, utxo.ForEachOutput(func(output *Output) bool {
		_, has := outputByID[output.mapKey()]
		require.True(t, has)
		delete(outputByID, output.mapKey())
		return true
	}))

	require.Empty(t, outputByID)

	require.NoError(t, utxo.ForEachUnspentExtendedOutput(nil, func(output *Output) bool {
		_, has := unspentExtendedByID[output.mapKey()]
		require.True(t, has)
		delete(unspentExtendedByID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentExtendedByID)

	require.NoError(t, utxo.ForEachUnspentNFTOutput(nil, func(output *Output) bool {
		_, has := unspentNFTByID[output.mapKey()]
		require.True(t, has)
		delete(unspentNFTByID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentNFTByID)

	require.NoError(t, utxo.ForEachUnspentAliasOutput(nil, func(output *Output) bool {
		_, has := unspentAliasByID[output.mapKey()]
		require.True(t, has)
		delete(unspentAliasByID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentAliasByID)

	require.NoError(t, utxo.ForEachUnspentFoundryOutput(nil, func(output *Output) bool {
		_, has := unspentFoundryByID[output.mapKey()]
		require.True(t, has)
		delete(unspentFoundryByID, output.mapKey())
		return true
	}))
	require.Empty(t, unspentFoundryByID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByID[spent.mapKey()]
		require.True(t, has)
		delete(spentByID, spent.mapKey())
		return true
	}))

	require.Empty(t, spentByID)
}

func TestUTXOIterationWithAddressFilterAndTypeFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := RandAddress(iotago.AddressEd25519)

	outputs := Outputs{
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 3242343),
		RandOutput(iotago.OutputExtended),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 5898566), // spent
		RandOutput(iotago.OutputExtended),                                      // spent
		RandOutputOnAddressWithAmount(iotago.OutputNFT, address, 23432423),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 78632467),
		RandOutput(iotago.OutputExtended),
		RandOutput(iotago.OutputAlias),
		RandOutput(iotago.OutputNFT),
		RandOutput(iotago.OutputNFT),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 98734278),
		RandOutputOnAddressWithAmount(iotago.OutputAlias, address, 98734278),
	}

	msIndex := milestone.Index(756)
	spents := Spents{
		RandSpent(outputs[3], msIndex),
		RandSpent(outputs[2], msIndex),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Prepare values to check
	anyUnspentByID := make(map[string]struct{})
	extendedUnspentByID := make(map[string]struct{})
	nftUnspentByID := make(map[string]struct{})
	aliasUnspentByID := make(map[string]struct{})
	spentByID := make(map[string]struct{})

	anyUnspentByID[outputs[0].mapKey()] = struct{}{}
	anyUnspentByID[outputs[4].mapKey()] = struct{}{}
	anyUnspentByID[outputs[5].mapKey()] = struct{}{}
	anyUnspentByID[outputs[10].mapKey()] = struct{}{}
	anyUnspentByID[outputs[11].mapKey()] = struct{}{}

	extendedUnspentByID[outputs[0].mapKey()] = struct{}{}
	extendedUnspentByID[outputs[5].mapKey()] = struct{}{}
	extendedUnspentByID[outputs[10].mapKey()] = struct{}{}

	nftUnspentByID[outputs[4].mapKey()] = struct{}{}

	aliasUnspentByID[outputs[11].mapKey()] = struct{}{}

	spentByID[outputs[2].mapKey()] = struct{}{}
	spentByID[outputs[3].mapKey()] = struct{}{}

	require.NoError(t, utxo.ForEachUnspentExtendedOutput(address, func(output *Output) bool {
		_, has := extendedUnspentByID[output.mapKey()]
		require.True(t, has)
		delete(extendedUnspentByID, output.mapKey())
		return true
	}))
	require.Empty(t, extendedUnspentByID)

	require.NoError(t, utxo.ForEachUnspentOutputOnAddress(address, nil, func(output *Output) bool {
		_, has := anyUnspentByID[output.mapKey()]
		require.True(t, has)
		delete(anyUnspentByID, output.mapKey())
		return true
	}))
	require.Empty(t, anyUnspentByID)

	require.NoError(t, utxo.ForEachUnspentOutputOnAddress(address, FilterOutputType(iotago.OutputNFT), func(output *Output) bool {
		_, has := nftUnspentByID[output.mapKey()]
		require.True(t, has)
		delete(nftUnspentByID, output.mapKey())
		return true
	}))
	require.Empty(t, nftUnspentByID)

	require.NoError(t, utxo.ForEachUnspentOutputOnAddress(address, FilterOutputType(iotago.OutputAlias), func(output *Output) bool {
		_, has := aliasUnspentByID[output.mapKey()]
		require.True(t, has)
		delete(aliasUnspentByID, output.mapKey())
		return true
	}))
	require.Empty(t, aliasUnspentByID)

	require.NoError(t, utxo.ForEachSpentOutput(func(spent *Spent) bool {
		_, has := spentByID[spent.mapKey()]
		require.True(t, has)
		delete(spentByID, spent.mapKey())
		return true
	}))

	require.Empty(t, spentByID)
}

func TestUTXOLoadMethodsAddressFilterAndTypeFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	address := RandAddress(iotago.AddressEd25519)

	nftID := RandNFTID()
	nftOutputID := RandOutputID()

	aliasID := RandAliasID()
	aliasOutputID := RandOutputID()

	foundryOutputID := RandOutputID()
	foundryAlias := RandAliasID().ToAddress()
	foundrySupply := new(big.Int).SetUint64(rand.Uint64())
	foundrySerialNumber := rand.Uint32()

	outputs := Outputs{
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 3242343),
		RandOutput(iotago.OutputExtended),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 5898566), // spent
		RandOutput(iotago.OutputExtended),                                      // spent
		CreateOutput(nftOutputID, RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           address,
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
		}),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 78632467),
		RandOutput(iotago.OutputExtended),
		RandOutput(iotago.OutputAlias),
		RandOutput(iotago.OutputNFT),
		RandOutput(iotago.OutputNFT),
		RandOutputOnAddressWithAmount(iotago.OutputExtended, address, 98734278),
		CreateOutput(aliasOutputID, RandMessageID(), RandMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              aliasID,
			StateController:      address,
			GovernanceController: address,
			StateMetadata:        []byte{},
		}),
		RandOutput(iotago.OutputFoundry),
		CreateOutput(foundryOutputID, RandMessageID(), RandMilestoneIndex(), &iotago.FoundryOutput{
			Address:           foundryAlias,
			Amount:            2156548,
			SerialNumber:      foundrySerialNumber,
			TokenTag:          RandTokenTag(),
			CirculatingSupply: foundrySupply,
			MaximumSupply:     foundrySupply,
			TokenScheme:       &iotago.SimpleTokenScheme{},
		}),
	}

	ms := marshalutil.New(iotago.FoundryIDLength)
	foundryAliasBytes, err := foundryAlias.Serialize(serializer.DeSeriModeNoValidation, nil)
	require.NoError(t, err)
	ms.WriteBytes(foundryAliasBytes)
	ms.WriteUint32(foundrySerialNumber)
	ms.WriteByte(byte(iotago.TokenSchemeSimple))
	foundryID := iotago.FoundryID{}
	copy(foundryID[:], ms.Bytes())

	msIndex := milestone.Index(756)
	spents := Spents{
		RandSpent(outputs[3], msIndex),
		RandSpent(outputs[2], msIndex),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(msIndex, outputs, spents, nil, nil))

	// Test no MaxResultCount
	loadedSpents, err := utxo.SpentOutputs()
	require.NoError(t, err)
	require.Equal(t, 2, len(loadedSpents))

	loadedSpents, err = utxo.SpentOutputs(MaxResultCount(1))
	require.NoError(t, err)
	require.Equal(t, 1, len(loadedSpents))

	loadExtendedUnspent, err := utxo.UnspentExtendedOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 5, len(loadExtendedUnspent))

	loadExtendedUnspent, err = utxo.UnspentExtendedOutputs(nil, MaxResultCount(2))
	require.NoError(t, err)
	require.Equal(t, 2, len(loadExtendedUnspent))

	loadExtendedUnspent, err = utxo.UnspentExtendedOutputs(address)
	require.NoError(t, err)
	require.Equal(t, 3, len(loadExtendedUnspent))

	loadExtendedUnspent, err = utxo.UnspentExtendedOutputs(address, MaxResultCount(1))
	require.NoError(t, err)
	require.Equal(t, 1, len(loadExtendedUnspent))

	loadUnspentNFTs, err := utxo.UnspentNFTOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 3, len(loadUnspentNFTs))

	loadUnspentNFTs, err = utxo.UnspentNFTOutputs(&nftID)
	require.NoError(t, err)
	require.Equal(t, 1, len(loadUnspentNFTs))
	require.Equal(t, nftOutputID[:], loadUnspentNFTs[0].OutputID()[:])

	loadUnspentAliases, err := utxo.UnspentAliasOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(loadUnspentAliases))

	loadUnspentAliases, err = utxo.UnspentAliasOutputs(&aliasID)
	require.NoError(t, err)
	require.Equal(t, 1, len(loadUnspentAliases))
	require.Equal(t, aliasOutputID[:], loadUnspentAliases[0].OutputID()[:])

	loadUnspentFoundries, err := utxo.UnspentFoundryOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 2, len(loadUnspentFoundries))

	loadUnspentFoundries, err = utxo.UnspentFoundryOutputs(&foundryID)
	require.NoError(t, err)
	require.Equal(t, 1, len(loadUnspentFoundries))
	require.Equal(t, foundryOutputID[:], loadUnspentFoundries[0].OutputID()[:])
}

func TestUTXOIssuerFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	issuerAddress := RandAddress(iotago.AddressEd25519)
	aliasIssuerAddress := RandAddress(iotago.AddressEd25519)

	nftID := RandNFTID()
	nftOutputIDs := []*iotago.OutputID{
		RandOutputID(),
		RandOutputID(),
	}

	aliasIssuerOutputIDs := []*iotago.OutputID{
		RandOutputID(),
		RandOutputID(),
	}
	aliasIssuerAliasOutputIDs := []*iotago.OutputID{
		aliasIssuerOutputIDs[0],
	}

	outputs := Outputs{
		CreateOutput(aliasIssuerAliasOutputIDs[0], RandMessageID(), RandMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              RandAliasID(),
			StateController:      RandAddress(iotago.AddressEd25519),
			GovernanceController: RandAddress(iotago.AddressEd25519),
			StateMetadata:        []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: aliasIssuerAddress,
				},
			},
		}),
		CreateOutput(RandOutputID(), RandMessageID(), RandMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              RandAliasID(),
			StateController:      RandAddress(iotago.AddressEd25519),
			GovernanceController: RandAddress(iotago.AddressEd25519),
			StateMetadata:        []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: RandAddress(iotago.AddressEd25519),
				},
			},
		}),
		CreateOutput(RandOutputID(), RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           RandAddress(iotago.AddressAlias),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: RandAddress(iotago.AddressAlias),
				},
			},
		}),
		CreateOutput(nftOutputIDs[0], RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           RandAddress(iotago.AddressNFT),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: issuerAddress,
				},
			},
		}),
		CreateOutput(aliasIssuerOutputIDs[1], RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           RandAddress(iotago.AddressEd25519),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: aliasIssuerAddress,
				},
			},
		}),
		CreateOutput(nftOutputIDs[1], RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           RandAddress(iotago.AddressAlias),
			Amount:            234342348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: issuerAddress,
				},
			},
		}),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(RandMilestoneIndex(), outputs, nil, nil, nil))

	loadUnspentNFTs, err := utxo.UnspentNFTOutputs(nil)
	require.NoError(t, err)
	require.Equal(t, 4, len(loadUnspentNFTs))

	var foundOutputIDs []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithIssuer(issuerAddress, nil, func(output *Output) bool {
		foundOutputIDs = append(foundOutputIDs, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, nftOutputIDs, foundOutputIDs)

	var aliasIssuerFoundOutputIDs []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithIssuer(aliasIssuerAddress, nil, func(output *Output) bool {
		aliasIssuerFoundOutputIDs = append(aliasIssuerFoundOutputIDs, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, aliasIssuerOutputIDs, aliasIssuerFoundOutputIDs)

	var aliasIssuerFoundAliasOutputIDs []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithIssuer(aliasIssuerAddress, FilterOutputType(iotago.OutputAlias), func(output *Output) bool {
		aliasIssuerFoundAliasOutputIDs = append(aliasIssuerFoundAliasOutputIDs, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, aliasIssuerAliasOutputIDs, aliasIssuerFoundAliasOutputIDs)
}

func TestUTXOSenderFilter(t *testing.T) {

	utxo := New(mapdb.NewMapDB())

	senderAddress := RandAddress(iotago.AddressEd25519)

	senderOutputIDs := []*iotago.OutputID{
		RandOutputID(),
		RandOutputID(),
		RandOutputID(),
		RandOutputID(),
		RandOutputID(),
	}
	senderOutputIDsWithIndex := []*iotago.OutputID{
		senderOutputIDs[3],
		senderOutputIDs[4],
	}
	senderOutputIDsWithIndexNFT := []*iotago.OutputID{
		senderOutputIDs[4],
	}

	outputs := Outputs{
		CreateOutput(senderOutputIDs[0], RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: senderAddress,
				},
			},
		}),
		CreateOutput(RandOutputID(), RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: RandAddress(iotago.AddressEd25519),
				},
			},
		}),
		CreateOutput(RandOutputID(), RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: RandAddress(iotago.AddressEd25519),
				},
			},
		}),
		CreateOutput(senderOutputIDs[1], RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: senderAddress,
				},
			},
		}),
		CreateOutput(senderOutputIDs[2], RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: senderAddress,
				},
				&iotago.IndexationFeatureBlock{
					Tag: []byte("TestingOther"),
				},
			},
		}),
		CreateOutput(senderOutputIDs[3], RandMessageID(), RandMilestoneIndex(), &iotago.ExtendedOutput{
			Amount:  59854598,
			Address: RandAddress(iotago.AddressEd25519),
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: senderAddress,
				},
				&iotago.IndexationFeatureBlock{
					Tag: []byte("Testing"),
				},
			},
		}),
		CreateOutput(senderOutputIDs[4], RandMessageID(), RandMilestoneIndex(), &iotago.NFTOutput{
			Address:           RandAddress(iotago.AddressAlias),
			Amount:            234342348,
			NFTID:             RandNFTID(),
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.SenderFeatureBlock{
					Address: senderAddress,
				},
				&iotago.IndexationFeatureBlock{
					Tag: []byte("Testing"),
				},
			},
		}),
	}

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(RandMilestoneIndex(), outputs, nil, nil, nil))

	loadUnspentOutputs, err := utxo.UnspentExtendedOutputs(nil)
	require.NoError(t, err)
	nftOutputs, err := utxo.UnspentNFTOutputs(nil)
	require.NoError(t, err)
	require.ElementsMatch(t, outputs, append(loadUnspentOutputs, nftOutputs...))

	var foundOutputIDs []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithSender(senderAddress, nil, func(output *Output) bool {
		foundOutputIDs = append(foundOutputIDs, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, senderOutputIDs, foundOutputIDs)

	var foundOutputIDsWithIndex []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithSenderAndIndexTag(senderAddress, []byte("Testing"), nil, func(output *Output) bool {
		foundOutputIDsWithIndex = append(foundOutputIDsWithIndex, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, senderOutputIDsWithIndex, foundOutputIDsWithIndex)

	var foundOutputIDsWithIndexNFT []*iotago.OutputID
	require.NoError(t, utxo.ForEachUnspentOutputWithSenderAndIndexTag(senderAddress, []byte("Testing"), FilterOutputType(iotago.OutputNFT), func(output *Output) bool {
		foundOutputIDsWithIndexNFT = append(foundOutputIDsWithIndexNFT, output.OutputID())
		return true
	}))
	require.ElementsMatch(t, senderOutputIDsWithIndexNFT, foundOutputIDsWithIndexNFT)
}
