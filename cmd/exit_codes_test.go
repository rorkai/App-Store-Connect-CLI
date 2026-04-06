package cmd

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestExitCodeFromError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: ExitSuccess,
		},
		{
			name:     "flag.ErrHelp returns usage",
			err:      flag.ErrHelp,
			expected: ExitUsage,
		},
		{
			name:     "ErrMissingAuth returns auth failure",
			err:      shared.ErrMissingAuth,
			expected: ExitAuth,
		},
		{
			name:     "ErrUnauthorized returns auth failure",
			err:      asc.ErrUnauthorized,
			expected: ExitAuth,
		},
		{
			name:     "ErrForbidden returns auth failure",
			err:      asc.ErrForbidden,
			expected: ExitAuth,
		},
		{
			name:     "ErrNotFound returns not found",
			err:      asc.ErrNotFound,
			expected: ExitNotFound,
		},
		{
			name:     "ErrConflict returns conflict",
			err:      asc.ErrConflict,
			expected: ExitConflict,
		},
		{
			name:     "generic error returns generic error",
			err:      errors.New("something went wrong"),
			expected: ExitError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExitCodeFromError(tt.err)
			if result != tt.expected {
				t.Errorf("ExitCodeFromError(%v) = %d, want %d", tt.err, result, tt.expected)
			}
		})
	}
}

func TestExitCodeFromError_Conflict(t *testing.T) {
	conflictErr := &asc.APIError{
		Code:   "CONFLICT",
		Title:  "Conflict",
		Detail: "Resource already exists",
	}
	result := ExitCodeFromError(conflictErr)
	if result != ExitConflict {
		t.Errorf("ExitCodeFromError(conflict) = %d, want %d (Conflict)", result, ExitConflict)
	}
}

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("ExitError = %d, want 1", ExitError)
	}
	if ExitUsage != 2 {
		t.Errorf("ExitUsage = %d, want 2", ExitUsage)
	}
	if ExitAuth != 3 {
		t.Errorf("ExitAuth = %d, want 3", ExitAuth)
	}
	if ExitNotFound != 4 {
		t.Errorf("ExitNotFound = %d, want 4", ExitNotFound)
	}
	if ExitConflict != 5 {
		t.Errorf("ExitConflict = %d, want 5", ExitConflict)
	}
}

func TestAPIErrorCodeToExitCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{"NOT_FOUND", "NOT_FOUND", ExitNotFound},
		{"CONFLICT", "CONFLICT", ExitConflict},
		{"UNAUTHORIZED", "UNAUTHORIZED", ExitAuth},
		{"FORBIDDEN", "FORBIDDEN", ExitAuth},
		{"BAD_REQUEST", "BAD_REQUEST", ExitHTTPBadRequest},
		{"unknown code", "SOME_ERROR", ExitError},
		{"empty code", "", ExitError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := APIErrorCodeToExitCode(tt.code)
			if result != tt.expected {
				t.Errorf("APIErrorCodeToExitCode(%q) = %d, want %d", tt.code, result, tt.expected)
			}
		})
	}
}

func TestExitCodeFromError_NonJSONAPIStatus(t *testing.T) {
	err := asc.ParseErrorWithStatus([]byte("<html>bad gateway</html>"), http.StatusBadGateway)
	result := ExitCodeFromError(err)
	if result != ExitHTTPBadGateway {
		t.Errorf("ExitCodeFromError(non-JSON 502) = %d, want %d", result, ExitHTTPBadGateway)
	}
}

