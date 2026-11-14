package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

// TestMigrateExportPreservesAppInfoPrivacyPolicyURL verifies that export keeps
// the App Store Connect privacy policy URL in fastlane's privacy_url.txt file,
// while omitted and empty values remain absent.
func TestMigrateExportPreservesAppInfoPrivacyPolicyURL(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	outputDir := filepath.Join(t.TempDir(), "fastlane")
	const privacyURL = "https://example.com/privacy?locale=en-US#policy"
	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_ID","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_ID"}}}}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/apps/APP_ID/appInfos"),
		asctest.JSON(http.StatusOK, `{"data":[{"type":"appInfos","id":"INFO_ID","attributes":{"state":"PREPARE_FOR_SUBMISSION"}}],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appInfos/INFO_ID/appInfoLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[
			{"type":"appInfoLocalizations","id":"LOC_EN","attributes":{"locale":"en-US","name":"English","subtitle":"English subtitle","privacyPolicyUrl":"`+privacyURL+`"}},
			{"type":"appInfoLocalizations","id":"LOC_FR","attributes":{"locale":"fr-FR","name":"French","subtitle":"French subtitle"}},
			{"type":"appInfoLocalizations","id":"LOC_DE","attributes":{"locale":"de-DE","name":"German","subtitle":"German subtitle","privacyPolicyUrl":""}}
		],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_ID/appClipDefaultExperience"),
		asctest.JSON(http.StatusOK, `{"data":null}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
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
	versionLocalizationsLimit200 := false
	for _, req := range reg.Requests() {
		if req.URL.Path == "/v1/appStoreVersions/VERSION_ID/appStoreVersionLocalizations" && req.URL.Query().Get("limit") == "200" {
			versionLocalizationsLimit200 = true
		}
	}
	if !versionLocalizationsLimit200 {
		t.Fatal("expected version localizations to be requested with limit=200")
	}
	var result struct {
		TotalFiles int `json:"totalFiles"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode export result: %v", err)
	}
	if result.TotalFiles != 7 {
		t.Fatalf("totalFiles = %d, want 7 including privacy_url.txt", result.TotalFiles)
	}

	privacyPath := filepath.Join(outputDir, "metadata", "en-US", "privacy_url.txt")
	data, err := os.ReadFile(privacyPath)
	if err != nil {
		t.Fatalf("read exported privacy URL: %v", err)
	}
	if got := string(data); got != privacyURL+"\n" {
		t.Fatalf("privacy_url.txt = %q, want %q", got, privacyURL+"\n")
	}

	for _, locale := range []string{"fr-FR", "de-DE"} {
		path := filepath.Join(outputDir, "metadata", locale, "privacy_url.txt")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected no privacy_url.txt for %s, stat error = %v", locale, err)
		}
	}
}
