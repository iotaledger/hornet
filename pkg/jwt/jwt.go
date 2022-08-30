package jwt

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/pkg/errors"
)

var (
	ErrJWTInvalidClaims = echo.NewHTTPError(http.StatusUnauthorized, "invalid jwt claims")
)

type Auth struct {
	subject        string
	sessionTimeout time.Duration
	nodeID         string
	secret         []byte
}

func NewAuth(subject string, sessionTimeout time.Duration, nodeID string, secret crypto.PrivKey) (*Auth, error) {

	if len(subject) == 0 {
		return nil, errors.New("subject must not be empty")
	}

	secretBytes, err := crypto.MarshalPrivateKey(secret)
	if err != nil {
		return nil, fmt.Errorf("unable to convert private key: %w", err)
	}

	return &Auth{
		subject:        subject,
		sessionTimeout: sessionTimeout,
		nodeID:         nodeID,
		secret:         secretBytes,
	}, nil
}

type AuthClaims struct {
	jwt.StandardClaims
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

func (c *AuthClaims) VerifySubject(expected string) bool {
	return c.compare(c.Subject, expected)
}

func (j *Auth) Middleware(skipper middleware.Skipper, allow func(c echo.Context, subject string, claims *AuthClaims) bool) echo.MiddlewareFunc {

	config := middleware.JWTConfig{
		ContextKey: "jwt",
		Claims:     &AuthClaims{},
		SigningKey: j.secret,
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {

		return func(c echo.Context) error {

			// skip unprotected endpoints
			if skipper(c) {
				return next(c)
			}

			// use the default JWT middleware to verify and extract the JWT
			handler := middleware.JWTWithConfig(config)(func(c echo.Context) error {
				return nil
			})

			// run the JWT middleware
			if err := handler(c); err != nil {
				return err
			}

			token, ok := c.Get("jwt").(*jwt.Token)
			if !ok {
				return fmt.Errorf("expected *jwt.Token, got %T", c.Get("jwt"))
			}

			// validate the signing method we expect
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}

			// read the claims set by the JWT middleware on the context
			claims, ok := token.Claims.(*AuthClaims)

			// do extended claims validation
			if !ok || !claims.VerifyAudience(j.nodeID, true) {
				return ErrJWTInvalidClaims
			}

			// validate claims
			if !allow(c, j.subject, claims) {
				return ErrJWTInvalidClaims
			}

			// go to the next handler
			return next(c)
		}
	}
}

func (j *Auth) IssueJWT() (string, error) {

	now := time.Now()

	// Set claims
	stdClaims := jwt.StandardClaims{
		Subject:   j.subject,
		Issuer:    j.nodeID,
		Audience:  j.nodeID,
		Id:        fmt.Sprintf("%d", now.Unix()),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
	}

	if j.sessionTimeout > 0 {
		stdClaims.ExpiresAt = now.Add(j.sessionTimeout).Unix()
	}

	claims := &AuthClaims{
		StandardClaims: stdClaims,
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	return token.SignedString(j.secret)
}

func (j *Auth) VerifyJWT(token string, allow func(claims *AuthClaims) bool) bool {

	t, err := jwt.ParseWithClaims(token, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
		// validate the signing method we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return j.secret, nil
	})
	if err == nil && t.Valid {
		claims, ok := t.Claims.(*AuthClaims)

		if !ok || !claims.VerifyAudience(j.nodeID, true) {
			return false
		}

		// validate claims
		if !allow(claims) {
			return false
		}

		return true
	}

	return false
}
