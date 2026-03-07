package snitch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"bug", true},
		{"friction", true},
		{"feature-request", true},
		{"Bug", false},
		{"critical", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isValidSeverity(tt.input); got != tt.want {
				t.Errorf("isValidSeverity(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveGitHubToken(t *testing.T) {
	t.Run("GITHUB_TOKEN", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "gh-token-123")
		t.Setenv("GH_TOKEN", "")
		if got := resolveGitHubToken(); got != "gh-token-123" {
			t.Errorf("got %q, want %q", got, "gh-token-123")
		}
	})

	t.Run("GH_TOKEN fallback", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "gh-fallback-456")
		if got := resolveGitHubToken(); got != "gh-fallback-456" {
			t.Errorf("got %q, want %q", got, "gh-fallback-456")
		}
	})

	t.Run("GITHUB_TOKEN takes precedence", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "primary")
		t.Setenv("GH_TOKEN", "secondary")
		if got := resolveGitHubToken(); got != "primary" {
			t.Errorf("got %q, want %q", got, "primary")
		}
	})

	t.Run("neither set", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		if got := resolveGitHubToken(); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestIssueTitle(t *testing.T) {
	tests := []struct {
		severity string
		desc     string
		want     string
	}{
		{"bug", "crashes command fails", "crashes command fails"},
		{"friction", "need --output table everywhere", "Friction: need --output table everywhere"},
		{"feature-request", "add asc snitch command", "Feature: add asc snitch command"},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			e := LogEntry{Severity: tt.severity, Description: tt.desc}
			if got := issueTitle(e); got != tt.want {
				t.Errorf("issueTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueBody(t *testing.T) {
	e := LogEntry{
		Description: "crashes --app doesn't support bundle ID",
		Repro:       `asc crashes --app "com.example.app"`,
		Expected:    "Bundle ID should resolve",
		Actual:      "Error: AppId is invalid",
		Severity:    "bug",
		ASCVersion:  "0.37.2",
		OS:          "darwin/arm64",
	}

	body := issueBody(e)

	checks := []string{
		"## Summary",
		"crashes --app doesn't support bundle ID",
		"## Reproduction",
		`asc crashes --app "com.example.app"`,
		"## Expected behavior",
		"Bundle ID should resolve",
		"## Actual behavior",
		"Error: AppId is invalid",
		"## Environment",
		"0.37.2",
		"darwin/arm64",
		"`asc snitch`",
	}

	for _, check := range checks {
		if !strings.Contains(body, check) {
			t.Errorf("issueBody() missing %q", check)
		}
	}
}

func TestIssueBodyMinimal(t *testing.T) {
	e := LogEntry{
		Description: "something broke",
		Severity:    "friction",
		ASCVersion:  "0.37.0",
		OS:          "linux/amd64",
	}

	body := issueBody(e)

	// Should contain summary and environment but not reproduction/expected/actual sections.
	if !strings.Contains(body, "## Summary") {
		t.Error("missing Summary section")
	}
	if strings.Contains(body, "## Reproduction") {
		t.Error("should not contain Reproduction when repro is empty")
	}
	if strings.Contains(body, "## Expected behavior") {
		t.Error("should not contain Expected when expected is empty")
	}
	if strings.Contains(body, "## Actual behavior") {
		t.Error("should not contain Actual when actual is empty")
	}
}

func TestSearchIssues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search/issues" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		q := r.URL.Query().Get("q")
		if !strings.Contains(q, "repo:rudrankriyam/App-Store-Connect-CLI") {
			t.Errorf("query missing repo filter: %s", q)
		}
		if !strings.Contains(q, "bundle ID") {
			t.Errorf("query missing search term: %s", q)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", auth)
		}

		resp := map[string]any{
			"total_count": 1,
			"items": []map[string]any{
				{
					"number":   42,
					"title":    "crashes --app doesn't support bundle ID",
					"html_url": "https://github.com/rudrankriyam/App-Store-Connect-CLI/issues/42",
					"state":    "open",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Override the GitHub API base for testing.
	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	issues, err := searchIssues(t.Context(), "test-token", "bundle ID")
	if err != nil {
		t.Fatalf("searchIssues() error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Number != 42 {
		t.Errorf("expected issue #42, got #%d", issues[0].Number)
	}
}

func TestCreateIssue(t *testing.T) {
	var receivedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/issues") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, _ := json.Marshal(map[string]any{})
		json.NewDecoder(r.Body).Decode(&receivedPayload)
		_ = body

		resp := map[string]any{
			"number":   99,
			"title":    receivedPayload["title"],
			"html_url": "https://github.com/rudrankriyam/App-Store-Connect-CLI/issues/99",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	entry := LogEntry{
		Description: "test issue",
		Severity:    "bug",
		ASCVersion:  "0.37.2",
		OS:          "darwin/arm64",
		Timestamp:   time.Now().UTC(),
	}

	issue, err := createIssue(t.Context(), "test-token", entry)
	if err != nil {
		t.Fatalf("createIssue() error: %v", err)
	}
	if issue.Number != 99 {
		t.Errorf("expected issue #99, got #%d", issue.Number)
	}

	// Verify labels were sent.
	labels, ok := receivedPayload["labels"].([]any)
	if !ok {
		t.Fatal("expected labels array")
	}
	foundSnitch := false
	for _, l := range labels {
		if l == "asc-snitch" {
			foundSnitch = true
		}
	}
	if !foundSnitch {
		t.Error("expected asc-snitch label")
	}
}

func TestWriteLocalLog(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(tmpDir)

	entry := LogEntry{
		Description: "local test entry",
		Severity:    "friction",
		ASCVersion:  "0.37.2",
		OS:          "darwin/arm64",
		Timestamp:   time.Now().UTC(),
	}

	if err := writeLocalLog(entry); err != nil {
		t.Fatalf("writeLocalLog() error: %v", err)
	}

	logPath := filepath.Join(".asc", "snitch.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var decoded LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &decoded); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}
	if decoded.Description != "local test entry" {
		t.Errorf("expected description 'local test entry', got %q", decoded.Description)
	}

	// Write a second entry and verify append.
	entry.Description = "second entry"
	if err := writeLocalLog(entry); err != nil {
		t.Fatalf("writeLocalLog() second call error: %v", err)
	}

	data, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}
}

func TestSearchIssuesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	_, err := searchIssues(t.Context(), "", "test")
	if err == nil {
		t.Fatal("expected error on 403 response")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("expected 403 in error, got: %v", err)
	}
}

func TestCreateIssueMissingToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	entry := LogEntry{
		Description: "test",
		Severity:    "bug",
		ASCVersion:  "0.37.2",
		OS:          "darwin/arm64",
	}

	_, err := createIssue(t.Context(), "", entry)
	if err == nil {
		t.Fatal("expected error on 401 response")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}
