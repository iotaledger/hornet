package storage

import (
	"github.com/iotaledger/hive.go/marshalutil"
)

type DustDiff struct {
	DustAllowanceBalanceDiff int64
	DustOutputCount          int64
}

func NewDustDiff(dustAllowanceBalance int64, dustOutputCount int64) *DustDiff {
	return &DustDiff{
		DustAllowanceBalanceDiff: dustAllowanceBalance,
		DustOutputCount:          dustOutputCount,
	}
}

func dustFromBytes(value []byte) (dustAllowanceBalance uint64, outputCount int64, err error) {
	marshalUtil := marshalutil.New(value)

	if dustAllowanceBalance, err = marshalUtil.ReadUint64(); err != nil {
		return
	}

	if outputCount, err = marshalUtil.ReadInt64(); err != nil {
		return
	}

	return
}

func bytesFromDust(dustAllowanceBalance uint64, outputCount int64) []byte {
	marshalUtil := marshalutil.New(16)
	marshalUtil.WriteUint64(dustAllowanceBalance)
	marshalUtil.WriteInt64(outputCount)
	return marshalUtil.Bytes()
}
