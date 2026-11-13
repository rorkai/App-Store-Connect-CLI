package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const signingManualStyle = `{"CODE_SIGN_STYLE":"manual"}`

// signingSettingsFixture is a settings manifest written next to the state
// directories of the plans built from it.
type signingSettingsFixture struct {
	project      string
	root         string
	settingsPath string
}

type signingFixtureManifest struct {
	target         string
	configurations []string
	raw            string
}

type signingSettingsOption func(*signingFixtureManifest)

func withWidgetSigningTarget() signingSettingsOption {
	return func(manifest *signingFixtureManifest) { manifest.target = "Widget" }
}

// withSigningConfiguration adds a configuration whose settings are a JSON
// object; the first one replaces the default manual Debug configuration.
func withSigningConfiguration(name, settings string) signingSettingsOption {
	return func(manifest *signingFixtureManifest) {
		manifest.configurations = append(manifest.configurations, fmt.Sprintf(`{"name":%q,"settings":%s}`, name, settings))
	}
}

func withSigningManifest(raw string) signingSettingsOption {
	return func(manifest *signingFixtureManifest) { manifest.raw = raw }
}

// newSigningSettingsFixture writes a schema 1 manifest for project. Without
// options it selects App Debug with a manual signing style.
func newSigningSettingsFixture(t *testing.T, project string, options ...signingSettingsOption) signingSettingsFixture {
	t.Helper()
	manifest := signingFixtureManifest{target: "App"}
	for _, option := range options {
		option(&manifest)
	}
	contents := manifest.raw
	if contents == "" {
		configurations := manifest.configurations
		if len(configurations) == 0 {
			configurations = []string{fmt.Sprintf(`{"name":"Debug","settings":%s}`, signingManualStyle)}
		}
		contents = fmt.Sprintf(`{"schemaVersion":1,"targets":[{"name":%q,"configurations":[%s]}]}`, manifest.target, strings.Join(configurations, ","))
	}
	root := t.TempDir()
	fixture := signingSettingsFixture{project: project, root: root, settingsPath: filepath.Join(root, "settings.json")}
	writeSigningSettingsTestFile(t, fixture.settingsPath, contents)
	return fixture
}

func (fixture signingSettingsFixture) planOptions(state string) SigningPlanOptions {
	return SigningPlanOptions{
		ProjectPath:      fixture.project,
		SettingsFilePath: fixture.settingsPath,
		StateDir:         filepath.Join(fixture.root, state),
	}
}

func (fixture signingSettingsFixture) buildPlan(t *testing.T) *SigningPlan {
	t.Helper()
	plan, err := BuildSigningPlan(fixture.planOptions("state"))
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	return plan
}

func (fixture signingSettingsFixture) writePlan(t *testing.T) *SigningPlan {
	t.Helper()
	plan := fixture.buildPlan(t)
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	return plan
}

// signingFileSnapshot records the contents of each path, or that it is
// absent, so a refused plan or apply can be shown to leave every input alone.
type signingFileSnapshot map[string]*string

func snapshotSigningFiles(t *testing.T, paths ...string) signingFileSnapshot {
	t.Helper()
	snapshot := make(signingFileSnapshot, len(paths))
	for _, path := range paths {
		snapshot[path] = readSigningSnapshotFile(t, path)
	}
	return snapshot
}

func (snapshot signingFileSnapshot) requireUnchanged(t *testing.T) {
	t.Helper()
	for path, before := range snapshot {
		after := readSigningSnapshotFile(t, path)
		switch {
		case before == nil && after != nil:
			t.Fatalf("%s was created: %q", path, *after)
		case before != nil && after == nil:
			t.Fatalf("%s was removed", path)
		case before != nil && *after != *before:
			t.Fatalf("%s changed: %q, want %q", path, *after, *before)
		}
	}
}

func readSigningSnapshotFile(t *testing.T, path string) *string {
	t.Helper()
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		t.Fatalf("Lstat(%s) error = %v", path, err)
	}
	contents := mustReadVersionTestFile(t, path)
	return &contents
}

