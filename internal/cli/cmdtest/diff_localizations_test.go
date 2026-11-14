package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestDiffLocalizationsRequiresAppID(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)
	t.Setenv("ASC_APP_ID", "")

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"diff", "localizations", "--path", "./metadata", "--version", "version-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: --app is required (or set ASC_APP_ID)") {
		t.Fatalf("expected missing app error, got %q", stderr)
	}
}

func TestDiffLocalizationsRequiresSourceSelector(t *testing.T) {
	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{"diff", "localizations", "--app", "app-1", "--version", "version-1"}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: either --path or --from-version is required") {
		t.Fatalf("expected source selector error, got %q", stderr)
	}
}

func TestDiffLocalizationsRejectsUnknownLocalField(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "localizations")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	content := "\"description\" = \"Test\";\n\"unknownKey\" = \"bad\";\n"
	if err := os.WriteFile(filepath.Join(inputDir, "en-US.strings"), []byte(content), 0o644); err != nil {
		t.Fatalf("write strings failed: %v", err)
	}

	root := RootCommand("1.2.3")
	root.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse([]string{
			"diff", "localizations",
			"--app", "app-1",
			"--path", inputDir,
			"--version", "version-1",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = root.Run(context.Background())
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: unsupported keys for locale \"en-US\": unknownKey") {
		t.Fatalf("expected unknown-key usage error, got %q", stderr)
	}
}

func TestDiffLocalizationsLocalToRemoteJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "localizations")
	if err := os.MkdirAll(inputDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	content := "\"description\" = \"Local description\";\n\"keywords\" = \"alpha,beta\";\n"
	if err := os.WriteFile(filepath.Join(inputDir, "en-US.strings"), []byte(content), 0o644); err != nil {
		t.Fatalf("write strings failed: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1", "include=app"),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-1",
					"attributes":{"versionString":"1.0"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-1/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data": [{
					"type":"appStoreVersionLocalizations",
					"id":"loc-1",
					"attributes":{
						"locale":"en-US",
						"description":"Remote description",
						"marketingUrl":"https://example.com/marketing"
					}
				}],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"diff", "localizations",
		"--app", "app-1",
		"--path", inputDir,
		"--version", "version-1",
	})
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
	if payload["scope"] != "localizations" {
		t.Fatalf("expected scope localizations, got %v", payload["scope"])
	}
	if payload["direction"] != "source-to-target" {
		t.Fatalf("expected source-to-target direction, got %v", payload["direction"])
	}

	adds, ok := payload["adds"].([]any)
	if !ok || len(adds) != 1 {
		t.Fatalf("expected one add, got %T %+v", payload["adds"], payload["adds"])
	}
	updates, ok := payload["updates"].([]any)
	if !ok || len(updates) != 1 {
		t.Fatalf("expected one update, got %T %+v", payload["updates"], payload["updates"])
	}
	deletes, ok := payload["deletes"].([]any)
	if !ok || len(deletes) != 1 {
		t.Fatalf("expected one delete, got %T %+v", payload["deletes"], payload["deletes"])
	}

	add, ok := adds[0].(map[string]any)
	if !ok {
		t.Fatalf("expected add item object, got %T", adds[0])
	}
	if add["key"] != "en-US:marketingUrl" || add["field"] != "marketingUrl" || add["to"] != "https://example.com/marketing" {
		t.Fatalf("unexpected add item: %+v", add)
	}
	if add["reason"] != "field exists in target but not in source" {
		t.Fatalf("unexpected add reason: %+v", add)
	}

	update, ok := updates[0].(map[string]any)
	if !ok {
		t.Fatalf("expected update item object, got %T", updates[0])
	}
	if update["key"] != "en-US:description" || update["from"] != "Local description" || update["to"] != "Remote description" {
		t.Fatalf("unexpected update item: %+v", update)
	}
	if update["reason"] != "field value differs" {
		t.Fatalf("unexpected update reason: %+v", update)
	}

	del, ok := deletes[0].(map[string]any)
	if !ok {
		t.Fatalf("expected delete item object, got %T", deletes[0])
	}
	if del["key"] != "en-US:keywords" || del["from"] != "alpha,beta" || del["field"] != "keywords" {
		t.Fatalf("unexpected delete item: %+v", del)
	}
	if del["reason"] != "field exists in source but not in target" {
		t.Fatalf("unexpected delete reason: %+v", del)
	}
}

