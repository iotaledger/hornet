package webapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	cnet "github.com/projectcalico/libcalico-go/lib/net"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/tangle"
	"github.com/gohornet/hornet/packages/shutdown"
)

// PLUGIN WebAPI
var (
	PLUGIN = node.NewPlugin("WebAPI", node.Enabled, configure, run)
	log    *logger.Logger

	server               *http.Server
	permitedEndpoints    = make(map[string]string)
	whitelistedNetworks  []net.IPNet
	implementedAPIcalls  = make(map[string]apiEndpoint)
	features             []string
	api                  *gin.Engine
	webAPIBase           = ""
	maxDepth             int
	serverShutdownSignal <-chan struct{}
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

	maxDepth = config.NodeConfig.GetInt(config.CfgTipSelMaxDepth)

	// Release mode
	gin.SetMode(gin.ReleaseMode)
	api = gin.New()
	// Recover from any panics and write a 500 if there was one
	api.Use(gin.Recovery())

	// CORS
	corsMiddleware := func(c *gin.Context) {

		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "User-Agent, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Origin, Cache-Control, X-Requested-With, X-IOTA-API-Version")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
	api.Use(corsMiddleware)

	// GZIP
	api.Use(gzip.Gzip(gzip.DefaultCompression))

	// Load allowed remote access to specific HTTP API commands
	pae := config.NodeConfig.GetStringSlice(config.CfgWebAPIPermitRemoteAccess)
	if len(pae) > 0 {
		for _, endpoint := range pae {
			ep := strings.ToLower(endpoint)
			permitedEndpoints[ep] = ep
		}
	}

	// load whitelisted addresses
	whitelist := append([]string{"127.0.0.1", "::1"}, config.NodeConfig.GetStringSlice(config.CfgWebAPIWhitelistedAddresses)...)
	for _, entry := range whitelist {
		_, ipnet, err := cnet.ParseCIDROrIP(entry)
		if err != nil {
			log.Warnf("Invalid whitelist address: %s", entry)
			continue
		}
		whitelistedNetworks = append(whitelistedNetworks, ipnet.IPNet)
	}

	// set basic auth if enabled
	if config.NodeConfig.GetBool(config.CfgWebAPIBasicAuthEnabled) {
		username := config.NodeConfig.GetString(config.CfgWebAPIBasicAuthUsername)
		password := config.NodeConfig.GetString(config.CfgWebAPIBasicAuthPassword)
		api.Use(gin.BasicAuth(gin.Accounts{username: password}))
	}

	// WebAPI route
	webAPIRoute()

	// return error, if route is not there
	api.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
	})
}

func run(plugin *node.Plugin) {
	log.Info("Starting WebAPI server ...")

	// Check for features
	if _, ok := permitedEndpoints["attachtotangle"]; ok {
		features = append(features, "RemotePOW")
	}

	if tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
		features = append(features, "WereAddressesSpentFrom")
	}

	daemon.BackgroundWorker("WebAPI server", func(shutdownSignal <-chan struct{}) {
		serverShutdownSignal = shutdownSignal

		log.Info("Starting WebAPI server ... done")

		bindAddr := config.NodeConfig.GetString(config.CfgWebAPIBindAddress)
		server = &http.Server{Addr: bindAddr, Handler: api}

		go func() {
			log.Infof("You can now access the API using: http://%s", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("Stopping WebAPI server due to an error ... done")
			}
		}()

		<-shutdownSignal
		log.Info("Stopping WebAPI server ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := server.Shutdown(ctx)
			if err != nil {
				log.Error(err.Error())
			}
			cancel()
		}
		log.Info("Stopping WebAPI server ... done")
	}, shutdown.ShutdownPriorityAPI)
}
