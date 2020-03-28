package spa

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/iotaledger/hive.go/websockethub"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/packages/config"
	"github.com/gohornet/hornet/packages/model/tangle"
)

var ErrInvalidParameter = errors.New("invalid config")
var ErrInternalError = errors.New("internal error")
var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")

// holds SPA assets
var appBox = packr.New("SPA_App", "./frontend/build")
var assetsBox = packr.New("SPA_Assets", "./frontend/src/assets")

func indexRoute(e echo.Context) error {
	if config.NodeConfig.GetBool(config.CfgDashboardDevMode) {
		res, err := http.Get("http://127.0.0.1:9090/")
		if err != nil {
			return err
		}
		devIndexHTML, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return e.HTMLBlob(http.StatusOK, devIndexHTML)
	}
	theme := config.NodeConfig.GetString(config.CfgDashboardTheme)
	indexHTML, err := appBox.Find("index.html")
	if theme == "light" {
		indexHTML, err = appBox.Find("index_light.html")
	}
	if err != nil {
		return err
	}
	return e.HTMLBlob(http.StatusOK, indexHTML)
}

func enforceMaxOneDotPerURL(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if strings.Count(c.Request().URL.Path, "..") != 0 {
			return c.String(http.StatusForbidden, "path not allowed")
		}
		return next(c)
	}
}

func setupRoutes(e *echo.Echo) {

	e.Pre(enforceMaxOneDotPerURL)

	if config.NodeConfig.GetBool(config.CfgDashboardDevMode) {
		e.Static("/assets", "./plugins/spa/frontend/src/assets")
	} else {
		// load assets from packr: either from within the binary or actual disk
		e.GET("/app/*", echo.WrapHandler(http.StripPrefix("/app", http.FileServer(appBox))))
		e.GET("/assets/*", echo.WrapHandler(http.StripPrefix("/assets", http.FileServer(assetsBox))))
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the SPA index
	e.GET("*", indexRoute)

	apiRoutes := e.Group("/api")

	setupExplorerRoutes(apiRoutes)

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		c.Logger().Error(err)

		var statusCode int
		var message string

		switch errors.Cause(err) {

		case echo.ErrNotFound:
			c.Redirect(http.StatusSeeOther, "/")
			return

		case echo.ErrUnauthorized:
			statusCode = http.StatusUnauthorized
			message = "unauthorized"

		case ErrForbidden:
			statusCode = http.StatusForbidden
			message = "access forbidden"

		case ErrInternalError:
			statusCode = http.StatusInternalServerError
			message = "internal server error"

		case ErrNotFound:
			statusCode = http.StatusNotFound
			message = "not found"

		case ErrInvalidParameter:
			statusCode = http.StatusBadRequest
			message = "bad request"

		default:
			statusCode = http.StatusInternalServerError
			message = "internal server error"
		}

		message = fmt.Sprintf("%s, error: %+v", message, err)
		c.String(statusCode, message)
	}
}

func websocketRoute(c echo.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered from panic within WS handle func: %s", r)
		}
	}()
	hub.ServeWebsocket(c.Response(), c.Request(), func(client *websockethub.Client) {
		log.Info("WebSocket client connection established")

		client.Send(&msg{MsgTypeNodeStatus, currentNodeStatus()})
		client.Send(&msg{MsgTypeConfirmedMsMetrics, cachedMilestoneMetrics})
		start := tangle.GetLatestMilestoneIndex()
		for i := start - 10; i <= start; i++ {
			if cachedMsTailTx := getMilestoneTail(i); cachedMsTailTx != nil { // tx +1
				client.Send(&msg{MsgTypeMs, &ms{cachedMsTailTx.GetTransaction().GetHash(), i}})
				cachedMsTailTx.Release(true) // tx -1
			} else {
				break
			}
		}
	})

	return nil
}
