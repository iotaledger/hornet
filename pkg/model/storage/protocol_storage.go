package storage

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

var (
	ErrProtocolParamsMilestoneOptAlreadyExists = errors.New("protocol parameters milestone option already exists")
)

const (
	MaxProtocolParametersActivationRange uint32 = 30
)

// ProtocolParamsMilestoneOptConsumer consumes the given ProtocolParamsMilestoneOpt.
// Returning false from this function indicates to abort the iteration.
type ProtocolParamsMilestoneOptConsumer func(*iotago.ProtocolParamsMilestoneOpt) bool

type ProtocolStorage struct {
	protocolStore     kvstore.KVStore
	protocolStoreLock sync.RWMutex
}

func NewProtocolStorage(protocolStore kvstore.KVStore) *ProtocolStorage {
	return &ProtocolStorage{
		protocolStore: protocolStore,
	}
}

// smallestActivationIndex searches the smallest activation index that is smaller than or equal to the given milestone index.
func (s *ProtocolStorage) smallestActivationIndex(msIndex iotago.MilestoneIndex) (iotago.MilestoneIndex, error) {
	var smallestIndex iotago.MilestoneIndex
	var smallestIndexFound bool

	if err := s.protocolStore.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		activationIndex := milestoneIndexFromDatabaseKey(key)

		if activationIndex >= smallestIndex && activationIndex <= msIndex {
			smallestIndex = activationIndex
			smallestIndexFound = true
		}

		return true
	}); err != nil {
		return 0, err
	}

	if !smallestIndexFound {
		return 0, errors.New("no protocol parameters milestone option found for the given milestone index")
	}

	return smallestIndex, nil
}

func (s *ProtocolStorage) StoreProtocolParametersMilestoneOption(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) error {
	s.protocolStoreLock.Lock()
	defer s.protocolStoreLock.Unlock()

	key := databaseKeyForMilestoneIndex(protoParamsMsOption.TargetMilestoneIndex)

	exists, err := s.protocolStore.Has(key)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to check if protocol parameters milestone option exists")
	}
	if exists {
		return errors.Wrapf(NewDatabaseError(ErrProtocolParamsMilestoneOptAlreadyExists), "target index %d already exists", protoParamsMsOption.TargetMilestoneIndex)
	}

	data, err := protoParamsMsOption.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to serialize protocol parameters milestone option")
	}

	if err := s.protocolStore.Set(key, data); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store protocol parameters milestone option")
	}

	return nil
}

func (s *ProtocolStorage) ProtocolParametersMilestoneOption(msIndex iotago.MilestoneIndex) (*iotago.ProtocolParamsMilestoneOpt, error) {
	s.protocolStoreLock.RLock()
	defer s.protocolStoreLock.RUnlock()

	smallestIndex, err := s.smallestActivationIndex(msIndex)
	if err != nil {
		return nil, err
	}

	data, err := s.protocolStore.Get(databaseKeyForMilestoneIndex(smallestIndex))
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve protocol parameters milestone option")
		}

		return nil, errors.Wrap(NewDatabaseError(err), "protocol parameters milestone option not found in database")
	}

	protoParamsMsOption := &iotago.ProtocolParamsMilestoneOpt{}
	if _, err := protoParamsMsOption.Deserialize(data, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters milestone option")
	}

	return protoParamsMsOption, nil

}

func (s *ProtocolStorage) ProtocolParameters(msIndex iotago.MilestoneIndex) (*iotago.ProtocolParameters, error) {

	protoParamsMsOption, err := s.ProtocolParametersMilestoneOption(msIndex)
	if err != nil {
		return nil, err
	}

	// TODO: needs to be adapted for when protocol parameters struct changes
	protoParams := &iotago.ProtocolParameters{}
	if _, err := protoParams.Deserialize(protoParamsMsOption.Params, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters")
	}

	return protoParams, nil
}

func (s *ProtocolStorage) ForEachProtocolParameterMilestoneOption(consumer ProtocolParamsMilestoneOptConsumer) error {
	s.protocolStoreLock.RLock()
	defer s.protocolStoreLock.RUnlock()

	var innerErr error
	if err := s.protocolStore.Iterate(kvstore.EmptyPrefix, func(_ kvstore.Key, value kvstore.Value) bool {
		protoParamsMsOption := &iotago.ProtocolParamsMilestoneOpt{}
		if _, err := protoParamsMsOption.Deserialize(value, serializer.DeSeriModeNoValidation, nil); err != nil {
			innerErr = errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters milestone option")

			return false
		}

		return consumer(protoParamsMsOption)
	}); err != nil {
		return err
	}

	return innerErr
}

