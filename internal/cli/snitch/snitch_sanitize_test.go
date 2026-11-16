package snitch

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// hostileTitle carries an OSC window-title sequence, a C1 CSI, and a bidi
// override, all of which a maintainer terminal or CI log viewer interprets.
const hostileTitle = "crash on launch\x1b]0;pwned\x07\u009b2K\u202egpj.exe"

func assertNoInterpretedSequences(t *testing.T, label string, output string) {
	t.Helper()

	for i, line := range strings.Split(output, "\n") {
		if asc.HasInterpretedTerminalSequence(line) {
			t.Fatalf("%s line %d = %q contains interpreted terminal sequences", label, i+1, line)
		}
	}
}

func TestPrintPotentialDuplicatesRemovesTerminalControls(t *testing.T) {
	var stderrBuffer bytes.Buffer
	printPotentialDuplicates(&stderrBuffer, []GitHubIssue{{
		Number:  42,
		Title:   hostileTitle,
		HTMLURL: "https://github.com/rorkai/App-Store-Connect-CLI/issues/42\x1b[2K",
		State:   "open",
	}})
	stderr := stderrBuffer.String()

	assertNoInterpretedSequences(t, "stderr", stderr)
	if !strings.Contains(stderr, "#42") {
		t.Fatalf("stderr = %q, want the issue number", stderr)
	}
	if !strings.Contains(stderr, "crash on launch") {
		t.Fatalf("stderr = %q, want the readable part of the title", stderr)
	}
}

func TestDuplicateSearchWarningRemovesTerminalControls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := w.Write([]byte("boom\x1b]0;pwned\x07\u202e")); err != nil {
			t.Fatalf("w.Write() error: %v", err)
		}
	}))
	defer server.Close()

	origBase := githubAPIBase
	defer func() { setGitHubAPIBase(origBase) }()
	setGitHubAPIBase(server.URL)

	t.Setenv("GITHUB_TOKEN", "token")
	t.Setenv("GH_TOKEN", "")

	_, stderr, err := runSnitchCommand(t, "1.2.3", "--dry-run", "friction report")
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	assertNoInterpretedSequences(t, "stderr", stderr)
	if !strings.Contains(stderr, "duplicate search failed") {
		t.Fatalf("stderr = %q, want the duplicate search warning", stderr)
	}
}

func TestSnitchPreviewRemovesTerminalControls(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	_, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "Error: failed\x1b]0;pwned\x07",
		hostileTitle,
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	assertNoInterpretedSequences(t, "stderr", stderr)
	if !strings.Contains(stderr, "**asc version:** 9.9.9") {
		t.Fatalf("stderr = %q, want the reported asc version in the preview body", stderr)
	}
}

func TestSnitchDryRunRedactsSensitiveActual(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")

	const secret = "eyJhbGciOiJFUzI1NiJ9.fake.signature"
	_, stderr, err := runSnitchCommand(
		t, "9.9.9",
		"--dry-run",
		"--actual", "Authorization: Bearer "+secret,
		"redaction probe",
	)
	if err != nil {
		t.Fatalf("run snitch: %v", err)
	}

	if strings.Contains(stderr, secret) {
		t.Fatalf("stderr leaked the credential: %q", stderr)
	}
	if !strings.Contains(stderr, "Authorization: [REDACTED]") {
		t.Fatalf("stderr = %q, want a redaction marker that preserves context", stderr)
	}
	if !strings.Contains(stderr, "sensitive values were redacted") {
		t.Fatalf("stderr = %q, want a generic redaction notice", stderr)
	}
}

func TestSnitchFlushRemovesTerminalControls(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snitch.log")

	entry := LogEntry{
		Description: hostileTitle,
		Severity:    "bug",
		Repro:       "asc status --app \"com.example\"\nasc builds list\x1b[2K",
		Expected:    "resolution works\u2066",
		Actual:      "Error: not found\u009b2K",
		Labels:      []string{"p3\x1b[31m", "bug"},
		Timestamp:   time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
		ASCVersion:  "1.2.3\x1b[0m",
		OS:          "darwin/arm64\u202e",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	stdout, _, err := runSnitchCommand(t, "1.2.3", "flush", "--file", logPath)
	if err != nil {
		t.Fatalf("run snitch flush: %v", err)
	}

	assertNoInterpretedSequences(t, "stdout", stdout)
	if !strings.Contains(stdout, "asc status --app \"com.example\"\nasc builds list") {
		t.Fatalf("stdout = %q, want the multi-line reproduction preserved", stdout)
	}
	if !strings.Contains(stdout, "Timestamp: 2026-03-07T12:00:00Z") {
		t.Fatalf("stdout = %q, want the timestamp", stdout)
	}
}

func TestSnitchFlushKeepsLogEntriesIntactOnDisk(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "snitch.log")

	entry := LogEntry{Description: hostileTitle, Severity: "bug"}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal() error: %v", err)
	}
	if err := os.WriteFile(logPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error: %v", err)
	}

	if _, _, err := runSnitchCommand(t, "1.2.3", "flush", "--file", logPath); err != nil {
		t.Fatalf("run snitch flush: %v", err)
	}

	entries, err := readLocalLog(newSnitchTestDeps(), logPath)
	if err != nil {
		t.Fatalf("readLocalLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Description != hostileTitle {
		t.Fatalf("stored description = %q, want the original %q", entries[0].Description, hostileTitle)
	}
}

func TestReadGitHubAPIErrorRemovesTerminalControls(t *testing.T) {
	body := "{\"message\":\"boom\x1b]0;pwned\x07\u009b2K\u202e\",\"line\":\"two\u2028three\"}"
	resp := &http.Response{
		StatusCode: http.StatusUnprocessableEntity,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := readGitHubAPIError(resp)
	if err == nil {
		t.Fatalf("expected an error for a non-2xx response")
	}
	if !strings.Contains(err.Error(), "422") {
		t.Fatalf("error %q should include the status code", err.Error())
	}
	if asc.HasInterpretedTerminalSequence(err.Error()) {
		t.Fatalf("readGitHubAPIError() = %q contains interpreted terminal sequences", err.Error())
	}
}
