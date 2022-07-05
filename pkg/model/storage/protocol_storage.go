package storage

import (
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
)

// ProtocolParamsMilestoneOptConsumer consumes the given ProtocolParamsMilestoneOpt.
type ProtocolParamsMilestoneOptConsumer func(*iotago.ProtocolParamsMilestoneOpt) bool

func (s *Storage) StoreProtocolParametersMilestoneOption(protoParamsMsOption *iotago.ProtocolParamsMilestoneOpt) error {
	s.protocolStoreLock.Lock()
	defer s.protocolStoreLock.Unlock()

	data, err := protoParamsMsOption.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to serialize protocol parameters milestone option")
	}

	if err := s.protocolStore.Set(databaseKeyForMilestoneIndex(protoParamsMsOption.TargetMilestoneIndex), data); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to store protocol parameters milestone option")
	}

	return nil
}

func (s *Storage) ProtocolParametersMilestoneOption(msIndex iotago.MilestoneIndex) (*iotago.ProtocolParamsMilestoneOpt, error) {
	s.protocolStoreLock.RLock()
	defer s.protocolStoreLock.RUnlock()

	// search the smallest activation index that is smaller than or equal to the given milestone index
	// to get the valid protocol parameters milestone option for the given milestone index.
	var smallestIndex iotago.MilestoneIndex
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

func (s *Storage) ProtocolParameters(msIndex iotago.MilestoneIndex) (*iotago.ProtocolParameters, error) {

	protoParamsMsOption, err := s.ProtocolParametersMilestoneOption(msIndex)
	if err != nil {
		return nil, err
	}

	protoParams := &iotago.ProtocolParameters{}
	if _, err := protoParams.Deserialize(protoParamsMsOption.Params, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters")
	}

	return protoParams, nil
}

func (s *Storage) ForEachProtocolParameterMilestoneOption(consumer ProtocolParamsMilestoneOptConsumer) error {
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

func (s *Storage) PruneProtocolParameterMilestoneOptions(pruningIndex iotago.MilestoneIndex) error {
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

func (s *Storage) CurrentProtocolParameters() (*iotago.ProtocolParameters, error) {
	ledgerIndex, err := s.UTXOManager().ReadLedgerIndex()
	if err != nil {
		return nil, fmt.Errorf("loading current protocol parameters failed: %w", err)
	}

	return s.ProtocolParameters(ledgerIndex)
}
