package faucet

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/gobuffalo/packr/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	// holds faucet website assets
	appBox = packr.New("Faucet_App", "./frontend/public")
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

func apiMiddlewares() []echo.MiddlewareFunc {

	proxySkipper := func(context echo.Context) bool {
		// Only proxy allowed routes, skip all others
		return !deps.FaucetAllowedAPIRoute(context)
	}

	apiBindAddr := deps.RestAPIBindAddress
	_, apiBindPort, err := net.SplitHostPort(apiBindAddr)
	if err != nil {
		Plugin.LogFatalf("wrong REST API bind address: %s", err)
	}

	apiURL, err := url.Parse(fmt.Sprintf("http://localhost:%s", apiBindPort))
	if err != nil {
		Plugin.LogFatalf("wrong faucet website API url: %s", err)
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

	return []echo.MiddlewareFunc{
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

func setupRoutes(e *echo.Echo) {

	e.Pre(enforceMaxOneDotPerURL)
	//e.Use(middleware.CSRF())

	e.Group("/*").Use(appBoxMiddleware())

	// Pass all the requests through to the local rest API
	e.Group("/api", apiMiddlewares()...)
}
