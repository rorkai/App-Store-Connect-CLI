package cmd

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/telemetry"
)

func TestRunReleaseStageReadinessBlockersAreExpectedNegative(t *testing.T) {
	resetReportFlags(t)
	t.Setenv("ASC_APP_ID", "")

	const version = `{"type":"appStoreVersions","id":"ver-1","attributes":{"versionString":"1.0.0","platform":"IOS","appVersionState":"PREPARE_FOR_SUBMISSION"},"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}}`
	responses := map[string]string{
		"/v1/builds/build-1":                                      `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1","processingState":"VALID","buildAudienceType":"APP_STORE_ELIGIBLE"}}}`,
		"/v1/builds/build-1/relationships/app":                    `{"data":{"type":"apps","id":"app-1"}}`,
		"/v1/builds/build-1/preReleaseVersion":                    `{"data":{"type":"preReleaseVersions","id":"pre-1","attributes":{"version":"1.0.0","platform":"IOS"}}}`,
		"/v1/appStoreVersions/ver-1":                              `{"data":` + version + `}`,
		"/v1/appStoreVersions/ver-1/build":                        `{"data":{"type":"builds","id":"build-1","attributes":{"version":"1","processingState":"VALID"}}}`,
		"/v1/appStoreVersions/ver-0/appStoreVersionLocalizations": `{"data":[]}`,
		"/v1/appStoreVersions/ver-1/appStoreVersionLocalizations": `{"data":[]}`,
		"/v1/apps/app-1":                                          `{"data":{"type":"apps","id":"app-1","attributes":{"primaryLocale":"en-US"}}}`,
		"/v1/apps/app-1/appInfos":                                 `{"data":[{"type":"appInfos","id":"info-1","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}]}`,
		"/v1/appInfos/info-1/appInfoLocalizations":                `{"data":[]}`,
		"/v1/territories":                                         `{"data":[]}`,
		"/v1/apps/app-1/subscriptionGroups":                       `{"data":[]}`,
		"/v1/apps/app-1/inAppPurchasesV2":                         `{"data":[]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			t.Errorf("unexpected mutation %s %s", req.Method, req.URL.Path)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if req.URL.Path == "/v1/apps/app-1/appStoreVersions" {
			versionString := req.URL.Query().Get("filter[versionString]")
			id := "ver-1"
			if versionString == "0.9.0" {
				id = "ver-0"
			}
			fmt.Fprintf(w, `{"data":[{"type":"appStoreVersions","id":%q,"attributes":{"versionString":%q,"platform":"IOS"}}]}`, id, versionString)
			return
		}
		body, ok := responses[req.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`)
			return
		}
		fmt.Fprint(w, body)
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
			"release", "stage",
			"--app", "app-1",
			"--version", "1.0.0",
			"--build-id", "build-1",
			"--copy-metadata-from", "0.9.0",
			"--checkpoint-file", filepath.Join(t.TempDir(), "stage.json"),
			"--output", "json",
			"--confirm",
		}, "5.10.1"); code != ExitError {
			t.Fatalf("Run() exit code = %d, want %d", code, ExitError)
		}
	})

	if !strings.Contains(stdout, `"failedStep":"validate_readiness"`) {
		t.Fatalf("stdout = %q, want failedStep validate_readiness", stdout)
	}
	if !strings.HasPrefix(stderr, "Error: release stage: validate_readiness: ") ||
		strings.Count(stderr, "\n  - ") != 5 ||
		!strings.Contains(stderr, "\n  - no version localizations found: Add at least one App Store version localization\n") ||
		!strings.HasSuffix(stderr, "more\nRun: asc validate --app \"app-1\" --version \"1.0.0\" --platform \"IOS\"\n") {
		t.Fatalf("stderr = %q, want readiness summary with blockers and the validate command", stderr)
	}
	if gotExitCode != ExitError ||
		gotContext.OutcomeKind != telemetry.OutcomeExpectedNegative ||
		gotContext.FailureStage != telemetry.FailureStageValidation {
		t.Fatalf("telemetry exit=%d context=%+v, want expected_negative validation", gotExitCode, gotContext)
	}
}
