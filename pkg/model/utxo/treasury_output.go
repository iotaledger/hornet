package utxo

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/marshalutil"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (
	// A prefix which denotes a spent treasury output.
	// Do not modify the value since we're writing this as a bool.
	TreasuryOutputSpentPrefix = 1
	// A prefix which denotes an unspent treasury output.
	// Do not modify the value since we're writing this as a bool.
	TreasuryOutputUnspentPrefix = 0
)

var (
	// ErrInvalidTreasuryState is returned when the state of the treasury is invalid.
	ErrInvalidTreasuryState = errors.New("invalid treasury state")
)

// TreasuryOutput represents the output of a treasury transaction.
type TreasuryOutput struct {
	// The ID of the milestone which generated this output.
	MilestoneID iotago.MilestoneID
	// The amount residing on this output.
	Amount uint64
	// Whether this output was already spent
	Spent bool
}

type jsonTreasuryOutput struct {
	MilestoneID string `json:"milestoneId"`
	Amount      string `json:"amount"`
}

func (t *TreasuryOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonTreasuryOutput{
		MilestoneID: t.MilestoneID.ToHex(),
		Amount:      iotago.EncodeUint64(t.Amount),
	})
}

func (t *TreasuryOutput) UnmarshalJSON(bytes []byte) error {
	j := &jsonTreasuryOutput{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}

	if len(j.MilestoneID) == 0 {
		return errors.New("missing milestone ID")
	}
	milestoneID, err := iotago.DecodeHex(j.MilestoneID)
	if err != nil {
		return err
	}
	if len(milestoneID) != iotago.MilestoneIDLength {
		return fmt.Errorf("invalid milestone ID length: %d", len(milestoneID))
	}

	copy(t.MilestoneID[:], milestoneID)

	t.Amount, err = iotago.DecodeUint64(j.Amount)
	if err != nil {
		return fmt.Errorf("invalid amount: %w", err)
	}

	return nil
}

func (t *TreasuryOutput) kvStorableKey() (key []byte) {
	return marshalutil.New(34).
		WriteByte(UTXOStoreKeyPrefixTreasuryOutput). // 1 byte
		WriteBool(t.Spent).                          // 1 byte
		WriteBytes(t.MilestoneID[:]).                // 32 bytes
		Bytes()
}

func (t *TreasuryOutput) kvStorableValue() (value []byte) {
	return marshalutil.New(8).
		WriteUint64(t.Amount). // 8 bytes
		Bytes()
}

func (t *TreasuryOutput) kvStorableLoad(_ *Manager, key []byte, value []byte) error {
	keyExt := marshalutil.New(key)
	// skip prefix
	if _, err := keyExt.ReadByte(); err != nil {
		return err
	}

	spent, err := keyExt.ReadBool()
	if err != nil {
		return err
	}

	milestoneID, err := keyExt.ReadBytes(iotago.MilestoneIDLength)
	if err != nil {
		return err
	}
	copy(t.MilestoneID[:], milestoneID)

	val := marshalutil.New(value)
	t.Amount, err = val.ReadUint64()
	if err != nil {
		return err
	}

	t.Spent = spent

	return nil
}

// stores the given treasury output.
func storeTreasuryOutput(output *TreasuryOutput, mutations kvstore.BatchedMutations) error {
	return mutations.Set(output.kvStorableKey(), output.kvStorableValue())
}

// deletes the given treasury output.
func deleteTreasuryOutput(output *TreasuryOutput, mutations kvstore.BatchedMutations) error {
	return mutations.Delete(output.kvStorableKey())
}

// marks the given treasury output as spent.
func markTreasuryOutputAsSpent(output *TreasuryOutput, mutations kvstore.BatchedMutations) error {
	outputCopy := *output
	outputCopy.Spent = false
	if err := mutations.Delete(outputCopy.kvStorableKey()); err != nil {
		return err
	}
	outputCopy.Spent = true

	return mutations.Set(outputCopy.kvStorableKey(), outputCopy.kvStorableValue())
}

// marks the given treasury output as unspent.
func markTreasuryOutputAsUnspent(output *TreasuryOutput, mutations kvstore.BatchedMutations) error {
	outputCopy := *output
	outputCopy.Spent = true
	if err := mutations.Delete(outputCopy.kvStorableKey()); err != nil {
		return err
	}
	outputCopy.Spent = false

	return mutations.Set(outputCopy.kvStorableKey(), outputCopy.kvStorableValue())
}

