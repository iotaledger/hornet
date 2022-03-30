package faucet

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"go.uber.org/dig"
	"golang.org/x/time/rate"

	"github.com/gohornet/hornet/pkg/common"
	"github.com/gohornet/hornet/pkg/model/faucet"
	"github.com/gohornet/hornet/pkg/model/storage"
	"github.com/gohornet/hornet/pkg/model/syncmanager"
	"github.com/gohornet/hornet/pkg/model/utxo"
	"github.com/gohornet/hornet/pkg/node"
	"github.com/gohornet/hornet/pkg/pow"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
	restapipkg "github.com/gohornet/hornet/pkg/restapi"
	"github.com/gohornet/hornet/pkg/shutdown"
	"github.com/gohornet/hornet/pkg/tangle"
	"github.com/gohornet/hornet/pkg/tipselect"
	"github.com/gohornet/hornet/pkg/utils"
	"github.com/gohornet/hornet/pkg/whiteflag"
	"github.com/gohornet/hornet/plugins/restapi"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/events"
	iotago "github.com/iotaledger/iota.go/v3"
)

const (

	// RouteFaucetInfo is the route to give info about the faucet address.
	// GET returns address and balance of the faucet.
	RouteFaucetInfo = "/info"

	// RouteFaucetEnqueue is the route to tell the faucet to pay out some funds to the given address.
	// POST enqueues a new request.
	RouteFaucetEnqueue = "/enqueue"
)

func init() {
	Plugin = &node.Plugin{
		Status: node.StatusDisabled,
		Pluggable: node.Pluggable{
			Name:      "Faucet",
			DepsFunc:  func(cDeps dependencies) { deps = cDeps },
			Params:    params,
			Provide:   provide,
			Configure: configure,
			Run:       run,
		},
	}
}

var (
	Plugin *node.Plugin
	deps   dependencies

	// closures
	onMilestoneConfirmed *events.Closure
)

type dependencies struct {
	dig.In
	NodeConfig            *configuration.Configuration `name:"nodeConfig"`
	RestAPIBindAddress    string                       `name:"restAPIBindAddress"`
	FaucetAllowedAPIRoute restapipkg.AllowedRoute      `name:"faucetAllowedAPIRoute"`
	Faucet                *faucet.Faucet
	Tangle                *tangle.Tangle
	ShutdownHandler       *shutdown.ShutdownHandler
	RestPluginManager     *restapi.RestPluginManager `optional:"true"`
}

func provide(c *dig.Container) {

	privateKeys, err := utils.LoadEd25519PrivateKeysFromEnvironment("FAUCET_PRV_KEY")
	if err != nil {
		Plugin.LogPanicf("loading faucet private key failed, err: %s", err)
	}

	if len(privateKeys) == 0 {
		Plugin.LogPanic("loading faucet private key failed, err: no private keys given")
	}

	if len(privateKeys) > 1 {
		Plugin.LogPanic("loading faucet private key failed, err: too many private keys given")
	}

	privateKey := privateKeys[0]
	if len(privateKey) != ed25519.PrivateKeySize {
		Plugin.LogPanic("loading faucet private key failed, err: wrong private key length")
	}

	faucetAddress := iotago.Ed25519AddressFromPubKey(privateKey.Public().(ed25519.PublicKey))
	faucetSigner := iotago.NewInMemoryAddressSigner(iotago.NewAddressKeysForEd25519Address(&faucetAddress, privateKey))

	type faucetDeps struct {
		dig.In
		Storage                   *storage.Storage
		SyncManager               *syncmanager.SyncManager
		PowHandler                *pow.Handler
		UTXOManager               *utxo.Manager
		NodeConfig                *configuration.Configuration `name:"nodeConfig"`
		NetworkID                 uint64                       `name:"networkId"`
		DeSerializationParameters *iotago.DeSerializationParameters
		BelowMaxDepth             int                  `name:"belowMaxDepth"`
		Bech32HRP                 iotago.NetworkPrefix `name:"bech32HRP"`
		TipSelector               *tipselect.TipSelector
		MessageProcessor          *gossip.MessageProcessor
	}

	if err := c.Provide(func(deps faucetDeps) *faucet.Faucet {
		return faucet.New(
			Plugin.Daemon(),
			deps.Storage,
			deps.SyncManager,
			deps.NetworkID,
			deps.DeSerializationParameters,
			deps.BelowMaxDepth,
			deps.UTXOManager,
			&faucetAddress,
			faucetSigner,
			deps.TipSelector.SelectNonLazyTips,
			deps.PowHandler,
			deps.MessageProcessor.Emit,
			faucet.WithLogger(Plugin.Logger()),
			faucet.WithHRPNetworkPrefix(deps.Bech32HRP),
			faucet.WithAmount(uint64(deps.NodeConfig.Int64(CfgFaucetAmount))),
			faucet.WithSmallAmount(uint64(deps.NodeConfig.Int64(CfgFaucetSmallAmount))),
			faucet.WithMaxAddressBalance(uint64(deps.NodeConfig.Int64(CfgFaucetMaxAddressBalance))),
			faucet.WithMaxOutputCount(deps.NodeConfig.Int(CfgFaucetMaxOutputCount)),
			faucet.WithTagMessage(deps.NodeConfig.String(CfgFaucetTagMessage)),
			faucet.WithBatchTimeout(deps.NodeConfig.Duration(CfgFaucetBatchTimeout)),
			faucet.WithPowWorkerCount(deps.NodeConfig.Int(CfgFaucetPoWWorkerCount)),
		)
	}); err != nil {
		Plugin.LogPanic(err)
	}
}

