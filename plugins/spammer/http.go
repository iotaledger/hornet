package spammer

import (
	"runtime"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/restapi"
)

const (
	// RouteSpammer is the route for controlling the integrated spammer.
	// GET returns the tips.
	// query parameters: "cmd" (start, stop)
	//					 "mpsRateLimit" (optional)
	//					 "cpuMaxUsage" (optional)
	RouteSpammer = "/api/plugins/spammer"
)

type spamSettings struct {
	Running           bool    `json:"running"`
	MpsRateLimit      float64 `json:"mpsRateLimit"`
	CpuMaxUsage       float64 `json:"cpuMaxUsage"`
	SpammerWorkers    int     `json:"spammerWorkers"`
	SpammerWorkersMax int     `json:"spammerWorkersMax"`
}

func handleSpammerCommand(c echo.Context) (*spamSettings, error) {

	command := strings.ToLower(c.QueryParam("cmd"))

	switch command {

	case "start":
		var err error
		var mpsRateLimit *float64 = nil
		var cpuMaxUsage *float64 = nil
		var spammerWorkers *int = nil

		mpsRateLimitQuery := c.QueryParam("mpsRateLimit")
		if mpsRateLimitQuery != "" {
			mpsRateLimitParsed, err := strconv.ParseFloat(mpsRateLimitQuery, 64)
			if err != nil || mpsRateLimitParsed < 0.0 {
				return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing mpsRateLimit failed: %s", err)
			}
			mpsRateLimit = &mpsRateLimitParsed
		}

		cpuMaxUsageQuery := c.QueryParam("cpuMaxUsage")
		if cpuMaxUsageQuery != "" {
			cpuMaxUsageParsed, err := strconv.ParseFloat(cpuMaxUsageQuery, 64)
			if err != nil || cpuMaxUsageParsed < 0.0 {
				return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing cpuMaxUsage failed: %s", err)
			}
			cpuMaxUsage = &cpuMaxUsageParsed
		}

		spammerWorkersQuery := c.QueryParam("spammerWorkers")
		if spammerWorkersQuery != "" {
			spammerWorkersParsed64, err := strconv.ParseInt(spammerWorkersQuery, 10, 32)
			spammerWorkersParsed := int(spammerWorkersParsed64)
			if err != nil {
				return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing spammerWorkers failed: %s", err)
			}
			if spammerWorkersParsed < 1 || spammerWorkersParsed >= runtime.NumCPU() {
				return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "parsing spammerWorkers failed: out of range")
			}
			spammerWorkers = &spammerWorkersParsed
		}

		err = start(mpsRateLimit, cpuMaxUsage, spammerWorkers)
		if err != nil {
			return nil, errors.WithMessagef(restapi.ErrInternalError, "starting spammer failed: %s", err)
		}

	case "stop":
		if err := stop(); err != nil {
			return nil, errors.WithMessagef(restapi.ErrInternalError, "stopping spammer failed: %s", err)
		}

	case "settings":
		// Nothing to do here, will fallthrough to returning settings

	case "":
		return nil, errors.WithMessage(restapi.ErrInvalidParameter, "no cmd given")

	default:
		return nil, errors.WithMessagef(restapi.ErrInvalidParameter, "unknown cmd: %s", command)
	}

	return &spamSettings{
		Running:           isRunning,
		MpsRateLimit:      mpsRateLimitRunning,
		CpuMaxUsage:       cpuMaxUsageRunning,
		SpammerWorkers:    spammerWorkersRunning,
		SpammerWorkersMax: runtime.NumCPU() - 1,
	}, nil
}
