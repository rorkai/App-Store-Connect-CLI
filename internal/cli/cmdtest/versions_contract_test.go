package cmdtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const releaseTypeValues = "MANUAL, AFTER_APPROVAL, SCHEDULED"

const appStoreVersionIncludeValues = "ageRatingDeclaration, app, appStoreVersionLocalizations, build, appStoreVersionPhasedRelease, gameCenterAppVersion, routingAppCoverage, appStoreReviewDetail, appStoreVersionSubmission, appClipDefaultExperience, appStoreVersionExperiments, appStoreVersionExperimentsV2, alternativeDistributionPackage"

func clearASCAuth(t *testing.T) {
	t.Helper()
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
}

func TestVersionsViewSendsExactSupportedIncludeQuery(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const body = `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"build":{"data":null},"appStoreVersionSubmission":{"data":null}}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/version-1"},"included":[]}`
	tests := []struct {
		name    string
		flags   []string
		include string
	}{
		{"explicit", []string{"--include", "app,appStoreVersionLocalizations,build,appStoreVersionPhasedRelease"}, "app,appStoreVersionLocalizations,build,appStoreVersionPhasedRelease"},
		{"legacy flags", []string{"--include-build", "--include-submission"}, "build,appStoreVersionSubmission"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			include := test.include
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1" {
					t.Fatalf("request = %s %s, want GET /v1/appStoreVersions/version-1", req.Method, req.URL.Path)
				}
				if got := req.URL.Query().Get("include"); got != include {
					t.Fatalf("include query = %q, want %q", got, include)
				}
				return jsonResponse(http.StatusOK, body)
			})

			root := RootCommand("test")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append([]string{"versions", "view", "--version-id", "version-1", "--output", "json"}, test.flags...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			assertAnalyticsViewJSONEqual(t, stdout, body)
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestVersionsViewResolvesVersionFromAppAndVersionString(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	requests := make([]string, 0, 2)
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch req.URL.Path {
		case "/v1/apps/app-1/appStoreVersions":
			query := req.URL.Query()
			if got := query.Get("filter[versionString]"); got != "1.2.3" {
				t.Fatalf("version filter = %q, want 1.2.3", got)
			}
			if got := query.Get("filter[platform]"); got != "IOS" {
				t.Fatalf("platform filter = %q, want IOS", got)
			}
			if got := query.Get("limit"); got != "10" {
				t.Fatalf("limit = %q, want 10", got)
			}
			return jsonResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"}}]}`)
		case "/v1/appStoreVersions/version-1":
			return jsonResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"}}}`)
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
			return nil, nil
		}
	})

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"versions", "view",
			"--app", "app-1",
			"--version", "1.2.3",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	var result asc.AppStoreVersionResponse
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
	}
	if result.Data.ID != "version-1" || result.Data.Attributes.VersionString != "1.2.3" || result.Data.Attributes.Platform != "IOS" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	wantRequests := []string{
		"GET /v1/apps/app-1/appStoreVersions",
		"GET /v1/appStoreVersions/version-1",
	}
	if fmt.Sprint(requests) != fmt.Sprint(wantRequests) {
		t.Fatalf("requests = %v, want %v", requests, wantRequests)
	}
}

