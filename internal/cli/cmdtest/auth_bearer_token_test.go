package cmdtest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
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
	return signedTestTokenForKey(t, "KEY123", expiresAt)
}

func signedTestTokenForKey(t *testing.T, keyID string, expiresAt time.Time) string {
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
	token.Header["kid"] = keyID
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("SignedString() error: %v", err)
	}
	return signed
}

func TestBearerTokenAuthenticatesWithoutAPrivateKey(t *testing.T) {
	token := signedTestToken(t, time.Now().Add(20*time.Minute))
	isolateBearerTokenAuth(t, token)
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

	_, stderr := captureOutput(t, func() {
		if code := cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3"); code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
	})

	if authorization := reg.Requests()[0].Header.Get("Authorization"); authorization != "Bearer "+token {
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
			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(asctest.GET("/v1/apps"), asctest.JSON(http.StatusOK, `{"data":[],"links":{}}`))

			stdout, stderr := captureOutput(t, func() {
				if code := cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3"); code != 0 {
					t.Fatalf("exit code = %d, want 0", code)
				}
			})

			if authorization := reg.Requests()[0].Header.Get("Authorization"); authorization != "Bearer "+test.want {
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
			asctest.NewRegistry(t).Install()

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
			asctest.NewRegistry(t).Install()

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
			reg := asctest.NewRegistry(t)
			reg.Install()
			unauthorized := asctest.JSON(http.StatusUnauthorized, `{"errors":[{"code":"NOT_AUTHORIZED","title":"Authentication credentials are missing or invalid.","detail":"Provide a properly configured and signed bearer token."}]}`)
			reg.Register(asctest.GET("/v1/apps"), func(req *http.Request) (*http.Response, error) {
				response, err := unauthorized(req)
				response.Request = req
				return response, err
			})

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3")
			})
			if gotBearer := reg.Requests()[0].Header.Get("Authorization") == "Bearer "+token; gotBearer != tc.bearer {
				t.Fatal("request used the wrong credential source")
			}
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
			asctest.NewRegistry(t).Install()

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