func TestGetCommandName(t *testing.T) {
	makeCommandTree := func() *ffcli.Command {
		rootFlags := flag.NewFlagSet("asc", flag.ContinueOnError)
		rootFlags.Bool("debug", false, "")
		rootFlags.String("report", "", "")
		rootFlags.String("report-file", "", "")
		rootFlags.String("profile", "", "")

		return &ffcli.Command{
			Name:    "asc",
			FlagSet: rootFlags,
			Subcommands: []*ffcli.Command{
				{
					Name: "builds",
					Subcommands: []*ffcli.Command{
						{Name: "list"},
						{Name: "get"},
					},
				},
				{
					Name: "apps",
					Subcommands: []*ffcli.Command{
						{Name: "list"},
					},
				},
				{Name: "completion"},
			},
		}
	}

	// Test cases use os.Args[1:] format (without program name).
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"root command", []string{}, "asc"},
		{"single level subcommand", []string{"builds"}, "asc builds"},
		{"nested subcommand", []string{"builds", "list"}, "asc builds list"},
		{"another nested subcommand", []string{"apps", "list"}, "asc apps list"},
		{"root flag before subcommand", []string{"--debug", "builds"}, "asc builds"},
		{"multiple root flags before subcommand", []string{"--report", "junit", "--report-file", "/tmp/report.xml", "completion"}, "asc completion"},
		{"flag value matches subcommand name", []string{"--profile", "builds", "completion"}, "asc completion"},
		{"subcommand then flags", []string{"builds", "list", "--output", "json"}, "asc builds list"},
		{"backward compatibility with program name", []string{"asc", "apps", "list"}, "asc apps list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getCommandName(makeCommandTree(), tt.args)
			if result != tt.expected {
				t.Errorf("getCommandName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestJUnitReportNameWithRootFlags(t *testing.T) {
	// Build the binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = ".." // Go up from cmd/ to project root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, out)
	}

	reportFile := filepath.Join(tmpDir, "junit.xml")
	// Run with root flags before subcommand
	runCmd := exec.Command(binaryPath, "--report", "junit", "--report-file", reportFile, "completion", "--shell", "zsh")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, _ := runCmd.CombinedOutput()

	// Read and parse the JUnit report
	data, err := os.ReadFile(reportFile)
	if err != nil {
		t.Fatalf("Failed to read JUnit report: %v", err)
	}

	var result struct {
		XMLName xml.Name `xml:"testsuite"`
		Cases   []struct {
			Name string `xml:"name,attr"`
		} `xml:"testcase"`
	}
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to parse JUnit XML: %v\nOutput: %s", err, output)
	}

	if len(result.Cases) != 1 {
		t.Fatalf("Expected 1 test case, got %d", len(result.Cases))
	}

	// The test case name should include the subcommand, not just "asc"
	if !strings.Contains(result.Cases[0].Name, "completion") {
		t.Errorf("Expected testcase name to contain 'completion', got %q. Full XML:\n%s", result.Cases[0].Name, data)
	}
}

func TestJUnitReportEndToEnd(t *testing.T) {
	// Build the binary
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = ".." // Go up from cmd/ to project root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, out)
	}

	tests := []struct {
		name       string
		args       []string
		expectName string
	}{
		{
			name:       "flags before nested subcommand",
			args:       []string{"--report", "junit", "--report-file", "report1.xml", "builds", "list"},
			expectName: "asc builds list",
		},
		{
			name:       "single subcommand",
			args:       []string{"--report", "junit", "--report-file", "report2.xml", "completion", "--shell", "bash"},
			expectName: "asc completion",
		},
		{
			name:       "flag value matching subcommand name",
			args:       []string{"--report", "junit", "--report-file", "report3.xml", "--profile", "builds", "completion", "--shell", "bash"},
			expectName: "asc completion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Find --report-file index and use that value
			reportFile := ""
			for i, arg := range tt.args {
				if arg == "--report-file" && i+1 < len(tt.args) {
					reportFile = filepath.Join(tmpDir, tt.args[i+1])
					break
				}
			}
			if reportFile == "" {
				t.Fatal("Could not find --report-file in args")
			}

			// Build actual args with full path
			var fullArgs []string
			for i := 0; i < len(tt.args); i++ {
				arg := tt.args[i]
				if arg == "--report-file" && i+1 < len(tt.args) {
					fullArgs = append(fullArgs, arg, reportFile)
					i++ // Skip the value
				} else {
					fullArgs = append(fullArgs, arg)
				}
			}

			runCmd := exec.Command(binaryPath, fullArgs...)
			runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
			_, _ = runCmd.CombinedOutput() // Ignore errors, we just care about the report

			data, err := os.ReadFile(reportFile)
			if err != nil {
				t.Fatalf("Failed to read JUnit report: %v", err)
			}

			var result struct {
				XMLName xml.Name `xml:"testsuite"`
				Cases   []struct {
					Name string `xml:"name,attr"`
				} `xml:"testcase"`
			}
			if err := xml.Unmarshal(data, &result); err != nil {
				t.Fatalf("Failed to parse JUnit XML: %v\nReport content:\n%s", err, data)
			}

			if len(result.Cases) != 1 {
				t.Fatalf("Expected 1 test case, got %d", len(result.Cases))
			}

			if result.Cases[0].Name != tt.expectName {
				t.Errorf("Expected testcase name %q, got %q", tt.expectName, result.Cases[0].Name)
			}
		})
	}
}

func TestBuildsListMissingAppExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "builds", "list", "--version", "1.2.3")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for missing --app, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}
	stderr := string(output)
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app required message, got %q", stderr)
	}
}

func TestScreenshotsUploadResumeMissingValueExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "screenshots", "upload", "--resume")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for missing --resume value, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "flag needs an argument: -resume") {
		t.Fatalf("expected missing resume value message, got %q", stderr)
	}
}

