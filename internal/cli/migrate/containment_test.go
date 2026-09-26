package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFastlaneMetadataRefusesSymlinkedVersionMetadataFile(t *testing.T) {
	metadataDir := t.TempDir()
	localeDir := filepath.Join(metadataDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	secretPath := filepath.Join(t.TempDir(), "id_rsa")
	writeMigrateContainmentFile(t, secretPath, "PRIVATE KEY MATERIAL")

	if err := os.Symlink(secretPath, filepath.Join(localeDir, "description.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	locs, err := readFastlaneMetadata(metadataDir)
	if err == nil {
		t.Fatalf("readFastlaneMetadata() error = nil, want symlink rejection (got %#v)", locs)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("readFastlaneMetadata() error = %v, want symlink rejection", err)
	}
	if strings.Contains(err.Error(), "PRIVATE KEY MATERIAL") {
		t.Fatalf("error leaked file contents: %v", err)
	}
}

func TestReadFastlaneAppInfoMetadataRefusesSymlinkedFile(t *testing.T) {
	metadataDir := t.TempDir()
	localeDir := filepath.Join(metadataDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	secretPath := filepath.Join(t.TempDir(), "secret.txt")
	writeMigrateContainmentFile(t, secretPath, "local secret")

	if err := os.Symlink(secretPath, filepath.Join(localeDir, "name.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	locs, err := readFastlaneAppInfoMetadata(metadataDir)
	if err == nil {
		t.Fatalf("readFastlaneAppInfoMetadata() error = nil, want symlink rejection (got %#v)", locs)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("readFastlaneAppInfoMetadata() error = %v, want symlink rejection", err)
	}
}

func TestReadFastlaneReviewInformationRefusesSymlinkedFile(t *testing.T) {
	metadataDir := t.TempDir()
	reviewDir := filepath.Join(metadataDir, "review_information")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	secretPath := filepath.Join(t.TempDir(), "secret.txt")
	writeMigrateContainmentFile(t, secretPath, "local secret")

	if err := os.Symlink(secretPath, filepath.Join(reviewDir, "notes.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	info, err := readFastlaneReviewInformation(metadataDir)
	if err == nil {
		t.Fatalf("readFastlaneReviewInformation() error = nil, want symlink rejection (got %#v)", info)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("readFastlaneReviewInformation() error = %v, want symlink rejection", err)
	}
}

func TestScanFastlaneMetadataLocaleDirsReportsSymlinkedLocaleDirectory(t *testing.T) {
	metadataDir := t.TempDir()
	external := t.TempDir()
	writeMigrateContainmentFile(t, filepath.Join(external, "description.txt"), "external")

	if err := os.Symlink(external, filepath.Join(metadataDir, "en-US")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	dirs, skipped, err := scanFastlaneMetadataLocaleDirs(metadataDir)
	if err != nil {
		t.Fatalf("scanFastlaneMetadataLocaleDirs() error = %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("scanFastlaneMetadataLocaleDirs() dirs = %#v, want none", dirs)
	}
	found := false
	for _, item := range skipped {
		if strings.Contains(item.Reason, "symlink") {
			found = true
		}
	}
	if !found {
		t.Fatalf("scanFastlaneMetadataLocaleDirs() skipped = %#v, want a symlink reason", skipped)
	}
}

func TestReadFastlaneMetadataReadsOrdinaryFiles(t *testing.T) {
	metadataDir := t.TempDir()
	localeDir := filepath.Join(metadataDir, "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeMigrateContainmentFile(t, filepath.Join(localeDir, "description.txt"), "English description\n")
	writeMigrateContainmentFile(t, filepath.Join(localeDir, "name.txt"), "App Name\n")

	locs, err := readFastlaneMetadata(metadataDir)
	if err != nil {
		t.Fatalf("readFastlaneMetadata() error = %v", err)
	}
	if len(locs) != 1 || locs[0].Description != "English description" {
		t.Fatalf("readFastlaneMetadata() = %#v", locs)
	}

	appInfo, err := readFastlaneAppInfoMetadata(metadataDir)
	if err != nil {
		t.Fatalf("readFastlaneAppInfoMetadata() error = %v", err)
	}
	if len(appInfo) != 1 || appInfo[0].Name != "App Name" {
		t.Fatalf("readFastlaneAppInfoMetadata() = %#v", appInfo)
	}
}

func TestMigrateExportRefusesSymlinkedLocaleDirectory(t *testing.T) {
	outputDir := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outputDir, "metadata"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(outputDir, "metadata", "en-US")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root, err := newMigrateExportRoot(outputDir)
	if err != nil {
		t.Fatalf("newMigrateExportRoot() error = %v", err)
	}
	err = root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755)
	if err == nil {
		t.Fatal("MkdirAll() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("MkdirAll() error = %v, want symlink rejection", err)
	}
	entries, readErr := os.ReadDir(external)
	if readErr != nil {
		t.Fatalf("ReadDir() error = %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("export escaped into the symlink target: %v", entries)
	}
}

func TestMigrateExportRefusesSymlinkedDestinationFile(t *testing.T) {
	outputDir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	writeMigrateContainmentFile(t, sentinelPath, "original")

	localeDir := filepath.Join(outputDir, "metadata", "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(sentinelPath, filepath.Join(localeDir, "description.txt")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root, err := newMigrateExportRoot(outputDir)
	if err != nil {
		t.Fatalf("newMigrateExportRoot() error = %v", err)
	}
	written, err := writeAndCount(root, filepath.Join("metadata", "en-US", "description.txt"), "attacker content")
	if err == nil {
		t.Fatal("writeAndCount() error = nil, want symlink rejection")
	}
	if written != 0 {
		t.Fatalf("writeAndCount() = %d, want 0", written)
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("writeAndCount() error = %v, want symlink rejection", err)
	}
	if got := readMigrateContainmentFile(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestMigrateExportEmptyContentRefusesSymlinkedDestinationFile(t *testing.T) {
	outputDir := t.TempDir()
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.txt")
	writeMigrateContainmentFile(t, sentinelPath, "original")

	localeDir := filepath.Join(outputDir, "metadata", "en-US")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	destination := filepath.Join(localeDir, "description.txt")
	if err := os.Symlink(sentinelPath, destination); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	root, err := newMigrateExportRoot(outputDir)
	if err != nil {
		t.Fatalf("newMigrateExportRoot() error = %v", err)
	}
	defer root.Close()
	written, err := writeAndCount(root, filepath.Join("metadata", "en-US", "description.txt"), "")
	if err == nil {
		t.Fatal("writeAndCount() error = nil, want symlink rejection")
	}
	if written != 0 {
		t.Fatalf("writeAndCount() = %d, want 0", written)
	}
	if _, err := os.Lstat(destination); err != nil {
		t.Fatalf("destination after refusal: %v", err)
	}
	if got := readMigrateContainmentFile(t, sentinelPath); got != "original" {
		t.Fatalf("sentinel content = %q, want %q", got, "original")
	}
}

func TestMigrateExportRejectsTraversingLocale(t *testing.T) {
	outputDir := t.TempDir()
	root, err := newMigrateExportRoot(outputDir)
	if err != nil {
		t.Fatalf("newMigrateExportRoot() error = %v", err)
	}

	if err := root.MkdirAll(filepath.Join("metadata", "../../escape"), 0o755); err == nil {
		t.Fatal("MkdirAll() error = nil, want traversal rejection")
	}
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(outputDir), "escape")); statErr == nil {
		t.Fatal("MkdirAll() created a directory outside the output root")
	}
}

func TestMigrateExportWritesOrdinaryFiles(t *testing.T) {
	outputDir := t.TempDir()
	root, err := newMigrateExportRoot(outputDir)
	if err != nil {
		t.Fatalf("newMigrateExportRoot() error = %v", err)
	}
	if err := root.MkdirAll(filepath.Join("metadata", "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	written, err := writeAndCount(root, filepath.Join("metadata", "en-US", "description.txt"), "English description")
	if err != nil {
		t.Fatalf("writeAndCount() error = %v", err)
	}
	if written != 1 {
		t.Fatalf("writeAndCount() = %d, want 1", written)
	}
	if got := readMigrateContainmentFile(t, filepath.Join(outputDir, "metadata", "en-US", "description.txt")); got != "English description\n" {
		t.Fatalf("content = %q", got)
	}

	skipped, err := writeAndCount(root, filepath.Join("metadata", "en-US", "keywords.txt"), "")
	if err != nil {
		t.Fatalf("writeAndCount() empty error = %v", err)
	}
	if skipped != 0 {
		t.Fatalf("writeAndCount() empty = %d, want 0", skipped)
	}
}

func TestResolveImportInputsRejectsAbsoluteDeliverfileMetadataPath(t *testing.T) {
	workDir := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	deliverfile := "metadata_path \"" + external + "\"\nskip_screenshots true\n"
	writeMigrateContainmentFile(t, filepath.Join(workDir, "Deliverfile"), deliverfile)

	_, _, err := resolveImportInputs(importInputOptions{WorkDir: workDir})
	if err == nil {
		t.Fatal("resolveImportInputs() error = nil, want rejection of absolute Deliverfile metadata_path")
	}
	if !strings.Contains(err.Error(), "must be relative") && !strings.Contains(err.Error(), "escapes trusted root") {
		t.Fatalf("resolveImportInputs() error = %v, want containment rejection", err)
	}
}

func TestResolveImportInputsRejectsTraversingDeliverfileMetadataPath(t *testing.T) {
	base := t.TempDir()
	workDir := filepath.Join(base, "checkout")
	external := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	deliverfile := "metadata_path \"../outside\"\nskip_screenshots true\n"
	writeMigrateContainmentFile(t, filepath.Join(workDir, "Deliverfile"), deliverfile)

	_, _, err := resolveImportInputs(importInputOptions{WorkDir: workDir})
	if err == nil {
		t.Fatal("resolveImportInputs() error = nil, want rejection of traversing Deliverfile metadata_path")
	}
	if !strings.Contains(err.Error(), "escapes trusted root") {
		t.Fatalf("resolveImportInputs() error = %v, want containment rejection", err)
	}
}

func TestResolveImportInputsAllowsExternalDeliverfileMetadataPathWithExplicitTrust(t *testing.T) {
	workDir := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	deliverfile := "metadata_path \"" + external + "\"\nskip_screenshots true\n"
	writeMigrateContainmentFile(t, filepath.Join(workDir, "Deliverfile"), deliverfile)

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:               workDir,
		AllowExternalMetadata: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want explicitly trusted path accepted", err)
	}
	expected, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if inputs.MetadataDir != expected {
		t.Fatalf("MetadataDir = %q, want %q", inputs.MetadataDir, expected)
	}
}

func TestResolveImportInputsRequiresTrustForSymlinkedFastlaneMetadata(t *testing.T) {
	fastlane := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(fastlane, "metadata")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, _, err := resolveImportInputs(importInputOptions{
		FastlaneDir:     fastlane,
		SkipScreenshots: true,
	}); err == nil {
		t.Fatal("resolveImportInputs() error = nil, want default symlink rejection")
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		FastlaneDir:           fastlane,
		SkipScreenshots:       true,
		AllowExternalMetadata: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want explicitly trusted symlink accepted", err)
	}
	expected, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if inputs.MetadataDir != expected {
		t.Fatalf("MetadataDir = %q, want resolved trusted target %q", inputs.MetadataDir, expected)
	}
}

func TestResolveImportInputsAllowsSymlinkedFastlaneScreenshotsWithExplicitTrust(t *testing.T) {
	fastlane := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fastlane, "metadata"), 0o755); err != nil {
		t.Fatalf("MkdirAll(metadata) error = %v", err)
	}
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll(screenshots) error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(fastlane, "screenshots")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		FastlaneDir:              fastlane,
		AllowExternalScreenshots: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want explicitly trusted screenshot symlink accepted", err)
	}
	expected, err := filepath.EvalSymlinks(external)
	if err != nil {
		t.Fatalf("EvalSymlinks() error = %v", err)
	}
	if inputs.ScreenshotsDir != expected {
		t.Fatalf("ScreenshotsDir = %q, want resolved trusted target %q", inputs.ScreenshotsDir, expected)
	}
}

func TestResolveImportInputsRejectsSymlinkedDeliverfileByDefault(t *testing.T) {
	workDir := t.TempDir()
	external := filepath.Join(t.TempDir(), "Deliverfile")
	writeMigrateContainmentFile(t, external, "skip_metadata true\nskip_screenshots true\n")
	if err := os.Symlink(external, filepath.Join(workDir, "Deliverfile")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, _, err := resolveImportInputs(importInputOptions{WorkDir: workDir}); err == nil {
		t.Fatal("resolveImportInputs() error = nil, want symlinked Deliverfile rejected")
	}

	inputs, _, err := resolveImportInputs(importInputOptions{
		WorkDir:                   workDir,
		AllowSymlinkedDeliverfile: true,
	})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want explicitly trusted Deliverfile accepted", err)
	}
	if !inputs.DeliverfileConfig.SkipMetadata || !inputs.DeliverfileConfig.SkipScreenshots {
		t.Fatalf("DeliverfileConfig = %+v, want parsed trusted file", inputs.DeliverfileConfig)
	}
}

func TestScanFastlaneMetadataLocaleDirsRefusesSymlinkedInTreeMetadataDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeMigrateContainmentFile(t, filepath.Join(external, "en-US", "description.txt"), "local secret")

	if err := os.MkdirAll(filepath.Join(workDir, "fastlane"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(workDir, "fastlane", "metadata")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	// The default fastlane/metadata layout ships with the checkout, so a
	// symlinked metadata directory must be refused, not silently followed.
	_, _, err = scanFastlaneMetadataLocaleDirs(filepath.Join(workDir, "fastlane", "metadata"))
	if err == nil {
		t.Fatal("scanFastlaneMetadataLocaleDirs() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("scanFastlaneMetadataLocaleDirs() error = %v, want symlink rejection", err)
	}
}

func TestResolveImportInputsResolvesDeliverfilePathsFromRelativeWorkDir(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	metadataDir := filepath.Join(cwd, "checkout", "fastlane", "metadata")
	if err := os.MkdirAll(filepath.Join(metadataDir, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	deliverfile := "metadata_path \"./fastlane/metadata\"\nskip_screenshots true\n"
	writeMigrateContainmentFile(t, filepath.Join(cwd, "checkout", "Deliverfile"), deliverfile)

	inputs, _, err := resolveImportInputs(importInputOptions{WorkDir: "checkout"})
	if err != nil {
		t.Fatalf("resolveImportInputs() error = %v, want relative work dir to resolve", err)
	}
	if inputs.MetadataDir != metadataDir {
		t.Fatalf("MetadataDir = %q, want %q", inputs.MetadataDir, metadataDir)
	}
}

func TestDiscoverScreenshotPlanRefusesSymlinkedInTreeScreenshotsDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workDir, "fastlane"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.Symlink(external, filepath.Join(workDir, "fastlane", "screenshots")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	// The default fastlane/screenshots layout ships with the checkout, so a
	// symlinked screenshots directory must be refused before any traversal.
	_, _, err = discoverScreenshotPlan(filepath.Join(workDir, "fastlane", "screenshots"))
	if err == nil {
		t.Fatal("discoverScreenshotPlan() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("discoverScreenshotPlan() error = %v, want symlink rejection", err)
	}
}

func TestResolveImportInputsRefusesSymlinkedExternalFastlaneMetadataChild(t *testing.T) {
	t.Chdir(t.TempDir())

	fastlane := t.TempDir()
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "en-US"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeMigrateContainmentFile(t, filepath.Join(external, "en-US", "description.txt"), "local secret")
	if err := os.Symlink(external, filepath.Join(fastlane, "metadata")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	// The fastlane checkout's contents are repository-controlled even when the
	// operator selected the fastlane root, so a symlinked metadata child must be
	// refused.
	_, _, err := resolveImportInputs(importInputOptions{FastlaneDir: fastlane, SkipScreenshots: true})
	if err == nil {
		t.Fatal("resolveImportInputs() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("resolveImportInputs() error = %v, want symlink rejection", err)
	}
}

func TestDiscoverScreenshotPlanSkipsAndReportsSymlinkedLocaleDirectory(t *testing.T) {
	screenshotsDir := t.TempDir()
	external := t.TempDir()
	// If a symlinked locale directory were followed, this file would surface as
	// an invalid-screenshot error; the entry must instead be skipped and
	// reported, matching the metadata scan.
	writeMigrateContainmentFile(t, filepath.Join(external, "external.png"), "not a real png")
	if err := os.Symlink(external, filepath.Join(screenshotsDir, "en-US")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	plans, skipped, err := discoverScreenshotPlan(screenshotsDir)
	if err != nil {
		t.Fatalf("discoverScreenshotPlan() error = %v, want symlinked locale skipped", err)
	}
	if len(plans) != 0 {
		t.Fatalf("plans = %#v, want none from a symlinked locale", plans)
	}
	found := false
	for _, item := range skipped {
		if strings.Contains(item.Reason, "symlink") {
			found = true
		}
	}
	if !found {
		t.Fatalf("skipped = %#v, want a reported symlinked screenshots entry", skipped)
	}
}

func writeMigrateContainmentFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readMigrateContainmentFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
