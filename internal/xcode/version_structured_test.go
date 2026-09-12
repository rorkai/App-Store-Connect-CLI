package xcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func requireStrictVersionMutationPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("strict identity mutation fails closed before publication on Windows")
	}
}

func TestStructuredVersionFallsBackWhenNoReplaceRenameIsUnsupported(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(preparedVersionWrite, []byte) (os.FileInfo, error) {
		return nil, secureopen.ErrRenameNoReplaceUnsupported
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  project,
		Target:      "App",
		Version:     "2.0.0",
		BuildNumber: "50",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v, want portable stable-command fallback", err)
	}
	if len(result.ChangedFiles) != 1 || !strings.HasSuffix(result.ChangedFiles[0], "project.pbxproj") {
		t.Fatalf("changed files = %#v", result.ChangedFiles)
	}
	updated, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if updated.Version != "2.0.0" || updated.BuildNumber != "50" {
		t.Fatalf("updated version = %#v", updated)
	}
}

func TestStructuredVersionFallsBackWhenIdentityMutationIsUnsupported(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(preparedVersionWrite, []byte) (os.FileInfo, error) {
		return nil, rootfs.ErrFileIdentityMutationUnsupported
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Version: "2.0.0", BuildNumber: "50",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v, want portable stable-command fallback", err)
	}
	if len(result.ChangedFiles) != 1 || !strings.HasSuffix(result.ChangedFiles[0], "project.pbxproj") {
		t.Fatalf("changed files = %#v", result.ChangedFiles)
	}
}

func TestStructuredVersionSupportsLargeXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	original := mustReadVersionTestFile(t, sharedPath)
	padding := strings.Repeat("// compatibility padding\n", signingPlanMaxBytes/len("// compatibility padding\n")+1)
	if err := os.WriteFile(sharedPath, []byte(padding+original), 0o640); err != nil {
		t.Fatalf("WriteFile(large Shared.xcconfig) error = %v", err)
	}
	if got := len(mustReadVersionTestFile(t, sharedPath)); got <= signingPlanMaxBytes {
		t.Fatalf("large xcconfig size = %d, want more than %d", got, signingPlanMaxBytes)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project,
		Target:     "App",
		Version:    "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v, want unrestricted stable compatibility read", err)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != sharedPath {
		t.Fatalf("changed files = %#v, want only %s", result.ChangedFiles, sharedPath)
	}
	updated := mustReadVersionTestFile(t, sharedPath)
	if len(updated) <= signingPlanMaxBytes || !strings.Contains(updated, "MARKETING_VERSION = 2.0.0") {
		t.Fatalf("large xcconfig update was not preserved: size=%d content contains new version=%t", len(updated), strings.Contains(updated, "MARKETING_VERSION = 2.0.0"))
	}
}

func TestStructuredVersionBumpSupportsLargeXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	original := mustReadVersionTestFile(t, sharedPath)
	const unrelated = "UNRELATED_SETTING = KEEP\n"
	padding := strings.Repeat("// compatibility padding\n", signingPlanMaxBytes/len("// compatibility padding\n")+1)
	if err := os.WriteFile(sharedPath, []byte(padding+original+unrelated), 0o640); err != nil {
		t.Fatalf("WriteFile(large Shared.xcconfig) error = %v", err)
	}
	if got := len(mustReadVersionTestFile(t, sharedPath)); got <= signingPlanMaxBytes {
		t.Fatalf("large xcconfig size = %d, want more than %d", got, signingPlanMaxBytes)
	}

	result, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project,
		Target:     "App",
		BumpType:   BumpPatch,
	})
	if err != nil {
		t.Fatalf("BumpVersion() error = %v, want unrestricted stable compatibility read", err)
	}
	if result.OldVersion != "1.2.3" || result.NewVersion != "1.2.4" {
		t.Fatalf("unexpected large xcconfig bump result: %#v", result)
	}
	updated := mustReadVersionTestFile(t, sharedPath)
	if len(updated) <= signingPlanMaxBytes || !strings.Contains(updated, "MARKETING_VERSION = 1.2.4") {
		t.Fatalf("large xcconfig bump was not preserved: size=%d content contains new version=%t", len(updated), strings.Contains(updated, "MARKETING_VERSION = 1.2.4"))
	}
	if !strings.Contains(updated, unrelated) {
		t.Fatalf("large xcconfig bump removed unrelated setting %q", unrelated)
	}
}

func TestStructuredVersion_TargetAndConfigurationScopedEditIsCrossPlatform(t *testing.T) {
	project := writeStructuredVersionProject(t, false)

	previousOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "2.0.0",
		BuildNumber:   "50",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if result.Target != "App" || result.Configuration != "Release" {
		t.Fatalf("unexpected scope: %#v", result)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected two setting changes, got %#v", result.Changes)
	}
	if len(result.ChangedFiles) != 1 || !strings.HasSuffix(result.ChangedFiles[0], "project.pbxproj") {
		t.Fatalf("expected only project.pbxproj to change, got %#v", result.ChangedFiles)
	}

	release, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(Release) error = %v", err)
	}
	if release.Version != "2.0.0" || release.BuildNumber != "50" {
		t.Fatalf("unexpected Release version: %#v", release)
	}

	debug, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Debug",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(Debug) error = %v", err)
	}
	if debug.Version != "1.2.3" || debug.BuildNumber != "42" {
		t.Fatalf("Debug should remain unchanged, got %#v", debug)
	}

	widget, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir:    project,
		Target:        "Widget",
		Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(Widget) error = %v", err)
	}
	if widget.Version != "1.2.3" || widget.BuildNumber != "42" {
		t.Fatalf("Widget should remain unchanged, got %#v", widget)
	}
}

func TestStructuredVersion_ProjectDirectoryWhitespaceIsPreservedDuringMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	originalRoot := filepath.Dir(project)
	whitespaceRoot := filepath.Join(filepath.Dir(originalRoot), " Project Root ")
	if err := os.Rename(originalRoot, whitespaceRoot); err != nil {
		t.Fatalf("Rename(project root) error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(whitespaceRoot) })

	projectPath := filepath.Join(whitespaceRoot, filepath.Base(project))
	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: whitespaceRoot,
		Version:    "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if result.ProjectDir != whitespaceRoot {
		t.Fatalf("result project directory = %q, want %q", result.ProjectDir, whitespaceRoot)
	}
	updated := mustReadVersionTestFile(t, filepath.Join(projectPath, "project.pbxproj"))
	if strings.Contains(updated, "MARKETING_VERSION = 1.2.3;") {
		t.Fatalf("version was not updated in exact whitespace directory: %s", updated)
	}
}

func TestStructuredVersion_ProjectWideEditUpdatesRecursiveXCConfigLosslessly(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:  project,
		Version:     "3.4.5",
		BuildNumber: "99",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if !containsPathSuffix(result.ChangedFiles, "Configs/Shared.xcconfig") {
		t.Fatalf("expected recursive xcconfig in changed files, got %#v", result.ChangedFiles)
	}
	for _, configuration := range []string{"Debug", "Release"} {
		for _, setting := range []string{marketingVersionSetting, currentProjectSetting} {
			if !containsVersionChange(result.Changes, "App", configuration, setting, "xcconfig") {
				t.Fatalf("missing exact xcconfig scope App/%s/%s in %#v", configuration, setting, result.Changes)
			}
		}
	}

	contents, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	got := string(contents)
	if !strings.Contains(got, "MARKETING_VERSION = 3.4.5 // keep this comment\r\n") {
		t.Fatalf("marketing version/comment/CRLF not preserved: %q", got)
	}
	if !strings.Contains(got, "CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 99\r\n") {
		t.Fatalf("conditional build number not updated: %q", got)
	}
	if !strings.Contains(got, "/* MARKETING_VERSION = 8.8.8; */\r\n") {
		t.Fatalf("block comment changed unexpectedly: %q", got)
	}

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Version != "3.4.5" || view.BuildNumber != "99" {
		t.Fatalf("unexpected xcconfig-backed view: %#v", view)
	}
	if !strings.HasSuffix(view.VersionSource, "Configs/Shared.xcconfig") {
		t.Fatalf("unexpected version source: %#v", view)
	}
}

func TestStructuredVersionWindowsCaseSensitiveIncludeGraphUsesLaterInclude(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	project := writeStructuredVersionProject(t, true)
	configDir := filepath.Join(filepath.Dir(project), "Configs")
	appPath := filepath.Join(configDir, "App.xcconfig")
	basePath := filepath.Join(configDir, "Base.xcconfig")
	lowerBasePath := filepath.Join(configDir, "base.xcconfig")
	if err := os.WriteFile(appPath, []byte("#include \"Base.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(basePath, []byte("MARKETING_VERSION = 1.0.0\nCURRENT_PROJECT_VERSION = 10\n#include \"base.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(Base.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(lowerBasePath, []byte("MARKETING_VERSION = 2.0.0\nCURRENT_PROJECT_VERSION = 20\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(base.xcconfig) error = %v", err)
	}
	if _, err := os.Stat(basePath); err != nil {
		t.Fatalf("Stat(Base.xcconfig) error = %v", err)
	}
	if _, err := os.Stat(lowerBasePath); err != nil {
		t.Fatalf("Stat(base.xcconfig) error = %v", err)
	}
	baseInfo, err := os.Stat(basePath)
	if err != nil {
		t.Fatalf("Stat(Base.xcconfig identity) error = %v", err)
	}
	lowerBaseInfo, err := os.Stat(lowerBasePath)
	if err != nil {
		t.Fatalf("Stat(base.xcconfig identity) error = %v", err)
	}
	if os.SameFile(baseInfo, lowerBaseInfo) {
		t.Skip("temporary filesystem is case-insensitive; Windows case-sensitive directory required")
	}

	before, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() before update error = %v", err)
	}
	if before.Version != "2.0.0" || before.BuildNumber != "20" || before.VersionSource != lowerBasePath {
		t.Fatalf("GetVersionScoped() before update = %#v, want later case-distinct include", before)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Version: "3.0.0", BuildNumber: "30",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if !containsString(result.ChangedFiles, basePath) || !containsString(result.ChangedFiles, lowerBasePath) {
		t.Fatalf("SetVersion() changed files = %#v, want both case-distinct includes", result.ChangedFiles)
	}
	for _, path := range []string{basePath, lowerBasePath} {
		contents := mustReadVersionTestFile(t, path)
		if !strings.Contains(contents, "MARKETING_VERSION = 3.0.0") || !strings.Contains(contents, "CURRENT_PROJECT_VERSION = 30") {
			t.Fatalf("updated %s = %q, want version/build changes", path, contents)
		}
	}
	after, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() after update error = %v", err)
	}
	if after.Version != "3.0.0" || after.BuildNumber != "30" || after.VersionSource != lowerBasePath {
		t.Fatalf("GetVersionScoped() after update = %#v, want later case-distinct include", after)
	}
}