func TestBuildsTestNotesUpdateConflictingFlagsExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "builds", "test-notes", "update",
		"--id", "loc-1", "--build", "build-1", "--whats-new", "test")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for conflicting flags, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "Warning: `--build` is deprecated. Use `--build-id`.") {
		t.Fatalf("expected legacy build warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "Warning: `--id` is deprecated. Use `--localization-id`.") {
		t.Fatalf("expected legacy id warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "--localization-id cannot be combined with build selectors or --locale") {
		t.Fatalf("expected conflict message, got %q", stderr)
	}
}

func TestBuildsLatestExcludeExpiredInvalidBooleanExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".." // Go up from cmd/ to project root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "builds", "latest", "--app", "APP_ID", "--exclude-expired=maybe")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid boolean value, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "invalid boolean value") {
		t.Fatalf("expected stderr to contain invalid boolean message, got %q", stderr)
	}
	if !strings.Contains(stderr, "exclude-expired") {
		t.Fatalf("expected stderr to mention exclude-expired flag, got %q", stderr)
	}
}

func TestPublishAppStoreDryRunInvalidBooleanExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(
		binaryPath,
		"publish", "appstore",
		"--app", "APP_ID",
		"--ipa", "app.ipa",
		"--version", "1.0.0",
		"--dry-run=maybe",
	)
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid boolean value, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "invalid boolean value") {
		t.Fatalf("expected stderr to contain invalid boolean message, got %q", stderr)
	}
	if !strings.Contains(stderr, "dry-run") {
		t.Fatalf("expected stderr to mention dry-run flag, got %q", stderr)
	}
}

func TestWebAuthLoginLegacyTwoFactorFlagExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(
		binaryPath,
		"web", "auth", "login",
		"--two-factor-code", "123456",
	)
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit when apple-id is missing, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "Warning: `--two-factor-code` is deprecated.") {
		t.Fatalf("expected deprecated flag warning, got %q", stderr)
	}
	if !strings.Contains(stderr, "--apple-id is required when no cached web session is available") {
		t.Fatalf("expected usage error after successful parsing, got %q", stderr)
	}
	if strings.Contains(stderr, "flag provided but not defined: -two-factor-code") {
		t.Fatalf("did not expect unknown flag parse failure, got %q", stderr)
	}
}

func TestAuthTokenConfirmInvalidBooleanExitCode(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".." // Go up from cmd/ to project root
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "auth", "token", "--confirm=maybe")
	runCmd.Env = isolatedCLITestEnv(filepath.Join(tmpDir, "config.json"))
	output, err := runCmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected non-zero exit for invalid boolean value, got success output: %s", output)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", err, err)
	}
	if exitErr.ExitCode() != ExitUsage {
		t.Fatalf("expected exit code %d, got %d (output: %s)", ExitUsage, exitErr.ExitCode(), output)
	}

	stderr := string(output)
	if !strings.Contains(stderr, "invalid boolean value") {
		t.Fatalf("expected stderr to contain invalid boolean message, got %q", stderr)
	}
	if !strings.Contains(stderr, "confirm") {
		t.Fatalf("expected stderr to mention confirm flag, got %q", stderr)
	}
}

