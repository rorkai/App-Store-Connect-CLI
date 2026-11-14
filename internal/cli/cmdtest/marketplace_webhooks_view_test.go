package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMarketplaceWebhooksViewUsesCollectionAndPrintsSingleResource(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/marketplaceWebhooks", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"marketplaceWebhooks","id":"wh-1","attributes":{"endpointUrl":"https://example.com/webhook"}}],"links":{}}`))

	stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "marketplace", "webhooks", "view", "--webhook-id", "wh-1", "--output", "json")
	if code != rootcmd.ExitSuccess {
		t.Fatalf("expected exit code %d, got %d", rootcmd.ExitSuccess, code)
	}

	if requests := len(reg.Requests()); requests != 1 {
		t.Fatalf("expected one request, got %d", requests)
	}
	if !strings.Contains(stderr, "Warning: marketplace webhooks endpoints are deprecated in App Store Connect API.") {
		t.Fatalf("expected deprecation warning, got %q", stderr)
	}
	var output struct {
		Data struct {
			ID         string `json:"id"`
			Attributes struct {
				EndpointURL string `json:"endpointUrl"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout=%s", err, stdout)
	}
	if output.Data.ID != "wh-1" || output.Data.Attributes.EndpointURL != "https://example.com/webhook" {
		t.Fatalf("unexpected output: %+v", output.Data)
	}
}

func TestMarketplaceWebhooksViewMissingIDReturnsUsageExitCode(t *testing.T) {
	stdout, stderr, code := runWithDeps(t, nil, "marketplace", "webhooks", "view")
	if code != rootcmd.ExitUsage {
		t.Fatalf("expected exit code %d, got %d", rootcmd.ExitUsage, code)
	}

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if strings.Count(stderr, "Error: --webhook-id is required") != 1 {
		t.Fatalf("expected one missing-ID error, got %q", stderr)
	}
}

func TestMarketplaceWebhooksViewFailureExitCodes(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantCode   int
		wantStderr string
	}{
		{
			name:       "not found",
			status:     http.StatusOK,
			body:       `{"data":[],"links":{}}`,
			wantCode:   rootcmd.ExitNotFound,
			wantStderr: `marketplace webhooks view: failed to fetch: marketplace webhook "wh-1" not found`,
		},
		{
			name:       "API error",
			status:     http.StatusBadRequest,
			body:       `{"errors":[{"status":"400","code":"INVALID_REQUEST","title":"Invalid Request","detail":"request rejected"}]}`,
			wantCode:   rootcmd.ExitHTTPBadRequest,
			wantStderr: "marketplace webhooks view: failed to fetch: Invalid Request: request rejected",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			setupAuth(t)
			t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

			reg := asctest.NewRegistry(t)
			reg.Register(asctest.GET("/v1/marketplaceWebhooks", "limit=200"), asctest.JSON(test.status, test.body))

			stdout, stderr, code := runWithDeps(t, stubClient(t, reg), "marketplace", "webhooks", "view", "--webhook-id", "wh-1", "--output", "json")
			if code != test.wantCode {
				t.Fatalf("expected exit code %d, got %d", test.wantCode, code)
			}

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantStderr) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantStderr, stderr)
			}
		})
	}
}
