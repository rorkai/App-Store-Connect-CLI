package cmdtest

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

type validateIAPFixture struct {
	iaps string
}

func newValidateIAPClient(t *testing.T, fixture validateIAPFixture) *asc.Client {
	t.Helper()

	tmpDir := t.TempDir()
	keyPath := filepath.Join(tmpDir, "key.p8")
	writeECDSAPEM(t, keyPath)

	notFound := `{"errors":[{"code":"NOT_FOUND","title":"Not Found","detail":"resource not found"}]}`

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet {
			return jsonResponse(http.StatusMethodNotAllowed, `{"errors":[{"status":405}]}`)
		}

		switch req.URL.Path {
		case "/v1/apps/app-1/inAppPurchasesV2":
			return jsonResponse(http.StatusOK, fixture.iaps)
		default:
			return jsonResponse(http.StatusNotFound, notFound)
		}
	})

	httpClient := &http.Client{Transport: transport}
	client, err := asc.NewClientWithHTTPClient("KEY123", "ISS456", keyPath, httpClient)
	if err != nil {
		t.Fatalf("NewClientWithHTTPClient() error: %v", err)
	}
	return client
}

func validValidateIAPFixture() validateIAPFixture {
	return validateIAPFixture{
		iaps: `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"NON_CONSUMABLE","state":"APPROVED"}}]}`,
	}
}

func TestValidateIAPRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	stdout, stderr, err := runRootWithDeps(t, nil, "validate", "iap")
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "--app is required") {
		t.Fatalf("expected --app required error, got %q", stderr)
	}
}

func TestValidateIAPOutputsJSONAndTable(t *testing.T) {
	fixture := validValidateIAPFixture()
	client := newValidateIAPClient(t, fixture)
	stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "iap", "--app", "app-1")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.IAPReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Errors != 0 || report.Summary.Warnings != 0 {
		t.Fatalf("expected no issues, got %+v", report.Summary)
	}

	stdout, _, err = runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "iap", "--app", "app-1", "--output", "table")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stdout, "Severity") {
		t.Fatalf("expected table output to include headers, got %q", stdout)
	}
}

func TestValidateIAPWarnsAndStrictFails(t *testing.T) {
	fixture := validValidateIAPFixture()
	fixture.iaps = `{"data":[{"type":"inAppPurchases","id":"iap-1","attributes":{"name":"Pro","productId":"com.example.pro","inAppPurchaseType":"NON_CONSUMABLE","state":"READY_TO_SUBMIT"}}]}`

	client := newValidateIAPClient(t, fixture)
	stdout, stderr, err := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "iap", "--app", "app-1")
	if err != nil {
		t.Fatalf("expected no error (warning-only), got %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var report validation.IAPReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	if report.Summary.Warnings == 0 {
		t.Fatalf("expected warnings, got %+v", report.Summary)
	}

	stdout, _, runErr := runRootWithDeps(t, func() (*asc.Client, error) { return client, nil }, "validate", "iap", "--app", "app-1", "--strict")
	if runErr == nil {
		t.Fatalf("expected error with --strict")
	}
	if _, ok := errors.AsType[ReportedError](runErr); !ok {
		t.Fatalf("expected ReportedError, got %v", runErr)
	}

	var strictReport validation.IAPReport
	if err := json.Unmarshal([]byte(stdout), &strictReport); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}
	found := false
	for _, check := range strictReport.Checks {
		if check.ID == "iap.review_readiness.needs_attention" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected iap.review_readiness.needs_attention check, got %+v", strictReport.Checks)
	}
}
