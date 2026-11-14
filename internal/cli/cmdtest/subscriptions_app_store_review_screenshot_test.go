package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestSubscriptionsReviewAppStoreScreenshotViewNullRelationshipReturnsNotFound(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	requests := 0
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.Method != http.MethodGet || req.URL.Path != "/v1/subscriptions/6759789602/appStoreReviewScreenshot" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return jsonHTTPResponse(http.StatusOK, `{"data":null,"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/6759789602/appStoreReviewScreenshot"}}`), nil
	})

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"subscriptions", "review", "app-store-screenshot", "view",
			"--subscription-id", "6759789602",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitNotFound {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitNotFound, code)
		}
	})

	if requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, `subscriptions review app-store-screenshot view: no App Store review screenshot found for subscription "6759789602"`) {
		t.Fatalf("expected clear not-found stderr, got %q", stderr)
	}
}

func TestSubscriptionsReviewAppStoreScreenshotViewPrintsPopulatedRelationship(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/subscriptions/6759789602/appStoreReviewScreenshot"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"subscriptionAppStoreReviewScreenshots","id":"shot-1","attributes":{"fileName":"review.png"}},"links":{"self":"https://api.appstoreconnect.apple.com/v1/subscriptions/6759789602/appStoreReviewScreenshot"}}`))

	stdout, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{
			"subscriptions", "review", "app-store-screenshot", "view",
			"--subscription-id", "6759789602",
			"--output", "json",
		}, "1.2.3")
		if code != cmd.ExitSuccess {
			t.Fatalf("expected exit code %d, got %d", cmd.ExitSuccess, code)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	var output struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout: %s", err, stdout)
	}
	if output.Data.ID != "shot-1" {
		t.Fatalf("expected screenshot id shot-1, got %q", output.Data.ID)
	}
}
