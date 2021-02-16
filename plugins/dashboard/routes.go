package dashboard

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

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

func loginRoute(c echo.Context) error {

	type loginRequest struct {
		JWT      string `json:"jwt"`
		User     string `json:"user"`
		Password string `json:"password"`
	}

	request := &loginRequest{}

	if err := c.Bind(request); err != nil {
		return errors.WithMessagef(ErrInvalidParameter, "invalid request, error: %s", err)
	}

	if len(request.JWT) > 0 {
		// Verify JWT is still valid
		if !jwtAuth.VerifyJWT(request.JWT) {
			return echo.ErrUnauthorized
		}
	} else if !jwtAuth.VerifyUsernameAndPassword(request.User, request.Password) {
		return echo.ErrUnauthorized
	}

	t, err := jwtAuth.IssueJWT()
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{
		"jwt": t,
	})
}

func setupRoutes(e *echo.Echo) {

	e.Pre(enforceMaxOneDotPerURL)

	e.GET("/ws", websocketRoute)
	e.GET("/", indexRoute)

	e.GET("/login", indexRoute)
	e.POST("/login", loginRoute)

	e.GET("/static/*", passThroughRoute)
	e.GET("/branding/*", passThroughRoute)
	e.GET("/favicon/*", passThroughRoute)

	// Hot reload code
	e.GET("/main*.js", passThroughRoute)

	// Pass all the explorer request through to the local rest API
	api := e.Group("/api/v1/")
	api.GET("info", passThroughAPIRoute)
	api.GET("messages*", passThroughAPIRoute)
	api.GET("outputs*", passThroughAPIRoute)
	api.GET("addresses*", passThroughAPIRoute)
	api.GET("milestones*", passThroughAPIRoute)
	api.GET("peers*", passThroughAPIRoute, jwtAuth.Middleware())
	//TODO: add support for POST/DELETE
	//api.POST("peers*", passThroughAPIRoute, jwtAuth.Middleware())
	//api.DELETE("peers*", passThroughAPIRoute, jwtAuth.Middleware())

	// Plugins
	e.GET("/api/plugins/*", passThroughAPIRoute, jwtAuth.Middleware())

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
