package indexer

import (
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	NullOutputID = iotago.OutputID{}
)

type outputIDBytes []byte
type addressBytes []byte
type nftIDBytes []byte
type aliasIDBytes []byte
type foundryIDBytes []byte

type status struct {
	ID          uint `gorm:"primaryKey;not null"`
	LedgerIndex milestone.Index
}

type queryResult struct {
	OutputID    outputIDBytes
	LedgerIndex milestone.Index
}

func (o outputIDBytes) ID() iotago.OutputID {
	id := iotago.OutputID{}
	copy(id[:], o)
	return id
}

type queryResults []queryResult

func (q queryResults) IDs() iotago.OutputIDs {
	outputIDs := iotago.OutputIDs{}
	for _, r := range q {
		outputIDs = append(outputIDs, r.OutputID.ID())
	}
	return outputIDs
}

func (a addressBytes) Address() (iotago.Address, error) {
	if len(a) != iotago.NFTAddressBytesLength || len(a) != iotago.AliasAddressBytesLength || len(a) != iotago.Ed25519AddressBytesLength {
		return nil, errors.New("invalid address length")
	}
	addr, err := iotago.AddressSelector(uint32(a[0]))
	if err != nil {
		return nil, err
	}
	_, err = addr.Deserialize(a, serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return nil, err
	}
	return addr, nil
}

func addressBytesForAddress(addr iotago.Address) (addressBytes, error) {
	return addr.Serialize(serializer.DeSeriModeNoValidation, nil)
}
