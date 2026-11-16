package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveImportInputs_ExplicitDirFlagsOverrideDeliverfilePaths(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "screenshots"), 0o755); err != nil {
		t.Fatalf("mkdir screenshots: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "custom_metadata"), 0o755); err != nil {
		t.Fatalf("mkdir custom_metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "custom_screens"), 0o755); err != nil {
		t.Fatalf("mkdir custom_screens: %v", err)
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
		WorkDir:        root,
		FastlaneDir:    fastlaneDir,
		MetadataDir:    filepath.Join(fastlaneDir, "metadata"),
		ScreenshotsDir: filepath.Join(fastlaneDir, "screenshots"),
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}

	if inputs.MetadataDir != filepath.Join(fastlaneDir, "metadata") {
		t.Fatalf("expected metadata dir to use explicit flag, got %q", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(fastlaneDir, "screenshots") {
		t.Fatalf("expected screenshots dir to use explicit flag, got %q", inputs.ScreenshotsDir)
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

func TestResolveImportInputs_FastlaneDirHonorsDeliverfilePaths(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	for _, dir := range []string{"metadata", "screenshots", "metadata_prod", "screens_prod"} {
		if err := os.MkdirAll(filepath.Join(fastlaneDir, dir), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	deliverfile := filepath.Join(fastlaneDir, "Deliverfile")
	content := `
		metadata_path "./metadata_prod"
		screenshots_path "./screens_prod"
	`
	if err := os.WriteFile(deliverfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	inputs, skipped, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}

	if inputs.MetadataDir != filepath.Join(fastlaneDir, "metadata_prod") {
		t.Fatalf("MetadataDir = %q, want Deliverfile metadata_path", inputs.MetadataDir)
	}
	if inputs.ScreenshotsDir != filepath.Join(fastlaneDir, "screens_prod") {
		t.Fatalf("ScreenshotsDir = %q, want Deliverfile screenshots_path", inputs.ScreenshotsDir)
	}
	if inputs.MetadataSource != pathSourceDeliverfile {
		t.Fatalf("MetadataSource = %q, want deliverfile", inputs.MetadataSource)
	}
	if inputs.ScreenshotsSource != pathSourceDeliverfile {
		t.Fatalf("ScreenshotsSource = %q, want deliverfile", inputs.ScreenshotsSource)
	}

	wantNotes := map[string]string{
		filepath.Join(fastlaneDir, "metadata"):    `unused because Deliverfile metadata_path "./metadata_prod" selects another directory`,
		filepath.Join(fastlaneDir, "screenshots"): `unused because Deliverfile screenshots_path "./screens_prod" selects another directory`,
	}
	for path, reason := range wantNotes {
		found := false
		for _, item := range skipped {
			if item.Path == path && item.Reason == reason {
				found = true
			}
		}
		if !found {
			t.Fatalf("skipped = %+v, want entry {%q, %q}", skipped, path, reason)
		}
	}
}

func TestResolveImportInputs_FastlaneDirRejectsMissingDeliverfileMetadataPath(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}

	deliverfile := filepath.Join(fastlaneDir, "Deliverfile")
	content := "metadata_path \"./metadata_prod\"\nskip_screenshots true\n"
	if err := os.WriteFile(deliverfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	_, _, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err == nil {
		t.Fatal("resolveImportInputs() error = nil, want missing Deliverfile metadata_path rejection")
	}
	if !strings.Contains(err.Error(), `deliverfile metadata_path "./metadata_prod"`) {
		t.Fatalf("resolveImportInputs() error = %v, want it to name the Deliverfile metadata_path", err)
	}
}

// TestResolveImportInputs_HintsAtProjectRootRelativeDeliverfilePaths covers the
// most common fastlane layout collision: deliver resolves metadata_path from
// the project root, so a Deliverfile inside fastlane/ often carries
// "./fastlane/metadata", which resolves one level too deep here.
func TestResolveImportInputs_HintsAtProjectRootRelativeDeliverfilePaths(t *testing.T) {
	t.Run("metadata", func(t *testing.T) {
		root := t.TempDir()
		fastlaneDir := filepath.Join(root, "fastlane")
		if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
			t.Fatalf("mkdir metadata: %v", err)
		}
		writeTestDeliverfile(t, fastlaneDir, "metadata_path \"./fastlane/metadata\"\nskip_screenshots true\n")

		_, _, err := resolveImportInputs(importInputOptions{
			WorkDir:     root,
			FastlaneDir: fastlaneDir,
		})
		if err == nil {
			t.Fatal("resolveImportInputs() error = nil, want the missing Deliverfile metadata_path rejection")
		}
		wantHint := fmt.Sprintf(
			"paths in a Deliverfile resolve relative to the Deliverfile; %s exists, so a project-root-relative value like \"./fastlane/metadata\" should be \"./metadata\"",
			filepath.Join(fastlaneDir, "metadata"),
		)
		if !strings.Contains(err.Error(), wantHint) {
			t.Fatalf("resolveImportInputs() error = %v, want the hint %q", err, wantHint)
		}
	})

	t.Run("screenshots", func(t *testing.T) {
		root := t.TempDir()
		fastlaneDir := filepath.Join(root, "fastlane")
		for _, dir := range []string{"metadata", "screenshots"} {
			if err := os.MkdirAll(filepath.Join(fastlaneDir, dir), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dir, err)
			}
		}
		writeTestDeliverfile(t, fastlaneDir, "screenshots_path \"./fastlane/screenshots\"\n")

		_, _, err := resolveImportInputs(importInputOptions{
			WorkDir:     root,
			FastlaneDir: fastlaneDir,
		})
		if err == nil {
			t.Fatal("resolveImportInputs() error = nil, want the missing Deliverfile screenshots_path rejection")
		}
		wantHint := fmt.Sprintf(
			"paths in a Deliverfile resolve relative to the Deliverfile; %s exists, so a project-root-relative value like \"./fastlane/screenshots\" should be \"./screenshots\"",
			filepath.Join(fastlaneDir, "screenshots"),
		)
		if !strings.Contains(err.Error(), wantHint) {
			t.Fatalf("resolveImportInputs() error = %v, want the hint %q", err, wantHint)
		}
	})

	t.Run("no conventional directory", func(t *testing.T) {
		root := t.TempDir()
		fastlaneDir := filepath.Join(root, "fastlane")
		if err := os.MkdirAll(fastlaneDir, 0o755); err != nil {
			t.Fatalf("mkdir fastlane: %v", err)
		}
		writeTestDeliverfile(t, fastlaneDir, "metadata_path \"./fastlane/metadata\"\nskip_screenshots true\n")

		_, _, err := resolveImportInputs(importInputOptions{
			WorkDir:     root,
			FastlaneDir: fastlaneDir,
		})
		if err == nil {
			t.Fatal("resolveImportInputs() error = nil, want the missing Deliverfile metadata_path rejection")
		}
		if !strings.Contains(err.Error(), `deliverfile metadata_path "./fastlane/metadata"`) {
			t.Fatalf("resolveImportInputs() error = %v, want it to name the Deliverfile metadata_path", err)
		}
		if strings.Contains(err.Error(), "project-root-relative") {
			t.Fatalf("resolveImportInputs() error = %v, want no hint when the conventional directory is absent too", err)
		}
	})
}

func writeTestDeliverfile(t *testing.T, fastlaneDir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fastlaneDir, "Deliverfile"), []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}
}

func TestResolveImportInputs_FastlaneDirRejectsEscapingDeliverfileMetadataPath(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "outside"), 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}

	deliverfile := filepath.Join(fastlaneDir, "Deliverfile")
	content := "metadata_path \"../outside\"\nskip_screenshots true\n"
	if err := os.WriteFile(deliverfile, []byte(content), 0o644); err != nil {
		t.Fatalf("write deliverfile: %v", err)
	}

	_, _, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err == nil {
		t.Fatal("resolveImportInputs() error = nil, want containment rejection")
	}
	if !strings.Contains(err.Error(), "escapes trusted root") {
		t.Fatalf("resolveImportInputs() error = %v, want containment rejection", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:               root,
		FastlaneDir:           fastlaneDir,
		AllowExternalMetadata: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want explicitly trusted path accepted", err)
	}
	expected, err := filepath.EvalSymlinks(filepath.Join(root, "outside"))
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if inputs.MetadataDir != expected {
		t.Fatalf("MetadataDir = %q, want %q", inputs.MetadataDir, expected)
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

func TestResolveImportInputsPreservesInvalidDeliverfileScreenshotsPathWhenFlagSkips(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(fastlaneDir, "Deliverfile"),
		[]byte("screenshots_path \"../outside\"\n"),
		0o600,
	); err != nil {
		t.Fatalf("write Deliverfile: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:         root,
		FastlaneDir:     fastlaneDir,
		SkipScreenshots: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want skipped path preserved without validation", err)
	}
	want := filepath.Join(root, "outside")
	if inputs.ScreenshotsDir != want {
		t.Fatalf("ScreenshotsDir = %q, want skipped Deliverfile path %q", inputs.ScreenshotsDir, want)
	}
	if inputs.ScreenshotsSource != pathSourceDeliverfile {
		t.Fatalf("ScreenshotsSource = %q, want %q", inputs.ScreenshotsSource, pathSourceDeliverfile)
	}
}

func TestResolveImportInputsPreservesSymlinkedScreenshotsPathWhenDeliverfileSkips(t *testing.T) {
	root := t.TempDir()
	fastlaneDir := filepath.Join(root, "fastlane")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	external := t.TempDir()
	screenshotsPath := filepath.Join(fastlaneDir, "screenshots")
	if err := os.Symlink(external, screenshotsPath); err != nil {
		t.Fatalf("symlink screenshots: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(fastlaneDir, "Deliverfile"),
		[]byte("skip_screenshots true\n"),
		0o600,
	); err != nil {
		t.Fatalf("write Deliverfile: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:     root,
		FastlaneDir: fastlaneDir,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want skipped symlink path preserved without validation", err)
	}
	if inputs.ScreenshotsDir != screenshotsPath {
		t.Fatalf("ScreenshotsDir = %q, want skipped symlink path %q", inputs.ScreenshotsDir, screenshotsPath)
	}
	if !inputs.DeliverfileConfig.SkipScreenshots {
		t.Fatalf("DeliverfileConfig = %+v, want skip_screenshots", inputs.DeliverfileConfig)
	}
}

func TestResolveImportInputsPreservesWhitespaceInFastlanePath(t *testing.T) {
	base := t.TempDir()
	fastlaneDir := filepath.Join(base, " fastlane ")
	if err := os.MkdirAll(filepath.Join(fastlaneDir, "metadata"), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:         base,
		FastlaneDir:     fastlaneDir,
		SkipScreenshots: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if inputs.MetadataDir != filepath.Join(fastlaneDir, "metadata") {
		t.Fatalf("MetadataDir = %q, want exact whitespace path %q", inputs.MetadataDir, filepath.Join(fastlaneDir, "metadata"))
	}
}

func TestResolveImportInputsPreservesWhitespaceInDeliverfilePathValue(t *testing.T) {
	workDir := t.TempDir()
	metadataName := " metadata "
	if err := os.MkdirAll(filepath.Join(workDir, metadataName), 0o755); err != nil {
		t.Fatalf("mkdir metadata: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(workDir, "Deliverfile"),
		[]byte("metadata_path \""+metadataName+"\"\nskip_screenshots true\n"),
		0o600,
	); err != nil {
		t.Fatalf("write Deliverfile: %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{WorkDir: workDir})
	if err != nil {
		t.Fatalf("resolveImportInputs() error: %v", err)
	}
	if inputs.MetadataDir != filepath.Join(workDir, metadataName) {
		t.Fatalf("MetadataDir = %q, want exact whitespace path %q", inputs.MetadataDir, filepath.Join(workDir, metadataName))
	}
}

func TestNewMigrateContentRootAnchorsAtDepsWorkingDirectory(t *testing.T) {
	processDir, depsDir := t.TempDir(), t.TempDir()
	t.Chdir(processDir)
	if err := os.MkdirAll(filepath.Join(depsDir, "fastlane", "metadata"), 0o755); err != nil {
		t.Fatalf("create metadata directory: %v", err)
	}
	d := migrateTestDeps()
	d.Getwd = func() (string, error) { return depsDir, nil }

	root, prefix, err := newMigrateContentRoot(d, filepath.Join("fastlane", "metadata"))
	if err != nil {
		t.Fatalf("newMigrateContentRoot() error = %v", err)
	}
	defer root.Close()
	wantRoot, err := filepath.EvalSymlinks(depsDir)
	if err != nil {
		t.Fatalf("resolve deps directory: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(root.Path())
	if err != nil {
		t.Fatalf("resolve root: %v", err)
	}
	if gotRoot != wantRoot || prefix != filepath.Join("fastlane", "metadata") {
		t.Fatalf("newMigrateContentRoot() = (%q, %q), want (%q, %q)", gotRoot, prefix, wantRoot, filepath.Join("fastlane", "metadata"))
	}
}
