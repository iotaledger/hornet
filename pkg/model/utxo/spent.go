package utxo

import (
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	iotago "github.com/iotaledger/iota.go/v2"
)

type SpentConsumer func(spent *Spent) bool

// Spent are already spent TXOs (transaction outputs) per address
type Spent struct {
	kvStorable

	outputID *iotago.UTXOInputID
	output   *Output

	targetTransactionID *iotago.TransactionID
	confirmationIndex   milestone.Index
}

func (s *Spent) Output() *Output {
	return s.output
}

func (s *Spent) OutputID() *iotago.UTXOInputID {
	return s.output.outputID
}

func (s *Spent) MessageID() hornet.MessageID {
	return s.output.messageID
}

func (s *Spent) OutputType() iotago.OutputType {
	return s.output.outputType
}

func (s *Spent) Address() iotago.Address {
	return s.output.address
}

func (s *Spent) Amount() uint64 {
	return s.output.amount
}

func (s *Spent) TargetTransactionID() *iotago.TransactionID {
	return s.targetTransactionID
}

func (s *Spent) ConfirmationIndex() milestone.Index {
	return s.confirmationIndex
}

type Spents []*Spent

func NewSpent(output *Output, targetTransactionID *iotago.TransactionID, confirmationIndex milestone.Index) *Spent {
	return &Spent{
		outputID:            output.outputID,
		output:              output,
		targetTransactionID: targetTransactionID,
		confirmationIndex:   confirmationIndex,
	}
}

func (o *Output) spentDatabaseKey() []byte {
	ms := marshalutil.New(69)
	ms.WriteByte(UTXOStoreKeyPrefixSpent) // 1 byte
	ms.WriteBytes(o.addressBytes())       // 33 bytes
	ms.WriteByte(o.outputType)            // 1 byte
	ms.WriteBytes(o.outputID[:])          // 34 bytes
	return ms.Bytes()
}

func (s *Spent) kvStorableKey() (key []byte) {
	return s.output.spentDatabaseKey()
}

func (s *Spent) kvStorableValue() (value []byte) {
	ms := marshalutil.New(36)
	ms.WriteBytes(s.targetTransactionID[:])     // 32 bytes
	ms.WriteUint32(uint32(s.confirmationIndex)) // 4 bytes
	return ms.Bytes()
}

func (s *Spent) kvStorableLoad(_ *Manager, key []byte, value []byte) error {

	// Parse key
	keyUtil := marshalutil.New(key)

	// Read prefix
	if _, err := keyUtil.ReadByte(); err != nil {
		return err
	}

	// Read address
	if _, err := parseAddress(keyUtil); err != nil {
		return err
	}

	// Read output type
	if _, err := keyUtil.ReadByte(); err != nil {
		return err
	}

	// Read outputID
	var err error
	if s.outputID, err = parseOutputID(keyUtil); err != nil {
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
	s.confirmationIndex = milestone.Index(index)

	return nil
}

func (u *Manager) loadOutputOfSpent(s *Spent) error {

	key := byteutils.ConcatBytes([]byte{UTXOStoreKeyPrefixOutput}, s.outputID[:])
	value, err := u.utxoStorage.Get(key)
	if err != nil {
		return err
	}

	output := &Output{}
	if err = output.kvStorableLoad(u, key, value); err != nil {
		return err
	}

	s.output = output

	return nil
}

func (u *Manager) ForEachSpentOutput(consumer SpentConsumer, options ...UTXOIterateOption) error {

	consumerFunc := consumer

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error

	key := []byte{UTXOStoreKeyPrefixSpent}

	// Filter by address
	if opt.address != nil {
		addrBytes, err := opt.address.Serialize(iotago.DeSeriModeNoValidation)
		if err != nil {
			return err
		}
		key = byteutils.ConcatBytes(key, addrBytes)

		// Filter by output type
		if opt.filterOutputType != nil {
			key = byteutils.ConcatBytes(key, []byte{*opt.filterOutputType})
		}
	} else if opt.filterOutputType != nil {
		// Filter results instead of using prefix iteration
		consumerFunc = func(spent *Spent) bool {
			if spent.OutputType() == *opt.filterOutputType {
				return consumer(spent)
			}
			return true
		}
	}

	var i int

	if err := u.utxoStorage.Iterate(key, func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		spent := &Spent{}
		if err := spent.kvStorableLoad(u, key, value); err != nil {
			innerErr = err
			return false
		}

		if err := u.loadOutputOfSpent(spent); err != nil {
			innerErr = err
			return false
		}

		return consumerFunc(spent)
	}); err != nil {
		return err
	}

	return innerErr
}

func storeSpentAndRemoveUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {

	unspentKey := spent.Output().unspentDatabaseKey()
	spentKey := spent.kvStorableKey()

	if err := mutations.Delete(unspentKey); err != nil {
		return err
	}

	return mutations.Set(spentKey, spent.kvStorableValue())
}

func deleteSpentAndMarkUnspent(spent *Spent, mutations kvstore.BatchedMutations) error {
	if err := deleteSpent(spent, mutations); err != nil {
		return err
	}

	return markAsUnspent(spent.output, mutations)
}

func deleteSpent(spent *Spent, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(spent.kvStorableKey())
}

func (u *Manager) readSpentForOutputIDWithoutLocking(outputID *iotago.UTXOInputID) (*Spent, error) {

	output, err := u.ReadOutputByOutputIDWithoutLocking(outputID)
	if err != nil {
		return nil, err
	}

	key := output.spentDatabaseKey()
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
