package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestTestFlightMetricsAppTestersValidationErrors(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "missing app",
			args:    []string{"testflight", "metrics", "app-testers"},
			wantErr: "Error: --app is required (or set ASC_APP_ID)",
		},
		{
			name:    "invalid period",
			args:    []string{"testflight", "metrics", "app-testers", "--app", "APP_ID", "--period", "P1D"},
			wantErr: "--period must be one of: P7D, P30D, P90D, P365D",
		},
		{
			name:    "limit out of range",
			args:    []string{"testflight", "metrics", "app-testers", "--app", "APP_ID", "--limit", "500"},
			wantErr: "Error: --limit must be between 1 and 200",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUsageExit(t, test.args, test.wantErr)
		})
	}
}

func TestTestFlightMetricsAppTestersNextWithoutApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	nextURL := "https://api.appstoreconnect.apple.com/v1/apps/app-123/metrics/betaTesterUsages?limit=2"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"id":"usage-1"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--next", nextURL})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"usage-1\"") {
		t.Fatalf("expected usage in output, got %q", stdout)
	}
}

func TestTestFlightMetricsAppTestersPaginate(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.p8")
	writeECDSAPEM(t, keyPath)
	t.Setenv("ASC_KEY_ID", "TEST_KEY")
	t.Setenv("ASC_ISSUER_ID", "TEST_ISSUER")
	t.Setenv("ASC_PRIVATE_KEY_PATH", keyPath)

	firstURL := "https://api.appstoreconnect.apple.com/v1/apps/app-123/metrics/betaTesterUsages?limit=2"
	secondURL := "https://api.appstoreconnect.apple.com/v1/apps/app-123/metrics/betaTesterUsages?page=2"

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "limit=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"id":"usage-1"}],"links":{"next":"`+secondURL+`"}}`))
	reg.Register(asctest.GET("/v1/apps/app-123/metrics/betaTesterUsages", "page=2"),
		asctest.JSON(http.StatusOK, `{"data":[{"id":"usage-2"}],"links":{"next":""}}`))

	stdout, stderr, err := runCommand(t, []string{"testflight", "metrics", "app-testers", "--paginate", "--next", firstURL})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "\"usage-1\"") || !strings.Contains(stdout, "\"usage-2\"") {
		t.Fatalf("expected both usages in output, got %q", stdout)
	}
}
