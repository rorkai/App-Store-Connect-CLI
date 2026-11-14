package cmdtest

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestBuildsUpdateSetsUsesNonExemptEncryption(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/builds/build-99").JSONBody(`{"data":{"type":"builds","id":"build-99","attributes":{"usesNonExemptEncryption":false}}}`),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-99","attributes":{"version":"2.0","uploadedDate":"2026-03-18T00:00:00Z","usesNonExemptEncryption":false}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "update",
		"--build-id", "build-99",
		"--uses-non-exempt-encryption=false",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 1 {
		t.Fatalf("expected exactly one request, got %d", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-99"`) {
		t.Fatalf("expected updated build in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"usesNonExemptEncryption":false`) {
		t.Fatalf("expected updated encryption attribute in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Updated build build-99") {
		t.Fatalf("expected success stderr, got %q", stderr)
	}
}

func TestBuildsUpdateTreatsAlreadySetValueAsNoOp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.PATCH("/v1/builds/build-99").JSONBody(`{"data":{"type":"builds","id":"build-99","attributes":{"usesNonExemptEncryption":false}}}`),
		asctest.JSON(http.StatusConflict, `{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"Build update conflict","detail":"The request could not be completed."}]}`))
	reg.Register(asctest.GET("/v1/builds/build-99"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"builds","id":"build-99","attributes":{"version":"2.0","uploadedDate":"2026-03-18T00:00:00Z","usesNonExemptEncryption":false}}}`))

	stdout, stderr, err := runCommand(t, []string{
		"builds", "update",
		"--build-id", "build-99",
		"--uses-non-exempt-encryption=false",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if requestCount := len(reg.Requests()); requestCount != 2 {
		t.Fatalf("expected PATCH then GET, got %d requests", requestCount)
	}
	if !strings.Contains(stdout, `"id":"build-99"`) {
		t.Fatalf("expected current build in stdout, got %q", stdout)
	}
	if !strings.Contains(stdout, `"usesNonExemptEncryption":false`) {
		t.Fatalf("expected false encryption state in stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Updated build build-99") {
		t.Fatalf("expected success stderr, got %q", stderr)
	}
}