func TestStructuredVersion_ProjectWideEditKeepsContinuedXCConfigCompatibility(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	contents := mustReadVersionTestFile(t, sharedPath)
	contents = strings.Replace(contents, "MARKETING_VERSION = 1.2.3 // keep this comment\r\n", "MARKETING_VERSION = 1.2.3 \\\r\n \r\n", 1)
	if err := os.WriteFile(sharedPath, []byte(contents), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	if _, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Version: "3.4.5"}); err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	updated := mustReadVersionTestFile(t, sharedPath)
	if !strings.Contains(updated, "MARKETING_VERSION = 3.4.5\r\n \r\n") {
		t.Fatalf("continued xcconfig compatibility update = %q", updated)
	}
}

func containsVersionChange(changes []VersionChange, target, configuration, setting, source string) bool {
	for _, change := range changes {
		if change.Target == target && change.Configuration == configuration && change.Setting == setting && change.Source == source {
			return true
		}
	}
	return false
}

func TestStructuredVersion_InvalidValueDoesNotMutateAnyFile(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	beforeShared := mustReadVersionTestFile(t, sharedPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project,
		Version:    "1.2.3\nEVIL = YES",
	})
	if err == nil {
		t.Fatal("expected newline validation error")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
		t.Fatal("project.pbxproj changed after validation failure")
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != beforeShared {
		t.Fatal("xcconfig changed after validation failure")
	}
}

func TestStructuredVersion_SyntaxChangingValuesDoNotMutateAnyFile(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "line comment", value: "1.2.3 // ignored"},
		{name: "block comment", value: "1.2.3 /* ignored */"},
		{name: "parenthesized setting", value: "$(UNDEFINED)"},
		{name: "braced setting", value: "${UNDEFINED}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, true)
			pbxprojPath := filepath.Join(project, "project.pbxproj")
			sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
			beforeProject := mustReadVersionTestFile(t, pbxprojPath)
			beforeShared := mustReadVersionTestFile(t, sharedPath)

			_, err := SetVersion(context.Background(), SetVersionOptions{
				ProjectDir: project,
				Version:    test.value,
			})
			if err == nil {
				t.Fatalf("expected %q to be rejected", test.value)
			}
			if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
				t.Fatal("project.pbxproj changed after value validation failure")
			}
			if got := mustReadVersionTestFile(t, sharedPath); got != beforeShared {
				t.Fatal("xcconfig changed after value validation failure")
			}
		})
	}
}

func TestStructuredVersion_MissingScopeErrorsWithoutMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "Missing",
		Configuration: "Release",
		Version:       "2.0.0",
	})
	if err == nil {
		t.Fatal("expected missing target error")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project.pbxproj changed after scope validation failure")
	}
}

func TestStructuredVersion_TargetScopeUpdatesAllTargetConfigurations(t *testing.T) {
	project := writeStructuredVersionProject(t, false)

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project,
		Target:     "App",
		Version:    "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected App Debug and Release changes, got %#v", result.Changes)
	}
	for _, configuration := range []string{"Debug", "Release"} {
		view := mustGetStructuredVersion(t, project, "App", configuration)
		if view.Version != "2.0.0" {
			t.Fatalf("App %s version = %q", configuration, view.Version)
		}
	}
	if view := mustGetStructuredVersion(t, project, "Widget", "Release"); view.Version != "1.2.3" {
		t.Fatalf("Widget changed outside target scope: %#v", view)
	}
}

func TestStructuredVersion_ConfigurationScopeUpdatesMatchingTargetsOnly(t *testing.T) {
	project := writeStructuredVersionProject(t, false)

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Configuration: "Debug",
		BuildNumber:   "55",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected App and Widget Debug changes, got %#v", result.Changes)
	}
	for _, target := range []string{"App", "Widget"} {
		if view := mustGetStructuredVersion(t, project, target, "Debug"); view.BuildNumber != "55" {
			t.Fatalf("%s Debug build = %q", target, view.BuildNumber)
		}
		if view := mustGetStructuredVersion(t, project, target, "Release"); view.BuildNumber != "42" {
			t.Fatalf("%s Release changed outside configuration scope: %#v", target, view)
		}
	}
}

func TestStructuredVersion_ConfigurationScopeDoesNotMaterializeRedundantInheritedOverrides(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };",
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };", 1)
	contents = strings.Replace(contents,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = {}; name = Release; };", 1)
	contents = strings.Replace(contents,
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Configuration: "Release", Version: "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.Changes) != 1 || result.Changes[0].Target != "" || result.Changes[0].Configuration != "Release" {
		t.Fatalf("expected one project-level inherited change, got %#v", result.Changes)
	}
	for _, target := range []string{"App", "Widget"} {
		if view := mustGetStructuredVersion(t, project, target, "Release"); view.Version != "2.0.0" {
			t.Fatalf("%s did not inherit updated project value: %#v", target, view)
		}
	}
}

func TestStructuredVersion_DefaultViewSelectsApplicationAndDefaultConfiguration(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	view, err := GetVersionScoped(context.Background(), GetVersionOptions{ProjectDir: project})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Target != "App" || view.Configuration != "Release" || view.Version != "1.2.3" || view.BuildNumber != "42" {
		t.Fatalf("unexpected default view: %#v", view)
	}
}

func TestStructuredVersion_GetVersionScopedPreservesSelectedProjectSymlink(t *testing.T) {
	targetProject := writeStructuredVersionProject(t, false)
	selectedRoot := t.TempDir()
	selectedProject := filepath.Join(selectedRoot, "Selected.xcodeproj")
	if err := os.Symlink(targetProject, selectedProject); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{ProjectDir: selectedProject})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v, want stable symlink compatibility", err)
	}
	if view.Version != "1.2.3" || view.BuildNumber != "42" {
		t.Fatalf("GetVersionScoped() = %#v, want target project version", view)
	}
}

func TestStructuredVersion_DirectInheritedValueUsesNextLowerLayer(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };",
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 7.8.9; }; name = Release; };", 1)
	contents = strings.Replace(contents,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = \"$(inherited)\"; CURRENT_PROJECT_VERSION = 42; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	view := mustGetStructuredVersion(t, project, "App", "Release")
	if view.Version != "7.8.9" {
		t.Fatalf("inherited version = %q, want project-level 7.8.9", view.Version)
	}
}

func TestStructuredVersion_TargetXCConfigInheritedUsesProjectLayer(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };",
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 7.8.9; CURRENT_PROJECT_VERSION = 78; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	if err := os.WriteFile(sharedPath, []byte("MARKETING_VERSION = $(inherited)\nCURRENT_PROJECT_VERSION = $(inherited)\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Version != "7.8.9" || view.BuildNumber != "78" {
		t.Fatalf("project-layer inheritance resolved to %#v", view)
	}
}

func TestStructuredVersion_UnscopedBumpSupportsMultipleApplicationTargets(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		`productType = "com.apple.product-type.app-extension";`,
		`productType = "com.apple.product-type.application";`, 1)
	contents = strings.Replace(contents,
		`explicitFileType = "wrapper.app-extension"; path = Widget.appex;`,
		`explicitFileType = wrapper.application; path = Widget.app;`, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	result, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project,
		BumpType:   BumpPatch,
	})
	if err != nil {
		t.Fatalf("BumpVersion() error = %v", err)
	}
	if result.OldVersion != "1.2.3" || result.NewVersion != "1.2.4" {
		t.Fatalf("unexpected unscoped bump result: %#v", result)
	}
	for _, target := range []string{"App", "Widget"} {
		for _, configuration := range []string{"Debug", "Release"} {
			if view := mustGetStructuredVersion(t, project, target, configuration); view.Version != "1.2.4" {
				t.Fatalf("%s/%s version = %q, want 1.2.4", target, configuration, view.Version)
			}
		}
	}
}

func TestStructuredVersion_BumpRejectsMissingSettingWithoutMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { CURRENT_PROJECT_VERSION = 42; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project,
		BumpType:   BumpPatch,
	})
	if err == nil || !strings.Contains(err.Error(), "MARKETING_VERSION") {
		t.Fatalf("expected missing MARKETING_VERSION error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after missing-setting validation failure")
	}
}

func TestStructuredVersion_EditRejectsMissingSettingWithoutMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { CURRENT_PROJECT_VERSION = 42; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project,
		Version:    "2.0.0",
	})
	if err == nil || !strings.Contains(err.Error(), "MARKETING_VERSION") {
		t.Fatalf("expected missing MARKETING_VERSION error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after missing-setting edit validation failure")
	}
}

