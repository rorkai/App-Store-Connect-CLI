package xcodecloud

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func TestAnalyzeDoctorLogBundleFindsITMSDiagnosticsAndExportStatus(t *testing.T) {
	data := doctorLogBundleFixture(t, map[string]string{
		"Export/IDEDistribution.standard.log": "** EXPORT SUCCEEDED **\nerror: ITMS-90478: Invalid Version",
		"Export/duplicate.log":                "error: ITMS-90478: Invalid Version",
		"Export/binary.log":                   "ignored\x00binary",
	})

	analysis, err := analyzeDoctorLogBundle(data)
	if err != nil {
		t.Fatalf("analyzeDoctorLogBundle() error = %v", err)
	}
	if analysis.ExportStatus != "SUCCEEDED" {
		t.Fatalf("ExportStatus = %q, want SUCCEEDED", analysis.ExportStatus)
	}
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %+v, want one deduplicated diagnostic", analysis.Diagnostics)
	}
	if analysis.Diagnostics[0].Code != "ITMS-90478" {
		t.Fatalf("diagnostic code = %q, want ITMS-90478", analysis.Diagnostics[0].Code)
	}
}

func TestAnalyzeDoctorLogBundleReportsDiagnosticTruncation(t *testing.T) {
	lines := make([]string, 0, maxDoctorDiagnostics+1)
	for index := 0; index <= maxDoctorDiagnostics; index++ {
		lines = append(lines, fmt.Sprintf("error: ITMS-%05d: diagnostic %d", index, index))
	}

	analysis, err := analyzeDoctorLogBundle([]byte(strings.Join(lines, "\n")))
	if err != nil {
		t.Fatalf("analyzeDoctorLogBundle() error = %v", err)
	}
	if len(analysis.Diagnostics) != maxDoctorDiagnostics || !analysis.DiagnosticsTruncated {
		t.Fatalf("analysis = %+v, want %d diagnostics and explicit truncation", analysis, maxDoctorDiagnostics)
	}
}

func TestAnalyzeDoctorLogTextTruncatesDiagnosticsOnUTF8Boundary(t *testing.T) {
	analysis := doctorLogBundleAnalysis{Diagnostics: make([]asc.XcodeCloudDoctorLogDiagnostic, 0)}
	contents := "ITMS-90000: x" + strings.Repeat("é", maxDoctorDiagnosticLength)

	analyzeDoctorLogText(&analysis, "export.log", contents)

	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %+v, want one diagnostic", analysis.Diagnostics)
	}
	if !utf8.ValidString(analysis.Diagnostics[0].Message) {
		t.Fatalf("diagnostic message is not valid UTF-8: %q", analysis.Diagnostics[0].Message)
	}
	if !strings.HasSuffix(analysis.Diagnostics[0].Message, "…") {
		t.Fatalf("diagnostic message = %q, want ellipsis suffix", analysis.Diagnostics[0].Message)
	}
}

func TestFinishXcodeCloudDoctorResultDoesNotInventImportFailure(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{
			BuildRunID:        "run-92",
			ExecutionProgress: "COMPLETE",
			CompletionStatus:  "FAILED",
		},
		Actions: []asc.XcodeCloudDoctorAction{{
			ID:               "archive-ios",
			CompletionStatus: "FAILED",
			Artifacts:        []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
		}},
		LogBundles: []asc.XcodeCloudDoctorLogBundle{{
			ArtifactID:   "log-92",
			ActionID:     "archive-ios",
			Inspected:    true,
			ExportStatus: "SUCCEEDED",
			Diagnostics:  []asc.XcodeCloudDoctorLogDiagnostic{},
		}},
		CoverageWarnings: []asc.XcodeCloudDoctorCoverageWarning{},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.Conclusion, "without an ITMS-level import diagnostic") {
		t.Fatalf("unexpected conclusion %q", result.Conclusion)
	}
	if len(result.CoverageWarnings) != 1 || result.CoverageWarnings[0].ID != "app_store_import_detail_unavailable" {
		t.Fatalf("unexpected coverage warnings: %+v", result.CoverageWarnings)
	}
	if strings.Contains(result.Conclusion, "Invalid Version") || strings.Contains(result.Conclusion, "pre-release train") {
		t.Fatalf("conclusion invented an import root cause: %q", result.Conclusion)
	}
}

