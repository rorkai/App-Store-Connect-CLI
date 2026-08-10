package release

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/metadata"
	validatecli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/validate"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type releaseRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn releaseRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func releaseJSONResponse(status int, body string) (*http.Response, error) {
	return &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func captureReleaseStderr(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	readDone := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		readDone <- string(data)
	}()

	os.Stderr = writer
	defer func() {
		os.Stderr = originalStderr
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stderr writer: %v", err)
	}
	return <-readDone
}

func newReleaseTestClient(t *testing.T) *asc.Client {
	t.Helper()
	pemBytes := newReleaseTestPrivateKeyPEM(t)

	client, err := asc.NewClientFromPEM("KEY_ID", "ISSUER_ID", string(pemBytes))
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return client
}

func newReleaseTestServerClient(t *testing.T, handler http.Handler) (*asc.Client, string) {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	serverTransport := server.Client().Transport
	httpClient := &http.Client{Transport: releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = target.Scheme
		cloned.URL.Host = target.Host
		cloned.Host = target.Host
		return serverTransport.RoundTrip(cloned)
	})}

	pemBytes := newReleaseTestPrivateKeyPEM(t)
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pemBytes, 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY_ID", "ISSUER_ID", keyPath, httpClient)
	if err != nil {
		t.Fatalf("new client with test server: %v", err)
	}
	return client, server.URL
}

func writeReleaseTestJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func newReleaseTestPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if pemBytes == nil {
		t.Fatal("encode pem: nil")
	}
	return pemBytes
}

func TestReleaseCommandShape(t *testing.T) {
	cmd := ReleaseCommand()
	if cmd == nil {
		t.Fatal("expected release command")
		return
	}
	if cmd.Name != "release" {
		t.Fatalf("expected command name release, got %q", cmd.Name)
	}
	if len(cmd.Subcommands) != 1 {
		t.Fatalf("expected 1 subcommand, got %d", len(cmd.Subcommands))
	}
	if cmd.Subcommands[0].Name != "stage" {
		t.Fatalf("expected subcommand stage, got %q", cmd.Subcommands[0].Name)
	}
}

