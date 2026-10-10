package xcode

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildSigningPlanRejectsPlanAliasToXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	planPath := filepath.Join(root, "plan.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	if err := os.Link(sharedPath, planPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want plan alias rejection", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got == "" {
		t.Fatal("shared xcconfig unexpectedly disappeared")
	}
}

func TestBuildSigningPlanRejectsReceiptAliasToEntitlements(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	entitlementsPath := filepath.Join(projectRoot, "App.entitlements")
	if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?>\n"), 0o644); err != nil {
		t.Fatalf("write entitlements file: %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	receiptPath := filepath.Join(root, "receipt.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_ENTITLEMENTS":"App.entitlements"}}]}]
	}`)
	if err := os.Link(entitlementsPath, receiptPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		StateDir: filepath.Join(root, "state"), ReceiptPath: receiptPath,
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want receipt alias rejection", err)
	}
	if got := mustReadVersionTestFile(t, entitlementsPath); got == "" {
		t.Fatal("entitlements file unexpectedly disappeared")
	}
}

func TestBuildSigningPlanRejectsPlanAliasToExternalDirectEntitlements(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	externalPath := filepath.Join(t.TempDir(), "External.entitlements")
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	old := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;"
	replacement := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; CODE_SIGN_ENTITLEMENTS = \"" + externalPath + "\";"
	if !strings.Contains(contents, old) {
		t.Fatalf("project fixture is missing direct build settings")
	}
	contents = strings.Replace(contents, old, replacement, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: externalPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want external direct-entitlements collision", err)
	}
	if _, statErr := os.Lstat(externalPath); !os.IsNotExist(statErr) {
		t.Fatalf("external entitlement after collision = %v, want absent", statErr)
	}
}

func TestBuildSigningPlanRejectsPhysicalAliasToExternalDirectEntitlements(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	externalRoot := t.TempDir()
	externalPath := filepath.Join(externalRoot, "App.entitlements")
	const original = "external entitlements"
	if err := os.WriteFile(externalPath, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile(external entitlements) error = %v", err)
	}
	aliasDir := filepath.Join(projectRoot, "artifact-alias")
	if err := os.Symlink(externalRoot, aliasDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	planPath := filepath.Join(aliasDir, filepath.Base(externalPath))
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"), `CODE_SIGN_ENTITLEMENTS = "`+externalPath+`";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want physical external-entitlement alias rejection", err)
	}
	if got := mustReadVersionTestFile(t, externalPath); got != original {
		t.Fatalf("external entitlement changed after alias rejection: %q", got)
	}
}

