package tangle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/iotaledger/hive.go/kvstore"
)

var (
	wfStore kvstore.KVStore
	// Returned when a wf-confirmation is already stored for a given milestone index.
	ErrWFConfirmationAlreadyStored = errors.New("wf-confirmation already stored")
	// Returned when a WhiteFlagConfirmation does not contain a WhiteFlagMutations.
	ErrWFMustContainMutations = errors.New("wf-confirmation must contain mutations struct")
)

func configureWhiteFlagStore(store kvstore.KVStore) {
	wfStore = store.WithRealm([]byte{StorePrefixWhiteFlag})
}

// StoreWhiteFlagConfirmation persists the given wf-confirmation object.
func StoreWhiteFlagConfirmation(conf *WhiteFlagConfirmation) error {
	confBytes, err := conf.MarshalBinary()
	if err != nil {
		return fmt.Errorf("unable to marshal wf-confirmation object: %w", err)
	}

	key := databaseKeyForMilestoneIndex(conf.MilestoneIndex)
	hasAlready, err := wfStore.Has(key)
	if err != nil {
		return fmt.Errorf("unable to check whether previous wf-confirmation object for milestone %d exists: %w", conf.MilestoneIndex, err)
	}
	if hasAlready {
		return fmt.Errorf("%w: index %d", ErrWFConfirmationAlreadyStored, conf.MilestoneIndex)
	}
	if err := wfStore.Set(key, confBytes); err != nil {
		return fmt.Errorf("unable to store wf-confirmation object: %w", err)
	}
	return nil
}

// GetWhiteFlagConfirmation gets the wf-confirmation object for the given milestone.
func GetWhiteFlagConfirmation(index milestone.Index) (*WhiteFlagConfirmation, error) {
	key := databaseKeyForMilestoneIndex(index)
	confBytes, err := wfStore.Get(key)
	if err != nil {
		if err != kvstore.ErrKeyNotFound {
			return nil, nil
		}
		return nil, err
	}

	conf := &WhiteFlagConfirmation{}
	if err := conf.UnmarshalBinary(confBytes); err != nil {
		return nil, fmt.Errorf("unable to unmarshal wf-confirmation object: %w", err)
	}

	return conf, nil
}

// WhiteFlagConfirmation represents a confirmation done via a milestone under the "white-flag" approach.
type WhiteFlagConfirmation struct {
	// The index of the milestone that got confirmed.
	MilestoneIndex milestone.Index `bson:"milestoneIndex" json:"milestone"`
	// The transaction hash of the tail transaction of the milestone that got confirmed.
	MilestoneHash hornet.Hash `bson:"milestoneHash" json:"milestoneHash"`
	// The ledger mutations and referenced transactions of this milestone.
	Mutations *WhiteFlagMutations `bson:"mutations" json:"mutations"`
}

func (w *WhiteFlagConfirmation) MarshalBinary() (data []byte, err error) {
	if w.Mutations == nil {
		return nil, fmt.Errorf("unable to serialize wf-confirmation: %w", ErrWFMustContainMutations)
	}

	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, w.MilestoneIndex); err != nil {
		return nil, fmt.Errorf("unable to serialize milestone index for wf-confirmation: %w", err)
	}
	if _, err := b.Write(w.MilestoneHash); err != nil {
		return nil, fmt.Errorf("unable to serialize milestone hash for wf-confirmation: %w", err)
	}

	serializeHashes := func(writer io.Writer, hashes hornet.Hashes, name string) error {
		if err := binary.Write(writer, binary.LittleEndian, uint64(len(hashes))); err != nil {
			return fmt.Errorf("unable to serialize mutations %s length prefix for wf-confirmation: %w", name, err)
		}

		for i := range hashes {
			if _, err := writer.Write(hashes[i]); err != nil {
				return fmt.Errorf("unable to serialize %s at pos %d for wf-confirmation: %w", name, i, err)
			}
		}

		return nil
	}

	if err := serializeHashes(&b, w.Mutations.TailsIncluded, "tail-included"); err != nil {
		return nil, err
	}

	if err := serializeHashes(&b, w.Mutations.TailsExcludedConflicting, "tail-excluded-conflicting"); err != nil {
		return nil, err
	}

	if err := serializeHashes(&b, w.Mutations.TailsExcludedZeroValue, "tail-excluded-zero-value"); err != nil {
		return nil, err
	}

	if err := serializeHashes(&b, w.Mutations.TailsReferenced, "tail-referenced"); err != nil {
		return nil, err
	}

	serializeAddrT5B1Map := func(writer io.Writer, m map[string]int64, name string) error {
		if err := binary.Write(writer, binary.LittleEndian, uint64(len(m))); err != nil {
			return fmt.Errorf("unable to serialize mutations addr map %s length prefix for wf-confirmation: %w", name, err)
		}

		for k, v := range m {
			// k isn't actually a valid UTF-8 string
			if _, err := writer.Write([]byte(k)); err != nil {
				return fmt.Errorf("unable to serialize addr map %s key for wf-confirmation: %w", name, err)
			}
			if err := binary.Write(writer, binary.LittleEndian, v); err != nil {
				return fmt.Errorf("unable to serialize addr map %s value for wf-confirmation: %w", name, err)
			}
		}

		return nil
	}

	if err := serializeAddrT5B1Map(&b, w.Mutations.NewAddressState, "new-address-state"); err != nil {
		return nil, err
	}

	if err := serializeAddrT5B1Map(&b, w.Mutations.AddressMutations, "address-mutations"); err != nil {
		return nil, err
	}

	if _, err := b.Write(w.Mutations.MerkleTreeHash); err != nil {
		return nil, fmt.Errorf("unable to serialize merkle tree hash hash for wf-confirmation: %w", err)
	}

	return b.Bytes(), nil
}

