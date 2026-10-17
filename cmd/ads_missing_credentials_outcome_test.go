package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunAdsMissingCredentialsIsAuthError(t *testing.T) {
	const remediation = "run 'asc ads auth login' to store Apple Ads credentials, set ASC_ADS_* environment credentials, or pass --ads-profile"

	tests := []struct {
		name       string
		config     string
		args       []string
		wantStderr string
	}{
		{
			name:       "auth discover without config",
			args:       []string{"ads", "auth", "discover"},
			wantStderr: "Error: ads auth discover: configuration not found; " + remediation,
		},
		{
			name:       "auth discover without default profile",
			config:     `{}`,
			args:       []string{"ads", "auth", "discover"},
			wantStderr: "Error: ads auth discover: default credentials not found; " + remediation,
		},
		{
			name:       "auth discover with profile and no config",
			args:       []string{"ads", "auth", "discover", "--ads-profile", "Missing"},
			wantStderr: "Error: ads auth discover: configuration not found",
		},
		{
			name:       "auth discover with unknown profile",
			config:     `{}`,
			args:       []string{"ads", "auth", "discover", "--ads-profile", "Missing"},
			wantStderr: `Error: ads auth discover: credentials not found for profile "Missing"`,
		},
		{
			name:       "me view without config",
			args:       []string{"ads", "me", "view"},
			wantStderr: "Error: ads: configuration not found; " + remediation,
		},
		{
			name:       "auth token without config",
			args:       []string{"ads", "auth", "token", "--confirm"},
			wantStderr: "Error: ads auth token: configuration not found; " + remediation,
		},
		{
			name:       "api request with unknown profile",
			config:     `{}`,
			args:       []string{"ads", "api", "request", "--method", "GET", "--path", "v1/me", "--ads-profile", "Missing"},
			wantStderr: `Error: ads api request: credentials not found for profile "Missing"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetReportFlags(t)
			tempDir := t.TempDir()
			for _, key := range []string{
				"ASC_ADS_ACCESS_TOKEN", "ASC_ADS_PROFILE", "ASC_ADS_STRICT_AUTH",
				"ASC_ADS_CLIENT_ID", "ASC_ADS_TEAM_ID", "ASC_ADS_KEY_ID",
				"ASC_ADS_PRIVATE_KEY", "ASC_ADS_PRIVATE_KEY_B64", "ASC_ADS_PRIVATE_KEY_PATH",
				"ASC_ADS_ORG_ID", "ASC_ADS_AD_ACCOUNT_ID",
			} {
				t.Setenv(key, "")
			}
			configPath := filepath.Join(tempDir, "config.json")
			t.Setenv("HOME", tempDir)
			t.Setenv("ASC_CONFIG_PATH", configPath)
			t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
			if test.config != "" {
				if err := os.WriteFile(configPath, []byte(test.config), 0o600); err != nil {
					t.Fatalf("WriteFile() error: %v", err)
				}
			}

			originalTransport := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = originalTransport })
			http.DefaultTransport = metadataApplyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				t.Fatalf("unexpected request without credentials: %s %s", req.Method, req.URL)
				return nil, nil
			})

			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "5.15.0"); code != ExitAuth {
					t.Errorf("Run() exit code = %d, want %d", code, ExitAuth)
				}
			})

			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if strings.TrimSpace(stderr) != test.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, test.wantStderr)
			}
			if gotExitCode != ExitAuth ||
				gotContext.OutcomeKind != telemetry.OutcomeAuthError ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.DiagnosticCode != "" ||
				gotContext.FailureParameter != "" {
				t.Fatalf("telemetry exit=%d outcome=%q stage=%q diagnostic=%q parameter=%q, want exit=%d outcome=%q stage=%q and no diagnostic or parameter",
					gotExitCode, gotContext.OutcomeKind, gotContext.FailureStage, gotContext.DiagnosticCode, gotContext.FailureParameter,
					ExitAuth, telemetry.OutcomeAuthError, telemetry.FailureStageValidation)
			}
		})
	}
}
