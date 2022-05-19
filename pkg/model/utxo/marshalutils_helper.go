package utxo

import (
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

func ParseOutputID(ms *marshalutil.MarshalUtil) (iotago.OutputID, error) {
	o := iotago.OutputID{}
	bytes, err := ms.ReadBytes(iotago.OutputIDLength)
	if err != nil {
		return o, err
	}
	copy(o[:], bytes)
	return o, nil
}

func parseTransactionID(ms *marshalutil.MarshalUtil) (iotago.TransactionID, error) {
	t := iotago.TransactionID{}
	bytes, err := ms.ReadBytes(iotago.TransactionIDLength)
	if err != nil {
		return t, err
	}
	copy(t[:], bytes)
	return t, nil
}

func ParseBlockID(ms *marshalutil.MarshalUtil) (iotago.BlockID, error) {
	bytes, err := ms.ReadBytes(iotago.BlockIDLength)
	if err != nil {
		return iotago.EmptyBlockID(), err
	}
	blockID := iotago.BlockID{}
	copy(blockID[:], bytes)
	return blockID, nil
}

func parseMilestoneIndex(ms *marshalutil.MarshalUtil) (milestone.Index, error) {
	index, err := ms.ReadUint32()
	if err != nil {
		return 0, err
	}
	return milestone.Index(index), nil
}

func parseAddress(ms *marshalutil.MarshalUtil) (iotago.Address, error) {

	addrType, err := ms.ReadByte()
	if err != nil {
		return nil, err
	}

	// Move the cursor back
	ms.ReadSeek(-1)

	addr, err := iotago.AddressSelector(uint32(addrType))
	if err != nil {
		return nil, err
	}

	address := addr.(iotago.Address)

	pre := ms.ReadOffset()
	readBytes, err := address.Deserialize(ms.ReadRemainingBytes(), serializer.DeSeriModePerformValidation, nil)
	if err != nil {
		return nil, err
	}
	post := ms.ReadOffset()

	bytesReadTooFar := post - pre - readBytes
	// Move the cursor back some bytes
	ms.ReadSeek(-bytesReadTooFar)

	return address, err
}
