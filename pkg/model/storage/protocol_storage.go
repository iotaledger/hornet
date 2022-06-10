package storage

import (
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/serializer/v2"
	iotago "github.com/iotaledger/iota.go/v3"
	"github.com/pkg/errors"
)

const (
	protocolParasStorageKey = "protoParas"
)

func (s *Storage) StoreProtocolParameters(protoPras *iotago.ProtocolParameters) error {
	data, err := protoPras.Serialize(serializer.DeSeriModeNoValidation, nil)
	if err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to serialize protocol parameters")
	}

	if err := s.protocolStore.Set([]byte(protocolParasStorageKey), data); err != nil {
		return errors.Wrap(NewDatabaseError(err), "failed to protocol parameters")
	}

	return nil
}

func (s *Storage) ProtocolParameters() (*iotago.ProtocolParameters, error) {
	data, err := s.protocolStore.Get([]byte(protocolParasStorageKey))
	if err != nil {
		if !errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, errors.Wrap(NewDatabaseError(err), "failed to retrieve protocol parameters")
		}
		return nil, nil
	}

	protoParas := &iotago.ProtocolParameters{}
	if _, err := protoParas.Deserialize(data, serializer.DeSeriModeNoValidation, nil); err != nil {
		return nil, errors.Wrap(NewDatabaseError(err), "failed to deserialize protocol parameters")
	}

	return protoParas, nil
}