func TestStructuredVersion_RejectsConditionalOnlyPBXProjValue(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; \"CURRENT_PROJECT_VERSION[sdk=iphoneos*]\" = 42; \"CURRENT_PROJECT_VERSION[sdk=macosx*]\" = 43; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	_, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err == nil || !strings.Contains(err.Error(), "conditional") {
		t.Fatalf("expected conditional-only build-setting error, got %v", err)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", BuildNumber: "50",
	})
	if err != nil {
		t.Fatalf("conditional-only direct edit should not need SDK resolution: %v", err)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("expected both conditional assignments to change, got %#v", result.Changes)
	}
	updated := mustReadVersionTestFile(t, pbxprojPath)
	if !strings.Contains(updated, `"CURRENT_PROJECT_VERSION[sdk=iphoneos*]" = 50;`) ||
		!strings.Contains(updated, `"CURRENT_PROJECT_VERSION[sdk=macosx*]" = 50;`) {
		t.Fatalf("conditional assignments were not both edited: %s", updated)
	}
}

func TestStructuredVersion_BumpRejectsDivergentConditionalPBXProjValue(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release;",
		"MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; \"CURRENT_PROJECT_VERSION[sdk=iphoneos*]\" = 100; }; name = Release;", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", BumpType: BumpBuild,
	})
	if err == nil || !strings.Contains(err.Error(), "differing conditional") {
		t.Fatalf("expected divergent conditional build-setting error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("pbxproj changed after rejected conditional bump")
	}
}

func TestStructuredVersion_BumpPreservesEquivalentConditionalPBXProjValues(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release;",
		"MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; \"CURRENT_PROJECT_VERSION[sdk=iphoneos*]\" = 42; }; name = Release;", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	result, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", BumpType: BumpBuild,
	})
	if err != nil {
		t.Fatalf("BumpVersion() error = %v", err)
	}
	if result.OldBuild != "42" || result.NewBuild != "43" || len(result.Changes) != 2 {
		t.Fatalf("unexpected equivalent-conditional bump: %#v", result)
	}
	updated := mustReadVersionTestFile(t, pbxprojPath)
	if !strings.Contains(updated, `"CURRENT_PROJECT_VERSION" = 43;`) ||
		!strings.Contains(updated, `"CURRENT_PROJECT_VERSION[sdk=iphoneos*]" = 43;`) {
		t.Fatalf("equivalent conditional values were not bumped together: %s", updated)
	}
}

func TestStructuredVersion_PartialBuildSettingsAreReported(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := strings.ReplaceAll(mustReadVersionTestFile(t, pbxprojPath), "CURRENT_PROJECT_VERSION = 42;", "")
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	parsed, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	structured, err := parsed.hasStructuredVersionSettingsForMutation("", "")
	if err == nil || !strings.Contains(err.Error(), "partially") {
		t.Fatalf("expected partial structured coverage error, got structured=%v err=%v", structured, err)
	}
}

func TestStructuredVersion_PartialMarketingVersionEditAndBumpStayStructured(t *testing.T) {
	t.Run("remote selection", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		pbxprojPath := filepath.Join(project, "project.pbxproj")
		contents := strings.ReplaceAll(mustReadVersionTestFile(t, pbxprojPath), "CURRENT_PROJECT_VERSION = 42;", "")
		if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(project) error = %v", err)
		}
		restore := overrideTestEnvironment(t)
		runtimeGOOS = "linux"
		lookPathFn = func(file string) (string, error) { return "", errors.New("not available") }
		t.Cleanup(restore)

		version, err := GetConsistentMarketingVersion(context.Background(), GetVersionOptions{ProjectDir: project})
		if err != nil {
			t.Fatalf("GetConsistentMarketingVersion() error = %v", err)
		}
		if version != "1.2.3" {
			t.Fatalf("consistent partial marketing version = %q, want 1.2.3", version)
		}
	})

	t.Run("edit", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		pbxprojPath := filepath.Join(project, "project.pbxproj")
		contents := strings.ReplaceAll(mustReadVersionTestFile(t, pbxprojPath), "CURRENT_PROJECT_VERSION = 42;", "")
		if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(project) error = %v", err)
		}
		restore := overrideTestEnvironment(t)
		runtimeGOOS = "linux"
		lookPathFn = func(file string) (string, error) { return "", errors.New("not available") }
		t.Cleanup(restore)

		result, err := SetVersion(context.Background(), SetVersionOptions{
			ProjectDir: project, Version: "2.0.0",
		})
		if err != nil {
			t.Fatalf("SetVersion() error = %v", err)
		}
		if result.Version != "2.0.0" || len(result.Changes) != 4 {
			t.Fatalf("unexpected partial structured edit: %#v", result)
		}
		if updated := mustReadVersionTestFile(t, pbxprojPath); strings.Contains(updated, "MARKETING_VERSION = 1.2.3;") {
			t.Fatalf("marketing version remained unchanged: %s", updated)
		}
	})

	t.Run("bump", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		pbxprojPath := filepath.Join(project, "project.pbxproj")
		contents := strings.ReplaceAll(mustReadVersionTestFile(t, pbxprojPath), "CURRENT_PROJECT_VERSION = 42;", "")
		if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("WriteFile(project) error = %v", err)
		}
		restore := overrideTestEnvironment(t)
		runtimeGOOS = "linux"
		lookPathFn = func(file string) (string, error) { return "", errors.New("not available") }
		t.Cleanup(restore)

		result, err := BumpVersion(context.Background(), BumpVersionOptions{
			ProjectDir: project, BumpType: BumpPatch,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v", err)
		}
		if result.OldVersion != "1.2.3" || result.NewVersion != "1.2.4" {
			t.Fatalf("unexpected partial structured bump: %#v", result)
		}
	})
}

func TestStructuredVersion_MixedStructuredAndLegacyEditFailsBeforeMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := strings.ReplaceAll(mustReadVersionTestFile(t, pbxprojPath), "CURRENT_PROJECT_VERSION = 42;", "")
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Version: "2.0.0", BuildNumber: "43",
	})
	if err == nil || !strings.Contains(err.Error(), "partially") {
		t.Fatalf("expected partial structured coverage error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after mixed structured/legacy edit rejection")
	}
}

func TestStructuredVersion_SelectedBrokenXCConfigNeverFallsBackToLegacy(t *testing.T) {
	operations := map[string]func(context.Context, string) error{
		"view": func(ctx context.Context, project string) error {
			_, err := GetVersionScoped(ctx, GetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Release"})
			return err
		},
		"marketing version": func(ctx context.Context, project string) error {
			_, err := GetConsistentMarketingVersion(ctx, GetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Release"})
			return err
		},
		"edit": func(ctx context.Context, project string) error {
			_, err := SetVersion(ctx, SetVersionOptions{ProjectDir: project, Target: "App", Configuration: "Release", Version: "2.0.0"})
			return err
		},
		"bump": func(ctx context.Context, project string) error {
			_, err := BumpVersion(ctx, BumpVersionOptions{ProjectDir: project, Target: "App", Configuration: "Release", BumpType: BumpPatch})
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, true)
			sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
			if err := os.Remove(sharedPath); err != nil {
				t.Fatalf("Remove(Shared.xcconfig) error = %v", err)
			}
			logPath := filepath.Join(t.TempDir(), "commands.log")
			restore := overrideTestEnvironment(t)
			runtimeGOOS = "darwin"
			lookPathFn = func(file string) (string, error) { return "/usr/bin/" + file, nil }
			commandContextFn = helperCommandContext(t, logPath)
			t.Setenv("ASC_XCODE_HELPER_SINGLE_TARGET", "1")
			t.Cleanup(restore)

			err := operation(context.Background(), project)
			if err == nil || !strings.Contains(err.Error(), "Shared.xcconfig") {
				t.Fatalf("expected selected xcconfig error, got %v", err)
			}
			if log, readErr := os.ReadFile(logPath); readErr == nil && len(log) > 0 {
				t.Fatalf("legacy command ran after selected xcconfig failure: %s", log)
			}
		})
	}
}

func TestStructuredVersion_SelectedPartialTargetUsesLegacyRouting(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "commands.log")
	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	commandContextFn = helperCommandContext(t, logPath)
	t.Setenv("ASC_XCODE_HELPER_SINGLE_TARGET", "1")
	t.Cleanup(restore)

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Modern || view.Version != "1.2.3" || view.BuildNumber != "41" {
		t.Fatalf("expected selected partial target to use legacy routing, got %#v", view)
	}
}

func TestStructuredVersion_SelectedDirectSettingsIgnoreUnrelatedBrokenXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"\t\t111111111111111111111111 /* Project object */ = {",
		"\t\tBBBBBBBBBBBBBBBBBBBBBBBB /* Broken.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Broken.xcconfig; sourceTree = SOURCE_ROOT; };\n\t\t111111111111111111111111 /* Project object */ = {", 1)
	contents = strings.Replace(contents,
		"999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };",
		"999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Version != "1.2.3" || view.BuildNumber != "42" {
		t.Fatalf("unexpected selected version: %#v", view)
	}

	if _, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", BuildNumber: "43",
	}); err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
}

func TestStructuredVersion_UnrelatedBrokenXCConfigDoesNotBlockLegacyRouting(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.ReplaceAll(contents, marketingVersionSetting, "LEGACY_MARKETING_VERSION")
	contents = strings.ReplaceAll(contents, currentProjectSetting, "LEGACY_CURRENT_PROJECT_VERSION")
	contents = strings.Replace(contents,
		"\t\t111111111111111111111111 /* Project object */ = {",
		"\t\tBBBBBBBBBBBBBBBBBBBBBBBB /* Broken.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Missing.xcconfig; sourceTree = SOURCE_ROOT; };\n\t\t111111111111111111111111 /* Project object */ = {", 1)
	contents = strings.Replace(contents,
		"999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { LEGACY_MARKETING_VERSION = 1.2.3; LEGACY_CURRENT_PROJECT_VERSION = 42; }; name = Debug; };",
		"999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB; buildSettings = { LEGACY_MARKETING_VERSION = 1.2.3; LEGACY_CURRENT_PROJECT_VERSION = 42; }; name = Debug; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "commands.log")
	restore := overrideTestEnvironment(t)
	runtimeGOOS = "darwin"
	lookPathFn = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	commandContextFn = helperCommandContext(t, logPath)
	t.Setenv("ASC_XCODE_HELPER_SINGLE_TARGET", "1")
	t.Cleanup(restore)

	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped() error = %v", err)
	}
	if view.Modern || view.Version != "1.2.3" || view.BuildNumber != "41" {
		t.Fatalf("expected legacy routing, got %#v", view)
	}
}