func TestVersionsViewSelectorValidationBeforeClient(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "direct ID with lookup selectors",
			args:       []string{"versions", "view", "--version-id", "version-1", "--app", "app-1", "--version", "1.2.3"},
			wantStderr: "--version-id cannot be combined with --app, --version, or --platform",
		},
		{
			name:       "explicit empty direct ID with lookup selectors",
			args:       []string{"versions", "view", "--version-id", " ", "--app", "app-1", "--version", "1.2.3"},
			wantStderr: "--version-id cannot be combined with --app, --version, or --platform",
		},
		{
			name:       "app without version",
			args:       []string{"versions", "view", "--app", "app-1"},
			wantStderr: "--version is required when resolving by app",
		},
		{
			name:       "version without app",
			args:       []string{"versions", "view", "--version", "1.2.3"},
			wantStderr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:       "version with whitespace app",
			args:       []string{"versions", "view", "--app", " ", "--version", "1.2.3"},
			wantStderr: "--app is required (or set ASC_APP_ID)",
		},
		{
			name:       "invalid platform",
			args:       []string{"versions", "view", "--app", "app-1", "--version", "1.2.3", "--platform", "ANDROID"},
			wantStderr: "versions view: --platform must be one of: IOS, MAC_OS, TV_OS, VISION_OS",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearASCAuth(t)
			clientFactoryCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, fmt.Errorf("client should not be created")
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "test"); code != rootcmd.ExitUsage {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			errorLine, _, _ := strings.Cut(stderr, "\n")
			if got := errorLine + "\n"; got != "Error: "+test.wantStderr+"\n" {
				t.Errorf("stderr error line = %q, want %q; full stderr = %q", got, "Error: "+test.wantStderr+"\n", stderr)
			}
			if clientFactoryCalls != 0 {
				t.Errorf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}

func TestVersionsViewIncludeValidationBeforeClient(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "unsupported include",
			args:       []string{"versions", "view", "--version-id", "version-1", "--include", "invalid"},
			wantStderr: "Error: versions view: --include must be one of: " + appStoreVersionIncludeValues + "\n",
		},
		{
			name:       "include with include build",
			args:       []string{"versions", "view", "--version-id", "version-1", "--include", "build", "--include-build"},
			wantStderr: "Error: --include cannot be used with --include-build or --include-submission\n",
		},
		{
			name:       "include with include submission",
			args:       []string{"versions", "view", "--version-id", "version-1", "--include", "appStoreVersionSubmission", "--include-submission"},
			wantStderr: "Error: --include cannot be used with --include-build or --include-submission\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearASCAuth(t)
			clientFactoryCalls := 0
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalls++
				return nil, fmt.Errorf("client should not be created")
			}))

			stdout, stderr := captureOutput(t, func() {
				if code := rootcmd.Run(test.args, "test"); code != rootcmd.ExitUsage {
					t.Errorf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			errorLine, _, _ := strings.Cut(stderr, "\n")
			if got := errorLine + "\n"; got != test.wantStderr {
				t.Errorf("stderr error line = %q, want %q; full stderr = %q", got, test.wantStderr, stderr)
			}
			if clientFactoryCalls != 0 {
				t.Errorf("client factory calls = %d, want 0", clientFactoryCalls)
			}
		})
	}
}

func TestVersionsReleaseAttributeValidationBeforeAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "create",
			args: []string{"versions", "create", "--app", "app-1", "--version", "1.0", "--release-type", "INVALID"},
			want: "Error: versions create: --release-type must be one of: " + releaseTypeValues,
		},
		{
			name: "update",
			args: []string{"versions", "update", "--version-id", "version-1", "--release-type", "INVALID"},
			want: "Error: versions update: --release-type must be one of: " + releaseTypeValues,
		},
		{
			name: "create date without a time",
			args: []string{"versions", "create", "--app", "app-1", "--version", "1.0", "--earliest-release-date", "2026-02-01"},
			want: "Error: versions create: --earliest-release-date must be an RFC3339 timestamp",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clearASCAuth(t)
			stdout, stderr := captureOutput(t, func() {
				code := rootcmd.Run(test.args, "test")
				if code != rootcmd.ExitUsage {
					t.Fatalf("exit code = %d, want %d", code, rootcmd.ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
			if strings.Contains(stderr, "missing authentication") {
				t.Fatalf("stderr = %q, validation must run before auth", stderr)
			}
		})
	}
}

func TestVersionsReleaseTypePayloads(t *testing.T) {
	tests := []struct {
		name                string
		args                []string
		method              string
		path                string
		responseID          string
		earliestReleaseDate string
	}{
		{
			name:       "create",
			args:       []string{"versions", "create", "--app", "app-1", "--version", "2.0", "--release-type", "scheduled", "--output", "json"},
			method:     http.MethodPost,
			path:       "/v1/appStoreVersions",
			responseID: "version-new",
		},
		{
			name:                "create with earliest release date",
			args:                []string{"versions", "create", "--app", "app-1", "--version", "2.0", "--release-type", "scheduled", "--earliest-release-date", "2026-02-01T08:00:00+00:00", "--output", "json"},
			method:              http.MethodPost,
			path:                "/v1/appStoreVersions",
			responseID:          "version-new",
			earliestReleaseDate: "2026-02-01T08:00:00+00:00",
		},
		{
			name:       "update",
			args:       []string{"versions", "update", "--version-id", "version-1", "--release-type", "scheduled", "--output", "json"},
			method:     http.MethodPatch,
			path:       "/v1/appStoreVersions/version-1",
			responseID: "version-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			requestErr := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != test.method || req.URL.Path != test.path {
					requestErr <- fmt.Errorf("request = %s %s, want %s %s", req.Method, req.URL.Path, test.method, test.path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				var request struct {
					Data struct {
						Attributes map[string]any `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
					requestErr <- fmt.Errorf("decode request: %w", err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				if got := request.Data.Attributes["releaseType"]; got != "SCHEDULED" {
					requestErr <- fmt.Errorf("releaseType = %#v, want SCHEDULED", got)
					http.Error(w, "invalid release type", http.StatusBadRequest)
					return
				}
				got, present := request.Data.Attributes["earliestReleaseDate"]
				if present != (test.earliestReleaseDate != "") || (present && got != test.earliestReleaseDate) {
					requestErr <- fmt.Errorf("earliestReleaseDate = %#v (present %t), want %q", got, present, test.earliestReleaseDate)
					http.Error(w, "invalid earliest release date", http.StatusBadRequest)
					return
				}
				requestErr <- nil
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":{"type":"appStoreVersions","id":"`+test.responseID+`","attributes":{"versionString":"2.0","platform":"IOS"}}}`)
			}))
			t.Cleanup(server.Close)

			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			}))

			root := RootCommand("test")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			var result struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(stdout), &result); err != nil {
				t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
			}
			if result.ID != test.responseID {
				t.Fatalf("response id = %q, want %q", result.ID, test.responseID)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			if err := <-requestErr; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestVersionsViewPreservesDownloadable(t *testing.T) {
	tests := []struct {
		name       string
		attributes string
		wantKey    bool
		wantValue  any
	}{
		{name: "false", attributes: `"downloadable":false`, wantKey: true, wantValue: false},
		{name: "true", attributes: `"downloadable":true`, wantKey: true, wantValue: true},
		{name: "null", attributes: `"downloadable":null`, wantKey: true},
		{name: "absent", attributes: `"copyright":"2026 Example"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			body := fmt.Sprintf(`{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS",%s}}}`, test.attributes)
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1" {
					t.Fatalf("request = %s %s, want GET /v1/appStoreVersions/version-1", req.Method, req.URL.Path)
				}
				return jsonResponse(http.StatusOK, body)
			})

			root := RootCommand("test")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse([]string{"versions", "view", "--version-id", "version-1", "--output", "json"}); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})
			if stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("unmarshal stdout: %v; stdout=%q", err, stdout)
			}
			data, ok := payload["data"].(map[string]any)
			if !ok {
				t.Fatalf("missing data envelope: %s", stdout)
			}
			attributes, ok := data["attributes"].(map[string]any)
			if !ok {
				t.Fatalf("missing attributes: %s", stdout)
			}
			value, ok := attributes["downloadable"]
			if ok != test.wantKey {
				t.Fatalf("downloadable present = %v, want %v; stdout=%q", ok, test.wantKey, stdout)
			}
			if !test.wantKey {
				return
			}
			if value != test.wantValue {
				t.Fatalf("downloadable = %v, want %v", value, test.wantValue)
			}
		})
	}
}

