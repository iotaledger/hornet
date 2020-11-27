package v1

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gohornet/hornet/pkg/restapi"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/pkg/model/milestone"
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
			return nil, errors.WithMessagef(restapi.ErrInternalError, "pruning database failed: %s", err)
		}
	} else {
		targetIndex, err = deps.Snapshot.PruneDatabaseByTargetIndex(milestone.Index(index))
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInternalError, "pruning database failed: %s", err)
		}
	}

	return &pruneDatabaseResponse{
		Index: targetIndex,
	}, nil
}

func createSnapshot(c echo.Context) (*createSnapshotResponse, error) {

	indexStr := strings.ToLower(c.QueryParam("index"))
	if indexStr == "" {
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "no index given")
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing index failed: %s", err)
	}

	snapshotFilePath := filepath.Join(filepath.Dir(deps.NodeConfig.String(snapshot.CfgSnapshotsFullPath)), fmt.Sprintf("export_%d.bin", index))

	// ToDo: abort signal?
	if err := deps.Snapshot.CreateFullSnapshot(milestone.Index(index), snapshotFilePath, false, nil); err != nil {
		return nil, errors.WithMessagef(restapi.ErrInternalError, "creating snapshot failed: %s", err)
	}

	return &createSnapshotResponse{
		Index:    milestone.Index(index),
		FilePath: snapshotFilePath,
	}, nil
}