func TestValidateSetVersionDoesNotMutateProjectOrXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	xcconfigPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	pbxprojBefore := mustReadVersionTestFile(t, pbxprojPath)
	xcconfigBefore := mustReadVersionTestFile(t, xcconfigPath)

	err := ValidateSetVersion(t.Context(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "9.8.7",
		BuildNumber:   "654",
	})
	if err != nil {
		t.Fatalf("ValidateSetVersion() error = %v", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != pbxprojBefore {
		t.Fatal("project.pbxproj changed during validation")
	}
	if after := mustReadVersionTestFile(t, xcconfigPath); after != xcconfigBefore {
		t.Fatal("xcconfig changed during validation")
	}
}

func TestGetConsistentMarketingVersionRejectsDivergentSelectedScope(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	if _, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug", Version: "2.0.0",
	}); err != nil {
		t.Fatalf("prepare divergent version: %v", err)
	}

	_, err := GetConsistentMarketingVersion(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected divergent marketing-version error, got %v", err)
	}

	version, err := GetConsistentMarketingVersion(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug",
	})
	if err != nil {
		t.Fatalf("scoped consistent version error = %v", err)
	}
	if version != "2.0.0" {
		t.Fatalf("scoped consistent version = %q, want 2.0.0", version)
	}
}

func TestStructuredVersion_GroupRelativeXCConfigReference(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT;",
		"path = App.xcconfig; sourceTree = \"<group>\";", 1)
	contents = strings.Replace(contents,
		"AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ =",
		"ABABABABABABABABABABABAB /* Configs */ = {isa = PBXGroup; children = (AAAAAAAAAAAAAAAAAAAAAAAA,); path = Configs; sourceTree = \"<group>\"; };\n\t\tAAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ =", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	view := mustGetStructuredVersion(t, project, "App", "Release")
	if view.Version != "1.2.3" || !strings.HasSuffix(view.VersionSource, "Configs/Shared.xcconfig") {
		t.Fatalf("group-relative xcconfig was not resolved: %#v", view)
	}
}

func TestStructuredVersion_AbsoluteGroupXCConfigReference(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	configDir := filepath.Join(filepath.Dir(project), "Configs")
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	old := "path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT;"
	if count := strings.Count(contents, old); count != 1 {
		t.Fatalf("expected one App.xcconfig reference, found %d", count)
	}
	contents = strings.Replace(contents,
		old,
		"path = App.xcconfig; sourceTree = \"<group>\";", 1)
	old = "AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ ="
	if count := strings.Count(contents, old); count != 1 {
		t.Fatalf("expected one App.xcconfig object, found %d", count)
	}
	contents = strings.Replace(contents,
		old,
		"ABABABABABABABABABABABAB /* Configs */ = {isa = PBXGroup; children = (AAAAAAAAAAAAAAAAAAAAAAAA,); path = \""+configDir+"\"; sourceTree = \"<absolute>\"; };\n\t\tAAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ =", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

	view := mustGetStructuredVersion(t, project, "App", "Release")
	if view.Version != "1.2.3" || !strings.HasSuffix(view.VersionSource, "Configs/Shared.xcconfig") {
		t.Fatalf("absolute-group xcconfig was not resolved: %#v", view)
	}
}

func TestStructuredVersion_ScopedBumps(t *testing.T) {
	t.Run("patch", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		result, err := BumpVersion(context.Background(), BumpVersionOptions{
			ProjectDir: project, Target: "App", Configuration: "Release", BumpType: BumpPatch,
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v", err)
		}
		if result.OldVersion != "1.2.3" || result.NewVersion != "1.2.4" || len(result.Changes) != 1 {
			t.Fatalf("unexpected patch result: %#v", result)
		}
		if view := mustGetStructuredVersion(t, project, "App", "Debug"); view.Version != "1.2.3" {
			t.Fatalf("patch leaked to Debug: %#v", view)
		}
	})

	t.Run("remote build override", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		result, err := BumpVersion(context.Background(), BumpVersionOptions{
			ProjectDir: project, Target: "Widget", Configuration: "Debug", BumpType: BumpBuild, BuildNumber: "108",
		})
		if err != nil {
			t.Fatalf("BumpVersion() error = %v", err)
		}
		if result.OldBuild != "42" || result.NewBuild != "108" || len(result.Changes) != 1 {
			t.Fatalf("unexpected build result: %#v", result)
		}
		if view := mustGetStructuredVersion(t, project, "Widget", "Debug"); view.BuildNumber != "108" {
			t.Fatalf("remote override not applied: %#v", view)
		}
	})
}

func TestStructuredVersion_NonBuildBumpRejectsBuildOverrideWithoutMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir:  project,
		BumpType:    BumpPatch,
		BuildNumber: "99",
	})
	if err == nil || !strings.Contains(err.Error(), "only supported for build bumps") {
		t.Fatalf("expected build override usage error, got %v", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != before {
		t.Fatal("project changed after rejected build override")
	}
}

func TestStructuredVersion_BumpIncludesMutatedProjectSettingInBaseline(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };",
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 9.0.0; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, Configuration: "Release", BumpType: BumpPatch,
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected project-level divergence error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after project-level baseline divergence")
	}
}

func TestStructuredVersion_BumpRefusesToFlattenDifferentValues(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	if _, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug", Version: "9.0.0",
	}); err != nil {
		t.Fatalf("prepare divergent version: %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, Target: "App", BumpType: BumpPatch,
	})
	if err == nil {
		t.Fatal("expected mixed-value bump to fail")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after inconsistent bump validation")
	}
}

func TestStructuredVersion_RemoteBuildBumpRefusesToFlattenDifferentValues(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	if _, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug", BuildNumber: "9",
	}); err != nil {
		t.Fatalf("prepare divergent build: %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, BumpType: BumpBuild, BuildNumber: "108",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected remote mixed-value bump to fail, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed after remote consistency validation")
	}
}

func TestValidateBumpVersionRejectsDivergentBuildsWithoutMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	if _, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug", BuildNumber: "9",
	}); err != nil {
		t.Fatalf("prepare divergent build: %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	err := ValidateBumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, BumpType: BumpBuild, BuildNumber: "108",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected remote mixed-value bump validation to fail, got %v", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("project changed during bump validation")
	}
}

func TestStructuredVersion_ScopedSharedXCConfigCreatesOverrideWithoutLeaking(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	before := mustReadVersionTestFile(t, sharedPath)

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "4.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) != 1 || !strings.HasSuffix(result.ChangedFiles[0], "project.pbxproj") {
		t.Fatalf("expected a scoped pbxproj override, got %#v", result.ChangedFiles)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != before {
		t.Fatal("shared xcconfig changed despite unselected consumers")
	}
	if view := mustGetStructuredVersion(t, project, "App", "Release"); view.Version != "4.0.0" {
		t.Fatalf("Release override was not applied: %#v", view)
	}
	if view := mustGetStructuredVersion(t, project, "App", "Debug"); view.Version != "1.2.3" {
		t.Fatalf("Debug leaked scoped change: %#v", view)
	}
}

func TestStructuredVersion_ScopedInheritedNoOpDoesNotCreateOverride(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "1.2.3",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) != 0 || len(result.Changes) != 0 {
		t.Fatalf("inherited no-op reported mutations: %#v", result)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("inherited no-op created a target-level override")
	}
}

func TestStructuredVersion_DirectInheritedNoOpPreservesPBXProjAssignment(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	contents = strings.Replace(contents,
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };",
		"999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 7.8.9; }; name = Release; };", 1)
	contents = strings.Replace(contents,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = \"$(inherited)\"; CURRENT_PROJECT_VERSION = 42; }; name = Release; };", 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	before := mustReadVersionTestFile(t, pbxprojPath)

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", Version: "7.8.9",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) != 0 || len(result.Changes) != 0 {
		t.Fatalf("direct inherited no-op reported mutations: %#v", result)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("direct inherited no-op replaced inheritance with a literal")
	}
}