func TestWebAuthLoginPromptInterruptDoesNotFallBackToUsageError(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	runCmd := exec.Command(binaryPath, "web", "auth", "login", "--apple-id", "user@example.com")
	runCmd.Env = append(
		isolatedCLITestEnv(filepath.Join(tmpDir, "config.json")),
		"ASC_WEB_SESSION_CACHE=1",
		"ASC_WEB_SESSION_CACHE_BACKEND=file",
		"ASC_WEB_SESSION_CACHE_DIR="+filepath.Join(tmpDir, "web-session-cache"),
		"ASC_IRIS_SESSION_CACHE=0",
	)

	ptmx, err := pty.Start(runCmd)
	if err != nil {
		t.Fatalf("failed to start PTY command: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	output, promptSeen, readDone := startPTYCapture(ptmx, "Apple Account password:")

	select {
	case <-promptSeen:
	case readErr := <-readDone:
		t.Fatalf("process exited before password prompt: %v\noutput:\n%s", readErr, output.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for password prompt\noutput:\n%s", output.String())
	}

	if _, err := ptmx.Write([]byte{3}); err != nil {
		t.Fatalf("failed to send Ctrl+C to PTY: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- runCmd.Wait()
	}()

	var runErr error
	select {
	case runErr = <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatalf("process did not exit promptly after interrupt\noutput:\n%s", output.String())
	}

	_ = ptmx.Close()

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("PTY reader did not exit after process completion\noutput:\n%s", output.String())
	}

	if runErr == nil {
		t.Fatalf("expected non-zero exit after interrupt\noutput:\n%s", output.String())
	}

	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T (%v)", runErr, runErr)
	}
	if exitErr.ExitCode() == ExitUsage {
		t.Fatalf("expected non-usage exit code after interrupt, got %d\noutput:\n%s", exitErr.ExitCode(), output.String())
	}

	stderr := output.String()
	if strings.Contains(stderr, "password is required") {
		t.Fatalf("expected no password-required fallback after interrupt, got %q", stderr)
	}
	if !strings.Contains(stderr, "password prompt interrupted") {
		t.Fatalf("expected interrupt-specific stderr, got %q", stderr)
	}
}

func TestWebAuthLoginPromptInterruptSkipsSkillsAutoCheck(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "asc-test")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	buildCmd.Dir = ".."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, out)
	}

	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"skills_checked_at":"2000-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	scriptDir := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		t.Fatalf("failed to create fake skills dir: %v", err)
	}

	markerPath := filepath.Join(tmpDir, "skills-check-ran")
	scriptPath := filepath.Join(scriptDir, "skills")
	script := "#!/bin/sh\n" +
		"printf 'ran' > \"$SKILLS_MARKER\"\n" +
		"sleep 2\n" +
		"printf 'update available\\n'\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to write fake skills command: %v", err)
	}

	runCmd := exec.Command(binaryPath, "web", "auth", "login", "--apple-id", "user@example.com")
	env := append(
		isolatedCLITestEnv(configPath),
		"ASC_WEB_SESSION_CACHE=1",
		"ASC_WEB_SESSION_CACHE_BACKEND=file",
		"ASC_WEB_SESSION_CACHE_DIR="+filepath.Join(tmpDir, "web-session-cache"),
		"ASC_IRIS_SESSION_CACHE=0",
		"ASC_SKILLS_AUTO_CHECK=1",
		"CI=",
		"SKILLS_MARKER="+markerPath,
		"PATH="+scriptDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	runCmd.Env = env

	ptmx, err := pty.Start(runCmd)
	if err != nil {
		t.Fatalf("failed to start PTY command: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	output, promptSeen, readDone := startPTYCapture(ptmx, "Apple Account password:")

	select {
	case <-promptSeen:
	case readErr := <-readDone:
		t.Fatalf("process exited before password prompt: %v\noutput:\n%s", readErr, output.String())
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for password prompt\noutput:\n%s", output.String())
	}

	if _, err := ptmx.Write([]byte{3}); err != nil {
		t.Fatalf("failed to send Ctrl+C to PTY: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- runCmd.Wait()
	}()

	select {
	case err = <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatalf("process did not exit promptly after interrupt\noutput:\n%s", output.String())
	}

	if err == nil {
		t.Fatalf("expected non-zero exit after interrupt\noutput:\n%s", output.String())
	}

	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatalf("PTY reader did not exit after process completion\noutput:\n%s", output.String())
	}

	if _, statErr := os.Stat(markerPath); !os.IsNotExist(statErr) {
		if statErr == nil {
			t.Fatalf("expected skills auto-check to be skipped after interrupt\noutput:\n%s", output.String())
		}
		t.Fatalf("failed to stat skills marker: %v", statErr)
	}

	if strings.Contains(output.String(), "skills updates may be available") {
		t.Fatalf("expected no skills update notice after interrupt, got %q", output.String())
	}
}

type ptyOutput struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (o *ptyOutput) Write(p []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	_, _ = o.buf.Write(p)
}

func (o *ptyOutput) String() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.buf.String()
}

func startPTYCapture(ptmx *os.File, prompt string) (*ptyOutput, <-chan struct{}, <-chan error) {
	output := &ptyOutput{}
	promptSeen := make(chan struct{})
	readDone := make(chan error, 1)
	var promptOnce sync.Once

	go func() {
		buf := make([]byte, 256)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
				if prompt != "" && strings.Contains(output.String(), prompt) {
					promptOnce.Do(func() {
						close(promptSeen)
					})
				}
			}
			if err != nil {
				readDone <- err
				return
			}
		}
	}()

	return output, promptSeen, readDone
}

func isolatedCLITestEnv(configPath string) []string {
	env := filterEnvVars(
		os.Environ(),
		"ASC_KEY_ID",
		"ASC_ISSUER_ID",
		"ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY",
		"ASC_PRIVATE_KEY_B64",
		"ASC_PROFILE",
		"ASC_CONFIG_PATH",
		"ASC_BYPASS_KEYCHAIN",
		"ASC_STRICT_AUTH",
		"ASC_APP_ID",
	)
	return append(env,
		"ASC_BYPASS_KEYCHAIN=1",
		"ASC_CONFIG_PATH="+configPath,
		"HOME="+filepath.Dir(configPath),
	)
}

func filterEnvVars(env []string, remove ...string) []string {
	removeSet := make(map[string]struct{}, len(remove))
	for _, key := range remove {
		removeSet[key] = struct{}{}
	}

	filtered := make([]string, 0, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		if _, exists := removeSet[parts[0]]; exists {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}
