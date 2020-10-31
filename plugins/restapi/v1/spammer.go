package v1

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/plugins/restapi/common"
	"github.com/gohornet/hornet/plugins/spammer"
)

func executeSpammerCommand(c echo.Context) (string, error) {

	command := strings.ToLower(c.QueryParam("cmd"))

	switch command {

	case "start":
		var err error
		var mpsRateLimit *float64 = nil
		var cpuMaxUsage *float64 = nil

		mpsRateLimitQuery := c.QueryParam("mpsRateLimit")
		if mpsRateLimitQuery != "" {
			mpsRateLimitParsed, err := strconv.ParseFloat(mpsRateLimitQuery, 64)
			if err != nil || mpsRateLimitParsed < 0.0 {
				return "", errors.WithMessagef(common.ErrInvalidParameter, "parsing mpsRateLimit failed: %w", err)
			}
			mpsRateLimit = &mpsRateLimitParsed
		}

		cpuMaxUsageQuery := c.QueryParam("cpuMaxUsage")
		if cpuMaxUsageQuery != "" {
			cpuMaxUsageParsed, err := strconv.ParseFloat(cpuMaxUsageQuery, 64)
			if err != nil || cpuMaxUsageParsed < 0.0 {
				return "", errors.WithMessagef(common.ErrInvalidParameter, "parsing cpuMaxUsage failed: %w", err)
			}
			cpuMaxUsage = &cpuMaxUsageParsed
		}

		usedMpsRateLimit, usedCPUMaxUsage, err := spammer.Start(mpsRateLimit, cpuMaxUsage)
		if err != nil {
			return "", errors.WithMessagef(common.ErrInternalError, "starting spammer failed: %w", err)
		}

		return fmt.Sprintf("started spamming (MPS Limit: %0.2f, CPU Limit: %0.2f%%)", usedMpsRateLimit, usedCPUMaxUsage*100.0), nil

	case "stop":
		if err := spammer.Stop(); err != nil {
			return "", errors.WithMessagef(common.ErrInternalError, "stopping spammer failed: %w", err)
		}
		return "stopped spamming", nil

	case "":
		return "", errors.WithMessage(common.ErrInvalidParameter, "no cmd given")

	default:
		return "", errors.WithMessagef(common.ErrInvalidParameter, "unknown cmd: %s", command)
	}
}
