package utxo

import (
	"bytes"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

// SpentConsumer is a function that consumes a spent output.
// Returning false from this function indicates to abort the iteration.
type SpentConsumer func(spent *Spent) bool

// LexicalOrderedSpents are spents ordered in lexical order by their outputID.
type LexicalOrderedSpents []*Spent

func (l LexicalOrderedSpents) Len() int {
	return len(l)
}

func (l LexicalOrderedSpents) Less(i, j int) bool {
	return bytes.Compare(l[i].outputID[:], l[j].outputID[:]) < 0
}

func (l LexicalOrderedSpents) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

// Spent are already spent TXOs (transaction outputs).
type Spent struct {
	kvStorable

	outputID iotago.OutputID
	// the ID of the transaction that spent the output
	transactionIDSpent iotago.TransactionID
	// the index of the milestone that spent the output
	msIndexSpent iotago.MilestoneIndex
	// the timestamp of the milestone that spent the output
	msTimestampSpent uint32

	output *Output
}

func (s *Spent) Output() *Output {
	return s.output
}

func (s *Spent) OutputID() iotago.OutputID {
	return s.outputID
}

func (s *Spent) MapKey() string {
	return string(s.outputID[:])
}

func (s *Spent) BlockID() iotago.BlockID {
	return s.output.BlockID()
}

func (s *Spent) OutputType() iotago.OutputType {
	return s.output.OutputType()
}

func (s *Spent) Deposit() uint64 {
	return s.output.Deposit()
}

// TransactionIDSpent returns the ID of the transaction that spent the output.
func (s *Spent) TransactionIDSpent() iotago.TransactionID {
	return s.transactionIDSpent
}

// MilestoneIndexSpent returns the index of the milestone that spent the output.
func (s *Spent) MilestoneIndexSpent() iotago.MilestoneIndex {
	return s.msIndexSpent
}

// MilestoneTimestampSpent returns the timestamp of the milestone that spent the output.
func (s *Spent) MilestoneTimestampSpent() uint32 {
	return s.msTimestampSpent
}

type Spents []*Spent

func NewSpent(output *Output, transactionIDSpent iotago.TransactionID, msIndexSpent iotago.MilestoneIndex, msTimestampSpent uint32) *Spent {
	return &Spent{
		outputID:           output.outputID,
		output:             output,
		transactionIDSpent: transactionIDSpent,
		msIndexSpent:       msIndexSpent,
		msTimestampSpent:   msTimestampSpent,
	}
}

func spentStorageKeyForOutputID(outputID iotago.OutputID) []byte {
	ms := marshalutil.New(35)
	ms.WriteByte(UTXOStoreKeyPrefixOutputSpent) // 1 byte
	ms.WriteBytes(outputID[:])                  // 34 bytes

	return ms.Bytes()
}

func (s *Spent) KVStorableKey() (key []byte) {
	return spentStorageKeyForOutputID(s.outputID)
}

func (s *Spent) KVStorableValue() (value []byte) {
	ms := marshalutil.New(40)
	ms.WriteBytes(s.transactionIDSpent[:]) // 32 bytes
	ms.WriteUint32(s.msIndexSpent)         // 4 bytes
	ms.WriteUint32(s.msTimestampSpent)     // 4 bytes

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
	if s.transactionIDSpent, err = parseTransactionID(valueUtil); err != nil {
		return err
	}

	// Read milestone index
	s.msIndexSpent, err = valueUtil.ReadUint32()
	if err != nil {
		return err
	}

	// Read milestone timestamp
	if s.msTimestampSpent, err = valueUtil.ReadUint32(); err != nil {
		return err
	}

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

func (u *Manager) ReadSpentForOutputIDWithoutLocking(outputID iotago.OutputID) (*Spent, error) {
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
	return mutations.Set(spent.KVStorableKey(), spent.KVStorableValue())
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(spent.KVStorableKey())
}
