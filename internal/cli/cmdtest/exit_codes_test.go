package cmdtest

import (
	"flag"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// TestExitCodeConstantsMatch tests that exit codes from cmd package match expected values
func TestExitCodeConstantsMatch(t *testing.T) {
	tests := []struct {
		name     string
		expected int
		getter   func() int
	}{
		{"Success", 0, func() int { return cmd.ExitSuccess }},
		{"Error", 1, func() int { return cmd.ExitError }},
		{"Usage", 2, func() int { return cmd.ExitUsage }},
		{"Auth", 3, func() int { return cmd.ExitAuth }},
		{"NotFound", 4, func() int { return cmd.ExitNotFound }},
		{"Conflict", 5, func() int { return cmd.ExitConflict }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.getter(); got != tt.expected {
				t.Errorf("%s = %d, want %d", tt.name, got, tt.expected)
			}
		})
	}
}

// TestExitCodeMapper_NilError tests that nil error returns success
func TestExitCodeMapper_NilError(t *testing.T) {
	result := cmd.ExitCodeFromError(nil)
	if result != cmd.ExitSuccess {
		t.Errorf("ExitCodeFromError(nil) = %d, want %d", result, cmd.ExitSuccess)
	}
}

// TestExitCodeMapper_UsageError tests that flag.ErrHelp returns usage
func TestExitCodeMapper_UsageError(t *testing.T) {
	result := cmd.ExitCodeFromError(flag.ErrHelp)
	if result != cmd.ExitUsage {
		t.Errorf("ExitCodeFromError(flag.ErrHelp) = %d, want %d", result, cmd.ExitUsage)
	}
}

// TestExitCodeMapper_SharedErrors tests that shared.ErrMissingAuth returns auth exit
func TestExitCodeMapper_SharedErrors(t *testing.T) {
	result := cmd.ExitCodeFromError(shared.ErrMissingAuth)
	if result != cmd.ExitAuth {
		t.Errorf("ExitCodeFromError(shared.ErrMissingAuth) = %d, want %d", result, cmd.ExitAuth)
	}
}

// TestExitCodeMapper_ASCErrors tests that asc errors return correct exit codes
func TestExitCodeMapper_ASCErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"ErrUnauthorized", asc.ErrUnauthorized, cmd.ExitAuth},
		{"ErrForbidden", asc.ErrForbidden, cmd.ExitAuth},
		{"ErrNotFound", asc.ErrNotFound, cmd.ExitNotFound},
		{"ErrConflict", asc.ErrConflict, cmd.ExitConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmd.ExitCodeFromError(tt.err)
			if result != tt.want {
				t.Errorf("ExitCodeFromError(%s) = %d, want %d", tt.name, result, tt.want)
			}
		})
	}
}

// TestExitCodeMapper_APIError tests that APIError with specific codes returns correct exit codes
func TestExitCodeMapper_APIError(t *testing.T) {
	tests := []struct {
		name string
		code string
		want int
	}{
		{"NOT_FOUND", "NOT_FOUND", cmd.ExitNotFound},
		{"CONFLICT", "CONFLICT", cmd.ExitConflict},
		{"UNAUTHORIZED", "UNAUTHORIZED", cmd.ExitAuth},
		{"FORBIDDEN", "FORBIDDEN", cmd.ExitAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &asc.APIError{Code: tt.code, Title: "Test", Detail: "Detail"}
			result := cmd.ExitCodeFromError(err)
			if result != tt.want {
				t.Errorf("ExitCodeFromError(APIError[%s]) = %d, want %d", tt.code, result, tt.want)
			}
		})
	}
}

// TestRun_MissingAuth tests that commands without auth return exit code 3
func TestRun_MissingAuth(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"apps", "list"}, "1.0.0")
		if code != cmd.ExitAuth {
			t.Errorf("expected exit code %d, got %d", cmd.ExitAuth, code)
		}
	})

	if !strings.Contains(stderr, "authentication") && !strings.Contains(stderr, "auth") {
		t.Errorf("expected auth-related error, got: %s", stderr)
	}
}

// TestRun_ReportFileWithoutReport tests that --report-file without --report returns usage error
func TestRun_ReportFileWithoutReport(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"--report-file", "/tmp/report.xml", "apps", "list"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Errorf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--report") {
		t.Errorf("expected --report error message, got: %s", stderr)
	}
}

// TestRun_InvalidReportFlag tests that invalid --report returns usage error
func TestRun_InvalidReportFlag(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"--report", "invalid", "apps", "list"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Errorf("expected exit code %d, got %d", cmd.ExitUsage, code)
		}
	})

	if !strings.Contains(stderr, "--report") {
		t.Errorf("expected --report error message, got: %s", stderr)
	}
}

// TestRun_UsageValidationErrorsReturnExitUsage verifies representative command-level
// validation failures map to usage exit code (2).
func TestRun_UsageValidationErrorsReturnExitUsage(t *testing.T) {
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_KEY_ID", "")
	t.Setenv("ASC_ISSUER_ID", "")
	t.Setenv("ASC_PRIVATE_KEY_PATH", "")
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_CONFIG_PATH", t.TempDir()+"/config.json")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "submit create conflicting version flags",
			args:    []string{"submit", "create", "--version", "1.0", "--version-id", "V123", "--build", "B1", "--confirm"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name: "auth login mutually exclusive validation flags",
			args: []string{
				"auth", "login",
				"--name", "demo",
				"--key-id", "KEY",
				"--issuer-id", "ISS",
				"--private-key", "/tmp/AuthKey.p8",
				"--skip-validation",
				"--network",
			},
			wantErr: "--skip-validation and --network are mutually exclusive",
		},
		{
			name:    "app-info get conflicting version flags",
			args:    []string{"app-info", "get", "--app", "APP_ID", "--version", "1.0.0", "--version-id", "VERSION_ID"},
			wantErr: "--version and --version-id are mutually exclusive",
		},
		{
			name:    "performance download mutually exclusive selectors",
			args:    []string{"performance", "download", "--app", "APP_ID", "--build", "BUILD_ID"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "xcode-cloud run mutually exclusive workflow flags",
			args:    []string{"xcode-cloud", "run", "--workflow", "CI", "--workflow-id", "WF_ID", "--branch", "main"},
			wantErr: "--workflow and --workflow-id are mutually exclusive",
		},
		{
			name:    "publish appstore invalid timeout",
			args:    []string{"publish", "appstore", "--app", "APP_123", "--ipa", "app.ipa", "--version", "1.0.0", "--timeout", "-1s"},
			wantErr: "--timeout must be greater than 0",
		},
		{
			name:    "builds list invalid processing-state",
			args:    []string{"builds", "list", "--app", "APP_123", "--processing-state", "WRONG"},
			wantErr: "--processing-state must be one of",
		},
		{
			name:    "builds wait missing selector",
			args:    []string{"builds", "wait"},
			wantErr: "--build is required, or provide --app and --build-number",
		},
		{
			name:    "builds find missing build-number",
			args:    []string{"builds", "find", "--app", "APP_123"},
			wantErr: "--build-number is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, stderr := captureOutput(t, func() {
				code := cmd.Run(test.args, "1.0.0")
				if code != cmd.ExitUsage {
					t.Fatalf("expected exit code %d, got %d", cmd.ExitUsage, code)
				}
			})
			if !strings.Contains(stderr, test.wantErr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantErr, stderr)
			}
		})
	}
}