func (u *Manager) readSpentTreasuryOutputWithoutLocking(msHash []byte) (*TreasuryOutput, error) {
	key := append([]byte{UTXOStoreKeyPrefixTreasuryOutput, TreasuryOutputSpentPrefix}, msHash...)
	val, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}
	to := &TreasuryOutput{}
	if err := to.kvStorableLoad(u, key, val); err != nil {
		return nil, err
	}

	return to, nil
}

func (u *Manager) readUnspentTreasuryOutputWithoutLocking(msHash []byte) (*TreasuryOutput, error) {
	key := append([]byte{UTXOStoreKeyPrefixTreasuryOutput, TreasuryOutputUnspentPrefix}, msHash...)
	val, err := u.utxoStorage.Get(key)
	if err != nil {
		return nil, err
	}
	to := &TreasuryOutput{}
	if err := to.kvStorableLoad(u, key, val); err != nil {
		return nil, err
	}

	return to, nil
}

// StoreUnspentTreasuryOutput stores the given unspent treasury output and also deletes
// any existing unspent one in the same procedure.
func (u *Manager) StoreUnspentTreasuryOutput(to *TreasuryOutput) error {
	if to.Spent {
		panic("provided spent output to persist as new unspent treasury output")
	}

	mutations, err := u.utxoStorage.Batched()
	if err != nil {
		return err
	}

	existing, err := u.UnspentTreasuryOutputWithoutLocking()
	if err == nil {
		if err = mutations.Delete(existing.kvStorableKey()); err != nil {
			mutations.Cancel()

			return err
		}
	}

	if err := mutations.Set(to.kvStorableKey(), to.kvStorableValue()); err != nil {
		mutations.Cancel()

		return err
	}

	return mutations.Commit()
}

// UnspentTreasuryOutputWithoutLocking returns the unspent treasury output.
func (u *Manager) UnspentTreasuryOutputWithoutLocking() (*TreasuryOutput, error) {
	var i int
	var innerErr error
	var unspentTreasuryOutput *TreasuryOutput
	if err := u.utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixTreasuryOutput, TreasuryOutputUnspentPrefix}, func(key kvstore.Key, value kvstore.Value) bool {
		i++
		unspentTreasuryOutput = &TreasuryOutput{}
		if err := unspentTreasuryOutput.kvStorableLoad(u, key, value); err != nil {
			innerErr = err

			return false
		}

		return true
	}); err != nil {
		return nil, err
	}

	if innerErr != nil {
		return nil, innerErr
	}

	switch {
	case i > 1:
		return nil, fmt.Errorf("%w: more than one unspent treasury output exists", ErrInvalidTreasuryState)
	case i == 0:
		return nil, fmt.Errorf("%w: no treasury output exists", ErrInvalidTreasuryState)
	}

	return unspentTreasuryOutput, nil
}

// TreasuryOutputConsumer is a function that consumes an output.
// Returning false from this function indicates to abort the iteration.
type TreasuryOutputConsumer func(output *TreasuryOutput) bool

// ForEachTreasuryOutput iterates over all stored treasury outputs.
func (u *Manager) ForEachTreasuryOutput(consumer TreasuryOutputConsumer, options ...IterateOption) error {

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixTreasuryOutput}, func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		output := &TreasuryOutput{}
		if err := output.kvStorableLoad(u, key, value); err != nil {
			innerErr = err

			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}

func (u *Manager) ForEachSpentTreasuryOutput(consumer TreasuryOutputConsumer, options ...IterateOption) error {

	opt := iterateOptions(options)

	if opt.readLockLedger {
		u.ReadLockLedger()
		defer u.ReadUnlockLedger()
	}

	var innerErr error
	var i int
	if err := u.utxoStorage.Iterate([]byte{UTXOStoreKeyPrefixTreasuryOutput, TreasuryOutputSpentPrefix}, func(key kvstore.Key, value kvstore.Value) bool {

		if (opt.maxResultCount > 0) && (i >= opt.maxResultCount) {
			return false
		}

		i++

		output := &TreasuryOutput{}
		if err := output.kvStorableLoad(u, key, value); err != nil {
			innerErr = err

			return false
		}

		return consumer(output)
	}); err != nil {
		return err
	}

	return innerErr
}
