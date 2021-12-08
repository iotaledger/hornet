package restapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/gohornet/hornet/pkg/jwt"
)

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

func apiMiddleware() echo.MiddlewareFunc {

	publicRoutes := compileRoutesAsRegexes(deps.NodeConfig.Strings(CfgRestAPIPublicRoutes))
	protectedRoutes := compileRoutesAsRegexes(deps.NodeConfig.Strings(CfgRestAPIProtectedRoutes))

	matchPublic := func(c echo.Context) bool {
		for _, reg := range publicRoutes {
			if reg.MatchString(strings.ToLower(c.Path())) {
				return true
			}
		}
		return false
	}

	matchExposed := func(c echo.Context) bool {
		for _, reg := range append(publicRoutes, protectedRoutes...) {
			if reg.MatchString(strings.ToLower(c.Path())) {
				return true
			}
		}
		return false
	}

	// configure JWT auth
	salt := deps.NodeConfig.String(CfgRestAPIJWTAuthSalt)
	if len(salt) == 0 {
		Plugin.LogFatalf("'%s' should not be empty", CfgRestAPIJWTAuthSalt)
	}

	// API tokens do not expire.
	var err error
	jwtAuth, err = jwt.NewJWTAuth(salt,
		0,
		deps.Host.ID().String(),
		deps.NodePrivateKey,
	)
	if err != nil {
		Plugin.LogPanicf("JWT auth initialization failed: %w", err)
	}

	jwtAllow := func(c echo.Context, subject string, claims *jwt.AuthClaims) bool {
		// Allow all JWT created for the API if the endpoints are exposed
		if matchExposed(c) && claims.API {
			return claims.VerifySubject(subject)
		}

		// Only allow Dashboard JWT for certain routes
		if claims.Dashboard {
			if deps.DashboardAuthUsername == "" {
				return false
			}
			return claims.VerifySubject(deps.DashboardAuthUsername) && dashboardAllowedAPIRoute(c)
		}

		return false
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		// Skip routes matching the publicRoutes
		publicSkipper := func(c echo.Context) bool {
			return matchPublic(c)
		}

		jwtMiddlewareHandler := jwtAuth.Middleware(publicSkipper, jwtAllow)(next)

		return func(c echo.Context) error {

			// Check if the route should be exposed (public or protected) or is required by the dashboard
			if matchExposed(c) || dashboardAllowedAPIRoute(c) {
				// Apply JWT middleware
				return jwtMiddlewareHandler(c)
			}

			return echo.ErrForbidden
		}
	}
}

var dashboardAllowedRoutes = map[string][]string{
	http.MethodGet: {
		"/api/v1/addresses",
		"/api/v1/info",
		"/api/v1/messages",
		"/api/v1/milestones",
		"/api/v1/outputs",
		"/api/v1/peers",
		"/api/v1/transactions",
		"/api/plugins/spammer",
		"/api/plugins/participation/events",
	},
	http.MethodPost: {
		"/api/v1/peers",
		"/api/plugins/spammer",
		"/api/plugins/participation/admin/events",
	},
	http.MethodDelete: {
		"/api/v1/peers",
		"/api/plugins/participation/admin/events",
	},
}

var faucetAllowedRoutes = map[string][]string{
	http.MethodGet: {
		"/api/plugins/faucet/info",
	},
	http.MethodPost: {
		"/api/plugins/faucet/enqueue",
	},
}

func checkAllowedAPIRoute(context echo.Context, allowedRoutes map[string][]string) bool {

	// Check for which route we will allow to access the API
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

func dashboardAllowedAPIRoute(context echo.Context) bool {
	return checkAllowedAPIRoute(context, dashboardAllowedRoutes)
}

func faucetAllowedAPIRoute(context echo.Context) bool {
	return checkAllowedAPIRoute(context, faucetAllowedRoutes)
}