func TestFinishXcodeCloudDoctorResultReportsCanceledAndSkippedRuns(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
	}{
		{name: "canceled", status: "CANCELED", conclusion: "was canceled"},
		{name: "skipped", status: "SKIPPED", conclusion: "was skipped"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := &asc.XcodeCloudDoctorResult{
				Run: &asc.XcodeCloudStatusResult{
					BuildRunID:        "run-1",
					ExecutionProgress: "COMPLETE",
					CompletionStatus:  test.status,
				},
			}

			finishXcodeCloudDoctorResult(result)

			if !strings.Contains(strings.ToLower(result.Conclusion), test.conclusion) {
				t.Fatalf("Conclusion = %q, want %q", result.Conclusion, test.conclusion)
			}
			if strings.Contains(strings.ToLower(result.Conclusion), "failed") {
				t.Fatalf("Conclusion = %q, must not describe %s run as failed", result.Conclusion, test.status)
			}
		})
	}
}

func TestShouldInspectDoctorLogsOnlyForFailuresUnlessSaving(t *testing.T) {
	tests := []struct {
		status            string
		executionProgress string
		options           xcodeCloudDoctorOptions
		want              bool
	}{
		{status: "FAILED", want: true},
		{status: "ERRORED", want: true},
		{status: "FAILED", executionProgress: "RUNNING", want: false},
		{status: "SUCCEEDED", want: false},
		{status: "CANCELED", want: false},
		{status: "SKIPPED", want: false},
		{status: "CANCELED", options: xcodeCloudDoctorOptions{SaveLogs: "logs"}, want: true},
		{status: "SKIPPED", options: xcodeCloudDoctorOptions{SaveLogs: "logs"}, want: true},
	}

	for _, test := range tests {
		t.Run(test.status+test.executionProgress+test.options.SaveLogs, func(t *testing.T) {
			executionProgress := test.executionProgress
			if executionProgress == "" {
				executionProgress = "COMPLETE"
			}
			result := &asc.XcodeCloudDoctorResult{
				Run: &asc.XcodeCloudStatusResult{
					ExecutionProgress: executionProgress,
					CompletionStatus:  test.status,
				},
				Summary: asc.XcodeCloudDoctorSummary{LogBundles: 1},
				Actions: []asc.XcodeCloudDoctorAction{{
					CompletionStatus: "FAILED",
					Artifacts:        []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
				}},
			}

			if got := shouldInspectDoctorLogs(result, test.options); got != test.want {
				t.Fatalf("shouldInspectDoctorLogs(%s, %+v) = %t, want %t", test.status, test.options, got, test.want)
			}
		})
	}
}

func TestDoctorFailureAggregationExcludesCanceledActions(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{Actions: []asc.XcodeCloudDoctorAction{
		{ID: "failed", CompletionStatus: "FAILED", Artifacts: []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}}},
		{ID: "errored", CompletionStatus: "ERRORED", Artifacts: []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}}},
		{ID: "canceled", CompletionStatus: "CANCELED", Artifacts: []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}}},
	}}

	summarizeXcodeCloudDoctorResult(result)

	if result.Summary.FailedActions != 2 || result.Summary.CanceledActions != 1 {
		t.Fatalf("summary = %+v, want 2 failed and 1 canceled action", result.Summary)
	}
	if got := doctorFailedActionLogBundleCount(result); got != 2 {
		t.Fatalf("doctorFailedActionLogBundleCount() = %d, want 2", got)
	}
}

func TestFinishXcodeCloudDoctorResultIgnoresSuccessfulActionLogBundles(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{ExecutionProgress: "COMPLETE", CompletionStatus: "FAILED"},
		Actions: []asc.XcodeCloudDoctorAction{
			{
				ID:               "failed-test",
				CompletionStatus: "FAILED",
				Issues: []asc.XcodeCloudDoctorIssue{{
					IssueType: "ERROR",
					Category:  "Testing",
					Message:   "A test failed",
				}},
			},
			{
				ID:               "successful-archive",
				CompletionStatus: "SUCCEEDED",
				Artifacts:        []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
			},
		},
		Summary: asc.XcodeCloudDoctorSummary{LogBundles: 1, LogBundlesInspected: 1, Errors: 1},
		LogBundles: []asc.XcodeCloudDoctorLogBundle{{
			ActionID:     "successful-archive",
			Inspected:    true,
			ExportStatus: "SUCCEEDED",
			Diagnostics: []asc.XcodeCloudDoctorLogDiagnostic{{
				Code:    "ITMS-90000",
				Message: "Incidental diagnostic",
			}},
		}},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.Conclusion, "reported actionable issues") {
		t.Fatalf("successful-action bundle changed diagnosis: conclusion=%q nextAction=%q", result.Conclusion, result.NextAction)
	}
}

