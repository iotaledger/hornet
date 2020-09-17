package webapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gohornet/hornet/plugins/spammer"
)

func spammerRoute() {
	api.GET("/spammer", func(c *gin.Context) {

		if !networkWhitelisted(c) {
			// network is not whitelisted, check if the route is permitted, otherwise deny it.
			if _, permitted := permittedRESTroutes["spammer"]; !permitted {
				c.JSON(http.StatusForbidden, ErrorReturn{Error: "route [spammer] is protected"})
				return
			}
		}

		switch strings.ToLower(c.Query("cmd")) {
		case "start":
			var err error
			var mpsRateLimit *float64 = nil
			var cpuMaxUsage *float64 = nil

			mpsRateLimitQuery := c.Query("mpsRateLimit")
			if mpsRateLimitQuery != "" {
				mpsRateLimitParsed, err := strconv.ParseFloat(mpsRateLimitQuery, 64)
				if err != nil || mpsRateLimitParsed < 0.0 {
					c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("parsing mpsRateLimit failed: %w", err).Error()})
					return
				}
				mpsRateLimit = &mpsRateLimitParsed
			}

			cpuMaxUsageQuery := c.Query("cpuMaxUsage")
			if cpuMaxUsageQuery != "" {
				cpuMaxUsageParsed, err := strconv.ParseFloat(cpuMaxUsageQuery, 64)
				if err != nil || cpuMaxUsageParsed < 0.0 {
					c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("parsing cpuMaxUsage failed: %w", err).Error()})
					return
				}
				cpuMaxUsage = &cpuMaxUsageParsed
			}

			usedMpsRateLimit, usedCPUMaxUsage, err := spammer.Start(mpsRateLimit, cpuMaxUsage)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("starting spammer failed: %w", err).Error()})
				return
			}

			c.JSON(http.StatusOK, ResultReturn{Message: fmt.Sprintf("started spamming (MPS Limit: %0.2f, CPU Limit: %0.2f%%)", usedMpsRateLimit, usedCPUMaxUsage*100.0)})
			return

		case "stop":
			if err := spammer.Stop(); err != nil {
				c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("stopping spammer failed: %w", err).Error()})
				return
			}
			c.JSON(http.StatusOK, ResultReturn{Message: "stopped spamming"})
			return

		case "":
			c.JSON(http.StatusBadRequest, ErrorReturn{Error: "no cmd given"})
			return

		default:
			c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Sprintf("unknown cmd: %s", strings.ToLower(c.Query("cmd")))})
			return
		}
	})
}
