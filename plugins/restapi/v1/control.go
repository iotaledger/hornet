package v1

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/restapi"
)

func pruneDatabase(c echo.Context) (*pruneDatabaseResponse, error) {

	var index int
	var depth int

	indexStr := strings.ToLower(c.QueryParam("index"))
	depthStr := strings.ToLower(c.QueryParam("depth"))

	var err error
	if indexStr != "" {
		index, err = strconv.Atoi(indexStr)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing index failed: %s", err)
		}
	}

	if depthStr != "" {
		depth, err = strconv.Atoi(depthStr)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing depth failed: %s", err)
		}
	}

	if (depth != 0 && index != 0) || (depth == 0 && index == 0) {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "either depth or index has to be specified")
	}

	var targetIndex milestone.Index
	if depth != 0 {
		targetIndex, err = deps.Snapshot.PruneDatabaseByDepth(milestone.Index(depth))
		if err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "pruning database failed: %s", err)
		}
	} else {
		targetIndex, err = deps.Snapshot.PruneDatabaseByTargetIndex(milestone.Index(index))
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
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "either fullIndex or deltaIndex has to be specified")
	}

	var fullIndex, deltaIndex milestone.Index
	var fullSnapshotFilePath, deltaSnapshotFilePath string

	if request.FullIndex != nil {
		fullIndex = milestone.Index(*request.FullIndex)
		fullSnapshotFilePath = filepath.Join(filepath.Dir(deps.NodeConfig.String(snapshot.CfgSnapshotsFullPath)), fmt.Sprintf("full_snapshot_%d.bin", fullIndex))

		// ToDo: abort signal?
		if err := deps.Snapshot.CreateFullSnapshot(fullIndex, fullSnapshotFilePath, false, nil); err != nil {
			return nil, errors.WithMessagef(echo.ErrInternalServerError, "creating full snapshot failed: %s", err)
		}
	}

	if request.DeltaIndex != nil {
		deltaIndex = milestone.Index(*request.DeltaIndex)
		deltaSnapshotFilePath = filepath.Join(filepath.Dir(deps.NodeConfig.String(snapshot.CfgSnapshotsDeltaPath)), fmt.Sprintf("delta_snapshot_%d.bin", deltaIndex))

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
