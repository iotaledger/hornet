package webapi

import (
	"fmt"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/model/milestone"
	"github.com/iotaledger/hornet/plugins/snapshot"
)

func (s *WebAPIServer) rpcCreateSnapshotFile(c echo.Context) (interface{}, error) {
	request := &CreateSnapshotFile{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	snapshotFilePath := filepath.Join(filepath.Dir(config.NodeConfig.GetString(config.CfgLocalSnapshotsPath)), fmt.Sprintf("export_%d.bin", request.TargetIndex))

	if err := snapshot.CreateLocalSnapshot(c.Request().Context(), milestone.Index(request.TargetIndex), snapshotFilePath, false); err != nil {
		return nil, errors.WithMessage(echo.ErrInternalServerError, err.Error())
	}

	return &CreateSnapshotFileResponse{}, nil
}
