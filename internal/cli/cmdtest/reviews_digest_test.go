package cmdtest

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

const (
	reviewsDigestPage1 = `{"data":[
		{"type":"customerReviews","id":"r1","attributes":{"rating":5,"title":"Great","body":"Love it","createdDate":"2026-10-20T10:00:00-07:00","territory":"USA"},"relationships":{"response":{"data":{"type":"customerReviewResponses","id":"resp-1"}}}},
		{"type":"customerReviews","id":"r2","attributes":{"rating":1,"title":"Crash","body":"Crashes on launch","createdDate":"2026-10-19T09:00:00Z","territory":"GBR"},"relationships":{"response":{"data":null}}}
	],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews?cursor=page2"}}`
	reviewsDigestPage2 = `{"data":[
		{"type":"customerReviews","id":"r2","attributes":{"rating":1,"title":"Crash","body":"Crashes on launch","createdDate":"2026-10-19T09:00:00Z","territory":"GBR"},"relationships":{"response":{"data":null}}},
		{"type":"customerReviews","id":"r3","attributes":{"rating":2,"title":"Slow","body":"Too slow","createdDate":"2026-10-18T08:00:00Z","territory":"USA"}},
		{"type":"customerReviews","id":"r4","attributes":{"rating":1,"title":"Old","body":"Before cursor","createdDate":"2026-10-15T00:00:00Z","territory":"USA"}}
	],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/customerReviews?cursor=page3"}}`
)