func TestStructuredVersion_ScopedConditionalSharedSettingDoesNotReportFalseSuccess(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	conditional := "MARKETING_VERSION[sdk=iphoneos*] = 1.2.3\nCURRENT_PROJECT_VERSION = 42\n"
	if err := os.WriteFile(sharedPath, []byte(conditional), 0o640); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	setOptions := SetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", Version: "2.0.0",
	}

	if err := ValidateSetVersion(t.Context(), setOptions); err == nil || !strings.Contains(err.Error(), "conditional") {
		t.Fatalf("expected conditional-only scoped preflight error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != conditional {
		t.Fatal("conditional shared xcconfig changed during preflight")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
		t.Fatal("pbxproj changed during rejected conditional preflight")
	}

	_, err := SetVersion(context.Background(), setOptions)
	if err == nil || !strings.Contains(err.Error(), "conditional") {
		t.Fatalf("expected conditional-only scoped edit error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != conditional {
		t.Fatal("conditional shared xcconfig changed after rejected edit")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
		t.Fatal("pbxproj changed after rejected conditional edit")
	}
}

func TestStructuredVersion_BumpRejectsDivergentConditionalXCConfigValue(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	contents := mustReadVersionTestFile(t, sharedPath)
	contents = strings.Replace(contents,
		"CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 42",
		"CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 100", 1)
	if err := os.WriteFile(sharedPath, []byte(contents), 0o640); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}
	before := mustReadVersionTestFile(t, sharedPath)

	_, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release", BumpType: BumpBuild,
	})
	if err == nil || !strings.Contains(err.Error(), "differing conditional") {
		t.Fatalf("expected divergent conditional xcconfig error, got %v", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != before {
		t.Fatal("xcconfig changed after rejected conditional bump")
	}
}

func TestStructuredVersion_BumpPreservesEquivalentConditionalXCConfigValues(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")

	result, err := BumpVersion(context.Background(), BumpVersionOptions{
		ProjectDir: project, BumpType: BumpBuild,
	})
	if err != nil {
		t.Fatalf("BumpVersion() error = %v", err)
	}
	if result.OldBuild != "42" || result.NewBuild != "43" {
		t.Fatalf("unexpected equivalent-conditional xcconfig bump: %#v", result)
	}
	updated := mustReadVersionTestFile(t, sharedPath)
	if !strings.Contains(updated, "CURRENT_PROJECT_VERSION = 43\r\n") ||
		!strings.Contains(updated, "CURRENT_PROJECT_VERSION[sdk=iphoneos*] = 43\r\n") {
		t.Fatalf("equivalent conditional xcconfig values were not bumped together: %q", updated)
	}
}

func TestStructuredVersion_NoOpReportsNoFilesOrChanges(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "1.2.3",
		BuildNumber:   "42",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if len(result.ChangedFiles) != 0 || len(result.Changes) != 0 {
		t.Fatalf("no-op reported mutations: %#v", result)
	}
}

func TestStructuredVersion_XCConfigPermissionsArePreserved(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")

	if _, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Version: "5.0.0"}); err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	info, err := os.Stat(sharedPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("xcconfig mode = %o, want 640", got)
	}
}

func TestStructuredVersion_ScopedEditTreatsXCConfigSymlinkAliasAsSharedConsumer(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	configDir := filepath.Join(filepath.Dir(project), "Configs")
	sharedPath := filepath.Join(configDir, "Shared.xcconfig")
	aliasPath := filepath.Join(configDir, "Alias.xcconfig")
	sharedBefore := mustReadVersionTestFile(t, sharedPath)

	projectContents := mustReadVersionTestFile(t, pbxprojPath)
	projectContents = strings.Replace(
		projectContents,
		"AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT; };",
		"AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT; };\n\t\tBBBBBBBBBBBBBBBBBBBBBBBB /* Alias.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Alias.xcconfig; sourceTree = SOURCE_ROOT; };",
		1,
	)
	projectContents = strings.Replace(
		projectContents,
		"999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA;",
		"999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB;",
		1,
	)
	if err := os.WriteFile(pbxprojPath, []byte(projectContents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	if err := os.Symlink("Shared.xcconfig", aliasPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := SetVersion(context.Background(), SetVersionOptions{
		ProjectDir:    project,
		Target:        "App",
		Configuration: "Release",
		Version:       "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != sharedBefore {
		t.Fatalf("shared xcconfig changed through its regular path despite an unselected alias consumer: %q", got)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != pbxprojPath {
		t.Fatalf("expected only a scoped pbxproj override, got %#v", result.ChangedFiles)
	}

	release, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Release",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(Release) error = %v", err)
	}
	if release.Version != "2.0.0" {
		t.Fatalf("Release version = %q, want 2.0.0", release.Version)
	}
	debug, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: "App", Configuration: "Debug",
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(Debug) error = %v", err)
	}
	if debug.Version != "1.2.3" {
		t.Fatalf("Debug version = %q, want unchanged 1.2.3", debug.Version)
	}
}

func TestStructuredVersion_RefusesXCConfigSymlinkBeforeMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	targetPath := filepath.Join(filepath.Dir(project), "outside.xcconfig")
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(targetPath, []byte(shared), 0o644); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.Remove(sharedPath); err != nil {
		t.Fatalf("Remove(shared) error = %v", err)
	}
	if err := os.Symlink(targetPath, sharedPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	_, err := SetVersion(context.Background(), SetVersionOptions{ProjectDir: project, Version: "6.0.0"})
	if err == nil {
		t.Fatal("expected symlink refusal")
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
		t.Fatal("project changed before symlink validation completed")
	}
	if got := mustReadVersionTestFile(t, targetPath); got != shared {
		t.Fatal("symlink target changed")
	}
}

func TestStructuredVersion_CommitFailureRollsBackEarlierFiles(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "a.xcconfig")
	secondPath := filepath.Join(root, "b.xcconfig")
	if err := os.WriteFile(firstPath, []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("old-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstInfo := mustStatVersionTestFile(t, firstPath)
	secondInfo := mustStatVersionTestFile(t, secondPath)

	injectedErr := errors.New("injected write failure")
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
		if write.path == secondPath {
			return nil, injectedErr
		}
		return atomicWritePreparedVersionFileInfo(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: secondPath, root: fileRoot, name: secondPath, original: []byte("old-b"), updated: []byte("new-b"), mode: 0o644, originalInfo: secondInfo},
		{path: firstPath, root: fileRoot, name: firstPath, original: []byte("old-a"), updated: []byte("new-a"), mode: 0o644, originalInfo: firstInfo},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("expected injected failure, got %v", err)
	}
	if got := mustReadVersionTestFile(t, firstPath); got != "old-a" {
		t.Fatalf("first file was not rolled back: %q", got)
	}
	if got := mustReadVersionTestFile(t, secondPath); got != "old-b" {
		t.Fatalf("second file changed: %q", got)
	}
	if opened, err := fileRoot.OpenRoot(); err == nil {
		_ = opened.Close()
		t.Fatal("commitVersionWrites() retained its pinned root after rollback")
	}
}

func TestStructuredVersion_CommitVerificationFailureRollsBackEarlierFiles(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "a.xcconfig")
	secondPath := filepath.Join(root, "b.xcconfig")
	if err := os.WriteFile(firstPath, []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("old-b"), 0o644); err != nil {
		t.Fatal(err)
	}
	firstInfo := mustStatVersionTestFile(t, firstPath)
	secondInfo := mustStatVersionTestFile(t, secondPath)

	originalWriter := atomicWriteVersionFileInfoFn
	firstWrite := true
	atomicWriteVersionFileInfoFn = func(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
		if write.path == firstPath && firstWrite {
			firstWrite = false
			info, err := atomicWritePreparedVersionFileInfo(write, data)
			if err != nil {
				return info, err
			}
			if err := os.WriteFile(secondPath, []byte("concurrent"), 0o644); err != nil {
				return info, err
			}
			return info, nil
		}
		return atomicWritePreparedVersionFileInfo(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	firstRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	secondRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: secondPath, root: secondRoot, name: secondPath, original: []byte("old-b"), updated: []byte("new-b"), mode: 0o644, originalInfo: secondInfo},
		{path: firstPath, root: firstRoot, name: firstPath, original: []byte("old-a"), updated: []byte("new-a"), mode: 0o644, originalInfo: firstInfo},
	})
	if err == nil || !strings.Contains(err.Error(), "source changed before commit") {
		t.Fatalf("commitVersionWrites() error = %v, want verification failure", err)
	}
	if got := mustReadVersionTestFile(t, firstPath); got != "old-a" {
		t.Fatalf("first file was not rolled back after verification failure: %q", got)
	}
	if got := mustReadVersionTestFile(t, secondPath); got != "concurrent" {
		t.Fatalf("concurrent second-file save was overwritten: %q", got)
	}
}

func TestStructuredVersion_CommitRejectsSameByteReplacementOfPinnedSource(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	original := []byte("same bytes")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	pinnedIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	replacement := filepath.Join(root, "replacement.xcconfig")
	if err := os.WriteFile(replacement, original, 0o644); err != nil {
		t.Fatal(err)
	}
	replacementInfo, err := os.Stat(replacement)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	err = commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: filepath.Base(path),
		original: original, updated: []byte("updated bytes"), mode: 0o644,
		originalIdentity: pinnedIdentity, strictIdentity: true,
	}})
	if err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("commitVersionWrites() error = %v, want pinned-identity rejection", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("same-byte replacement was overwritten: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("replacement mode = %o, want 644", info.Mode().Perm())
	}
	if os.SameFile(pinnedIdentity.Info(), info) {
		t.Fatal("replacement destination still refers to the pinned source inode")
	}
	if !os.SameFile(replacementInfo, info) {
		t.Fatal("replacement destination does not retain the replacement inode")
	}
}

func TestStructuredVersion_CommitCapturesSourceIdentityBeforePublication(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	if err := commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: filepath.Base(path),
		original: []byte("old"), updated: []byte("new"), mode: 0o644,
		strictIdentity: true,
	}}); err != nil {
		t.Fatalf("commitVersionWrites() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("committed contents = %q, want new", got)
	}
}

func TestStructuredVersion_CommitRejectsReplacementAfterIdentityCapture(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.xcconfig")
	original := []byte("old")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	previous := afterVersionSourceCaptureFn
	t.Cleanup(func() { afterVersionSourceCaptureFn = previous })
	afterVersionSourceCaptureFn = func([]preparedVersionWrite) error {
		if err := os.Remove(path); err != nil {
			return err
		}
		return os.WriteFile(path, original, 0o644)
	}
	err = commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: filepath.Base(path), original: original,
		updated: []byte("new"), mode: 0o644, strictIdentity: true,
	}})
	if err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("commit error = %v, want replacement rejection", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("replacement contents = %q, want original", got)
	}
}