func TestReleaseStageCommand_MissingRequiredFlags(t *testing.T) {
	cmd := ReleaseStageCommand()
	if err := cmd.FlagSet.Parse([]string{"--dry-run"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	err := cmd.Exec(context.Background(), nil)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}

func TestReleaseStageCommandExposesRoutingCoverageFile(t *testing.T) {
	cmd := ReleaseStageCommand()
	flag := cmd.FlagSet.Lookup("routing-coverage-file")
	if flag == nil {
		t.Fatal("expected --routing-coverage-file flag")
	}
	if !strings.HasPrefix(flag.Usage, "[experimental] ") {
		t.Fatalf("expected --routing-coverage-file to be introduced as experimental, got %q", flag.Usage)
	}
	if !strings.Contains(flag.Usage, "before readiness") {
		t.Fatalf("expected routing coverage timing in flag help, got %q", flag.Usage)
	}
}

func TestReleaseStageCommandValidatesRoutingCoverageBeforePipeline(t *testing.T) {
	originalClientFactory := releaseClientFactory
	t.Cleanup(func() { releaseClientFactory = originalClientFactory })
	clientCalled := false
	releaseClientFactory = func() (*asc.Client, error) {
		clientCalled = true
		return nil, errors.New("client must not be created")
	}

	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(`{"type":"MultiPolygon","coordinates":`), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	t.Chdir(filepath.Dir(coveragePath))

	cmd := ReleaseStageCommand()
	if err := cmd.FlagSet.Parse([]string{
		"--app", "APP_123",
		"--version", "2.4.0",
		"--build", "BUILD_123",
		"--copy-metadata-from", "2.3.2",
		"--routing-coverage-file", coveragePath,
		"--dry-run",
	}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	var execErr error
	stderr := captureReleaseStderr(t, func() {
		execErr = cmd.Exec(context.Background(), nil)
	})
	if !errors.Is(execErr, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", execErr)
	}
	if !strings.Contains(stderr, "--routing-coverage-file is not usable") || !strings.Contains(stderr, "invalid JSON") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	if clientCalled {
		t.Fatal("release pipeline started before routing coverage validation")
	}
}

func TestDefaultCheckpointPathSanitizesValues(t *testing.T) {
	path := defaultCheckpointPath("app/123", "1.2.3-beta", "build#12", "IOS")
	want := filepath.Join(".asc", "release", "checkpoints", "app_123_1.2.3-beta_build_12_IOS.json")
	if path != want {
		t.Fatalf("unexpected checkpoint path: got %q want %q", path, want)
	}
}

func TestCheckpointModeMatches(t *testing.T) {
	tests := []struct {
		name        string
		existing    string
		desired     string
		wantMatched bool
	}{
		{name: "legacy run checkpoint", existing: "", desired: releaseModeRun, wantMatched: true},
		{name: "legacy stage mismatch", existing: "", desired: releaseModeStage, wantMatched: false},
		{name: "trimmed run mode", existing: "  run  ", desired: releaseModeRun, wantMatched: true},
		{name: "trimmed stage mode", existing: "\tstage\n", desired: releaseModeStage, wantMatched: true},
		{name: "mismatched mode", existing: "run", desired: releaseModeStage, wantMatched: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkpointModeMatches(tt.existing, tt.desired)
			if got != tt.wantMatched {
				t.Fatalf("checkpointModeMatches(%q, %q) = %v, want %v", tt.existing, tt.desired, got, tt.wantMatched)
			}
		})
	}
}

func TestExecuteStageResumesPartialCheckpointAfterRemovingRoutingCoverage(t *testing.T) {
	originalClientFactory := releaseClientFactory
	originalCopyExecutor := metadataCopyExecutor
	originalReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		releaseClientFactory = originalClientFactory
		metadataCopyExecutor = originalCopyExecutor
		readinessReportBuilder = originalReadinessBuilder
	})

	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"processingState":"VALID"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	releaseClientFactory = func() (*asc.Client, error) { return client, nil }
	metadataRuns := 0
	metadataCopyExecutor = func(context.Context, *asc.Client, metadataCopyOptions) (*asc.AppStoreVersionMetadataCopySummary, error) {
		metadataRuns++
		return &asc.AppStoreVersionMetadataCopySummary{CopiedLocales: 1}, nil
	}
	readinessRuns := 0
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessRuns++
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
	if err := saveCheckpoint(checkpointPath, runCheckpoint{
		AppID:               "APP_123",
		Version:             "2.4.0",
		BuildID:             "BUILD_123",
		CopyMetadataFrom:    "2.3.2",
		RoutingCoverageFile: filepath.Join(t.TempDir(), "invalid-coverage.geojson"),
		Platform:            "IOS",
		VersionID:           "VERSION_123",
		Mode:                releaseModeStage,
		Completed: map[string]bool{
			stepEnsureVersion: true,
			stepApplyMetadata: true,
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	result, err := executeStage(context.Background(), runOptions{
		AppID:            "APP_123",
		Version:          "2.4.0",
		BuildID:          "BUILD_123",
		CopyMetadataFrom: "2.3.2",
		Platform:         "IOS",
		Timeout:          releaseRunTimeout,
		Confirm:          true,
		CheckpointFile:   checkpointPath,
	})
	if err != nil {
		t.Fatalf("executeStage() error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("executeStage() resumed = false, want true")
	}
	if len(result.Steps) != 4 {
		t.Fatalf("executeStage() steps = %#v, want four steps without routing coverage", result.Steps)
	}
	for _, step := range result.Steps {
		if step.Name == stepApplyRoutingCoverage {
			t.Fatalf("executeStage() retained removed routing coverage step: %#v", result.Steps)
		}
	}
	if metadataRuns != 1 || readinessRuns != 1 {
		t.Fatalf("executeStage() reruns metadata=%d readiness=%d, want 1 each", metadataRuns, readinessRuns)
	}
	saved, err := loadCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if saved == nil {
		t.Fatal("load checkpoint = nil")
	}
	if saved.RoutingCoverageFile != "" {
		t.Fatalf("saved routingCoverageFile = %q, want empty", saved.RoutingCoverageFile)
	}
	if saved.Completed[stepApplyRoutingCoverage] {
		t.Fatalf("saved checkpoint claims removed routing coverage complete: %#v", saved.Completed)
	}
}

func TestExecuteStageRejectsRemovingRoutingCoverageAfterCompletedOrUnknownStep(t *testing.T) {
	originalClientFactory := releaseClientFactory
	t.Cleanup(func() { releaseClientFactory = originalClientFactory })

	for _, completedStep := range []string{
		stepApplyRoutingCoverage,
		stepAttachBuild,
		stepValidateReadiness,
		stepSubmitReview,
		"unrecognized_step",
	} {
		t.Run(completedStep, func(t *testing.T) {
			clientCalled := false
			releaseClientFactory = func() (*asc.Client, error) {
				clientCalled = true
				return nil, errors.New("client must not be created")
			}
			checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
			if err := saveCheckpoint(checkpointPath, runCheckpoint{
				AppID:               "APP_123",
				Version:             "2.4.0",
				BuildID:             "BUILD_123",
				CopyMetadataFrom:    "2.3.2",
				RoutingCoverageFile: filepath.Join(t.TempDir(), "coverage.geojson"),
				Platform:            "IOS",
				VersionID:           "VERSION_123",
				Mode:                releaseModeStage,
				Completed: map[string]bool{
					stepEnsureVersion: true,
					completedStep:     true,
				},
			}); err != nil {
				t.Fatalf("save checkpoint: %v", err)
			}

			_, err := executeStage(context.Background(), runOptions{
				AppID:            "APP_123",
				Version:          "2.4.0",
				BuildID:          "BUILD_123",
				CopyMetadataFrom: "2.3.2",
				Platform:         "IOS",
				Timeout:          releaseRunTimeout,
				Confirm:          true,
				CheckpointFile:   checkpointPath,
			})
			if err == nil || !strings.Contains(err.Error(), "checkpoint does not match current run arguments") {
				t.Fatalf("executeStage() error = %v, want checkpoint mismatch", err)
			}
			if clientCalled {
				t.Fatal("executeStage() created a client before rejecting unsafe checkpoint transition")
			}
		})
	}
}

func TestExecuteStageResumesCheckpointAfterAddingRoutingCoverage(t *testing.T) {
	originalClientFactory := releaseClientFactory
	originalCopyExecutor := metadataCopyExecutor
	originalReadinessBuilder := readinessReportBuilder
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = originalClientFactory
		metadataCopyExecutor = originalCopyExecutor
		readinessReportBuilder = originalReadinessBuilder
		http.DefaultTransport = originalTransport
	})

	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	checkpointPath := filepath.Join(t.TempDir(), "stage-checkpoint.json")
	if err := saveCheckpoint(checkpointPath, runCheckpoint{
		AppID:            "APP_123",
		Version:          "2.4.0",
		BuildID:          "BUILD_123",
		CopyMetadataFrom: "2.3.2",
		Platform:         "IOS",
		VersionID:        "VERSION_123",
		Mode:             releaseModeStage,
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
		},
	}); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	metadataRuns := 0
	metadataCopyExecutor = func(context.Context, *asc.Client, metadataCopyOptions) (*asc.AppStoreVersionMetadataCopySummary, error) {
		metadataRuns++
		return &asc.AppStoreVersionMetadataCopySummary{CopiedLocales: 1}, nil
	}
	coverageCommitted := false
	readinessRuns := 0
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessRuns++
		if !coverageCommitted {
			t.Fatal("readiness ran before newly added routing coverage completed")
		}
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404"}]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			return releaseJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/coverage","length":%d,"offset":0}]}}}`, len(validReleaseRoutingCoverageGeoJSON)))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return releaseJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			coverageCommitted = true
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})
	releaseClientFactory = func() (*asc.Client, error) { return newReleaseTestClient(t), nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:               "APP_123",
		Version:             "2.4.0",
		BuildID:             "BUILD_123",
		CopyMetadataFrom:    "2.3.2",
		RoutingCoverageFile: coveragePath,
		Platform:            "IOS",
		Timeout:             releaseRunTimeout,
		Confirm:             true,
		CheckpointFile:      checkpointPath,
	})
	if err != nil {
		t.Fatalf("executeStage() error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("executeStage() resumed = false, want true")
	}
	if metadataRuns != 1 || readinessRuns != 1 || !coverageCommitted {
		t.Fatalf("executeStage() metadata=%d readiness=%d coverageCommitted=%t, want 1, 1, true", metadataRuns, readinessRuns, coverageCommitted)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("executeStage() steps = %#v, want five steps", result.Steps)
	}
	if result.Steps[0].Status != "skipped" || result.Steps[3].Status != "skipped" {
		t.Fatalf("executeStage() did not preserve verified ensure/build completions: %#v", result.Steps)
	}
	if result.Steps[2].Name != stepApplyRoutingCoverage || result.Steps[2].Status != "ok" {
		t.Fatalf("executeStage() routing step = %#v, want newly applied coverage", result.Steps[2])
	}

	saved, err := loadCheckpoint(checkpointPath)
	if err != nil {
		t.Fatalf("load checkpoint: %v", err)
	}
	if saved == nil || saved.RoutingCoverageFile != coveragePath {
		t.Fatalf("saved checkpoint = %#v, want routing file %q", saved, coveragePath)
	}
	for _, step := range []string{stepEnsureVersion, stepApplyMetadata, stepApplyRoutingCoverage, stepAttachBuild, stepValidateReadiness} {
		if !saved.Completed[step] {
			t.Fatalf("saved checkpoint missing completed step %q: %#v", step, saved.Completed)
		}
	}
}

func TestCheckpointRejectsAddingRoutingCoverageAfterUnsafeCompletion(t *testing.T) {
	for _, completedStep := range []string{stepApplyRoutingCoverage, stepSubmitReview, "unrecognized_step"} {
		t.Run(completedStep, func(t *testing.T) {
			existing := &runCheckpoint{
				AppID:            "APP_123",
				Version:          "2.4.0",
				BuildID:          "BUILD_123",
				CopyMetadataFrom: "2.3.2",
				Platform:         "IOS",
				Mode:             releaseModeStage,
				Completed:        map[string]bool{completedStep: true},
			}
			opts := runOptions{
				AppID:               "APP_123",
				Version:             "2.4.0",
				BuildID:             "BUILD_123",
				CopyMetadataFrom:    "2.3.2",
				RoutingCoverageFile: "/tmp/coverage.geojson",
				Platform:            "IOS",
				Mode:                releaseModeStage,
			}
			if checkpointMatchesRunArguments(existing, opts) {
				t.Fatalf("checkpointMatchesRunArguments() = true with unsafe completed step %q", completedStep)
			}
		})
	}
}

func TestExecuteRun_ResumesCompletedCheckpoint(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
	})

	client := newCheckpointBindingClient(t, func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/SUBMISSION_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"SUBMISSION_123","attributes":{"state":"WAITING_FOR_REVIEW","platform":"IOS"},"relationships":{"appStoreVersionForReview":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	releaseClientFactory = func() (*asc.Client, error) { return client, nil }
	metadataRuns := 0
	readinessRuns := 0
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataRuns++
		return metadata.PushPlanResult{VersionID: "VERSION_123"}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		readinessRuns++
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	dir := t.TempDir()
	checkpointPath := filepath.Join(dir, "release-checkpoint.json")
	checkpoint := runCheckpoint{
		AppID:        "APP_123",
		Version:      "2.4.0",
		BuildID:      "BUILD_123",
		MetadataDir:  "./metadata/version/2.4.0",
		Platform:     "IOS",
		VersionID:    "VERSION_123",
		SubmissionID: "SUBMISSION_123",
		Completed: map[string]bool{
			stepEnsureVersion:     true,
			stepApplyMetadata:     true,
			stepAttachBuild:       true,
			stepValidateReadiness: true,
			stepSubmitReview:      true,
		},
	}
	if err := saveCheckpoint(checkpointPath, checkpoint); err != nil {
		t.Fatalf("save checkpoint: %v", err)
	}

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: checkpointPath,
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected resumed result")
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID from checkpoint, got %q", result.VersionID)
	}
	if result.SubmissionID != "SUBMISSION_123" {
		t.Fatalf("expected submissionID from checkpoint, got %q", result.SubmissionID)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 skipped steps, got %d", len(result.Steps))
	}
	if metadataRuns != 1 || readinessRuns != 1 {
		t.Fatalf("expected unprovable local steps to rerun once, got metadata=%d readiness=%d", metadataRuns, readinessRuns)
	}
	for _, index := range []int{0, 2, 4} {
		if result.Steps[index].Status != "skipped" {
			t.Fatalf("expected remotely verified step %d skipped, got %q", index, result.Steps[index].Status)
		}
	}
	for _, index := range []int{1, 3} {
		if result.Steps[index].Status == "skipped" {
			t.Fatalf("expected unprovable local step %d to rerun", index)
		}
	}
}

func TestExecuteRun_SuccessPath(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataCalled := false
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		metadataCalled = true
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessCalled := false
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 1, Blocking: 0},
			Checks: []validation.CheckResult{{
				ID:           "privacy.publish_state.unverified",
				Severity:     validation.SeverityInfo,
				ResourceType: "appPrivacy",
				ResourceID:   "APP_123",
				Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Remediation:  "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/APP_123/appPrivacy",
			}},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/reviewSubmissions":
			return releaseJSONResponse(http.StatusOK, `{"data":[]}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissions":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissions","id":"REV_SUB_123","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"ITEM_123"}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/REV_SUB_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"REV_SUB_123","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/REV_SUB_123/items":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"ITEM_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/REV_SUB_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"REV_SUB_123","attributes":{"state":"SUBMITTED","platform":"IOS","submittedDate":"2026-03-02T00:00:00Z"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})
	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID VERSION_123, got %q", result.VersionID)
	}
	if result.SubmissionID != "REV_SUB_123" {
		t.Fatalf("expected submissionID REV_SUB_123, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if !metadataCalled {
		t.Fatal("expected metadata step to be executed")
	}
	if !readinessCalled {
		t.Fatal("expected readiness checks to be executed")
	}
	if result.Steps[3].Message != "readiness checks passed with 1 advisory; App Privacy may still block submission" {
		t.Fatalf("expected readiness advisory message, got %q", result.Steps[3].Message)
	}
}