func TestVersionsReadsPrintAppleEnvelope(t *testing.T) {
	const attributes = `{"platform":"IOS","versionString":"2.5.1","appStoreState":"READY_FOR_SALE","appVersionState":"READY_FOR_DISTRIBUTION","copyright":"2026 Example","reviewType":"APP_STORE","releaseType":"AFTER_APPROVAL","earliestReleaseDate":null,"usesIdfa":null,"downloadable":true,"createdDate":"2026-10-24T05:58:07-07:00"}`
	const resource = `{"type":"appStoreVersions","id":"version-1","attributes":` + attributes + `,"relationships":{"build":{"data":{"type":"builds","id":"build-1"}}}}`
	tests := []struct {
		name       string
		args       []string
		path       string
		body       string
		wantStdout string
		wantStderr string
	}{
		{
			name: "view without include",
			args: []string{"versions", "view", "--version-id", "version-1", "--output", "json"},
			path: "/v1/appStoreVersions/version-1",
			body: `{"data":` + resource + `,"links":{"self":"https://example.test/v1/appStoreVersions/version-1"}}`,
		},
		{
			name: "view with include",
			args: []string{"versions", "view", "--version-id", "version-1", "--include", "build", "--output", "json"},
			path: "/v1/appStoreVersions/version-1",
			body: `{"data":` + resource + `,"included":[{"type":"builds","id":"build-1","attributes":{"version":"42"}}],"links":{"self":"https://example.test/v1/appStoreVersions/version-1"}}`,
		},
		{
			name:       "view table output has no envelope warning",
			args:       []string{"versions", "view", "--version-id", "version-1", "--output", "table"},
			path:       "/v1/appStoreVersions/version-1",
			body:       `{"data":` + resource + `}`,
			wantStdout: "2.5.1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			stubTransport(t, func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet || req.URL.Path != test.path {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL.Path, test.path)
				}
				return jsonResponse(http.StatusOK, test.body)
			})

			root := RootCommand("test")
			root.FlagSet.SetOutput(io.Discard)
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			switch {
			case slices.Contains(test.args, "table"):
				if !strings.Contains(stdout, test.wantStdout) {
					t.Fatalf("stdout = %q, want table containing %q", stdout, test.wantStdout)
				}
			case test.wantStdout == "":
				assertAnalyticsViewJSONEqual(t, stdout, test.body)
			default:
				assertAnalyticsViewJSONEqual(t, stdout, test.wantStdout)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}

