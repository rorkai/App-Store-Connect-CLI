package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunReviewSubmitMissingVersionIsNotFound(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/v1/apps/app-1/appStoreVersions" {
			t.Fatalf("request = %s %s, want GET /v1/apps/app-1/appStoreVersions", req.Method, req.URL.Path)
		}
		if got := req.URL.Query().Get("filter[versionString]"); got != "9.9.9" {
			t.Fatalf("filter[versionString] = %q, want 9.9.9", got)
		}
		if got := req.URL.Query().Get("filter[platform]"); got != "IOS" {
			t.Fatalf("filter[platform] = %q, want IOS", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[]}`)
	}))
	defer server.Close()

	client := newHTTPStatusTestClient(t, server.URL)
	t.Cleanup(shared.SetASCClientFactoryForTesting(func() (*asc.Client, error) {
		return client, nil
	}))

	originalEmitTelemetry := emitTelemetry
	t.Cleanup(func() { emitTelemetry = originalEmitTelemetry })
	var gotExitCode int
	var gotContext telemetry.EventContext
	emitTelemetry = func(_ string, _ string, _ time.Duration, exitCode int, eventContext telemetry.EventContext) {
		gotExitCode = exitCode
		gotContext = eventContext
	}

	stdout, stderr := captureCommandOutput(t, func() {
		if code := Run([]string{
			"review", "submit",
			"--app", "app-1",
			"--version", "9.9.9",
			"--build", "build-1",
			"--dry-run",
		}, "4.0.0"); code != ExitNotFound {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitNotFound)
		}
	})

	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `app store version not found for version "9.9.9" and platform "IOS"`) {
		t.Fatalf("stderr = %q, want missing-version diagnostic", stderr)
	}
	if gotExitCode != ExitNotFound || gotContext.OutcomeKind != telemetry.OutcomeNotFound {
		t.Fatalf("unexpected telemetry: exit=%d context=%+v", gotExitCode, gotContext)
	}
}