func TestFinishXcodeCloudDoctorResultUsesFailedActionInspectionCount(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{ExecutionProgress: "COMPLETE", CompletionStatus: "FAILED"},
		Actions: []asc.XcodeCloudDoctorAction{
			{
				ID:               "failed-archive",
				CompletionStatus: "FAILED",
				Issues: []asc.XcodeCloudDoctorIssue{{
					IssueType: "ERROR",
					Message:   "Preparing build for App Store Connect failed",
				}},
				Artifacts: []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
			},
			{ID: "successful-archive", CompletionStatus: "SUCCEEDED"},
		},
		Summary: asc.XcodeCloudDoctorSummary{LogBundles: 2, LogBundlesInspected: 1, Errors: 1},
		LogBundles: []asc.XcodeCloudDoctorLogBundle{{
			ActionID:  "successful-archive",
			Inspected: true,
		}},
		CoverageWarnings: []asc.XcodeCloudDoctorCoverageWarning{},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.Conclusion, "failed-action log bundles were not inspected") {
		t.Fatalf("Conclusion = %q, want failed-action inspection gap", result.Conclusion)
	}
	if len(result.CoverageWarnings) != 1 || strings.Contains(result.CoverageWarnings[0].Message, "inspected log bundles") {
		t.Fatalf("coverage warnings = %+v, must not count successful-action inspection", result.CoverageWarnings)
	}
}

func TestFinishXcodeCloudDoctorResultPointsToSavedFailedActionLogs(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{ExecutionProgress: "COMPLETE", CompletionStatus: "FAILED"},
		Actions: []asc.XcodeCloudDoctorAction{{
			ID:               "failed-archive",
			CompletionStatus: "FAILED",
			Artifacts:        []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
		}},
		LogBundles: []asc.XcodeCloudDoctorLogBundle{{
			ActionID:  "failed-archive",
			SavedPath: "/tmp/logs/failed.zip",
		}},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.NextAction, "saved failed-action log bundles") {
		t.Fatalf("NextAction = %q, want saved bundle guidance", result.NextAction)
	}
}

func TestFinishXcodeCloudDoctorResultDoesNotRepeatFailedInspection(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{ExecutionProgress: "COMPLETE", CompletionStatus: "FAILED"},
		Actions: []asc.XcodeCloudDoctorAction{{
			ID:               "failed-archive",
			CompletionStatus: "FAILED",
			Issues: []asc.XcodeCloudDoctorIssue{{
				IssueType: "ERROR",
				Message:   "Preparing build for App Store Connect failed",
			}},
			Artifacts: []asc.XcodeCloudDoctorArtifact{{FileType: "LOG_BUNDLE"}},
		}},
		CoverageWarnings: []asc.XcodeCloudDoctorCoverageWarning{{ID: "log_bundle_inspection_failed"}},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.NextAction, "inspection remediation") || strings.Contains(result.NextAction, "Re-run") {
		t.Fatalf("NextAction = %q, want existing inspection failure remediation", result.NextAction)
	}
}

func TestFinishXcodeCloudDoctorResultReportsPartialFailedBundleCoverage(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{
		Run: &asc.XcodeCloudStatusResult{ExecutionProgress: "COMPLETE", CompletionStatus: "FAILED"},
		Actions: []asc.XcodeCloudDoctorAction{{
			ID:               "failed-archive",
			CompletionStatus: "FAILED",
			Issues: []asc.XcodeCloudDoctorIssue{{
				IssueType: "ERROR",
				Message:   "Preparing build for App Store Connect failed",
			}},
			Artifacts: []asc.XcodeCloudDoctorArtifact{
				{ID: "log-1", FileType: "LOG_BUNDLE"},
				{ID: "log-2", FileType: "LOG_BUNDLE"},
			},
		}},
		LogBundles: []asc.XcodeCloudDoctorLogBundle{
			{ArtifactID: "log-1", ActionID: "failed-archive", Inspected: true, ExportStatus: "SUCCEEDED"},
			{ArtifactID: "log-2", ActionID: "failed-archive"},
		},
		CoverageWarnings: []asc.XcodeCloudDoctorCoverageWarning{{ID: "log_bundle_inspection_failed"}},
	}

	finishXcodeCloudDoctorResult(result)

	if !strings.Contains(result.Conclusion, "one or more failed-action log bundles were not inspected") {
		t.Fatalf("Conclusion = %q, want partial inspection caveat", result.Conclusion)
	}
	if !strings.Contains(result.NextAction, "inspection remediation") || strings.Contains(result.NextAction, "server-side import rejection") {
		t.Fatalf("NextAction = %q, want inspection-first guidance", result.NextAction)
	}
	if len(result.CoverageWarnings) != 2 || result.CoverageWarnings[1].ID != "app_store_import_detail_unavailable" || !strings.Contains(result.CoverageWarnings[1].Message, "not inspected") {
		t.Fatalf("coverage warnings = %+v, want explicit partial App Store coverage", result.CoverageWarnings)
	}
}