func TestExecuteStage_CopyMetadataSuccessPath(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origMetadataCopyExecutor := metadataCopyExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		metadataCopyExecutor = origMetadataCopyExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	copyCalled := false
	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata dir executor should not be called for copy-metadata stage flow")
		return metadata.PushPlanResult{}, nil
	}
	metadataCopyExecutor = func(_ context.Context, _ *asc.Client, opts metadataCopyOptions) (*asc.AppStoreVersionMetadataCopySummary, error) {
		copyCalled = true
		if opts.AppID != "APP_123" {
			t.Fatalf("expected app id APP_123, got %q", opts.AppID)
		}
		if opts.Platform != "IOS" {
			t.Fatalf("expected platform IOS, got %q", opts.Platform)
		}
		if opts.SourceVersion != "2.3.2" {
			t.Fatalf("expected source version 2.3.2, got %q", opts.SourceVersion)
		}
		if opts.DestinationVersionID != "VERSION_123" {
			t.Fatalf("expected destination version VERSION_123, got %q", opts.DestinationVersionID)
		}
		if opts.DryRun {
			t.Fatal("expected live copy metadata execution")
		}
		if got, want := strings.Join(opts.SelectedFields, ","), "description,keywords"; got != want {
			t.Fatalf("expected selected fields %q, got %q", want, got)
		}
		return &asc.AppStoreVersionMetadataCopySummary{
			SourceVersion:      "2.3.2",
			SourceVersionID:    "SOURCE_VERSION_123",
			SelectedFields:     []string{"description", "keywords"},
			CopiedLocales:      2,
			CopiedFieldUpdates: 4,
		}, nil
	}

	readinessCalled := false
	readinessReportBuilder = func(_ context.Context, opts validatecli.ReadinessOptions) (validation.Report, error) {
		readinessCalled = true
		if opts.VersionID != "VERSION_123" {
			t.Fatalf("expected readiness version VERSION_123, got %q", opts.VersionID)
		}
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 1, Blocking: 0},
			Checks: []validation.CheckResult{{
				ID:           "privacy.publish_state.unverified",
				Severity:     validation.SeverityInfo,
				ResourceType: "appPrivacy",
				ResourceID:   "APP_123",
				Message:      "App Privacy publish state is not verifiable via the public App Store Connect API and may still block submission",
				Remediation:  "Confirm App Privacy is published in App Store Connect before submitting: https://appstoreconnect.apple.com/apps/APP_123/appPrivacy",
			}},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/appStoreVersions/VERSION_123/relationships/build":
			return releaseJSONResponse(http.StatusNoContent, "")
		case strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissions"), strings.HasPrefix(req.URL.Path, "/v1/reviewSubmissionItems"):
			t.Fatalf("did not expect submission request for stage flow: %s %s", req.Method, req.URL.Path)
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeStage(context.Background(), runOptions{
		AppID:              "APP_123",
		Version:            "2.4.0",
		BuildID:            "BUILD_123",
		CopyMetadataFrom:   "2.3.2",
		SelectedCopyFields: []string{"description", "keywords"},
		Platform:           "IOS",
		Timeout:            releaseRunTimeout,
		DryRun:             false,
		Confirm:            true,
		StrictValidate:     false,
		CheckpointFile:     filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.VersionID != "VERSION_123" {
		t.Fatalf("expected versionID VERSION_123, got %q", result.VersionID)
	}
	if result.SubmissionID != "" {
		t.Fatalf("expected empty submissionID, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(result.Steps))
	}
	if result.Steps[0].Name != stepEnsureVersion {
		t.Fatalf("expected first step %q, got %q", stepEnsureVersion, result.Steps[0].Name)
	}
	if result.Steps[1].Name != stepApplyMetadata {
		t.Fatalf("expected second step %q, got %q", stepApplyMetadata, result.Steps[1].Name)
	}
	if result.Steps[2].Name != stepAttachBuild {
		t.Fatalf("expected third step %q, got %q", stepAttachBuild, result.Steps[2].Name)
	}
	if result.Steps[3].Name != stepValidateReadiness {
		t.Fatalf("expected fourth step %q, got %q", stepValidateReadiness, result.Steps[3].Name)
	}
	if !copyCalled {
		t.Fatal("expected metadata copy executor to be called")
	}
	if !readinessCalled {
		t.Fatal("expected readiness checks to be called")
	}
	if result.Steps[3].Message != "readiness checks passed with 1 advisory; App Privacy may still block submission" {
		t.Fatalf("expected readiness advisory message, got %q", result.Steps[3].Message)
	}
}

func TestExecuteStageAppliesRoutingCoverageBeforeReadiness(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataCopyExecutor := metadataCopyExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataCopyExecutor = origMetadataCopyExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	coveragePath := filepath.Join(t.TempDir(), "coverage.geojson")
	if err := os.WriteFile(coveragePath, []byte(validReleaseRoutingCoverageGeoJSON), 0o600); err != nil {
		t.Fatalf("write routing coverage fixture: %v", err)
	}
	t.Chdir(filepath.Dir(coveragePath))

	metadataCopyExecutor = func(context.Context, *asc.Client, metadataCopyOptions) (*asc.AppStoreVersionMetadataCopySummary, error) {
		return &asc.AppStoreVersionMetadataCopySummary{CopiedLocales: 1}, nil
	}
	coverageCommitted := false
	oldCoverageDeleted := false
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		if !coverageCommitted {
			t.Fatal("readiness ran before routing coverage completed")
		}
		return validation.Report{Summary: validation.Summary{}}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/routingAppCoverage":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_OLD","attributes":{"sourceFileChecksum":"old-checksum","assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodDelete && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_OLD":
			oldCoverageDeleted = true
			return releaseJSONResponse(http.StatusNoContent, "")
		case req.Method == http.MethodPost && req.URL.Path == "/v1/routingAppCoverages":
			if !oldCoverageDeleted {
				t.Fatal("new routing coverage was created before the old asset was deleted")
			}
			return releaseJSONResponse(http.StatusCreated, fmt.Sprintf(`{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"uploadOperations":[{"method":"PUT","url":"https://upload.example/coverage","length":%d,"offset":0}]}}}`, len(validReleaseRoutingCoverageGeoJSON)))
		case req.Method == http.MethodPut && req.URL.Host == "upload.example":
			return releaseJSONResponse(http.StatusOK, `{}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			coverageCommitted = true
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"UPLOAD_COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/routingAppCoverages/COVERAGE_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"routingAppCoverages","id":"COVERAGE_123","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"processingState":"VALID"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		}
	})

	releaseClientFactory = func() (*asc.Client, error) { return newReleaseTestClient(t), nil }
	result, err := executeStage(context.Background(), runOptions{
		AppID:               "APP_123",
		Version:             "2.4.0",
		BuildID:             "BUILD_123",
		CopyMetadataFrom:    "2.3.2",
		Platform:            "IOS",
		RoutingCoverageFile: coveragePath,
		Timeout:             releaseRunTimeout,
		Confirm:             true,
		CheckpointFile:      filepath.Join(t.TempDir(), "stage-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeStage() error: %v", err)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected five stage steps, got %#v", result.Steps)
	}
	wantSteps := []string{stepEnsureVersion, stepApplyMetadata, stepApplyRoutingCoverage, stepAttachBuild, stepValidateReadiness}
	for i, want := range wantSteps {
		if result.Steps[i].Name != want {
			t.Fatalf("step %d = %q, want %q", i, result.Steps[i].Name, want)
		}
	}
	if result.RoutingCoverageFile != coveragePath {
		t.Fatalf("routingCoverageFile = %q, want %q", result.RoutingCoverageFile, coveragePath)
	}
}

func TestExecuteRun_IdempotentWhenSubmissionExists(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	requests := make([]string, 0, 4)
	legacySubmissionLookups := 0
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.Path)
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"WAITING_FOR_REVIEW"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			legacySubmissionLookups++
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"appStoreVersionSubmissions","id":"SUBMISSION_123"}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }
	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 0, Blocking: 0},
		}, nil
	}

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if result.SubmissionID != "SUBMISSION_123" {
		t.Fatalf("expected existing submission id, got %q", result.SubmissionID)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if result.Steps[2].Status != "skipped" {
		t.Fatalf("expected attach step skipped, got %q", result.Steps[2].Status)
	}
	if result.Steps[4].Status != "skipped" {
		t.Fatalf("expected submit step skipped, got %q", result.Steps[4].Status)
	}

	for _, req := range requests {
		if strings.HasPrefix(req, "POST /v1/reviewSubmissions") || strings.HasPrefix(req, "POST /v1/reviewSubmissionItems") {
			t.Fatalf("expected idempotent path without new submission creation, saw %q", req)
		}
	}
	if legacySubmissionLookups != 1 {
		t.Fatalf("expected exactly 1 legacy submission lookup, got %d", legacySubmissionLookups)
	}
}

func TestExecuteRun_EmitsSubmitProgressMessagesToStderr(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 0, Blocking: 0},
		}, nil
	}

	itemReads := 0
	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"builds","id":"BUILD_123","attributes":{"version":"42","processingState":"VALID"}}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/reviewSubmissions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissions","id":"STALE_123","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/STALE_123/items":
			itemReads++
			if itemReads == 1 {
				return releaseJSONResponse(http.StatusOK, `{"data":[],"links":{}}`)
			}
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"reviewSubmissionItems","id":"ITEM_123","relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_123"}}}}],"links":{}}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/reviewSubmissions/STALE_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"STALE_123","attributes":{"state":"READY_FOR_REVIEW","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_123"}}}}}`)
		case req.Method == http.MethodPost && req.URL.Path == "/v1/reviewSubmissionItems":
			return releaseJSONResponse(http.StatusCreated, `{"data":{"type":"reviewSubmissionItems","id":"ITEM_123"}}`)
		case req.Method == http.MethodPatch && req.URL.Path == "/v1/reviewSubmissions/STALE_123":
			return releaseJSONResponse(http.StatusOK, `{"data":{"type":"reviewSubmissions","id":"STALE_123","attributes":{"state":"SUBMITTED","platform":"IOS","submittedDate":"2026-03-16T00:00:00Z"}}}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	stderr := captureReleaseStderr(t, func() {
		result, err := executeRun(context.Background(), runOptions{
			AppID:          "APP_123",
			Version:        "2.4.0",
			BuildID:        "BUILD_123",
			MetadataDir:    "./metadata/version/2.4.0",
			Platform:       "IOS",
			Timeout:        releaseRunTimeout,
			DryRun:         false,
			Confirm:        true,
			StrictValidate: false,
			CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
		})
		if err != nil {
			t.Fatalf("executeRun error: %v", err)
		}
		if result.SubmissionID != "STALE_123" {
			t.Fatalf("expected submissionID STALE_123, got %q", result.SubmissionID)
		}
	})

	if !strings.Contains(stderr, "Reusing existing review submission STALE_123") {
		t.Fatalf("expected safe reuse progress on stderr, got %q", stderr)
	}
}

func TestExecuteRun_DryRunReadinessStepMarkedDryRun(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(_ context.Context, opts metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		return metadata.PushPlanResult{
			AppID:     opts.AppID,
			Version:   opts.Version,
			VersionID: "VERSION_123",
			Dir:       opts.Dir,
			DryRun:    opts.DryRun,
			Includes:  []string{"localizations"},
		}, nil
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		return validation.Report{
			AppID:     "APP_123",
			VersionID: "VERSION_123",
			Summary:   validation.Summary{Errors: 0, Warnings: 0, Infos: 0, Blocking: 0},
		}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions":
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/build":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		case req.Method == http.MethodGet && req.URL.Path == "/v1/appStoreVersions/VERSION_123/appStoreVersionSubmission":
			return releaseJSONResponse(http.StatusNotFound, `{"errors":[{"status":"404","code":"NOT_FOUND","title":"Not Found"}]}`)
		default:
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		Confirm:        false,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if result.Steps[3].Name != stepValidateReadiness {
		t.Fatalf("expected step 4 to be %q, got %q", stepValidateReadiness, result.Steps[3].Name)
	}
	if result.Steps[3].Status != "dry-run" {
		t.Fatalf("expected readiness step dry-run status, got %q", result.Steps[3].Status)
	}
}

func TestExecuteRun_DryRunDefersAttachAndSubmitWhenVersionWouldBeCreated(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(context.Context, metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		t.Fatal("metadata executor should not run when dry-run defers version creation")
		return metadata.PushPlanResult{}, nil
	}
	readinessReportBuilder = func(context.Context, validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness builder should not run when dry-run defers version creation")
		return validation.Report{}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions" {
			return releaseJSONResponse(http.StatusOK, `{"data":[]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	result, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "9.9.9",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/9.9.9",
		Platform:       "IOS",
		Timeout:        releaseRunTimeout,
		DryRun:         true,
		Confirm:        false,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if err != nil {
		t.Fatalf("executeRun error: %v", err)
	}
	if result.Status != "dry-run" {
		t.Fatalf("expected status dry-run, got %q", result.Status)
	}
	if len(result.Steps) != 5 {
		t.Fatalf("expected 5 steps, got %d", len(result.Steps))
	}
	if result.Steps[2].Name != stepAttachBuild || result.Steps[2].Message != "build attach deferred until version exists" {
		t.Fatalf("expected attach step deferred, got %+v", result.Steps[2])
	}
	if result.Steps[4].Name != stepSubmitReview || result.Steps[4].Message != "submission deferred until version exists" {
		t.Fatalf("expected submit step deferred, got %+v", result.Steps[4])
	}
}

func TestExecuteRun_TimeoutCancelsPipeline(t *testing.T) {
	origClientFactory := releaseClientFactory
	origMetadataExecutor := metadataPushExecutor
	origReadinessBuilder := readinessReportBuilder
	origTransport := http.DefaultTransport
	t.Cleanup(func() {
		releaseClientFactory = origClientFactory
		metadataPushExecutor = origMetadataExecutor
		readinessReportBuilder = origReadinessBuilder
		http.DefaultTransport = origTransport
	})

	metadataPushExecutor = func(ctx context.Context, _ metadata.PushExecutionOptions) (metadata.PushPlanResult, error) {
		<-ctx.Done()
		return metadata.PushPlanResult{}, ctx.Err()
	}
	readinessReportBuilder = func(_ context.Context, _ validatecli.ReadinessOptions) (validation.Report, error) {
		t.Fatal("readiness should not run when metadata step times out")
		return validation.Report{}, nil
	}

	http.DefaultTransport = releaseRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet && req.URL.Path == "/v1/apps/APP_123/appStoreVersions" {
			return releaseJSONResponse(http.StatusOK, `{"data":[{"type":"appStoreVersions","id":"VERSION_123","attributes":{"versionString":"2.4.0","platform":"IOS","appStoreState":"PREPARE_FOR_SUBMISSION"}}]}`)
		}
		return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
	})

	testClient := newReleaseTestClient(t)
	releaseClientFactory = func() (*asc.Client, error) { return testClient, nil }

	_, err := executeRun(context.Background(), runOptions{
		AppID:          "APP_123",
		Version:        "2.4.0",
		BuildID:        "BUILD_123",
		MetadataDir:    "./metadata/version/2.4.0",
		Platform:       "IOS",
		Timeout:        20 * time.Millisecond,
		DryRun:         false,
		Confirm:        true,
		StrictValidate: false,
		CheckpointFile: filepath.Join(t.TempDir(), "release-checkpoint.json"),
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