func TestStructuredVersion_CommitNoOpSkipsSourceIdentityCapture(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	dir := t.TempDir()
	fileRoot, err := rootfs.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	const contents = "unchanged"
	err = commitVersionWrites([]preparedVersionWrite{
		{path: filepath.Join(dir, "missing.xcconfig"), root: fileRoot, name: "missing.xcconfig", original: []byte(contents), updated: []byte(contents), strictIdentity: true},
	})
	if err != nil {
		t.Fatalf("no-op commit unexpectedly failed: %v", err)
	}
}

func TestSetVersionKeepsProvenWindowsCaseDistinctXCConfigsSeparate(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	projectPath := writeStructuredVersionProject(t, true)
	root := filepath.Dir(projectPath)
	configDir := filepath.Join(root, "Configs")
	firstPath := filepath.Join(configDir, "Config.xcconfig")
	secondPath := filepath.Join(configDir, "config.xcconfig")
	if _, err := os.Lstat(secondPath); err == nil {
		t.Skip("temporary filesystem is case-insensitive")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(case variant) error = %v", err)
	}
	contents := []byte("MARKETING_VERSION = 1.2.3\nCURRENT_PROJECT_VERSION = 42\n")
	if err := os.WriteFile(firstPath, contents, 0o640); err != nil {
		t.Fatalf("WriteFile(first xcconfig) error = %v", err)
	}
	if err := os.WriteFile(secondPath, contents, 0o640); err != nil {
		t.Fatalf("WriteFile(second xcconfig) error = %v", err)
	}
	firstInfo, err := os.Stat(firstPath)
	if err != nil {
		t.Fatalf("Stat(first xcconfig) error = %v", err)
	}
	secondInfo, err := os.Stat(secondPath)
	if err != nil {
		t.Fatalf("Stat(second xcconfig) error = %v", err)
	}
	if os.SameFile(firstInfo, secondInfo) {
		t.Skip("temporary filesystem is case-insensitive")
	}

	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	pbxproj := mustReadVersionTestFile(t, pbxprojPath)
	pbxproj = strings.Replace(pbxproj,
		"AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT; };",
		"AAAAAAAAAAAAAAAAAAAAAAAA /* Config.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Config.xcconfig; sourceTree = SOURCE_ROOT; };\n\t\tBBBBBBBBBBBBBBBBBBBBBBBB /* config.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/config.xcconfig; sourceTree = SOURCE_ROOT; };", 1)
	pbxproj = strings.Replace(pbxproj,
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA;",
		"999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB;", 1)
	if err := os.WriteFile(pbxprojPath, []byte(pbxproj), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	_, err = SetVersion(context.Background(), SetVersionOptions{
		ProjectDir: projectPath,
		Target:     "App",
		Version:    "2.0.0",
	})
	if err != nil {
		t.Fatalf("SetVersion() error = %v", err)
	}
	for _, path := range []string{firstPath, secondPath} {
		if got := mustReadVersionTestFile(t, path); !strings.Contains(got, "MARKETING_VERSION = 2.0.0") {
			t.Fatalf("case-distinct xcconfig %q was not updated: %q", path, got)
		}
	}
}

func TestStructuredVersion_RollbackPreservesConcurrentSave(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(filePath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileInfo := mustStatVersionTestFile(t, filePath)

	injectedErr := errors.New("injected receipt failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			if err := os.WriteFile(filePath, []byte("concurrent"), 0o644); err != nil {
				return nil, err
			}
			return nil, injectedErr
		}
		return originalCreator(write, data)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: filePath, root: fileRoot, name: filePath, original: []byte("old"), updated: []byte("new"), mode: 0o644, originalInfo: fileInfo},
		{path: receiptPath, root: receiptRoot, name: receiptPath, updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "concurrent change") {
		t.Fatalf("commitVersionWrites() error = %v, want receipt failure with preserved concurrent save", err)
	}
	if got := mustReadVersionTestFile(t, filePath); got != "concurrent" {
		t.Fatalf("concurrent save was overwritten during rollback: %q", got)
	}
	if _, statErr := os.Lstat(receiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after injected failure = %v, want absent", statErr)
	}
}

func TestStructuredVersion_RollbackDoesNotRestoreWhenWriteIdentityCaptureFails(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected identity capture failure")
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := originalWriter(write, data)
		if write.path == path && err == nil {
			return nil, injectedErr
		}
		return info, err
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: filepath.Base(path),
		original: []byte("old"), updated: []byte("new"), mode: 0o644,
		originalIdentity: originalIdentity, strictIdentity: true,
	}})
	if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "identity unavailable") {
		t.Fatalf("commitVersionWrites() error = %v, want identity uncertainty", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new" {
		t.Fatalf("published write after identity-retention failure = %q, want new", got)
	}
}

func TestStructuredVersion_RollbackUsesConditionalIdentityAndPreservesSameContentReplacement(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	replacementPath := filepath.Join(root, "replacement.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected receipt failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			return nil, injectedErr
		}
		return originalCreator(write, data)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })
	var committedInfo os.FileInfo
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalWriter(write, data)
		if err == nil && identity != nil && write.path == path {
			committedInfo = identity.Info()
		}
		return identity, err
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	var replacementInfo os.FileInfo
	originalHook := rollbackOrdinaryVersionBeforeConditionalWriteForTest
	rollbackOrdinaryVersionBeforeConditionalWriteForTest = func() {
		if err := os.WriteFile(replacementPath, []byte("new"), 0o640); err != nil {
			t.Fatalf("replace updated file with same-content replacement: %v", err)
		}
		var err error
		replacementInfo, err = os.Stat(replacementPath)
		if err != nil {
			t.Fatalf("Stat(same-content replacement) error = %v", err)
		}
		if err := os.Rename(replacementPath, path); err != nil {
			t.Fatalf("install same-content replacement: %v", err)
		}
	}
	t.Cleanup(func() { rollbackOrdinaryVersionBeforeConditionalWriteForTest = originalHook })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "concurrent change") {
		t.Fatalf("commitVersionWrites() error = %v, want receipt failure with conditional rollback uncertainty", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new" {
		t.Fatalf("same-content replacement was overwritten during conditional rollback: %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("replacement mode = %o, want preserved replacement mode 640", got)
	}
	if committedInfo == nil || os.SameFile(committedInfo, info) {
		t.Fatal("replacement destination still refers to the transaction write inode")
	}
	if replacementInfo == nil || !os.SameFile(replacementInfo, info) {
		t.Fatal("replacement destination does not retain the replacement inode")
	}
}

func TestStructuredVersion_RollbackCreatedFilePreservesDisappearance(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "receipt.json")
	contents := []byte("receipt")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	write := preparedVersionWrite{
		path: path, root: fileRoot, name: filepath.Base(path), updated: contents, createdIdentity: identity, createOnly: true,
	}
	removeCreatedVersionFileBeforeFinalCheckForTest = func() {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove created file in race hook: %v", err)
		}
	}
	t.Cleanup(func() { removeCreatedVersionFileBeforeFinalCheckForTest = nil })

	err = removeCreatedPreparedVersionFile(write)
	if err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("removeCreatedPreparedVersionFile() error = %v, want disappearance uncertainty", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("disappeared receipt after rollback = %v, want absent", statErr)
	}
}

func TestStructuredVersion_RollbackCreatedFilePreservesReplacement(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "receipt.json")
	replacementPath := filepath.Join(root, "replacement.json")
	contents := []byte("receipt")
	const replacement = "concurrent receipt wins"
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replacementPath, []byte(replacement), 0o600); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = fileRoot.Close() })
	write := preparedVersionWrite{
		path: path, root: fileRoot, name: filepath.Base(path), updated: contents, createdIdentity: identity, createOnly: true,
	}
	removeCreatedVersionFileBeforeFinalCheckForTest = func() {
		if err := os.Remove(path); err != nil {
			t.Fatalf("remove created file in race hook: %v", err)
		}
		if err := os.Rename(replacementPath, path); err != nil {
			t.Fatalf("install replacement in race hook: %v", err)
		}
	}
	t.Cleanup(func() { removeCreatedVersionFileBeforeFinalCheckForTest = nil })

	err = removeCreatedPreparedVersionFile(write)
	if err == nil || !strings.Contains(err.Error(), "file identity changed") {
		t.Fatalf("removeCreatedPreparedVersionFile() error = %v, want replacement uncertainty", err)
	}
	if got := mustReadVersionTestFile(t, path); got != replacement {
		t.Fatalf("replacement after rollback = %q, want preserved replacement", got)
	}
}

func TestStructuredVersion_CommitClosesPreparedRoots(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "version.xcconfig")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalInfo := mustStatVersionTestFile(t, path)

	if err := commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: path, original: []byte("old"), updated: []byte("new"), mode: 0o644, originalInfo: originalInfo,
	}}); err != nil {
		t.Fatal(err)
	}
	if opened, err := fileRoot.OpenRoot(); err == nil {
		_ = opened.Close()
		t.Fatal("commitVersionWrites() retained its pinned root after commit")
	}
}

