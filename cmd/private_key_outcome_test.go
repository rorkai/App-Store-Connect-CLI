package cmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunUnusableCredentialSetupIsAuthErrorWithDiagnostic(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "AuthKey_KEY123.p8")
	tests := []struct {
		name          string
		config        string
		configMode    os.FileMode
		env           map[string]string
		wantStderr    string
		wantDiagnosis shared.DiagnosticCode
		wantParameter string
	}{
		{
			name:          "missing private key file",
			env:           map[string]string{"ASC_KEY_ID": "KEY123", "ASC_ISSUER_ID": "ISSUER123", "ASC_PRIVATE_KEY_PATH": keyPath},
			wantStderr:    "invalid private key: private key file not found",
			wantDiagnosis: shared.DiagnosticFileNotFound,
			wantParameter: "--private-key",
		},
		{
			name:          "undecodable private key variable",
			env:           map[string]string{"ASC_KEY_ID": "KEY123", "ASC_ISSUER_ID": "ISSUER123", "ASC_PRIVATE_KEY_B64": "!!!"},
			wantStderr:    "invalid private key environment: ASC_PRIVATE_KEY_B64: illegal base64 data",
			wantDiagnosis: shared.DiagnosticInvalidInput,
		},
		{
			name:          "unreadable config file",
			config:        `{}`,
			configMode:    0o000,
			wantStderr:    "failed to read config: ",
			wantDiagnosis: shared.DiagnosticFilePermissionDenied,
		},
		{
			name:          "strict auth with an ignored key ID variable",
			config:        `{"key_id":"KEY123","issuer_id":"ISSUER123","private_key_path":"` + filepath.ToSlash(keyPath) + `"}`,
			configMode:    0o600,
			env:           map[string]string{"ASC_STRICT_AUTH": "1", "ASC_KEY_ID": "OTHER456"},
			wantStderr:    "mixed authentication sources detected: ASC_KEY_ID ignored",
			wantDiagnosis: shared.DiagnosticConflictingInput,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.configMode == 0 && test.config != "" && (runtime.GOOS == "windows" || os.Geteuid() == 0) {
				t.Skip("file modes do not deny reads here")
			}
			resetReportFlags(t)
			resetSelectedProfile(t)
			configPath := filepath.Join(t.TempDir(), "config.json")
			if test.config != "" {
				if err := os.WriteFile(configPath, []byte(test.config), test.configMode); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
			t.Setenv("ASC_CONFIG_PATH", configPath)
			for _, name := range []string{"ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_KEY_TYPE", "ASC_PRIVATE_KEY_PATH", "ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_STRICT_AUTH"} {
				t.Setenv(name, test.env[name])
			}

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run([]string{"apps", "list"}, "4.0.0"); code != ExitAuth {
					t.Errorf("Run() exit code = %d, want %d", code, ExitAuth)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) || !strings.Contains(stderr, "asc auth login") {
				t.Fatalf("stderr = %q, want %q and an auth login hint", stderr, test.wantStderr)
			}
			if gotExitCode != ExitAuth ||
				gotContext.OutcomeKind != telemetry.OutcomeAuthError ||
				gotContext.DiagnosticCode != string(test.wantDiagnosis) ||
				gotContext.FailureParameter != test.wantParameter {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}