func TestBuildSigningPlanRejectsMissingEntitlementArtifactCollision(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_ENTITLEMENTS":"plan.json"}}]}]
	}`)
	planPath := filepath.Join(filepath.Dir(project), "plan.json")
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Dir(project),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want missing-input collision", err)
	}
	if _, statErr := os.Lstat(planPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing entitlement artifact after collision = %v, want absent", statErr)
	}
}

func TestBuildSigningPlanRejectsNonStringEntitlementsBeforeArtifactWrite(t *testing.T) {
	for _, test := range []struct {
		name            string
		configurationID string
	}{
		{name: "selected", configurationID: "999999999999999999999993"},
		{name: "unselected", configurationID: "999999999999999999999995"},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			injectSigningBuildSetting(t, filepath.Join(project, "project.pbxproj"), test.configurationID, `CODE_SIGN_ENTITLEMENTS = ( "plan.json" );`)

			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			planPath := filepath.Join(filepath.Dir(project), "plan.json")
			const existingPlan = "existing plan bytes\n"
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
				t.Fatalf("WriteFile(existing plan) error = %v", err)
			}

			_, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: planPath, StateDir: filepath.Join(root, "state"),
			})
			if err == nil || !strings.Contains(err.Error(), "non-string CODE_SIGN_ENTITLEMENTS") {
				t.Fatalf("BuildSigningPlan() error = %v, want non-string entitlement rejection", err)
			}
			if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
				t.Fatalf("existing plan changed after non-string entitlement rejection: %q", got)
			}
		})
	}
}

func TestValidateSigningArtifactAliasesRejectsMissingInputThroughSymlinkedArtifactParent(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	linkedParent := filepath.Join(root, "linked")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatalf("Mkdir(real parent) error = %v", err)
	}
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	protectedPath := filepath.Join(realParent, "protected.json")
	planPath := filepath.Join(linkedParent, "protected.json")
	receiptPath := filepath.Join(root, "receipt.json")

	err := validateSigningArtifactAliases(planPath, receiptPath, nil, []string{protectedPath})
	if err == nil || !strings.Contains(err.Error(), "aliases protected project input") {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want prospective alias rejection", err)
	}
	if _, statErr := os.Lstat(protectedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("protected path after alias validation = %v, want absent", statErr)
	}
}

func TestBuildSigningPlanRejectsMissingOptionalIncludeThroughSymlinkedArtifactParent(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	realParent := filepath.Join(projectRoot, "real")
	linkedParent := filepath.Join(projectRoot, "linked")
	if err := os.Mkdir(realParent, 0o755); err != nil {
		t.Fatalf("Mkdir(real parent) error = %v", err)
	}
	if err := os.Symlink(realParent, linkedParent); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	appXCConfig := filepath.Join(projectRoot, "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = "#include? \"../real/protected.json\"\n" + contents
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(linkedParent, "protected.json")
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want missing optional-include alias rejection", err)
	}
	if _, statErr := os.Lstat(filepath.Join(realParent, "protected.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("protected path after planning = %v, want absent", statErr)
	}
}

func TestBuildSigningPlanRejectsUnprotectableTraversalEntitlement(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	old := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;"
	replacement := old + ` CODE_SIGN_ENTITLEMENTS = "../plan.json";`
	if !strings.Contains(contents, old) {
		t.Fatalf("project fixture is missing direct build settings")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, old, replacement, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(filepath.Dir(filepath.Dir(project)), "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("write existing plan error = %v", err)
	}

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() error = %v, want fail-closed traversal protection", err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after unprotectable entitlement: %q", got)
	}
}

func TestBuildSigningPlanFailsClosedOnArtifactInspectionWithInputBlocker(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual","CODE_SIGN_ENTITLEMENTS":"Missing.entitlements"}}]}]
	}`)
	stateDir := filepath.Join(root, "state")
	missingPath := filepath.Join(filepath.Dir(project), "Missing.entitlements")
	injected := errors.New("injected artifact inspection failure")
	previousInfo := signingArtifactPathInfoFn
	signingArtifactPathInfoFn = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == missingPath {
			return nil, injected
		}
		return previousInfo(path)
	}
	t.Cleanup(func() { signingArtifactPathInfoFn = previousInfo })

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if plan != nil {
		t.Fatalf("BuildSigningPlan() returned blocked plan after alias inspection failure: %#v", plan)
	}
	if err == nil || !errors.Is(err, injected) || !strings.Contains(err.Error(), "inspect project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want injected alias inspection failure", err)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after alias inspection failure = %v, want absent", statErr)
	}
}

func TestBuildSigningPlanRejectsResolvedEntitlementArtifactCollision(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	old := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42;"
	replacement := "MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; CODE_SIGN_ENTITLEMENTS = \"$(ENTITLEMENTS_FILE)\"; ENTITLEMENTS_FILE = \"plan.json\";"
	if !strings.Contains(contents, old) {
		t.Fatalf("project fixture is missing direct build settings")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, old, replacement, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(filepath.Dir(project), "plan.json")
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Dir(project),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want resolved-input collision", err)
	}
	if _, statErr := os.Lstat(planPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("resolved entitlement artifact after collision = %v, want absent", statErr)
	}
}

func TestValidateSigningArtifactAliasesRejectsSymlinkArtifact(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "unrelated.json")
	planPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(target, []byte("unrelated"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, planPath); err != nil {
		t.Fatalf("symlink plan: %v", err)
	}

	if err := validateSigningArtifactAliases(planPath, receiptPath, nil, nil); err == nil {
		t.Fatal("validateSigningArtifactAliases() accepted a symlinked artifact")
	}
	if got := mustReadVersionTestFile(t, target); got != "unrelated" {
		t.Fatalf("symlink target changed: %q", got)
	}
}

