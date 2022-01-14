package indexer

import (
	"time"

	"github.com/pkg/errors"
	
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

type outputIDBytes []byte
type addressBytes []byte
type nftIDBytes []byte
type aliasIDBytes []byte
type foundryIDBytes []byte

type status struct {
	ID          uint `gorm:"primarykey;not null"`
	LedgerIndex milestone.Index
}

type outputID struct {
	OutputID outputIDBytes
}

func (o *outputID) ID() *iotago.OutputID {
	id := &iotago.OutputID{}
	copy(id[:], o.OutputID)
	return id
}

type extendedOutput struct {
	OutputID            outputIDBytes `gorm:"primaryKey;not null"`
	Address             addressBytes  `gorm:"not null;index:extended_address"`
	Amount              uint64        `gorm:"not null"`
	Sender              addressBytes  `gorm:"index:extended_sender_tag"`
	Tag                 []byte        `gorm:"index:extended_sender_tag"`
	DustReturn          *uint64
	TimelockMilestone   *milestone.Index
	TimelockTime        *time.Time
	ExpirationMilestone *milestone.Index
	ExpirationTime      *time.Time
}

type foundry struct {
	FoundryID foundryIDBytes `gorm:"primaryKey;not null"`
	OutputID  outputIDBytes  `gorm:"unique;not null"`
	Amount    uint64         `gorm:"not null"`
	AliasID   aliasIDBytes   `gorm:"not null;index:foundries_alias_id"`
}

type nft struct {
	NFTID               nftIDBytes    `gorm:"primaryKey;not null"`
	OutputID            outputIDBytes `gorm:"unique;not null"`
	Amount              uint64        `gorm:"not null"`
	Issuer              addressBytes  `gorm:"index:nft_issuer"`
	Sender              addressBytes  `gorm:"index:nft_sender_tag"`
	Tag                 []byte        `gorm:"index:nft_sender_tag"`
	DustReturn          *uint64
	TimelockMilestone   *milestone.Index
	TimelockTime        *time.Time
	ExpirationMilestone *milestone.Index
	ExpirationTime      *time.Time
}

type alias struct {
	AliasID              aliasIDBytes  `gorm:"primaryKey;not null"`
	OutputID             outputIDBytes `gorm:"unique;not null"`
	Amount               uint64        `gorm:"not null"`
	StateController      addressBytes  `gorm:"not null;index:alias_state_controller""`
	GovernanceController addressBytes  `gorm:"not null;index:alias_governance_controller"`
	Issuer               addressBytes  `gorm:"index:alias_issuer"`
	Sender               addressBytes  `gorm:"index:alias_sender"`
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

func newAddressBytes(addr iotago.Address) (addressBytes, error) {
	return addr.Serialize(serializer.DeSeriModeNoValidation, nil)
}
