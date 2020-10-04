package v1

import (
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/hornet"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/plugins/cli"
	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/spammer"
	tangleplugin "github.com/gohornet/hornet/plugins/tangle"
	"github.com/gohornet/hornet/plugins/urts"
)

const (
	waitForNodeSyncedTimeout = 2000 * time.Millisecond
)

var (
	// The route for the info REST API call.
	NodeAPIRouteInfo = "/info"
	// The route for the tips REST API call.
	NodeAPIRouteTips = "/tips"


	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
)

var (
	features = []string{} // Workaround until https://github.com/golang/go/issues/27589 is fixed
)


func SetupApiRoutesV1(routeGroup *echo.Group) {

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// Check for features
		if config.NodeConfig.GetBool(config.CfgNodeEnableProofOfWork) {
			features = append(features, "PoW")
		}
	}

	// only handle spammer api calls if the spammer plugin is enabled
	if !node.IsSkipped(spammer.PLUGIN) {
		//setupSpammerRoute(routeGroup)
	}

	routeGroup.GET(NodeAPIRouteInfo, func(c echo.Context) error {
		info, err := info()
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, info)
	})

	// only handle tips api calls if the URTS plugin is enabled
	if !node.IsSkipped(urts.PLUGIN) {
		routeGroup.GET(NodeAPIRouteTips, func(c echo.Context) error {
			tips, err := tips()
			if err != nil {
				return err
			}
			return c.JSON(http.StatusOK, tips)
		})
	}
}

func info() (*infoResponse, error) {

	// latest milestone index
	latestMilestoneMessageID := hornet.GetNullMessageID().Hex()
	latestMilestoneIndex := tangle.GetLatestMilestoneIndex()

	// latest milestone message ID
	cachedLatestMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(latestMilestoneIndex)
	if cachedLatestMilestoneMsg != nil {
		latestMilestoneMessageID = cachedLatestMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedLatestMilestoneMsg.Release(true)
	}

	// solid milestone index
	solidMilestoneMessageID := hornet.GetNullMessageID().Hex()
	solidMilestoneIndex := tangle.GetSolidMilestoneIndex()

	// solid milestone message ID
	cachedSolidMilestoneMsg := tangle.GetMilestoneCachedMessageOrNil(solidMilestoneIndex)
	if cachedSolidMilestoneMsg != nil {
		solidMilestoneMessageID = cachedSolidMilestoneMsg.GetMessage().GetMessageID().Hex()
		cachedSolidMilestoneMsg.Release(true)
	}

	// pruning index
	var pruningIndex milestone.Index
	snapshotInfo := tangle.GetSnapshotInfo()
	if snapshotInfo != nil {
		pruningIndex = snapshotInfo.PruningIndex
	}

	return &infoResponse{
		Name:                          cli.AppName,
		Version:                       cli.AppVersion,
		IsHealthy:                     tangleplugin.IsNodeHealthy(),
		IsSynced:                      tangle.IsNodeSyncedWithThreshold(),
		CoordinatorPublicKey:          config.NodeConfig.GetString(config.CfgCoordinatorPublicKey),
		LatestMilestoneMessageID:      latestMilestoneMessageID,
		LatestMilestoneIndex:          uint64(latestMilestoneIndex),
		LatestSolidMilestoneMessageID: solidMilestoneMessageID,
		LatestSolidMilestoneIndex:     uint64(solidMilestoneIndex),
		PruningIndex:                  uint64(pruningIndex),
		Features:                      features,
	}, nil
}

func tips() (*tipsResponse, error) {

	tips, err := urts.TipSelector.SelectNonLazyTips()
	if err != nil {
		if err == tangle.ErrNodeNotSynced || err == tipselect.ErrNoTipsAvailable {
			return nil, errors.Wrap(common.ErrServiceUnavailable, err.Error())
		}
		return nil, errors.Wrap(common.ErrInternalError, err.Error())
	}

	return &tipsResponse{Tip1: tips[0].Hex(), Tip2: tips[1].Hex()}, nil
}
