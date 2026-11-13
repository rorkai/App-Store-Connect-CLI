package cmdtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"io"
	"net/http"
	"runtime"
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
	t.Setenv("ASC_KEY_TYPE", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")
	t.Setenv("ASC_BEARER_TOKEN", token)
	t.Setenv("ASC_BEARER_TOKEN_COMMAND", "")
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

func TestBearerTokenCommandSuppliesTheToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}
	fresh := signedTestToken(t, time.Now().Add(20*time.Minute))
	tests := []struct {
		name    string
		initial string
		command string
		want    string
	}{
		{name: "command alone", command: "printf '%s\\n' " + fresh, want: fresh},
		{name: "expired ASC_BEARER_TOKEN is renewed", initial: signedTestToken(t, time.Now().Add(-time.Minute)), command: "printf '%s' " + fresh, want: fresh},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateBearerTokenAuth(t, test.initial)
			t.Setenv("ASC_BEARER_TOKEN_COMMAND", test.command)
			var authorization string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				authorization = req.Header.Get("Authorization")
				response := jsonHTTPResponse(http.StatusOK, `{"data":[],"links":{}}`)
				response.Request = req
				return response, nil
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3"); code != 0 {
					t.Fatalf("exit code = %d, want 0", code)
				}
			})

			if authorization != "Bearer "+test.want {
				t.Fatalf("Authorization = %q, want the token ASC_BEARER_TOKEN_COMMAND printed; stderr: %s", authorization, stderr)
			}
			if strings.Contains(stdout+stderr, test.want) {
				t.Fatal("command output exposed the bearer token")
			}
		})
	}
}

func TestFailingBearerTokenCommandFailsAsAuthBeforeAnyRequest(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}
	tests := []struct {
		name    string
		command string
		wantErr string
	}{
		{name: "exits non-zero", command: "echo signer unavailable >&2; exit 7", wantErr: "ASC_BEARER_TOKEN_COMMAND exited with status 7"},
		{name: "prints nothing", command: "true", wantErr: "ASC_BEARER_TOKEN_COMMAND printed no token"},
		{name: "prints an expired token", command: "printf '%s' " + signedTestToken(t, time.Now().Add(-time.Minute)), wantErr: "the new token already expired"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateBearerTokenAuth(t, "")
			t.Setenv("ASC_BEARER_TOKEN_COMMAND", test.command)
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request to %s", req.URL)
				return nil, nil
			}))

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run([]string{"apps", "list"}, "1.2.3"); code != cmd.ExitAuth {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitAuth)
				}
			})

			if !strings.Contains(stderr, test.wantErr) || !strings.Contains(stderr, "Hint: Set ASC_BEARER_TOKEN to a new token") {
				t.Fatalf("stderr = %q, want %q and the bearer token hint", stderr, test.wantErr)
			}
		})
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

func TestAppsListUnauthorizedHintMatchesCredentialSource(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bearer   bool
		wantHint string
	}{
		{name: "externally signed token", bearer: true, wantHint: "Set ASC_BEARER_TOKEN to a new token"},
		{name: "private key", wantHint: "Try `asc auth status` and re-login if needed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := ""
			if tc.bearer {
				token = signedTestToken(t, time.Now().Add(20*time.Minute))
			}
			isolateBearerTokenAuth(t, token)
			if !tc.bearer {
				setupAuth(t)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/apps" {
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
				}
				if gotBearer := req.Header.Get("Authorization") == "Bearer "+token; gotBearer != tc.bearer {
					t.Fatal("request used the wrong credential source")
				}
				response := jsonHTTPResponse(http.StatusUnauthorized, `{"errors":[{"code":"NOT_AUTHORIZED","title":"Authentication credentials are missing or invalid.","detail":"Provide a properly configured and signed bearer token."}]}`)
				response.Request = req
				return response, nil
			}))

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3")
			})
			if token != "" && strings.Contains(stdout+stderr, token) {
				t.Fatal("command output exposed the bearer token")
			}
			if code != cmd.ExitAuth || stdout != "" || !strings.Contains(stderr, "Authentication credentials are missing or invalid.: Provide a properly configured and signed bearer token.") {
				t.Fatalf("exit = %d, stdout = %q, stderr = %q, want provider error with auth exit and no stdout", code, stdout, stderr)
			}
			if !strings.Contains(stderr, tc.wantHint) || (tc.bearer && strings.Contains(stderr, "asc auth status")) {
				t.Fatalf("stderr = %q, want hint %q for the credential source", stderr, tc.wantHint)
			}
		})
	}
}

func TestBearerTokenRejectsSelectionsItCannotHonorBeforeAnyRequest(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		args    []string
		wantErr string
	}{
		{
			name:    "profile flag",
			args:    []string{"--profile", "Other", "apps", "list"},
			wantErr: `ASC_BEARER_TOKEN cannot be combined with profile "Other"`,
		},
		{
			name:    "profile env",
			env:     map[string]string{"ASC_PROFILE": "Other"},
			args:    []string{"apps", "list"},
			wantErr: `ASC_BEARER_TOKEN cannot be combined with profile "Other"`,
		},
		{
			name:    "enterprise key type",
			env:     map[string]string{"ASC_KEY_TYPE": "enterprise"},
			args:    []string{"apps", "list"},
			wantErr: "ASC_BEARER_TOKEN does not support ASC_KEY_TYPE=enterprise",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isolateBearerTokenAuth(t, signedTestToken(t, time.Now().Add(20*time.Minute)))
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request to %s", req.URL)
				return nil, nil
			}))

			_, stderr := captureOutput(t, func() {
				if code := cmd.Run(test.args, "1.2.3"); code != cmd.ExitError {
					t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
				}
			})

			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantErr)
			}
		})
	}
}