func TestDiffLocalizationsRemoteToRemoteJSON(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-from", "include=app"),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-from",
					"attributes":{"versionString":"1.0"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-to", "include=app"),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version-to",
					"attributes":{"versionString":"1.1"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-from/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appStoreVersionLocalizations","id":"from-1","attributes":{"locale":"en-US","description":"A"}}],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-to/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appStoreVersionLocalizations","id":"to-1","attributes":{"locale":"en-US","description":"B"}}],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"diff", "localizations",
		"--app", "app-1",
		"--from-version", "version-from",
		"--to-version", "version-to",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if requests := len(reg.Requests()); requests != 4 {
		t.Fatalf("expected 4 requests, got %d", requests)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("unmarshal output: %v\nstdout=%s", err, stdout)
	}
	if payload["direction"] != "source-to-target" {
		t.Fatalf("expected source-to-target direction, got %v", payload["direction"])
	}
	updates, ok := payload["updates"].([]any)
	if !ok || len(updates) != 1 {
		t.Fatalf("expected one update, got %T %+v", payload["updates"], payload["updates"])
	}

	update, ok := updates[0].(map[string]any)
	if !ok {
		t.Fatalf("expected update item object, got %T", updates[0])
	}
	if update["key"] != "en-US:description" || update["from"] != "A" || update["to"] != "B" {
		t.Fatalf("unexpected update item: %+v", update)
	}
	if update["reason"] != "field value differs" {
		t.Fatalf("unexpected update reason: %+v", update)
	}
}

func TestDiffLocalizationsTableOutput(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-from", "include=app"),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version",
					"attributes":{"versionString":"1.0"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-to", "include=app"),
		asctest.JSON(http.StatusOK, `{
				"data":{
					"type":"appStoreVersions",
					"id":"version",
					"attributes":{"versionString":"1.0"},
					"relationships":{"app":{"data":{"type":"apps","id":"app-1"}}}
				}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-from/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appStoreVersionLocalizations","id":"from-1","attributes":{"locale":"en-US","description":"A"}}],
				"links":{"next":""}
			}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/version-to/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{
				"data":[{"type":"appStoreVersionLocalizations","id":"to-1","attributes":{"locale":"en-US","description":"B"}}],
				"links":{"next":""}
			}`))

	stdout, stderr, err := runCommand(t, []string{
		"diff", "localizations",
		"--app", "app-1",
		"--from-version", "version-from",
		"--to-version", "version-to",
		"--output", "table",
	})
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	for _, needle := range []string{"change", "key", "locale", "field", "reason"} {
		if !strings.Contains(strings.ToLower(stdout), needle) {
			t.Fatalf("expected table output to contain %q, got %q", needle, stdout)
		}
	}
}

func TestDiffLocalizationsRejectsCrossAppVersionIDs(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))
	t.Setenv("ASC_APP_ID", "")

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/version-from", "include=app"),
		asctest.JSON(http.StatusOK, `{
			"data":{
				"type":"appStoreVersions",
				"id":"version-from",
				"attributes":{"versionString":"1.0"},
				"relationships":{"app":{"data":{"type":"apps","id":"app-other"}}}
			}
		}`))

	stdout, stderr, runErr := runCommand(t, []string{
		"diff", "localizations",
		"--app", "app-1",
		"--from-version", "version-from",
		"--to-version", "version-to",
	})

	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "Error: version \"version-from\" belongs to app \"app-other\", expected --app \"app-1\"") {
		t.Fatalf("expected app mismatch usage error, got %q", stderr)
	}
}