func TestDoctorHasAppStorePreparationIssueUsesFailedErrorActions(t *testing.T) {
	result := &asc.XcodeCloudDoctorResult{Actions: []asc.XcodeCloudDoctorAction{
		{
			CompletionStatus: "SUCCEEDED",
			Issues: []asc.XcodeCloudDoctorIssue{{
				IssueType: "WARNING",
				Message:   "App Store Connect warning",
			}},
		},
		{
			CompletionStatus: "FAILED",
			Issues: []asc.XcodeCloudDoctorIssue{{
				IssueType: "ERROR",
				Message:   "Compilation failed",
			}},
		},
	}}

	if doctorHasAppStorePreparationIssue(result) {
		t.Fatal("successful warning must not be treated as the failed run's App Store preparation issue")
	}

	result.Actions[1].Issues[0].Category = "PrepareBuildForAppStoreConnect"
	result.Actions[1].Issues[0].Message = "Preparing build for App Store Connect failed"
	if !doctorHasAppStorePreparationIssue(result) {
		t.Fatal("failed error action's App Store preparation issue was not detected")
	}
}

func TestAnalyzeDoctorLogBundleRejectsUninspectableBundles(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantErr string
	}{
		{
			name: "oversized text entry",
			data: doctorLogBundleFixture(t, map[string]string{
				"Export/IDEDistribution.standard.log": strings.Repeat("x", maxDoctorLogEntryBytes+1),
			}),
			wantErr: "IDEDistribution.standard.log",
		},
		{
			name: "no readable text entries",
			data: doctorLogBundleFixture(t, map[string]string{
				"Build/Build.xcactivitylog": "binary activity log",
				"Export/binary.log":         "ignored\x00binary",
			}),
			wantErr: "no readable text entries",
		},
		{name: "empty plain content", data: nil, wantErr: "log bundle is empty"},
		{
			name:    "empty zip entry",
			data:    doctorLogBundleFixture(t, map[string]string{"Build/empty.log": ""}),
			wantErr: "no readable text entries",
		},
		{name: "unknown binary", data: []byte{'P', 'K', 0, 1, 2}, wantErr: "neither a ZIP archive nor plain text"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := analyzeDoctorLogBundle(test.data)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("analyzeDoctorLogBundle() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestAnalyzeDoctorLogBundleCountsBinaryCandidatesTowardAggregateLimit(t *testing.T) {
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for index := 0; index <= maxDoctorLogUncompressedBytes/maxDoctorLogEntryBytes; index++ {
		entry, err := archive.Create(fmt.Sprintf("Build/%d.log", index))
		if err != nil {
			t.Fatalf("create binary log entry: %v", err)
		}
		size := int64(maxDoctorLogEntryBytes)
		if index == maxDoctorLogUncompressedBytes/maxDoctorLogEntryBytes {
			size = 1
		}
		if _, err := io.CopyN(entry, doctorZeroReader{}, size); err != nil {
			t.Fatalf("write binary log entry: %v", err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close binary log archive: %v", err)
	}

	_, err := analyzeDoctorLogBundle(buffer.Bytes())
	if err == nil || !strings.Contains(err.Error(), "uncompressed inspection limit") {
		t.Fatalf("analyzeDoctorLogBundle() error = %v, want aggregate limit error", err)
	}
}

func TestInspectXcodeCloudDoctorLogsContinuesAfterSaveFailure(t *testing.T) {
	directory := t.TempDir()
	artifacts := []asc.XcodeCloudDoctorArtifact{
		{ID: "artifact-1", FileName: "first.zip", FileType: "LOG_BUNDLE"},
		{ID: "artifact-2", FileName: "second.zip", FileType: "LOG_BUNDLE"},
	}
	blockedPath := filepath.Join(directory, doctorSavedLogBundleName(artifacts[1]))
	if err := os.WriteFile(blockedPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write blocked destination: %v", err)
	}
	bundle := doctorLogBundleFixture(t, map[string]string{"Export/export.log": "** EXPORT SUCCEEDED **"})
	client := newDoctorLogTestClient(t, doctorLogRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := bundle
		contentType := "application/octet-stream"
		if strings.HasPrefix(request.URL.Path, "/v1/ciArtifacts/") {
			artifactID := filepath.Base(request.URL.Path)
			body = []byte(fmt.Sprintf(`{"data":{"type":"ciArtifacts","id":%q,"attributes":{"downloadUrl":"https://appstoreconnect.apple.com/downloads/%s.zip"}}}`, artifactID, artifactID))
			contentType = "application/json"
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{contentType}},
			Request:    request,
		}, nil
	}))
	result := &asc.XcodeCloudDoctorResult{Actions: []asc.XcodeCloudDoctorAction{{
		ID:               "failed-action",
		CompletionStatus: "FAILED",
		Artifacts:        artifacts,
	}}}

	err := inspectXcodeCloudDoctorLogs(context.Background(), client, result, xcodeCloudDoctorOptions{SaveLogs: directory})
	if err != nil {
		t.Fatalf("inspectXcodeCloudDoctorLogs() error = %v, want report with coverage warning", err)
	}
	if len(result.LogBundles) != 2 || !result.LogBundles[0].Inspected || result.LogBundles[1].SavedPath != "" {
		t.Fatalf("log bundles = %+v, want first inspected and second reported as unsaved", result.LogBundles)
	}
	if len(result.CoverageWarnings) != 1 || result.CoverageWarnings[0].ID != "log_bundle_inspection_failed" {
		t.Fatalf("coverage warnings = %+v, want save failure warning", result.CoverageWarnings)
	}
}

func TestInspectXcodeCloudDoctorLogsPropagatesCancellation(t *testing.T) {
	client := newDoctorLogTestClient(t, doctorLogRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	}))
	result := &asc.XcodeCloudDoctorResult{Actions: []asc.XcodeCloudDoctorAction{{
		ID:               "failed-action",
		CompletionStatus: "FAILED",
		Artifacts: []asc.XcodeCloudDoctorArtifact{{
			ID:       "artifact-1",
			FileType: "LOG_BUNDLE",
		}},
	}}}

	err := inspectXcodeCloudDoctorLogs(context.Background(), client, result, xcodeCloudDoctorOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("inspectXcodeCloudDoctorLogs() error = %v, want context cancellation", err)
	}
	if len(result.CoverageWarnings) != 0 {
		t.Fatalf("coverage warnings = %+v, must not downgrade cancellation", result.CoverageWarnings)
	}
}