func TestStructuredVersion_ContainedTargetsReuseProjectRoot(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "a.xcconfig")
	secondPath := filepath.Join(root, "b.xcconfig")
	for _, path := range []string{firstPath, secondPath} {
		if err := os.WriteFile(path, []byte("VERSION = 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	projectRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	projectRoot = projectRoot.AllowingInternalSymlinks()
	project := &structuredVersionProject{rootDir: root}
	first, err := project.versionFileTarget(projectRoot, firstPath, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := project.versionFileTarget(projectRoot, secondPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if first.ownsRoot || second.ownsRoot {
		t.Fatal("contained version targets unexpectedly allocated separate roots")
	}
	if err := first.root.Close(); err != nil {
		t.Fatal(err)
	}
	if opened, err := second.root.OpenRoot(); err == nil {
		_ = opened.Close()
		t.Fatal("contained version targets did not share the project root")
	}
}

func mustGetStructuredVersion(t *testing.T, project, target, configuration string) *VersionInfo {
	t.Helper()
	view, err := GetVersionScoped(context.Background(), GetVersionOptions{
		ProjectDir: project, Target: target, Configuration: configuration,
	})
	if err != nil {
		t.Fatalf("GetVersionScoped(%s/%s) error = %v", target, configuration, err)
	}
	return view
}

func containsPathSuffix(paths []string, suffix string) bool {
	for _, path := range paths {
		if strings.HasSuffix(filepath.ToSlash(path), suffix) {
			return true
		}
	}
	return false
}

func mustReadVersionTestFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(contents)
}

func mustStatVersionTestFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", path, err)
	}
	return info
}

func writeStructuredVersionProject(t *testing.T, xcconfigBacked bool) string {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}

	appSettings := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;"
	baseConfiguration := ""
	fileReference := ""
	if xcconfigBacked {
		appSettings = ""
		baseConfiguration = "baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA;"
		fileReference = `
		AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT; };`
		configDir := filepath.Join(root, "Configs")
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(config) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "App.xcconfig"), []byte("#include \"Shared.xcconfig\"\r\nOTHER_SETTING = YES\r\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
		}
		shared := "// leading comment\r\nMARKETING_VERSION = 1.2.3 // keep this comment\r\nCURRENT_PROJECT_VERSION = 42\r\nCURRENT_PROJECT_VERSION[sdk=iphoneos*] = 42\r\n/* MARKETING_VERSION = 8.8.8; */\r\n"
		if err := os.WriteFile(filepath.Join(configDir, "Shared.xcconfig"), []byte(shared), 0o640); err != nil {
			t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
		}
	}

	project := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {};
	objectVersion = 77;
	objects = {
` + fileReference + `
		111111111111111111111111 /* Project object */ = {
			isa = PBXProject;
			attributes = {};
			buildConfigurationList = 222222222222222222222222;
			targets = (
				333333333333333333333333,
				444444444444444444444444,
			);
		};
		333333333333333333333333 /* App */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = 555555555555555555555555;
			buildPhases = ();
			dependencies = ();
			name = App;
			productName = App;
			productReference = 666666666666666666666666;
			productType = "com.apple.product-type.application";
		};
		444444444444444444444444 /* Widget */ = {
			isa = PBXNativeTarget;
			buildConfigurationList = 777777777777777777777777;
			buildPhases = ();
			dependencies = ();
			name = Widget;
			productName = Widget;
			productReference = 888888888888888888888888;
			productType = "com.apple.product-type.app-extension";
		};
		666666666666666666666666 /* App.app */ = {isa = PBXFileReference; explicitFileType = wrapper.application; path = App.app; sourceTree = BUILT_PRODUCTS_DIR; };
		888888888888888888888888 /* Widget.appex */ = {isa = PBXFileReference; explicitFileType = "wrapper.app-extension"; path = Widget.appex; sourceTree = BUILT_PRODUCTS_DIR; };
		999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };
		999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };
		999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; ` + baseConfiguration + ` buildSettings = { ` + appSettings + ` }; name = Debug; };
		999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; ` + baseConfiguration + ` buildSettings = { ` + appSettings + ` }; name = Release; };
		999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };
		999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };
		222222222222222222222222 /* Project configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999991, 999999999999999999999992); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		555555555555555555555555 /* App configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999993, 999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		777777777777777777777777 /* Widget configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999995, 999999999999999999999996); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
	};
	rootObject = 111111111111111111111111 /* Project object */;
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "project.pbxproj"), []byte(project), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	return projectPath
}

// TestStructuredVersion_RollsBackWritePublishedBeforeError pins the caller-side
// path for a write that reports an error after publication. The rooted writer
// returns the installed identity from its publication operation, which lets the
// caller distinguish the transaction's replacement from a later pathname
// replacement during rollback.
func TestStructuredVersion_RollsBackWritePublishedBeforeError(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.xcconfig")
	if err := os.WriteFile(path, []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}

	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := atomicWritePreparedVersionFile(write, data)
		if err != nil {
			return info, err
		}
		if string(data) == "new-a" {
			return info, errors.New(`replacement succeeded but remove backup ".asc-backup-123": permission denied`)
		}
		return info, nil
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: path, original: []byte("old-a"), updated: []byte("new-a"), mode: 0o644, originalIdentity: originalIdentity, strictIdentity: true, portableFallback: true},
	})
	if err == nil {
		t.Fatal("commitVersionWrites() error = nil, want publication failure")
	}
	if got := mustReadVersionTestFile(t, path); got != "old-a" {
		t.Fatalf("write published before the error was not rolled back: %q", got)
	}
	// The backup path is known only to the rootfs primitive and reaches this
	// layer inside the error text, so it must stay visible to the operator.
	if !strings.Contains(err.Error(), "remove backup") {
		t.Fatalf("stranded backup file was not reported: %v", err)
	}
}

