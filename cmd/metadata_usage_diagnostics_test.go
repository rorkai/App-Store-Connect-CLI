package cmd

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

type metadataUsageRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn metadataUsageRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestRunMetadataRequiredInputsAreConciseAndStructured(t *testing.T) {
	resetReportFlags(t)
	for _, key := range []string{
		"ASC_APP_ID", "ASC_PROFILE", "ASC_KEY_ID", "ASC_ISSUER_ID", "ASC_PRIVATE_KEY_PATH",
		"ASC_PRIVATE_KEY", "ASC_PRIVATE_KEY_B64", "ASC_KEY_TYPE", "ASC_STRICT_AUTH",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = metadataUsageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("metadata required-input validation unexpectedly reached the network: " + req.URL.String())
	})

	tests := []struct {
		name          string
		args          []string
		wantError     string
		wantStderr    string
		wantParameter string
	}{
		{
			name:          "pull missing app",
			args:          []string{"metadata", "pull", "--version", "1.2.3", "--dir", "./metadata"},
			wantError:     "--app is required (or set ASC_APP_ID)",
			wantParameter: "--app",
		},
		{
			name:          "pull missing dir",
			args:          []string{"metadata", "pull", "--app", "app-1", "--version", "1.2.3"},
			wantError:     "--dir is required",
			wantParameter: "--dir",
		},
		{
			name:          "push missing app",
			args:          []string{"metadata", "push", "--version", "1.2.3", "--dir", "./metadata"},
			wantError:     "--app is required (or set ASC_APP_ID)",
			wantParameter: "--app",
		},
		{
			name:          "push missing version",
			args:          []string{"metadata", "push", "--app", "app-1", "--dir", "./metadata"},
			wantError:     "--version is required. Pass the App Store version string, for example --version 1.2.3. If you have version localizations, match the folder under version/ in --dir.",
			wantParameter: "--version",
		},
		{
			name:          "push missing dir",
			args:          []string{"metadata", "push", "--app", "app-1", "--version", "1.2.3"},
			wantError:     "--dir is required",
			wantParameter: "--dir",
		},
		{
			name:          "plan missing app",
			args:          []string{"metadata", "plan", "--version", "1.2.3", "--dir", "./metadata"},
			wantError:     "--app is required (or set ASC_APP_ID)",
			wantParameter: "--app",
		},
		{
			name:          "plan missing version",
			args:          []string{"metadata", "plan", "--app", "app-1", "--dir", "./metadata"},
			wantError:     "--version is required. Pass the App Store version string, for example --version 1.2.3. If you have version localizations, match the folder under version/ in --dir.",
			wantParameter: "--version",
		},
		{
			name:          "plan missing dir",
			args:          []string{"metadata", "plan", "--app", "app-1", "--version", "1.2.3"},
			wantError:     "--dir is required",
			wantParameter: "--dir",
		},
		{
			name:          "apply missing app",
			args:          []string{"metadata", "apply", "--version", "1.2.3", "--dir", "./metadata"},
			wantError:     "--app is required (or set ASC_APP_ID)",
			wantParameter: "--app",
		},
		{
			name:          "apply missing version",
			args:          []string{"metadata", "apply", "--app", "app-1", "--dir", "./metadata"},
			wantError:     "--version is required. Pass the App Store version string, for example --version 1.2.3. If you have version localizations, match the folder under version/ in --dir.",
			wantParameter: "--version",
		},
		{
			name:          "apply missing dir",
			args:          []string{"metadata", "apply", "--app", "app-1", "--version", "1.2.3"},
			wantError:     "--dir is required",
			wantParameter: "--dir",
		},
		{
			name:          "validate missing dir",
			args:          []string{"metadata", "validate"},
			wantError:     "--dir is required",
			wantParameter: "--dir",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalEmitTelemetry := emitTelemetry
			t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })

			var gotExitCode int
			var gotContext telemetry.EventContext
			emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
				gotExitCode = exitCode
				gotContext = eventContext
			}

			stdout, stderr := captureCommandOutput(t, func() {
				if code := Run(test.args, "1.2.3"); code != ExitUsage {
					t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
				}
			})

			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			want := test.wantStderr
			if want == "" {
				want = "Error: " + test.wantError + "\n"
			}
			if stderr != want {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
			if strings.Contains(stderr, "DESCRIPTION") || strings.Contains(stderr, "USAGE") || strings.Contains(stderr, "FLAGS") {
				t.Fatalf("required-input failure dumped command help: %q", stderr)
			}
			if gotExitCode != ExitUsage ||
				gotContext.ErrorKind != telemetry.ErrorKindMissingRequired ||
				gotContext.FailureStage != telemetry.FailureStageValidation ||
				gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
				gotContext.FailureParameter != test.wantParameter ||
				gotContext.DiagnosticCode != string(shared.DiagnosticRequiredInputMissing) {
				t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
			}
		})
	}
}

func TestRunMetadataAmbiguousVersionRetainsMissingPlatformClassification(t *testing.T) {
	resetReportFlags(t)
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "AuthKey.p8")
	writeRunTestECDSAPEM(t, keyPath)
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "missing.json"))
	t.Setenv("ASC_PROFILE", "")
	t.Setenv("ASC_KEY_ID", "ENVKEY")
	t.Setenv("ASC_ISSUER_ID", "ENVISS")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)
	t.Setenv("ASC_PRIVATE_KEY", "")
	t.Setenv("ASC_PRIVATE_KEY_B64", "")
	t.Setenv("ASC_STRICT_AUTH", "")
	t.Setenv("ASC_MAX_RETRIES", "0")
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_APP_ID", "")
	resetSelectedProfile(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = metadataUsageRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/123/appStoreVersions" {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"data":[
				{"type":"appStoreVersions","id":"version-ios","attributes":{"versionString":"1.2.3","platform":"IOS"}},
				{"type":"appStoreVersions","id":"version-mac","attributes":{"versionString":"1.2.3","platform":"MAC_OS"}}
			],"links":{"next":""}}`)),
		}, nil
	})

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	var gotExitCode int
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"metadata", "pull", "--app", "123", "--version", "1.2.3", "--dir", filepath.Join(tempDir, "metadata")}, "1.2.3"); code != ExitUsage {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
		}
	})
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `pass --platform with one of:`) {
		t.Fatalf("stderr = %q, want platform guidance", stderr)
	}
	if gotExitCode != ExitUsage ||
		gotContext.ErrorKind != telemetry.ErrorKindMissingRequired ||
		gotContext.FailureStage != telemetry.FailureStageValidation ||
		gotContext.OutcomeKind != telemetry.OutcomeUsageError ||
		gotContext.FailureParameter != "--platform" ||
		gotContext.DiagnosticCode != "" {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}
