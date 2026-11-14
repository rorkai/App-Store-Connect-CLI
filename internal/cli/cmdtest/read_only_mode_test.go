package cmdtest

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
	webcmd "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/web"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	webcore "github.com/rudrankriyam/App-Store-Connect-CLI/internal/web"
)

func TestReadOnlyEnvRefusesPublicAPIMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv(readonly.EnvVar, "1")

	reg := asctest.NewRegistry(t)
	reg.Install()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"builds", "update",
			"--build-id", "build-99",
			"--uses-non-exempt-encryption=false",
		}, "1.0.0")
	})
	if code != cmd.ExitReadOnly {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitReadOnly, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: ASC_READ_ONLY is set; refusing PATCH /v1/builds/build-99\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	mutating := 0
	for _, req := range reg.Requests() {
		if req.Method != http.MethodGet {
			mutating++
		}
	}
	if mutating != 0 {
		t.Fatalf("%d mutating requests reached the transport, want 0", mutating)
	}
}

func TestReadOnlyFlagRefusesPublicAPIMutationAndNamesTheFlag(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv(readonly.EnvVar, "")

	asctest.NewRegistry(t).Install()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"--read-only",
			"builds", "update",
			"--build-id", "build-99",
			"--uses-non-exempt-encryption=false",
		}, "1.0.0")
	})
	if code != cmd.ExitReadOnly {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitReadOnly, stdout, stderr)
	}
	want := "Error: --read-only is set; refusing PATCH /v1/builds/build-99\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}

	// The flag is per invocation: a later run without it must not inherit it.
	if readonly.Enabled() {
		t.Fatal("read-only mode still enabled after the invocation finished; the root flag leaked")
	}
}

