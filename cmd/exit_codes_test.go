package cmd

import (
	"encoding/xml"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
	if !strings.Contains(stderr, "--id cannot be combined with --build or --locale") {
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