func TestStructuredVersion_PublishedWriteRollbackPortableFallbackClearsUnsupportedError(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	if err := os.WriteFile(path, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	committedInfo := mustStatVersionTestFile(t, path)

	originalRollback := rollbackPublishedVersionWriteInfoFn
	rollbackPublishedVersionWriteInfoFn = func(preparedVersionWrite) error {
		return secureopen.ErrRenameNoReplaceUnsupported
	}
	t.Cleanup(func() { rollbackPublishedVersionWriteInfoFn = originalRollback })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = rollbackPublishedVersionWriteAfterError(preparedVersionWrite{
		path:             path,
		root:             fileRoot,
		name:             filepath.Base(path),
		original:         []byte("old"),
		updated:          []byte("new"),
		mode:             0o640,
		committedInfo:    committedInfo,
		portableFallback: true,
	})
	if err != nil {
		t.Fatalf("rollbackPublishedVersionWriteAfterError() error = %v", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "old" {
		t.Fatalf("portable rollback contents = %q, want old", got)
	}
}

func TestStructuredVersion_PublishedWriteCleanupFailureLeavesNoReceipt(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected staged-file cleanup failure")
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := originalWriter(write, data)
		if err != nil {
			return info, err
		}
		if write.path == path {
			return info, injectedErr
		}
		return info, nil
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("commitVersionWrites() error = %v, want staged cleanup failure", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "old" {
		t.Fatalf("ordinary write was not rolled back: %q", got)
	}
	if _, statErr := os.Lstat(receiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after ordinary publication uncertainty = %v, want absent", statErr)
	}
}

func TestStructuredVersion_UnknownReceiptPublicationKeepsEarlierWrites(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected publication identity retention failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, _ []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			return nil, errors.Join(rootfs.ErrFilePublicationUncertain, injectedErr)
		}
		return originalCreator(write, write.updated)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !errors.Is(err, rootfs.ErrFilePublicationUncertain) {
		t.Fatalf("commitVersionWrites() error = %v, want publication uncertainty", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new" {
		t.Fatalf("publication uncertainty rolled back the earlier write: %q", got)
	}
}

func TestStructuredVersion_KnownAbsentReceiptRollsBackEarlierWrites(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected post-publication receipt failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return identity, err
		}
		return identity, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })
	removeCreatedVersionFileBeforeFinalCheckForTest = func() {
		if err := os.Remove(receiptPath); err != nil {
			t.Fatalf("remove receipt before identity cleanup: %v", err)
		}
	}
	t.Cleanup(func() { removeCreatedVersionFileBeforeFinalCheckForTest = nil })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !errors.Is(err, rootfs.ErrFileIdentityRemoved) {
		t.Fatalf("commitVersionWrites() error = %v, want known-absent receipt cleanup result", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "old" {
		t.Fatalf("known-absent receipt did not roll back the earlier write: %q", got)
	}
	if _, statErr := os.Lstat(receiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after known-absent cleanup = %v, want absent", statErr)
	}
}

func TestStructuredVersion_ReceiptCleanupReplacementKeepsEarlierWrites(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	replacementPath := filepath.Join(root, "replacement.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected post-publication receipt failure")
	var publishedReceiptInfo os.FileInfo
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return identity, err
		}
		publishedReceiptInfo = identity.Info()
		return identity, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	var replacementInfo os.FileInfo
	originalRemover := removeCreatedVersionFileFn
	removeCreatedVersionFileFn = func(write preparedVersionWrite) error {
		if err := os.WriteFile(replacementPath, write.updated, write.mode); err != nil {
			t.Fatalf("write replacement receipt: %v", err)
		}
		var err error
		replacementInfo, err = os.Stat(replacementPath)
		if err != nil {
			t.Fatalf("Stat(replacement receipt) error = %v", err)
		}
		if err := os.Rename(replacementPath, receiptPath); err != nil {
			t.Fatalf("install replacement receipt: %v", err)
		}
		return rootfs.ErrFileIdentityChanged
	}
	t.Cleanup(func() { removeCreatedVersionFileFn = originalRemover })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !errors.Is(err, rootfs.ErrFileIdentityChanged) || errors.Is(err, rootfs.ErrFileIdentityRemoved) {
		t.Fatalf("commitVersionWrites() error = %v, want surviving replacement receipt", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new" {
		t.Fatalf("surviving replacement receipt rolled back the earlier write: %q", got)
	}
	if got := mustReadVersionTestFile(t, receiptPath); got != "receipt" {
		t.Fatalf("replacement receipt contents = %q, want receipt", got)
	}
	finalReceiptInfo, err := os.Stat(receiptPath)
	if err != nil {
		t.Fatalf("Stat(replacement receipt destination) error = %v", err)
	}
	if publishedReceiptInfo == nil || os.SameFile(publishedReceiptInfo, finalReceiptInfo) {
		t.Fatal("replacement receipt destination still refers to the transaction receipt inode")
	}
	if replacementInfo == nil || !os.SameFile(replacementInfo, finalReceiptInfo) {
		t.Fatal("replacement receipt inode was not preserved")
	}
}

func TestStructuredVersion_QuarantineCleanupFailureAfterCanonicalAbsenceRollsBackEarlierWrites(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected post-publication receipt failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return identity, err
		}
		return identity, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	originalRemover := removeCreatedVersionFileFn
	removeCreatedVersionFileFn = func(write preparedVersionWrite) error {
		if err := os.Remove(write.path); err != nil {
			t.Fatalf("remove canonical receipt: %v", err)
		}
		quarantinePath := filepath.Join(root, ".asc-tmp-rollback-receipt")
		if err := os.WriteFile(quarantinePath, write.updated, 0o600); err != nil {
			t.Fatalf("preserve receipt quarantine: %v", err)
		}
		return errors.Join(
			rootfs.ErrQuarantineCleanupUncertain,
			rootfs.ErrFileIdentityRemoved,
			fmt.Errorf("quarantined file %q was left in place", quarantinePath),
		)
	}
	t.Cleanup(func() { removeCreatedVersionFileFn = originalRemover })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !errors.Is(err, rootfs.ErrQuarantineCleanupUncertain) || !errors.Is(err, rootfs.ErrFileIdentityRemoved) {
		t.Fatalf("commitVersionWrites() error = %v, want receipt cleanup failure with removed sentinel", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "old" {
		t.Fatalf("ordinary write after absent receipt cleanup failure = %q, want old", got)
	}
	if _, statErr := os.Lstat(receiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("canonical receipt after cleanup failure = %v, want absent", statErr)
	}
	if !strings.Contains(err.Error(), ".asc-tmp-rollback-receipt") {
		t.Fatalf("cleanup error = %v, want quarantine evidence", err)
	}
}

func TestStructuredVersion_QuarantineCleanupFailureWithReplacementKeepsEarlierWrites(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "settings.xcconfig")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected post-publication receipt failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return identity, err
		}
		return identity, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	originalRemover := removeCreatedVersionFileFn
	removeCreatedVersionFileFn = func(write preparedVersionWrite) error {
		if err := os.Remove(write.path); err != nil {
			t.Fatalf("remove transaction receipt: %v", err)
		}
		replacementPath := write.path + ".replacement"
		if err := os.WriteFile(replacementPath, []byte("concurrent receipt"), 0o600); err != nil {
			t.Fatalf("write replacement receipt: %v", err)
		}
		if err := os.Rename(replacementPath, write.path); err != nil {
			t.Fatalf("install replacement receipt: %v", err)
		}
		quarantinePath := filepath.Join(root, ".asc-tmp-rollback-receipt")
		if err := os.WriteFile(quarantinePath, write.updated, 0o600); err != nil {
			t.Fatalf("preserve receipt quarantine: %v", err)
		}
		return errors.Join(
			rootfs.ErrQuarantineCleanupUncertain,
			fmt.Errorf("quarantined file %q was left in place", quarantinePath),
		)
	}
	t.Cleanup(func() { removeCreatedVersionFileFn = originalRemover })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	receiptRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old"), updated: []byte("new"), mode: 0o640, preserveMetadata: true, originalIdentity: originalIdentity, strictIdentity: true},
		{path: receiptPath, root: receiptRoot, name: filepath.Base(receiptPath), updated: []byte("receipt"), mode: 0o600, createOnly: true},
	})
	if !errors.Is(err, injectedErr) || !errors.Is(err, rootfs.ErrQuarantineCleanupUncertain) || errors.Is(err, rootfs.ErrFileIdentityRemoved) {
		t.Fatalf("commitVersionWrites() error = %v, want surviving replacement without removed sentinel", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new" {
		t.Fatalf("ordinary write after replacement receipt cleanup failure = %q, want new", got)
	}
	if got := mustReadVersionTestFile(t, receiptPath); got != "concurrent receipt" {
		t.Fatalf("replacement receipt = %q, want concurrent receipt", got)
	}
	if got := mustReadVersionTestFile(t, filepath.Join(root, ".asc-tmp-rollback-receipt")); got != "receipt" {
		t.Fatalf("preserved receipt quarantine = %q, want transaction receipt", got)
	}
}

func TestStructuredVersion_PublishedWriteRollbackPreservesPostCaptureReplacement(t *testing.T) {
	requireStrictVersionMutationPlatform(t)
	root := t.TempDir()
	path := filepath.Join(root, "a.xcconfig")
	replacementPath := filepath.Join(root, "replacement.xcconfig")
	if err := os.WriteFile(path, []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("published write cleanup failure")
	var publishedInfo os.FileInfo
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := atomicWritePreparedVersionFile(write, data)
		if err != nil {
			return info, err
		}
		if string(data) == "new-a" {
			publishedInfo = info.Info()
			return info, injectedErr
		}
		return info, nil
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	var replacementInfo os.FileInfo
	originalHook := rollbackPublishedVersionBeforeConditionalWriteForTest
	rollbackPublishedVersionBeforeConditionalWriteForTest = func() {
		if err := os.WriteFile(replacementPath, []byte("new-a"), 0o644); err != nil {
			t.Fatalf("write replacement: %v", err)
		}
		var err error
		replacementInfo, err = os.Stat(replacementPath)
		if err != nil {
			t.Fatalf("Stat(replacement) error = %v", err)
		}
		if err := os.Rename(replacementPath, path); err != nil {
			t.Fatalf("install replacement: %v", err)
		}
	}
	t.Cleanup(func() { rollbackPublishedVersionBeforeConditionalWriteForTest = originalHook })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	originalIdentity, err := fileRoot.CaptureFile(filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{{
		path: path, root: fileRoot, name: filepath.Base(path), original: []byte("old-a"), updated: []byte("new-a"), mode: 0o644,
		originalIdentity: originalIdentity, strictIdentity: true,
	}})
	if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "identity changed") {
		t.Fatalf("commitVersionWrites() error = %v, want publication error with replacement uncertainty", err)
	}
	if got := mustReadVersionTestFile(t, path); got != "new-a" {
		t.Fatalf("post-capture replacement = %q, want preserved replacement", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("post-capture replacement mode = %o, want 644", got)
	}
	if publishedInfo == nil || os.SameFile(publishedInfo, info) {
		t.Fatal("post-capture replacement still refers to the transaction write inode")
	}
	if replacementInfo == nil || !os.SameFile(replacementInfo, info) {
		t.Fatal("post-capture replacement inode was not preserved")
	}
}

func TestStructuredVersion_LeavesUnpublishedFailedWriteAlone(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.xcconfig")
	if err := os.WriteFile(path, []byte("old-a"), 0o644); err != nil {
		t.Fatal(err)
	}
	originalInfo := mustStatVersionTestFile(t, path)

	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(preparedVersionWrite, []byte) (os.FileInfo, error) {
		return nil, errors.New("open destination: permission denied")
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{path: path, root: fileRoot, name: path, original: []byte("old-a"), updated: []byte("new-a"), mode: 0o644, originalInfo: originalInfo},
	})
	if err == nil {
		t.Fatal("commitVersionWrites() error = nil, want write failure")
	}
	if got := mustReadVersionTestFile(t, path); got != "old-a" {
		t.Fatalf("unpublished failure disturbed the destination: %q", got)
	}
	if strings.Contains(err.Error(), "restore") {
		t.Fatalf("unpublished failure attempted a rollback: %v", err)
	}
}

func TestStructuredVersion_RollsBackLargeXCConfigAfterLaterWriteFailure(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "a-large.xcconfig")
	secondPath := filepath.Join(root, "b.xcconfig")
	const (
		versionSetting   = "MARKETING_VERSION = 1.2.3\n"
		unrelatedSetting = "UNRELATED_SETTING = KEEP\n"
		paddingLine      = "// compatibility padding\n"
	)
	padding := strings.Repeat(paddingLine, signingPlanMaxBytes/len(paddingLine)+1)
	originalLarge := []byte(padding + versionSetting + unrelatedSetting)
	updatedLarge := bytes.Replace(originalLarge, []byte(versionSetting), []byte("MARKETING_VERSION = 2.0.0\n"), 1)
	if len(originalLarge) <= signingPlanMaxBytes {
		t.Fatalf("large xcconfig size = %d, want more than %d", len(originalLarge), signingPlanMaxBytes)
	}
	if err := os.WriteFile(firstPath, originalLarge, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secondPath, []byte("old-b"), 0o640); err != nil {
		t.Fatal(err)
	}
	firstInfo := mustStatVersionTestFile(t, firstPath)
	secondInfo := mustStatVersionTestFile(t, secondPath)

	injectedErr := errors.New("injected later write failure")
	originalWriter := atomicWriteVersionFileInfoFn
	atomicWriteVersionFileInfoFn = func(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
		if write.path == secondPath {
			return nil, injectedErr
		}
		return atomicWritePreparedVersionFileInfo(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileInfoFn = originalWriter })

	fileRoot, err := rootfs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	err = commitVersionWrites([]preparedVersionWrite{
		{
			path: firstPath, root: fileRoot, name: filepath.Base(firstPath),
			original: originalLarge, updated: updatedLarge, mode: 0o640, originalInfo: firstInfo,
		},
		{
			path: secondPath, root: fileRoot, name: filepath.Base(secondPath),
			original: []byte("old-b"), updated: []byte("new-b"), mode: 0o640, originalInfo: secondInfo,
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("commitVersionWrites() error = %v, want later write failure", err)
	}
	gotLarge := mustReadVersionTestFile(t, firstPath)
	if !bytes.Equal([]byte(gotLarge), originalLarge) {
		t.Fatalf("large xcconfig was not restored byte-for-byte after rollback")
	}
	if !strings.Contains(gotLarge, unrelatedSetting) {
		t.Fatalf("large xcconfig lost unrelated setting after rollback: %q", unrelatedSetting)
	}
	if got := mustReadVersionTestFile(t, secondPath); got != "old-b" {
		t.Fatalf("later failed file changed: %q", got)
	}
}
