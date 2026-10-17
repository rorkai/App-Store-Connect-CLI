package cmd

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunExistingReportOutputFileIsClassified(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")
	t.Setenv("ASC_VENDOR_NUMBER", "")
	t.Setenv("ASC_ANALYTICS_VENDOR_NUMBER", "")

	var report bytes.Buffer
	gz := gzip.NewWriter(&report)
	_, _ = gz.Write([]byte("Provider\tUnits\nAPPLE\t1\n"))
	_ = gz.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/salesReports" {
			w.Header().Set("Content-Type", "application/a-gzip")
			_, _ = w.Write(report.Bytes())
			return
		}
		t.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) { return client, nil }))

	outputPath := filepath.Join(t.TempDir(), "sales.tsv.gz")
	if err := os.WriteFile(outputPath, []byte("previous run"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	args := []string{"analytics", "sales", "--vendor", "12345678", "--type", "SALES", "--subtype", "SUMMARY", "--frequency", "DAILY", "--date", "2026-10-01", "--output", outputPath}
	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run(args, "4.0.0"); code != ExitError {
			t.Errorf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "output file already exists") {
		t.Fatalf("stderr = %q, want existing-output guidance", stderr)
	}
	if data, err := os.ReadFile(outputPath); err != nil || string(data) != "previous run" {
		t.Fatalf("existing output = %q, %v; want it left untouched", data, err)
	}
	if gotExitCode != ExitError ||
		gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.DiagnosticCode != string(shared.DiagnosticResourceConflict) {
		t.Fatalf("telemetry exit=%d context=%+v, want expected_negative resource_conflict", gotExitCode, gotContext)
	}
}
