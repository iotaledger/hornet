package coreapi

import (
	"fmt"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/bytes"
	"github.com/pkg/errors"

	"github.com/iotaledger/inx-app/httpserver"
	iotago "github.com/iotaledger/iota.go/v3"
)

func pruneDatabase(c echo.Context) (*pruneDatabaseResponse, error) {

	if deps.SnapshotManager.IsSnapshotting() || deps.PruningManager.IsPruning() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is already creating a snapshot or pruning is running")
	}

	request := &pruneDatabaseRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if (request.Index == nil && request.Depth == nil && request.TargetDatabaseSize == nil) ||
		(request.Index != nil && request.Depth != nil) ||
		(request.Index != nil && request.TargetDatabaseSize != nil) ||
		(request.Depth != nil && request.TargetDatabaseSize != nil) {
		return nil, errors.WithMessage(httpserver.ErrInvalidParameter, "either index, depth or size has to be specified")
	}

	var err error
	var targetIndex iotago.MilestoneIndex

	if request.Index != nil {
		targetIndex, err = deps.PruningManager.PruneDatabaseByTargetIndex(Plugin.Daemon().ContextStopped(), *request.Index)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	if request.Depth != nil {
		targetIndex, err = deps.PruningManager.PruneDatabaseByDepth(Plugin.Daemon().ContextStopped(), *request.Depth)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	if request.TargetDatabaseSize != nil {
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(*request.TargetDatabaseSize)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}

		targetIndex, err = deps.PruningManager.PruneDatabaseBySize(Plugin.Daemon().ContextStopped(), pruningTargetDatabaseSizeBytes)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	return &pruneDatabaseResponse{
		Index: targetIndex,
	}, nil
}

func createSnapshots(c echo.Context) (*createSnapshotsResponse, error) {

	if deps.SnapshotManager.IsSnapshotting() || deps.PruningManager.IsPruning() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is already creating a snapshot or pruning is running")
	}

	request := &createSnapshotsRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(httpserver.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if request.Index == 0 {
		return nil, errors.WithMessage(httpserver.ErrInvalidParameter, "index needs to be specified")
	}

	filePath := filepath.Join(filepath.Dir(deps.SnapshotsFullPath), fmt.Sprintf("full_snapshot_%d.bin", request.Index))
	if err := deps.SnapshotManager.CreateFullSnapshot(Plugin.Daemon().ContextStopped(), request.Index, filePath, false); err != nil {
		return nil, errors.WithMessagef(echo.ErrInternalServerError, "creating snapshot failed: %s", err)
	}

	return &createSnapshotsResponse{
		Index:    request.Index,
		FilePath: filePath,
	}, nil
}
