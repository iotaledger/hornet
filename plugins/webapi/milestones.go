package webapi

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/pkg/model/tangle"
)

func (s *WebAPIServer) milestone(c echo.Context) (interface{}, error) {
	msIndexIotaGo, err := ParseMilestoneIndexParam(c, ParameterMilestoneIndex)
	if err != nil {
		return nil, err
	}
	msIndex := milestone.Index(msIndexIotaGo)

	smi := tangle.GetSolidMilestoneIndex()
	if msIndex > smi {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid milestone index: %d, lsmi is %d", msIndex, smi)
	}

	cachedMsBndl := tangle.GetMilestoneOrNil(msIndex)
	if cachedMsBndl == nil {
		return nil, fmt.Errorf("milestone not found: %d", msIndex)
	}
	defer cachedMsBndl.Release(true)

	cachedTx := cachedMsBndl.GetBundle().GetTail()
	defer cachedTx.Release(true)

	return milestoneResponse{
		MilestoneIndex:     msIndex,
		MilestoneHash:      cachedTx.GetTransaction().Tx.Hash,
		MilestoneTimestamp: cachedTx.GetTransaction().Tx.Timestamp,
	}, nil
}
