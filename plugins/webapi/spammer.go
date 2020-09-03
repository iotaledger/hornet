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
			var tpsRateLimit *float64 = nil
			var cpuMaxUsage *float64 = nil
			var bundleSize *int = nil
			var valueSpam *bool = nil

			tpsRateLimitQuery := c.Query("tpsRateLimit")
			if tpsRateLimitQuery != "" {
				tpsRateLimitParsed, err := strconv.ParseFloat(tpsRateLimitQuery, 64)
				if err != nil || tpsRateLimitParsed < 0.0 {
					c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("parsing tpsRateLimit failed: %w", err).Error()})
					return
				}
				tpsRateLimit = &tpsRateLimitParsed
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

			bundleSizeQuery := c.Query("bundleSize")
			if bundleSizeQuery != "" {
				bundleSizeParsed, err := strconv.Atoi(bundleSizeQuery)
				if err != nil || bundleSizeParsed < 1 {
					c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("parsing bundleSize failed: %w", err).Error()})
					return
				}
				bundleSize = &bundleSizeParsed
			}

			valueSpamQuery := c.Query("valueSpam")
			if valueSpamQuery != "" {
				valueSpamParsed, err := strconv.ParseBool(valueSpamQuery)
				if err != nil {
					c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("parsing valueSpam failed: %w", err).Error()})
					return
				}
				valueSpam = &valueSpamParsed
			}

			usedTpsRateLimit, usedCPUMaxUsage, usedBundleSize, usedValueSpam, err := spammer.Start(tpsRateLimit, cpuMaxUsage, bundleSize, valueSpam)
			if err != nil {
				c.JSON(http.StatusBadRequest, ErrorReturn{Error: fmt.Errorf("starting spammer failed: %w", err).Error()})
				return
			}

			c.JSON(http.StatusOK, ResultReturn{Message: fmt.Sprintf("started spamming (TPS Limit: %0.2f, CPU Limit: %0.2f%%, BundleSize: %d, ValueSpam: %t)", usedTpsRateLimit, usedCPUMaxUsage*100.0, usedBundleSize, usedValueSpam)})
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