func requireSigningErrorContains(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one containing %q", wants)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to contain %q", err, want)
		}
	}
}

// requireSigningPlanRefused builds a plan that must fail with an error
// containing want and must leave every path in untouched as it was.
func requireSigningPlanRefused(t *testing.T, options SigningPlanOptions, want string, untouched ...string) *SigningPlan {
	t.Helper()
	snapshot := snapshotSigningFiles(t, untouched...)
	plan, err := BuildSigningPlan(options)
	requireSigningErrorContains(t, err, want)
	snapshot.requireUnchanged(t)
	return plan
}

// runSigningArtifactPathCases runs check once with artifactPath as the plan
// path and once with it as the receipt path.
func runSigningArtifactPathCases(t *testing.T, project, artifactPath string, check func(*testing.T, SigningPlanOptions)) {
	t.Helper()
	for _, artifact := range []struct {
		name string
		set  func(*SigningPlanOptions)
	}{
		{name: "plan path", set: func(options *SigningPlanOptions) { options.PlanPath = artifactPath }},
		{name: "receipt path", set: func(options *SigningPlanOptions) { options.ReceiptPath = artifactPath }},
	} {
		t.Run(artifact.name, func(t *testing.T) {
			options := newSigningSettingsFixture(t, project).planOptions("state")
			artifact.set(&options)
			check(t, options)
		})
	}
}

// signingChangesBySetting indexes changes by setting, keeping the last change
// for a setting that appears more than once.
func signingChangesBySetting(changes []SigningSettingChange) map[string]*SigningSettingChange {
	bySetting := make(map[string]*SigningSettingChange, len(changes))
	for index := range changes {
		bySetting[changes[index].Setting] = &changes[index]
	}
	return bySetting
}

// requireSigningApplyRefused applies a plan that must fail with an error
// containing want and must leave every path in untouched as it was.
func requireSigningApplyRefused(t *testing.T, options SigningApplyOptions, want string, untouched ...string) {
	t.Helper()
	snapshot := snapshotSigningFiles(t, untouched...)
	_, err := ApplySigningPlan(options)
	requireSigningErrorContains(t, err, want)
	snapshot.requireUnchanged(t)
}

func applySigningTestPlan(t *testing.T, plan *SigningPlan) *SigningApplyResult {
	t.Helper()
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	return result
}

// openSigningTestDebugConfiguration opens the App target's Debug configuration.
func openSigningTestDebugConfiguration(t *testing.T, projectPath string) (*structuredVersionProject, *versionConfiguration) {
	t.Helper()
	project, err := openStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	configuration, err := signingConfigurationFor(project, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor(App, Debug) error = %v", err)
	}
	return project, configuration
}

// setSigningTestPlatform makes signing code see goos and, when
// caseInsensitive is not nil, that volume case model until the test ends.
func setSigningTestPlatform(t *testing.T, goos string, caseInsensitive func(string) (bool, bool)) {
	t.Helper()
	previousOS, previousCaseInsensitive := runtimeGOOS, signingCaseInsensitiveVolumeFn
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseInsensitive
	})
	runtimeGOOS = goos
	if caseInsensitive != nil {
		signingCaseInsensitiveVolumeFn = caseInsensitive
	}
}

// signingCaseVariantName flips the case of the first letter of name and
// skips the test when that character has no case.
func signingCaseVariantName(t *testing.T, name string) string {
	t.Helper()
	variant := strings.ToUpper(name[:1]) + name[1:]
	if variant == name {
		variant = strings.ToLower(name[:1]) + name[1:]
	}
	if variant == name {
		t.Skipf("%q has no case variant", name)
	}
	return variant
}

// includeMissingOptionalSigningXCConfig makes App.xcconfig include an absent
// Missing.xcconfig with #include? and returns the App.xcconfig path.
func includeMissingOptionalSigningXCConfig(t *testing.T, project string) string {
	t.Helper()
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = strings.Replace(contents, `#include "Shared.xcconfig"`, "#include? \"Missing.xcconfig\"\n#include \"Shared.xcconfig\"", 1)
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}
	return appXCConfig
}
