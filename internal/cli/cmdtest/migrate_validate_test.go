package cmdtest

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/migrate"
)

func TestMigrateValidateReportsUnsupportedCreateLocale(t *testing.T) {
	root := t.TempDir()
	for locale, description := range map[string]string{
		"nl":    "Dutch description",
		"en-US": "English description",
	} {
		localeDir := filepath.Join(root, "metadata", locale)
		if err := os.MkdirAll(localeDir, 0o755); err != nil {
			t.Fatalf("mkdir metadata %s: %v", locale, err)
		}
		writeFile(t, filepath.Join(localeDir, "description.txt"), description)
	}

	result := runMigrateValidate(t, root)

	if result.Valid {
		t.Fatalf("valid = true, want false for a locale migrate import would reject")
	}
	if result.ErrorCount != 1 {
		t.Fatalf("errorCount = %d, want 1", result.ErrorCount)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Locale != "nl" || issue.Field != "locale" {
			continue
		}
		if issue.Severity != "error" {
			t.Fatalf("locale issue severity = %q, want error", issue.Severity)
		}
		if issue.Message != `unsupported locale "nl"; did you mean: nl-NL` {
			t.Fatalf("locale issue message = %q, want the import rejection message", issue.Message)
		}
		found = true
	}
	if !found {
		t.Fatalf("issues = %+v, want a locale issue for \"nl\"", result.Issues)
	}
}

func TestMigrateValidateAcceptsSupportedLocales(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "metadata", "nl-NL")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(localeDir, "description.txt"), "Dutch description")

	result := runMigrateValidate(t, root)

	if !result.Valid || result.ErrorCount != 0 {
		t.Fatalf("result = %+v, want a clean validation", result)
	}
}

// TestMigrateValidateHonorsDeliverfileMetadataPath pins validate to the
// directory migrate import reads. Reading the conventional metadata directory
// here would green-light a tree whose import fails on the selected one.
func TestMigrateValidateHonorsDeliverfileMetadataPath(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")

	conventionalDir := filepath.Join(fastlaneDir, "metadata", "de-DE")
	if err := os.MkdirAll(conventionalDir, 0o755); err != nil {
		t.Fatalf("mkdir conventional metadata: %v", err)
	}
	writeFile(t, filepath.Join(conventionalDir, "description.txt"), "Kurze Beschreibung")

	selectedDir := filepath.Join(fastlaneDir, "localized", "en-US")
	if err := os.MkdirAll(selectedDir, 0o755); err != nil {
		t.Fatalf("mkdir selected metadata: %v", err)
	}
	writeFile(t, filepath.Join(selectedDir, "description.txt"), strings.Repeat("a", 4500))

	writeFile(t, filepath.Join(fastlaneDir, "Deliverfile"), "metadata_path \"./localized\"\n")

	result := runMigrateValidate(t, fastlaneDir)

	if result.Valid {
		t.Fatalf("valid = true, want false for the directory migrate import reads")
	}
	if result.ErrorCount != 1 {
		t.Fatalf("errorCount = %d, want 1 (issues = %+v)", result.ErrorCount, result.Issues)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Field != "description" || issue.Severity != "error" {
			continue
		}
		if issue.Locale != "en-US" {
			t.Fatalf("issue locale = %q, want en-US from the Deliverfile metadata_path", issue.Locale)
		}
		if issue.Length != 4500 || issue.Limit != 4000 {
			t.Fatalf("issue length/limit = %d/%d, want 4500/4000", issue.Length, issue.Limit)
		}
		found = true
	}
	if !found {
		t.Fatalf("issues = %+v, want the description limit error from the Deliverfile directory", result.Issues)
	}
	if len(result.Locales) != 1 || result.Locales[0] != "en-US" {
		t.Fatalf("locales = %v, want only en-US (fastlane/metadata must not be read)", result.Locales)
	}

	wantSkippedPath := filepath.Join(fastlaneDir, "metadata")
	wantSkippedReason := `unused because Deliverfile metadata_path "./localized" selects another directory`
	skippedFound := false
	for _, item := range result.Skipped {
		if item.Path == wantSkippedPath && item.Reason == wantSkippedReason {
			skippedFound = true
		}
	}
	if !skippedFound {
		t.Fatalf("skipped = %+v, want {%q, %q}", result.Skipped, wantSkippedPath, wantSkippedReason)
	}
}

func TestMigrateValidateRejectsSymlinkedCopyright(t *testing.T) {
	root := t.TempDir()
	localeDir := filepath.Join(root, "metadata", "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	writeFile(t, filepath.Join(localeDir, "description.txt"), "English description")
	secretPath := filepath.Join(t.TempDir(), "secret.txt")
	writeFile(t, secretPath, "2026 Example Inc.")
	if err := os.Symlink(secretPath, filepath.Join(root, "metadata", "copyright.txt")); err != nil {
		t.Fatalf("symlink copyright.txt: %v", err)
	}

	rootCmd := RootCommand("1.2.3")
	rootCmd.FlagSet.SetOutput(io.Discard)
	var runErr error
	stdout, _ := captureOutput(t, func() {
		if err := rootCmd.Parse([]string{"migrate", "validate", "--fastlane-dir", root}); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = rootCmd.Run(context.Background())
	})

	if runErr == nil || !strings.HasPrefix(runErr.Error(), "migrate validate: ") || !strings.Contains(runErr.Error(), "symlink") {
		t.Fatalf("run error = %v, want the symlink rejection migrate import reports", runErr)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}

func runMigrateValidate(t *testing.T, fastlaneDir string) migrate.MigrateValidateResult {
	t.Helper()

	stdout, stderr, err := runRootWithDeps(t, nil, "migrate", "validate", "--fastlane-dir", fastlaneDir)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var result migrate.MigrateValidateResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	return result
}

func TestMigrateValidateRejectsMissingMetadataDir(t *testing.T) {
	root := t.TempDir()
	stdout, _, runErr := runRootWithDeps(t, nil, "migrate", "validate", "--fastlane-dir", root)

	want := "migrate validate: metadata directory not found: " + filepath.Join(root, "metadata")
	if runErr == nil || runErr.Error() != want {
		t.Fatalf("run error = %v, want %q", runErr, want)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
}
