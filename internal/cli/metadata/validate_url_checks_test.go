package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/metadataurl"
)

type fakeMetadataURLChecker struct {
	mu      sync.Mutex
	results map[string]metadataurl.Result
	errors  map[string]error
	calls   map[string]int
}

func (f *fakeMetadataURLChecker) Check(_ context.Context, rawURL string) (metadataurl.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[rawURL]++
	if err := f.errors[rawURL]; err != nil {
		return metadataurl.Result{}, err
	}
	return f.results[rawURL], nil
}

func (f *fakeMetadataURLChecker) totalCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	total := 0
	for _, count := range f.calls {
		total += count
	}
	return total
}

func TestValidateDirCheckURLsWarnsForRedirectedHostAndSiteRoot(t *testing.T) {
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(appInfoDirName, "en-US.json"):          `{"name":"Example App","privacyPolicyUrl":"https://app.example.com/privacy"}`,
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"https://support.example.com/help"}`,
	})
	checker := &fakeMetadataURLChecker{results: map[string]metadataurl.Result{
		"https://app.example.com/privacy": {
			FinalURL:   mustParseMetadataURL(t, "https://app.example.com/"),
			StatusCode: 200,
		},
		"https://support.example.com/help": {
			FinalURL:   mustParseMetadataURL(t, "https://example.com/"),
			StatusCode: 200,
		},
	}}

	result, err := validateDirWithOptions(context.Background(), dir, validateDirOptions{
		checkURLs:  true,
		urlChecker: checker,
	})
	if err != nil {
		t.Fatalf("validateDirWithOptions() error: %v", err)
	}
	if result.ErrorCount != 0 || !result.Valid {
		t.Fatalf("expected URL findings to stay warning-only, got %+v", result)
	}
	if result.WarningCount != 3 {
		t.Fatalf("expected 3 URL warnings, got %+v", result.Issues)
	}

	wantMessages := []string{
		"privacy policy URL resolves to a site root instead of a dedicated page",
		"support URL redirects to a different host (support.example.com -> example.com)",
		"support URL resolves to a site root instead of a dedicated page",
	}
	for _, want := range wantMessages {
		if !hasMetadataURLWarning(result.Issues, want) {
			t.Fatalf("missing warning %q in %+v", want, result.Issues)
		}
	}
}

func TestMetadataURLCheckMessagesDescribesReturningHostWithoutAtoA(t *testing.T) {
	message := metadataURLCheckMessages(metadataURLTarget{
		rawURL: "https://support.example/help",
		label:  "support URL",
	}, metadataURLCheckOutcome{result: metadataurl.Result{
		FinalURL:       mustParseMetadataURL(t, "https://support.example/final"),
		StatusCode:     http.StatusOK,
		RedirectedHost: true,
	}})
	if len(message) != 1 {
		t.Fatalf("messages = %v, want one redirect warning", message)
	}
	want := "support URL redirects through a different host before returning to support.example"
	if message[0] != want {
		t.Fatalf("message = %q, want %q", message[0], want)
	}
	if strings.Contains(message[0], "support.example -> support.example") {
		t.Fatalf("message contains contradictory A -> A transition: %q", message[0])
	}
}

func TestValidateDirCheckURLsWarnsForStatusAndRequestFailure(t *testing.T) {
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(appInfoDirName, "en-US.json"):          `{"name":"Example App","privacyPolicyUrl":"https://app.example.com/privacy"}`,
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"https://support.example.com/help"}`,
	})
	checker := &fakeMetadataURLChecker{
		results: map[string]metadataurl.Result{
			"https://support.example.com/help": {
				FinalURL:   mustParseMetadataURL(t, "https://parked.example.com/missing"),
				StatusCode: 404,
			},
		},
		errors: map[string]error{
			"https://app.example.com/privacy": errors.New("dial tcp 192.0.2.1:443: connection refused"),
		},
	}

	result, err := validateDirWithOptions(context.Background(), dir, validateDirOptions{
		checkURLs:  true,
		urlChecker: checker,
	})
	if err != nil {
		t.Fatalf("validateDirWithOptions() error: %v", err)
	}
	if result.WarningCount != 3 || result.ErrorCount != 0 || !result.Valid {
		t.Fatalf("expected 3 warning-only findings, got %+v", result)
	}
	if !hasMetadataURLWarning(result.Issues, "support URL returned HTTP 404") {
		t.Fatalf("missing status warning in %+v", result.Issues)
	}
	if !hasMetadataURLWarning(result.Issues, "support URL redirects to a different host (support.example.com -> parked.example.com)") {
		t.Fatalf("missing redirect warning in %+v", result.Issues)
	}
	if !hasMetadataURLWarning(result.Issues, "privacy policy URL could not be checked: request failed") {
		t.Fatalf("missing generic request warning in %+v", result.Issues)
	}
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "192.0.2.1") {
			t.Fatalf("warning leaked transport details: %+v", issue)
		}
	}
}

