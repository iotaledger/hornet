package dashboard

import (
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/libp2p/go-libp2p-core/crypto"

	"github.com/gohornet/hornet/pkg/basicauth"
)

type JWTAuth struct {
	username       string
	passwordHash   []byte
	passwordSalt   []byte
	secret         []byte
	nodeId         string
	sessionTimeout time.Duration
}

func NewJWTAuth(username string, passwordHashHex string, passwordSaltHex string, sessionTimeout time.Duration, nodeId string, secret crypto.PrivKey) *JWTAuth {
	if len(username) == 0 {
		log.Fatalf("'%s' must not be empty", CfgDashboardAuthUsername)
	}

	if len(passwordHashHex) != 64 {
		log.Fatalf("'%s' must be 64 (hex encoded scrypt hash) in length", CfgDashboardAuthPasswordHash)
	}

	if len(passwordSaltHex) != 64 {
		log.Fatalf("'%s' must be 64 (hex encoded) in length", CfgDashboardAuthPasswordSalt)
	}

	var err error
	passwordHash, err := hex.DecodeString(passwordHashHex)
	if err != nil {
		log.Fatalf("'%s' must be hex encoded", CfgDashboardAuthPasswordHash)
	}

	passwordSalt, err := hex.DecodeString(passwordSaltHex)
	if err != nil {
		log.Fatalf("'%s' must be hex encoded", CfgDashboardAuthPasswordSalt)
	}

	secretBytes, err := secret.Bytes()
	if err != nil {
		log.Fatal(err)
	}

	return &JWTAuth{
		username:       username,
		passwordHash:   passwordHash,
		passwordSalt:   passwordSalt,
		secret:         secretBytes,
		sessionTimeout: sessionTimeout,
		nodeId:         nodeId,
	}
}

type jwtClaims struct {
	jwt.StandardClaims
}

func (c *jwtClaims) Valid() error {
	if err := c.StandardClaims.Valid(); err != nil {
		return err
	}
	if !c.VerifyAudience() {
		return fmt.Errorf("invalid aud %s", c.StandardClaims.Audience)
	}
	if !c.VerifySubject() {
		return fmt.Errorf("invalid sub %s", c.StandardClaims.Subject)
	}
	return nil
}

func (c *jwtClaims) compare(field string, expected string) bool {
	if field == "" {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(field), []byte(expected)) != 0 {
		return true
	}

	return false
}

func (c *jwtClaims) VerifySubject() bool {
	return c.compare(c.Subject, jwtAuth.username)
}

func (c *jwtClaims) VerifyAudience() bool {
	return c.compare(c.Audience, jwtAuth.nodeId)
}

func (j *JWTAuth) Middleware(skipper middleware.Skipper) echo.MiddlewareFunc {

	config := middleware.JWTConfig{
		Skipper:    skipper,
		Claims:     &jwtClaims{},
		SigningKey: j.secret,
	}

	return middleware.JWTWithConfig(config)
}

func (j *JWTAuth) IssueJWT() (string, error) {

	now := time.Now()

	// Set claims
	claims := &jwt.StandardClaims{
		Subject:   j.username,
		Issuer:    j.nodeId,
		Audience:  j.nodeId,
		Id:        fmt.Sprintf("%d", now.Unix()),
		IssuedAt:  now.Unix(),
		NotBefore: now.Unix(),
		ExpiresAt: now.Add(j.sessionTimeout).Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate encoded token and send it as response.
	return token.SignedString(j.secret)
}

func (j *JWTAuth) VerifyJWT(token string) bool {

	t, err := jwt.ParseWithClaims(token, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return j.secret, nil
	})
	if err == nil && t.Valid {
		return true
	}
	return false
}

func (j *JWTAuth) VerifyUsernameAndPassword(username string, password string) bool {
	if username != j.username {
		return false
	}

	valid, _ := basicauth.VerifyPassword([]byte(password), j.passwordSalt, j.passwordHash)
	return valid
}
