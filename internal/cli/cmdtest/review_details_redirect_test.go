package cmdtest

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReviewDetailsSelectedReadsRefuseRedirects(t *testing.T) {
	tests := []struct {
		name                string
		appEnv              string
		appChanges          bool
		args                []string
		redirectPath        string
		redirectAfterSparse bool
		wantRequests        []string
		wantCode            int
	}{
		{
			name:         "default version selection",
			wantCode:     cmd.ExitError,
			args:         []string{"review", "details-get", "--app", "100000001", "--output", "json"},
			redirectPath: "/v1/apps/100000001/appStoreVersions",
			wantRequests: []string{"GET /v1/apps/100000001/appStoreVersions"},
		},
		{
			name:         "selected version detail",
			wantCode:     cmd.ExitError,
			args:         []string{"review", "details-get", "--app", "100000001", "--version-id", "version-1", "--output", "json"},
			redirectPath: "/v1/appStoreVersions/version-1/appStoreReviewDetail",
			wantRequests: []string{"GET /v1/appStoreVersions/version-1", "GET /v1/appStoreVersions/version-1/appStoreReviewDetail"},
		},
		{
			name:         "configured version detail",
			wantCode:     cmd.ExitError,
			appEnv:       "100000001",
			args:         []string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"},
			redirectPath: "/v1/appStoreVersions/version-1/appStoreReviewDetail",
			wantRequests: []string{"GET /v1/appStoreVersions/version-1", "GET /v1/appStoreVersions/version-1/appStoreReviewDetail"},
		},
		{
			name:                "direct detail content",
			wantCode:            cmd.ExitError,
			appEnv:              "100000001",
			args:                []string{"review", "details-get", "--id", "detail-1", "--output", "json"},
			redirectPath:        "/v1/appStoreReviewDetails/detail-1",
			redirectAfterSparse: true,
			wantRequests: []string{
				"GET /v1/appStoreReviewDetails/detail-1",
				"GET /v1/appStoreVersions/version-1",
				"GET /v1/appStoreReviewDetails/detail-1",
			},
		},
		{
			name:         "update ownership proof",
			wantCode:     cmd.ExitError,
			appEnv:       "100000001",
			args:         []string{"review", "details-update", "--id", "detail-1", "--notes", "Updated"},
			redirectPath: "/v1/appStoreVersions/version-1",
			wantRequests: []string{"GET /v1/appStoreReviewDetails/detail-1", "GET /v1/appStoreVersions/version-1"},
		},
		{
			name:         "configured read keeps initial app selection",
			wantCode:     cmd.ExitError,
			appEnv:       "100000001",
			appChanges:   true,
			args:         []string{"review", "details-for-version", "--version-id", "version-1", "--output", "json"},
			wantRequests: []string{"GET /v1/appStoreVersions/version-1"},
		},
		{
			name:       "configured create keeps initial app selection",
			wantCode:   cmd.ExitError,
			appEnv:     "100000001",
			appChanges: true,
			args: []string{
				"review", "details-create", "--version-id", "version-1",
				"--contact-first-name", "Dev", "--contact-phone", "+1 408 555 0100",
			},
			wantRequests: []string{"GET /v1/appStoreVersions/version-1"},
		},
		{
			name:     "create conflict recovery",
			wantCode: cmd.ExitConflict,
			appEnv:   "100000001",
			args: []string{
				"review", "details-create", "--version-id", "version-1",
				"--contact-first-name", "Dev", "--contact-phone", "+1 408 555 0100",
				"--if-exists", "update", "--output", "json",
			},
			redirectPath: "/v1/appStoreVersions/version-1/appStoreReviewDetail",
			wantRequests: []string{
				"GET /v1/appStoreVersions/version-1",
				"POST /v1/appStoreReviewDetails",
				"GET /v1/appStoreVersions/version-1/appStoreReviewDetail",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", test.appEnv)

			var mu sync.Mutex
			var requests []string
			sentinelReached := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				if r.URL.Path == "/fixture/redirect-sentinel" {
					sentinelReached = true
					http.Error(w, "unexpected redirected review read", http.StatusInternalServerError)
					return
				}
				if r.URL.Path == test.redirectPath && (!test.redirectAfterSparse || r.URL.Query().Get("include") == "") {
					http.Redirect(w, r, "/fixture/redirect-sentinel", http.StatusTemporaryRedirect)
					return
				}

				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreVersions/version-1":
					w.Header().Set("Content-Type", "application/json")
					appID := "100000001"
					if test.appChanges {
						appID = "100000002"
					}
					_, _ = w.Write([]byte(`{"data":{"type":"appStoreVersions","id":"version-1","attributes":{"versionString":"1.2.3","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"` + appID + `"}}}}}`))
				case r.Method == http.MethodGet && r.URL.Path == "/v1/appStoreReviewDetails/detail-1":
					if r.URL.Query().Get("include") == "appStoreVersion" {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write([]byte(`{"data":{"type":"appStoreReviewDetails","id":"detail-1","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"version-1"}}}}}`))
						return
					}
					http.Error(w, "unexpected unrestricted detail read", http.StatusInternalServerError)
				case r.Method == http.MethodPost && r.URL.Path == "/v1/appStoreReviewDetails":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"errors":[{"status":"409","code":"STATE_ERROR.ALREADY_EXISTS","title":"Resource already exists."}]}`))
				default:
					http.Error(w, "unexpected review request", http.StatusInternalServerError)
				}
			}))
			t.Cleanup(server.Close)

			deps, stdout, stderr := newTestDeps(t, stubServerClient(t, server))
			appLookups := 0
			if test.appChanges {
				lookupEnv := deps.LookupEnv
				deps.LookupEnv = func(name string) (string, bool) {
					if name != "ASC_APP_ID" {
						return lookupEnv(name)
					}
					appLookups++
					if appLookups == 1 {
						return test.appEnv, true
					}
					return "", true
				}
			}
			code := cmd.RunWithDeps(test.args, "1.2.3", deps)
			if test.appChanges && (appLookups != 1 || !strings.Contains(stderr.String(), "belongs to app")) {
				t.Fatalf("app lookups=%d stderr=%q, want one lookup and ownership refusal", appLookups, stderr)
			}
			mu.Lock()
			defer mu.Unlock()
			if code != test.wantCode || stdout.Len() != 0 {
				t.Fatalf("%s exit=%d stdout=%q stderr=%q, want error and no output", test.name, code, stdout, stderr)
			}
			if sentinelReached {
				t.Fatalf("redirected %s read followed the inert sentinel: %v", test.name, requests)
			}
			if !slices.Equal(requests, test.wantRequests) {
				t.Fatalf("requests=%v, want=%v", requests, test.wantRequests)
			}
		})
	}
}
