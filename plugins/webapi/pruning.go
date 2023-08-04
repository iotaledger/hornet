package webapi

import (
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/plugins/snapshot"
)

func (s *WebAPIServer) rpcPruneDatabase(c echo.Context) (interface{}, error) {
	request := &PruneDatabase{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if (request.Depth != 0 && request.TargetIndex != 0) || (request.Depth == 0 && request.TargetIndex == 0) {
		return nil, errors.WithMessage(echo.ErrBadRequest, "Either depth or targetIndex has to be specified")
	}

	if request.Depth != 0 {
		if err := snapshot.PruneDatabaseByDepth(c.Request().Context(), request.Depth); err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	} else {
		if err := snapshot.PruneDatabaseByTargetIndex(c.Request().Context(), request.TargetIndex); err != nil {
			return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
		}
	}

	return &PruneDatabaseResponse{}, nil
}