func TestSaveAndAnalyzeDoctorLogBundleKeepsOversizedFile(t *testing.T) {
	directory := t.TempDir()
	root, err := rootfs.New(directory)
	if err != nil {
		t.Fatalf("rootfs.New() error = %v", err)
	}
	defer root.Close()

	const inspectionLimit = int64(32)
	name := "large-log-bundle.zip"
	result := asc.XcodeCloudDoctorLogBundle{}

	result, err = saveAndAnalyzeDoctorLogBundle(
		root,
		directory,
		name,
		result,
		io.LimitReader(doctorZeroReader{}, inspectionLimit+1),
		inspectionLimit,
	)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("saveAndAnalyzeDoctorLogBundle() error = %v, want inspection limit error", err)
	}
	if result.SavedPath == "" || result.Inspected {
		t.Fatalf("result = %+v, want saved but not inspected", result)
	}
	info, statErr := os.Stat(filepath.Join(directory, name))
	if statErr != nil {
		t.Fatalf("saved bundle stat error = %v", statErr)
	}
	if info.Size() != inspectionLimit+1 || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved bundle info = size %d mode %o", info.Size(), info.Mode().Perm())
	}
}

type doctorZeroReader struct{}

func (doctorZeroReader) Read(buffer []byte) (int, error) {
	clear(buffer)
	return len(buffer), nil
}

func TestDoctorSavedLogBundleNameSanitizesRemoteComponents(t *testing.T) {
	artifact := asc.XcodeCloudDoctorArtifact{
		ID:       "../../artifact/id",
		FileName: `..\..\Build 92 Logs.zip`,
	}
	name := doctorSavedLogBundleName(artifact)
	if strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		t.Fatalf("unsafe saved name %q", name)
	}
}

func doctorLogBundleFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, contents := range files {
		file, err := archive.Create(name)
		if err != nil {
			t.Fatalf("create %q: %v", name, err)
		}
		if _, err := io.WriteString(file, contents); err != nil {
			t.Fatalf("write %q: %v", name, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}

type doctorLogRoundTripFunc func(*http.Request) (*http.Response, error)

func (function doctorLogRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func newDoctorLogTestClient(t *testing.T, transport http.RoundTripper) *asc.Client {
	t.Helper()
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate private key: %v", err)
	}
	encodedKey, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	keyPath := filepath.Join(t.TempDir(), "AuthKey_TEST.p8")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedKey}), 0o600); err != nil {
		t.Fatalf("write private key: %v", err)
	}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("create ASC client: %v", err)
	}
	return client
}