func newReviewsDigestServer(t *testing.T, pages map[string]string, versionOverrides ...map[string]string) (func() (*asc.Client, error), *[]string) {
	t.Helper()
	versionPages := map[string]string{
		"/v1/apps/app-1/appStoreVersions?":                 `{"data":[{"type":"appStoreVersions","id":"v1","attributes":{"versionString":"1.0","platform":"IOS"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/apps/app-1/appStoreVersions?cursor=versions2"}}`,
		"/v1/apps/app-1/appStoreVersions?versions2":        `{"data":[{"type":"appStoreVersions","id":"v2","attributes":{"versionString":"1.0","platform":"MAC_OS"}}]}`,
		"/v1/appStoreVersions/v1/customerReviews?":         `{"data":[{"type":"customerReviews","id":"r1","attributes":{"createdDate":"2026-10-20T10:00:00-07:00"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/v1/customerReviews?cursor=reviews2"}}`,
		"/v1/appStoreVersions/v1/customerReviews?reviews2": `{"data":[{"type":"customerReviews","id":"r1","attributes":{"createdDate":"2026-10-20T10:00:00-07:00"}},{"type":"customerReviews","id":"r2","attributes":{"createdDate":"2026-10-19T09:00:00Z"}},{"type":"customerReviews","id":"r4","attributes":{"createdDate":"2026-10-15T00:00:00Z"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/v1/customerReviews?cursor=reviews3"}}`,
		"/v1/appStoreVersions/v2/customerReviews?":         `{"data":[{"type":"customerReviews","id":"later","attributes":{"createdDate":"2026-10-22T00:00:00Z"}},{"type":"customerReviews","id":"r1","attributes":{"createdDate":"2026-10-20T10:00:00-07:00"}},{"type":"customerReviews","id":"r3","attributes":{"createdDate":"2026-10-18T08:00:00Z"}}]}`,
	}
	for _, overrides := range versionOverrides {
		for path, body := range overrides {
			versionPages[path] = body
		}
	}
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		requests = append(requests, req.URL.RequestURI())
		if req.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		cursor := req.URL.Query().Get("cursor")
		if cursor == "" && req.URL.Path != "/v1/apps/app-1/customerReviews" {
			if req.URL.Query().Get("limit") != "200" {
				t.Errorf("expected limit=200: %s", req.URL.RequestURI())
			}
			if strings.HasSuffix(req.URL.Path, "/customerReviews") && req.URL.Query().Get("sort") != "-createdDate" {
				t.Errorf("expected newest-first reviews: %s", req.URL.RequestURI())
			}
		}
		body, ok := versionPages[req.URL.Path+"?"+cursor]
		if req.URL.Path == "/v1/apps/app-1/customerReviews" {
			body, ok = pages[cursor]
		}
		if !ok {
			t.Errorf("unexpected page request: %s", req.URL.RequestURI())
			body = `{"data":[]}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return stubServerClient(t, server), &requests
}

func runReviewsDigest(t *testing.T, client func() (*asc.Client, error), args ...string) (int, string, string) {
	t.Helper()
	stdout, stderr, code := runWithDeps(t, client, append([]string{"reviews", "digest"}, args...)...)
	return code, stdout, stderr
}

func TestReviewsDigestStopsPaginatingAtCursorAndAggregates(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	client, requests := newReviewsDigestServer(t, map[string]string{"": reviewsDigestPage1, "page2": reviewsDigestPage2})

	code, stdout, stderr := runReviewsDigest(t, client, "--app", "app-1", "--since", "2026-10-16", "--lowest", "2", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if len(*requests) != 7 {
		t.Fatalf("requests = %v, want two app pages, two version pages, and three version-review pages", *requests)
	}
	first := (*requests)[0]
	for _, want := range []string{"sort=-createdDate", "include=response", "limit=200"} {
		if !strings.Contains(first, want) {
			t.Fatalf("first request query %q missing %q", first, want)
		}
	}

	var got asc.CustomerReviewDigestResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode stdout %q: %v", stdout, err)
	}
	want := asc.CustomerReviewDigestResult{
		AppID:         "app-1",
		Since:         "2026-10-16T00:00:00Z",
		CursorSource:  "since",
		Cursor:        "2026-10-20T10:00:00-07:00",
		Total:         3,
		AverageRating: 2.67,
		Unanswered:    2,
		Ratings: []asc.CustomerReviewDigestRating{
			{Rating: 5, Count: 1}, {Rating: 4, Count: 0}, {Rating: 3, Count: 0}, {Rating: 2, Count: 1}, {Rating: 1, Count: 1},
		},
		Territories: []asc.CustomerReviewDigestGroup{
			{Territory: "USA", Count: 2, AverageRating: 3.5, Unanswered: 1},
			{Territory: "GBR", Count: 1, AverageRating: 1, Unanswered: 1},
		},
		Versions: []asc.CustomerReviewDigestVersion{
			{VersionID: "v1", Version: "1.0", Platform: "IOS", Count: 2, AverageRating: 3, Unanswered: 1},
			{VersionID: "v2", Version: "1.0", Platform: "MAC_OS", Count: 1, AverageRating: 2, Unanswered: 1},
		},
		LowestRated: []asc.CustomerReviewDigestReview{
			{ID: "r2", Rating: 1, Title: "Crash", Body: "Crashes on launch", Territory: "GBR", CreatedDate: "2026-10-19T09:00:00Z"},
			{ID: "r3", Rating: 2, Title: "Slow", Body: "Too slow", Territory: "USA", CreatedDate: "2026-10-18T08:00:00Z"},
		},
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("digest =\n%s\nwant\n%s", gotJSON, wantJSON)
	}
	for _, format := range []string{"table", "markdown"} {
		code, stdout, stderr := runReviewsDigest(t, client, "--app", "app-1", "--since", "2026-10-16", "--lowest", "2", "--output", format)
		rows := strings.Join(strings.Fields(strings.NewReplacer("|", " ", "│", " ").Replace(stdout)), " ")
		if code != rootcmd.ExitSuccess || stderr != "" || !strings.Contains(rows, "1.0 IOS v1 2 3.00 1") || !strings.Contains(rows, "1.0 MAC_OS v2 1 2.00 1") {
			t.Fatalf("format=%s code=%d stdout=%q stderr=%q", format, code, stdout, stderr)
		}
	}
}

func TestReviewsDigestStopsVersionLookupOnceAllReviewsMapped(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	client, requests := newReviewsDigestServer(t, map[string]string{"": reviewsDigestPage1, "page2": reviewsDigestPage2}, map[string]string{
		"/v1/appStoreVersions/v1/customerReviews?":         `{"data":[{"type":"customerReviews","id":"r1","attributes":{"createdDate":"2026-10-20T10:00:00-07:00"}},{"type":"customerReviews","id":"r2","attributes":{"createdDate":"2026-10-19T09:00:00Z"}},{"type":"customerReviews","id":"r3","attributes":{"createdDate":"2026-10-18T08:00:00Z"}}],"links":{"next":"https://api.appstoreconnect.apple.com/v1/appStoreVersions/v1/customerReviews?cursor=reviews2"}}`,
		"/v1/appStoreVersions/v1/customerReviews?reviews2": "invalid-json",
		"/v1/apps/app-1/appStoreVersions?versions2":        "invalid-json",
	})
	code, stdout, stderr := runReviewsDigest(t, client, "--app", "app-1", "--since", "2026-10-16", "--output", "json")
	if code != rootcmd.ExitSuccess || stderr != "" {
		t.Fatalf("completed associations must not read irrelevant pages: code=%d stdout=%q stderr=%q requests=%v", code, stdout, stderr, *requests)
	}
	var got asc.CustomerReviewDigestResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatal(err)
	}
	if len(*requests) != 4 || got.Total != 3 || got.UnassignedVersionReviews != 0 || len(got.Versions) != 1 || got.Versions[0].Count != 3 || got.Versions[0].AverageRating != 2.67 || got.Versions[0].Unanswered != 2 {
		t.Fatalf("digest=%+v requests=%v", got, *requests)
	}
}

func TestReviewsDigestReportsUnassignedVersions(t *testing.T) {
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	client, _ := newReviewsDigestServer(t, map[string]string{"": reviewsDigestPage1, "page2": reviewsDigestPage2}, map[string]string{"/v1/apps/app-1/appStoreVersions?": `{"data":[]}`})
	for _, format := range []string{"json", "table", "markdown"} {
		code, stdout, stderr := runReviewsDigest(t, client, "--app", "app-1", "--since", "2026-10-16", "--output", format)
		if code != rootcmd.ExitSuccess || stderr != "" {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
		if format == "json" {
			var got struct {
				Total                    int `json:"total"`
				UnassignedVersionReviews int `json:"unassignedVersionReviews"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatal(err)
			}
			if got.Total != 3 || got.UnassignedVersionReviews != 3 {
				t.Fatalf("unassociated reviews must remain visible: %s", stdout)
			}
		} else if rows := strings.Join(strings.Fields(strings.NewReplacer("|", " ", "│", " ").Replace(stdout)), " "); !strings.Contains(rows, "Reviews without version 3") {
			t.Fatalf("unassociated reviews missing from %s output: %s", format, stdout)
		}
	}
}

