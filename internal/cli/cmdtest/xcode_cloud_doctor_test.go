package cmdtest

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestXcodeCloudDoctorReportsFailedRunAndInspectsLogBundle(t *testing.T) {
	setupAuth(t)

	logBundle := buildXcodeCloudLogBundle(t, "Export/IDEDistribution.standard.log", strings.Join([]string{
		"Exported Presset to: /Volumes/workspace/appstoreexport",
		"** EXPORT SUCCEEDED **",
		"error: ITMS-90478: Invalid Version - Choose a different version number.",
	}, "\n"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	requests := make(map[string]int)
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests[req.URL.Path]++
		var body string
		switch req.URL.Path {
		case "/v1/ciBuildRuns/run-1":
			body = `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"number":92,"executionProgress":"COMPLETE","completionStatus":"FAILED"},"relationships":{"workflow":{"data":{"type":"ciWorkflows","id":"workflow-1"}}}}}`
		case "/v1/ciBuildRuns/run-1/actions":
			body = `{"data":[{"type":"ciBuildActions","id":"action-1","attributes":{"name":"Archive - iOS","actionType":"ARCHIVE","executionProgress":"COMPLETE","completionStatus":"FAILED"}}]}`
		case "/v1/ciBuildActions/action-1/issues":
			body = `{"data":[{"type":"ciIssues","id":"issue-1","attributes":{"issueType":"ERROR","category":"Prepare Build for App Store Connect","message":"Preparing build for App Store Connect failed"}}]}`
		case "/v1/ciBuildActions/action-1/artifacts":
			body = `{"data":[{"type":"ciArtifacts","id":"log-1","attributes":{"fileType":"LOG_BUNDLE","fileName":"Build 92 Logs.zip","fileSize":512}}]}`
		case "/v1/ciArtifacts/log-1":
			body = `{"data":{"type":"ciArtifacts","id":"log-1","attributes":{"fileType":"LOG_BUNDLE","fileName":"Build 92 Logs.zip","fileSize":512,"downloadUrl":"https://appstoreconnect.apple.com/logs/log-1.zip"}}}`
		case "/logs/log-1.zip":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(logBundle)),
				Header:     http.Header{"Content-Type": []string{"application/zip"}},
			}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "doctor", "--run-id", "run-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var result struct {
		Run struct {
			BuildRunID       string `json:"buildRunId"`
			CompletionStatus string `json:"completionStatus"`
		} `json:"run"`
		Summary struct {
			FailedActions int `json:"failedActions"`
		} `json:"summary"`
		LogBundles []struct {
			ArtifactID   string `json:"artifactId"`
			Inspected    bool   `json:"inspected"`
			ExportStatus string `json:"exportStatus"`
			Diagnostics  []struct {
				Code string `json:"code"`
			} `json:"diagnostics"`
		} `json:"logBundles"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output %q: %v", stdout, err)
	}
	if result.Run.BuildRunID != "run-1" || result.Run.CompletionStatus != "FAILED" {
		t.Fatalf("unexpected run result: %+v", result.Run)
	}
	if result.Summary.FailedActions != 1 {
		t.Fatalf("failedActions = %d, want 1", result.Summary.FailedActions)
	}
	if len(result.LogBundles) != 1 || result.LogBundles[0].ArtifactID != "log-1" || !result.LogBundles[0].Inspected {
		t.Fatalf("unexpected log bundle result: %+v", result.LogBundles)
	}
	if result.LogBundles[0].ExportStatus != "SUCCEEDED" {
		t.Fatalf("exportStatus = %q, want SUCCEEDED", result.LogBundles[0].ExportStatus)
	}
	if len(result.LogBundles[0].Diagnostics) != 1 || result.LogBundles[0].Diagnostics[0].Code != "ITMS-90478" {
		t.Fatalf("unexpected diagnostics: %+v", result.LogBundles[0].Diagnostics)
	}
	if !strings.Contains(result.Conclusion, "App Store import diagnostics") {
		t.Fatalf("unexpected conclusion %q", result.Conclusion)
	}
	for _, path := range []string{
		"/v1/ciBuildRuns/run-1",
		"/v1/ciBuildRuns/run-1/actions",
		"/v1/ciBuildActions/action-1/issues",
		"/v1/ciBuildActions/action-1/artifacts",
		"/v1/ciArtifacts/log-1",
		"/logs/log-1.zip",
	} {
		if requests[path] != 1 {
			t.Fatalf("request count for %s = %d, want 1", path, requests[path])
		}
	}
}

func TestXcodeCloudDoctorRejectsInvalidFlagsBeforeRequests(t *testing.T) {
	setupAuth(t)

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing run id", args: []string{"xcode-cloud", "doctor"}, wantErr: "--run-id is required"},
		{name: "poll interval without wait", args: []string{"xcode-cloud", "doctor", "--run-id", "run-1", "--poll-interval", "1s"}, wantErr: "--poll-interval requires --wait"},
		{name: "save logs with skip logs", args: []string{"xcode-cloud", "doctor", "--run-id", "run-1", "--save-logs", t.TempDir(), "--skip-logs"}, wantErr: "--save-logs and --skip-logs are mutually exclusive"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUsageExit(t, tt.args, tt.wantErr)
		})
	}
}

func TestXcodeCloudDoctorHelpRegistersNewSurface(t *testing.T) {
	root := RootCommand("1.2.3")
	doctor := findCommand(root, "xcode-cloud", "doctor")
	if doctor == nil {
		t.Fatal("xcode-cloud doctor command is not registered")
	}

	for _, name := range []string{"run-id", "wait", "poll-interval", "timeout", "skip-logs", "save-logs"} {
		flagValue := doctor.FlagSet.Lookup(name)
		if flagValue == nil {
			t.Fatalf("flag --%s is not registered", name)
		}
	}
}

func TestXcodeCloudDoctorSavesLogBundleWithoutOverwriting(t *testing.T) {
	setupAuth(t)

	logBundle := buildXcodeCloudLogBundle(t, "Export/export.log", "** EXPORT FAILED **\nerror: ITMS-90062: Invalid bundle")
	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/v1/ciBuildRuns/run-1":
			body = `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"COMPLETE","completionStatus":"FAILED"}}}`
		case "/v1/ciBuildRuns/run-1/actions":
			body = `{"data":[{"type":"ciBuildActions","id":"action-1","attributes":{"actionType":"ARCHIVE","executionProgress":"COMPLETE","completionStatus":"FAILED"}}]}`
		case "/v1/ciBuildActions/action-1/issues":
			body = `{"data":[]}`
		case "/v1/ciBuildActions/action-1/artifacts":
			body = `{"data":[{"type":"ciArtifacts","id":"log-1","attributes":{"fileType":"LOG_BUNDLE","fileName":"Build 92 Logs.zip","fileSize":512}}]}`
		case "/v1/ciArtifacts/log-1":
			body = `{"data":{"type":"ciArtifacts","id":"log-1","attributes":{"downloadUrl":"https://appstoreconnect.apple.com/logs/log-1.zip"}}}`
		case "/logs/log-1.zip":
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(logBundle))}, nil
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	saveDir := t.TempDir()
	runDoctor := func() error {
		root := RootCommand("1.2.3")
		root.FlagSet.SetOutput(io.Discard)
		if err := root.Parse([]string{"xcode-cloud", "doctor", "--run-id", "run-1", "--save-logs", saveDir, "--output", "json"}); err != nil {
			return err
		}
		return root.Run(context.Background())
	}

	var firstRunErr error
	captureOutput(t, func() {
		firstRunErr = runDoctor()
	})
	if firstRunErr != nil {
		t.Fatalf("first doctor run error: %v", firstRunErr)
	}
	entries, err := os.ReadDir(saveDir)
	if err != nil {
		t.Fatalf("read save directory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("saved entries = %d, want 1", len(entries))
	}
	saved, err := os.ReadFile(saveDir + string(os.PathSeparator) + entries[0].Name())
	if err != nil {
		t.Fatalf("read saved log bundle: %v", err)
	}
	if !bytes.Equal(saved, logBundle) {
		t.Fatal("saved log bundle does not match download")
	}
	var secondRunErr error
	secondStdout, _ := captureOutput(t, func() {
		secondRunErr = runDoctor()
	})
	if secondRunErr != nil {
		t.Fatalf("second doctor run error = %v, want completed report", secondRunErr)
	}
	var secondResult struct {
		CoverageWarnings []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"coverageWarnings"`
	}
	if err := json.Unmarshal([]byte(secondStdout), &secondResult); err != nil {
		t.Fatalf("decode second doctor output %q: %v", secondStdout, err)
	}
	if len(secondResult.CoverageWarnings) != 1 || secondResult.CoverageWarnings[0].ID != "log_bundle_inspection_failed" || !strings.Contains(secondResult.CoverageWarnings[0].Message, "exist") {
		t.Fatalf("second doctor warnings = %+v, want existing-file coverage warning", secondResult.CoverageWarnings)
	}
}

