package cmd

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunAdsRejectedOAuthClientIsAuthError(t *testing.T) {
	resetReportFlags(t)
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "ads-key.pem")
	writeRunTestECDSAPEM(t, keyPath)
	for _, key := range []string{"ASC_ADS_ACCESS_TOKEN", "ASC_ADS_PRIVATE_KEY", "ASC_ADS_PRIVATE_KEY_B64", "ASC_ADS_PROFILE", "ASC_ADS_ORG_ID", "ASC_ADS_AD_ACCOUNT_ID", "ASC_ADS_STRICT_AUTH"} {
		t.Setenv(key, "")
	}
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(tempDir, "config.json"))
	t.Setenv("ASC_ADS_BYPASS_KEYCHAIN", "1")
	t.Setenv("ASC_ADS_CLIENT_ID", "SEARCHADS.CLIENT")
	t.Setenv("ASC_ADS_TEAM_ID", "SEARCHADS.TEAM")
	t.Setenv("ASC_ADS_KEY_ID", "KEY_ID")
	t.Setenv("ASC_ADS_PRIVATE_KEY_PATH", keyPath)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = metadataApplyRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/auth/oauth2/token" {
			t.Fatalf("unexpected request after token rejection: %s %s", req.Method, req.URL)
		}
		return runSubmitPreflightJSONResponse(http.StatusBadRequest, `{"error":"invalid_client"}`)
	})

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, _ int, eventContext telemetry.EventContext) {
		gotContext = eventContext
	}

	_, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"ads", "campaigns", "find", "--ad-account", "987654"}, "4.0.0"); code != ExitAuth {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitAuth)
		}
	})

	for _, want := range []string{"invalid_client", "ASC_ADS_CLIENT_ID", "asc ads auth login"} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr = %q, want it to mention %q", stderr, want)
		}
	}
	if gotContext.OutcomeKind != telemetry.OutcomeAuthError {
		t.Fatalf("telemetry outcome = %q, want %q", gotContext.OutcomeKind, telemetry.OutcomeAuthError)
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{"--error-format", "json", "ads", "campaigns", "find", "--ad-account", "987654"}, "4.0.0"); code != ExitAuth {
			t.Fatalf("JSON Run() exit code = %d, want %d", code, ExitAuth)
		}
	})
	var envelope struct {
		Error jsonError `json:"error"`
	}
	if err := json.Unmarshal([]byte(stderr), &envelope); err != nil {
		t.Fatalf("JSON stderr = %q: %v", stderr, err)
	}
	if stdout != "" || envelope.Error.Type != "authentication_required" || envelope.Error.ExitCode != ExitAuth ||
		!strings.Contains(envelope.Error.Hint, "asc ads auth status") || strings.Contains(stderr, "invalid_client") {
		t.Fatalf("unexpected JSON auth result: stdout=%q stderr=%q", stdout, stderr)
	}
}
