package jwt

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"
)

// Errors
var (
	ErrJWTInvalidClaims = echo.NewHTTPError(http.StatusUnauthorized, "invalid jwt claims")
)

type JWTAuth struct {
	subject        string
	secret         []byte
	nodeId         string
	sessionTimeout time.Duration
}

func NewJWTAuth(subject string, sessionTimeout time.Duration, nodeId string, secret crypto.PrivKey) *JWTAuth {

	if len(subject) == 0 {
		log.Fatal("subject must not be empty")
	}

	secretBytes, err := secret.Bytes()
	if err != nil {
		log.Fatal(err)
	}

	return &JWTAuth{
		subject:        subject,
		secret:         secretBytes,
		sessionTimeout: sessionTimeout,
		nodeId:         nodeId,
	}
}

type AuthClaims struct {
	jwt.StandardClaims
	Dashboard bool `json:"dashboard"`
	API       bool `json:"api"`
}

func (c *AuthClaims) compare(field string, expected string) bool {
	if field == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(field), []byte(expected)) != 0 {
		return true
	}

	return false
}

func (c *AuthClaims) verifySubject(expected string) bool {
	return c.compare(c.Subject, expected)
}

func (c *AuthClaims) verifyAudience(expected string) bool {
	return c.compare(c.Audience, expected)
}

func (j *JWTAuth) Middleware(skipper middleware.Skipper, allow func(c echo.Context, claims *AuthClaims) bool) echo.MiddlewareFunc {

	config := middleware.JWTConfig{
		ContextKey: "jwt",
		Claims:     &AuthClaims{},
		SigningKey: j.secret,
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		return func(c echo.Context) error {

			// Skip unprotected endpoints
			if skipper(c) {
				return next(c)
			}

			// Use the default JWT middleware to verify and extract the JWT
			handler := middleware.JWTWithConfig(config)(func(c echo.Context) error {
				return nil
			})

			// Run the JWT middleware
			err := handler(c)

			if err != nil {
				return err
			}

			// Read the claims set by the JWT middleware on the context
			claims, ok := c.Get("jwt").(*jwt.Token).Claims.(*AuthClaims)

			// Do extended claims validation
			if !ok || !claims.verifyAudience(j.nodeId) || !claims.verifySubject(j.subject) {
				return ErrJWTInvalidClaims
			}

			// Validate claims
			if !allow(c, claims) {
				return ErrJWTInvalidClaims
			}

			//Go to the next handler
			return next(c)
		}
	}
}

func (j *JWTAuth) IssueJWT(api bool, dashboard bool) (string, error) {

	now := time.Now()

	// Set claims
	stdClaims := jwt.StandardClaims{
		Subject:   j.subject,
		Issuer:    j.nodeId,
		Audience:  j.nodeId,
		Id:        fmt.Sprintf("%d", now.Unix()),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
	}

	if j.sessionTimeout > 0 {
		stdClaims.ExpiresAt = now.Add(j.sessionTimeout).Unix()
	}

	claims := &AuthClaims{
		StandardClaims: stdClaims,
		Dashboard:      dashboard,
		API:            api,
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	return token.SignedString(j.secret)
}

func (j *JWTAuth) VerifyJWT(token string) bool {

	t, err := jwt.ParseWithClaims(token, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err == nil && t.Valid {
		claims, ok := t.Claims.(*AuthClaims)
		if !ok {
			return false
		}
		if !claims.verifyAudience(j.nodeId) || !claims.verifySubject(j.subject) {
			return false
		}

		return true
	}
	return false
}
