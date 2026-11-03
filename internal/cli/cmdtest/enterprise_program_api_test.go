package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

const enterpriseAPIHost = "api.enterprise.developer.apple.com"

func enterpriseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

func setupEnterpriseAuth(t *testing.T) {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_KEY_TYPE", "enterprise")
}

func installEnterpriseTransport(t *testing.T, requests *requestLog, bodies *[]string) {
	t.Helper()
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(req.Method + " " + req.URL.Host + req.URL.Path)
		if bodies != nil && req.Body != nil {
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read body: %v", err)
			}
			*bodies = append(*bodies, string(body))
		}
		if req.Method == http.MethodPost {
			return enterpriseResponse(`{"data":{"type":"userInvitations","id":"1"}}`), nil
		}
		return enterpriseResponse(`{"data":[]}`), nil
	}))
}

func TestEnterpriseLoginStoresKeyTypeAndRoutesCommands(t *testing.T) {
	clearAuthEnvWithConfig(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	requests := newRequestLog(4)
	installEnterpriseTransport(t, requests, nil)

	_, stderr := captureOutput(t, func() {
		args := []string{
			"auth", "login", "--network", "--bypass-keychain", "--name", "enterprise",
			"--key-id", "ABC123", "--issuer-id", "00000000-0000-0000-0000-000000000000",
			"--key-type", "enterprise", "--private-key", keyPath,
		}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("auth login exit code = %d", code)
		}
		args = []string{"--profile", "enterprise", "certificates", "list", "--output", "json"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("certificates list exit code = %d", code)
		}
	})

	want := []string{
		"GET " + enterpriseAPIHost + "/v1/bundleIds",
		"GET " + enterpriseAPIHost + "/v1/certificates",
	}
	if got := requests.Snapshot(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests = %v, want %v\nstderr=%s", got, want, stderr)
	}
}

func TestEnterpriseKeyFailsOutsideTheAPIWithoutARequest(t *testing.T) {
	setupEnterpriseAuth(t)
	requests := newRequestLog(1)
	installEnterpriseTransport(t, requests, nil)

	_, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"apps", "list", "--output", "json"}, "1.2.3"); code == rootcmd.ExitSuccess {
			t.Fatal("apps list succeeded on an Enterprise key")
		}
	})
	if !strings.Contains(stderr, "GET /v1/apps: not available on the Enterprise Program API") {
		t.Fatalf("stderr = %q", stderr)
	}
	if got := requests.Snapshot(); len(got) != 0 {
		t.Fatalf("requests = %v, want none", got)
	}
}

func TestEnterpriseKeyAPICommandReachesUnlistedOperations(t *testing.T) {
	setupEnterpriseAuth(t)
	requests := newRequestLog(1)
	installEnterpriseTransport(t, requests, nil)

	captureOutput(t, func() {
		args := []string{"api", "get", "/v1/apps", "--output", "json"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d", code)
		}
	})
	want := "GET " + enterpriseAPIHost + "/v1/apps"
	if got := requests.Snapshot(); len(got) != 1 || got[0] != want {
		t.Fatalf("requests = %v, want %q", got, want)
	}
}

func TestEnterpriseAuthTokenUsesEnterpriseAudience(t *testing.T) {
	setupEnterpriseAuth(t)

	stdout, _ := captureOutput(t, func() {
		args := []string{"auth", "token", "--confirm", "--output", "json"}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d", code)
		}
	})
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	token, _, err := jwt.NewParser().ParseUnverified(payload.Token, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("ParseUnverified() error: %v", err)
	}
	audience, err := token.Claims.GetAudience()
	if err != nil || len(audience) != 1 || audience[0] != "apple-developer-enterprise-v1" {
		t.Fatalf("audience = %v, err = %v", audience, err)
	}
}

func TestEnterpriseUsersInviteOmitsAppVisibility(t *testing.T) {
	setupEnterpriseAuth(t)
	requests := newRequestLog(1)
	var bodies []string
	installEnterpriseTransport(t, requests, &bodies)

	captureOutput(t, func() {
		args := []string{
			"users", "invite", "--email", "user@example.com", "--first-name", "Jane",
			"--last-name", "Doe", "--roles", "ADMIN", "--output", "json",
		}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("exit code = %d", code)
		}
	})
	if len(bodies) != 1 || strings.Contains(bodies[0], "allAppsVisible") {
		t.Fatalf("bodies = %v", bodies)
	}
}

func TestEnterpriseUsersRejectUnsupportedFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "invite all apps",
			args: []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--all-apps"},
		},
		{
			name: "invite visible app",
			args: []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--visible-app", "APP"},
		},
		{
			name: "update visible app",
			args: []string{"users", "update", "--id", "USER", "--roles", "ADMIN", "--visible-app", "APP", "--confirm"},
		},
		{
			name: "invite provisioning allowed",
			args: []string{"users", "invite", "--email", "user@example.com", "--first-name", "Jane", "--last-name", "Doe", "--roles", "ADMIN", "--provisioning-allowed", "false"},
		},
		{
			name: "update provisioning allowed",
			args: []string{"users", "update", "--id", "USER", "--roles", "ADMIN", "--provisioning-allowed", "true"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupEnterpriseAuth(t)
			requests := newRequestLog(1)
			installEnterpriseTransport(t, requests, nil)

			_, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})
			if !strings.Contains(stderr, "not available on the Enterprise Program API") {
				t.Fatalf("stderr = %q", stderr)
			}
			if got := requests.Snapshot(); len(got) != 0 {
				t.Fatalf("requests = %v, want none", got)
			}
		})
	}
}

