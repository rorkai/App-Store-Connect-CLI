package cmdtest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMetadataKeywordsSyncConfiguredBundleDoesNotFallBackToName(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "com.example.missing")

	dir := t.TempDir()
	inputPath := filepath.Join(t.TempDir(), "keywords.json")
	if err := os.WriteFile(inputPath, []byte(`{"locale":"en-US","keywords":["habit tracker"]}`), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(
		asctest.GET("/v1/apps", "filter[bundleId]=com.example.missing", "limit=200"),
		asctest.JSON(200, `{"data":[{"type":"apps","id":"100000001","attributes":{"bundleId":"com.example.other","name":"com.example.missing"}}],"links":{}}`),
	)

	stdout, _, err := runRootWithDeps(
		t,
		stubClient(t, reg),
		"metadata", "keywords", "sync",
		"--version", "1.2.3",
		"--dir", dir,
		"--input", inputPath,
		"--format", "json",
		"--dry-run",
	)
	if err == nil || !strings.Contains(err.Error(), `no app found for bundle ID "com.example.missing" (exact match required)`) {
		t.Fatalf("err = %v, want exact configured-bundle refusal", err)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty output", stdout)
	}
	requests := reg.Requests()
	if len(requests) != 1 || requests[0].URL.Path != "/v1/apps" {
		t.Fatalf("requests = %v, want only the exact app lookup", requests)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "version", "1.2.3", "en-US.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected dry-run to avoid local writes, got err=%v", statErr)
	}
}
