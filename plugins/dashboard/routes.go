package dashboard

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"

	"github.com/iotaledger/hornet-dashboard"
	"github.com/iotaledger/hornet/pkg/jwt"
	"github.com/iotaledger/hornet/pkg/restapi"
)

const (
	WebsocketCmdRegister   = 0
	WebsocketCmdUnregister = 1
)

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

func compileRouteAsRegex(route string) *regexp.Regexp {

	r := regexp.QuoteMeta(route)
	r = strings.Replace(r, `\*`, "(.*?)", -1)
	r = r + "$"

	reg, err := regexp.Compile(r)
	if err != nil {
		return nil
	}
	return reg
}

func compileRoutesAsRegexes(routes []string) []*regexp.Regexp {
	var regexes []*regexp.Regexp
	for _, route := range routes {
		reg := compileRouteAsRegex(route)
		if reg == nil {
			Plugin.LogFatalf("Invalid route in config: %s", route)
			continue
		}
		regexes = append(regexes, reg)
	}
	return regexes
}

func apiMiddlewares() []echo.MiddlewareFunc {

	apiBindAddr := deps.RestAPIBindAddress
	_, apiBindPort, err := net.SplitHostPort(apiBindAddr)
	if err != nil {
		Plugin.LogFatalf("wrong REST API bind address: %s", err)
	}

	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", apiBindPort))
	if err != nil {
		Plugin.LogFatalf("wrong dashboard API url: %s", err)
	}

	proxySkipper := func(context echo.Context) bool {
		// Only proxy allowed routes, skip all others
		return !deps.DashboardAllowedAPIRoute(context)
	}

	balancer := middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
		{
			URL: apiURL,
		},
	})

	proxyConfig := middleware.ProxyConfig{
		Skipper:  proxySkipper,
		Balancer: balancer,
	}

	// the HTTP REST routes which can be called without authorization.
	// Wildcards using * are allowed
	publicRoutes := []string{
		"/api/plugins/indexer/v1/*",
	}

	// the HTTP REST routes which need to be called with authorization even if the API is not protected.
	// Wildcards using * are allowed
	protectedRoutes := []string{
		"/api/v2/peers*",
		"/api/plugins/*",
	}

	publicRoutesRegEx := compileRoutesAsRegexes(publicRoutes)
	protectedRoutesRegEx := compileRoutesAsRegexes(protectedRoutes)

	matchPublic := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Request().URL.EscapedPath())

		for _, reg := range publicRoutesRegEx {
			if reg.MatchString(loweredPath) {
				return true
			}
		}
		return false
	}

	matchProtected := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Request().URL.EscapedPath())

		for _, reg := range protectedRoutesRegEx {
			if reg.MatchString(loweredPath) {
				return true
			}
		}
		return false
	}

	// Skip routes explicitely matching the publicRoutes, or not matching the protectedRoutes
	jwtAuthSkipper := func(c echo.Context) bool {
		return matchPublic(c) || !matchProtected(c)
	}

	// Only allow JWT created for the dashboard
	jwtAllow := func(_ echo.Context, subject string, claims *jwt.AuthClaims) bool {
		if claims.Dashboard {
			return claims.VerifySubject(subject)
		}
		return false
	}

	return []echo.MiddlewareFunc{
		jwtAuth.Middleware(jwtAuthSkipper, jwtAllow),
		middleware.ProxyWithConfig(proxyConfig),
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
		return errors.WithMessagef(restapi.ErrInvalidParameter, "invalid request, error: %s", err)
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

	e.Use(middleware.CSRF())

	mw := dashboard.FrontendMiddleware()
	if ParamsDashboard.DevMode {
		mw = devModeReverseProxyMiddleware()
	}
	e.Group("/*").Use(mw)

	// Pass all the dashboard request through to the local rest API
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
