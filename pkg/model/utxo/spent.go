package utxo

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

type SpentConsumer func(spent *Spent) bool

// Spent are already spent TXOs (transaction outputs)
type Spent struct {
	kvStorable

	outputID            *iotago.OutputID
	targetTransactionID *iotago.TransactionID
	milestoneIndex      milestone.Index

	output *Output
}

func (s *Spent) Output() *Output {
	return s.output
}

func (s *Spent) OutputID() *iotago.OutputID {
	return s.outputID
}

func (s *Spent) mapKey() string {
	return string(s.outputID[:])
}

func (s *Spent) MessageID() hornet.MessageID {
	return s.output.MessageID()
}

func (s *Spent) OutputType() iotago.OutputType {
	return s.output.OutputType()
}

func (s *Spent) Address() iotago.Address {
	return s.output.Address()
}

func (s *Spent) Deposit() uint64 {
	return s.output.Deposit()
}

func (s *Spent) TargetTransactionID() *iotago.TransactionID {
	return s.targetTransactionID
}

func (s *Spent) MilestoneIndex() milestone.Index {
	return s.milestoneIndex
}

type Spents []*Spent

func NewSpent(output *Output, targetTransactionID *iotago.TransactionID, confirmationIndex milestone.Index) *Spent {
	return &Spent{
		outputID:            output.outputID,
		output:              output,
		targetTransactionID: targetTransactionID,
		milestoneIndex:      confirmationIndex,
	}
}

func spentStorageKeyForOutputID(outputID *iotago.OutputID) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutputSpent) // 1 byte
	ms.WriteBytes(outputID[:])                  // 34 bytes
	return ms.Bytes()
}

func (s *Spent) kvStorableKey() (key []byte) {
	return spentStorageKeyForOutputID(s.outputID)
}

func (s *Spent) kvStorableValue() (value []byte) {
	ms := marshalutil.New(36)
	ms.WriteBytes(s.targetTransactionID[:])  // 32 bytes
	ms.WriteUint32(uint32(s.milestoneIndex)) // 4 bytes
	return ms.Bytes()
}

func (s *Spent) kvStorableLoad(_ *Manager, key []byte, value []byte) error {

	// Parse key
	keyUtil := marshalutil.New(key)

	// Read prefix output
	_, err := keyUtil.ReadByte()
	if err != nil {
		return err
	}

	// Read OutputID
	if s.outputID, err = ParseOutputID(keyUtil); err != nil {
		return err
	}

	// Parse value
	valueUtil := marshalutil.New(value)

	// Read transaction ID
	if s.targetTransactionID, err = parseTransactionID(valueUtil); err != nil {
		return err
	}

	// Read milestone index
	index, err := valueUtil.ReadUint32()
	if err != nil {
		return err
	}
	s.milestoneIndex = milestone.Index(index)

	return nil
}

func (u *Manager) loadOutputOfSpent(s *Spent) error {
	output, err := u.ReadOutputByOutputIDWithoutLocking(s.outputID)
	if err != nil {
		return err
	}
	s.output = output
	return nil
}

func (u *Manager) readSpentForOutputIDWithoutLocking(outputID *iotago.OutputID) (*Spent, error) {
	output, err := u.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return nil, err
	}

	key := spentStorageKeyForOutputID(outputID)
	value, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}

	spent := &Spent{}
	if err := spent.kvStorableLoad(u, key, value); err != nil {
		return nil, err
	}

	spent.output = output
	return spent, nil
}

func storeSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Set(spent.kvStorableKey(), spent.kvStorableValue())
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(spent.kvStorableKey())
}