func TestAuthStatusReportsTheBearerToken(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}
	const agreementBody = `{"errors":[{"status":"403","code":"FORBIDDEN.REQUIRED_AGREEMENTS_MISSING_OR_EXPIRED","title":"A required agreement is missing or has expired","detail":"This request requires an in-effect agreement that has not been signed or has expired."}]}`
	const unauthorizedBody = `{"errors":[{"status":"401","code":"NOT_AUTHORIZED","title":"Authentication credentials are missing or invalid.","detail":"Provide a properly configured and signed bearer token."}]}`
	type bearerToken struct {
		Source          string `json:"source"`
		KeyID           string `json:"keyId"`
		ExpiresAt       string `json:"expiresAt"`
		Expired         *bool  `json:"expired"`
		Error           string `json:"error"`
		Validation      string `json:"validation"`
		ValidationError string `json:"validationError"`
	}
	fresh := time.Now().Add(20 * time.Minute).Truncate(time.Second)
	expired := time.Now().Add(-time.Minute).Truncate(time.Second)
	renewed := time.Now().Add(15 * time.Minute).Truncate(time.Second)
	renewedToken := signedTestTokenForKey(t, "OTHERKEY99", renewed)
	for _, tc := range []struct {
		name          string
		envExpiresAt  time.Time
		command       bool
		env           map[string]string
		validate      bool
		status        int
		body          string
		wantCode      int
		wantSource    string
		wantKeyID     string
		wantExpiresAt time.Time
		wantExpired   bool
		wantNoToken   bool
		wantError     string
		wantValidity  string
		wantValErr    string
	}{
		{name: "unexpired token", envExpiresAt: fresh, wantCode: cmd.ExitSuccess, wantKeyID: "KEY123", wantExpiresAt: fresh},
		{name: "expired token", envExpiresAt: expired, wantCode: cmd.ExitSuccess, wantKeyID: "KEY123", wantExpiresAt: expired, wantExpired: true},
		{name: "profile refuses the token", envExpiresAt: fresh, env: map[string]string{"ASC_PROFILE": "Other"}, wantCode: cmd.ExitSuccess, wantKeyID: "KEY123", wantExpiresAt: fresh, wantError: `cannot be combined with profile "Other"`},
		{name: "enterprise key type refuses the token", envExpiresAt: fresh, env: map[string]string{"ASC_KEY_TYPE": "enterprise"}, wantCode: cmd.ExitSuccess, wantKeyID: "KEY123", wantExpiresAt: fresh, wantError: "does not support ASC_KEY_TYPE=enterprise"},
		{name: "command is not run without validate", command: true, wantCode: cmd.ExitSuccess, wantSource: "ASC_BEARER_TOKEN_COMMAND", wantNoToken: true},
		{name: "validate works", envExpiresAt: fresh, validate: true, status: http.StatusOK, body: `{"data":[],"links":{}}`, wantCode: cmd.ExitSuccess, wantKeyID: "KEY123", wantExpiresAt: fresh, wantValidity: "works"},
		{name: "validate reports the token the command renewed", envExpiresAt: expired, command: true, validate: true, status: http.StatusOK, body: `{"data":[],"links":{}}`, wantCode: cmd.ExitSuccess, wantKeyID: "OTHERKEY99", wantExpiresAt: renewed, wantValidity: "works"},
		{name: "validate reports the command's token", command: true, validate: true, status: http.StatusOK, body: `{"data":[],"links":{}}`, wantCode: cmd.ExitSuccess, wantSource: "ASC_BEARER_TOKEN_COMMAND", wantKeyID: "OTHERKEY99", wantExpiresAt: renewed, wantValidity: "works"},
		{name: "validate agreement blocked", envExpiresAt: fresh, validate: true, status: http.StatusForbidden, body: agreementBody, wantCode: cmd.ExitError, wantKeyID: "KEY123", wantExpiresAt: fresh, wantValidity: "failed", wantValErr: "An Account Holder must accept it"},
		{name: "validate rejected by Apple", envExpiresAt: fresh, validate: true, status: http.StatusUnauthorized, body: unauthorizedBody, wantCode: cmd.ExitError, wantKeyID: "KEY123", wantExpiresAt: fresh, wantValidity: "failed", wantValErr: "Authentication credentials are missing or invalid."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token := ""
			if !tc.envExpiresAt.IsZero() {
				token = signedTestToken(t, tc.envExpiresAt)
			}
			isolateBearerTokenAuth(t, token)
			ranMarker := filepath.Join(t.TempDir(), "ran")
			if tc.command {
				t.Setenv("ASC_BEARER_TOKEN_COMMAND", "touch '"+ranMarker+"'; printf '%s' "+renewedToken)
			}
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			reg := asctest.NewRegistry(t)
			reg.Install()
			args := []string{"auth", "status", "--output", "json"}
			if tc.validate {
				args = append(args, "--validate")
				reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(tc.status, tc.body))
			}

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(args, "1.2.3")
			})

			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr: %s", code, tc.wantCode, stderr)
			}
			if (token != "" && strings.Contains(stdout+stderr, token)) || strings.Contains(stdout+stderr, renewedToken) {
				t.Fatal("auth status exposed the bearer token")
			}
			var payload struct {
				BearerToken *bearerToken `json:"bearerToken"`
			}
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("stdout is not JSON: %v; stdout: %q", err, stdout)
			}
			wantSource := tc.wantSource
			if wantSource == "" {
				wantSource = "ASC_BEARER_TOKEN"
			}
			wantExpiresAt := tc.wantExpiresAt.UTC().Format(time.RFC3339)
			got := payload.BearerToken
			if tc.wantNoToken {
				if got == nil || got.Source != wantSource || got.KeyID != "" || got.ExpiresAt != "" || got.Expired != nil {
					t.Fatalf("bearerToken = %+v, want only the source %s", got, wantSource)
				}
			} else if got == nil || got.Source != wantSource || got.KeyID != tc.wantKeyID || got.ExpiresAt != wantExpiresAt || got.Expired == nil || *got.Expired != tc.wantExpired {
				t.Fatalf("bearerToken = %+v, want %s for %s expiring %s (expired %t)", got, wantSource, tc.wantKeyID, wantExpiresAt, tc.wantExpired)
			}
			if tc.wantError == "" && got.Error != "" || !strings.Contains(got.Error, tc.wantError) {
				t.Fatalf("error = %q, want %q", got.Error, tc.wantError)
			}
			if got.Validation != tc.wantValidity || !strings.Contains(got.ValidationError, tc.wantValErr) {
				t.Fatalf("validation = %q (%q), want %q containing %q", got.Validation, got.ValidationError, tc.wantValidity, tc.wantValErr)
			}
			if _, err := os.Stat(ranMarker); (err == nil) != (tc.command && tc.validate) {
				t.Fatalf("ASC_BEARER_TOKEN_COMMAND ran = %t, want %t", err == nil, tc.command && tc.validate)
			}
			if tc.validate {
				sent := strings.TrimPrefix(reg.Requests()[0].Header.Get("Authorization"), "Bearer ")
				if want := map[bool]string{true: renewedToken, false: token}[tc.command]; sent != want {
					t.Fatal("--validate sent a different token than the one asc would use")
				}
			}
		})
	}
}

