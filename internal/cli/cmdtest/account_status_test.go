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
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"sku"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{"account", "status"})
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
	for _, item := range checks {
		check, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if check["name"] == "api_access" {
			foundAPICheck = true
		}
	}
	if !foundAPICheck {
		t.Fatalf("expected api_access check in %v", checks)
	}
}

func TestAccountStatusForbiddenProducesWarning(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusForbidden, `{
		"errors":[{"status":"403","code":"FORBIDDEN","title":"Forbidden","detail":"permission denied"}]
	}`))

	stdout, stderr, err := runCommand(t, []string{"account", "status"})
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
	reg.Install()
	reg.Register(asctest.GET("/v1/apps", "limit=1"), asctest.JSON(http.StatusOK, `{
			"data":[
				{"type":"apps","id":"app-1","attributes":{"name":"My App","bundleId":"com.example.myapp","sku":"sku"}}
			],
			"links":{"next":""}
		}`))

	stdout, stderr, err := runCommand(t, []string{"account", "status", "--output", "table"})
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
