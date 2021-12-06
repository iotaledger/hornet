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

	initialOutput := randOutputOnAddressWithAmount(iotago.OutputExtended, randAddress(iotago.AddressEd25519), 2_134_656_365)
	require.NoError(t, utxo.AddUnspentOutput(initialOutput))
	require.NoError(t, utxo.AddUnspentOutput(randOutputOnAddressWithAmount(iotago.OutputAlias, randAddress(iotago.AddressAlias), 56_549_524)))
	require.NoError(t, utxo.AddUnspentOutput(randOutputOnAddressWithAmount(iotago.OutputFoundry, randAddress(iotago.AddressAlias), 25_548_858)))
	require.NoError(t, utxo.AddUnspentOutput(randOutputOnAddressWithAmount(iotago.OutputNFT, randAddress(iotago.AddressEd25519), 545_699_656)))
	require.NoError(t, utxo.AddUnspentOutput(randOutputOnAddressWithAmount(iotago.OutputExtended, randAddress(iotago.AddressAlias), 626_659_696)))

	msIndex := milestone.Index(756)

	outputs := Outputs{
		randOutputOnAddressWithAmount(iotago.OutputExtended, randAddress(iotago.AddressNFT), 2_134_656_365),
	}

	spents := Spents{
		randomSpent(initialOutput, msIndex),
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
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressEd25519)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressNFT)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressAlias)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressEd25519)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressNFT)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressAlias)),
		randOutputOnAddress(iotago.OutputExtended, randAddress(iotago.AddressEd25519)),
	}
	nftOutputs := Outputs{
		randOutputOnAddress(iotago.OutputNFT, randAddress(iotago.AddressEd25519)),
		randOutputOnAddress(iotago.OutputNFT, randAddress(iotago.AddressAlias)),
		randOutputOnAddress(iotago.OutputNFT, randAddress(iotago.AddressNFT)),
		randOutputOnAddress(iotago.OutputNFT, randAddress(iotago.AddressAlias)),
	}
	aliasOutputs := Outputs{
		randOutputOnAddress(iotago.OutputAlias, randAddress(iotago.AddressEd25519)),
	}
	foundryOutputs := Outputs{
		randOutputOnAddress(iotago.OutputFoundry, randAddress(iotago.AddressAlias)),
		randOutputOnAddress(iotago.OutputFoundry, randAddress(iotago.AddressAlias)),
		randOutputOnAddress(iotago.OutputFoundry, randAddress(iotago.AddressAlias)),
	}

	msIndex := milestone.Index(756)

	spents := Spents{
		randomSpent(extendedOutputs[3], msIndex),
		randomSpent(extendedOutputs[2], msIndex),
		randomSpent(nftOutputs[2], msIndex),
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

	address := randAddress(iotago.AddressEd25519)

	outputs := Outputs{
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 3242343),
		randOutput(iotago.OutputExtended),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 5898566), // spent
		randOutput(iotago.OutputExtended),                                      // spent
		randOutputOnAddressWithAmount(iotago.OutputNFT, address, 23432423),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 78632467),
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputAlias),
		randOutput(iotago.OutputNFT),
		randOutput(iotago.OutputNFT),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 98734278),
		randOutputOnAddressWithAmount(iotago.OutputAlias, address, 98734278),
	}

	msIndex := milestone.Index(756)
	spents := Spents{
		randomSpent(outputs[3], msIndex),
		randomSpent(outputs[2], msIndex),
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

	address := randAddress(iotago.AddressEd25519)

	nftID := randNFTID()
	nftOutputID := randOutputID()

	aliasID := randAliasID()
	aliasOutputID := randOutputID()

	foundryOutputID := randOutputID()
	foundryAlias := randAliasID().ToAddress()
	foundrySupply := new(big.Int).SetUint64(rand.Uint64())
	foundrySerialNumber := rand.Uint32()

	outputs := Outputs{
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 3242343),
		randOutput(iotago.OutputExtended),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 5898566), // spent
		randOutput(iotago.OutputExtended),                                      // spent
		CreateOutput(nftOutputID, randMessageID(), randMilestoneIndex(), &iotago.NFTOutput{
			Address:           address,
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
		}),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 78632467),
		randOutput(iotago.OutputExtended),
		randOutput(iotago.OutputAlias),
		randOutput(iotago.OutputNFT),
		randOutput(iotago.OutputNFT),
		randOutputOnAddressWithAmount(iotago.OutputExtended, address, 98734278),
		CreateOutput(aliasOutputID, randMessageID(), randMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              aliasID,
			StateController:      address,
			GovernanceController: address,
			StateMetadata:        []byte{},
		}),
		randOutput(iotago.OutputFoundry),
		CreateOutput(foundryOutputID, randMessageID(), randMilestoneIndex(), &iotago.FoundryOutput{
			Address:           foundryAlias,
			Amount:            2156548,
			SerialNumber:      foundrySerialNumber,
			TokenTag:          randTokenTag(),
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
		randomSpent(outputs[3], msIndex),
		randomSpent(outputs[2], msIndex),
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

	issuerAddress := randAddress(iotago.AddressEd25519)
	aliasIssuerAddress := randAddress(iotago.AddressEd25519)

	nftID := randNFTID()
	nftOutputIDs := []*iotago.OutputID{
		randOutputID(),
		randOutputID(),
	}

	aliasIssuerOutputIDs := []*iotago.OutputID{
		randOutputID(),
		randOutputID(),
	}
	aliasIssuerAliasOutputIDs := []*iotago.OutputID{
		aliasIssuerOutputIDs[0],
	}

	outputs := Outputs{
		CreateOutput(aliasIssuerAliasOutputIDs[0], randMessageID(), randMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              randAliasID(),
			StateController:      randAddress(iotago.AddressEd25519),
			GovernanceController: randAddress(iotago.AddressEd25519),
			StateMetadata:        []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: aliasIssuerAddress,
				},
			},
		}),
		CreateOutput(randOutputID(), randMessageID(), randMilestoneIndex(), &iotago.AliasOutput{
			Amount:               59854598,
			AliasID:              randAliasID(),
			StateController:      randAddress(iotago.AddressEd25519),
			GovernanceController: randAddress(iotago.AddressEd25519),
			StateMetadata:        []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: randAddress(iotago.AddressEd25519),
				},
			},
		}),
		CreateOutput(randOutputID(), randMessageID(), randMilestoneIndex(), &iotago.NFTOutput{
			Address:           randAddress(iotago.AddressAlias),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: randAddress(iotago.AddressAlias),
				},
			},
		}),
		CreateOutput(nftOutputIDs[0], randMessageID(), randMilestoneIndex(), &iotago.NFTOutput{
			Address:           randAddress(iotago.AddressNFT),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: issuerAddress,
				},
			},
		}),
		CreateOutput(aliasIssuerOutputIDs[1], randMessageID(), randMilestoneIndex(), &iotago.NFTOutput{
			Address:           randAddress(iotago.AddressEd25519),
			Amount:            234348,
			NFTID:             nftID,
			ImmutableMetadata: []byte{},
			Blocks: iotago.FeatureBlocks{
				&iotago.IssuerFeatureBlock{
					Address: aliasIssuerAddress,
				},
			},
		}),
		CreateOutput(nftOutputIDs[1], randMessageID(), randMilestoneIndex(), &iotago.NFTOutput{
			Address:           randAddress(iotago.AddressAlias),
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

	require.NoError(t, utxo.ApplyConfirmationWithoutLocking(randMilestoneIndex(), outputs, nil, nil, nil))

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
