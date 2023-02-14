package restapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/iotaledger/hornet/pkg/jwt"
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
	regexes := make([]*regexp.Regexp, len(routes))
	for i, route := range routes {
		reg := compileRouteAsRegex(route)
		if reg == nil {
			Plugin.LogFatalf("Invalid route in config: %s", route)
			continue
		}
		regexes[i] = reg
	}

	return regexes
}

func apiMiddleware() echo.MiddlewareFunc {

	publicRoutesRegEx := compileRoutesAsRegexes(deps.NodeConfig.Strings(CfgRestAPIPublicRoutes))
	protectedRoutesRegEx := compileRoutesAsRegexes(deps.NodeConfig.Strings(CfgRestAPIProtectedRoutes))

	matchPublic := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Request().RequestURI)

		for _, reg := range publicRoutesRegEx {
			if reg.MatchString(loweredPath) {
				return true
			}
		}

		return false
	}

	matchExposed := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Request().RequestURI)

		for _, reg := range append(publicRoutesRegEx, protectedRoutesRegEx...) {
			if reg.MatchString(loweredPath) {
				return true
			}
		}

		return false
	}

	// configure JWT auth
	salt := deps.NodeConfig.String(CfgRestAPIJWTAuthSalt)
	if len(salt) == 0 {
		Plugin.LogFatalf("'%s' should not be empty", CfgRestAPIJWTAuthSalt)
		return nil
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
			return claims.VerifySubject(deps.DashboardAuthUsername) && dashboardAllowedAPIRoute(c.Request().Method, c.Request().RequestURI)
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
			if matchExposed(c) || dashboardAllowedAPIRoute(c.Request().Method, c.Request().RequestURI) {
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
		"/api/plugins/poi/create/",
	},
	http.MethodPost: {
		"/api/v1/peers",
		"/api/plugins/spammer",
		"/api/plugins/participation/admin/events",
		"/api/plugins/poi/validate",
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

func checkAllowedAPIRoute(method string, path string, allowedRoutes map[string][]string) bool {

	// Check for which route we will allow to access the API
	routesForMethod, exists := allowedRoutes[method]
	if !exists {
		return false
	}

	for _, prefix := range routesForMethod {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

func dashboardAllowedAPIRoute(method string, path string) bool {
	return checkAllowedAPIRoute(method, path, dashboardAllowedRoutes)
}

func faucetAllowedAPIRoute(method string, path string) bool {
	return checkAllowedAPIRoute(method, path, faucetAllowedRoutes)
}
