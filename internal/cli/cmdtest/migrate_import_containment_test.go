package cmdtest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asctest"
)

func TestMigrateImportRejectsSymlinkedMetadataBeforeAnyRequest(t *testing.T) {
	setupAuth(t)
	t.Setenv("ASC_CONFIG_PATH", filepath.Join(t.TempDir(), "nonexistent.json"))

	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	metadataDir := filepath.Join(fastlaneDir, "metadata", "en-US")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots root: %v", err)
	}

	secretPath := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, secretPath, "local secret that must not be published")
	if err := os.Symlink(secretPath, filepath.Join(metadataDir, "description.txt")); err != nil {
		t.Fatalf("symlink description.txt: %v", err)
	}

	reg := asctest.NewRegistry(t)
	reg.Install()

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)

	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{
			"migrate", "import",
			"--app", "APP_ID",
			"--version-id", "VERSION_ID",
			"--fastlane-dir", fastlaneDir,
			"--confirm",
		}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil {
		t.Fatal("expected migrate import to fail for symlinked metadata")
	}
	if !strings.Contains(runErr.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", runErr)
	}
	if requests := len(reg.Requests()); requests != 0 {
		t.Fatalf("expected no App Store Connect requests, got %d", requests)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
