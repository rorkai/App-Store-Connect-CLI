package asc

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrBearerTokenExpired reports that a token signed elsewhere expired; asc
// cannot sign a new one without the private key.
var ErrBearerTokenExpired = errors.New("the App Store Connect token expired; mint a new one")

// ErrInvalidBearerToken reports a token that is not a JWT with an expiry.
var ErrInvalidBearerToken = errors.New("invalid bearer token")

// ErrNotaryNeedsPrivateKey reports a notarization request from a client that
// only holds an App Store Connect token.
var ErrNotaryNeedsPrivateKey = errors.New("notarization needs a private key; ASC_BEARER_TOKEN only authorizes App Store Connect API requests")

// NewClientFromBearerToken creates a client that sends a token signed
// elsewhere (for example by a service that holds the private key) instead of
// signing its own. The token must be an App Store Connect API JWT with an
// expiry; the client refuses to send it once it expired.
func NewClientFromBearerToken(token string, timeout time.Duration) (*Client, error) {
	token = strings.TrimSpace(token)
	claims := jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidBearerToken, err)
	}
	if claims.ExpiresAt == nil {
		return nil, fmt.Errorf("%w: it has no expiry", ErrInvalidBearerToken)
	}
	if !time.Now().Before(claims.ExpiresAt.Time) {
		return nil, ErrBearerTokenExpired
	}
	if timeout <= 0 {
		timeout = ResolveTimeout()
	}
	return &Client{
		httpClient:           newDefaultHTTPClient(timeout),
		bearerToken:          token,
		bearerTokenExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// bearerJWT returns the token signed elsewhere while it is still valid.
func (c *Client) bearerJWT(now time.Time) (string, error) {
	if !now.Before(c.bearerTokenExpiresAt) {
		return "", ErrBearerTokenExpired
	}
	return c.bearerToken, nil
}
