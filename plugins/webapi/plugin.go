package webapi

import (
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/gohornet/hornet/pkg/basicauth"
	"github.com/gohornet/hornet/plugins/spammer"
	cnet "github.com/projectcalico/libcalico-go/lib/net"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
	"github.com/gohornet/hornet/pkg/shutdown"
)

// PLUGIN WebAPI
var (
	PLUGIN = node.NewPlugin("WebAPI", node.Enabled, configure, run)
	log    *logger.Logger

	server               *http.Server
	permitedEndpoints    = make(map[string]struct{})
	permitedRESTroutes   = make(map[string]struct{})
	whitelistedNetworks  []net.IPNet
	implementedAPIcalls  = make(map[string]apiEndpoint)
	features             []string
	api                  *gin.Engine
	webAPIBase           = ""
	serverShutdownSignal <-chan struct{}
)

func configure(plugin *node.Plugin) {
	log = logger.NewLogger(plugin.Name)

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
	permittedAPIendpoints := config.NodeConfig.GetStringSlice(config.CfgWebAPIPermitRemoteAccess)
	if len(permittedAPIendpoints) > 0 {
		for _, endpoint := range permittedAPIendpoints {
			permitedEndpoints[strings.ToLower(endpoint)] = struct{}{}
		}
	}

	// Load allowed remote access to specific HTTP REST routes
	permittedRoutes := config.NodeConfig.GetStringSlice(config.CfgWebAPIPermittedRoutes)
	if len(permittedRoutes) > 0 {
		for _, route := range permittedRoutes {
			permitedRESTroutes[strings.ToLower(route)] = struct{}{}
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

	exclHealthCheckFromAuth := config.NodeConfig.GetBool(config.CfgWebAPIExcludeHealthCheckFromAuth)
	if exclHealthCheckFromAuth {
		// Handle route without auth
		healthzRoute()
	}

	// set basic auth if enabled
	// TODO: replace gin with echo so we don't have to write this middleware ourselves
	if config.NodeConfig.GetBool(config.CfgWebAPIBasicAuthEnabled) {
		const basicAuthPrefix = "Basic "
		expectedUsername := config.NodeConfig.GetString(config.CfgWebAPIBasicAuthUsername)
		expectedPasswordHash := config.NodeConfig.GetString(config.CfgWebAPIBasicAuthPasswordHash)
		passwordSalt := config.NodeConfig.GetString(config.CfgWebAPIBasicAuthPasswordSalt)

		if len(expectedUsername) == 0 {
			log.Fatalf("'%s' must not be empty if web API basic auth is enabled", config.CfgWebAPIBasicAuthUsername)
		}

		if len(expectedPasswordHash) != 64 {
			log.Fatalf("'%s' must be 64 (sha256 hash) in length if web API basic auth is enabled", config.CfgWebAPIBasicAuthPasswordHash)
		}

		unauthorizedReq := func(c *gin.Context) {
			c.Header("WWW-Authenticate", "Authorization Required")
			c.AbortWithStatus(http.StatusUnauthorized)
		}

		api.Use(func(c *gin.Context) {
			authVal := c.Request.Header.Get("Authorization")
			if len(authVal) <= len(basicAuthPrefix) {
				unauthorizedReq(c)
				return
			}

			base64EncodedUserPW := strings.TrimPrefix(authVal, basicAuthPrefix)
			userAndPWBytes, err := base64.StdEncoding.DecodeString(base64EncodedUserPW)
			if err != nil {
				unauthorizedReq(c)
				return
			}

			reqUsernameAndPW := string(userAndPWBytes)
			colonIndex := strings.Index(reqUsernameAndPW, ":")
			if colonIndex == -1 || colonIndex+1 >= len(reqUsernameAndPW) {
				unauthorizedReq(c)
				return
			}

			// username and password are split by a colon
			reqUsername := reqUsernameAndPW[:colonIndex]
			reqPasword := reqUsernameAndPW[colonIndex+1:]

			if reqUsername != expectedUsername || !basicauth.VerifyPassword(reqPasword, passwordSalt, expectedPasswordHash) {
				unauthorizedReq(c)
			}
		})
	}

	if !exclHealthCheckFromAuth {
		// Handle route with auth
		healthzRoute()
	}

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		webAPIRoute()

		// only handle spammer api calls if the spammer plugin is enabled
		if !node.IsSkipped(spammer.PLUGIN) {
			spammerRoute()
		}
	}

	// return error, if route is not there
	api.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}

func run(_ *node.Plugin) {
	log.Info("Starting WebAPI server ...")

	if !config.NodeConfig.GetBool(config.CfgNetAutopeeringRunAsEntryNode) {
		// Check for features
		if _, ok := permitedEndpoints["attachtotangle"]; ok {
			features = append(features, "RemotePOW")
		}

		if tangle.GetSnapshotInfo().IsSpentAddressesEnabled() {
			features = append(features, "WereAddressesSpentFrom")
		}
	}

	daemon.BackgroundWorker("WebAPI server", func(shutdownSignal <-chan struct{}) {
		serverShutdownSignal = shutdownSignal

		log.Info("Starting WebAPI server ... done")

		bindAddr := config.NodeConfig.GetString(config.CfgWebAPIBindAddress)
		server = &http.Server{Addr: bindAddr, Handler: api}

		go func() {
			log.Infof("You can now access the API using: http://%s", bindAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Warn("Stopping WebAPI server due to an error ... done")
			}
		}()

		<-shutdownSignal
		log.Info("Stopping WebAPI server ...")

		if server != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := server.Shutdown(ctx)
			if err != nil {
				log.Warn(err.Error())
			}
			cancel()
		}
		log.Info("Stopping WebAPI server ... done")
	}, shutdown.PriorityAPI)
}
