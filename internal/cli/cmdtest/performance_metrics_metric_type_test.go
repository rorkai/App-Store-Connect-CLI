package cmdtest

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func runPerformanceMetricsListWithMetricType(t *testing.T, metricType string) (string, string, error) {
	t.Helper()

	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps/app-1/perfPowerMetrics", "filter[metricType]="+metricType), func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"productData":[],"version":"1.0"}`)),
			Header:     http.Header{"Content-Type": []string{"application/vnd.apple.xcode-metrics+json"}},
		}, nil
	})

	return runRootWithDeps(t, stubClient(t, reg), "performance", "metrics", "list", "--app", "app-1", "--metric-type", metricType)
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
	stdout, _, runErr := runRootWithDeps(t, nil, "performance", "metrics", "list", "--app", "app-1", "--metric-type", "BOGUS")

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
	stdout, _, runErr := runRootWithDeps(t, nil, "performance", "metrics", "view", "--build-id", "build-1", "--metric-type", "BOGUS")

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