func (s *ProtocolStorage) ForEachActiveProtocolParameterMilestoneOption(msIndex iotago.MilestoneIndex, consumer ProtocolParamsMilestoneOptConsumer) error {
	s.protocolStoreLock.RLock()
	defer s.protocolStoreLock.RUnlock()

	smallestIndex, err := s.smallestActivationIndex(msIndex)
	if err != nil {
		return err
	}

	var innerErr error
	if err := s.protocolStore.Iterate(kvstore.EmptyPrefix, func(_ kvstore.Key, value kvstore.Value) bool {
		protoParamsMsOption := &iotago.ProtocolParamsMilestoneOpt{}
		if _, err := protoParamsMsOption.Deserialize(value, serializer.DeSeriModeNoValidation, nil); err != nil {
			innerErr = errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters milestone option")

			return false
		}

		if protoParamsMsOption.TargetMilestoneIndex < smallestIndex {
			// protocol parameters are older than the smallest index => not active
			return true
		}

		if protoParamsMsOption.TargetMilestoneIndex > msIndex+MaxProtocolParametersActivationRange {
			// protocol parameters are newer than the given index + the max activation range => they do not count as active
			return true
		}

		return consumer(protoParamsMsOption)
	}); err != nil {
		return err
	}

	return innerErr
}

func (s *ProtocolStorage) PruneProtocolParameterMilestoneOptions(pruningIndex iotago.MilestoneIndex) error {
	s.protocolStoreLock.Lock()
	defer s.protocolStoreLock.Unlock()

	// we will prune all protocol parameters milestone options that are smaller than the given pruning index,
	// except the last one, which is still valid.
	var biggestIndexBeforePruningIndex iotago.MilestoneIndex
	if err := s.protocolStore.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		activationIndex := milestoneIndexFromDatabaseKey(key)

		if activationIndex >= biggestIndexBeforePruningIndex && activationIndex <= pruningIndex {
			biggestIndexBeforePruningIndex = activationIndex
		}

		return true
	}); err != nil {
		return err
	}

	var innerErr error

	// we loop again to delete all protocol parameters milestone options that are smaller than the found index.
	if err := s.protocolStore.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		activationIndex := milestoneIndexFromDatabaseKey(key)

		if activationIndex < biggestIndexBeforePruningIndex {
			if err := s.protocolStore.Delete(key); err != nil {
				innerErr = err

				return false
			}
		}

		return true
	}); err != nil {
		return err
	}

	return innerErr
}

func (s *ProtocolStorage) ActiveProtocolParameterMilestoneOptionsHash(msIndex iotago.MilestoneIndex) ([]byte, error) {

	// compute the sha256 of the latest active protocol parameters (current+pending)
	protoParamsHash := sha256.New()

	activeProtoParamsMsOpts := []*iotago.ProtocolParamsMilestoneOpt{}
	if err := s.ForEachActiveProtocolParameterMilestoneOption(msIndex, func(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) bool {
		activeProtoParamsMsOpts = append(activeProtoParamsMsOpts, protoParamsMsOption)

		return true
	}); err != nil {
		return nil, fmt.Errorf("failed to iterate over protocol parameters milestone options: %w", err)
	}

	// sort by target index, oldest index first
	sort.Slice(activeProtoParamsMsOpts, func(i int, j int) bool {
		return activeProtoParamsMsOpts[i].TargetMilestoneIndex < activeProtoParamsMsOpts[j].TargetMilestoneIndex
	})

	for _, protoParamsMsOption := range activeProtoParamsMsOpts {
		data, err := protoParamsMsOption.Serialize(serializer.DeSeriModeNoValidation, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize protocol parameters milestone option: %w", err)
		}

		if _, err = protoParamsHash.Write(data); err != nil {
			return nil, fmt.Errorf("failed to hash protocol parameters milestone option: %w", err)
		}
	}

	return protoParamsHash.Sum(nil), nil
}
