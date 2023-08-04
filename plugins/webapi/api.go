package webapi

import (
	"encoding/hex"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/hornet/pkg/config"
	"github.com/iotaledger/hornet/pkg/jwt"
)

const (
	MIMETextCSV = "text/csv"
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
			log.Fatalf("Invalid route in config: %s", route)
		}
		regexes[i] = reg
	}

	return regexes
}

func apiMiddleware() echo.MiddlewareFunc {

	publicRoutesRegEx := compileRoutesAsRegexes(config.NodeConfig.GetStringSlice(config.CfgWebAPIPublicRoutes))
	protectedRoutesRegEx := compileRoutesAsRegexes(config.NodeConfig.GetStringSlice(config.CfgWebAPIProtectedRoutes))

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

	// get nodes private key
	privKey := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthPrivateKey)
	privKeyFilePath := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthPrivateKeyPath)

	// load up the previously generated identity or create a new one
	jwtPrivateKey, _, err := LoadOrCreateIdentityPrivateKey(privKeyFilePath, privKey)
	if err != nil {
		log.Panic(err)
	}

	// create an ID by hashing the public key of the JWT private key
	jwtPublicKeyBytes, err := jwtPrivateKey.GetPublic().Raw()
	if err != nil {
		log.Panic(err)
	}
	jwtIDBytes := blake2b.Sum256(jwtPublicKeyBytes)
	jwtID := hex.EncodeToString(jwtIDBytes[:])

	// configure JWT auth
	salt := config.NodeConfig.GetString(config.CfgWebAPIJWTAuthSalt)
	if len(salt) == 0 {
		log.Fatalf("'%s' should not be empty", config.CfgWebAPIJWTAuthSalt)
	}

	// API tokens do not expire.
	jwtAuth, err := jwt.NewAuth(salt,
		0,
		jwtID,
		jwtPrivateKey,
	)
	if err != nil {
		log.Fatalf("JWT auth initialization failed: %s", err)
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

func getAcceptHeaderContentType(c echo.Context, supportedContentTypes ...string) (string, error) {
	ctype := c.Request().Header.Get(echo.HeaderAccept)
	for _, supportedContentType := range supportedContentTypes {
		if strings.HasPrefix(ctype, supportedContentType) {
			return supportedContentType, nil
		}
	}

	return "", ErrNotAcceptable
}