func configure() {
	// check if RestAPI plugin is disabled
	if Plugin.Node.IsSkipped(restapi.Plugin) {
		Plugin.LogPanic("RestAPI plugin needs to be enabled to use the Faucet plugin")
	}

	routeGroup := deps.RestPluginManager.AddPlugin("faucet/v1")

	allowedRoutes := map[string][]string{
		http.MethodGet: {
			"/api/plugins/faucet/v1/info",
		},
	}

	rateLimiterSkipper := func(context echo.Context) bool {
		// Check for which route we will skip the rate limiter
		routesForMethod, exists := allowedRoutes[context.Request().Method]
		if !exists {
			return false
		}

		path := context.Request().URL.EscapedPath()
		for _, prefix := range routesForMethod {
			if strings.HasPrefix(path, prefix) {
				return true
			}
		}

		return false
	}

	rateLimiterConfig := middleware.RateLimiterConfig{
		Skipper: rateLimiterSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(1 / 300.0), // 1 request every 5 minutes
				Burst:     10,                    // additional burst of 10 requests
				ExpiresIn: 5 * time.Minute,
			},
		),
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			id := ctx.RealIP()
			return id, nil
		},
		ErrorHandler: func(context echo.Context, err error) error {
			return context.JSON(http.StatusForbidden, nil)
		},
		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(http.StatusTooManyRequests, nil)
		},
	}
	routeGroup.Use(middleware.RateLimiterWithConfig(rateLimiterConfig))

	routeGroup.GET(RouteFaucetInfo, func(c echo.Context) error {
		resp, err := getFaucetInfo(c)
		if err != nil {
			return err
		}

		return restapipkg.JSONResponse(c, http.StatusOK, resp)
	})

	routeGroup.POST(RouteFaucetEnqueue, func(c echo.Context) error {
		resp, err := addFaucetOutputToQueue(c)
		if err != nil {
			// own error handler to have nicer user facing error messages.
			var statusCode int
			var message string

			var e *echo.HTTPError
			if errors.As(err, &e) {
				statusCode = e.Code
				if errors.Is(err, restapipkg.ErrInvalidParameter) {
					message = strings.Replace(err.Error(), ": "+errors.Unwrap(err).Error(), "", 1)
				} else {
					message = err.Error()
				}
			} else {
				statusCode = http.StatusInternalServerError
				message = fmt.Sprintf("internal server error. error: %s", err.Error())
			}

			return c.JSON(statusCode, restapipkg.HTTPErrorResponseEnvelope{Error: restapipkg.HTTPErrorResponse{Code: strconv.Itoa(statusCode), Message: message}})
		}

		return restapipkg.JSONResponse(c, http.StatusAccepted, resp)
	})

	configureEvents()
}

func run() {
	// create a background worker that handles the enqueued faucet requests
	if err := Plugin.Daemon().BackgroundWorker("Faucet", func(ctx context.Context) {
		attachEvents()
		if err := deps.Faucet.RunFaucetLoop(ctx, nil); err != nil && common.IsCriticalError(err) != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("faucet plugin hit a critical error: %s", err.Error()))
		}
		detachEvents()
	}, shutdown.PriorityFaucet); err != nil {
		Plugin.LogPanicf("failed to start worker: %s", err)
	}

	websiteEnabled := deps.NodeConfig.Bool(CfgFaucetWebsiteEnabled)

	if websiteEnabled {
		bindAddr := deps.NodeConfig.String(CfgFaucetWebsiteBindAddress)

		e := echo.New()
		e.HideBanner = true
		e.Use(middleware.Recover())

		setupRoutes(e)

		go func() {
			Plugin.LogInfof("You can now access the faucet website using: http://%s", bindAddr)

			if err := e.Start(bindAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
				Plugin.LogWarnf("Stopped faucet website server due to an error (%s)", err)
			}
		}()
	}
}

func configureEvents() {
	onMilestoneConfirmed = events.NewClosure(func(confirmation *whiteflag.Confirmation) {
		if err := deps.Faucet.ApplyConfirmation(confirmation); err != nil && common.IsCriticalError(err) != nil {
			deps.ShutdownHandler.SelfShutdown(fmt.Sprintf("faucet plugin hit a critical error: %s", err.Error()))
		}
	})
}

func attachEvents() {
	deps.Tangle.Events.MilestoneConfirmed.Attach(onMilestoneConfirmed)
}

func detachEvents() {
	deps.Tangle.Events.MilestoneConfirmed.Detach(onMilestoneConfirmed)
}
