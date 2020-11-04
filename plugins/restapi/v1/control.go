package v1

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/core/snapshot"
	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/plugins/restapi/common"
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
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "parsing index failed: %w", err)
		}
	}

	if depthStr != "" {
		depth, err = strconv.Atoi(depthStr)
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInvalidParameter, "parsing depth failed: %w", err)
		}
	}

	if (depth != 0 && index != 0) || (depth == 0 && index == 0) {
		return nil, errors.WithMessage(common.ErrInvalidParameter, "either depth or index has to be specified")
	}

	var targetIndex milestone.Index
	if depth != 0 {
		targetIndex, err = snapshot.PruneDatabaseByDepth(milestone.Index(depth))
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInternalError, "pruning database failed: %w", err)
		}
	} else {
		targetIndex, err = snapshot.PruneDatabaseByTargetIndex(milestone.Index(index))
		if err != nil {
			return nil, errors.WithMessagef(common.ErrInternalError, "pruning database failed: %w", err)
		}
	}

	return &pruneDatabaseResponse{
		Index: targetIndex,
	}, nil
}

func createSnapshot(c echo.Context) (*createSnapshotResponse, error) {

	indexStr := strings.ToLower(c.QueryParam("index"))
	if indexStr == "" {
		return nil, errors.WithMessage(common.ErrInvalidParameter, "no index given")
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return nil, errors.WithMessagef(common.ErrInvalidParameter, "parsing index failed: %w", err)
	}

	snapshotFilePath := filepath.Join(filepath.Dir(deps.NodeConfig.String(config.CfgSnapshotsPath)), fmt.Sprintf("export_%d.bin", index))

	// ToDo: abort signal?
	if err := snapshot.CreateLocalSnapshot(milestone.Index(index), snapshotFilePath, false, nil); err != nil {
		return nil, errors.WithMessagef(common.ErrInternalError, "creating snapshot failed: %w", err)
	}

	return &createSnapshotResponse{
		Index:    milestone.Index(index),
		FilePath: snapshotFilePath,
	}, nil
}
