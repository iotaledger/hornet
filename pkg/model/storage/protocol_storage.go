package storage

import (
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ProtocolParametersConsumer consumes the given protocol parameter during looping through all protocol parameters.
type ProtocolParametersConsumer func(*iotago.ProtocolParamsMilestoneOpt) bool

func (s *Storage) StoreProtocolParameters(protoParsMsOpt *iotago.ProtocolParamsMilestoneOpt) error {
	data, err := protoParsMsOpt.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to serialize protocol parameters")
	}

	if err := s.protocolStore.Set(databaseKeyForMilestoneIndex(milestone.Index(protoParsMsOpt.TargetMilestoneIndex)), data); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store protocol parameters")
	}

	return nil
}

func (s *Storage) ProtocolParameters(msIndex milestone.Index) (*iotago.ProtocolParamsMilestoneOpt, error) {

	// search the smallest activation index that is smaller than or equal to the given milestone index
	// to get the valid protocol parameters for the given milestone index.
	var smallestIndex milestone.Index
	if err := s.protocolStore.IterateKeys(kvstore.EmptyPrefix, func(key kvstore.Key) bool {
		activationIndex := milestoneIndexFromDatabaseKey(key)

		if activationIndex >= smallestIndex && activationIndex <= msIndex {
			smallestIndex = activationIndex
		}

		return true
	}); err != nil {
		return nil, err
	}

	data, err := s.protocolStore.Get(databaseKeyForMilestoneIndex(smallestIndex))
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve protocol parameters")
		}
		return nil, err
	}

	protoParsMsOpt := &iotago.ProtocolParamsMilestoneOpt{}
	if _, err := protoParsMsOpt.Deserialize(data, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters")
	}

	return protoParsMsOpt, nil
}

func (s *Storage) ForEachProtocolParameters(consumer ProtocolParametersConsumer) error {

	var innerErr error
	if err := s.protocolStore.Iterate(kvstore.EmptyPrefix, func(_ kvstore.Key, value kvstore.Value) bool {
		protoParsMsOpt := &iotago.ProtocolParamsMilestoneOpt{}
		if _, err := protoParsMsOpt.Deserialize(value, serializer.DeSeriModeNoValidation, nil); err != nil {
			innerErr = errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters")
			return false
		}

		return consumer(protoParsMsOpt)
	}); err != nil {
		return err
	}

	return innerErr
}

func (s *Storage) PruneProtocolParameters(pruningIndex milestone.Index) error {

	// we will prune all protocol parameters that are smaller than the given pruning index,
	// except the last one, which is still valid.
	var biggestIndexBeforePruningIndex milestone.Index
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

	// we loop again to delete all protocol parameters that are smaller than the found index.
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
