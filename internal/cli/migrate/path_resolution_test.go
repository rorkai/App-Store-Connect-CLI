package migrate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveImportInputs_ExplicitFastlaneDirOverridesDeliverfilePaths(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}

	deliverfile := filepath.Join(fastlaneDir, "Deliverfile")
	content := `
		metadata_path "./custom_metadata"
		screenshots_path "./custom_screens"
	`
	if err := os.WriteFile(deliverfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}

	if inputs.MetadataDir != filepath.Join(fastlaneDir, "metadata") {
		t.Fatalf("expected metadata dir to use fastlane dir, got %q", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(fastlaneDir, "screenshots") {
		t.Fatalf("expected screenshots dir to use fastlane dir, got %q", inputs.ScreenshotsDir)
	}
	if inputs.MetadataSource != pathSourceFlag {
		t.Fatalf("expected metadata source flag, got %q", inputs.MetadataSource)
	}
	if inputs.ScreenshotsSource != pathSourceFlag {
		t.Fatalf("expected screenshots source flag, got %q", inputs.ScreenshotsSource)
	}
	if inputs.DeliverfilePath == "" {
		t.Fatal("expected deliverfile path to be discovered")
	}
}

func TestResolveImportInputs_UsesDeliverfilePathsWhenPresent(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "meta"), 0o755); err != nil {
		t.Fatalf("mkdir meta: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "shots"), 0o755); err != nil {
		t.Fatalf("mkdir shots: %v", err)
	}

	deliverfile := filepath.Join(root, "Deliverfile")
	content := `
		metadata_path "./meta"
		screenshots_path "./shots"
	`
	if err := os.WriteFile(deliverfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir: root,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}

	if inputs.MetadataDir != filepath.Join(root, "meta") {
		t.Fatalf("expected metadata dir to use deliverfile path, got %q", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(root, "shots") {
		t.Fatalf("expected screenshots dir to use deliverfile path, got %q", inputs.ScreenshotsDir)
	}
	if inputs.MetadataSource != pathSourceDeliverfile {
		t.Fatalf("expected metadata source deliverfile, got %q", inputs.MetadataSource)
	}
	if inputs.ScreenshotsSource != pathSourceDeliverfile {
		t.Fatalf("expected screenshots source deliverfile, got %q", inputs.ScreenshotsSource)
	}
}

func TestResolveImportInputs_FallsBackToDefaultDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}

	inputs, skipped, err := resolveImportInputs(importInputOptions{
		WorkDir: root,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected no skipped items, got %d", len(skipped))
	}
	if inputs.MetadataDir != filepath.Join(root, "metadata") {
		t.Fatalf("expected default metadata dir, got %q", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(root, "screenshots") {
		t.Fatalf("expected default screenshots dir, got %q", inputs.ScreenshotsDir)
	}
	if inputs.MetadataSource != pathSourceDefault {
		t.Fatalf("expected metadata source default, got %q", inputs.MetadataSource)
	}
	if inputs.ScreenshotsSource != pathSourceDefault {
		t.Fatalf("expected screenshots source default, got %q", inputs.ScreenshotsSource)
	}
}

func TestResolveImportInputs_SkipsMissingDefaultDirectories(t *testing.T) {
	root := t.TempDir()

	inputs, skipped, err := resolveImportInputs(importInputOptions{
		WorkDir: root,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if inputs.MetadataDir != "" {
		t.Fatalf("expected empty metadata dir, got %q", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != "" {
		t.Fatalf("expected empty screenshots dir, got %q", inputs.ScreenshotsDir)
	}
	if len(skipped) != 2 {
		t.Fatalf("expected 2 skipped entries, got %d", len(skipped))
	}
}

func TestResolveImportInputs_AllowsMissingFastlaneScreenshotsWhenSkipped(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}

	inputs, skipped, err := resolveImportInputs(importInputOptions{
		WorkDir:         root,
		FastlaneDir:     fastlaneDir,
		SkipScreenshots: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected no skipped entries during path resolution, got %+v", skipped)
	}
	if inputs.MetadataDir != filepath.Join(fastlaneDir, "metadata") {
		t.Fatalf("expected metadata dir %q, got %q", filepath.Join(fastlaneDir, "metadata"), inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(fastlaneDir, "screenshots") {
		t.Fatalf("expected screenshots dir %q, got %q", filepath.Join(fastlaneDir, "screenshots"), inputs.ScreenshotsDir)
	}
}

func TestResolveImportInputs_AllowsMissingFastlaneScreenshotsWhenDeliverfileSkips(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	deliverfile := filepath.Join(fastlaneDir, "Deliverfile")
	if err := os.WriteFile(deliverfile, []byte("skip_screenshots true\n"), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	inputs, skipped, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected no skipped entries during path resolution, got %+v", skipped)
	}
	if !inputs.DeliverfileConfig.SkipScreenshots {
		t.Fatalf("expected deliverfile skip_screenshots to be true, got %+v", inputs.DeliverfileConfig)
	}
	if inputs.ScreenshotsDir != filepath.Join(fastlaneDir, "screenshots") {
		t.Fatalf("expected screenshots dir %q, got %q", filepath.Join(fastlaneDir, "screenshots"), inputs.ScreenshotsDir)
	}
}
