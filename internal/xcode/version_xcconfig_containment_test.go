package xcode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// externalXCConfigProject builds an xcconfig-backed project whose base
// configuration reference points at a directory outside the project root.
func externalXCConfigProject(t *testing.T) (projectPath string, externalDir string) {
	t.Helper()

	projectPath = writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(projectPath)
	externalDir = t.TempDir()

	for _, name := range []string{"App.xcconfig", "Shared.xcconfig"} {
		source := filepath.Join(projectRoot, "Configs", name)
		data := mustReadVersionTestFile(t, source)
		if err := os.WriteFile(filepath.Join(externalDir, name), []byte(data), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
		if err := os.Remove(source); err != nil {
			t.Fatalf("Remove(%s) error = %v", name, err)
		}
	}

	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	old := "path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT;"
	if count := strings.Count(contents, old); count != 1 {
		t.Fatalf("expected one App.xcconfig reference, found %d", count)
	}
	contents = strings.Replace(contents, old,
		`path = "`+filepath.Join(externalDir, "App.xcconfig")+`"; sourceTree = "<absolute>";`, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	return projectPath, externalDir
}

func TestSetVersionRefusesExternalXCConfigByDefault(t *testing.T) {
	projectPath, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	before := mustReadVersionTestFile(t, externalPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err == nil {
		t.Fatal("SetVersion() error = nil, want external xcconfig rejection")
	}
	if !strings.Contains(err.Error(), "--allow-external-xcconfig") {
		t.Fatalf("SetVersion() error = %v, want migration guidance naming --allow-external-xcconfig", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatal("external xcconfig was rewritten despite the rejection")
	}
}

func TestValidateSetVersionRefusesExternalXCConfigByDefault(t *testing.T) {
	projectPath, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	before := mustReadVersionTestFile(t, externalPath)

	err := ValidateSetVersion(t.Context(), SetVersionOptions{
		ProjectDir:  projectPath,
		BuildNumber: "1",
	})
	if err == nil {
		t.Fatal("ValidateSetVersion() error = nil, want external xcconfig rejection")
	}
	if !strings.Contains(err.Error(), "--allow-external-xcconfig") {
		t.Fatalf("ValidateSetVersion() error = %v, want migration guidance", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatal("external xcconfig changed during validation")
	}
}

func TestBumpVersionRefusesExternalXCConfigByDefault(t *testing.T) {
	projectPath, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	before := mustReadVersionTestFile(t, externalPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: projectPath,
		BumpType:   BumpPatch,
	})
	if err == nil {
		t.Fatal("BumpVersion() error = nil, want external xcconfig rejection")
	}
	if !strings.Contains(err.Error(), "--allow-external-xcconfig") {
		t.Fatalf("BumpVersion() error = %v, want migration guidance", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatal("external xcconfig was rewritten despite the rejection")
	}
}

func TestSetVersionAllowsExternalXCConfigWhenAuthorized(t *testing.T) {
	projectPath, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:            projectPath,
		Version:               "2.0.0",
		BuildNumber:           "99",
		AllowExternalXCConfig: true,
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) == 0 {
		t.Fatalf("SetVersion() result = %#v, want changed files", result)
	}
	if after := mustReadVersionTestFile(t, externalPath); !strings.Contains(after, "MARKETING_VERSION = 2.0.0") {
		t.Fatalf("external xcconfig content = %q, want authorized update", after)
	}
}

func TestSetVersionReportsSymlinkedXCConfigAsSymlinkRejection(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(projectPath)
	sharedPath := filepath.Join(projectRoot, "Configs", "Shared.xcconfig")

	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	data := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(externalPath, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Remove(sharedPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := os.Symlink(externalPath, sharedPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	before := mustReadVersionTestFile(t, externalPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err == nil {
		t.Fatal("SetVersion() error = nil, want symlinked xcconfig rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("SetVersion() error = %v, want symlink rejection", err)
	}
	if strings.Contains(err.Error(), "outside project directory") {
		t.Fatalf("SetVersion() error = %v, want the symlink-specific message, not the containment one", err)
	}
	// The write path rejects symlinked xcconfig files even when authorized, so
	// the error must not present --allow-external-xcconfig as a remedy.
	if strings.Contains(err.Error(), "--allow-external-xcconfig") {
		t.Fatalf("SetVersion() error = %v, want no --allow-external-xcconfig guidance for symlinks", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatal("symlink target was rewritten despite the rejection")
	}
}

func TestSetVersionAdvisesOverrideForEscapingSymlinkedXCConfigParent(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(projectPath)
	configsDir := filepath.Join(projectRoot, "Configs")
	externalDir := filepath.Join(t.TempDir(), "Configs")
	if err := os.Rename(configsDir, externalDir); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := os.Symlink(externalDir, configsDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err == nil {
		t.Fatal("SetVersion() error = nil, want escaping symlinked parent rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("SetVersion() error = %v, want symlink rejection", err)
	}
	// The write path only refuses a symlinked final component, so the override
	// flag is a supported recovery for a symlinked parent and must be offered.
	if !strings.Contains(err.Error(), "--allow-external-xcconfig") {
		t.Fatalf("SetVersion() error = %v, want --allow-external-xcconfig guidance", err)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:            projectPath,
		Version:               "2.0.0",
		BuildNumber:           "99",
		AllowExternalXCConfig: true,
	})
	if err != nil {
		t.Fatalf("SetVersion() authorized error = %v", err)
	}
	if len(result.ChangedFiles) == 0 {
		t.Fatalf("SetVersion() result = %#v, want changed files", result)
	}
	if after := mustReadVersionTestFile(t, filepath.Join(externalDir, "Shared.xcconfig")); !strings.Contains(after, "MARKETING_VERSION = 2.0.0") {
		t.Fatalf("xcconfig content = %q, want authorized update through the symlinked parent", after)
	}
}

func TestSetVersionStillEditsXCConfigInsideProjectRoot(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(projectPath), "Configs", "Shared.xcconfig")

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) == 0 {
		t.Fatalf("SetVersion() result = %#v, want changed files", result)
	}
	if after := mustReadVersionTestFile(t, sharedPath); !strings.Contains(after, "MARKETING_VERSION = 2.0.0") {
		t.Fatalf("in-root xcconfig content = %q, want update", after)
	}
}

func TestSetVersionRefusesXCConfigParentSwapAfterValidation(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(projectPath)
	configsDir := filepath.Join(projectRoot, "Configs")
	sharedPath := filepath.Join(configsDir, "Shared.xcconfig")
	originalConfigsDir := filepath.Join(projectRoot, "Configs-original")
	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	externalBefore := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(externalPath, []byte(externalBefore), 0o640); err != nil {
		t.Fatalf("WriteFile(external Shared.xcconfig) error = %v", err)
	}

	swapped := false
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
		if write.path == sharedPath && !swapped {
			swapped = true
			if err := os.Rename(configsDir, originalConfigsDir); err != nil {
				return nil, err
			}
			if err := os.Symlink(externalDir, configsDir); err != nil {
				return nil, err
			}
		}
		return atomicWritePreparedVersionFileInfo(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err == nil {
		t.Fatal("SetVersion() error = nil, want swapped xcconfig parent refusal")
	}
	if !swapped {
		t.Fatal("test did not swap the xcconfig parent")
	}
	if after := mustReadVersionTestFile(t, externalPath); after != externalBefore {
		t.Fatalf("external xcconfig changed through swapped parent: %q", after)
	}
	if after := mustReadVersionTestFile(t, filepath.Join(originalConfigsDir, "Shared.xcconfig")); after != externalBefore {
		t.Fatalf("original in-root xcconfig changed after parent swap: %q", after)
	}
}

func TestSetVersionRefusesXcodeprojParentSwapAfterValidation(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(projectPath)
	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	originalProjectPath := filepath.Join(projectRoot, "Demo-original.xcodeproj")
	externalRoot := t.TempDir()
	externalProjectPath := filepath.Join(externalRoot, "Demo.xcodeproj")
	if err := os.MkdirAll(externalProjectPath, 0o755); err != nil {
		t.Fatal(err)
	}
	externalPBXProjPath := filepath.Join(externalProjectPath, "project.pbxproj")
	externalBefore := mustReadVersionTestFile(t, pbxprojPath)
	if err := os.WriteFile(externalPBXProjPath, []byte(externalBefore), 0o644); err != nil {
		t.Fatalf("WriteFile(external project.pbxproj) error = %v", err)
	}

	swapped := false
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
		if write.path == pbxprojPath && !swapped {
			swapped = true
			if err := os.Rename(projectPath, originalProjectPath); err != nil {
				return nil, err
			}
			if err := os.Symlink(externalProjectPath, projectPath); err != nil {
				return nil, err
			}
		}
		return atomicWritePreparedVersionFileInfo(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err == nil {
		t.Fatal("SetVersion() error = nil, want swapped .xcodeproj parent refusal")
	}
	if !swapped {
		t.Fatal("test did not swap the .xcodeproj directory")
	}
	if after := mustReadVersionTestFile(t, externalPBXProjPath); after != externalBefore {
		t.Fatalf("external project.pbxproj changed through swapped parent: %q", after)
	}
	if after := mustReadVersionTestFile(t, filepath.Join(originalProjectPath, "project.pbxproj")); after != externalBefore {
		t.Fatalf("original project.pbxproj changed after parent swap: %q", after)
	}
}

func TestSetVersionCoalescesInProjectParentSymlinkXCConfigAliases(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(projectPath)
	configsDir := filepath.Join(projectRoot, "Configs")
	aliasDir := filepath.Join(projectRoot, "AliasDir")
	if err := os.Symlink(configsDir, aliasDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const aliasReference = "DDDDDDDDDDDDDDDDDDDDDDDD"
	fileReference := "\t\t" + aliasReference + " /* Alias Shared.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = AliasDir/Shared.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + aliasReference + "; buildSettings = {}; name = Debug; };"
	if !strings.Contains(contents, widgetConfiguration) {
		t.Fatal("project fixture is missing Widget Debug configuration")
	}
	contents = strings.Replace(contents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  projectPath,
		Version:     "2.0.0",
		BuildNumber: "99",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v, want in-project parent-symlink aliases coalesced into one write", err)
	}
	if len(result.ChangedFiles) == 0 {
		t.Fatalf("SetVersion() result = %#v, want changed files", result)
	}
	shared := mustReadVersionTestFile(t, filepath.Join(configsDir, "Shared.xcconfig"))
	if !strings.Contains(shared, "MARKETING_VERSION = 2.0.0") || !strings.Contains(shared, "CURRENT_PROJECT_VERSION = 99") {
		t.Fatalf("shared xcconfig = %q, want coalesced version update", shared)
	}
}
