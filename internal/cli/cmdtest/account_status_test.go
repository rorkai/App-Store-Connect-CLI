package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestAccountStatusJSON(t *testing.T) {
	for _, tc := range []struct {
		name        string
		brokerURL   string
		brokerToken string
		want        []string
		reject      []string
	}{
		{
			name: "own web session",
			want: []string{"asc web agreements status", "asc web business agreements list"},
		},
		{
			// A lent web session cannot read agreements, so naming the
			// agreement commands would send the caller into a refusal.
			name:        "lent web session",
			brokerURL:   "https://broker.example.com",
			brokerToken: "broker-token",
			want:        []string{"Account Holder", "https://appstoreconnect.apple.com/business", "https://developer.apple.com/account", "Program License Agreement"},
			reject:      []string{"asc web", "asc "},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agreementsMessage := runAccountStatusAgreementsCheck(t, tc.brokerURL, tc.brokerToken)
			for _, want := range tc.want {
				if !strings.Contains(agreementsMessage, want) {
					t.Fatalf("expected agreements check to name %q, got %q", want, agreementsMessage)
				}
			}
			for _, reject := range tc.reject {
				if strings.Contains(agreementsMessage, reject) {
					t.Fatalf("expected agreements check not to name %q, got %q", reject, agreementsMessage)
				}
			}
		})
	}
}

func runAccountStatusAgreementsCheck(t *testing.T, brokerURL, brokerToken string) string {
	t.Helper()
	setupAuth(t)
	t.Setenv("ASC_WEB_BROKER_URL", brokerURL)
	t.Setenv("ASC_WEB_BROKER_TOKEN", brokerToken)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"sku"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "account", "status")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	summary, ok := payload["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary object, got %T", payload["summary"])
	}
	if summary["health"] == "" {
		t.Fatalf("expected summary.health, got %v", summary)
	}

	checks, ok := payload["checks"].([]any)
	if !ok || len(checks) == 0 {
		t.Fatalf("expected checks array, got %T %v", payload["checks"], payload["checks"])
	}

	foundAPICheck := false
	agreementsMessage := ""
	for _, item := range checks {
		check, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if check["name"] == "api_access" {
			foundAPICheck = true
		}
		if check["name"] == "agreements" {
			agreementsMessage, _ = check["message"].(string)
		}
	}
	if !foundAPICheck {
		t.Fatalf("expected api_access check in %v", checks)
	}
	return agreementsMessage
}

func TestAccountStatusForbiddenProducesWarning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusForbidden, `{
		"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"permission denied"}]
	}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "account", "status")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}

	checks, ok := payload["checks"].([]any)
	if !ok {
		t.Fatalf("expected checks array, got %T", payload["checks"])
	}
	var apiStatus string
	for _, item := range checks {
		check, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if check["name"] == "api_access" {
			apiStatus, _ = check["status"].(string)
		}
	}
	if apiStatus != "warn" {
		t.Fatalf("expected api_access warn status, got %q", apiStatus)
	}
}

func TestAccountStatusTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"sku"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runRootWithDeps(t, stubClient(t, reg), "account", "status", "--output", "table")
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, "SUMMARY") || !strings.Contains(stdout, "CHECKS") {
		t.Fatalf("expected summary/checks sections in table output, got %q", stdout)
	}
}
