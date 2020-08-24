package dashboard

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/websockethub"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/pkg/config"
	"github.com/gohornet/hornet/pkg/model/tangle"
)

const (
	WebsocketCmdRegister   = 0
	WebsocketCmdUnregister = 1
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = errors.New("invalid parameter")

	// ErrInternalError defines the internal error.
	ErrInternalError = errors.New("internal error")

	// ErrNotFound defines the not found error.
	ErrNotFound = errors.New("not found")

	// ErrForbidden defines the forbidden error.
	ErrForbidden = errors.New("forbidden")

	// holds dashboard assets
	appBox    = packr.New("Dashboard_App", "./frontend/build")
	assetsBox = packr.New("Dashboard_Assets", "./frontend/src/assets")
)

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
		e.Static("/assets", "./plugins/dashboard/frontend/src/assets")
	} else {
		// load assets from packr: either from within the binary or actual disk
		e.GET("/app/*", echo.WrapHandler(http.StripPrefix("/app", http.FileServer(appBox))))
		e.GET("/assets/*", echo.WrapHandler(http.StripPrefix("/assets", http.FileServer(assetsBox))))
	}

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	// used to route into the dashboard index
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

func websocketRoute(ctx echo.Context) error {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("recovered from panic within WS handle func: %s", r)
		}
	}()

	// this function sends the initial values for some topics
	sendInitValue := func(client *websockethub.Client, initValuesSent map[byte]struct{}, topic byte) {
		if _, sent := initValuesSent[topic]; sent {
			return
		}
		initValuesSent[topic] = struct{}{}

		switch topic {
		case MsgTypeNodeStatus:
			client.Send(&msg{MsgTypeNodeStatus, currentNodeStatus()})

		case MsgTypeConfirmedMsMetrics:
			client.Send(&msg{MsgTypeConfirmedMsMetrics, cachedMilestoneMetrics})

		case MsgTypeDatabaseSizeMetric:
			client.Send(&msg{MsgTypeDatabaseSizeMetric, cachedDbSizeMetrics})

		case MsgTypeDatabaseCleanupEvent:
			client.Send(&msg{MsgTypeDatabaseCleanupEvent, lastDbCleanup})

		case MsgTypeMs:
			start := tangle.GetLatestMilestoneIndex()
			for i := start - 10; i <= start; i++ {
				if msTailTxHash := getMilestoneTailHash(i); msTailTxHash != nil {
					client.Send(&msg{MsgTypeMs, &ms{msTailTxHash.Trytes(), i}})
				} else {
					break
				}
			}
		}
	}

	topicsLock := syncutils.RWMutex{}
	registeredTopics := make(map[byte]struct{})
	initValuesSent := make(map[byte]struct{})

	hub.ServeWebsocket(ctx.Response(), ctx.Request(),
		// onCreate gets called when the client is created
		func(client *websockethub.Client) {
			client.FilterCallback = func(c *websockethub.Client, data interface{}) bool {
				msg, ok := data.(*msg)
				if !ok {
					return false
				}

				topicsLock.RLock()
				_, registered := registeredTopics[msg.Type]
				topicsLock.RUnlock()
				return registered
			}
			client.ReceiveChan = make(chan *websockethub.WebsocketMsg, 100)

			go func() {
				for {
					select {
					case <-client.ExitSignal:
						// client was disconnected
						return

					case msg, ok := <-client.ReceiveChan:
						if !ok {
							// client was disconnected
							return
						}

						if msg.MsgType == websockethub.BinaryMessage {
							if len(msg.Data) < 2 {
								continue
							}

							cmd := msg.Data[0]
							topic := msg.Data[1]

							if cmd == WebsocketCmdRegister {
								// register topic fo this client
								topicsLock.Lock()
								registeredTopics[topic] = struct{}{}
								topicsLock.Unlock()

								sendInitValue(client, initValuesSent, topic)

							} else if cmd == WebsocketCmdUnregister {
								// unregister topic fo this client
								topicsLock.Lock()
								delete(registeredTopics, topic)
								topicsLock.Unlock()
							}
						}
					}
				}
			}()
		},

		// onConnect gets called when the client was registered
		func(client *websockethub.Client) {
			log.Info("WebSocket client connection established")
		})

	return nil
}