func TestReviewsDigestVersionFailurePreservesCursor(t *testing.T) {
	for _, path := range []string{"/v1/apps/app-1/appStoreVersions?versions2", "/v1/appStoreVersions/v1/customerReviews?reviews2"} {
		t.Run(path, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			client, _ := newReviewsDigestServer(t, map[string]string{"": reviewsDigestPage1, "page2": reviewsDigestPage2}, map[string]string{path: "invalid-json"})
			statePath := filepath.Join(t.TempDir(), "cursor.json")
			original := `{"cursor":"2026-10-16T00:00:00Z"}`
			if err := os.WriteFile(statePath, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			code, stdout, stderr := runReviewsDigest(t, client, "--app", "app-1", "--state-file", statePath, "--write-state", "--output", "json")
			after, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatal(err)
			}
			if code != rootcmd.ExitError || stdout != "" || string(after) != original || !strings.Contains(stderr, "failed to parse response") {
				t.Fatalf("code=%d stdout=%q stderr=%q cursor=%s", code, stdout, stderr, after)
			}
		})
	}
}

func TestReviewsDigestStateFileCursor(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		args       []string
		wantSource string
		wantSince  string
		wantState  string
	}{
		{
			name:       "existing state file supplies the cursor and is advanced",
			state:      `{"cursor":"2026-10-18T08:00:00Z"}`,
			args:       []string{"--since", "2026-01-01", "--write-state"},
			wantSource: "stateFile",
			wantSince:  "2026-10-18T08:00:00Z",
			wantState:  "2026-10-20T10:00:00-07:00",
		},
		{
			name:       "missing state file is seeded from --since",
			args:       []string{"--since", "2026-10-16T00:00:00Z", "--write-state"},
			wantSource: "since",
			wantSince:  "2026-10-16T00:00:00Z",
			wantState:  "2026-10-20T10:00:00-07:00",
		},
		{
			name:       "state file is not written without --write-state",
			state:      `{"cursor":"2026-10-18T08:00:00Z"}`,
			wantSource: "stateFile",
			wantSince:  "2026-10-18T08:00:00Z",
			wantState:  "2026-10-18T08:00:00Z",
		},
		{
			name:       "no newer reviews keeps the exact cursor",
			state:      `{"cursor":"2026-10-21T00:00:00.5Z"}`,
			args:       []string{"--write-state"},
			wantSource: "stateFile",
			wantSince:  "2026-10-21T00:00:00.5Z",
			wantState:  "2026-10-21T00:00:00.5Z",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			client, requests := newReviewsDigestServer(t, map[string]string{"": reviewsDigestPage1, "page2": reviewsDigestPage2})

			statePath := filepath.Join(t.TempDir(), "digest-state.json")
			if test.state != "" {
				if err := os.WriteFile(statePath, []byte(test.state), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			args := append([]string{"--app", "app-1", "--state-file", statePath, "--output", "json"}, test.args...)
			code, stdout, stderr := runReviewsDigest(t, client, args...)
			if code != rootcmd.ExitSuccess {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr)
			}
			if len(*requests) == 0 {
				t.Fatal("expected at least one request")
			}
			var got asc.CustomerReviewDigestResult
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("decode stdout %q: %v", stdout, err)
			}
			if got.CursorSource != test.wantSource || got.Since != test.wantSince {
				t.Fatalf("cursorSource=%q since=%q, want %q %q", got.CursorSource, got.Since, test.wantSource, test.wantSince)
			}

			data, err := os.ReadFile(statePath)
			if err != nil {
				t.Fatalf("read state: %v", err)
			}
			var state struct {
				Cursor string `json:"cursor"`
			}
			if err := json.Unmarshal(data, &state); err != nil {
				t.Fatalf("decode state %q: %v", data, err)
			}
			if state.Cursor != test.wantState {
				t.Fatalf("state cursor = %q, want %q", state.Cursor, test.wantState)
			}
		})
	}
}

