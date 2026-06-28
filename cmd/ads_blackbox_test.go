package cmd

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdsUsageErrorsExitTwoWithBuiltBinary(t *testing.T) {
	binaryPath := buildASCBlackboxBinary(t)

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "invalid endpoint output",
			args:       []string{"ads", "campaigns", "--output", "xml"},
			wantStderr: "unsupported format: xml",
		},
		{
			name:       "unexpected endpoint arg",
			args:       []string{"ads", "campaigns", "--output", "json", "unexpected"},
			wantStderr: "unexpected argument(s): unexpected",
		},
		{
			name:       "missing destructive confirm",
			args:       []string{"ads", "campaigns", "delete", "--campaign", "123"},
			wantStderr: "--confirm is required",
		},
		{
			name:       "missing required query flag",
			args:       []string{"ads", "apps", "search", "--org", "123456", "--output", "json"},
			wantStderr: "--query is required",
		},
		{
			name:       "invalid raw api method",
			args:       []string{"ads", "api", "request", "--method", "PATCH", "--path", "v5/campaigns"},
			wantStderr: "--method must be one of: GET, POST, PUT, DELETE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "config.json")
			runCmd := exec.Command(binaryPath, tt.args...)
			runCmd.Env = isolatedAdsBlackboxEnv(configPath)
			output, err := runCmd.CombinedOutput()
			if err == nil {
				t.Fatalf("expected usage failure, got success\n%s", output)
			}
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got %T: %v\n%s", err, err, output)
			}
			if got := exitErr.ExitCode(); got != ExitUsage {
				t.Fatalf("exit code = %d, want %d\n%s", got, ExitUsage, output)
			}
			if !strings.Contains(string(output), tt.wantStderr) {
				t.Fatalf("output = %q, want %q", output, tt.wantStderr)
			}
		})
	}
}

func isolatedAdsBlackboxEnv(configPath string) []string {
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
		"ASC_ADS_ACCESS_TOKEN",
		"ASC_ADS_CLIENT_ID",
		"ASC_ADS_TEAM_ID",
		"ASC_ADS_KEY_ID",
		"ASC_ADS_PRIVATE_KEY_PATH",
		"ASC_ADS_PRIVATE_KEY",
		"ASC_ADS_PRIVATE_KEY_B64",
		"ASC_ADS_ORG_ID",
		"ASC_ADS_PROFILE",
		"ASC_ADS_BYPASS_KEYCHAIN",
		"ASC_ADS_STRICT_AUTH",
	)
	return append(
		env,
		"ASC_BYPASS_KEYCHAIN=1",
		"ASC_ADS_BYPASS_KEYCHAIN=1",
		"ASC_CONFIG_PATH="+configPath,
		"HOME="+filepath.Dir(configPath),
	)
}
