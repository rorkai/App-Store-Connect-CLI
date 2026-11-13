package cmdtest

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestGameCenterDetailsListNoDetailReturnsEmptyList(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	expectedURL := "https://api.appstoreconnect.apple.com/v1/apps/APP_ID/gameCenterDetail"

	reg := asctest.NewRegistry(t)
	reg.Install()
	// App Store Connect returns 200 with an empty id when no Game Center detail exists yet.
	reg.Register(asctest.GET("/v1/apps/APP_ID/gameCenterDetail"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"gameCenterDetails","id":"","attributes":{}},"links":{"self":"`+expectedURL+`"}}`))

	stdout, stderr, err := runCommand(t, []string{"game-center", "details", "list", "--app", "APP_ID"})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !strings.Contains(stderr, "Warning: no Game Center detail exists for this app") {
		t.Fatalf("expected warning in stderr, got %q", stderr)
	}

	var resp struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nstdout: %q", err, stdout)
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected empty data array, got %d items", len(resp.Data))
	}
}
