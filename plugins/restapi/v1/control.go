package v1

import (
	"fmt"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/bytes"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/restapi"
)

func pruneDatabase(c echo.Context) (*pruneDatabaseResponse, error) {

	if deps.Snapshot.IsSnapshottingOrPruning() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is already creating a snapshot or pruning is running")
	}

	request := &pruneDatabaseRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if (request.Index == nil && request.Depth == nil && request.TargetDatabaseSize == nil) ||
		(request.Index != nil && request.Depth != nil) ||
		(request.Index != nil && request.TargetDatabaseSize != nil) ||
		(request.Depth != nil && request.TargetDatabaseSize != nil) {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "either index, depth or size has to be specified")
	}

	var err error
	var targetIndex milestone.Index

	if request.Index != nil {
		targetIndex, err = deps.Snapshot.PruneDatabaseByTargetIndex(*request.Index)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	if request.Depth != nil {
		targetIndex, err = deps.Snapshot.PruneDatabaseByDepth(*request.Depth)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	if request.TargetDatabaseSize != nil {
		pruningTargetDatabaseSizeBytes, err := bytes.Parse(*request.TargetDatabaseSize)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}

		targetIndex, err = deps.Snapshot.PruneDatabaseBySize(pruningTargetDatabaseSizeBytes)
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	}

	return &pruneDatabaseResponse{
		Index: targetIndex,
	}, nil
}

func createSnapshots(c echo.Context) (*createSnapshotsResponse, error) {

	if deps.Snapshot.IsSnapshottingOrPruning() {
		return nil, errors.WithMessage(echo.ErrServiceUnavailable, "node is already creating a snapshot or pruning is running")
	}

	request := &createSnapshotsRequest{}
	if err := c.Bind(request); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if request.FullIndex == nil && request.DeltaIndex == nil {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "at least fullIndex or deltaIndex has to be specified")
	}

	var fullIndex, deltaIndex milestone.Index
	var fullSnapshotFilePath, deltaSnapshotFilePath string

	if request.FullIndex != nil {
		fullIndex = milestone.Index(*request.FullIndex)
		fullSnapshotFilePath = filepath.Join(filepath.Dir(deps.SnapshotsFullPath), fmt.Sprintf("full_snapshot_%d.bin", fullIndex))

		// ToDo: abort signal?
		if err := deps.Snapshot.CreateFullSnapshot(fullIndex, fullSnapshotFilePath, false, nil); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "creating full snapshot failed: %s", err)
		}
	}

	if request.DeltaIndex != nil {
		deltaIndex = milestone.Index(*request.DeltaIndex)
		deltaSnapshotFilePath = filepath.Join(filepath.Dir(deps.SnapshotsDeltaPath), fmt.Sprintf("delta_snapshot_%d.bin", deltaIndex))

		// ToDo: abort signal?
		// if no full snapshot was created, the last existing full snapshot will be used
		if err := deps.Snapshot.CreateDeltaSnapshot(deltaIndex, deltaSnapshotFilePath, false, nil, fullSnapshotFilePath); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "creating delta snapshot failed: %s", err)
		}
	}

	return &createSnapshotsResponse{
		FullIndex:     fullIndex,
		FullFilePath:  fullSnapshotFilePath,
		DeltaIndex:    deltaIndex,
		DeltaFilePath: deltaSnapshotFilePath,
	}, nil
}