func TestValidateSigningArtifactAliasesRejectsSymlinkInput(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "input-source.txt")
	inputPath := filepath.Join(root, "input.txt")
	planPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := os.WriteFile(target, []byte("source"), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	if err := os.Symlink(target, inputPath); err != nil {
		t.Fatalf("symlink input: %v", err)
	}

	if err := validateSigningArtifactAliases(planPath, receiptPath, []string{inputPath}, nil); err == nil {
		t.Fatal("validateSigningArtifactAliases() accepted a symlinked input")
	}
	if got := mustReadVersionTestFile(t, target); got != "source" {
		t.Fatalf("symlink target changed: %q", got)
	}
}

func TestValidateSigningArtifactAliasesUsesWindowsCaseInsensitiveLexicalProtection(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	planPath := filepath.Join(root, "Plan.JSON")
	protectedPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	err := validateSigningArtifactAliases(planPath, receiptPath, nil, []string{protectedPath})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want case-insensitive lexical collision", err)
	}
}

func TestValidateSigningArtifactAliasesRejectsMissingCaseVariantOnInsensitiveVolume(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires Darwin filesystem semantics")
	}
	root := t.TempDir()
	caseInsensitive, known := signingCaseInsensitiveVolumeFor(root)
	if known && !caseInsensitive {
		t.Skip("test volume is genuinely case-sensitive")
	}

	planPath := filepath.Join(root, "Plan.JSON")
	inputPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := validateSigningArtifactAliases(planPath, receiptPath, []string{inputPath}, nil); err == nil {
		t.Fatal("validateSigningArtifactAliases() accepted missing case-variant input")
	} else if !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want project-input alias rejection", err)
	}
	for _, path := range []string{planPath, inputPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Lstat(%q) error = %v, want path to remain missing", path, err)
		}
	}
}

func TestValidateSigningArtifactAliasesRejectsMissingSymlinkParentCollision(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("mkdir real directory: %v", err)
	}
	symlinkDir := filepath.Join(root, "alias")
	if err := os.Symlink(realDir, symlinkDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	inputPath := filepath.Join(realDir, "future.entitlements")
	planPath := filepath.Join(symlinkDir, "future.entitlements")
	receiptPath := filepath.Join(root, "receipt.json")
	if err := validateSigningArtifactAliases(planPath, receiptPath, []string{inputPath}, nil); err == nil {
		t.Fatal("validateSigningArtifactAliases() accepted missing symlink-parent alias")
	} else if !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want project-input alias rejection", err)
	}
	for _, path := range []string{inputPath, planPath} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Lstat(%q) error = %v, want path to remain missing", path, err)
		}
	}
}

func TestSigningProjectInputPathsFailsClosedWhenConfigDisappears(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	missingPath := filepath.Join(filepath.Dir(project), "Missing.xcconfig")

	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	_, _, _, err = signingProjectInputPaths(
		structured,
		settingsPath,
		map[string][]string{"configuration": {missingPath}},
		nil,
		nil,
		false,
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "read xcconfig") {
		t.Fatalf("signingProjectInputPaths() error = %v, want missing-source failure", err)
	}
}

func TestValidateSigningRelativePathUsesPOSIXRules(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "single component", value: "App.entitlements", valid: true},
		{name: "nested components", value: "Config/App.entitlements", valid: true},
		{name: "parent traversal", value: "../App.entitlements"},
		{name: "embedded traversal", value: "Config/../App.entitlements"},
		{name: "dot component", value: "Config/./App.entitlements"},
		{name: "backslash separator", value: `Config\App.entitlements`},
		{name: "unix absolute", value: "/tmp/App.entitlements"},
		{name: "windows drive relative", value: "C:App.entitlements"},
		{name: "windows drive absolute", value: "C:/App.entitlements"},
		{name: "home shorthand", value: "~/App.entitlements"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateSigningRelativePath(test.value)
			if test.valid && err != nil {
				t.Fatalf("validateSigningRelativePath(%q) error = %v", test.value, err)
			}
			if !test.valid && err == nil {
				t.Fatalf("validateSigningRelativePath(%q) unexpectedly accepted", test.value)
			}
		})
	}
}