func TestXcodeCloudDoctorSuccessfulRunDoesNotDownloadLogs(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	requestedArtifactDetail := false
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/v1/ciBuildRuns/run-1":
			body = `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"COMPLETE","completionStatus":"SUCCEEDED"}}}`
		case "/v1/ciBuildRuns/run-1/actions":
			body = `{"data":[{"type":"ciBuildActions","id":"action-1","attributes":{"actionType":"ARCHIVE","executionProgress":"COMPLETE","completionStatus":"SUCCEEDED"}}]}`
		case "/v1/ciBuildActions/action-1/issues":
			body = `{"data":[]}`
		case "/v1/ciBuildActions/action-1/artifacts":
			body = `{"data":[{"type":"ciArtifacts","id":"log-1","attributes":{"fileType":"LOG_BUNDLE"}}]}`
		case "/v1/ciArtifacts/log-1":
			requestedArtifactDetail = true
			t.Fatalf("successful run should not download log bundle")
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "doctor", "--run-id", "run-1", "--output", "json"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requestedArtifactDetail {
		t.Fatal("successful run requested artifact details")
	}
	var result struct {
		Summary struct {
			LogBundles          int `json:"logBundles"`
			LogBundlesInspected int `json:"logBundlesInspected"`
		} `json:"summary"`
		Conclusion string `json:"conclusion"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if result.Summary.LogBundles != 1 || result.Summary.LogBundlesInspected != 0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if !strings.Contains(result.Conclusion, "completed successfully") {
		t.Fatalf("unexpected conclusion %q", result.Conclusion)
	}
}

func TestXcodeCloudDoctorWaitsForTerminalRunAndRendersTable(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	runRequests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/v1/ciBuildRuns/run-1":
			runRequests++
			if runRequests == 1 {
				body = `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"RUNNING"}}}`
			} else {
				body = `{"data":{"type":"ciBuildRuns","id":"run-1","attributes":{"executionProgress":"COMPLETE","completionStatus":"FAILED"}}}`
			}
		case "/v1/ciBuildRuns/run-1/actions":
			body = `{"data":[]}`
		default:
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"xcode-cloud", "doctor", "--run-id", "run-1", "--wait", "--poll-interval", "1ms", "--output", "table"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if runRequests != 2 {
		t.Fatalf("run requests = %d, want 2", runRequests)
	}
	for _, expected := range []string{"Summary field", "completionStatus", "FAILED", "Actions ID", "Xcode Cloud build run failed"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("table output missing %q: %s", expected, stdout)
		}
	}
}

func TestXcodeCloudDoctorReportsRunAPIError(t *testing.T) {
	setupAuth(t)

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Path != "/v1/ciBuildRuns/run-1" {
			t.Fatalf("unexpected request: %s", req.URL.String())
		}
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(`{"errors":[{"status":"500","code":"SERVER_ERROR","title":"Unavailable"}]}`)),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	if err := root.Parse([]string{"xcode-cloud", "doctor", "--run-id", "run-1"}); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	err := root.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "xcode-cloud doctor") {
		t.Fatalf("run error = %v, want doctor API error", err)
	}
}

func buildXcodeCloudLogBundle(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	file, err := archive.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := io.WriteString(file, content); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buffer.Bytes()
}
