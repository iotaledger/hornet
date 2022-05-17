package spammer

import (
	"net/http"
	"runtime"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/pkg/restapi"
)

const (
	// RouteSpammerStatus is the route to get the status of the spammer.
	// GET the current status of the spammer.
	RouteSpammerStatus = "/status"

	// RouteSpammerStart is the route to start the spammer (with optional changing the settings).
	// POST the settings to change and start the spammer.
	RouteSpammerStart = "/start"

	// RouteSpammerStop is the route to stop the spammer.
	// POST to stop the spammer.
	RouteSpammerStop = "/stop"
)

type spammerStatus struct {
	Running           bool    `json:"running"`
	BpsRateLimit      float64 `json:"bpsRateLimit"`
	CPUMaxUsage       float64 `json:"cpuMaxUsage"`
	SpammerWorkers    int     `json:"spammerWorkers"`
	SpammerWorkersMax int     `json:"spammerWorkersMax"`
}

type startCommand struct {
	BpsRateLimit   *float64 `json:"bpsRateLimit,omitempty"`
	CPUMaxUsage    *float64 `json:"cpuMaxUsage,omitempty"`
	SpammerWorkers *int     `json:"spammerWorkers,omitempty"`
}

func setupRoutes(g *echo.Group) {

	g.GET(RouteSpammerStatus, func(c echo.Context) error {
		return restapi.JSONResponse(c, http.StatusOK, &spammerStatus{
			Running:           isRunning,
			BpsRateLimit:      bpsRateLimitRunning,
			CPUMaxUsage:       cpuMaxUsageRunning,
			SpammerWorkers:    spammerWorkersRunning,
			SpammerWorkersMax: runtime.NumCPU() - 1,
		})
	})

	g.POST(RouteSpammerStart, func(c echo.Context) error {
		cmd := &startCommand{}
		if err := c.Bind(&cmd); err != nil {
			return err
		}

		if err := start(cmd.BpsRateLimit, cmd.CPUMaxUsage, cmd.SpammerWorkers); err != nil {
			return err
		}
		return c.JSON(http.StatusAccepted, nil)
	})

	g.POST(RouteSpammerStop, func(c echo.Context) error {
		if err := stop(); err != nil {
			return err
		}
		return c.JSON(http.StatusAccepted, nil)
	})
}