func TestEnterpriseUserReadsDoNotInventAppVisibility(t *testing.T) {
	tests := []struct {
		name string
		args []string
		body string
	}{
		{
			name: "users table",
			args: []string{"users", "list", "--output", "table"},
			body: `{"data":[{"type":"users","id":"1","attributes":{"username":"jane","firstName":"Jane","lastName":"Doe","roles":["ADMIN"]}}]}`,
		},
		{
			name: "invitations json",
			args: []string{"users", "invites", "list", "--output", "json"},
			body: `{"data":[{"type":"userInvitations","id":"1","attributes":{"email":"jane@example.com","firstName":"Jane","lastName":"Doe","roles":["ADMIN"]}}]}`,
		},
		{
			name: "invitations table",
			args: []string{"users", "invites", "list", "--output", "table"},
			body: `{"data":[{"type":"userInvitations","id":"1","attributes":{"email":"jane@example.com","firstName":"Jane","lastName":"Doe","roles":["ADMIN"]}}]}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupEnterpriseAuth(t)
			installDefaultTransport(t, roundTripFunc(func(*http.Request) (*http.Response, error) {
				return enterpriseResponse(test.body), nil
			}))

			stdout, _ := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "1.2.3"); code != rootcmd.ExitSuccess {
					t.Fatalf("exit code = %d", code)
				}
			})
			for _, invented := range []string{"false", "allAppsVisible", "provisioningAllowed"} {
				if strings.Contains(stdout, invented) {
					t.Fatalf("stdout contains %q: %s", invented, stdout)
				}
			}
		})
	}
}

func TestEnterpriseValidationNamesTheBundleIDProbe(t *testing.T) {
	clearAuthEnvWithConfig(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey.p8")
	writeECDSAPEM(t, keyPath)
	installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/bundleIds" {
			t.Fatalf("unexpected request: %s", req.URL)
		}
		response := enterpriseResponse(`{"errors":[{"status":"403","code":"FORBIDDEN_ERROR","title":"Forbidden","detail":"No access"}]}`)
		response.StatusCode = http.StatusForbidden
		return response, nil
	}))

	stdout, _ := captureOutput(t, func() {
		args := []string{
			"auth", "login", "--skip-validation", "--bypass-keychain", "--name", "enterprise",
			"--key-id", "ABC123", "--issuer-id", "00000000-0000-0000-0000-000000000000",
			"--key-type", "enterprise", "--private-key", keyPath,
		}
		if code := rootcmd.Run(args, "1.2.3"); code != rootcmd.ExitSuccess {
			t.Fatalf("auth login exit code = %d", code)
		}
		rootcmd.Run([]string{"auth", "status", "--validate", "--output", "json"}, "1.2.3")
	})
	if !strings.Contains(stdout, "insufficient permissions for bundle IDs list") {
		t.Fatalf("stdout = %s", stdout)
	}
}

func TestEnterpriseKeyRefusesAltool(t *testing.T) {
	setupEnterpriseAuth(t)
	ipaPath := filepath.Join(t.TempDir(), "App.ipa")
	if err := os.WriteFile(ipaPath, []byte("ipa"), 0o600); err != nil {
		t.Fatalf("write ipa: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		if code := rootcmd.Run([]string{"xcode", "validate", "--ipa", ipaPath}, "1.2.3"); code == rootcmd.ExitSuccess {
			t.Fatal("xcode validate succeeded on an Enterprise key")
		}
	})
	if !strings.Contains(stderr, "not available on the Enterprise Program API") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestEnterpriseCertificateFilterOnlyTrimsInferredTypes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "inferred by signing",
			args: []string{"signing", "fetch", "--bundle-id", "com.example.app", "--profile-type", "MAC_APP_DIRECT", "--create-missing", "--output", "OUTPUT"},
			want: "DEVELOPER_ID_APPLICATION",
		},
		{
			name: "named by the caller",
			args: []string{"certificates", "list", "--certificate-type", "DEVELOPER_ID_APPLICATION,DEVELOPER_ID_APPLICATION_G2", "--output", "json"},
			want: "DEVELOPER_ID_APPLICATION,DEVELOPER_ID_APPLICATION_G2",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupEnterpriseAuth(t)
			var got string
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/v1/certificates":
					got = req.URL.Query().Get("filter[certificateType]")
				case "/v1/bundleIds":
					return enterpriseResponse(`{"data":[{"type":"bundleIds","id":"B1","attributes":{"identifier":"com.example.app","name":"App","platform":"MAC_OS"}}]}`), nil
				}
				return enterpriseResponse(`{"data":[]}`), nil
			}))

			args := make([]string, len(test.args))
			for index, arg := range test.args {
				if arg == "OUTPUT" {
					arg = t.TempDir()
				}
				args[index] = arg
			}
			captureOutput(t, func() {
				rootcmd.Run(args, "1.2.3")
			})
			if got != test.want {
				t.Fatalf("filter = %q, want %q", got, test.want)
			}
		})
	}
}
