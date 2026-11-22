package asc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrBearerTokenExpired reports that a token signed elsewhere expired and
// there is no way to fetch a new one.
var ErrBearerTokenExpired = errors.New("the App Store Connect token expired; mint a new one")

// ErrInvalidBearerToken reports a malformed or rejected external bearer token.
var ErrInvalidBearerToken = errors.New("invalid bearer token")

// ErrBearerTokenRenewal reports that fetching a new token signed elsewhere failed.
var ErrBearerTokenRenewal = errors.New("could not fetch a new App Store Connect token")

// ErrNotaryNeedsPrivateKey reports a notarization request from a client that
// only holds an App Store Connect token.
var ErrNotaryNeedsPrivateKey = errors.New("notarization needs a private key; ASC_BEARER_TOKEN only authorizes App Store Connect API requests")

// BearerTokenRenewer fetches a new App Store Connect token signed elsewhere.
type BearerTokenRenewer func(ctx context.Context) (string, error)

// bearerRenewSkew renews a token this long before it expires, so a request
// built just before expiry does not reach Apple with a dead token.
const bearerRenewSkew = time.Minute

// NewClientFromBearerToken creates a client that sends a token signed
// elsewhere (for example by a service that holds the private key) instead of
// signing its own. The token must be an App Store Connect API JWT with an
// expiry. Without renew, the client refuses to send it once it expired; with
// renew, the client calls renew for the next token shortly before the current
// one expires, and for the first token when token is empty.
func NewClientFromBearerToken(ctx context.Context, token string, renew BearerTokenRenewer, timeout time.Duration) (*Client, error) {
	source := newBearerTokenSource(renew)
	if strings.TrimSpace(token) != "" || renew == nil {
		if err := source.set(token); err != nil {
			return nil, err
		}
	}
	if _, err := source.current(ctx, time.Now()); err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = ResolveTimeout()
	}
	return &Client{
		httpClient: newDefaultHTTPClient(timeout),
		bearer:     source,
	}, nil
}

// bearerTokenSource holds a token signed elsewhere and, when it can, renews it.
type bearerTokenSource struct {
	renew BearerTokenRenewer

	// lock is a one-slot semaphore rather than a mutex so a request can stop
	// waiting for another request's renewal when its context ends.
	lock      chan struct{}
	token     string
	keyID     string
	expiresAt time.Time
}

func newBearerTokenSource(renew BearerTokenRenewer) *bearerTokenSource {
	return &bearerTokenSource{renew: renew, lock: make(chan struct{}, 1)}
}

// BearerTokenInfo is what a token signed elsewhere says about itself.
type BearerTokenInfo struct {
	// KeyID is the token's kid header, the API key that signed it.
	KeyID     string
	ExpiresAt time.Time
}

// InspectBearerToken reads a token's key ID and expiry without verifying its
// signature, after the checks a client applies before sending it: it must be
// a JWT with an expiry.
func InspectBearerToken(token string) (BearerTokenInfo, error) {
	claims := jwt.RegisteredClaims{}
	parsed, _, err := jwt.NewParser().ParseUnverified(strings.TrimSpace(token), &claims)
	if err != nil {
		return BearerTokenInfo{}, fmt.Errorf("%w: %w", ErrInvalidBearerToken, err)
	}
	if claims.ExpiresAt == nil {
		return BearerTokenInfo{}, fmt.Errorf("%w: it has no expiry", ErrInvalidBearerToken)
	}
	keyID, _ := parsed.Header["kid"].(string)
	return BearerTokenInfo{KeyID: strings.TrimSpace(keyID), ExpiresAt: claims.ExpiresAt.Time}, nil
}

// set accepts a token after checking that it is a JWT with an expiry.
func (s *bearerTokenSource) set(token string) error {
	info, err := InspectBearerToken(token)
	if err != nil {
		return err
	}
	s.token = strings.TrimSpace(token)
	s.keyID = info.KeyID
	s.expiresAt = info.ExpiresAt
	return nil
}

// info reports the held token's key ID and expiry.
func (s *bearerTokenSource) info() BearerTokenInfo {
	s.lock <- struct{}{}
	defer func() { <-s.lock }()
	return BearerTokenInfo{KeyID: s.keyID, ExpiresAt: s.expiresAt}
}

// BearerTokenInfo reports the key ID and expiry of the token signed elsewhere
// that the client sends now, which can be a renewed one; ok is false for a
// client that signs its own tokens.
func (c *Client) BearerTokenInfo() (info BearerTokenInfo, ok bool) {
	if c == nil || c.bearer == nil {
		return BearerTokenInfo{}, false
	}
	return c.bearer.info(), true
}

func (s *bearerTokenSource) renewToken(ctx context.Context, now time.Time) error {
	token, err := s.renew(ctx)
	if err != nil {
		if ctx.Err() != nil {
			return err
		}
		return fmt.Errorf("%w: %w", ErrBearerTokenRenewal, err)
	}
	if err := s.set(token); err != nil {
		return fmt.Errorf("%w: %w", ErrBearerTokenRenewal, err)
	}
	// The command can run for a while; judge the new token by when it returned.
	if returned := time.Now(); returned.After(now) {
		now = returned
	}
	if !now.Before(s.expiresAt) {
		return fmt.Errorf("%w: the new token already expired", ErrBearerTokenRenewal)
	}
	if !now.Add(bearerRenewSkew).Before(s.expiresAt) {
		return fmt.Errorf("%w: the new token expires within %s", ErrBearerTokenRenewal, bearerRenewSkew)
	}
	return nil
}

// current returns a token valid at now, renewing it first when it is about to
// expire. Concurrent requests share one renewal.
func (s *bearerTokenSource) current(ctx context.Context, now time.Time) (string, error) {
	select {
	case s.lock <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-s.lock }()
	if s.renew != nil && !now.Before(s.expiresAt.Add(-bearerRenewSkew)) {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := s.renewToken(ctx, now); err != nil {
			return "", err
		}
	}
	if !now.Before(s.expiresAt) {
		return "", ErrBearerTokenExpired
	}
	return s.token, nil
}