func TestValidateDirCheckURLsCachesDuplicateURLs(t *testing.T) {
	const supportURL = "https://support.example.com/help"
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"` + supportURL + `"}`,
		filepath.Join(versionDirName, "1.2.3", "fr-FR.json"): `{"description":"Description francaise complete","supportUrl":"` + supportURL + `"}`,
	})
	checker := &fakeMetadataURLChecker{results: map[string]metadataurl.Result{
		supportURL: {
			FinalURL:   mustParseMetadataURL(t, "https://support.example.com/"),
			StatusCode: 200,
		},
	}}

	result, err := validateDirWithOptions(context.Background(), dir, validateDirOptions{
		checkURLs:  true,
		urlChecker: checker,
	})
	if err != nil {
		t.Fatalf("validateDirWithOptions() error: %v", err)
	}
	if checker.totalCalls() != 1 {
		t.Fatalf("expected duplicate URL to be checked once, got %d calls", checker.totalCalls())
	}
	if result.WarningCount != 2 {
		t.Fatalf("expected one warning per metadata file, got %+v", result.Issues)
	}
}

func TestValidateDirRemainsOfflineWithoutCheckURLs(t *testing.T) {
	const supportURL = "https://support.example.com/help"
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"` + supportURL + `"}`,
	})
	checker := &fakeMetadataURLChecker{results: map[string]metadataurl.Result{
		supportURL: {
			FinalURL:   mustParseMetadataURL(t, "https://support.example.com/"),
			StatusCode: 200,
		},
	}}

	result, err := validateDirWithOptions(context.Background(), dir, validateDirOptions{
		urlChecker: checker,
	})
	if err != nil {
		t.Fatalf("validateDirWithOptions() error: %v", err)
	}
	if checker.totalCalls() != 0 {
		t.Fatalf("expected offline validation to make no URL checks, got %d calls", checker.totalCalls())
	}
	if result.WarningCount != 0 {
		t.Fatalf("expected no online warnings without --check-urls, got %+v", result.Issues)
	}
}

func TestValidateDirCheckURLsPreservesContextCancellation(t *testing.T) {
	const supportURL = "https://support.example.com/help"
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"` + supportURL + `"}`,
	})
	checker := &fakeMetadataURLChecker{errors: map[string]error{
		supportURL: context.Canceled,
	}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := validateDirWithOptions(ctx, dir, validateDirOptions{
		checkURLs:  true,
		urlChecker: checker,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestMetadataValidateCommandPrintsWarningsAndExitsSuccessfully(t *testing.T) {
	const supportURL = "https://support.example.com/help"
	dir := writeMetadataURLFixtures(t, map[string]string{
		filepath.Join(versionDirName, "1.2.3", "en-US.json"): `{"description":"English app description","supportUrl":"` + supportURL + `"}`,
	})
	checker := &fakeMetadataURLChecker{results: map[string]metadataurl.Result{
		supportURL: {
			FinalURL:   mustParseMetadataURL(t, "https://support.example.com/"),
			StatusCode: 200,
		},
	}}
	originalFactory := newMetadataURLChecker
	newMetadataURLChecker = func() metadataurl.Checker { return checker }
	t.Cleanup(func() { newMetadataURLChecker = originalFactory })

	var runErr error
	stdout := captureStdout(t, func() {
		runErr = MetadataValidateCommand().ParseAndRun(context.Background(), []string{
			"--dir", dir,
			"--check-urls",
			"--output", "json",
		})
	})
	if runErr != nil {
		t.Fatalf("ParseAndRun() error: %v", runErr)
	}
	var result ValidateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("json.Unmarshal() error: %v; stdout=%q", err, stdout)
	}
	if !result.Valid || result.ErrorCount != 0 || result.WarningCount != 1 {
		t.Fatalf("expected warning-only successful output, got %+v", result)
	}
}

func TestMetadataURLCheckMessagesAllowsQueryAndFragmentRoutes(t *testing.T) {
	target := metadataURLTarget{rawURL: "https://example.com/support", label: "support URL"}
	for _, finalURL := range []string{
		"https://example.com/?page=support",
		"https://example.com/#/support",
	} {
		t.Run(finalURL, func(t *testing.T) {
			messages := metadataURLCheckMessages(target, metadataURLCheckOutcome{result: metadataurl.Result{
				FinalURL:   mustParseMetadataURL(t, finalURL),
				StatusCode: http.StatusOK,
			}})
			if len(messages) != 0 {
				t.Fatalf("expected routed root URL to pass, got %v", messages)
			}
		})
	}
}

func writeMetadataURLFixtures(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for relativePath, body := range files {
		path := filepath.Join(dir, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%q): %v", path, err)
		}
	}
	return dir
}

func mustParseMetadataURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", rawURL, err)
	}
	return parsed
}

func hasMetadataURLWarning(issues []ValidateIssue, message string) bool {
	for _, issue := range issues {
		if issue.Severity == issueSeverityWarning && issue.Message == message {
			return true
		}
	}
	return false
}