func TestReviewsDigestRejectsInvalidInputBeforeRequests(t *testing.T) {
	dir := t.TempDir()
	emptyState := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyState, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	missingState := filepath.Join(dir, "missing.json")

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStderr string
	}{
		{name: "no cursor", args: []string{"--app", "app-1"}, wantCode: rootcmd.ExitUsage, wantStderr: "--since or --state-file is required"},
		{name: "missing state file without since", args: []string{"--app", "app-1", "--state-file", missingState}, wantCode: rootcmd.ExitUsage, wantStderr: "--since is required when --state-file does not exist"},
		{name: "write state without state file", args: []string{"--app", "app-1", "--since", "2026-10-01", "--write-state"}, wantCode: rootcmd.ExitUsage, wantStderr: "--write-state requires --state-file"},
		{name: "invalid since", args: []string{"--app", "app-1", "--since", "last week"}, wantCode: rootcmd.ExitUsage, wantStderr: "--since must be an RFC3339 timestamp or YYYY-MM-DD date"},
		{name: "negative lowest", args: []string{"--app", "app-1", "--since", "2026-10-01", "--lowest", "-1"}, wantCode: rootcmd.ExitUsage, wantStderr: "--lowest must be 0 or greater"},
		{name: "missing app", args: []string{"--since", "2026-10-01"}, wantCode: rootcmd.ExitUsage, wantStderr: "--app is required"},
		{name: "state file without cursor", args: []string{"--app", "app-1", "--state-file", emptyState}, wantCode: rootcmd.ExitError, wantStderr: "has no valid cursor"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
			t.Setenv("ASC_APP_ID", "")
			client, requests := newReviewsDigestServer(t, map[string]string{})

			code, stdout, stderr := runReviewsDigest(t, client, test.args...)
			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, test.wantCode, stderr)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if len(*requests) != 0 {
				t.Fatalf("expected no requests, got %v", *requests)
			}
		})
	}
}
