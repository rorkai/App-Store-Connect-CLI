package snitch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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

func TestIssueLabelsIncludesCustomLabelsWithoutDuplicates(t *testing.T) {
	labels := issueLabels(LogEntry{
		Severity: "feature-request",
		Labels:   []string{"enhancement", "p3", "easy", "P3"},
	})

	want := []string{"asc-snitch", "enhancement", "p3", "easy"}
	if len(labels) != len(want) {
		t.Fatalf("issueLabels() length = %d, want %d (%v)", len(labels), len(want), labels)
	}
	for i, label := range want {
		if labels[i] != label {
			t.Fatalf("issueLabels()[%d] = %q, want %q (full=%v)", i, labels[i], label, labels)
		}
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
		if !strings.Contains(q, "repo:rorkai/App-Store-Connect-CLI") {
			t.Errorf("query missing repo filter: %s", q)
		}
		if !strings.Contains(q, "is:open") {
			t.Errorf("query missing open issue filter: %s", q)
		}
		if !strings.Contains(q, "in:title") {
			t.Errorf("query missing title filter: %s", q)
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
					"html_url": "https://github.com/rorkai/App-Store-Connect-CLI/issues/42",
					"state":    "open",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("json.NewEncoder().Encode() error: %v", err)
		}
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

func TestSnitchCommandPreviewWithoutConfirmDoesNotCreateIssue(t *testing.T) {
	searchCalls := 0
	createCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			searchCalls++
			resp := map[string]any{"items": []map[string]any{}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		case "/repos/rorkai/App-Store-Connect-CLI/issues":
			createCalls++
			t.Fatal("createIssue should not be called without --confirm")
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	stdout, stderr, err := runSnitchCommand(t, "1.2.3", "preview", "without", "confirm")
	if err != nil {
		t.Fatalf("runSnitchCommand() error: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected no stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Preview only: rerun with --confirm to create issue") {
		t.Fatalf("expected preview banner, got %q", stderr)
	}
	if !strings.Contains(stderr, "preview without confirm") {
		t.Fatalf("expected full multi-word description, got %q", stderr)
	}
	if searchCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCalls)
	}
	if createCalls != 0 {
		t.Fatalf("expected 0 create calls, got %d", createCalls)
	}
}

func TestSnitchCommandConfirmCreatesIssue(t *testing.T) {
	searchCalls := 0
	createCalls := 0
	labelCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			searchCalls++
			resp := map[string]any{"items": []map[string]any{}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		case "/repos/rorkai/App-Store-Connect-CLI/issues":
			createCalls++
			resp := map[string]any{
				"number":   77,
				"title":    "confirmed issue",
				"html_url": "https://github.com/rorkai/App-Store-Connect-CLI/issues/77",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		case "/repos/rorkai/App-Store-Connect-CLI/issues/77/labels":
			labelCalls++
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{"labels": []string{"asc-snitch", "bug"}}); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	stdout, stderr, err := runSnitchCommand(t, "1.2.3", "--confirm", "confirmed", "issue")
	if err != nil {
		t.Fatalf("runSnitchCommand() error: %v", err)
	}
	if !strings.Contains(stderr, "Issue created: #77") {
		t.Fatalf("expected issue creation message, got %q", stderr)
	}
	if !strings.Contains(stdout, `"number":77`) {
		t.Fatalf("expected JSON stdout with issue number, got %q", stdout)
	}
	if searchCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCalls)
	}
	if createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", createCalls)
	}
	if labelCalls != 1 {
		t.Fatalf("expected 1 label call, got %d", labelCalls)
	}
}

