package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type buildsExpireAllVersionFixtureBuild struct {
	id                  string
	buildNumber         string
	uploadedDate        string
	expired             bool
	preReleaseVersionID string
}

// buildsExpireAllVersionServer serves a mixed-version build history and honors
// filter[preReleaseVersion] the way App Store Connect does, so a command that
// forgets the version filter sees every version's builds.
func buildsExpireAllVersionServer(t *testing.T, patched *[]string, mu *sync.Mutex) *httptest.Server {
	t.Helper()

	fixture := []buildsExpireAllVersionFixtureBuild{
		{id: "build-latest-123", buildNumber: "12", uploadedDate: "2026-03-01T00:00:00Z", preReleaseVersionID: "prv-123"},
		{id: "build-other-200", buildNumber: "20", uploadedDate: "2026-02-15T00:00:00Z", preReleaseVersionID: "prv-200"},
		{id: "build-old-123", buildNumber: "11", uploadedDate: "2026-02-01T00:00:00Z", preReleaseVersionID: "prv-123"},
		{id: "build-expired-123", buildNumber: "10", uploadedDate: "2026-01-15T00:00:00Z", expired: true, preReleaseVersionID: "prv-123"},
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := req.URL.Query()

		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/preReleaseVersions":
			if got := query.Get("filter[app]"); got != "app-1" {
				t.Errorf("pre-release lookup filter[app] = %q, want app-1", got)
			}
			if got := query.Get("filter[version]"); got != "1.2.3" {
				t.Errorf("pre-release lookup filter[version] = %q, want 1.2.3", got)
			}
			_, _ = io.WriteString(w, `{"data":[{"type":"preReleaseVersions","id":"prv-123","attributes":{"version":"1.2.3","platform":"IOS"}}],"links":{"next":""}}`)

		case req.Method == http.MethodGet && req.URL.Path == "/v1/builds":
			if got := query.Get("filter[app]"); got != "app-1" {
				t.Errorf("builds filter[app] = %q, want app-1", got)
			}
			if got := query.Get("sort"); got != "-uploadedDate" {
				t.Errorf("builds sort = %q, want -uploadedDate", got)
			}
			if got := query.Get("limit"); got != "200" {
				t.Errorf("builds limit = %q, want 200", got)
			}

			allowed := map[string]bool{}
			for _, id := range strings.Split(query.Get("filter[preReleaseVersion]"), ",") {
				if trimmed := strings.TrimSpace(id); trimmed != "" {
					allowed[trimmed] = true
				}
			}

			items := make([]string, 0, len(fixture))
			for _, build := range fixture {
				if len(allowed) > 0 && !allowed[build.preReleaseVersionID] {
					continue
				}
				payload, err := json.Marshal(map[string]any{
					"type": "builds",
					"id":   build.id,
					"attributes": map[string]any{
						"version":      build.buildNumber,
						"uploadedDate": build.uploadedDate,
						"expired":      build.expired,
					},
				})
				if err != nil {
					t.Errorf("marshal fixture build %s: %v", build.id, err)
					continue
				}
				items = append(items, string(payload))
			}
			_, _ = io.WriteString(w, `{"data":[`+strings.Join(items, ",")+`],"links":{"next":""}}`)

		case req.Method == http.MethodPatch && strings.HasPrefix(req.URL.Path, "/v1/builds/"):
			buildID := strings.TrimPrefix(req.URL.Path, "/v1/builds/")
			var payload struct {
				Data struct {
					Type       string `json:"type"`
					ID         string `json:"id"`
					Attributes struct {
						Expired bool `json:"expired"`
					} `json:"attributes"`
				} `json:"data"`
			}
			if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
				t.Errorf("decode expire payload for %s: %v", buildID, err)
			}
			if payload.Data.Type != "builds" || payload.Data.ID != buildID || !payload.Data.Attributes.Expired {
				t.Errorf("expire payload for %s = %+v, want expired builds resource", buildID, payload.Data)
			}

			mu.Lock()
			*patched = append(*patched, buildID)
			mu.Unlock()

			_, _ = io.WriteString(w, `{"data":{"type":"builds","id":"`+buildID+`","attributes":{"expired":true}},"links":{}}`)

		default:
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
			http.Error(w, "unexpected request", http.StatusInternalServerError)
		}
	}))
}

func setBuildsExpireAllTestClient(t *testing.T, server *httptest.Server) {
	t.Helper()

	client := newReviewTestServerClient(t, server)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))
}

