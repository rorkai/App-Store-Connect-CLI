package cmdtest

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func runPerformanceMetricsListWithMetricType(t *testing.T, metricType string) (string, string, error) {
	t.Helper()

	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/v1/apps/app-1/perfPowerMetrics" {
			t.Fatalf("expected perfPowerMetrics path, got %s", req.URL.Path)
		}
		if got := req.URL.Query().Get("filter[metricType]"); got != metricType {
			t.Fatalf("expected filter[metricType]=%q, got %q", metricType, got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"productData":[],"version":"1.0"}`)),
			Header:     http.Header{"Content-Type": []string{"application/vnd.apple.xcode-metrics+json"}},
		}, nil
	})

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"performance", "metrics", "list", "--app", "app-1", "--metric-type", metricType}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func TestPerformanceMetricsListAcceptsStorageMetricType(t *testing.T) {
	stdout, stderr, err := runPerformanceMetricsListWithMetricType(t, "STORAGE")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"productData"`) {
		t.Fatalf("expected metrics payload in stdout, got %q", stdout)
	}
}

func TestPerformanceMetricsListRejectsInvalidMetricType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"performance", "metrics", "list", "--app", "app-1", "--metric-type", "BOGUS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	wantErr := "--metric-type must be one of: ANIMATION, BATTERY, DISK, HANG, LAUNCH, MEMORY, STORAGE, TERMINATION"
	if !strings.Contains(runErr.Error(), wantErr) {
		t.Fatalf("expected error %q, got %v", wantErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func TestPerformanceMetricsViewRejectsInvalidMetricType(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := root.Parse([]string{"performance", "metrics", "view", "--build", "build-1", "--metric-type", "BOGUS"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected error, got nil")
	}
	wantErr := "--metric-type must be one of: ANIMATION, BATTERY, DISK, HANG, LAUNCH, MEMORY, STORAGE, TERMINATION"
	if !strings.Contains(runErr.Error(), wantErr) {
		t.Fatalf("expected error %q, got %v", wantErr, runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
