package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMigrateExportRemovesStaleKnownMetadataFiles(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputDir := filepath.Join(t.TempDir(), "fastlane")
	localeDir := filepath.Join(outputDir, "metadata", "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("create locale directory: %v", err)
	}
	staleFiles := map[string]string{
		"description.txt": "obsolete description\n",
		"keywords.txt":    "obsolete,keywords\n",
		"name.txt":        "obsolete name\n",
	}
	for name, content := range staleFiles {
		if err := os.WriteFile(filepath.Join(localeDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(localeDir, "keep.txt"), []byte("unrelated file\n"), 0o644); err != nil {
		t.Fatalf("write unrelated file: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appStoreVersionLocalizations","id":"LOC_EN","attributes":{"locale":"en-US","description":"","keywords":"","whatsNew":"","promotionalText":"","supportUrl":"","marketingUrl":""}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/APP_ID/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appInfos/INFO_ID/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfoLocalizations","id":"LOC_EN","attributes":{"locale":"en-US","name":"","subtitle":"","privacyPolicyUrl":""}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience"),
		asctest.JSON(http.StatusOK, `{"data":null}`))
	reg.Register(asctest.GET("/v1/appStoreVersionLocalizations/LOC_EN/appPreviewSets", "include=appPreviews", "limit[appPreviews]=50"),
		asctest.JSON(http.StatusOK, `{"data":[]}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, stderr := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "export",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--output-dir", outputDir,
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})
	if runErr != nil {
		t.Fatalf("migrate export error: %v", runErr)
	}
	if stderr != "Warning: version has no App Clip default experience\n" {
		t.Fatalf("stderr = %q, want only the missing App Clip warning", stderr)
	}
	var result struct {
		TotalFiles int `json:"totalFiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode export result: %v", err)
	}
	if result.TotalFiles != 0 {
		t.Fatalf("totalFiles = %d, want 0", result.TotalFiles)
	}

	if runtime.GOOS == "windows" {
		for name, want := range staleFiles {
			got, err := os.ReadFile(filepath.Join(localeDir, name))
			if err != nil {
				t.Fatalf("read stale %s after unsupported export: %v", name, err)
			}
			if string(got) != want {
				t.Fatalf("stale %s = %q, want unchanged %q", name, got, want)
			}
		}
		if got, err := os.ReadFile(filepath.Join(localeDir, "keep.txt")); err != nil {
			t.Fatalf("read unrelated file after unsupported export: %v", err)
		} else if string(got) != "unrelated file\n" {
			t.Fatalf("unrelated file = %q, want unchanged", got)
		}
		return
	}

	for _, name := range []string{"description.txt", "keywords.txt", "name.txt"} {
		if _, err := os.Stat(filepath.Join(localeDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s still exists, stat error = %v", name, err)
		}
	}
	if got, err := os.ReadFile(filepath.Join(localeDir, "keep.txt")); err != nil {
		t.Fatalf("read unrelated file: %v", err)
	} else if string(got) != "unrelated file\n" {
		t.Fatalf("unrelated file = %q, want unchanged", got)
	}
}
