package dashboard

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gobuffalo/packr/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"

	"github.com/gohornet/hornet/pkg/jwt"
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
				// If the asset cannot be found, fall back to the index.html for routing
				path = "index.html"
				contentType = echo.MIMETextHTMLCharsetUTF8
				staticBlob, err = appBox.Find(path)
				if err != nil {
					return next(c)
				}
			}
			return c.Blob(http.StatusOK, contentType, staticBlob)
		}
	}
}

func devModeReverseProxyMiddleware() echo.MiddlewareFunc {

	apiURL, err := url.Parse("http://127.0.0.1:9090")
	if err != nil {
		Plugin.LogFatalf("wrong devmode url: %s", err)
	}

	return middleware.Proxy(middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiURL,
		},
	}))
}

func apiMiddlewares() []echo.MiddlewareFunc {

	proxySkipper := func(context echo.Context) bool {
		// Only proxy allowed routes, skip all others
		return !deps.DashboardAllowedAPIRoute(context)
	}

	apiBindAddr := deps.RestAPIBindAddress
	_, apiBindPort, err := net.SplitHostPort(apiBindAddr)
	if err != nil {
		Plugin.LogFatalf("wrong REST API bind address: %s", err)
	}

	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", apiBindPort))
	if err != nil {
		Plugin.LogFatalf("wrong dashboard API url: %s", err)
	}

	balancer := middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiURL,
		},
	})

	config := middleware.ProxyConfig{
		Skipper:  proxySkipper,
		Balancer: balancer,
	}

	// Protect this routes with JWT even if the API is not protected
	jwtAuthRoutes := []string{
		"/api/v1/peers",
		"/api/plugins",
	}

	jwtAuthSkipper := func(context echo.Context) bool {
		path := context.Request().URL.EscapedPath()
		for _, prefix := range jwtAuthRoutes {
			if strings.HasPrefix(path, prefix) {
				return false
			}
		}
		return true
	}

	// Only allow JWT created for the dashboard
	jwtAuthAllow := func(_ echo.Context, subject string, claims *jwt.AuthClaims) bool {
		if claims.Dashboard {
			return claims.VerifySubject(subject)
		}
		return false
	}

	return []echo.MiddlewareFunc{
		jwtAuth.Middleware(jwtAuthSkipper, jwtAuthAllow),
		middleware.ProxyWithConfig(config),
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
		if !jwtAuth.VerifyJWT(request.JWT, func(claims *jwt.AuthClaims) bool {
			return claims.Dashboard
		}) {
			return echo.ErrUnauthorized
		}
	} else if !basicAuth.VerifyUsernameAndPassword(request.User, request.Password) {
		return echo.ErrUnauthorized
	}

	t, err := jwtAuth.IssueJWT(false, true)
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

	mw := appBoxMiddleware()
	if deps.NodeConfig.Bool(CfgDashboardDevMode) {
		mw = devModeReverseProxyMiddleware()
	}
	e.Group("/*").Use(mw)

	// Pass all the explorer request through to the local rest API
	e.Group("/api", apiMiddlewares()...)

	e.GET("/ws", websocketRoute)

	// Rate-limit the auth endpoint
	rateLimiterConfig := middleware.RateLimiterConfig{
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      rate.Limit(5 / 60.0), // 5 request every 1 minute
				Burst:     10,                   // additional burst of 10 requests
				ExpiresIn: 5 * time.Minute,
			},
		),
	}

	e.POST("/auth", authRoute, middleware.RateLimiterWithConfig(rateLimiterConfig))
}