func TestReadOnlyAllowsPublicAPIReads(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv(readonly.EnvVar, "true")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"apps","id":"123","attributes":{"name":"App","bundleId":"com.example.app","sku":"SKU"}}],"links":{}}`))

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"apps", "list", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr)
	}
	if gets := len(reg.Requests()); gets != 1 {
		t.Fatalf("GET requests = %d, want 1", gets)
	}
	if !strings.Contains(stdout, `"id":"123"`) {
		t.Fatalf("stdout = %q, want app list", stdout)
	}
}

func TestReadOnlyEnvRefusesWebMutationAfterPreflightReads(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var patchCalls int
	restoreSession := webcmd.SetResolveWebSession(func(ctx context.Context, appleID, password, twoFactorCode string) (*webcore.AuthSession, string, error) {
		return &webcore.AuthSession{
			Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/1234567890":
					return webAppsDeleteJSONResponse(`{
						"data": {
							"type": "apps",
							"id": "1234567890",
							"attributes": {
								"name": "Throwaway",
								"bundleId": "com.example.throwaway",
								"removed": false,
								"appStoreLegacyStatus": "PREPARE_FOR_SUBMISSION",
								"marketplace": "APP_STORE"
							},
							"relationships": {
								"displayableVersions": {"data": []}
							}
						}
					}`), nil
				case req.Method == http.MethodGet && req.URL.Path == "/iris/v1/apps/1234567890/appAvailabilityV2":
					return webAppsDeleteJSONResponse(`{
						"data": {
							"id": "avail-1",
							"type": "appAvailabilities",
							"attributes": {"availableInNewTerritories": false},
							"relationships": {
								"availableTerritories": {"data": []}
							}
						}
					}`), nil
				case req.Method == http.MethodPatch:
					patchCalls++
					t.Errorf("PATCH left the web client under read-only mode: %s", req.URL.String())
					return webAppsDeleteJSONResponse(`{}`), nil
				default:
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, io.ErrUnexpectedEOF
				}
			})},
		}, "cache", nil
	})
	t.Cleanup(restoreSession)

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"web", "apps", "delete",
			"--app", "1234567890",
			"--confirm",
			"--output", "json",
		}, "1.0.0")
	})
	if code != cmd.ExitReadOnly {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitReadOnly, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: ASC_READ_ONLY is set; refusing PATCH https://appstoreconnect.apple.com/iris/v1/apps/1234567890\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if patchCalls != 0 {
		t.Fatalf("PATCH calls = %d, want 0", patchCalls)
	}
}

func TestReadOnlyRefusesConfirmedDeleteBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv(readonly.EnvVar, "1")

	asctest.NewRegistry(t).Install()

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"bundle-ids", "delete", "--id", "bundle-1", "--confirm"}, "1.0.0")
	})
	if code != cmd.ExitReadOnly {
		t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitReadOnly, stdout, stderr)
	}
	// The command wraps the refusal, but the rendered line stays the refusal
	// itself so agents can match one stable message.
	want := "Error: ASC_READ_ONLY is set; refusing DELETE /v1/bundleIds/bundle-1\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestReadOnlyRawAPITransport(t *testing.T) {
	for _, source := range []string{"environment", "flag"} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete} {
			t.Run(source+"/"+method, func(t *testing.T) {
				setupAuth(t)
				t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
				t.Setenv(readonly.EnvVar, "")
				var args []string
				refusalSource := "--read-only"
				if source == "environment" {
					t.Setenv(readonly.EnvVar, "1")
					refusalSource = readonly.EnvVar
				} else {
					args = append(args, "--read-only")
				}
				path := "/v1/betaGroups/group-1"
				if method == http.MethodGet {
					path = "/v1/apps"
				}
				if method == http.MethodPost {
					path = "/v1/betaGroups"
				}
				args = append(args, "api", method, path)
				if method != http.MethodGet {
					args = append(args, "--confirm")
				}
				if method == http.MethodPost || method == http.MethodPatch {
					args = append(args, "--body", `{"data":{}}`)
				}
				reg := asctest.NewRegistry(t)
				reg.Install()
				if method == http.MethodGet {
					reg.Register(asctest.GET(path), asctest.JSON(http.StatusOK, `{"data":[]}`))
				}
				var code int
				stdout, stderr := captureOutput(t, func() { code = cmd.Run(args, "1.0.0") })
				requests := len(reg.Requests())
				if method == http.MethodGet {
					if code != cmd.ExitSuccess || requests != 1 || strings.TrimSpace(stdout) != `{"data":[]}` {
						t.Fatalf("read result: code=%d requests=%d stdout=%q stderr=%q", code, requests, stdout, stderr)
					}
					return
				}
				want := "Error: " + refusalSource + " is set; refusing " + method + " " + path + "\n"
				if code != cmd.ExitReadOnly || requests != 0 || stdout != "" || stderr != want {
					t.Fatalf("mutation result: code=%d requests=%d stdout=%q stderr=%q, want %q", code, requests, stdout, stderr, want)
				}
			})
		}
	}
}

func TestReadOnlyRefusalInBatchCommandExitsReadOnly(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		args    func(input string) []string
		listGET asctest.Matcher
		listRes string
	}{
		{
			name:  "metadata keywords push",
			input: `{"en-US":"alpha,beta"}`,
			args: func(input string) []string {
				return []string{"metadata", "keywords", "push", "--version-id", "ver-1", "--input", input}
			},
			listGET: asctest.GET("/v1/appStoreVersions/ver-1/appStoreVersionLocalizations", "limit=200"),
			listRes: `{"data":[{"type":"appStoreVersionLocalizations","id":"loc-en","attributes":{"locale":"en-US","keywords":"old"}}],"links":{}}`,
		},
		{
			name:  "devices register-batch",
			input: "AA-BB-CC\tPhone One\tIOS\n",
			args: func(input string) []string {
				return []string{"devices", "register-batch", "--file", input, "--confirm"}
			},
			listGET: asctest.GET("/v1/devices", "fields[devices]=name,udid,platform,status", "limit=200"),
			listRes: `{"data":[],"links":{}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv(readonly.EnvVar, "1")
			t.Chdir(t.TempDir())
			if err := os.WriteFile("input", []byte(tt.input), 0o600); err != nil {
				t.Fatalf("WriteFile() error: %v", err)
			}

			reg := asctest.NewRegistry(t)
			reg.Install()
			reg.Register(tt.listGET, asctest.JSON(http.StatusOK, tt.listRes))

			var code int
			stdout, stderr := captureOutput(t, func() {
				code = cmd.Run(append(tt.args("input"), "--output", "json"), "1.0.0")
			})
			if code != cmd.ExitReadOnly {
				t.Fatalf("exit code = %d, want %d; stdout=%q stderr=%q", code, cmd.ExitReadOnly, stdout, stderr)
			}
			if !strings.Contains(stdout, "ASC_READ_ONLY is set; refusing") {
				t.Fatalf("stdout = %q, want the per-item refusal", stdout)
			}
		})
	}
}
