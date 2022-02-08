package utils

import (
	"math/big"
	"math/rand"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

// returns length amount random bytes
func RandBytes(length int) []byte {
	var b []byte
	for i := 0; i < length; i++ {
		b = append(b, byte(rand.Intn(256)))
	}
	return b
}

func Rand32ByteHash() [32]byte {
	var h [32]byte
	copy(h[:], RandBytes(32))
	return h
}

func RandMessageID() hornet.MessageID {
	return RandBytes(iotago.MessageIDLength)
}

func RandTransactionID() *iotago.TransactionID {
	transactionID := &iotago.TransactionID{}
	copy(transactionID[:], RandBytes(iotago.TransactionIDLength))
	return transactionID
}

func RandNFTID() iotago.NFTID {
	nft := iotago.NFTID{}
	copy(nft[:], RandBytes(iotago.NFTIDLength))
	return nft
}

func RandAliasID() iotago.AliasID {
	alias := iotago.AliasID{}
	copy(alias[:], RandBytes(iotago.AliasIDLength))
	return alias
}

func RandTokenTag() iotago.TokenTag {
	tokenTag := iotago.TokenTag{}
	copy(tokenTag[:], RandBytes(iotago.TokenTagLength))
	return tokenTag
}

func RandMilestoneIndex() milestone.Index {
	return milestone.Index(rand.Uint32())
}

func RandAddress(addressType iotago.AddressType) iotago.Address {
	switch addressType {
	case iotago.AddressEd25519:
		address := &iotago.Ed25519Address{}
		addressBytes := RandBytes(32)
		copy(address[:], addressBytes)
		return address
	case iotago.AddressNFT:
		return RandNFTID().ToAddress()
	case iotago.AddressAlias:
		return RandAliasID().ToAddress()
	default:
		panic("unknown address type")
	}
}

func RandOutputID() *iotago.OutputID {
	outputID := &iotago.OutputID{}
	copy(outputID[:], RandBytes(iotago.OutputIDLength))
	return outputID
}

func RandOutputType() iotago.OutputType {
	return iotago.OutputType(byte(rand.Intn(3) + 3))
}

func RandOutput(outputType iotago.OutputType) iotago.Output {
	var addr iotago.Address
	if outputType == iotago.OutputFoundry {
		addr = RandAddress(iotago.AddressAlias)
	} else {
		addr = RandAddress(iotago.AddressEd25519)
	}
	return RandOutputOnAddress(outputType, addr)
}

func RandOutputOnAddress(outputType iotago.OutputType, address iotago.Address) iotago.Output {
	return RandOutputOnAddressWithAmount(outputType, address, rand.Uint64())
}

func RandOutputOnAddressWithAmount(outputType iotago.OutputType, address iotago.Address, amount uint64) iotago.Output {

	var iotaOutput iotago.Output

	switch outputType {
	case iotago.OutputBasic:
		iotaOutput = &iotago.BasicOutput{
			Amount: amount,
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: address,
				},
			},
		}
	case iotago.OutputAlias:
		iotaOutput = &iotago.AliasOutput{
			Amount:        amount,
			AliasID:       RandAliasID(),
			StateMetadata: []byte{},
			Conditions: iotago.UnlockConditions{
				&iotago.StateControllerAddressUnlockCondition{
					Address: address,
				},
				&iotago.GovernorAddressUnlockCondition{
					Address: address,
				},
			},
		}
	case iotago.OutputFoundry:
		if address.Type() != iotago.AddressAlias {
			panic("not an alias address")
		}
		supply := new(big.Int).SetUint64(rand.Uint64())
		iotaOutput = &iotago.FoundryOutput{
			Amount:            amount,
			SerialNumber:      0,
			TokenTag:          RandTokenTag(),
			CirculatingSupply: supply,
			MaximumSupply:     supply,
			TokenScheme:       &iotago.SimpleTokenScheme{},
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: address,
				},
			},
		}
	case iotago.OutputNFT:
		iotaOutput = &iotago.NFTOutput{
			Amount:            amount,
			NFTID:             RandNFTID(),
			ImmutableMetadata: []byte{},
			Conditions: iotago.UnlockConditions{
				&iotago.AddressUnlockCondition{
					Address: address,
				},
			},
		}
	default:
		panic("unhandled output type")
	}

	return iotaOutput
}