func TestSnitchCommandConfirmCreatesIssueWhenLabelsCannotBeApplied(t *testing.T) {
	searchCalls := 0
	createCalls := 0
	labelCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search/issues":
			searchCalls++
			resp := map[string]any{"items": []map[string]any{}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		case "/repos/rorkai/App-Store-Connect-CLI/issues":
			createCalls++

			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("json.NewDecoder().Decode() error: %v", err)
			}
			if _, ok := payload["labels"]; ok {
				w.WriteHeader(http.StatusForbidden)
				if _, err := w.Write([]byte(`{"message":"Resource not accessible by integration"}`)); err != nil {
					t.Fatalf("w.Write() error: %v", err)
				}
				return
			}

			resp := map[string]any{
				"number":   77,
				"title":    "confirmed issue",
				"html_url": "https://github.com/rorkai/App-Store-Connect-CLI/issues/77",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("json.NewEncoder().Encode() error: %v", err)
			}
		case "/repos/rorkai/App-Store-Connect-CLI/issues/77/labels":
			labelCalls++
			w.WriteHeader(http.StatusForbidden)
			if _, err := w.Write([]byte(`{"message":"Resource not accessible by integration"}`)); err != nil {
				t.Fatalf("w.Write() error: %v", err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GH_TOKEN", "")

	stdout, stderr, err := runSnitchCommand(t, "1.2.3", "--confirm", "confirmed", "issue")
	if err != nil {
		t.Fatalf("runSnitchCommand() error: %v", err)
	}
	if !strings.Contains(stderr, "Issue created: #77") {
		t.Fatalf("expected issue creation message, got %q", stderr)
	}
	if !strings.Contains(stderr, "labels could not be applied") {
		t.Fatalf("expected label warning, got %q", stderr)
	}
	if !strings.Contains(stdout, `"number":77`) {
		t.Fatalf("expected JSON stdout with issue number, got %q", stdout)
	}
	if searchCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCalls)
	}
	if createCalls != 1 {
		t.Fatalf("expected 1 create call, got %d", createCalls)
	}
	if labelCalls != 1 {
		t.Fatalf("expected 1 label call, got %d", labelCalls)
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

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Fatalf("json.NewDecoder().Decode() error: %v", err)
		}

		resp := map[string]any{
			"number":   99,
			"title":    receivedPayload["title"],
			"html_url": "https://github.com/rorkai/App-Store-Connect-CLI/issues/99",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("json.NewEncoder().Encode() error: %v", err)
		}
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

	if _, ok := receivedPayload["labels"]; ok {
		t.Fatal("did not expect labels in createIssue payload")
	}
}

func TestAddIssueLabels(t *testing.T) {
	var receivedPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/issues/99/labels" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Fatalf("json.NewDecoder().Decode() error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"labels": []string{"asc-snitch", "bug"}}); err != nil {
			t.Fatalf("json.NewEncoder().Encode() error: %v", err)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	if err := addIssueLabels(t.Context(), "test-token", 99, []string{"asc-snitch", "bug"}); err != nil {
		t.Fatalf("addIssueLabels() error: %v", err)
	}

	labels, ok := receivedPayload["labels"].([]any)
	if !ok {
		t.Fatal("expected labels array")
	}
	if len(labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(labels))
	}
}

func TestListRepoLabelsPaginatesAndDedupes(t *testing.T) {
	pageCalls := 0
	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase("https://example.test")

	origClient := githubHTTPClient
	defer func() { githubHTTPClient = origClient }()
	githubHTTPClient = func() *http.Client {
		return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			pageCalls++
			if r.URL.Path != "/repos/rorkai/App-Store-Connect-CLI/labels" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}

			page := r.URL.Query().Get("page")
			var payload any
			switch page {
			case "1":
				labels := make([]map[string]any, 0, labelsPerPage)
				for i := 0; i < labelsPerPage; i++ {
					name := fmt.Sprintf("label-%03d", i)
					if i == 0 {
						name = "enhancement"
					}
					labels = append(labels, map[string]any{"name": name})
				}
				payload = labels
			case "2":
				payload = []map[string]any{
					{"name": "enhancement"},
					{"name": "p3"},
				}
			default:
				t.Fatalf("unexpected page query: %q", page)
			}

			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("json.Marshal() error: %v", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			}, nil
		})}
	}

	labels, err := listRepoLabels(t.Context(), "test-token")
	if err != nil {
		t.Fatalf("listRepoLabels() error: %v", err)
	}
	if pageCalls != 2 {
		t.Fatalf("expected 2 page calls, got %d", pageCalls)
	}
	if !slices.Contains(labels, "enhancement") {
		t.Fatalf("expected enhancement in labels, got %v", labels)
	}
	if !slices.Contains(labels, "p3") {
		t.Fatalf("expected p3 in labels, got %v", labels)
	}
	enhancementCount := 0
	for _, label := range labels {
		if label == "enhancement" {
			enhancementCount++
		}
	}
	if enhancementCount != 1 {
		t.Fatalf("expected enhancement once, got %d in %v", enhancementCount, labels)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestWriteLocalLog(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

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
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("failed to stat log file: %v", err)
	}
	if got := info.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("expected log file to be private, got mode %o", info.Mode().Perm())
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

func TestWriteLocalLogSecuresExistingFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Fatalf("os.Chdir restore error: %v", err)
		}
	}()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("os.Chdir temp dir error: %v", err)
	}

	if err := os.MkdirAll(".asc", 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error: %v", err)
	}
	logPath := filepath.Join(".asc", "snitch.log")
	if err := os.WriteFile(logPath, []byte(""), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	entry := LogEntry{
		Description: "secure existing file",
		Severity:    "bug",
		ASCVersion:  "0.37.2",
		OS:          "darwin/arm64",
		Timestamp:   time.Now().UTC(),
	}
	if err := writeLocalLog(entry); err != nil {
		t.Fatalf("writeLocalLog() error: %v", err)
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("os.Stat() error: %v", err)
	}
	if got := info.Mode().Perm() & 0o077; got != 0 {
		t.Fatalf("expected existing log file permissions to be tightened, got mode %o", info.Mode().Perm())
	}
}

func TestReadLocalLogAndFormatEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snitch.log")

	entry := LogEntry{
		Description: "status command needs bundle ID support",
		Severity:    "friction",
		Repro:       `asc status --app "com.example.app"`,
		Expected:    "Bundle ID resolution should work",
		Actual:      "Error: app not found",
		Timestamp:   time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		ASCVersion:  "1.2.3",
		OS:          "darwin/arm64",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if err := os.WriteFile(logPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	entries, err := readLocalLog(logPath)
	if err != nil {
		t.Fatalf("readLocalLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	formatted := formatLocalEntries(entries)
	checks := []string{
		"[1] friction: status command needs bundle ID support",
		"Timestamp: 2026-03-07T12:00:00Z",
		"ASC version: 1.2.3",
		"OS: darwin/arm64",
		"Reproduction:",
		`asc status --app "com.example.app"`,
		"Expected:",
		"Bundle ID resolution should work",
		"Actual:",
		"Error: app not found",
	}
	for _, check := range checks {
		if !strings.Contains(formatted, check) {
			t.Fatalf("formatted output missing %q: %q", check, formatted)
		}
	}
}

func TestReadLocalLogSupportsLargeEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snitch.log")

	entry := LogEntry{
		Description: "large local entry",
		Severity:    "bug",
		Actual:      strings.Repeat("stacktrace line\n", 6000),
		ASCVersion:  "1.2.3",
		OS:          "darwin/arm64",
		Timestamp:   time.Now().UTC(),
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if len(data) <= 64*1024 {
		t.Fatalf("expected marshaled entry to exceed scanner limit, got %d bytes", len(data))
	}
	if err := os.WriteFile(logPath, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	entries, err := readLocalLog(logPath)
	if err != nil {
		t.Fatalf("readLocalLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != entry.Description {
		t.Fatalf("expected description %q, got %q", entry.Description, entries[0].Description)
	}
	if entries[0].Actual != entry.Actual {
		t.Fatalf("expected large actual payload to round-trip")
	}
}

func TestReadLocalLogInvalidLine(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snitch.log")
	if err := os.WriteFile(logPath, []byte("{invalid json}\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	_, err := readLocalLog(logPath)
	if err == nil {
		t.Fatal("expected invalid log entry error")
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Fatalf("expected line number in error, got %v", err)
	}
}

func TestSearchIssuesHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		if _, err := w.Write([]byte("rate limited")); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
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
		if _, err := w.Write([]byte(`{"message":"Bad credentials"}`)); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
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

func runSnitchCommand(t *testing.T, version string, args ...string) (string, string, error) {
	t.Helper()

	cmd := SnitchCommand(version)
	cmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := cmd.Parse(args); err != nil {
			runErr = err
			return
		}
		runErr = cmd.Run(context.Background())
	})

	return stdout, stderr, runErr
}

func captureOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	origStdout := os.Stdout
	origStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stdout error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe stderr error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutDone := make(chan string, 1)
	stderrDone := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(stdoutR)
		stdoutDone <- string(data)
	}()
	go func() {
		data, _ := io.ReadAll(stderrR)
		stderrDone <- string(data)
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	stdout := <-stdoutDone
	stderr := <-stderrDone
	return stdout, stderr
}