func TestSigningApplyRejectsSourceChangedAfterPreparation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	originalHook := beforeSigningCommitForTest
	beforeSigningCommitForTest = func() {
		beforeSigningCommitForTest = nil
		contents := mustReadVersionTestFile(t, pbxprojPath)
		if err := os.WriteFile(pbxprojPath, append([]byte(contents), '\n'), 0o644); err != nil {
			t.Fatalf("mutate project after preparation: %v", err)
		}
	}
	t.Cleanup(func() { beforeSigningCommitForTest = originalHook })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApplySigningPlan() error = %v, want stale-source rejection", err)
	}
	if strings.Contains(mustReadVersionTestFile(t, pbxprojPath), `CODE_SIGN_STYLE = Manual;`) {
		t.Fatal("source drift after preparation was applied")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !os.IsNotExist(statErr) {
		t.Fatalf("receipt after source drift = %v, want absent", statErr)
	}
}

func TestValidateSigningArtifactAliasesRejectsProtectedSymlinkToArtifact(t *testing.T) {
	for _, label := range []string{"plan", "receipt"} {
		t.Run(label, func(t *testing.T) {
			root := t.TempDir()
			planPath := filepath.Join(root, "plan.json")
			receiptPath := filepath.Join(root, "receipt.json")
			target := planPath
			if label == "receipt" {
				target = receiptPath
			}
			const original = "existing artifact bytes\n"
			if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", label, err)
			}
			protectedPath := filepath.Join(t.TempDir(), "External.entitlements")
			if err := os.Symlink(target, protectedPath); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}

			err := validateSigningArtifactAliases(planPath, receiptPath, nil, []string{protectedPath})
			if err == nil || !strings.Contains(err.Error(), "is a symlink and cannot be aliased by an artifact") {
				t.Fatalf("validateSigningArtifactAliases() error = %v, want protected symlink alias rejection", err)
			}
			var aliasErr signingArtifactAliasError
			if !errors.As(err, &aliasErr) {
				t.Fatalf("validateSigningArtifactAliases() error = %v, want signingArtifactAliasError classification", err)
			}
			if got := mustReadVersionTestFile(t, target); got != original {
				t.Fatalf("%s changed after protected symlink rejection: %q", label, got)
			}
		})
	}
}

func TestValidateSigningArtifactAliasesSurfacesProtectedInspectionIOError(t *testing.T) {
	root := t.TempDir()
	planPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	protectedPath := filepath.Join(t.TempDir(), "External.entitlements")
	injected := errors.New("injected protected inspection failure")
	previousInfo := signingArtifactPathInfoFn
	signingArtifactPathInfoFn = func(path string) (os.FileInfo, error) {
		if filepath.Clean(path) == filepath.Clean(protectedPath) {
			return nil, injected
		}
		return previousInfo(path)
	}
	t.Cleanup(func() { signingArtifactPathInfoFn = previousInfo })

	err := validateSigningArtifactAliases(planPath, receiptPath, nil, []string{protectedPath})
	if err == nil || !errors.Is(err, injected) {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want injected protected inspection failure", err)
	}
	if !strings.Contains(err.Error(), "inspect protected project input") {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want protected inspection context", err)
	}
	var aliasErr signingArtifactAliasError
	if errors.As(err, &aliasErr) {
		t.Fatalf("validateSigningArtifactAliases() error = %v, want an I/O failure rather than an alias classification", err)
	}
}

func TestBuildSigningPlanRejectsExternalEntitlementSymlinkToArtifact(t *testing.T) {
	for _, label := range []string{"plan", "receipt"} {
		t.Run(label, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			planPath := filepath.Join(root, "plan.json")
			receiptPath := filepath.Join(root, "receipt.json")
			target := planPath
			if label == "receipt" {
				target = receiptPath
			}
			const original = "existing artifact bytes\n"
			if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", label, err)
			}
			externalPath := filepath.Join(t.TempDir(), "External.entitlements")
			if err := os.Symlink(target, externalPath); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"), `CODE_SIGN_ENTITLEMENTS = "`+externalPath+`";`)
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: planPath, ReceiptPath: receiptPath,
				StateDir: filepath.Join(root, "state"),
			})
			if plan != nil {
				t.Fatalf("BuildSigningPlan() returned a publishable plan for a symlinked protected input: %#v", plan)
			}
			if err == nil || !strings.Contains(err.Error(), "is a symlink and cannot be aliased by an artifact") {
				t.Fatalf("BuildSigningPlan() error = %v, want protected symlink alias rejection", err)
			}
			if got := mustReadVersionTestFile(t, target); got != original {
				t.Fatalf("%s changed after protected symlink rejection: %q", label, got)
			}
		})
	}
}
