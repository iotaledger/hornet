package webapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	daemon "github.com/iotaledger/hive.go/daemon/ordered"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/parameter"

	"github.com/gohornet/hornet/packages/node"
	"github.com/gohornet/hornet/packages/shutdown"
)

// PLUGIN WebAPI
var (
	PLUGIN = node.NewPlugin("WebAPI", node.Enabled, configure, run)
	log    *logger.Logger

	server              *http.Server
	permitedEndpoints   = make(map[string]string)
	implementedAPIcalls = make(map[string]apiEndpoint)
	features            []string
	api                 *gin.Engine
	webAPIBase          = ""
	auth                string
	maxDepth            int
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger("WebAPI", logger.LogLevel(parameter.NodeConfig.GetInt("node.logLevel")))

	maxDepth = parameter.NodeConfig.GetInt("tipsel.maxDepth")

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

	// Load allowed remote access to specific api commands
	pae := parameter.NodeConfig.GetStringSlice("api.permitRemoteAccess")
	if len(pae) > 0 {
		for _, endpoint := range pae {
			ep := strings.ToLower(endpoint)
			permitedEndpoints[ep] = ep
		}
	}

	// Check for features
	if _, ok := permitedEndpoints["attachtotangle"]; ok {
		features = append(features, "RemotePOW")
	}

	// Set basic auth if enabled
	auth = parameter.NodeConfig.GetString("api.remoteauth")

	if len(auth) > 0 {
		authSlice := strings.Split(auth, ":")
		api.Use(gin.BasicAuth(gin.Accounts{authSlice[0]: authSlice[1]}))
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

	daemon.BackgroundWorker("WebAPI server", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting WebAPI server ... done")

		serveAddress := fmt.Sprintf("%s:%d", parameter.NodeConfig.GetString("api.host"), parameter.NodeConfig.GetInt("api.port"))

		server = &http.Server{
			Addr:    serveAddress,
			Handler: api,
		}

		go func() {
			log.Infof("You can now access the API using: http://%s", serveAddress)
			err := server.ListenAndServe()
			if err != nil {
				if err == http.ErrServerClosed {
					log.Info("Stopping WebAPI server ... done")
				} else {
					log.Error("Stopping WebAPI server due to an error ... done")
				}
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