func (w *WhiteFlagConfirmation) UnmarshalBinary(data []byte) error {
	w.Mutations = &WhiteFlagMutations{}

	buf := bytes.NewBuffer(data)
	if err := binary.Read(buf, binary.LittleEndian, &w.MilestoneIndex); err != nil {
		return fmt.Errorf("unable to deserialize milestone index for wf-confirmation: %w", err)
	}
	w.MilestoneHash = make([]byte, 49)
	if _, err := buf.Read(w.MilestoneHash); err != nil {
		return fmt.Errorf("unable to deserialize milestone hash for wf-confirmation: %w", err)
	}

	deserializeHashes := func(r io.Reader, name string) (hornet.Hashes, error) {
		var length uint64
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return nil, fmt.Errorf("unable to deserialize mutations %s length prefix for wf-confirmation: %w", name, err)
		}

		var i uint64
		hashes := make(hornet.Hashes, length)
		for ; i < length; i++ {
			hash := make(hornet.Hash, 49)
			if _, err := buf.Read(hash); err != nil {
				return nil, fmt.Errorf("unable to deserialize %s hash for wf-confirmation: %w", name, err)
			}
			hashes[i] = hash
		}

		return hashes, nil
	}

	var err error
	w.Mutations.TailsIncluded, err = deserializeHashes(buf, "tail-included")
	if err != nil {
		return err
	}

	w.Mutations.TailsExcludedConflicting, err = deserializeHashes(buf, "tail-excluded-conflicting")
	if err != nil {
		return err
	}

	w.Mutations.TailsExcludedZeroValue, err = deserializeHashes(buf, "tail-excluded-zero-value")
	if err != nil {
		return err
	}

	w.Mutations.TailsReferenced, err = deserializeHashes(buf, "tail-referenced")
	if err != nil {
		return err
	}

	deserializeAddrT5B1Map := func(r io.Reader, name string) (map[string]int64, error) {
		var length uint64
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return nil, fmt.Errorf("unable to deserialize addr map %s length prefix for wf-confirmation: %w", name, err)
		}

		var i uint64
		// note it is clear that this isn't actually a map made up of valid UTF-8 strings
		m := map[string]int64{}
		for ; i < length; i++ {
			hash := make(hornet.Hash, 49)
			if _, err := buf.Read(hash); err != nil {
				return nil, fmt.Errorf("unable to deserialize addr map %s key for wf-confirmation: %w", name, err)
			}
			var value int64
			if err := binary.Read(r, binary.LittleEndian, &value); err != nil {
				return nil, fmt.Errorf("unable to dserialize addr map %s value for wf-confirmation: %w", name, err)
			}
			m[string(hash)] = value
		}

		return m, nil
	}

	w.Mutations.NewAddressState, err = deserializeAddrT5B1Map(buf, "new-address-state")
	if err != nil {
		return err
	}

	w.Mutations.AddressMutations, err = deserializeAddrT5B1Map(buf, "address-mutations")
	if err != nil {
		return err
	}

	return nil
}

// WhiteFlagMutations contains the ledger mutations and referenced transactions applied to a cone under the "white-flag" approach.
type WhiteFlagMutations struct {
	// The tails of bundles which mutate the ledger in the order in which they were applied.
	TailsIncluded hornet.Hashes `bson:"tailsIncluded" json:"tailsIncluded"`
	// The tails of bundles which were excluded as they were conflicting with the mutations.
	TailsExcludedConflicting hornet.Hashes `bson:"tailsExcludedConflicting" json:"tailsExcludedConflicting"`
	// The tails which were excluded because they were part of a zero or spam value transfer.
	TailsExcludedZeroValue hornet.Hashes `bson:"tailsExcludedZeroValue" json:"tailsExcludedZeroValue"`
	// The tails which were referenced by the milestone (should be the sum of TailsIncluded + TailsExcludedConflicting + TailsExcludedZeroValue).
	TailsReferenced hornet.Hashes `bson:"tailsReferenced" json:"tailsReferenced"`
	// Contains the updated state of the addresses which were mutated by the given confirmation.
	NewAddressState map[string]int64 `bson:"newAddressState" json:"newAddressState"`
	// Contains the mutations to the state of the addresses for the given confirmation.
	AddressMutations map[string]int64 `bson:"addressMutations" json:"addressMutations"`
	// The merkle tree root hash of all tails.
	MerkleTreeHash []byte `bson:"merkleTreeHash" json:"merkleTreeHash"`
}
