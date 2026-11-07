package cmdtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

// isolateBearerTokenAuth leaves ASC_BEARER_TOKEN as the only credential.
func isolateBearerTokenAuth(t *testing.T, token string) {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")
	t.Setenv("ASC_BEARER_TOKEN", token)
}

func signedTestToken(t *testing.T, expiresAt time.Time) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error: %v", err)
	}
	claims := jwt.RegisteredClaims{
		Issuer:   "ISSUER",
		Audience: jwt.ClaimStrings{"appstoreconnect-v1"},
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}
	if !expiresAt.IsZero() {
		claims.ExpiresAt = jwt.NewNumericDate(expiresAt)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "KEY123"
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() error: %v", err)
	}
	return signed
}

func TestBearerTokenAuthenticatesWithoutAPrivateKey(t *testing.T) {
	token := signedTestToken(t, time.Now().Add(20*time.Minute))
	isolateBearerTokenAuth(t, token)
	var authorization string
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		authorization = req.Header.Get("Authorization")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"data":[],"links":{}}`)),
			Request:    req,
		}, nil
	}))

	_, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3"); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})

	if authorization != "Bearer "+token {
		t.Fatalf("Authorization = %q, want the ASC_BEARER_TOKEN token; stderr: %s", authorization, stderr)
	}
}

func TestUnusableBearerTokenFailsAsAuthBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"expired", signedTestToken(t, time.Now().Add(-time.Minute))},
		{"malformed", "not-a-jwt"},
		{"no expiry", signedTestToken(t, time.Time{})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateBearerTokenAuth(t, test.token)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request to %s", req.URL)
				return nil, nil
			}))

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run([]string{"apps", "list"}, "1.2.3"); code != cmd.ExitAuth {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitAuth)
				}
			})

			if !strings.Contains(stderr, "Error: apps: ASC_BEARER_TOKEN: ") {
				t.Fatalf("stderr = %q, want an ASC_BEARER_TOKEN error", stderr)
			}
			if !strings.Contains(stderr, "Hint: Set ASC_BEARER_TOKEN to a new token") || strings.Contains(stderr, "asc auth login") {
				t.Fatalf("stderr = %q, want the bearer token hint instead of the private key hint", stderr)
			}
		})
	}
}
