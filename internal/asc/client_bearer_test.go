package asc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewClientFromBearerTokenSendsTheTokenAsIs(t *testing.T) {
	token, err := GenerateJWT("KEY123", "ISS456", testJWTPrivateKey(t))
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}

	client, err := NewClientFromBearerToken(context.Background(), " "+token+"\n", nil, 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}
	req, err := client.newRequest(context.Background(), http.MethodGet, "/v1/apps", nil)
	if err != nil {
		t.Fatalf("newRequest() error: %v", err)
	}

	if got := req.Header.Get("Authorization"); got != "Bearer "+token {
		t.Fatalf("Authorization = %q, want the given token", got)
	}
}

func TestNewClientFromBearerTokenRejectsExpiredAndMalformedTokens(t *testing.T) {
	if _, err := NewClientFromBearerToken(context.Background(), "not-a-jwt", nil, 0); err == nil {
		t.Fatal("NewClientFromBearerToken(not-a-jwt) error = nil, want invalid bearer token")
	}

	client, err := NewClientFromBearerToken(context.Background(), mustGenerateJWT(t), nil, 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}
	if _, err := client.bearer.current(context.Background(), time.Now().Add(tokenLifetime+time.Second)); !errors.Is(err, ErrBearerTokenExpired) {
		t.Fatalf("current() after expiry error = %v, want ErrBearerTokenExpired", err)
	}
}

func TestRenewableBearerTokenRenewsOnceBeforeExpiry(t *testing.T) {
	key := testJWTPrivateKey(t)
	first, err := GenerateJWT("KEY123", "ISS456", key)
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}
	second := signedTestJWT(t, time.Now().Add(2*tokenLifetime))
	var renewals atomic.Int32
	client, err := NewClientFromBearerToken(context.Background(), first, func(context.Context) (string, error) {
		renewals.Add(1)
		return second, nil
	}, 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}

	if token, err := client.bearer.current(context.Background(), time.Now()); err != nil || token != first {
		t.Fatalf("current() = %q, %v; want the initial token without renewing", token, err)
	}
	nearExpiry := time.Now().Add(tokenLifetime - bearerRenewSkew/2)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if token, err := client.bearer.current(context.Background(), nearExpiry); err != nil || token != second {
				t.Errorf("current() near expiry = %q, %v; want the renewed token", token, err)
			}
		})
	}
	wg.Wait()
	if got := renewals.Load(); got != 1 {
		t.Fatalf("renewals = %d, want 1 shared by concurrent requests", got)
	}
}

func TestRenewableBearerTokenReportsRenewalFailures(t *testing.T) {
	tests := []struct {
		name  string
		renew BearerTokenRenewer
	}{
		{"renewer fails", func(context.Context) (string, error) { return "", errors.New("helper exited with status 1") }},
		{"renewer prints an expired token", func(context.Context) (string, error) { return signedTestJWT(t, time.Now().Add(-time.Minute)), nil }},
		{"renewer prints a token inside the renewal window", func(context.Context) (string, error) { return signedTestJWT(t, time.Now().Add(bearerRenewSkew/2)), nil }},
		{"renewer prints no JWT", func(context.Context) (string, error) { return "not-a-jwt", nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewClientFromBearerToken(context.Background(), "", test.renew, 0)
			if !errors.Is(err, ErrBearerTokenRenewal) {
				t.Fatalf("NewClientFromBearerToken() error = %v, want ErrBearerTokenRenewal", err)
			}
		})
	}
}

// Ctrl-C during a renewal is an interruption, not an authentication failure.
func TestRenewableBearerTokenReportsInterruptionAsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewClientFromBearerToken(ctx, "", func(ctx context.Context) (string, error) {
		return "", fmt.Errorf("token command interrupted: %w", ctx.Err())
	}, 0)
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrBearerTokenRenewal) {
		t.Fatalf("NewClientFromBearerToken() error = %v, want context.Canceled without ErrBearerTokenRenewal", err)
	}
}

// A slow token command can return a token that was valid when the renewal
// started but expired by the time it finished.
func TestRenewableBearerTokenRejectsATokenThatExpiredWhileRenewing(t *testing.T) {
	source := &bearerTokenSource{renew: func(context.Context) (string, error) {
		return signedTestJWT(t, time.Now().Add(-time.Minute)), nil
	}}
	renewalStarted := time.Now().Add(-2 * time.Minute)
	if token, err := source.current(context.Background(), renewalStarted); !errors.Is(err, ErrBearerTokenRenewal) {
		t.Fatalf("current() = %q, %v; want ErrBearerTokenRenewal for a token that expired during renewal", token, err)
	}
}

func TestBearerTokenClientRefusesNotarization(t *testing.T) {
	client, err := NewClientFromBearerToken(context.Background(), mustGenerateJWT(t), nil, 0)
	if err != nil {
		t.Fatalf("NewClientFromBearerToken() error: %v", err)
	}

	if _, err := client.newNotaryRequest(context.Background(), http.MethodGet, "/notary/v2/submissions", nil); !errors.Is(err, ErrNotaryNeedsPrivateKey) {
		t.Fatalf("newNotaryRequest() error = %v, want ErrNotaryNeedsPrivateKey", err)
	}
}

func signedTestJWT(t *testing.T, expiresAt time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)})
	signed, err := token.SignedString(testJWTPrivateKey(t))
	if err != nil {
		t.Fatalf("SignedString() error: %v", err)
	}
	return signed
}

func mustGenerateJWT(t *testing.T) string {
	t.Helper()
	token, err := GenerateJWT("KEY123", "ISS456", testJWTPrivateKey(t))
	if err != nil {
		t.Fatalf("GenerateJWT() error: %v", err)
	}
	return token
}
