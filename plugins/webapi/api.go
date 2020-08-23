package webapi

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/plugins/spammer"
	"github.com/gohornet/hornet/plugins/tangle"
)

const (
	spammerRoute = "spammer"
	healthzRoute = "healthz"
)

var (
	// ErrNodeNotSync is returned when the node was not synced.
	ErrNodeNotSync = errors.New("node not synced")
	// ErrInternalError is returned when there was an internal node error.
	ErrInternalError = errors.New("internal error")
)

func webAPIRoute() {
	api.POST(webAPIBase, func(c *gin.Context) {

		request := make(map[string]interface{})

		err := c.ShouldBindJSON(&request)
		if err != nil {
			fmt.Println(err)
		}

		// Get the command and check if it's implemented
		originCommand := fmt.Sprint(request["command"])
		cmd := strings.ToLower(originCommand)

		implementation, apiCallExists := implementedAPIcalls[cmd]

		whitelisted := false
		remoteHost, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		remoteAddress := net.ParseIP(remoteHost)
		for _, whitelistedNet := range whitelistedNetworks {
			if whitelistedNet.Contains(remoteAddress) {
				whitelisted = true
				break
			}
		}

		if !whitelisted {
			// Check if command is permitted. If it's not permited and the request does not come from localhost, deny it.
			_, permited := permitedEndpoints[cmd]
			if apiCallExists && !permited {
				e := ErrorReturn{
					Error: fmt.Sprintf("Command [%v] is protected", originCommand),
				}
				c.JSON(http.StatusForbidden, e)
				return
			}
		}

		if !apiCallExists {
			e := ErrorReturn{
				Error: fmt.Sprintf("Command [%v] is unknown", originCommand),
			}
			c.JSON(http.StatusBadRequest, e)
			return
		}

		implementation(&request, c, serverShutdownSignal)
	})
}

// health check
func restHealthzRoute() {

	if config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// autopeering entry node mode
		// GET /healthz
		api.GET(healthzRoute, func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		return
	}

	// node mode
	// GET /healthz
	api.GET(healthzRoute, func(c *gin.Context) {
		if !tangle.IsNodeHealthy() {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		c.Status(http.StatusOK)
	})
}

// spammer commands
func restSpammerRoute() {

	// GET /spammer
	api.GET(spammerRoute, func(c *gin.Context) {

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
