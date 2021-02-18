package dashboard

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"

	"github.com/gohornet/hornet/plugins/restapi"
)

const (
	WebsocketCmdRegister   = 0
	WebsocketCmdUnregister = 1
)

var (
	// ErrInvalidParameter defines the invalid parameter error.
	ErrInvalidParameter = echo.ErrBadRequest

	// ErrInternalError defines the internal error.
	ErrInternalError = echo.ErrInternalServerError

	// ErrNotFound defines the not found error.
	ErrNotFound = echo.ErrNotFound

	// ErrForbidden defines the forbidden error.
	ErrForbidden = echo.ErrForbidden

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

func appBoxMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			contentType := calculateMimeType(c)

			path := strings.TrimPrefix(c.Request().URL.Path, "/")
			if len(path) == 0 {
				path = "index.html"
				contentType = echo.MIMETextHTMLCharsetUTF8
			}
			staticBlob, err := appBox.Find(path)
			if err != nil {
				return next(c)
			}
			return c.Blob(http.StatusOK, contentType, staticBlob)
		}
	}
}

func devModeReverseProxyMiddleware() echo.MiddlewareFunc {

	apiUrl, err := url.Parse("http://127.0.0.1:9090")
	if err != nil {
		log.Fatalf("wrong devmode url: %s", err)
	}

	return middleware.Proxy(middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiUrl,
		},
	}))
}

func apiMiddlewares() []echo.MiddlewareFunc {

	allowedRoutes := map[string][]string{
		http.MethodGet: {
			"/api/v1/info",
			"/api/v1/messages",
			"/api/v1/outputs",
			"/api/v1/addresses",
			"/api/v1/milestones",
			"/api/v1/peers",
			"/api/plugins/spammer",
		},
		http.MethodPost: {
			"/api/v1/peers",
		},
		http.MethodDelete: {
			"/api/v1/peers",
		},
	}

	jwtAuthRoutes := []string{
		"/api/v1/peers",
		"/api/plugins",
	}

	proxySkipper := func(context echo.Context) bool {
		// Check which for which route we will skip JWT authentication
		routesForMethod, exists := allowedRoutes[context.Request().Method]
		if !exists {
			return true
		}

		path := echo.GetPath(context.Request())
		for _, prefix := range routesForMethod {
			if strings.HasPrefix(path, prefix) {
				return false
			}
		}

		return true
	}

	apiBindAddr := deps.NodeConfig.String(restapi.CfgRestAPIBindAddress)
	apiUrl, err := url.Parse(fmt.Sprintf("http://%s", apiBindAddr))
	if err != nil {
		log.Fatalf("wrong dashboard API url: %s", err)
	}

	balancer := middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiUrl,
		},
	})

	config := middleware.ProxyConfig{
		Skipper:  proxySkipper,
		Balancer: balancer,
	}

	jwtAuthSkipper := func(context echo.Context) bool {
		path := echo.GetPath(context.Request())
		for _, prefix := range jwtAuthRoutes {
			if strings.HasPrefix(path, prefix) {
				return false
			}
		}
		return true
	}

	notFoundMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			return ErrNotFound
		}
	}

	return []echo.MiddlewareFunc{
		jwtAuth.Middleware(jwtAuthSkipper),
		middleware.ProxyWithConfig(config),
		notFoundMiddleware,
	}

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
		return echo.MIMETextHTMLCharsetUTF8
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

func authRoute(c echo.Context) error {

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
	e.Use(middleware.CSRF())

	middleware := appBoxMiddleware()
	if deps.NodeConfig.Bool(CfgDashboardDevMode) {
		middleware = devModeReverseProxyMiddleware()
	}
	e.Group("/*").Use(middleware)

	// Pass all the explorer request through to the local rest API
	e.Group("/api", apiMiddlewares()...)

	e.GET("/ws", websocketRoute)
	e.POST("/auth", authRoute)
}