func TestAuthStatusTableBearerTokenLine(t *testing.T) {
	// A JSON string can carry C1 controls, bidi overrides and DEL; the JWT
	// library echoes this exp value in its error.
	unsafeExp := "x\u009b31mC1\u202e\u007fZ"
	unsafeToken := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","kid":"KEY123"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(`{"exp":"`+unsafeExp+`"}`)) + ".c2ln"
	for _, tc := range []struct {
		name     string
		token    string
		env      map[string]string
		validate bool
		wantCode int
		want     string
		reject   []string
	}{
		{
			name:     "kid with terminal controls",
			token:    signedTestTokenForKey(t, "K\x1b[31mRED\nStored credentials: fake", time.Now().Add(20*time.Minute)),
			wantCode: cmd.ExitSuccess,
			want:     "Bearer token: ASC_BEARER_TOKEN (key K",
			reject:   []string{"\x1b", "\nStored credentials: fake"},
		},
		{
			name:     "error with terminal controls",
			token:    unsafeToken,
			wantCode: cmd.ExitSuccess,
			want:     "Bearer token: ASC_BEARER_TOKEN (invalid bearer token",
			reject:   []string{"\u009b", "\u202e", "\u007f"},
		},
		{
			name:     "validation error with terminal controls",
			token:    unsafeToken,
			validate: true,
			wantCode: cmd.ExitError,
			want:     ": failed (",
			reject:   []string{"\u009b", "\u202e", "\u007f"},
		},
		{
			name:     "token every command refuses",
			token:    signedTestToken(t, time.Now().Add(20*time.Minute)),
			env:      map[string]string{"ASC_PROFILE": "Other"},
			wantCode: cmd.ExitSuccess,
			want:     `ASC_BEARER_TOKEN cannot be combined with profile "Other"`,
			reject:   []string{"used ahead of stored and environment credentials"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			isolateBearerTokenAuth(t, tc.token)
			for name, value := range tc.env {
				t.Setenv(name, value)
			}
			asctest.NewRegistry(t).Install()
			args := []string{"auth", "status", "--output", "table"}
			if tc.validate {
				args = append(args, "--validate")
			}

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(args, "1.2.3")
			})

			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr: %s", code, tc.wantCode, stderr)
			}
			var line string
			for candidate := range strings.SplitSeq(stdout, "\n") {
				if strings.HasPrefix(candidate, "Bearer token: ") {
					line = candidate
				}
			}
			if !strings.Contains(line, tc.want) {
				t.Fatalf("Bearer token line = %q, want it to contain %q; stdout: %q", line, tc.want, stdout)
			}
			for _, rejected := range tc.reject {
				if strings.Contains(stdout, rejected) {
					t.Fatalf("stdout = %q, want no %q", stdout, rejected)
				}
			}
		})
	}
}