func TestBuildsExpireAllVersionKeepsLatestOfThatVersionOnly(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var mu sync.Mutex
	patched := []string{}
	server := buildsExpireAllVersionServer(t, &patched, &mu)
	t.Cleanup(server.Close)
	setBuildsExpireAllTestClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "expire-all",
			"--app", "app-1",
			"--version", "1.2.3",
			"--keep-latest", "1",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	mu.Lock()
	gotPatched := append([]string(nil), patched...)
	mu.Unlock()
	if len(gotPatched) != 1 || gotPatched[0] != "build-old-123" {
		t.Fatalf("expired builds = %v, want only build-old-123", gotPatched)
	}

	var result asc.BuildExpireAllResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.SelectedCount != 1 || result.ExpiredCount != 1 || len(result.Failures) != 0 {
		t.Fatalf("unexpected summary: %+v", result)
	}
	if len(result.Builds) != 1 || result.Builds[0].ID != "build-old-123" {
		t.Fatalf("expired builds in receipt = %+v, want only build-old-123", result.Builds)
	}
	if result.SkippedExpiredCount == nil || *result.SkippedExpiredCount != 1 {
		t.Fatalf("skippedExpiredCount = %v, want 1", result.SkippedExpiredCount)
	}
}

func TestBuildsExpireAllVersionCombinesWithOlderThan(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var mu sync.Mutex
	patched := []string{}
	server := buildsExpireAllVersionServer(t, &patched, &mu)
	t.Cleanup(server.Close)
	setBuildsExpireAllTestClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "expire-all",
			"--app", "app-1",
			"--version", "1.2.3",
			"--older-than", "2026-02-10",
			"--confirm",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	mu.Lock()
	gotPatched := append([]string(nil), patched...)
	mu.Unlock()
	if len(gotPatched) != 1 || gotPatched[0] != "build-old-123" {
		t.Fatalf("expired builds = %v, want only build-old-123", gotPatched)
	}

	var result asc.BuildExpireAllResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.Version == nil || *result.Version != "1.2.3" {
		t.Fatalf("receipt version = %v, want 1.2.3", result.Version)
	}
	if result.SelectedCount != 1 || result.ExpiredCount != 1 {
		t.Fatalf("unexpected summary: %+v", result)
	}
}

func TestBuildsExpireAllVersionDryRunDoesNotExpire(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	var mu sync.Mutex
	patched := []string{}
	server := buildsExpireAllVersionServer(t, &patched, &mu)
	t.Cleanup(server.Close)
	setBuildsExpireAllTestClient(t, server)

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"builds", "expire-all",
			"--app", "app-1",
			"--version", "1.2.3",
			"--keep-latest", "1",
			"--dry-run",
			"--output", "json",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("run error: %v; stderr=%q", runErr, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	mu.Lock()
	gotPatched := append([]string(nil), patched...)
	mu.Unlock()
	if len(gotPatched) != 0 {
		t.Fatalf("expired builds = %v, want none in dry run", gotPatched)
	}

	var result asc.BuildExpireAllResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if !result.DryRun || result.SelectedCount != 1 || result.ExpiredCount != 0 {
		t.Fatalf("unexpected dry-run summary: %+v", result)
	}
	if len(result.Builds) != 1 || result.Builds[0].ID != "build-old-123" || result.Builds[0].Expired != nil {
		t.Fatalf("dry-run builds = %+v, want build-old-123 without expired receipt", result.Builds)
	}
}

func TestBuildsExpireAllVersionUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "missing confirm",
			args:       []string{"builds", "expire-all", "--app", "app-1", "--version", "1.2.3", "--keep-latest", "1"},
			wantStderr: "Error: --confirm is required to expire builds",
		},
		{
			name:       "empty version",
			args:       []string{"builds", "expire-all", "--app", "app-1", "--version", "", "--keep-latest", "1", "--confirm"},
			wantStderr: "--version must not be empty",
		},
		{
			name:       "version alone does not select builds",
			args:       []string{"builds", "expire-all", "--app", "app-1", "--version", "1.2.3", "--confirm"},
			wantStderr: "Error: --older-than or --keep-latest is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			clientFactoryCalled := false
			t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
				clientFactoryCalled = true
				return nil, errors.New("client should not be created for a usage error")
			}))

			root := RootCommand("1.2.3")
			root.FlagSet.SetOutput(io.Discard)

			var runErr error
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(test.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				runErr = root.Run(context.Background())
			})

			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("run error = %v, want usage error (flag.ErrHelp)", runErr)
			}
			if clientFactoryCalled {
				t.Fatal("client factory called, want validation before any request")
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, test.wantStderr)
			}
			if strings.Contains(stdout, `"expiredCount"`) {
				t.Fatalf("stdout = %q, want no expiration receipt", stdout)
			}
		})
	}
}
