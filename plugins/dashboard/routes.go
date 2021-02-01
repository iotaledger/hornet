package dashboard

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/websockethub"

	"github.com/gohornet/hornet/plugins/restapi"
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
	appBox = packr.New("Dashboard_App", "./frontend/build")
)

func indexRoute(e echo.Context) error {
	if deps.NodeConfig.Bool(CfgDashboardDevMode) {
		res, err := http.Get("http://127.0.0.1:9090/")
		if err != nil {
			return err
		}
		defer res.Body.Close()

		devIndexHTML, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}
		return e.HTMLBlob(http.StatusOK, devIndexHTML)
	}

	indexHTML, err := appBox.Find("index.html")
	if err != nil {
		return err
	}

	return e.HTMLBlob(http.StatusOK, indexHTML)
}

func readDataFromURL(url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return ioutil.ReadAll(res.Body)
}

func passThroughRoute(e echo.Context) error {
	contentType := calculateMimeType(e)

	if deps.NodeConfig.Bool(CfgDashboardDevMode) {
		data, err := readDataFromURL("http://127.0.0.1:9090" + e.Request().URL.Path)
		if err != nil {
			return err
		}
		return e.Blob(http.StatusOK, contentType, data)
	}
	staticBlob, err := appBox.Find(e.Request().URL.Path)
	if err != nil {
		return err
	}
	return e.Blob(http.StatusOK, contentType, staticBlob)
}

func passThroughAPIRoute(e echo.Context) error {
	apiBindAddr := deps.NodeConfig.String(restapi.CfgRestAPIBindAddress)

	data, err := readDataFromURL("http://" + apiBindAddr + e.Request().URL.Path + "?" + e.Request().URL.RawQuery)
	if err != nil {
		return err
	}
	return e.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, data)
}

func passThroughSpammerRoute(e echo.Context) error {
	apiBindAddr := deps.NodeConfig.String(restapi.CfgRestAPIBindAddress)

	data, err := readDataFromURL("http://" + apiBindAddr + "/spammer?" + e.Request().URL.RawQuery)
	if err != nil {
		return err
	}

	return e.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, data)
}

func calculateMimeType(e echo.Context) string {
	url := e.Request().URL.String()

	switch {
	case strings.HasSuffix(url, ".html"):
		return echo.MIMETextHTMLCharsetUTF8
	case strings.HasSuffix(url, ".css"):
		return "text/css"
	case strings.HasSuffix(url, ".js"):
		return echo.MIMEApplicationJavaScript
	case strings.HasSuffix(url, ".json"):
		return echo.MIMEApplicationJSONCharsetUTF8
	case strings.HasSuffix(url, ".png"):
		return "image/png"
	case strings.HasSuffix(url, ".svg"):
		return "image/svg+xml"
	default:
		return echo.MIMEOctetStream
	}
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

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	e.GET("/static/*", passThroughRoute)
	e.GET("/branding/*", passThroughRoute)
	e.GET("/favicon/*", passThroughRoute)

	// Hot reload code
	e.GET("/main*.js", passThroughRoute)

	// Pass all the explorer request through to the local rest API
	e.GET("/api/*", passThroughAPIRoute)
	e.GET("/plugins/spammer*", passThroughSpammerRoute)

	// Everything else fallback to index for routing.
	e.GET("*", indexRoute)

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
		case MsgTypeSyncStatus:
			client.Send(&Msg{Type: MsgTypeSyncStatus, Data: currentSyncStatus()})

		case MsgTypeNodeStatus:
			client.Send(&Msg{Type: MsgTypeNodeStatus, Data: currentNodeStatus()})

		case MsgTypeConfirmedMsMetrics:
			client.Send(&Msg{Type: MsgTypeConfirmedMsMetrics, Data: cachedMilestoneMetrics})

		case MsgTypeDatabaseSizeMetric:
			client.Send(&Msg{Type: MsgTypeDatabaseSizeMetric, Data: cachedDbSizeMetrics})

		case MsgTypeDatabaseCleanupEvent:
			client.Send(&Msg{Type: MsgTypeDatabaseCleanupEvent, Data: lastDbCleanup})

		case MsgTypeMs:
			start := deps.Storage.GetLatestMilestoneIndex()
			for i := start - 10; i <= start; i++ {
				if milestoneMessageID := getMilestoneMessageID(i); milestoneMessageID != nil {
					client.Send(&Msg{Type: MsgTypeMs, Data: &LivefeedMilestone{MessageID: milestoneMessageID.ToHex(), Index: i}})
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
				msg, ok := data.(*Msg)
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
