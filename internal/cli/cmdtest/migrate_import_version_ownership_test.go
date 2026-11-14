package cmdtest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func writeMigrateOwnershipFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(metadataDir, "description.txt"), "English description")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots root: %v", err)
	}
	return fastlaneDir
}

// TestMigrateImportRejectsVersionOwnedByAnotherAppBeforeMutation proves that an
// explicit --version-id is checked against --app before migrate import writes
// version localizations, review details, or screenshots.
func TestMigrateImportRejectsVersionOwnedByAnotherAppBeforeMutation(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	fastlaneDir := writeMigrateOwnershipFixture(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_B", "include=app"),
		asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_B","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_B"}}}}}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_A",
			"--version-id", "VERSION_B",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail for a version owned by another app")
	}
	if !strings.Contains(runErr.Error(), "belongs to app") {
		t.Fatalf("expected ownership error, got %v", runErr)
	}
	var mutations []string
	for _, req := range reg.Requests() {
		if req.Method != http.MethodGet {
			mutations = append(mutations, req.Method+" "+req.URL.Path)
		}
	}
	if len(mutations) != 0 {
		t.Fatalf("expected no mutating requests, got %v", mutations)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

// TestMigrateImportAcceptsVersionOwnedByApp proves the verified path still
// reaches the version-localization mutation.
func TestMigrateImportAcceptsVersionOwnedByApp(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	fastlaneDir := writeMigrateOwnershipFixture(t)

	reg := asctest.NewRegistry(t)
	reg.Install()
	versionA := asctest.JSON(http.StatusOK, `{"data":{"type":"appStoreVersions","id":"VERSION_A","attributes":{"versionString":"1.0","platform":"IOS"},"relationships":{"app":{"data":{"type":"apps","id":"APP_A"}}}}}`)
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_A", "include=app"), versionA)
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_A/appStoreVersionLocalizations", "limit=200"),
		asctest.JSON(http.StatusOK, `{"data":[],"links":{"next":""}}`))
	reg.Register(asctest.GET("/v1/appStoreVersions/VERSION_A", "include=app"), versionA)
	reg.Register(asctest.GET("/v1/apps/APP_A/appStoreVersions", "filter[appStoreState]=READY_FOR_SALE,DEVELOPER_REMOVED_FROM_SALE,REMOVED_FROM_SALE", "filter[platform]=IOS", "limit=1"),
		func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.String())
		})
	reg.Register(asctest.POST("/v1/appStoreVersionLocalizations").JSONBody(`{"data":{"type":"appStoreVersionLocalizations","attributes":{"locale":"en-US","description":"English description"},"relationships":{"appStoreVersion":{"data":{"type":"appStoreVersions","id":"VERSION_A"}}}}}`),
		asctest.JSON(http.StatusCreated, `{"data":{"type":"appStoreVersionLocalizations","id":"loc-1","attributes":{"locale":"en-US","description":"English description"}}}`))

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_A",
			"--version-id", "VERSION_A",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := rootCmd.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	ownershipChecked := metadataRequestCount(reg, http.MethodGet, "/v1/appStoreVersions/VERSION_A") > 0
	if !ownershipChecked {
		t.Fatal("expected migrate import to resolve the explicit version through App Store Connect")
	}
	createCount := metadataRequestCount(reg, http.MethodPost, "/v1/appStoreVersionLocalizations")
	if createCount != 1 {
		t.Fatalf("expected one localization create, got %d", createCount)
	}
}
