package restapi

import (
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

	publicRoutesRegEx := compileRoutesAsRegexes(ParamsRestAPI.PublicRoutes)
	protectedRoutesRegEx := compileRoutesAsRegexes(ParamsRestAPI.ProtectedRoutes)

	matchPublic := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Path())

		for _, reg := range publicRoutesRegEx {
			if reg.MatchString(loweredPath) {
				return true
			}
		}
		return false
	}

	matchExposed := func(c echo.Context) bool {
		loweredPath := strings.ToLower(c.Path())

		for _, reg := range append(publicRoutesRegEx, protectedRoutesRegEx...) {
			if reg.MatchString(loweredPath) {
				return true
			}
		}
		return false
	}

	// configure JWT auth
	salt := ParamsRestAPI.JWTAuth.Salt
	if len(salt) == 0 {
		Plugin.LogFatalf("'%s' should not be empty", Plugin.App.Config().GetParameterPath(&(ParamsRestAPI.JWTAuth.Salt)))
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
		if matchExposed(c) {
			return claims.VerifySubject(subject)
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

			// Check if the route should be exposed (public or protected)
			if matchExposed(c) {
				// Apply JWT middleware
				return jwtMiddlewareHandler(c)
			}

			return echo.ErrForbidden
		}
	}
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