func TestVersionsPhasedReleaseViewPrintsNullData(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	const body = `{"data":null,"links":{"self":"https://example.test/v1/appStoreVersions/version-1/appStoreVersionPhasedRelease"}}`
	stubTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/appStoreVersions/version-1/appStoreVersionPhasedRelease" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		return jsonResponse(http.StatusOK, body)
	})

	root := RootCommand("test")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"versions", "phased-release", "view", "--version-id", "version-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	assertAnalyticsViewJSONEqual(t, stdout, body)
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestVersionsReleaseReadsStateAfterAmbiguousFailure(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		attributes   string
		wantReads    int
		wantJSON     string
		wantStateErr bool
	}{
		{
			name:       "accepted release behind a 500",
			status:     http.StatusInternalServerError,
			attributes: `{"appVersionState":"PROCESSING_FOR_DISTRIBUTION"}`,
			wantReads:  1,
			wantJSON:   `{"releaseRequestId":"","versionId":"ver-1","reconciled":true,"versionState":"PROCESSING_FOR_DISTRIBUTION"}`,
		},
		{
			name:       "accepted release behind a 504 in the legacy state",
			status:     http.StatusGatewayTimeout,
			attributes: `{"appStoreState":"READY_FOR_SALE"}`,
			wantReads:  1,
			wantJSON:   `{"releaseRequestId":"","versionId":"ver-1","reconciled":true,"versionState":"READY_FOR_SALE"}`,
		},
		{
			name:         "pre-order awaiting release keeps the failure",
			status:       http.StatusInternalServerError,
			attributes:   `{"appVersionState":"PENDING_DEVELOPER_RELEASE","appStoreState":"PREORDER_READY_FOR_SALE"}`,
			wantReads:    2,
			wantStateErr: true,
		},
		{
			name:         "legacy pre-order listing keeps the failure",
			status:       http.StatusInternalServerError,
			attributes:   `{"appStoreState":"PREORDER_READY_FOR_SALE"}`,
			wantReads:    2,
			wantStateErr: true,
		},
		{
			name:         "unreleased version keeps the failure and is not resent",
			status:       http.StatusInternalServerError,
			attributes:   `{"appVersionState":"PENDING_DEVELOPER_RELEASE"}`,
			wantReads:    2,
			wantStateErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_MAX_RETRIES", "1")
			t.Setenv("ASC_BASE_DELAY", "1ms")
			t.Setenv("ASC_MAX_DELAY", "1ms")

			posts, reads := 0, 0
			installDefaultTransport(t, roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case req.Method == http.MethodPost && req.URL.Path == "/v1/appStoreVersionReleaseRequests":
					posts++
					return jsonResponse(test.status, `{"errors":[{"status":"500","code":"UNEXPECTED_ERROR","title":"An unexpected error occurred."}]}`)
				case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/ver-1":
					reads++
					return jsonResponse(http.StatusOK, fmt.Sprintf(`{"data":{"type":"appStoreVersions","id":"ver-1","attributes":%s}}`, test.attributes))
				default:
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
					return nil, nil
				}
			}))

			stdout, stderr, err := runRootCommand(t, []string{"versions", "release", "--version-id", "ver-1", "--confirm", "--output", "json"})
			if posts != 1 || reads != test.wantReads {
				t.Fatalf("posts = %d, reads = %d; want 1 and %d", posts, reads, test.wantReads)
			}
			if test.wantStateErr {
				if err == nil || !strings.Contains(err.Error(), "versions release:") {
					t.Fatalf("expected the release failure, got %v", err)
				}
				if stdout != "" {
					t.Fatalf("expected no result on stdout, got %q", stdout)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected the accepted release to succeed: %v", err)
			}
			if strings.TrimSpace(stdout) != test.wantJSON {
				t.Fatalf("stdout = %q, want %q", stdout, test.wantJSON)
			}
			if !strings.Contains(stderr, "ver-1") || !strings.Contains(stderr, "the release was accepted") {
				t.Fatalf("expected a stderr note about the failed response, got %q", stderr)
			}
		})
	}
}
