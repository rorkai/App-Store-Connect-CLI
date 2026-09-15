package xcode

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

func requireStrictSigningPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("strict signing mutation fails closed before publication on Windows")
	}
}

func TestBuildAndApplySigningPlanForDirectSettings(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, true)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{
			"name": "App",
			"configurations": [{
				"name": "Debug",
				"settings": {
					"CODE_SIGN_STYLE": "manual",
					"DEVELOPMENT_TEAM": "ABCDE12345",
					"CODE_SIGN_IDENTITY": "Apple Development",
					"PROVISIONING_PROFILE": "01234567-89ab-cdef-0123-456789abcdef",
					"PRODUCT_BUNDLE_IDENTIFIER": "com.example.demo"
				}
			}]
		}]
	}`)

	pbxprojPath := filepath.Join(project, "project.pbxproj")
	before := mustReadVersionTestFile(t, pbxprojPath)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:      project,
		SettingsFilePath: settingsPath,
		StateDir:         stateDir,
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 5 {
		t.Fatalf("expected five changes, got %#v", plan.Changes)
	}
	for _, change := range plan.Changes {
		if change.Resolution != "missing" {
			t.Fatalf("new direct setting resolution = %q, want missing", change.Resolution)
		}
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != before {
		t.Fatal("planning changed the project")
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != pbxprojPath {
		t.Fatalf("unexpected changed files: %#v", result.ChangedFiles)
	}
	updated := mustReadVersionTestFile(t, pbxprojPath)
	for _, expected := range []string{
		`"CODE_SIGN_STYLE" = Manual;`,
		`"DEVELOPMENT_TEAM" = ABCDE12345;`,
		`"CODE_SIGN_IDENTITY" = "Apple Development";`,
		`"PROVISIONING_PROFILE" = "01234567-89ab-cdef-0123-456789abcdef";`,
		`"PRODUCT_BUNDLE_IDENTIFIER" = "com.example.demo";`,
	} {
		if !strings.Contains(updated, expected) {
			t.Fatalf("applied project is missing %q: %s", expected, updated)
		}
	}
	receiptInfo, err := os.Stat(plan.ReceiptPath)
	if err != nil {
		t.Fatalf("Stat(receipt) error = %v", err)
	}
	if receiptInfo.Mode().Perm() != 0o600 {
		t.Fatalf("receipt mode = %o, want 600", receiptInfo.Mode().Perm())
	}
	receipt, err := readSigningRegularFile(plan.ReceiptPath, signingPlanMaxBytes)
	if err != nil {
		t.Fatalf("read receipt error = %v", err)
	}
	if !strings.Contains(string(receipt), `"beforeSha256"`) || !strings.Contains(string(receipt), `"afterSha256"`) {
		t.Fatalf("receipt does not contain per-file digests: %s", receipt)
	}
	if !strings.Contains(string(receipt), `"completed": true`) || !result.Completed {
		t.Fatalf("receipt completion state = result=%t contents=%s, want completed", result.Completed, receipt)
	}
}

func TestSigningSettingResolverUsesImplicitBaseForPBXInherited(t *testing.T) {
	root := t.TempDir()
	configuration := &versionConfiguration{
		target: "App",
		name:   "Debug",
		buildSettings: serialized.Object{
			"PROJECT_DIR": "$(inherited)/Sub",
		},
	}
	project := &structuredVersionProject{
		rootDir:        root,
		configurations: []*versionConfiguration{configuration},
	}
	resolver := newSigningSettingResolver(project, nil, false, nil)
	got, _, err := resolver.resolveSettingReferenceWithContext(
		configuration,
		configuration,
		"PROJECT_DIR",
		map[string]bool{"PROJECT_DIR": true},
	)
	if err != nil {
		t.Fatalf("resolveSettingReferenceWithContext() error = %v", err)
	}
	if want := filepath.Join(root, "Sub"); got != want {
		t.Fatalf("PROJECT_DIR = %q, want implicit base %q", got, want)
	}
}

func TestBuildSigningPlanRejectsOversizedXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	original := mustReadVersionTestFile(t, sharedPath)
	padding := strings.Repeat("// signing compatibility padding\n", signingPlanMaxBytes/len("// signing compatibility padding\n")+1)
	if err := os.WriteFile(sharedPath, []byte(padding+original), 0o640); err != nil {
		t.Fatalf("WriteFile(large Shared.xcconfig) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	// An internal xcconfig that cannot be fully read is not merely a blocker:
	// unread assignments may name an artifact path, so the plan is refused
	// outright rather than serialized in a blocked state.
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), signingIncompleteInternalXCConfigMessage) {
		t.Fatalf("BuildSigningPlan() error = %v, want incomplete internal xcconfig rejection", err)
	}
	if plan != nil {
		t.Fatalf("BuildSigningPlan() = %#v, want no plan for an unreadable internal xcconfig", plan)
	}
}

func TestBuildSigningPlanRejectsOversizedUnselectedXCConfigBeforeArtifactPublication(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	configDir := filepath.Join(projectRoot, "Configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	const graphRoot = "Graph-0000.xcconfig"
	for index := 0; index < signingPlanMaxFiles; index++ {
		name := fmt.Sprintf("Graph-%04d.xcconfig", index)
		contents := "CODE_SIGN_STYLE = Manual\n"
		if index+1 < signingPlanMaxFiles {
			contents = fmt.Sprintf("#include \"Graph-%04d.xcconfig\"\n", index+1)
		}
		if err := os.WriteFile(filepath.Join(configDir, name), []byte(contents), 0o640); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	attachSigningWidgetXCConfig(t, project, fmt.Sprintf("#include \"%s\"\n", graphRoot))

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(state) error = %v", err)
	}
	planPath := filepath.Join(stateDir, "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: stateDir,
	})
	if err == nil || !isXCConfigSourceGraphLimitError(err) || !strings.Contains(err.Error(), "more than 4096 files") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want unselected source-graph limit", plan, err)
	}
	if plan != nil {
		t.Fatalf("BuildSigningPlan() returned a plan after unselected graph overflow: %#v", plan)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after unselected graph overflow: %q", got)
	}
}

func TestBuildSigningPlanSharesXCConfigBudgetAcrossConfigurationRoots(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a 4,096-file xcconfig graph")
	}

	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	configDir := filepath.Join(projectRoot, "Configs")
	const graphSize = signingPlanMaxFiles / 2
	writeGraph := func(name string) {
		dir := filepath.Join(configDir, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", name, err)
		}
		for index := 0; index < graphSize; index++ {
			fileName := fmt.Sprintf("Graph-%04d.xcconfig", index)
			contents := "CODE_SIGN_STYLE = Manual\n"
			if index+1 < graphSize {
				contents = fmt.Sprintf("#include \"Graph-%04d.xcconfig\"\n", index+1)
			}
			if err := os.WriteFile(filepath.Join(dir, fileName), []byte(contents), 0o640); err != nil {
				t.Fatalf("WriteFile(%s/%s) error = %v", name, fileName, err)
			}
		}
	}
	writeGraph("AppGraph")
	writeGraph("WidgetGraph")
	attachSigningAppXCConfig(t, project, "#include \"AppGraph/Graph-0000.xcconfig\"\n")
	attachSigningWidgetXCConfig(t, project, "#include \"WidgetGraph/Graph-0000.xcconfig\"\n")

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(root, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatalf("MkdirAll(state) error = %v", err)
	}
	planPath := filepath.Join(stateDir, "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: stateDir,
	})
	if err == nil || !isXCConfigSourceGraphLimitError(err) || !strings.Contains(err.Error(), "more than 4096 files") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want shared multi-root source-graph limit", plan, err)
	}
	if plan != nil {
		t.Fatalf("BuildSigningPlan() returned a plan after shared graph overflow: %#v", plan)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after shared graph overflow: %q", got)
	}
}

func TestSigningApplyDoesNotUseStablePortableWriteFallback(t *testing.T) {
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
	before := mustReadVersionTestFile(t, filepath.Join(project, "project.pbxproj"))
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(preparedVersionWrite, []byte) (*rootfs.FileIdentity, error) {
		return nil, secureopen.ErrRenameNoReplaceUnsupported
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) {
		t.Fatalf("ApplySigningPlan() error = %v, want signing transaction refusal", err)
	}
	if after := mustReadVersionTestFile(t, filepath.Join(project, "project.pbxproj")); after != before {
		t.Fatal("failed signing transaction modified the project")
	}
}

func TestSigningApplyUnsupportedOnWindowsLeavesProjectUntouched(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("strict signing apply is unsupported only on Windows")
	}
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
	before := mustReadVersionTestFile(t, pbxprojPath)

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if !errors.Is(err, rootfs.ErrFileIdentityMutationUnsupported) {
		t.Fatalf("ApplySigningPlan() error = %v, want ErrFileIdentityMutationUnsupported", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != before {
		t.Fatal("unsupported signing apply modified the project")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after unsupported signing apply = %v, want absent", statErr)
	}
}

func TestSigningPlanWritesCompletedNoOpReceipt(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, false)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	firstPlan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "first"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan(first) error = %v", err)
	}
	if err := WriteSigningPlanArtifact(firstPlan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact(first) error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: firstPlan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan(first) error = %v", err)
	}

	noOpPlan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "no-op"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan(no-op) error = %v", err)
	}
	if !noOpPlan.Ready || len(noOpPlan.Changes) != 0 {
		t.Fatalf("no-op plan = ready=%t changes=%#v blockers=%#v", noOpPlan.Ready, noOpPlan.Changes, noOpPlan.Blockers)
	}
	if err := WriteSigningPlanArtifact(noOpPlan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact(no-op) error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: noOpPlan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan(no-op) error = %v", err)
	}
	if !result.Completed || len(result.ChangedFiles) != 0 || len(result.Files) != 0 {
		t.Fatalf("no-op receipt result = %#v, want completed with no changed files", result)
	}
	receipt, err := readSigningRegularFile(noOpPlan.ReceiptPath, signingPlanMaxBytes)
	if err != nil {
		t.Fatalf("read no-op receipt error = %v", err)
	}
	if !strings.Contains(string(receipt), `"completed": true`) {
		t.Fatalf("no-op receipt = %s, want completed", receipt)
	}
}

func TestSigningApplyRefusesExistingReceiptBeforeMutation(t *testing.T) {
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
	const existingReceipt = "existing receipt must remain unchanged\n"
	if err := os.WriteFile(plan.ReceiptPath, []byte(existingReceipt), 0o600); err != nil {
		t.Fatalf("WriteFile(receipt) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("ApplySigningPlan() error = %v, want existing-receipt rejection", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("existing receipt rejection mutated the project")
	}
	if after := mustReadVersionTestFile(t, plan.ReceiptPath); after != existingReceipt {
		t.Fatalf("existing receipt changed: %q", after)
	}
}

func TestSigningApplyKeepsCompletedReceiptWhenRetainedCloseFails(t *testing.T) {
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

	injected := errors.New("injected retained close failure")
	originalClose := closeVersionWritesFn
	closeVersionWritesFn = func(writes []preparedVersionWrite) error {
		if err := originalClose(writes); err != nil {
			return errors.Join(err, injected)
		}
		return injected
	}
	t.Cleanup(func() { closeVersionWritesFn = originalClose })

	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v, want success when only post-publish close fails", err)
	}
	if result == nil || !result.Completed {
		t.Fatalf("ApplySigningPlan() result = %#v, want completed receipt", result)
	}
	if _, statErr := os.Stat(plan.ReceiptPath); statErr != nil {
		t.Fatalf("receipt after retained close failure = %v, want present", statErr)
	}
}

func TestSigningPathLexicallyContainedUsesLinuxVolumeSemantics(t *testing.T) {
	previousOS := runtimeGOOS
	previousFn := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "linux"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousFn
	})

	root := "/mnt/vfat/Project"
	variantPath := "/mnt/vfat/project/Config.xcconfig"
	project := &structuredVersionProject{rootDir: root}
	if signingPathDefinitelyExternal(project, variantPath) {
		t.Fatalf("linux case-variant path %q was treated as definitely external", variantPath)
	}
	if !signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("linux case-variant path %q was not contained in %q", variantPath, root)
	}

	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	if signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("linux case-variant path %q was authorized on a known case-sensitive volume", variantPath)
	}
}

func TestSigningArtifactLexicalPathEqualUsesVolumeSeamOnLinux(t *testing.T) {
	previousOS := runtimeGOOS
	previousFn := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "linux"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousFn
	})
	if !signingArtifactLexicalPathEqual("/mnt/vfat/plan.json", "/mnt/vfat/PLAN.json") {
		t.Fatal("linux case-insensitive volume did not treat PLAN.json as an artifact alias")
	}
}

func TestWriteSigningPlanArtifactRejectsSymlinkParent(t *testing.T) {
	realParent := t.TempDir()
	linkParent := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Fatalf("Symlink(parent) error = %v", err)
	}
	plan := &SigningPlan{
		SchemaVersion: 1,
		Command:       "asc xcode signing plan",
		PlanPath:      filepath.Join(linkParent, "plan.json"),
		Ready:         true,
	}
	if err := WriteSigningPlanArtifact(plan, false); err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("WriteSigningPlanArtifact() error = %v, want symlink parent rejection", err)
	}
	if _, statErr := os.Lstat(filepath.Join(realParent, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact through symlink parent = %v, want absent", statErr)
	}
}

func TestSigningApplyRollsBackProjectWhenReceiptFinalizationFails(t *testing.T) {
	requireStrictSigningPlatform(t)
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	beforeInfo, err := os.Stat(pbxprojPath)
	if err != nil {
		t.Fatalf("Stat(project) error = %v", err)
	}
	injectedErr := errors.New("injected receipt finalization failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			return nil, injectedErr
		}
		return originalCreator(write, data)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ApplySigningPlan() error = %v, want injected receipt failure", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("receipt finalization failure left project changes behind")
	}
	afterInfo, err := os.Stat(pbxprojPath)
	if err != nil {
		t.Fatalf("Stat(project after rollback) error = %v", err)
	}
	if afterInfo.Mode().Perm() != beforeInfo.Mode().Perm() {
		t.Fatalf("project mode after rollback = %o, want %o", afterInfo.Mode().Perm(), beforeInfo.Mode().Perm())
	}
	if _, err := os.Lstat(plan.ReceiptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("receipt after failed transaction = %v, want absent", err)
	}
}

func TestSigningApplyRejectsUntouchedSourceChangeAfterReceiptPublication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict signing apply fails closed before publication on Windows")
	}
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	beforeProjectInfo, err := os.Stat(pbxprojPath)
	if err != nil {
		t.Fatalf("Stat(project) error = %v", err)
	}
	beforeSettings := mustReadVersionTestFile(t, settingsPath)
	beforeSettingsInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("Stat(settings before replacement) error = %v", err)
	}
	replacementPath := filepath.Join(root, "settings-replacement.json")
	var replacementInfo os.FileInfo
	swapped := false
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly || swapped {
			return identity, err
		}
		// The receipt has been published. Replace an untouched source with the
		// same bytes but a new inode before the post-create source check so the
		// transaction must reject the receipt and preserve this replacement.
		swapped = true
		if err := os.WriteFile(replacementPath, []byte(beforeSettings), 0o600); err != nil {
			t.Fatalf("write racing settings replacement: %v", err)
		}
		replacementInfo, err = os.Stat(replacementPath)
		if err != nil {
			t.Fatalf("Stat(racing settings replacement) error = %v", err)
		}
		if err := os.Rename(replacementPath, settingsPath); err != nil {
			t.Fatalf("install racing settings replacement: %v", err)
		}
		return identity, nil
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "after receipt") {
		t.Fatalf("ApplySigningPlan() error = %v, want post-publication source rejection", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("post-publication source rejection left project changes behind")
	}
	afterProjectInfo, err := os.Stat(pbxprojPath)
	if err != nil {
		t.Fatalf("Stat(project after rollback) error = %v", err)
	}
	if afterProjectInfo.Mode().Perm() != beforeProjectInfo.Mode().Perm() {
		t.Fatalf("project mode after rollback = %o, want %o", afterProjectInfo.Mode().Perm(), beforeProjectInfo.Mode().Perm())
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after post-publication source drift = %v, want absent", statErr)
	}
	if after := mustReadVersionTestFile(t, settingsPath); after != beforeSettings {
		t.Fatalf("racing settings replacement was changed: %q", after)
	}
	settingsInfo, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("Stat(settings replacement) error = %v", err)
	}
	if settingsInfo.Mode().Perm() != 0o600 {
		t.Fatalf("racing settings replacement mode = %o, want 600", settingsInfo.Mode().Perm())
	}
	if os.SameFile(beforeSettingsInfo, settingsInfo) {
		t.Fatal("racing settings replacement still refers to the original source inode")
	}
	if replacementInfo == nil || !os.SameFile(replacementInfo, settingsInfo) {
		t.Fatal("racing settings replacement inode was not preserved")
	}
}

func TestSigningApplyRejectsCreateOnlySuccessWithoutIdentity(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict signing apply fails closed before receipt creation on Windows")
	}
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			return nil, nil
		}
		return originalCreator(write, data)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "created identity unavailable") {
		t.Fatalf("ApplySigningPlan() error = %v, want create-only identity failure", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("create-only identity failure left project changes behind")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after create-only identity failure = %v, want absent", statErr)
	}
}

func TestSigningApplyRejectsReceiptContentChangedAfterPublication(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict signing apply fails closed before receipt publication on Windows")
	}
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
	const racingReceipt = "racing receipt contents\n"
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return identity, err
		}
		if err := os.WriteFile(write.path, []byte(racingReceipt), 0o600); err != nil {
			t.Fatalf("change receipt after publication: %v", err)
		}
		return identity, nil
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "verify created file") || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("ApplySigningPlan() error = %v, want receipt-content rejection with safe rollback uncertainty", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); !strings.Contains(after, `"CODE_SIGN_STYLE" = Manual;`) {
		t.Fatal("receipt-content uncertainty rolled back project changes while the receipt remained")
	}
	if after := mustReadVersionTestFile(t, plan.ReceiptPath); after != racingReceipt {
		t.Fatalf("racing receipt was removed or changed: %q", after)
	}
}

func TestSigningApplyRemovesReceiptWhenPostCreateVerificationFails(t *testing.T) {
	requireStrictSigningPlatform(t)
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	injectedErr := errors.New("injected post-create receipt verification failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		createdInfo, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return createdInfo, err
		}
		return createdInfo, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ApplySigningPlan() error = %v, want post-create verification failure", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("post-create verification failure left project changes behind")
	}
	if _, err := os.Lstat(plan.ReceiptPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("receipt after post-create verification failure = %v, want absent", err)
	}
}

func TestSigningApplyRollsBackProjectWhenReceiptIdentityObservationFailsAfterPublication(t *testing.T) {
	requireStrictSigningPlatform(t)
	// Compose a post-publication error with the proven installed identity and
	// verify receipt cleanup plus ordinary project rollback. Supported strict
	// publication paths retain that identity before later observations begin.
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	const transientObservationFailure = "injected post-publication identity observation failure"
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := atomicCreatePreparedVersionFile(write, data)
		if err != nil || !write.createOnly {
			return info, err
		}
		// Publication has already returned a proven installed identity; callers
		// may use it for conditional rollback even though a later check fails.
		return info, errors.New(transientObservationFailure)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), transientObservationFailure) {
		t.Fatalf("ApplySigningPlan() error = %v, want retained-identity observation failure", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("receipt identity observation failure left ordinary project changes behind")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after identity observation failure = %v, want absent", statErr)
	}
}

func TestSigningApplyPreservesReceiptReplacementWhenRollbackIdentityChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict signing apply fails closed before receipt publication on Windows")
	}
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
	injectedErr := errors.New("injected post-create receipt verification failure")
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		createdInfo, err := originalCreator(write, data)
		if err != nil || !write.createOnly {
			return createdInfo, err
		}
		// Replace the pathname with a distinct inode containing the same
		// completed receipt. Cleanup must remain identity-coupled and refuse to
		// remove this concurrent publication.
		replacementPath := write.path + ".replacement"
		if err := os.WriteFile(replacementPath, data, 0o600); err != nil {
			t.Fatalf("stage replacement receipt: %v", err)
		}
		replacementInfo, err := os.Stat(replacementPath)
		if err != nil {
			t.Fatalf("stat replacement receipt: %v", err)
		}
		if err := os.Rename(replacementPath, write.path); err != nil {
			t.Fatalf("replace receipt after publication: %v", err)
		}
		publishedInfo, err := os.Stat(write.path)
		if err != nil {
			t.Fatalf("stat published replacement receipt: %v", err)
		}
		if !os.SameFile(replacementInfo, publishedInfo) {
			t.Fatalf("replacement receipt inode changed during rename")
		}
		if createdInfo == nil || createdInfo.Info() == nil || os.SameFile(createdInfo.Info(), publishedInfo) {
			t.Fatal("receipt pathname was not replaced with a distinct inode")
		}
		return createdInfo, injectedErr
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if !errors.Is(err, injectedErr) || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("ApplySigningPlan() error = %v, want post-create failure with rollback uncertainty", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); !strings.Contains(after, `"CODE_SIGN_STYLE" = Manual;`) {
		t.Fatal("receipt cleanup uncertainty rolled back project changes")
	}
	if after := mustReadVersionTestFile(t, plan.ReceiptPath); !strings.Contains(after, `"completed": true`) {
		t.Fatalf("completed concurrent receipt was removed or changed: %q", after)
	}
}

func TestSigningApplyRollsBackProjectWhenReceiptRacesIntoPlace(t *testing.T) {
	requireStrictSigningPlatform(t)
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	const racingReceipt = "concurrent receipt wins\n"
	originalCreator := atomicCreateVersionFileFn
	atomicCreateVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		if write.createOnly {
			if err := os.WriteFile(write.path, []byte(racingReceipt), 0o600); err != nil {
				t.Fatalf("create racing receipt: %v", err)
			}
		}
		return originalCreator(write, data)
	}
	t.Cleanup(func() { atomicCreateVersionFileFn = originalCreator })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("ApplySigningPlan() error = %v, want receipt race rejection", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("receipt race left project changes behind")
	}
	if after := mustReadVersionTestFile(t, plan.ReceiptPath); after != racingReceipt {
		t.Fatalf("racing receipt changed: %q", after)
	}
}

func TestSigningPlanRejectsStaleProjectBeforeMutation(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
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
	if err := os.WriteFile(pbxprojPath, append([]byte(mustReadVersionTestFile(t, pbxprojPath)), []byte("\n")...), 0o644); err != nil {
		t.Fatalf("modify project error = %v", err)
	}
	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("expected stale-plan error, got %v", err)
	}
	if strings.Contains(mustReadVersionTestFile(t, pbxprojPath), `"CODE_SIGN_STYLE" = Manual;`) {
		t.Fatal("stale apply mutated the project")
	}
}

func TestSigningApplyRechecksSourcesBeforeReceipt(t *testing.T) {
	requireStrictSigningPlatform(t)
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	originalWriter := atomicWriteVersionFileFn
	mutated := false
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		info, err := originalWriter(write, data)
		if err == nil && !write.createOnly && !mutated {
			mutated = true
			settings := mustReadVersionTestFile(t, settingsPath)
			if err := os.WriteFile(settingsPath, []byte(settings+"\n"), 0o600); err != nil {
				t.Fatalf("mutate settings after ordinary write: %v", err)
			}
		}
		return info, err
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "before receipt") {
		t.Fatalf("ApplySigningPlan() error = %v, want final source recheck failure", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("source recheck failure left project changes behind")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after final source drift = %v, want absent", statErr)
	}
}

func TestSigningApplyRejectsOptionalIncludeAppearingAfterPreparation(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = strings.Replace(contents, `#include "Shared.xcconfig"`, "#include? \"Missing.xcconfig\"\n#include \"Shared.xcconfig\"", 1)
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}

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
	if len(plan.MissingOptionalIncludes) != 1 {
		t.Fatalf("missing optional includes = %#v, want one assertion", plan.MissingOptionalIncludes)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}

	missingPath := plan.MissingOptionalIncludes[0]
	projectPath := filepath.Join(project, "project.pbxproj")
	projectBefore := mustReadVersionTestFile(t, projectPath)
	originalHook := beforeSigningCommitForTest
	beforeSigningCommitForTest = func() {
		beforeSigningCommitForTest = nil
		if err := os.WriteFile(missingPath, []byte("CODE_SIGN_STYLE = late;\n"), 0o640); err != nil {
			t.Fatalf("create late optional include: %v", err)
		}
	}
	t.Cleanup(func() { beforeSigningCommitForTest = originalHook })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "optional xcconfig include appeared") {
		t.Fatalf("ApplySigningPlan() error = %v, want late optional-include race rejection", err)
	}
	if after := mustReadVersionTestFile(t, projectPath); after != projectBefore {
		t.Fatal("late optional-include rejection modified the project")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after late optional-include rejection = %v, want absent", statErr)
	}
}

func TestSigningApplyRejectsOptionalIncludeAppearingBeforeReceipt(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = strings.Replace(contents, `#include "Shared.xcconfig"`, "#include? \"Missing.xcconfig\"\n#include \"Shared.xcconfig\"", 1)
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}

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
	if len(plan.MissingOptionalIncludes) != 1 {
		t.Fatalf("missing optional includes = %#v, want one assertion", plan.MissingOptionalIncludes)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}

	missingPath := plan.MissingOptionalIncludes[0]
	projectPath := filepath.Join(project, "project.pbxproj")
	projectBefore := mustReadVersionTestFile(t, projectPath)
	originalWriter := atomicWriteVersionFileFn
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		identity, err := originalWriter(write, data)
		if err == nil && !write.createOnly {
			if createErr := os.WriteFile(missingPath, []byte("CODE_SIGN_STYLE = late;\n"), 0o640); createErr != nil {
				t.Fatalf("create late optional include: %v", createErr)
			}
		}
		return identity, err
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "before receipt") {
		t.Fatalf("ApplySigningPlan() error = %v, want final optional-include race rejection", err)
	}
	if after := mustReadVersionTestFile(t, projectPath); after != projectBefore {
		t.Fatal("late optional-include rejection left the ordinary project write behind")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after final optional-include rejection = %v, want absent", statErr)
	}
}

func TestSigningPlanUsesExclusiveXCConfigWhenSelected(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o644); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("expected one scoped change per configuration, got %#v", plan.Changes)
	}
	for _, change := range plan.Changes {
		if change.Source != "xcconfig" || change.Path != sharedPath {
			t.Fatalf("expected shared xcconfig source, got %#v", change)
		}
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	updated := mustReadVersionTestFile(t, sharedPath)
	if !strings.Contains(updated, "CODE_SIGN_STYLE = Manual\r\n") {
		t.Fatalf("shared xcconfig was not updated: %q", updated)
	}
}

func TestSigningPlanRejectsContinuedXCConfigAssignmentBeforeEditing(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	original := mustReadVersionTestFile(t, sharedPath)
	continued := original + "CODE_SIGN_IDENTITY = Apple \\\n Development\n"
	if err := os.WriteFile(sharedPath, []byte(continued), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_IDENTITY":"Apple Distribution"}},
			{"name":"Release","settings":{"CODE_SIGN_IDENTITY":"Apple Distribution"}}
		]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "line continuation") {
		t.Fatalf("BuildSigningPlan() error = %v, want continued-assignment refusal", err)
	}
	if after := mustReadVersionTestFile(t, sharedPath); after != continued {
		t.Fatal("planning a continued assignment modified the xcconfig")
	}
}

func TestSigningPlanIgnoresUnselectedSigningContinuation(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	widgetPath := attachSigningWidgetXCConfig(t, project, "CODE_SIGN_STYLE = Automatic \\\n+\tcontinued\n")
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("unselected signing continuation blocked plan: %#v", plan.Blockers)
	}
	for _, file := range plan.Files {
		if file.Path == widgetPath {
			return
		}
	}
	t.Fatalf("plan omitted unselected consumer input %s: %#v", widgetPath, plan.Files)
}

func TestSigningPlanBlocksUnterminatedQuotedSigningValue(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	before := `DEVELOPMENT_TEAM = "BROKEN` + "\n" + mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte(before), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"DEVELOPMENT_TEAM":"ABCDEFGHIJ"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want incomplete internal collection to fail closed", plan, err)
	}
	if after := mustReadVersionTestFile(t, sharedPath); after != before {
		t.Fatal("planning a malformed quoted assignment modified the xcconfig")
	}
}

func TestSigningPlanBlocksUnsafeUnquotedXCConfigValue(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	before := `PROVISIONING_PROFILE_SPECIFIER = Old Profile` + "\r\n" + mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte(before), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":"New Profile\\"}},
			{"name":"Release","settings":{"PROVISIONING_PROFILE_SPECIFIER":"New Profile\\"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want blocked plan", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "trailing backslash") {
		t.Fatalf("BuildSigningPlan() = ready %t, blockers %#v", plan.Ready, plan.Blockers)
	}
	if after := mustReadVersionTestFile(t, sharedPath); after != before {
		t.Fatal("planning an unsafe quoted value modified the xcconfig")
	}
}

func TestSigningPlanApplyIsIdempotentForQuotedProfileSpecifierEscapes(t *testing.T) {
	for _, test := range []struct {
		name  string
		quote string
		value string
	}{
		{name: "double quoted", quote: `"`, value: `Profile\Path "Preview"\Tail\`},
		{name: "single quoted", quote: `'`, value: `Profile\Path 'Preview'\Tail\`},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, true)
			sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
			shared := mustReadVersionTestFile(t, sharedPath)
			shared = fmt.Sprintf("PROVISIONING_PROFILE_SPECIFIER = %sOld Profile%s\r\n", test.quote, test.quote) + shared
			if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
				t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
			}

			encodedValue, err := json.Marshal(test.value)
			if err != nil {
				t.Fatalf("json.Marshal(profile specifier) error = %v", err)
			}
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			settings := fmt.Sprintf(`{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[
					{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":%s}},
					{"name":"Release","settings":{"PROVISIONING_PROFILE_SPECIFIER":%s}}
				]}]
			}`, encodedValue, encodedValue)
			writeSigningSettingsTestFile(t, settingsPath, settings)

			first, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "first"),
			})
			if err != nil {
				t.Fatalf("BuildSigningPlan(first) error = %v", err)
			}
			if !first.Ready || len(first.Changes) == 0 {
				t.Fatalf("first plan = ready=%t changes=%#v blockers=%#v, want a ready update", first.Ready, first.Changes, first.Blockers)
			}
			if err := WriteSigningPlanArtifact(first, false); err != nil {
				t.Fatalf("WriteSigningPlanArtifact(first) error = %v", err)
			}
			if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: first.PlanPath}); err != nil {
				t.Fatalf("ApplySigningPlan(first) error = %v", err)
			}

			updated := mustReadVersionTestFile(t, sharedPath)
			serialized := quoteXCConfigValue(test.value, test.quote)
			if !strings.Contains(updated, "PROVISIONING_PROFILE_SPECIFIER = "+serialized) {
				t.Fatalf("applied xcconfig does not preserve the logical quoted value: %q", updated)
			}

			second, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "second"),
			})
			if err != nil {
				t.Fatalf("BuildSigningPlan(second) error = %v", err)
			}
			if !second.Ready || len(second.Changes) != 0 {
				t.Fatalf("second plan = ready=%t changes=%#v blockers=%#v, want ready no-op", second.Ready, second.Changes, second.Blockers)
			}
		})
	}
}

func TestSigningPlanBlocksAmbiguousProjectNames(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{
			name: "duplicate target name",
			mutate: func(contents string) string {
				return strings.Replace(contents, "\t\t\tname = Widget;\n\t\t\tproductName = Widget;", "\t\t\tname = App;\n\t\t\tproductName = Widget;", 1)
			},
			want: `multiple targets named "App"`,
		},
		{
			name: "duplicate configuration name",
			mutate: func(contents string) string {
				const original = "999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };"
				const duplicate = "999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
				return strings.Replace(contents, original, duplicate, 1)
			},
			want: `multiple configurations named "Debug"`,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			pbxprojPath := filepath.Join(project, "project.pbxproj")
			before := mustReadVersionTestFile(t, pbxprojPath)
			mutated := testCase.mutate(before)
			if mutated == before {
				t.Fatal("fixture mutation did not change the project")
			}
			if err := os.WriteFile(pbxprojPath, []byte(mutated), 0o644); err != nil {
				t.Fatalf("WriteFile(project) error = %v", err)
			}
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[
					{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}
				]}]
			}`)

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
			})
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want blocked plan", err)
			}
			if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), testCase.want) {
				t.Fatalf("BuildSigningPlan() = ready %t, blockers %#v, want %q", plan.Ready, plan.Blockers, testCase.want)
			}
			if after := mustReadVersionTestFile(t, pbxprojPath); after != mutated {
				t.Fatal("planning an ambiguous project modified the pbxproj")
			}
		})
	}
}

func TestSigningPlanRejectsDuplicateProjectConfigurationNames(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const original = "999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Release; };"
	const duplicate = "999999999999999999999992 /* Project Release */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	if !strings.Contains(contents, original) {
		t.Fatal("project fixture is missing project Release configuration")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, original, duplicate, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), `project contains multiple configurations named "Debug"`) {
		t.Fatalf("BuildSigningPlan() error = %v, want duplicate project configuration rejection", err)
	}
}

func TestPrepareSigningOperationsUsesWindowsXCConfigIdentity(t *testing.T) {
	// The Windows path semantics under test are simulated through runtimeGOOS,
	// but the commit below publishes through the strict identity path, which
	// fails closed before publication on a real Windows host.
	requireStrictSigningPlatform(t)
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	projectPath := writeStructuredVersionProject(t, false)
	configDir := filepath.Join(filepath.Dir(projectPath), "Configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	configPath := filepath.Join(configDir, "Config.xcconfig")
	caseVariantPath := filepath.Join(configDir, "config.xcconfig")
	original := []byte("CODE_SIGN_STYLE = Automatic\n")
	if err := os.WriteFile(configPath, original, 0o640); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	caseVariantExists := false
	if _, err := os.Lstat(caseVariantPath); err == nil {
		caseVariantExists = true
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(case variant) error = %v", err)
	}
	if !caseVariantExists {
		if err := os.WriteFile(caseVariantPath, original, 0o640); err != nil {
			t.Fatalf("WriteFile(case variant) error = %v", err)
		}
	}

	project, err := openStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	debug, err := signingConfigurationFor(project, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor(Debug) error = %v", err)
	}
	release, err := signingConfigurationFor(project, "App", "Release")
	if err != nil {
		t.Fatalf("signingConfigurationFor(Release) error = %v", err)
	}
	configInfo, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("Stat(config) error = %v", err)
	}
	caseVariantInfo, err := os.Stat(caseVariantPath)
	if err != nil {
		t.Fatalf("Stat(case variant) error = %v", err)
	}
	sameFile := os.SameFile(configInfo, caseVariantInfo)
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return sameFile, true }
	t.Cleanup(func() { signingCaseInsensitiveVolumeFn = previousCaseSemantics })
	secondIdentity := "identity:second"
	if sameFile {
		secondIdentity = "identity:first"
	}
	fileIdentities := map[string]string{
		normalizeSigningLexicalPath(configPath):      "identity:first",
		normalizeSigningLexicalPath(caseVariantPath): secondIdentity,
	}
	built := &signingPlanBuild{
		plan:           &SigningPlan{AllowExternalXCConfig: false},
		project:        project,
		fileIdentities: fileIdentities,
		operations: []signingPlanOperation{
			{
				SigningSettingChange: SigningSettingChange{
					Target: "App", Configuration: "Debug", Setting: "CODE_SIGN_STYLE",
					Operation: "set", NewValue: stringPtr("Manual"), Path: configPath, Source: "xcconfig",
				},
				configuration: debug,
			},
			{
				SigningSettingChange: SigningSettingChange{
					Target: "App", Configuration: "Release", Setting: "CODE_SIGN_STYLE",
					Operation: "set", NewValue: stringPtr("Manual"), Path: caseVariantPath, Source: "xcconfig",
				},
				configuration: release,
			},
		},
	}
	prepared, err := prepareSigningOperations(built)
	if err != nil {
		t.Fatalf("prepareSigningOperations() error = %v", err)
	}
	defer func() { _ = closeVersionWrites(prepared.writes) }()
	defer func() { _ = prepared.projectRoot.Close() }()
	wantWrites := 2
	if sameFile {
		wantWrites = 1
	}
	if len(prepared.writes) != wantWrites || len(prepared.changedFiles) != wantWrites {
		t.Fatalf("prepared writes = %d, changed files = %#v, want %d identity-grouped writes", len(prepared.writes), prepared.changedFiles, wantWrites)
	}

	plan := &SigningPlan{Files: []SigningPlanFile{{
		Path: configPath, SHA256: signingFileDigestBytes(original), Source: "xcconfig",
	}}}
	if !sameFile {
		plan.Files = append(plan.Files, SigningPlanFile{
			Path: caseVariantPath, SHA256: signingFileDigestBytes(original), Source: "xcconfig",
		})
	}
	if err := verifySigningPlanSources(plan, prepared.writes, fileIdentities); err != nil {
		t.Fatalf("verifySigningPlanSources() error = %v", err)
	}
	fileChanges, err := signingReceiptFileChanges(plan, prepared.writes, prepared.changedFiles, fileIdentities)
	if err != nil {
		t.Fatalf("signingReceiptFileChanges() error = %v", err)
	}
	if len(fileChanges) != wantWrites {
		t.Fatalf("file changes = %#v, want %d", fileChanges, wantWrites)
	}
	if err := commitVersionWrites(prepared.writes); err != nil {
		t.Fatalf("commitVersionWrites() error = %v", err)
	}
	recheckRoot, err := rootfs.New(configDir)
	if err != nil {
		t.Fatalf("rootfs.New(recheck) error = %v", err)
	}
	defer recheckRoot.Close()
	committed := make([]preparedVersionWrite, 0, len(prepared.writes))
	for _, write := range prepared.writes {
		committedIdentity, captureErr := recheckRoot.CaptureFile(filepath.Base(write.path))
		if captureErr != nil {
			t.Fatalf("CaptureFile(%q) error = %v", write.path, captureErr)
		}
		committed = append(committed, preparedVersionWrite{
			path: write.path, name: filepath.Base(write.path), root: recheckRoot,
			original: write.original, updated: write.updated, committedIdentity: committedIdentity,
		})
	}
	if err := verifySigningPlanSourcesBeforeReceipt(plan, committed, prepared.writes, fileIdentities); err != nil {
		t.Fatalf("verifySigningPlanSourcesBeforeReceipt() error = %v", err)
	}
	for _, path := range []string{configPath, caseVariantPath} {
		if got := mustReadVersionTestFile(t, path); !strings.Contains(got, "CODE_SIGN_STYLE = Manual") {
			t.Fatalf("identity-grouped mutation did not apply to %q: %q", path, got)
		}
	}
}

func TestResolveSigningSharedCandidatesCoalescesCaseVariantSameFile(t *testing.T) {
	root := t.TempDir()
	canonicalPath := filepath.Join(root, "Config.xcconfig")
	caseVariantPath := filepath.Join(root, "config.xcconfig")
	if err := os.WriteFile(canonicalPath, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	if err := os.Link(canonicalPath, caseVariantPath); err != nil && !errors.Is(err, os.ErrExist) {
		t.Skipf("hard links unavailable: %v", err)
	}
	canonicalInfo, err := os.Stat(canonicalPath)
	if err != nil {
		t.Fatalf("Stat(canonical) error = %v", err)
	}
	caseVariantInfo, err := os.Stat(caseVariantPath)
	if err != nil {
		t.Fatalf("Stat(case variant) error = %v", err)
	}
	if !os.SameFile(canonicalInfo, caseVariantInfo) {
		t.Skip("case-variant paths are distinct files on this filesystem")
	}

	candidates := []signingCandidate{
		{mode: "xcconfig", desired: stringPtr("Manual"), paths: []string{canonicalPath}},
		{mode: "xcconfig", desired: stringPtr("Automatic"), paths: []string{caseVariantPath}},
	}
	identities := map[string]string{
		signingLexicalPathKey(canonicalPath):   canonicalPath,
		signingLexicalPathKey(caseVariantPath): canonicalPath,
	}
	resolveSigningSharedCandidates(candidates, identities)
	for index, candidate := range candidates {
		if candidate.mode != "pbxproj" || len(candidate.paths) != 0 {
			t.Fatalf("candidate[%d] = %#v, want conflicting same-file operation blocked from shared mutation", index, candidate)
		}
	}
}

func TestSigningXCConfigOperationKeyPreservesHardLinkPathIntent(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "First.xcconfig")
	secondPath := filepath.Join(root, "Second.xcconfig")
	if err := os.WriteFile(firstPath, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}
	if err := os.Link(firstPath, secondPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	firstInfo, err := os.Stat(firstPath)
	if err != nil {
		t.Fatalf("Stat(first) error = %v", err)
	}
	secondInfo, err := os.Stat(secondPath)
	if err != nil {
		t.Fatalf("Stat(second) error = %v", err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("test fixture is not hard-linked")
	}
	identities := map[string]string{
		normalizeSigningLexicalPath(firstPath):  "shared-identity",
		normalizeSigningLexicalPath(secondPath): "shared-identity",
	}
	firstIdentity, firstOK := signingFileIdentity(firstPath, identities)
	secondIdentity, secondOK := signingFileIdentity(secondPath, identities)
	if !firstOK || !secondOK || firstIdentity != secondIdentity {
		t.Fatal("hard-linked paths should retain one physical identity for consumer safety")
	}
	if signingXCConfigOperationKey(firstPath, identities) == signingXCConfigOperationKey(secondPath, identities) {
		t.Fatal("hard-linked paths must retain distinct operation path intent")
	}
	oldData := []byte("CODE_SIGN_STYLE = Automatic\n")
	firstUpdated := []byte("CODE_SIGN_STYLE = Manual\n")
	secondUpdated := []byte("CODE_SIGN_STYLE = Automatic /* retained */\n")
	plan := &SigningPlan{Files: []SigningPlanFile{
		{Path: firstPath, SHA256: signingFileDigestBytes(oldData), Source: "xcconfig"},
		{Path: secondPath, SHA256: signingFileDigestBytes(oldData), Source: "xcconfig"},
	}}
	changes, err := signingReceiptFileChanges(plan, []preparedVersionWrite{
		{path: firstPath, original: oldData, updated: firstUpdated},
		{path: secondPath, original: oldData, updated: secondUpdated},
	}, []string{firstPath, secondPath}, identities)
	if err != nil {
		t.Fatalf("signingReceiptFileChanges() error = %v", err)
	}
	if len(changes) != 2 || changes[0].Path != firstPath || changes[1].Path != secondPath {
		t.Fatalf("receipt changes = %#v, want one change per hard-linked path intent", changes)
	}
}

func TestSigningXCConfigOperationKeyCoalescesParentSymlinkDirectoryEntry(t *testing.T) {
	root := t.TempDir()
	realDir := filepath.Join(root, "Configs")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatalf("Mkdir(Configs) error = %v", err)
	}
	canonicalPath := filepath.Join(realDir, "Shared.xcconfig")
	if err := os.WriteFile(canonicalPath, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}
	aliasDir := filepath.Join(root, "AliasDir")
	if err := os.Symlink(realDir, aliasDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	aliasPath := filepath.Join(aliasDir, "Shared.xcconfig")
	canonicalInfo, err := os.Stat(canonicalPath)
	if err != nil {
		t.Fatalf("Stat(canonical) error = %v", err)
	}
	aliasInfo, err := os.Stat(aliasPath)
	if err != nil {
		t.Fatalf("Stat(alias) error = %v", err)
	}
	if !os.SameFile(canonicalInfo, aliasInfo) {
		t.Fatal("parent-symlink alias did not resolve to the same file")
	}
	identities := map[string]string{
		normalizeSigningLexicalPath(canonicalPath): "shared-identity",
		normalizeSigningLexicalPath(aliasPath):     "shared-identity",
	}
	if signingXCConfigOperationKey(canonicalPath, identities) != signingXCConfigOperationKey(aliasPath, identities) {
		t.Fatal("parent-symlink aliases of one directory entry must share an operation key")
	}
	if signingXCConfigPhysicalKey(canonicalPath, identities) != signingXCConfigPhysicalKey(aliasPath, identities) {
		t.Fatal("parent-symlink aliases must retain one physical identity")
	}
}

func TestResolveSigningSharedCandidatesKeepsProvenWindowsCaseDistinctFilesSeparate(t *testing.T) {
	previousOS := runtimeGOOS
	runtimeGOOS = "windows"
	t.Cleanup(func() { runtimeGOOS = previousOS })

	root := t.TempDir()
	firstPath := filepath.Join(root, "Config.xcconfig")
	secondPath := filepath.Join(root, "config.xcconfig")
	identities := map[string]string{
		normalizeSigningLexicalPath(firstPath):  "identity:first",
		normalizeSigningLexicalPath(secondPath): "identity:second",
	}
	if signingXCConfigOperationKey(firstPath, identities) == signingXCConfigOperationKey(secondPath, identities) {
		t.Fatal("proven-distinct Windows xcconfig identities collapsed to one operation key")
	}

	candidates := []signingCandidate{
		{mode: "xcconfig", setting: "CODE_SIGN_STYLE", desired: stringPtr("Manual"), paths: []string{firstPath}},
		{mode: "xcconfig", setting: "CODE_SIGN_STYLE", desired: stringPtr("Automatic"), paths: []string{secondPath}},
	}
	resolveSigningSharedCandidates(candidates, identities)
	for index, candidate := range candidates {
		if candidate.mode != "xcconfig" || len(candidate.paths) != 1 {
			t.Fatalf("candidate[%d] = %#v, want separate case-sensitive file operation", index, candidate)
		}
	}
}

func TestSigningPathLexicallyContainedAcceptsCaseVariantRootOnInsensitiveVolume(t *testing.T) {
	root := t.TempDir()
	caseInsensitive, known := signingCaseInsensitiveVolumeFor(root)
	if !known || !caseInsensitive {
		t.Skip("temporary filesystem is not known to be case-insensitive")
	}

	name := filepath.Base(root)
	variantName := strings.ToUpper(name[:1]) + name[1:]
	if variantName == name {
		variantName = strings.ToLower(name[:1]) + name[1:]
	}
	if variantName == name {
		t.Skip("temporary directory name has no case variant")
	}
	path := filepath.Join(root, "Config.xcconfig")
	if err := os.WriteFile(path, []byte("CODE_SIGN_STYLE = Manual\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	variantPath := filepath.Join(filepath.Dir(root), variantName, filepath.Base(path))
	if _, err := os.Stat(variantPath); err != nil {
		t.Fatalf("Stat(case-variant internal path) error = %v", err)
	}

	project := &structuredVersionProject{rootDir: root}
	if !signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("case-variant internal path %q was classified outside %q", variantPath, root)
	}
}

func TestSigningPathLexicallyContainedRejectsCaseVariantRootOnSensitiveWindowsDirectory(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootName := filepath.Base(root)
	variantName := strings.ToUpper(rootName[:1]) + rootName[1:]
	if variantName == rootName {
		variantName = strings.ToLower(rootName[:1]) + rootName[1:]
	}
	if variantName == rootName {
		t.Skip("temporary directory name has no case variant")
	}
	variantPath := filepath.Join(filepath.Dir(root), variantName, "Config.xcconfig")
	project := &structuredVersionProject{rootDir: root}
	if signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("case-variant path %q was authorized inside case-sensitive Windows root %q", variantPath, root)
	}

	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }
	if !signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("case-variant path %q was not accepted on a proven case-insensitive Windows directory", variantPath)
	}
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, false }
	if signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("case-variant path %q was authorized with unknown Windows directory semantics", variantPath)
	}
}

func TestValidateSigningXCConfigPathRejectsCaseVariantRootOnSensitiveWindowsDirectory(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	rootName := filepath.Base(root)
	variantName := strings.ToUpper(rootName[:1]) + rootName[1:]
	if variantName == rootName {
		variantName = strings.ToLower(rootName[:1]) + rootName[1:]
	}
	if variantName == rootName {
		t.Skip("temporary directory name has no case variant")
	}
	project := &structuredVersionProject{rootDir: root}
	path := filepath.Join(filepath.Dir(root), variantName, "Config.xcconfig")
	if err := validateSigningXCConfigPath(project, path, false); !errors.Is(err, rootfs.ErrEscapesRoot) {
		t.Fatalf("validateSigningXCConfigPath() error = %v, want path escape for case-sensitive Windows root", err)
	}
}

func TestSigningPlanKeepsUnselectedUnresolvedEntitlementsAsUncertainty(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const widgetMarker = "999999999999999999999995 /* Widget Debug */ = {"
	start := strings.Index(contents, widgetMarker)
	if start < 0 {
		t.Fatalf("project fixture is missing Widget Debug configuration")
	}
	const settingsMarker = "buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; };"
	relative := strings.Index(contents[start:], settingsMarker)
	if relative < 0 {
		t.Fatalf("project fixture is missing Widget Debug build settings")
	}
	settingsStart := start + relative
	updated := `buildSettings = { CODE_SIGN_ENTITLEMENTS = "$(MISSING_ENTITLEMENTS)"; MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; };`
	contents = contents[:settingsStart] + updated + contents[settingsStart+len(settingsMarker):]
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() error = %v, want fail-closed unbounded entitlement error", err)
	}
	if plan != nil {
		t.Fatalf("unselected unresolved entitlement produced a plan: %#v", plan)
	}
}

func TestSigningPlanIgnoresShadowedUnresolvedXCConfigEntitlements(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		assignments string
	}{
		{
			name:        "later assignment replaces unresolved value",
			assignments: "CODE_SIGN_ENTITLEMENTS = $(MISSING)\nCODE_SIGN_ENTITLEMENTS = App.entitlements\n",
		},
		{
			name:        "skipped conditional assignment follows concrete value",
			assignments: "CODE_SIGN_ENTITLEMENTS = App.entitlements\nCODE_SIGN_ENTITLEMENTS ?= $(MISSING)\n",
		},
		{
			name:        "later concrete assignment replaces SDK conditional default",
			assignments: "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= $(MISSING)\nCODE_SIGN_ENTITLEMENTS = App.entitlements\n",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, true)
			projectRoot := filepath.Dir(project)
			appXCConfig := filepath.Join(projectRoot, "Configs", "App.xcconfig")
			contents := mustReadVersionTestFile(t, appXCConfig) + testCase.assignments
			if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
				t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
			}
			entitlementsPath := filepath.Join(projectRoot, "App.entitlements")
			if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
				t.Fatalf("WriteFile(entitlements) error = %v", err)
			}
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
			if !plan.Ready {
				t.Fatalf("shadowed entitlement expression blocked plan: %#v", plan.Blockers)
			}
		})
	}
}

func TestSigningPlanProtectsUnconditionalEntitlementWhenConditionalResolutionDiverges(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	widgetXCConfig := filepath.Join(projectRoot, "Widget.xcconfig")
	const widgetReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
	projectContents := mustReadVersionTestFile(t, filepath.Join(project, "project.pbxproj"))
	fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Widget.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(projectContents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	projectContents = strings.Replace(projectContents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(projectContents, widgetConfiguration) {
		t.Fatalf("project fixture is missing Widget Debug configuration")
	}
	projectContents = strings.Replace(projectContents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(filepath.Join(project, "project.pbxproj"), []byte(projectContents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	if err := os.WriteFile(widgetXCConfig, []byte(
		"CODE_SIGN_ENTITLEMENTS = plan.json\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = Widget.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(projectRoot, "plan.json")
	const existingPlan = "existing entitlement bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want unconditional entitlement alias rejection", plan, err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("unconditional entitlement was overwritten during divergent-resolution planning: %q", got)
	}
}

func TestSigningPlanProtectsConditionalInheritedEntitlementPath(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	composedPath := filepath.Join(projectRoot, "BaseSuffix")
	const existingEntitlements = "existing entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}
	attachSigningWidgetXCConfig(
		t, project,
		"CODE_SIGN_ENTITLEMENTS = Base\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited)Suffix\n",
	)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want conditional inherited entitlement path rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsConditionalComposedEntitlementAppendPath(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	composedPath := filepath.Join(projectRoot, "Base Suffix")
	const existingEntitlements = "existing entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}
	attachSigningWidgetXCConfig(
		t, project,
		"CODE_SIGN_ENTITLEMENTS = Base\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] += Suffix\n",
	)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want conditional composed entitlement append path rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsComposedEntitlementAppendPath(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	composedPath := filepath.Join(projectRoot, "Base Suffix")
	const existingEntitlements = "existing entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}
	attachSigningWidgetXCConfig(
		t, project,
		"CODE_SIGN_ENTITLEMENTS = Base\n"+
			"CODE_SIGN_ENTITLEMENTS += Suffix\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = Widget.entitlements\n",
	)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want composed entitlement append path rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanSkipsShadowedConditionalAfterProjectEntitlementBase(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	widgetXCConfig := filepath.Join(projectRoot, "Widget.xcconfig")
	const widgetReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Widget.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(contents, widgetConfiguration) {
		t.Fatal("project fixture is missing Widget Debug configuration")
	}
	contents = strings.Replace(contents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991", `CODE_SIGN_ENTITLEMENTS = safe.entitlements;`)
	if err := os.WriteFile(widgetXCConfig, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = other.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= plan.json\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(projectRoot, "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(t.TempDir(), "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want shadowed conditional to be omitted", err)
	}
	if plan.Ready {
		t.Fatalf("BuildSigningPlan() produced ready plan despite divergent conditional resolution: %#v", plan)
	}
	if strings.Contains(strings.Join(plan.Blockers, "\n"), "aliases project input") {
		t.Fatalf("shadowed conditional was incorrectly treated as an artifact alias: %#v", plan.Blockers)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("plan artifact changed during planning: %q", got)
	}
}

func TestSigningXCConfigEntitlementCandidatesOmitProvenShadowedAssignments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS = Shadowed.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS = Current.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = Conditional.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if candidates[key][0] {
		t.Fatal("assignment shadowed by a later unconditional value remained a candidate")
	}
	if !candidates[key][1] || !candidates[key][2] {
		t.Fatalf("live entitlement assignments = %#v, want current unconditional and conditional values", candidates[key])
	}
}

func TestSigningXCConfigEntitlementCandidatesSkipConditionalDefaultAfterDivergentOverride(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS = safe.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = other.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= shadowed.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if !candidates[key][0] || !candidates[key][1] {
		t.Fatalf("live entitlement assignments = %#v, want unconditional and divergent conditional values", candidates[key])
	}
	if candidates[key][2] {
		t.Fatal("conditional ?= assignment shadowed by unconditional value remained a candidate")
	}
}

func TestSigningXCConfigEntitlementCandidatesKeepConditionalValueBeforeSameSelectorDefault(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = first.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= fallback.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if !candidates[key][0] {
		t.Fatalf("same-selector conditional assignment = %#v, want the earlier value retained", candidates[key])
	}
	if candidates[key][1] {
		t.Fatal("same-selector ?= fallback incorrectly replaced the earlier conditional value")
	}
}

func TestSigningXCConfigEntitlementCandidatesKeepSameSelectorInheritedAssignment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = first.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited) appended.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if !candidates[key][0] || !candidates[key][1] {
		t.Fatalf("same-selector inherited assignments = %#v, want both assignments retained", candidates[key])
	}
}

func TestSigningXCConfigEntitlementCandidatesUseInheritedBaseState(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = other.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] ?= shadowed.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	projectConfiguration := &versionConfiguration{
		id:           "project-debug",
		name:         "Debug",
		projectLevel: true,
		buildSettings: serialized.Object{
			"CODE_SIGN_ENTITLEMENTS": "safe.entitlements",
		},
	}
	targetConfiguration := &versionConfiguration{
		id:     "widget-debug",
		target: "Widget",
		name:   "Debug",
	}
	project := &structuredVersionProject{
		rootDir:        root,
		configurations: []*versionConfiguration{projectConfiguration, targetConfiguration},
	}
	resolver := newSigningSettingResolver(project, map[string][]string{targetConfiguration.id: {path}}, false, nil)
	base, err := resolver.resolveXCConfigBaseWithContext(
		targetConfiguration,
		targetConfiguration,
		path,
		"CODE_SIGN_ENTITLEMENTS",
	)
	if err != nil {
		t.Fatalf("resolveXCConfigBaseWithContext() error = %v", err)
	}
	if !base.found {
		t.Fatalf("inherited base = %#v, want a found project-level value", base)
	}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(targetConfiguration, []string{path}, resolver, base)
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if !candidates[key][0] {
		t.Fatalf("divergent conditional assignment = %#v, want protected candidate", candidates[key])
	}
	if candidates[key][1] {
		t.Fatal("conditional ?= assignment remained live despite an inherited project-level value")
	}
}

func TestSigningXCConfigEntitlementCandidatesReplayRepeatedIncludesInOrder(t *testing.T) {
	root := t.TempDir()
	rootPath := filepath.Join(root, "Root.xcconfig")
	childPath := filepath.Join(root, "Child.xcconfig")
	if err := os.WriteFile(rootPath, []byte(
		"#include \"Child.xcconfig\"\n"+
			"CODE_SIGN_ENTITLEMENTS = safe.entitlements\n"+
			"#include \"Child.xcconfig\"\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = other.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Root.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(childPath, []byte("CODE_SIGN_ENTITLEMENTS = child.entitlements\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(Child.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {rootPath, childPath}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{rootPath, childPath}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	rootKey := normalizeSigningLexicalPath(rootPath)
	childKey := normalizeSigningLexicalPath(childPath)
	if candidates[rootKey][1] {
		t.Fatal("unconditional value shadowed by a later repeated include remained a candidate")
	}
	if !candidates[childKey][0] || !candidates[rootKey][3] {
		t.Fatalf("live entitlement assignments = %#v, want repeated include and divergent selector", candidates)
	}
}

func TestSigningXCConfigEntitlementCandidatesKeepOnlyFinalRepeatedSelectorAssignment(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = iphone.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = iphone-final.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=macosx*] = macos.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if candidates[key][0] {
		t.Fatal("earlier same-selector assignment remained a candidate")
	}
	if !candidates[key][1] || !candidates[key][2] {
		t.Fatalf("live entitlement assignments = %#v, want final same-selector and different-selector values", candidates[key])
	}
}

func TestSigningXCConfigEntitlementCandidatesRespectSelectorSpecificity(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = iphone.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*][arch=arm64] ?= arm64-default.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if !candidates[key][0] || !candidates[key][1] {
		t.Fatalf("selector-specific assignments = %#v, want both conditional values", candidates[key])
	}
}

func TestSigningXCConfigEntitlementCandidatesNormalizeReorderedSelectors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Widget.xcconfig")
	if err := os.WriteFile(path, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*][arch=arm64] = first.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[arch=arm64][sdk=iphoneos*] = final.entitlements\n"+"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*][arch=x86_64] = different.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {path}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{path}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	key := normalizeSigningLexicalPath(path)
	if candidates[key][0] {
		t.Fatal("earlier assignment with reordered equivalent selectors remained a candidate")
	}
	if !candidates[key][1] || !candidates[key][2] {
		t.Fatalf("selector candidates = %#v, want final equivalent and genuinely different selectors", candidates[key])
	}
}

func TestSigningXCConfigEntitlementCandidatesKeepCaseDistinctPaths(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	runtimeGOOS = "windows"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
	})

	root := t.TempDir()
	upperPath := filepath.Join(root, "Widget.xcconfig")
	lowerPath := filepath.Join(root, "widget.xcconfig")
	if _, err := os.Lstat(lowerPath); err == nil {
		t.Skip("temporary filesystem is case-insensitive")
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Lstat(case variant) error = %v", err)
	}
	if err := os.WriteFile(upperPath, []byte(
		"#include \"widget.xcconfig\"\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = upper.entitlements\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(lowerPath, []byte("CODE_SIGN_ENTITLEMENTS[sdk=macosx*] = lower.entitlements\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(widget.xcconfig) error = %v", err)
	}
	previousReader := signingXCConfigReadFileFn
	signingXCConfigReadFileFn = func(path string, _ int64) ([]byte, error) {
		switch filepath.Clean(path) {
		case upperPath:
			return []byte("#include \"widget.xcconfig\"\nCODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = upper.entitlements\n"), nil
		case lowerPath:
			return []byte("CODE_SIGN_ENTITLEMENTS[sdk=macosx*] = lower.entitlements\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}
	t.Cleanup(func() { signingXCConfigReadFileFn = previousReader })
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {upperPath, lowerPath}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{upperPath, lowerPath}, resolver, xcconfigResolvedValue{})
	if err != nil {
		t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v", err)
	}
	if !candidates[normalizeSigningLexicalPath(upperPath)][1] || !candidates[normalizeSigningLexicalPath(lowerPath)][0] {
		t.Fatalf("case-distinct candidates = %#v, want both exact paths", candidates)
	}
}

func TestSigningXCConfigEntitlementCandidatesCoalesceCaseVariantSelfInclude(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousIdentity := signingXCConfigIdentityFn
	previousReader := signingXCConfigReadFileFn
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingXCConfigIdentityFn = previousIdentity
		signingXCConfigReadFileFn = previousReader
	})

	for _, platform := range []string{"darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			runtimeGOOS = platform
			signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return true, true }

			root := t.TempDir()
			rootPath := filepath.Join(root, "Widget.xcconfig")
			if err := os.WriteFile(rootPath, []byte(
				"#include \"widget.xcconfig\"\n"+
					"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = widget.entitlements\n",
			), 0o640); err != nil {
				t.Fatalf("WriteFile(Widget.xcconfig) error = %v", err)
			}
			identity, err := os.Stat(rootPath)
			if err != nil {
				t.Fatalf("Stat(Widget.xcconfig) error = %v", err)
			}
			variantPath := filepath.Join(root, "widget.xcconfig")
			var readPaths []string
			signingXCConfigReadFileFn = func(path string, _ int64) ([]byte, error) {
				readPaths = append(readPaths, filepath.Clean(path))
				return os.ReadFile(path)
			}
			signingXCConfigIdentityFn = func(path string) (os.FileInfo, error) {
				if filepath.Clean(path) == variantPath {
					return identity, nil
				}
				return previousIdentity(path)
			}
			project := &structuredVersionProject{rootDir: root}
			resolver := newSigningSettingResolver(project, map[string][]string{"widget": {rootPath}}, false, nil)
			configuration := &versionConfiguration{id: "widget"}
			candidates, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{rootPath}, resolver, xcconfigResolvedValue{})
			if err != nil {
				t.Fatalf("signingXCConfigEntitlementAssignmentCandidates() error = %v, want case-variant self-include to coalesce", err)
			}
			if !candidates[normalizeSigningLexicalPath(rootPath)][1] {
				t.Fatalf("case-variant self-include candidates = %#v, want root assignment", candidates)
			}
			if len(readPaths) != 1 || readPaths[0] != normalizeSigningLexicalPath(rootPath) {
				t.Fatalf("case-variant self-include reads = %#v, want one canonical read", readPaths)
			}
		})
	}
}

func TestSigningXCConfigEntitlementCandidatesRejectUncollectedPathBeforeProbes(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousIdentity := signingXCConfigIdentityFn
	previousReader := signingXCConfigReadFileFn
	previousStat := signingXCConfigStatFileFn
	runtimeGOOS = "darwin"
	caseChecks := 0
	identityChecks := 0
	readChecks := 0
	statChecks := 0
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) {
		caseChecks++
		return true, true
	}
	signingXCConfigIdentityFn = func(string) (os.FileInfo, error) {
		identityChecks++
		return nil, errors.New("identity probe must not run")
	}
	signingXCConfigReadFileFn = func(string, int64) ([]byte, error) {
		readChecks++
		return nil, errors.New("read probe must not run")
	}
	signingXCConfigStatFileFn = func(string) (os.FileInfo, error) {
		statChecks++
		return nil, errors.New("stat probe must not run")
	}
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingXCConfigIdentityFn = previousIdentity
		signingXCConfigReadFileFn = previousReader
		signingXCConfigStatFileFn = previousStat
	})

	root := t.TempDir()
	authorizedPath := filepath.Join(root, "Widget.xcconfig")
	// Use a clearly external path: a Darwin case-insensitive volume probe must
	// not be needed to establish that it is unauthorized.
	uncollectedPath := filepath.Join(t.TempDir(), "Outside.xcconfig")
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{"widget": {authorizedPath}}, false, nil)
	configuration := &versionConfiguration{id: "widget"}
	_, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{uncollectedPath}, resolver, xcconfigResolvedValue{})
	if err == nil {
		t.Fatal("uncollected case-equivalent xcconfig unexpectedly accepted")
	}
	if caseChecks != 0 || identityChecks != 0 || readChecks != 0 || statChecks != 0 {
		t.Fatalf("unauthorized xcconfig probes = case:%d identity:%d read:%d stat:%d, want all zero", caseChecks, identityChecks, readChecks, statChecks)
	}
}

func TestSigningXCConfigEntitlementCandidatesDoNotReadIncludeCollectedForAnotherConfiguration(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousIdentity := signingXCConfigIdentityFn
	previousReader := signingXCConfigReadFileFn
	previousStat := signingXCConfigStatFileFn
	runtimeGOOS = "darwin"
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	readPaths := make([]string, 0, 1)
	statPaths := make([]string, 0, 1)
	identityPaths := make([]string, 0, 1)
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingXCConfigIdentityFn = previousIdentity
		signingXCConfigReadFileFn = previousReader
		signingXCConfigStatFileFn = previousStat
	})

	root := t.TempDir()
	appPath := filepath.Join(root, "App.xcconfig")
	otherPath := filepath.Join(root, "Other.xcconfig")
	if err := os.WriteFile(appPath, []byte("#include \"Other.xcconfig\"\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("CODE_SIGN_ENTITLEMENTS = other.entitlements\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(Other.xcconfig) error = %v", err)
	}
	signingXCConfigReadFileFn = func(path string, _ int64) ([]byte, error) {
		path = filepath.Clean(path)
		readPaths = append(readPaths, path)
		if path == appPath {
			return []byte("#include \"Other.xcconfig\"\n"), nil
		}
		return nil, errors.New("unexpected cross-configuration read")
	}
	signingXCConfigStatFileFn = func(path string) (os.FileInfo, error) {
		statPaths = append(statPaths, filepath.Clean(path))
		return os.Stat(path)
	}
	signingXCConfigIdentityFn = func(path string) (os.FileInfo, error) {
		identityPaths = append(identityPaths, filepath.Clean(path))
		return os.Stat(path)
	}
	project := &structuredVersionProject{rootDir: root}
	resolver := newSigningSettingResolver(project, map[string][]string{
		"app":   {appPath},
		"other": {otherPath},
	}, false, nil)
	configuration := &versionConfiguration{id: "app"}
	_, _, err := signingXCConfigEntitlementAssignmentCandidates(configuration, []string{appPath}, resolver, xcconfigResolvedValue{})
	if err == nil {
		t.Fatal("cross-configuration include unexpectedly resolved")
	}
	if len(readPaths) != 1 || readPaths[0] != appPath {
		t.Fatalf("cross-configuration reads = %#v, want only %q", readPaths, appPath)
	}
	if len(statPaths) != 0 || len(identityPaths) != 1 || identityPaths[0] != appPath {
		t.Fatalf("cross-configuration probes = stat:%#v identity:%#v, want only authorized root identity %q", statPaths, identityPaths, appPath)
	}
}

func TestSigningPlanRejectsSelectedUnresolvedEntitlements(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"), `CODE_SIGN_ENTITLEMENTS = "$(MISSING_ENTITLEMENTS)";`)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(t.TempDir(), "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "unresolved build-setting reference") || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS") {
		t.Fatalf("BuildSigningPlan() error = %v, want selected entitlement resolution failure", err)
	}
}

func TestSigningPlanClassifiesXCConfigEntitlementPathByConsumer(t *testing.T) {
	tests := []struct {
		name         string
		selectWidget bool
		wantPlan     bool
		wantReady    bool
		wantError    bool
	}{
		{name: "unselected", wantPlan: true, wantReady: false},
		{name: "selected", selectWidget: true, wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, true)
			projectRoot := filepath.Dir(project)
			widgetXCConfig := filepath.Join(projectRoot, "Configs", "Widget.xcconfig")
			if err := os.WriteFile(widgetXCConfig, []byte("CODE_SIGN_ENTITLEMENTS = ../Widget.entitlements\n"), 0o640); err != nil {
				t.Fatalf("write widget xcconfig error = %v", err)
			}
			pbxprojPath := filepath.Join(project, "project.pbxproj")
			contents := mustReadVersionTestFile(t, pbxprojPath)
			const widgetReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
			fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Widget.xcconfig; sourceTree = SOURCE_ROOT; };\n"
			marker := "\t\t111111111111111111111111 /* Project object */ = {"
			if !strings.Contains(contents, marker) {
				t.Fatalf("project fixture is missing project object marker")
			}
			contents = strings.Replace(contents, marker, fileReference+marker, 1)
			widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
			updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
			if !strings.Contains(contents, widgetConfiguration) {
				t.Fatalf("project fixture is missing Widget Debug configuration")
			}
			contents = strings.Replace(contents, widgetConfiguration, updatedWidgetConfiguration, 1)
			if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
				t.Fatalf("write project error = %v", err)
			}

			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			targets := `{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}`
			if test.selectWidget {
				targets += `,{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}`
			}
			writeSigningSettingsTestFile(t, settingsPath, `{"schemaVersion":1,"targets":[`+targets+`]}`)

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
			})
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS") {
					t.Fatalf("BuildSigningPlan() error = %v, want selected invalid entitlement failure", err)
				}
				return
			}
			if err != nil {
				if test.wantPlan {
					if !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried") {
						t.Fatalf("BuildSigningPlan() error = %v, want fail-closed unselected entitlement error", err)
					}
					return
				}
				t.Fatalf("BuildSigningPlan() error = %v, want selected invalid entitlement failure", err)
			}
			if (plan != nil) != test.wantPlan || plan == nil || plan.Ready != test.wantReady {
				t.Fatalf("plan = %#v, want present=%t ready=%t", plan, test.wantPlan, test.wantReady)
			}
			if !strings.Contains(strings.Join(plan.Blockers, "\n"), "Widget") || !strings.Contains(strings.Join(plan.Blockers, "\n"), "CODE_SIGN_ENTITLEMENTS") {
				t.Fatalf("plan blockers = %#v, want unselected invalid-entitlement blocker", plan.Blockers)
			}
		})
	}
}

func TestSigningPlanDoesNotAuthorizeSharedXCConfigPerSelectedConfiguration(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o644); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"DEVELOPMENT_TEAM":"ABCDE12345"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("expected one target-level change per requested setting, got %#v", plan.Changes)
	}
	for _, change := range plan.Changes {
		if change.Source != "pbxproj" {
			t.Fatalf("shared xcconfig change was authorized by unrelated selected configuration: %#v", change)
		}
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "shared xcconfig") {
		t.Fatalf("expected shared xcconfig safety warning, got %#v", plan.Warnings)
	}
}

func TestSigningPlanRejectsReusedConfigurationIDAcrossTargetConsumers(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const widgetConfigurations = "buildConfigurations = (999999999999999999999995, 999999999999999999999996);"
	const reusedWidgetConfigurations = "buildConfigurations = (999999999999999999999993, 999999999999999999999996);"
	if !strings.Contains(contents, widgetConfigurations) {
		t.Fatal("project fixture is missing Widget configuration list")
	}
	contents = strings.Replace(contents, widgetConfigurations, reusedWidgetConfigurations, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE = Automatic\n#include \"Shared.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(t.TempDir(), "state")
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err == nil || !strings.Contains(err.Error(), "reused by multiple project or target consumers") {
		t.Fatalf("BuildSigningPlan() error = %v, want reused-configuration rejection", err)
	}
	if plan != nil {
		t.Fatalf("BuildSigningPlan() plan = %#v, want no plan on ambiguous configuration identity", plan)
	}
	if _, statErr := os.Stat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after reused-configuration rejection = %v, want absent", statErr)
	}
}

func TestSigningPlanRejectsReusedConfigurationIDAcrossProjectAndTargetConsumers(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const projectConfigurations = "buildConfigurations = (999999999999999999999991, 999999999999999999999992);"
	const reusedProjectConfigurations = "buildConfigurations = (999999999999999999999993, 999999999999999999999992);"
	if !strings.Contains(contents, projectConfigurations) {
		t.Fatal("project fixture is missing project configuration list")
	}
	contents = strings.Replace(contents, projectConfigurations, reusedProjectConfigurations, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE = Automatic\n#include \"Shared.xcconfig\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(t.TempDir(), "state")
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err == nil || !strings.Contains(err.Error(), "reused by multiple project or target consumers") {
		t.Fatalf("BuildSigningPlan() error = %v, want reused-configuration rejection", err)
	}
	if plan != nil {
		t.Fatalf("BuildSigningPlan() plan = %#v, want no plan on ambiguous configuration identity", plan)
	}
	if _, statErr := os.Stat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after reused-configuration rejection = %v, want absent", statErr)
	}
}

func TestSigningPlanDoesNotRewriteSharedXCConfigPastNoOpConsumer(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o644); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"automatic"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("expected only the conflicting Release override, got %#v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.Configuration != "Release" || change.Source != "pbxproj" || change.Setting != "CODE_SIGN_STYLE" {
		t.Fatalf("unexpected no-op consumer resolution: %#v", change)
	}
}

func TestSigningPlanExternalXCConfigRequiresOptInForPlanAndApply(t *testing.T) {
	requireStrictSigningPlatform(t)
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, externalPath)
	if err := os.WriteFile(externalPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o640); err != nil {
		t.Fatalf("write external shared xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "blocked"),
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig") {
		t.Fatalf("BuildSigningPlan() without opt-in error = %v, want generic unauthorized external failure", err)
	}
	if _, statErr := os.Lstat(filepath.Join(root, "blocked", "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("blocked plan artifact after unauthorized external config = %v, want absent", statErr)
	}

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "authorized"),
		AllowExternalXCConfig: true,
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() with opt-in error = %v", err)
	}
	if !plan.Ready || len(plan.Changes) != 2 {
		t.Fatalf("external xcconfig with opt-in = ready=%t changes=%#v blockers=%#v", plan.Ready, plan.Changes, plan.Blockers)
	}
	for _, change := range plan.Changes {
		if change.Source != "xcconfig" || change.Path != externalPath {
			t.Fatalf("authorized external change = %#v, want xcconfig %s", change, externalPath)
		}
	}
	if len(plan.Warnings) == 0 || !strings.Contains(strings.Join(plan.Warnings, "\n"), "external xcconfig") {
		t.Fatalf("authorized external warnings = %#v, want explicit warning", plan.Warnings)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}

	before := mustReadVersionTestFile(t, externalPath)
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err == nil || !strings.Contains(err.Error(), "allow-external-xcconfig") {
		t.Fatalf("ApplySigningPlan() without opt-in error = %v, want opt-in rejection", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatal("external xcconfig changed when apply opt-in was omitted")
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath, AllowExternalXCConfig: true}); err != nil {
		t.Fatalf("ApplySigningPlan() with opt-in error = %v", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); !strings.Contains(after, "CODE_SIGN_STYLE = Manual") {
		t.Fatalf("authorized external xcconfig = %q, want Manual", after)
	}
}

func TestSigningPlanRejectsUnauthorizedMalformedExternalXCConfigBeforeReading(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousIdentity := signingXCConfigIdentityFn
	previousStat := signingXCConfigStatFileFn
	previousReader := signingXCConfigReadFileFn
	runtimeGOOS = "darwin"
	caseChecks := 0
	identityChecks := 0
	statChecks := 0
	readChecks := 0
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) {
		caseChecks++
		return true, true
	}
	signingXCConfigIdentityFn = func(string) (os.FileInfo, error) {
		identityChecks++
		return nil, errors.New("identity probe must not run")
	}
	signingXCConfigStatFileFn = func(string) (os.FileInfo, error) {
		statChecks++
		return nil, errors.New("stat probe must not run")
	}
	signingXCConfigReadFileFn = func(string, int64) ([]byte, error) {
		readChecks++
		return nil, errors.New("read probe must not run")
	}
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingXCConfigIdentityFn = previousIdentity
		signingXCConfigStatFileFn = previousStat
		signingXCConfigReadFileFn = previousReader
	})

	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	if err := os.WriteFile(externalPath, []byte("/* unterminated\n"), 0o640); err != nil {
		t.Fatalf("write malformed external xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig") {
		t.Fatalf("BuildSigningPlan() error = %v, want generic unauthorized external failure without reading source", err)
	}
	if caseChecks != 0 || identityChecks != 0 || statChecks != 0 || readChecks != 0 {
		t.Fatalf("unauthorized external probes = case:%d identity:%d stat:%d read:%d, want all zero", caseChecks, identityChecks, statChecks, readChecks)
	}
}

func TestSigningPlanRejectsCaseVariantRootOnSensitiveDarwinBeforeArtifactOverwrite(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousIdentity := signingXCConfigIdentityFn
	previousStat := signingXCConfigStatFileFn
	previousReader := signingXCConfigReadFileFn
	previousProspective := signingResolveProspectivePathFn
	previousArtifactInfo := signingArtifactPathInfoFn
	runtimeGOOS = "darwin"
	caseChecks := 0
	identityChecks := 0
	statChecks := 0
	readChecks := 0
	prospectiveExternalChecks := 0
	artifactExternalChecks := 0
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) {
		caseChecks++
		return false, true
	}
	signingXCConfigIdentityFn = func(path string) (os.FileInfo, error) {
		identityChecks++
		t.Fatalf("case-sensitive external xcconfig identity probe: %s", path)
		return nil, errors.New("identity probe must not run")
	}
	signingXCConfigStatFileFn = func(path string) (os.FileInfo, error) {
		statChecks++
		t.Fatalf("case-sensitive external xcconfig stat probe: %s", path)
		return nil, errors.New("stat probe must not run")
	}
	signingXCConfigReadFileFn = func(path string, limit int64) ([]byte, error) {
		readChecks++
		t.Fatalf("case-sensitive external xcconfig read probe: %s", path)
		return nil, errors.New("read probe must not run")
	}
	signingResolveProspectivePathFn = func(path string) (string, error) {
		if strings.Contains(strings.ToLower(path), "app.xcconfig") {
			prospectiveExternalChecks++
			t.Fatalf("case-sensitive external xcconfig prospective probe: %s", path)
		}
		return previousProspective(path)
	}
	signingArtifactPathInfoFn = func(path string) (os.FileInfo, error) {
		if strings.Contains(strings.ToLower(path), "app.xcconfig") {
			artifactExternalChecks++
			t.Fatalf("case-sensitive external xcconfig artifact probe: %s", path)
		}
		return previousArtifactInfo(path)
	}
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingXCConfigIdentityFn = previousIdentity
		signingXCConfigStatFileFn = previousStat
		signingXCConfigReadFileFn = previousReader
		signingResolveProspectivePathFn = previousProspective
		signingArtifactPathInfoFn = previousArtifactInfo
	})

	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	rootParent := filepath.Dir(projectRoot)
	rootName := filepath.Base(rootParent)
	variantName := strings.ToLower(rootName[:1]) + rootName[1:]
	if variantName == rootName {
		variantName = strings.ToUpper(rootName[:1]) + rootName[1:]
	}
	if variantName == rootName {
		t.Skip("temporary directory parent has no case variant")
	}
	variantRoot := filepath.Join(filepath.Dir(rootParent), variantName, filepath.Base(projectRoot))
	variantConfig := filepath.Join(variantRoot, "Configs", "App.xcconfig")
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const reference = "path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT;"
	replacement := "path = \"" + variantConfig + "\"; sourceTree = \"<absolute>\";"
	if !strings.Contains(contents, reference) {
		t.Fatalf("project fixture is missing App.xcconfig source-root reference")
	}
	contents = strings.Replace(contents, reference, replacement, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(root, "state")
	planPath := filepath.Join(stateDir, "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(state) error = %v", err)
	}
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: stateDir,
	})
	if err == nil || !strings.Contains(err.Error(), signingUnauthorizedExternalXCConfigMessage) {
		t.Fatalf("BuildSigningPlan() error = %v, want case-sensitive external authorization failure", err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after case-sensitive external rejection: %q", got)
	}
	if identityChecks != 0 || statChecks != 0 || readChecks != 0 || prospectiveExternalChecks != 0 || artifactExternalChecks != 0 {
		t.Fatalf("case-sensitive external probes = identity:%d stat:%d read:%d prospective:%d artifact:%d, want all zero", identityChecks, statChecks, readChecks, prospectiveExternalChecks, artifactExternalChecks)
	}
	if caseChecks == 0 {
		t.Fatal("case-sensitive model did not exercise directory case validation")
	}
}

func TestSigningPlanBlocksUnselectedExternalXCConfigWithoutReadingIt(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	if err := os.WriteFile(externalPath, []byte("/* unterminated\n"), 0o640); err != nil {
		t.Fatalf("write malformed external xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig") {
		t.Fatalf("BuildSigningPlan() error = %v, want generic unauthorized external failure", err)
	}
}

func TestSigningPlanBlocksUnselectedEscapingXCConfigSymlink(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	configPath := filepath.Join(projectRoot, "Configs", "App.xcconfig")
	externalPath := filepath.Join(t.TempDir(), "App.xcconfig")
	if err := os.WriteFile(externalPath, []byte("/* unterminated external config\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(external) error = %v", err)
	}
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("Remove(config) error = %v", err)
	}
	if err := os.Symlink(externalPath, configPath); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(t.TempDir(), "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig") {
		t.Fatalf("BuildSigningPlan() error = %v, want generic unauthorized external failure", err)
	}
}

func TestSigningPlanRejectsUnselectedExternalXCConfigArtifactCollision(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	before := mustReadVersionTestFile(t, externalPath)
	previousReader := signingXCConfigReadFileFn
	previousStat := signingXCConfigStatFileFn
	previousProspective := signingResolveProspectivePathFn
	previousArtifactInfo := signingArtifactPathInfoFn
	signingXCConfigReadFileFn = func(path string, limit int64) ([]byte, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig read during artifact collision: %s", path)
		}
		return previousReader(path, limit)
	}
	signingXCConfigStatFileFn = func(path string) (os.FileInfo, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig stat/open during artifact collision: %s", path)
		}
		return previousStat(path)
	}
	signingResolveProspectivePathFn = func(path string) (string, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig prospective inspection during artifact collision: %s", path)
		}
		return previousProspective(path)
	}
	signingArtifactPathInfoFn = func(path string) (os.FileInfo, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig artifact inspection: %s", path)
		}
		return previousArtifactInfo(path)
	}
	t.Cleanup(func() {
		signingXCConfigReadFileFn = previousReader
		signingXCConfigStatFileFn = previousStat
		signingResolveProspectivePathFn = previousProspective
		signingArtifactPathInfoFn = previousArtifactInfo
	})
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: externalPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want protected-input collision", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatalf("external config changed after artifact collision: %q", after)
	}
}

func TestSigningPlanRejectsAllowedUnselectedMalformedExternalArtifactCollision(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	if err := os.WriteFile(externalPath, []byte("/* unterminated\n"), 0o640); err != nil {
		t.Fatalf("write malformed external xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	before := mustReadVersionTestFile(t, externalPath)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: externalPath,
		StateDir: filepath.Join(root, "state"), AllowExternalXCConfig: true,
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want protected-input collision before artifact inspection", err)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatalf("external malformed config changed after artifact collision: %q", after)
	}
}

func TestSigningPlanRejectsAllowedMissingExternalIncludeArtifactCollision(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	missingInclude := filepath.Join(externalDir, "Missing.xcconfig")
	if err := os.WriteFile(externalPath, []byte("#include \"Missing.xcconfig\"\n"), 0o640); err != nil {
		t.Fatalf("write external xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, ReceiptPath: missingInclude,
		StateDir: filepath.Join(root, "state"), AllowExternalXCConfig: true,
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want missing-include collision before artifact inspection", err)
	}
	if _, statErr := os.Lstat(missingInclude); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing include after collision = %v, want absent", statErr)
	}
}

func TestSigningPlanRejectsBlockedExternalPlanAliasToMissingEntitlements(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	if err := os.WriteFile(externalPath, []byte("/* unterminated\n"), 0o640); err != nil {
		t.Fatalf("write malformed external xcconfig error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	marker := `999999999999999999999993 /* App Debug */ = {`
	start := strings.Index(contents, marker)
	if start < 0 {
		t.Fatalf("project fixture is missing App Debug configuration")
	}
	settingMarker := "buildSettings = {  };"
	relative := strings.Index(contents[start:], settingMarker)
	if relative < 0 {
		t.Fatalf("project fixture is missing App Debug build settings")
	}
	settingStart := start + relative
	updatedContents := contents[:settingStart] + "buildSettings = { CODE_SIGN_ENTITLEMENTS = \"plan.json\"; };" + contents[settingStart+len(settingMarker):]
	if err := os.WriteFile(pbxprojPath, []byte(updatedContents), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	planPath := filepath.Join(filepath.Dir(project), "plan.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, PlanPath: planPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want missing-entitlement alias rejection", err)
	}
	if _, statErr := os.Lstat(planPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("missing entitlement artifact after collision = %v, want absent", statErr)
	}
}

func TestSigningPlanKeepsAllowedExternalCollectionFailureBlockedWithoutOverwrite(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	before := "/* unterminated\n"
	if err := os.WriteFile(externalPath, []byte(before), 0o640); err != nil {
		t.Fatalf("write malformed external xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
		AllowExternalXCConfig: true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want incomplete opted-in collection to fail closed", plan, err)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after incomplete opted-in collection = %v, want absent", statErr)
	}
	if after := mustReadVersionTestFile(t, externalPath); after != before {
		t.Fatalf("external malformed config changed while refusing publication: %q", after)
	}
}

func TestSigningPlanContinuesXCConfigProtectionAfterSelectedCollectionFailure(t *testing.T) {
	tests := []struct {
		name          string
		planPath      bool
		receiptPath   bool
		allowExternal bool
	}{
		{name: "base path collides with plan", planPath: true},
		{name: "missing include collides with receipt", receiptPath: true, allowExternal: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project, externalBase, missingInclude := writeSigningMultiConfigFailureProject(t)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			stateDir := filepath.Join(root, "state")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			opts := SigningPlanOptions{
				ProjectPath:           project,
				SettingsFilePath:      settingsPath,
				StateDir:              stateDir,
				AllowExternalXCConfig: test.allowExternal,
			}
			if test.planPath {
				opts.PlanPath = externalBase
			}
			if test.receiptPath {
				opts.ReceiptPath = missingInclude
			}

			_, err := BuildSigningPlan(opts)
			if err == nil || !strings.Contains(err.Error(), "protected project input") {
				t.Fatalf("BuildSigningPlan() error = %v, want lexical protected-path collision", err)
			}
			if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("plan artifact after collision = %v, want absent", statErr)
			}
			if _, statErr := os.Lstat(filepath.Join(stateDir, "receipt.json")); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("receipt artifact after collision = %v, want absent", statErr)
			}
			if test.planPath {
				if got := mustReadVersionTestFile(t, externalBase); !strings.Contains(got, "MissingInclude.xcconfig") {
					t.Fatalf("external base changed after collision: %q", got)
				}
			} else if _, statErr := os.Lstat(missingInclude); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("missing include after collision = %v, want absent", statErr)
			}
		})
	}

	project, _, _ := writeSigningMultiConfigFailureProject(t)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath:           project,
		SettingsFilePath:      settingsPath,
		StateDir:              stateDir,
		AllowExternalXCConfig: true,
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want incomplete opted-in collection to fail closed", plan, err)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after incomplete opted-in collection = %v, want absent", statErr)
	}
}

func writeSigningMultiConfigFailureProject(t *testing.T) (projectPath, externalBase, missingInclude string) {
	t.Helper()
	projectPath = writeStructuredVersionProject(t, true)
	externalDir := t.TempDir()
	externalBase = filepath.Join(externalDir, "Later.xcconfig")
	missingInclude = filepath.Join(externalDir, "MissingInclude.xcconfig")
	if err := os.WriteFile(externalBase, []byte("#include \"MissingInclude.xcconfig\"\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(external base) error = %v", err)
	}

	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReferences := `
			CCCCCCCCCCCCCCCCCCCCCCCC /* MissingSelected.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/MissingSelected.xcconfig; sourceTree = SOURCE_ROOT; };
			DDDDDDDDDDDDDDDDDDDDDDDD /* Later.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = "` + externalBase + `"; sourceTree = "<absolute>"; };`
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReferences+"\n"+marker, 1)
	selectedOld := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA;"
	selectedNew := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = CCCCCCCCCCCCCCCCCCCCCCCC;"
	if !strings.Contains(contents, selectedOld) {
		t.Fatalf("project fixture is missing selected App Debug base reference")
	}
	contents = strings.Replace(contents, selectedOld, selectedNew, 1)
	widgetOld := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	widgetNew := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = DDDDDDDDDDDDDDDDDDDDDDDD; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(contents, widgetOld) {
		t.Fatalf("project fixture is missing later Widget Debug configuration")
	}
	contents = strings.Replace(contents, widgetOld, widgetNew, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
	return projectPath, externalBase, missingInclude
}

func TestSigningApplyUsesMetadataPreservingWrites(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o640); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}, {"name":"Widget","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
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

	originalWriter := atomicWriteVersionFileFn
	var writes []preparedVersionWrite
	atomicWriteVersionFileFn = func(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
		writes = append(writes, write)
		return originalWriter(write, data)
	}
	t.Cleanup(func() { atomicWriteVersionFileFn = originalWriter })
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if len(writes) != 2 {
		t.Fatalf("observed writes = %d, want project and xcconfig writes", len(writes))
	}
	for _, write := range writes {
		if !write.preserveMetadata {
			t.Fatalf("signing write %s did not request metadata preservation", write.path)
		}
	}
}

func TestSigningApplyPreflightsMetadataPreservationBeforeWrites(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	aliasPath := filepath.Join(t.TempDir(), "project.pbxproj")
	if err := os.Link(pbxprojPath, aliasPath); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
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
	before := mustReadVersionTestFile(t, pbxprojPath)
	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "multiply linked") {
		t.Fatalf("ApplySigningPlan() error = %v, want metadata preflight refusal", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != before {
		t.Fatal("metadata preflight failure changed project before the transaction")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after metadata preflight failure = %v, want absent", statErr)
	}
}

func TestSigningApplyRejectsHardLinkAddedAfterPreparation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("strict signing apply fails closed before replacement on Windows")
	}
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
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
	beforeProject := mustReadVersionTestFile(t, pbxprojPath)
	aliasPath := filepath.Join(root, "project-alias.pbxproj")
	originalHook := beforeSigningCommitForTest
	beforeSigningCommitForTest = func() {
		beforeSigningCommitForTest = nil
		if err := os.Link(pbxprojPath, aliasPath); err != nil {
			t.Skipf("hard links are unavailable: %v", err)
		}
	}
	t.Cleanup(func() { beforeSigningCommitForTest = originalHook })

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "hard-link state changed") {
		t.Fatalf("ApplySigningPlan() error = %v, want late hard-link rejection", err)
	}
	if after := mustReadVersionTestFile(t, pbxprojPath); after != beforeProject {
		t.Fatal("late hard-link rejection changed the project")
	}
	if after := mustReadVersionTestFile(t, aliasPath); after != beforeProject {
		t.Fatal("late hard-link rejection changed the alias")
	}
	if _, statErr := os.Lstat(plan.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after late hard-link rejection = %v, want absent", statErr)
	}
}

func TestSigningPlanAnchorsEntitlementsToProjectRoot(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(t *testing.T, projectRoot string) string
		wantBlocker string
		wantError   bool
	}{
		{
			name:        "missing",
			setup:       func(_ *testing.T, _ string) string { return "App.entitlements" },
			wantBlocker: "no such file",
		},
		{
			name: "directory",
			setup: func(t *testing.T, projectRoot string) string {
				if err := os.Mkdir(filepath.Join(projectRoot, "App.entitlements"), 0o755); err != nil {
					t.Fatalf("Mkdir() error = %v", err)
				}
				return "App.entitlements"
			},
			wantBlocker: "not a regular file",
			wantError:   true,
		},
		{
			name: "symlink",
			setup: func(t *testing.T, projectRoot string) string {
				target := filepath.Join(t.TempDir(), "real.entitlements")
				if err := os.WriteFile(target, []byte("{}\n"), 0o644); err != nil {
					t.Fatalf("WriteFile(target) error = %v", err)
				}
				link := filepath.Join(projectRoot, "App.entitlements")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
				return "App.entitlements"
			},
			wantBlocker: "symlink",
			wantError:   true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			projectRoot := filepath.Dir(project)
			entitlements := test.setup(t, projectRoot)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_ENTITLEMENTS":"`+entitlements+`"}}]}]
			}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
			})
			if test.wantError {
				if err == nil {
					t.Fatal("BuildSigningPlan() error = nil, want alias-inspection failure")
				}
				if !strings.Contains(err.Error(), test.wantBlocker) {
					t.Fatalf("BuildSigningPlan() error = %v, want %q", err, test.wantBlocker)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v", err)
			}
			if plan.Ready {
				t.Fatalf("entitlements %s unexpectedly produced ready plan: %#v", test.name, plan)
			}
			if !strings.Contains(strings.Join(plan.Blockers, "\n"), test.wantBlocker) {
				t.Fatalf("entitlements blockers = %#v, want %q", plan.Blockers, test.wantBlocker)
			}
		})
	}
}

func TestSigningSettingsManifestIsStrictAndValidatesValues(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "unknown field",
			body: `{"schemaVersion":1,"extra":true,"targets":[]}`,
			want: "unknown field",
		},
		{
			name: "unsupported setting",
			body: `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"OTHER":"x"}}]}]}`,
			want: "unsupported signing setting",
		},
		{
			name: "null required value",
			body: `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"DEVELOPMENT_TEAM":null}}]}]}`,
			want: "does not support null",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			writeSigningSettingsTestFile(t, path, test.body)
			manifest, err := readSigningSettingsManifest(path)
			if err == nil {
				_, _, err = normalizeSigningRequests(manifest)
			}
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("readSigningSettingsManifest() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestSigningSettingsManifestRejectsDuplicateNestedJSONKeys(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_STYLE":"manual",
			"CODE_SIGN_STYLE":"automatic"
		}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		StateDir: filepath.Join(filepath.Dir(settingsPath), "state"),
	})
	if err == nil || !IsSigningInputError(err) {
		t.Fatalf("BuildSigningPlan() error = %v, want duplicate-key signing input classification", err)
	}
	if !strings.Contains(err.Error(), "duplicate JSON object key") || !strings.Contains(err.Error(), "CODE_SIGN_STYLE") {
		t.Fatalf("BuildSigningPlan() error = %v, want nested duplicate-key diagnostic", err)
	}
}

func TestSigningSettingsManifestRejectsDuplicateTopLevelJSONKeys(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	settingsPath := filepath.Join(t.TempDir(), "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
	})
	if err == nil || !IsSigningInputError(err) {
		t.Fatalf("BuildSigningPlan() error = %v, want duplicate-key signing input classification", err)
	}
	if !strings.Contains(err.Error(), "duplicate JSON object key") || !strings.Contains(err.Error(), "schemaVersion") {
		t.Fatalf("BuildSigningPlan() error = %v, want top-level duplicate-key diagnostic", err)
	}
}

func TestBuildSigningPlanMarksDeterministicManifestValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"schemaVersion":1,"targets": [`},
		{name: "wrong schema", body: `{"schemaVersion":2,"targets":[]}`},
		{name: "unsupported setting", body: `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"OTHER":"x"}}]}]}`},
		{name: "invalid team ID", body: `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"DEVELOPMENT_TEAM":"short"}}]}]}`},
		{name: "invalid entitlements path", body: `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_ENTITLEMENTS":"../App.entitlements"}}]}]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			settingsPath := filepath.Join(t.TempDir(), "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, test.body)
			_, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
			})
			if err == nil || !IsSigningInputError(err) {
				t.Fatalf("BuildSigningPlan() error = %v, want signing input classification", err)
			}
		})
	}
}

func writeSigningSettingsTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

// injectSigningDirectBuildSetting adds a direct assignment to one existing
// XCBuildConfiguration so a test can model a project that keeps a pbxproj-level
// setting alongside its base xcconfig.
func injectSigningDirectBuildSetting(t *testing.T, pbxprojPath, assignment string) {
	t.Helper()
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993", assignment)
}

func injectSigningBuildSetting(t *testing.T, pbxprojPath, configurationID, assignment string) {
	t.Helper()
	data := mustReadVersionTestFile(t, pbxprojPath)
	start := strings.Index(data, configurationID)
	if start < 0 {
		t.Fatalf("configuration %s not found in project", configurationID)
	}
	const marker = "buildSettings = {"
	offset := strings.Index(data[start:], marker)
	if offset < 0 {
		t.Fatalf("buildSettings not found for configuration %s", configurationID)
	}
	insert := start + offset + len(marker)
	updated := data[:insert] + " " + assignment + data[insert:]
	if err := os.WriteFile(pbxprojPath, []byte(updated), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}
}

func TestSigningPlanRetainsInheritedDirectConsumerInSharedConflict(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o644); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	// Debug keeps a direct assignment that still defers to the shared value.
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_STYLE = "$(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"automatic"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 1 {
		t.Fatalf("expected only the conflicting Release override, got %#v", plan.Changes)
	}
	change := plan.Changes[0]
	if change.Configuration != "Release" {
		t.Fatalf("unexpected configuration in change: %#v", change)
	}
	// Rewriting the shared file would also change Debug's inherited value, so
	// the plan must use a target-level override instead.
	if change.Source != "pbxproj" {
		t.Fatalf("shared xcconfig rewritten past an inherited-direct consumer: %#v", change)
	}
}

func TestSigningPlanMaterializesSharedMutationWhenInheritedNoOpWouldChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	sharedBefore := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("PROVISIONING_PROFILE_SPECIFIER = Base\r\n"+sharedBefore), 0o640); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`PROVISIONING_PROFILE_SPECIFIER = "Prefix $(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":"Prefix Base"}},
			{"name":"Release","settings":{"PROVISIONING_PROFILE_SPECIFIER":"Prefix Base"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Configuration != "Release" || plan.Changes[0].Source != "pbxproj" {
		t.Fatalf("plan changes = %#v, want one Release target-level materialization", plan.Changes)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if got := mustReadVersionTestFile(t, sharedPath); got != "PROVISIONING_PROFILE_SPECIFIER = Base\r\n"+sharedBefore {
		t.Fatalf("shared xcconfig changed during target-level materialization: %q", got)
	}

	secondPlan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "second-state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan(second) error = %v", err)
	}
	if !secondPlan.Ready || len(secondPlan.Changes) != 0 {
		t.Fatalf("second plan = ready=%t changes=%#v blockers=%#v, want both effective values preserved", secondPlan.Ready, secondPlan.Changes, secondPlan.Blockers)
	}
}

func TestSigningPlanDigestsXCConfigsUsedOnlyForResolution(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	configDir := filepath.Join(filepath.Dir(project), "Configs")
	sharedPath := filepath.Join(configDir, "Shared.xcconfig")
	appPath := filepath.Join(configDir, "App.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o644); err != nil {
		t.Fatalf("write shared xcconfig error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}},
			{"name":"Release","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}

	digested := make(map[string]bool, len(plan.Files))
	for _, file := range plan.Files {
		digested[file.Path] = true
	}
	// Shared.xcconfig carries the assignment and is rewritten. App.xcconfig only
	// includes it, but a later overriding assignment there would change the
	// effective value, so it must be digested and rechecked too.
	if !digested[sharedPath] {
		t.Fatalf("mutated xcconfig missing from plan files: %#v", plan.Files)
	}
	if !digested[appPath] {
		t.Fatalf("resolution-only xcconfig %s missing from plan files: %#v", appPath, plan.Files)
	}
}

func TestSigningPlanRevalidatesUnselectedConsumerInput(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	widgetPath := attachSigningWidgetXCConfig(t, project, "CODE_SIGN_STYLE = Automatic\n")
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if err := os.WriteFile(widgetPath, []byte("CODE_SIGN_STYLE = Manual\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(widget xcconfig) error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApplySigningPlan() error = %v, want stale-plan rejection", err)
	}
}

func TestSigningPlanRevalidatesNoOpReferenceAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningDirectBuildSetting(t, pbxprojPath,
		`PRODUCT_BUNDLE_IDENTIFIER = com.example.old; CODE_SIGN_IDENTITY = "$(PRODUCT_BUNDLE_IDENTIFIER)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a reference-dependent identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want literal requested value", identityChange)
	}
}

func TestSigningPlanReclassifiesNoOpAfterDependentRemoval(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_IDENTITY = com.example.old; PROVISIONING_PROFILE_SPECIFIER = "$(CODE_SIGN_IDENTITY)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":null,
			"PROVISIONING_PROFILE_SPECIFIER":"com.example.old"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got blockers %#v", plan.Blockers)
	}
	var identityRemoval, profileChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityRemoval = change
		case "PROVISIONING_PROFILE_SPECIFIER":
			profileChange = change
		}
	}
	if identityRemoval == nil || identityRemoval.Operation != "remove" {
		t.Fatalf("identity change = %#v, want direct removal", identityRemoval)
	}
	if profileChange == nil || profileChange.Source != "pbxproj" || profileChange.NewValue == nil || *profileChange.NewValue != "com.example.old" {
		t.Fatalf("profile change = %#v, want promoted target-level literal", profileChange)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	if _, _, err := updated.resolveSetting(configuration, "CODE_SIGN_IDENTITY"); !errors.Is(err, errVersionSettingNotFound) {
		t.Fatalf("resolved identity error = %v, want setting removal", err)
	}
	profile, _, err := updated.resolveSetting(configuration, "PROVISIONING_PROFILE_SPECIFIER")
	if err != nil {
		t.Fatalf("resolve applied profile: %v", err)
	}
	if profile != "com.example.old" {
		t.Fatalf("applied profile = %q, want requested no-op value", profile)
	}
}

// TestSigningPlanDefersRemovalFallbackBlockerUntilNoOpPromotion covers the
// convergence case where a removal exposes a fallback changed by a sibling
// operation while that same staged pass promotes a dependent no-op. The
// promotion must be applied before the fallback comparison is treated as a
// final blocker: the target-level literal then restores the removal-only
// fallback and the next pass can converge safely.
func TestSigningPlanDefersRemovalFallbackBlockerUntilNoOpPromotion(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = \"$(PROVISIONING_PROFILE_SPECIFIER)\"\r\n" +
		"CODE_SIGN_IDENTITY = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" +
		"PROVISIONING_PROFILE_SPECIFIER = com.example.old\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	// Removing the direct identity exposes the shared identity fallback. The
	// target's bundle identifier initially resolves to com.example.old through
	// the shared profile, so it is a no-op until the requested profile change
	// stages; that change makes the bundle identifier a dependent no-op that must
	// be promoted to preserve its requested value.
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_IDENTITY = debug-profile;`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":null,
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.old",
			"PROVISIONING_PROFILE_SPECIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected converged ready plan, got blockers %#v", plan.Blockers)
	}
	if len(plan.Changes) != 3 {
		t.Fatalf("plan changes = %#v, want identity removal plus profile and promoted bundle-ID writes", plan.Changes)
	}
	var identityRemoval, bundleChange, profileChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityRemoval = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		case "PROVISIONING_PROFILE_SPECIFIER":
			profileChange = change
		}
	}
	if identityRemoval == nil || identityRemoval.Operation != "remove" {
		t.Fatalf("identity change = %#v, want direct removal", identityRemoval)
	}
	if bundleChange == nil || bundleChange.Source != "pbxproj" || bundleChange.NewValue == nil || *bundleChange.NewValue != "com.example.old" {
		t.Fatalf("bundle-ID change = %#v, want promoted target-level literal", bundleChange)
	}
	if profileChange == nil || profileChange.NewValue == nil || *profileChange.NewValue != "com.example.new" {
		t.Fatalf("profile change = %#v, want requested profile", profileChange)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	for _, expected := range []struct {
		setting string
		value   string
	}{
		{setting: "PRODUCT_BUNDLE_IDENTIFIER", value: "com.example.old"},
		{setting: "PROVISIONING_PROFILE_SPECIFIER", value: "com.example.new"},
		{setting: "CODE_SIGN_IDENTITY", value: "com.example.old"},
	} {
		resolved, _, err := updated.resolveSetting(configuration, expected.setting)
		if err != nil {
			t.Fatalf("resolve applied %s: %v", expected.setting, err)
		}
		if resolved != expected.value {
			t.Fatalf("applied %s = %q, want %q", expected.setting, resolved, expected.value)
		}
	}
}

func TestSigningPlanBlocksRemovalWhenSiblingWriteChangesSharedFallback(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	if err := os.WriteFile(sharedPath, []byte("PROVISIONING_PROFILE_SPECIFIER = base-profile\r\n"), 0o640); err != nil {
		t.Fatalf("write shared xcconfig: %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`PROVISIONING_PROFILE_SPECIFIER = debug-profile;`)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":null}},
			{"name":"Release","settings":{"PROVISIONING_PROFILE_SPECIFIER":"release-profile"}}
		]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.Ready || !strings.Contains(strings.Join(plan.Blockers, "\n"), "differs from value after removal alone") {
		t.Fatalf("plan = ready=%t blockers=%#v, want shared fallback collision blocker", plan.Ready, plan.Blockers)
	}
}

func TestSigningPlanRemovalOnlyUsesFallbackAfterRemoval(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	if err := os.WriteFile(sharedPath, []byte("PROVISIONING_PROFILE_SPECIFIER = base-profile\r\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"), `PROVISIONING_PROFILE_SPECIFIER = debug-profile;`)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{"schemaVersion":1,"targets":[{"name":"App","configurations":[{"name":"Debug","settings":{"PROVISIONING_PROFILE_SPECIFIER":null}}]}]}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready {
		t.Fatalf("removal-only plan blocked: %#v", plan.Blockers)
	}
	var removal *SigningSettingChange
	for index := range plan.Changes {
		if plan.Changes[index].Setting == "PROVISIONING_PROFILE_SPECIFIER" {
			removal = &plan.Changes[index]
			break
		}
	}
	if removal == nil || removal.Operation != "remove" {
		t.Fatalf("plan changes = %#v, want provisioning-profile removal", plan.Changes)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatal(err)
	}
	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatal(err)
	}
	fallback, _, err := updated.resolveSetting(configuration, "PROVISIONING_PROFILE_SPECIFIER")
	if err != nil || fallback != "base-profile" {
		t.Fatalf("effective fallback = %q, err=%v; want base-profile", fallback, err)
	}
}

func TestSigningRemovalFallbackChangedDetectsMissingToPresent(t *testing.T) {
	if !signingRemovalFallbackChanged("staged", nil, "", errVersionSettingNotFound) {
		t.Fatal("missing baseline with staged value must be treated as a fallback collision")
	}
}

func TestSigningRemovalFallbackChangedDetectsErrorToEmpty(t *testing.T) {
	if !signingRemovalFallbackChanged("", nil, "", errors.New("baseline resolution failed")) {
		t.Fatal("successful empty staged value after failed baseline must be treated as changed")
	}
}

func TestSigningPlanReclassifiesNoOpAfterProjectFallbackDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	// The target inherits its identity from the project configuration. The
	// project-level identity expands PRODUCT_BUNDLE_IDENTIFIER in the selected
	// target context, so changing the target's bundle identifier changes the
	// value that initially looked like a no-op.
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`PRODUCT_BUNDLE_IDENTIFIER = com.example.old; CODE_SIGN_IDENTITY = "$(PRODUCT_BUNDLE_IDENTIFIER)";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_IDENTITY = "$(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a project-fallback identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want requested value", identityChange)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	configuration, err := signingConfigurationFor(structured, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor() error = %v", err)
	}
	resolver := newSigningSettingResolver(structured, nil, false, nil)
	resolvedIdentity, _, err := resolver.resolveSetting(configuration, "CODE_SIGN_IDENTITY")
	if err != nil {
		t.Fatalf("resolveSetting(CODE_SIGN_IDENTITY) error = %v", err)
	}
	if resolvedIdentity != "com.example.old" {
		t.Fatalf("resolved identity = %q, want com.example.old", resolvedIdentity)
	}
	resolvedBundleID, _, err := resolver.resolveSetting(configuration, "PRODUCT_BUNDLE_IDENTIFIER")
	if err != nil {
		t.Fatalf("resolveSetting(PRODUCT_BUNDLE_IDENTIFIER) error = %v", err)
	}
	if resolvedBundleID != "com.example.new" {
		t.Fatalf("resolved bundle ID = %q, want com.example.new", resolvedBundleID)
	}
}

func TestSigningPlanSecondPassHashAndStaleDependency(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`PRODUCT_BUNDLE_IDENTIFIER = com.example.old; CODE_SIGN_IDENTITY = "$(PRODUCT_BUNDLE_IDENTIFIER)";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_IDENTITY = "$(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.PlanHash == "" || plan.PlanHash != signingPlanHash(plan) {
		t.Fatalf("plan hash = %q, recomputed = %q", plan.PlanHash, signingPlanHash(plan))
	}
	repeated, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("repeated BuildSigningPlan() error = %v", err)
	}
	if repeated.PlanHash != plan.PlanHash {
		t.Fatalf("repeated plan hash = %q, want %q", repeated.PlanHash, plan.PlanHash)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	contents := mustReadVersionTestFile(t, pbxprojPath)
	mutated := strings.Replace(contents, "PRODUCT_BUNDLE_IDENTIFIER = com.example.old;", "PRODUCT_BUNDLE_IDENTIFIER = com.example.changed;", 1)
	if mutated == contents {
		t.Fatal("failed to mutate project dependency for stale-plan check")
	}
	if err := os.WriteFile(pbxprojPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApplySigningPlan() error = %v, want stale-plan rejection", err)
	}
	if got := mustReadVersionTestFile(t, pbxprojPath); got != mutated {
		t.Fatal("stale apply changed the project")
	}
}

func TestSigningApplyRejectsStaleXCConfigAndManifest(t *testing.T) {
	t.Run("xcconfig input", func(t *testing.T) {
		project := writeStructuredVersionProject(t, true)
		sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
		shared := mustReadVersionTestFile(t, sharedPath)
		if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_STYLE = Automatic\r\n"+shared), 0o640); err != nil {
			t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
		}
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
		if plan.PlanHash == "" || plan.PlanHash != signingPlanHash(plan) {
			t.Fatalf("plan hash = %q, recomputed = %q", plan.PlanHash, signingPlanHash(plan))
		}
		if err := WriteSigningPlanArtifact(plan, false); err != nil {
			t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
		}
		beforeProject := mustReadVersionTestFile(t, filepath.Join(project, "project.pbxproj"))
		beforeShared := mustReadVersionTestFile(t, sharedPath)
		mutatedShared := beforeShared + "// stale source\r\n"
		if err := os.WriteFile(sharedPath, []byte(mutatedShared), 0o640); err != nil {
			t.Fatalf("mutate shared xcconfig: %v", err)
		}
		if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("ApplySigningPlan() error = %v, want stale xcconfig rejection", err)
		}
		if got := mustReadVersionTestFile(t, filepath.Join(project, "project.pbxproj")); got != beforeProject {
			t.Fatal("stale xcconfig apply changed the project")
		}
		if got := mustReadVersionTestFile(t, sharedPath); got != mutatedShared {
			t.Fatal("stale xcconfig apply changed the source again")
		}
	})

	t.Run("manifest input", func(t *testing.T) {
		project := writeStructuredVersionProject(t, false)
		root := t.TempDir()
		settingsPath := filepath.Join(root, "settings.json")
		settings := `{
			"schemaVersion": 1,
			"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
		}`
		writeSigningSettingsTestFile(t, settingsPath, settings)
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
		beforeProject := mustReadVersionTestFile(t, pbxprojPath)
		mutatedSettings := strings.Replace(settings, `"CODE_SIGN_STYLE":"manual"`, `"CODE_SIGN_STYLE":"automatic"`, 1)
		if mutatedSettings == settings {
			t.Fatal("failed to mutate manifest for stale-plan check")
		}
		if err := os.WriteFile(settingsPath, []byte(mutatedSettings), 0o644); err != nil {
			t.Fatalf("mutate settings file: %v", err)
		}
		if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err == nil || !strings.Contains(err.Error(), "stale") {
			t.Fatalf("ApplySigningPlan() error = %v, want stale manifest rejection", err)
		}
		if got := mustReadVersionTestFile(t, pbxprojPath); got != beforeProject {
			t.Fatal("stale manifest apply changed the project")
		}
		if got := mustReadVersionTestFile(t, settingsPath); got != mutatedSettings {
			t.Fatal("stale manifest apply changed the source again")
		}
	})
}

func TestSigningPlanRevalidatesTransitiveNoOpReferenceAfterDependentChange(t *testing.T) {
	requireStrictSigningPlatform(t)
	project := writeStructuredVersionProject(t, false)
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`PRODUCT_BUNDLE_IDENTIFIER = com.example.old; IDENTITY_ALIAS = "$(PRODUCT_BUNDLE_IDENTIFIER)"; CODE_SIGN_IDENTITY = "$(IDENTITY_ALIAS)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a transitive reference-dependent identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want literal target-level value", identityChange)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if result == nil || !result.Completed {
		t.Fatalf("ApplySigningPlan() result = %#v, want completed receipt", result)
	}

	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	identity, _, err := newSigningSettingResolver(updated, nil, false, nil).resolveSetting(configuration, "CODE_SIGN_IDENTITY")
	if err != nil {
		t.Fatalf("resolve applied identity: %v", err)
	}
	if identity != "com.example.old" {
		t.Fatalf("applied identity = %q, want requested no-op value", identity)
	}
}

func TestSigningPlanRevalidatesNoOpXCConfigReferenceAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nCODE_SIGN_IDENTITY = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated an xcconfig reference-dependent identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want a literal target-level value", identityChange)
	}
}

func TestSigningPlanAppliesPromotedXCConfigNoOpAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nPROVISIONING_PROFILE_SPECIFIER = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"PROVISIONING_PROFILE_SPECIFIER":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	var profileChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "PROVISIONING_PROFILE_SPECIFIER":
			profileChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if profileChange == nil || bundleChange == nil {
		t.Fatalf("plan omitted dependent signing changes: %#v", plan.Changes)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if result == nil || !result.Completed {
		t.Fatalf("ApplySigningPlan() result = %#v, want completed receipt", result)
	}
	projectPath := filepath.Join(project, "project.pbxproj")
	projectChanged := false
	for _, path := range result.ChangedFiles {
		if path == projectPath {
			projectChanged = true
			break
		}
	}
	if !projectChanged {
		t.Fatalf("changed files = %#v, want project update", result.ChangedFiles)
	}

	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	profile, _, err := updated.resolveSetting(configuration, "PROVISIONING_PROFILE_SPECIFIER")
	if err != nil {
		t.Fatalf("resolve applied profile: %v", err)
	}
	if profile != "com.example.old" {
		t.Fatalf("applied profile = %q, want requested no-op value", profile)
	}
	bundleID, _, err := updated.resolveSetting(configuration, "PRODUCT_BUNDLE_IDENTIFIER")
	if err != nil {
		t.Fatalf("resolve applied bundle ID: %v", err)
	}
	if bundleID != "com.example.new" {
		t.Fatalf("applied bundle ID = %q, want com.example.new", bundleID)
	}
}

func TestSigningPlanRevalidatesTransitiveNoOpXCConfigReferenceAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nIDENTITY_ALIAS = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\nCODE_SIGN_IDENTITY = \"$(IDENTITY_ALIAS)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a transitive xcconfig identity reference as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want literal target-level value", identityChange)
	}
}

func TestSigningPlanAppliesMultipleTransitiveXCConfigNoOpsAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nIDENTITY_ALIAS = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\nCODE_SIGN_IDENTITY = \"$(IDENTITY_ALIAS)\"\r\nPROVISIONING_PROFILE_SPECIFIER = \"$(CODE_SIGN_IDENTITY)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PROVISIONING_PROFILE_SPECIFIER":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	if len(plan.Changes) != 3 {
		t.Fatalf("changes = %#v, want bundle ID plus two promoted settings", plan.Changes)
	}
	for _, change := range plan.Changes {
		if change.Setting != "CODE_SIGN_IDENTITY" && change.Setting != "PROVISIONING_PROFILE_SPECIFIER" && change.Setting != "PRODUCT_BUNDLE_IDENTIFIER" {
			t.Fatalf("unexpected signing change = %#v", change)
		}
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	for _, expected := range []struct {
		setting string
		value   string
	}{
		{setting: "CODE_SIGN_IDENTITY", value: "com.example.old"},
		{setting: "PROVISIONING_PROFILE_SPECIFIER", value: "com.example.old"},
		{setting: "PRODUCT_BUNDLE_IDENTIFIER", value: "com.example.new"},
	} {
		resolved, _, err := updated.resolveSetting(configuration, expected.setting)
		if err != nil {
			t.Fatalf("resolve applied %s: %v", expected.setting, err)
		}
		if resolved != expected.value {
			t.Fatalf("applied %s = %q, want %q", expected.setting, resolved, expected.value)
		}
	}
}

func TestSigningPlanRevalidatesProjectXCConfigFallbackReferenceAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	projectConfigPath := filepath.Join(projectRoot, "Configs", "Project.xcconfig")
	if err := os.WriteFile(projectConfigPath, []byte("PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nIDENTITY_ALIAS = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\nCODE_SIGN_IDENTITY = \"$(IDENTITY_ALIAS)\"\r\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(project xcconfig) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReference := `
		BBBBBBBBBBBBBBBBBBBBBBBB /* Project.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Project.xcconfig; sourceTree = SOURCE_ROOT; };`
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+"\n"+marker, 1)
	projectConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	updatedConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB; buildSettings = {}; name = Debug; };"
	if !strings.Contains(contents, projectConfiguration) {
		t.Fatal("project fixture is missing project Debug configuration")
	}
	contents = strings.Replace(contents, projectConfiguration, updatedConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted project fallback bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a project fallback reference-dependent identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want literal target-level value", identityChange)
	}
}

func TestSigningPlanRevalidatesTransitiveNoOpReferenceThroughInheritedAlias(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nIDENTITY_ALIAS = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`IDENTITY_ALIAS = "$(inherited)"; CODE_SIGN_IDENTITY = "$(IDENTITY_ALIAS)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated an inherited transitive identity reference as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want literal target-level value", identityChange)
	}
}

func TestSigningPlanKeepsStableInheritedAliasNoOp(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "IDENTITY_ALIAS = \"Apple Development\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`IDENTITY_ALIAS = "$(inherited)"; CODE_SIGN_IDENTITY = "$(IDENTITY_ALIAS)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"Apple Development"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("stable inherited alias produced a spurious change: %#v", plan.Changes)
	}
}

func TestSigningPlanTerminatesProjectXCConfigInheritedTraversal(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	projectConfigPath := filepath.Join(projectRoot, "Configs", "Project.xcconfig")
	if err := os.WriteFile(projectConfigPath, []byte(
		"DEVELOPMENT_TEAM = ABCDE12345\n"+
			"CODE_SIGN_IDENTITY = Apple Development\n"+
			"CODE_SIGN_IDENTITY = $(inherited) $(DEVELOPMENT_TEAM)\n",
	), 0o640); err != nil {
		t.Fatalf("WriteFile(project xcconfig) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReference := `
		BBBBBBBBBBBBBBBBBBBBBBBB /* Project.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Project.xcconfig; sourceTree = SOURCE_ROOT; };`
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+"\n"+marker, 1)
	projectConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	updatedConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB; buildSettings = {}; name = Debug; };"
	if !strings.Contains(contents, projectConfiguration) {
		t.Fatal("project fixture is missing project Debug configuration")
	}
	contents = strings.Replace(contents, projectConfiguration, updatedConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"Apple Development ABCDE12345",
			"DEVELOPMENT_TEAM":"XYZAB67890"
		}}]}]
	}`)

	if _, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	}); err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want inherited project xcconfig traversal to terminate", err)
	}
}

func TestSigningPlanMaterializesTopLevelInheritedIdentityWhenTeamChanges(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`DEVELOPMENT_TEAM = ABCDE12345; CODE_SIGN_IDENTITY = "Prefix $(DEVELOPMENT_TEAM)";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_IDENTITY = "Outer $(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	stateDir := filepath.Join(root, "state")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"Outer Prefix ABCDE12345",
			"DEVELOPMENT_TEAM":"XYZAB67890"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: stateDir,
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, teamChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "DEVELOPMENT_TEAM":
			teamChange = change
		}
	}
	if teamChange == nil {
		t.Fatalf("plan omitted team change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated a top-level $(inherited) identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "Outer Prefix ABCDE12345" {
		t.Fatalf("identity change = %#v, want a literal target-level value", identityChange)
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	result, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath})
	if err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	if result == nil || !result.Completed {
		t.Fatalf("ApplySigningPlan() result = %#v, want completed receipt", result)
	}

	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	identity, _, err := newSigningSettingResolver(updated, nil, false, nil).resolveSetting(configuration, "CODE_SIGN_IDENTITY")
	if err != nil {
		t.Fatalf("resolve applied identity: %v", err)
	}
	if identity != "Outer Prefix ABCDE12345" {
		t.Fatalf("applied identity = %q, want requested no-op value", identity)
	}
}

func TestSigningPlanRevalidatesInheritedNoOpXCConfigReferenceAfterDependentChange(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nCODE_SIGN_IDENTITY = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	// The direct assignment preserves the target's existing scope while
	// inheriting the effective identity from Shared.xcconfig.
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_IDENTITY = "$(inherited)";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.example.old",
			"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	var identityChange, bundleChange *SigningSettingChange
	for index := range plan.Changes {
		change := &plan.Changes[index]
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			identityChange = change
		case "PRODUCT_BUNDLE_IDENTIFIER":
			bundleChange = change
		}
	}
	if bundleChange == nil {
		t.Fatalf("plan omitted dependent bundle-ID change: %#v", plan.Changes)
	}
	if identityChange == nil {
		t.Fatalf("plan treated an inherited reference-dependent identity as a no-op: %#v", plan.Changes)
	}
	if identityChange.Source != "pbxproj" || identityChange.NewValue == nil || *identityChange.NewValue != "com.example.old" {
		t.Fatalf("identity change = %#v, want a literal target-level value", identityChange)
	}
}

func TestSigningSettingResolverExpandsProjectFallbackInSelectedTargetContext(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	// The project-level identity references a setting that the selected target
	// overrides. Xcode expands that reference in the target's effective context,
	// not in the project configuration that supplied the fallback value.
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`PRODUCT_BUNDLE_IDENTIFIER = com.project; CODE_SIGN_IDENTITY = "$(PRODUCT_BUNDLE_IDENTIFIER)";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_BUNDLE_IDENTIFIER = com.target;`)
	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	configuration, err := signingConfigurationFor(structured, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor() error = %v", err)
	}
	resolved, _, err := newSigningSettingResolver(structured, nil, false, nil).resolveSetting(configuration, "CODE_SIGN_IDENTITY")
	if err != nil {
		t.Fatalf("resolveSetting(CODE_SIGN_IDENTITY) error = %v", err)
	}
	if resolved != "com.target" {
		t.Fatalf("resolveSetting(CODE_SIGN_IDENTITY) = %q, want selected-target expansion", resolved)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"com.project"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	for _, change := range plan.Changes {
		if change.Setting != "CODE_SIGN_IDENTITY" {
			continue
		}
		if change.OldValue == nil || *change.OldValue != "com.target" {
			t.Fatalf("CODE_SIGN_IDENTITY old value = %#v, want selected-target expansion", change.OldValue)
		}
		if change.NewValue == nil || *change.NewValue != "com.project" {
			t.Fatalf("CODE_SIGN_IDENTITY new value = %#v, want requested project value", change.NewValue)
		}
		return
	}
	t.Fatalf("plan treated project fallback identity as a no-op: %#v", plan.Changes)
}

func TestSigningPlanRejectsBuildSettingReferenceModifiers(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	configuration, err := signingConfigurationFor(structured, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor() error = %v", err)
	}
	resolver := newSigningSettingResolver(structured, nil, false, nil)
	_, _, err = resolver.expandSettingReferences(configuration, "$(MARKETING_VERSION:rfc1034identifier)", map[string]bool{})
	if err == nil || !strings.Contains(err.Error(), "reference modifier") {
		t.Fatalf("expandSettingReferences() error = %v, want unsupported modifier", err)
	}
}

func TestSigningSettingResolverRejectsIncompleteBuildSettingReferences(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	configuration, err := signingConfigurationFor(structured, "App", "Debug")
	if err != nil {
		t.Fatalf("signingConfigurationFor() error = %v", err)
	}
	configuration.buildSettings["KNOWN_SETTING"] = "resolved"
	resolver := newSigningSettingResolver(structured, nil, false, nil)
	tests := []struct {
		name      string
		value     string
		wantError bool
		wantValue string
	}{
		{name: "incomplete parenthesis", value: "$(MISSING", wantError: true},
		{name: "incomplete brace", value: "${MISSING", wantError: true},
		{name: "resolved then truncated", value: "prefix-$(KNOWN_SETTING)-${", wantError: true},
		{name: "ordinary dollar", value: "price $5", wantValue: "price $5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, _, err := resolver.expandSettingReferences(configuration, test.value, map[string]bool{})
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "incomplete build-setting reference") {
					t.Fatalf("expandSettingReferences(%q) error = %v, want incomplete-reference error", test.value, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expandSettingReferences(%q) error = %v, want nil", test.value, err)
			}
			if value != test.wantValue {
				t.Fatalf("expandSettingReferences(%q) = %q, want %q", test.value, value, test.wantValue)
			}
		})
	}
}

func TestSigningPlanBlocksIncompleteBuildSettingReference(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"), `CODE_SIGN_IDENTITY = "$(MISSING";`)
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_IDENTITY":"Apple Development"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want blocked plan", err)
	}
	if plan.Ready {
		t.Fatalf("incomplete build-setting reference produced ready plan: %#v", plan)
	}
	blockers := strings.Join(plan.Blockers, "\n")
	if !strings.Contains(blockers, "CODE_SIGN_IDENTITY") || !strings.Contains(blockers, "incomplete build-setting reference") {
		t.Fatalf("plan blockers = %#v, want incomplete identity reference blocker", plan.Blockers)
	}
}

func TestSigningPlanDoesNotSwallowProjectFallbackResolutionError(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const projectDebug = "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	malformedProjectDebug := `999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = { CODE_SIGN_STYLE = "$(MISSING"; }; name = Debug; };`
	if !strings.Contains(contents, projectDebug) {
		t.Fatalf("project fixture is missing project Debug configuration")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, projectDebug, malformedProjectDebug, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	appContents := mustReadVersionTestFile(t, appXCConfig)
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE ?= Automatic\n"+appContents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want blocked plan", err)
	}
	if plan.Ready {
		t.Fatalf("project fallback resolution error was swallowed: %#v", plan)
	}
	blockers := strings.Join(plan.Blockers, "\n")
	if !strings.Contains(blockers, "incomplete build-setting reference") || !strings.Contains(blockers, "CODE_SIGN_STYLE") {
		t.Fatalf("plan blockers = %#v, want project fallback resolution error", plan.Blockers)
	}
}

func TestSigningPlanAllowsOptionalXCConfigAssignmentWithoutProjectFallback(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	appContents := mustReadVersionTestFile(t, appXCConfig)
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE ?= Automatic\n"+appContents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want ready plan", err)
	}
	if !plan.Ready {
		t.Fatalf("optional xcconfig assignment without fallback blocked plan: %#v", plan.Blockers)
	}
}

func TestSigningPlanAllowsDirectlyMissingOptionalXCConfigInclude(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = strings.Replace(contents, `#include "Shared.xcconfig"`, "#include? \"Missing.xcconfig\"\n#include \"Shared.xcconfig\"", 1)
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}

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
		t.Fatalf("BuildSigningPlan() error = %v, want optional missing include to be ignored", err)
	}
	if !plan.Ready {
		t.Fatalf("optional missing include blocked plan: %#v", plan.Blockers)
	}
	missingPath := filepath.Join(filepath.Dir(appXCConfig), "Missing.xcconfig")
	if len(plan.MissingOptionalIncludes) != 1 || plan.MissingOptionalIncludes[0] != missingPath {
		t.Fatalf("missing optional includes = %#v, want [%q]", plan.MissingOptionalIncludes, missingPath)
	}
}

func TestSigningPlanMissingOptionalIncludesRoundTripAndHash(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "plan.json")
	missingPath := filepath.Join(filepath.Dir(planPath), "Missing.xcconfig")
	plan := &SigningPlan{
		SchemaVersion:           signingPlanSchemaVersion,
		Command:                 signingPlanCommand,
		PlanPath:                planPath,
		MissingOptionalIncludes: []string{missingPath},
	}
	plan.PlanHash = signingPlanHash(plan)
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	read, err := readSigningPlanArtifact(planPath)
	if err != nil {
		t.Fatalf("readSigningPlanArtifact() error = %v", err)
	}
	if len(read.MissingOptionalIncludes) != 1 || read.MissingOptionalIncludes[0] != missingPath {
		t.Fatalf("round-trip missing optional includes = %#v, want [%q]", read.MissingOptionalIncludes, missingPath)
	}
	if read.PlanHash != signingPlanHash(read) {
		t.Fatalf("round-trip plan hash = %q, want %q", read.PlanHash, signingPlanHash(read))
	}
}

func TestSigningPlanLegacyOptionalIncludeFieldKeepsSchemaOneHashCompatibility(t *testing.T) {
	plan := &SigningPlan{
		SchemaVersion: signingPlanSchemaVersion,
		Command:       signingPlanCommand,
		PlanPath:      filepath.Join(t.TempDir(), "plan.json"),
		Ready:         true,
	}
	legacyHash := signingPlanHash(plan)
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("json.Marshal(legacy plan) error = %v", err)
	}
	if strings.Contains(string(data), "missingOptionalIncludes") {
		t.Fatalf("legacy plan unexpectedly serialized empty optional-include field: %s", data)
	}
	var decoded SigningPlan
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(legacy plan) error = %v", err)
	}
	if got := signingPlanHash(&decoded); got != legacyHash {
		t.Fatalf("legacy plan hash = %q, want %q", got, legacyHash)
	}
}

func TestSigningApplyRejectsLegacyPlanOmittingOptionalIncludeAssertion(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig)
	contents = strings.Replace(contents, `#include "Shared.xcconfig"`, "#include? \"Missing.xcconfig\"\n#include \"Shared.xcconfig\"", 1)
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}

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
	if !plan.Ready || len(plan.MissingOptionalIncludes) != 1 {
		t.Fatalf("plan = ready=%t missing=%#v blockers=%#v, want ready plan with one absence assertion", plan.Ready, plan.MissingOptionalIncludes, plan.Blockers)
	}

	projectPath := filepath.Join(project, "project.pbxproj")
	projectBefore := mustReadVersionTestFile(t, projectPath)
	legacy := *plan
	legacy.MissingOptionalIncludes = nil
	legacy.PlanHash = signingPlanHash(&legacy)
	if err := WriteSigningPlanArtifact(&legacy, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact(legacy) error = %v", err)
	}

	_, err = ApplySigningPlan(SigningApplyOptions{PlanPath: legacy.PlanPath})
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("ApplySigningPlan(legacy) error = %v, want stale-plan rejection", err)
	}
	if after := mustReadVersionTestFile(t, projectPath); after != projectBefore {
		t.Fatal("legacy stale-plan rejection modified the project")
	}
	if _, statErr := os.Lstat(legacy.ReceiptPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("receipt after legacy stale-plan rejection = %v, want absent", statErr)
	}
}

func TestReadSigningPlanRejectsUnboundedMissingOptionalIncludes(t *testing.T) {
	tests := []struct {
		name string
		list []string
	}{
		{name: "too many entries", list: make([]string, signingPlanMaxMissingOptionalIncludes+1)},
		{name: "oversized path", list: []string{strings.Repeat("a", signingPlanMaxMissingOptionalIncludePathBytes+1)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			planPath := filepath.Join(t.TempDir(), "plan.json")
			plan := &SigningPlan{
				SchemaVersion:           signingPlanSchemaVersion,
				Command:                 signingPlanCommand,
				MissingOptionalIncludes: test.list,
			}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatalf("json.Marshal(plan) error = %v", err)
			}
			if err := os.WriteFile(planPath, data, 0o600); err != nil {
				t.Fatalf("WriteFile(plan) error = %v", err)
			}
			if _, err := readSigningPlanArtifact(planPath); err == nil || !IsSigningInputError(err) {
				t.Fatalf("readSigningPlanArtifact() error = %v, want bounded input error", err)
			}
		})
	}
}

func TestReadSigningPlanRejectsUnboundedSigningSources(t *testing.T) {
	planPath := filepath.Join(t.TempDir(), "plan.json")
	files := make([]SigningPlanFile, signingPlanMaxFiles+1)
	for index := range files {
		files[index] = SigningPlanFile{Path: fmt.Sprintf("source-%d.xcconfig", index), SHA256: strings.Repeat("0", 64), Source: "xcconfig"}
	}
	plan := &SigningPlan{SchemaVersion: signingPlanSchemaVersion, Command: signingPlanCommand, Files: files}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("json.Marshal(plan) error = %v", err)
	}
	if err := os.WriteFile(planPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}
	if _, err := readSigningPlanArtifact(planPath); err == nil || !IsSigningInputError(err) || !strings.Contains(err.Error(), "signing source files") {
		t.Fatalf("readSigningPlanArtifact() error = %v, want bounded source error", err)
	}
}

func TestSigningSettingResolverDoesNotShareOptionalIncludeAuthorizationBetweenConfigurations(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, true)
	project, err := openStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	app, err := signingConfigurationFor(project, "App", "Debug")
	if err != nil {
		t.Fatalf("find App configuration: %v", err)
	}
	widget, err := signingConfigurationFor(project, "Widget", "Debug")
	if err != nil {
		t.Fatalf("find Widget configuration: %v", err)
	}
	shared := filepath.Join(filepath.Dir(projectPath), "Configs", "LateShared.xcconfig")
	resolver := newSigningSettingResolver(
		project,
		map[string][]string{
			app.id:    {filepath.Join(filepath.Dir(projectPath), "Configs", "App.xcconfig")},
			widget.id: {shared},
		},
		false,
		map[string][]string{app.id: {shared}},
	)
	if err := os.WriteFile(shared, []byte("CODE_SIGN_STYLE = manual\n"), 0o640); err != nil {
		t.Fatalf("write late optional include: %v", err)
	}

	if _, err := resolver.statXCConfigFor(app, shared); err == nil || !strings.Contains(err.Error(), "appeared after configuration collection") {
		t.Fatalf("App stat error = %v, want late-appearance refusal", err)
	}
	if _, err := resolver.readXCConfigFor(app, shared); err == nil || !strings.Contains(err.Error(), "not collected for this configuration") {
		t.Fatalf("App read error = %v, want configuration-bound refusal", err)
	}
	if _, err := resolver.readXCConfigFor(widget, shared); err != nil {
		t.Fatalf("Widget read error = %v, want collected consumer access", err)
	}
}

func TestSigningPlanDoesNotSwallowProjectFallbackForAppendingXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const projectDebug = "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	malformedProjectDebug := `999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = { CODE_SIGN_STYLE = "$(MISSING"; }; name = Debug; };`
	if !strings.Contains(contents, projectDebug) {
		t.Fatalf("project fixture is missing project Debug configuration")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, projectDebug, malformedProjectDebug, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	appContents := mustReadVersionTestFile(t, appXCConfig)
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE += Automatic\n"+appContents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want blocked plan", err)
	}
	if plan.Ready {
		t.Fatalf("appending xcconfig swallowed project fallback resolution error: %#v", plan)
	}
	blockers := strings.Join(plan.Blockers, "\n")
	if !strings.Contains(blockers, "incomplete build-setting reference") || !strings.Contains(blockers, "CODE_SIGN_STYLE") {
		t.Fatalf("plan blockers = %#v, want project fallback resolution error", plan.Blockers)
	}
}

func TestSigningPlanAllowsDirectXCConfigOverrideWithInvalidProjectFallback(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const projectDebug = "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	malformedProjectDebug := `999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = { CODE_SIGN_STYLE = "$(MISSING"; }; name = Debug; };`
	if !strings.Contains(contents, projectDebug) {
		t.Fatalf("project fixture is missing project Debug configuration")
	}
	if err := os.WriteFile(pbxprojPath, []byte(strings.Replace(contents, projectDebug, malformedProjectDebug, 1)), 0o644); err != nil {
		t.Fatalf("write project error = %v", err)
	}
	appXCConfig := filepath.Join(filepath.Dir(project), "Configs", "App.xcconfig")
	appContents := mustReadVersionTestFile(t, appXCConfig)
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_STYLE = Automatic\n"+appContents), 0o644); err != nil {
		t.Fatalf("write app xcconfig error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want ready plan", err)
	}
	if !plan.Ready {
		t.Fatalf("direct xcconfig override incorrectly inherited project fallback error: %#v", plan.Blockers)
	}
}

func TestSigningPlanDigestsProjectLevelXCConfigUsedForResolution(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	projectConfigPath := filepath.Join(projectRoot, "Configs", "Project.xcconfig")
	if err := os.MkdirAll(filepath.Dir(projectConfigPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	if err := os.WriteFile(projectConfigPath, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReference := `
		BBBBBBBBBBBBBBBBBBBBBBBB /* Project.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Project.xcconfig; sourceTree = SOURCE_ROOT; };`
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+"\n"+marker, 1)
	projectConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	updatedConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = BBBBBBBBBBBBBBBBBBBBBBBB; buildSettings = {}; name = Debug; };"
	if !strings.Contains(contents, projectConfiguration) {
		t.Fatal("project fixture is missing project Debug configuration")
	}
	contents = strings.Replace(contents, projectConfiguration, updatedConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project) error = %v", err)
	}

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
	if !plan.Ready {
		t.Fatalf("expected ready plan, got blockers %#v", plan.Blockers)
	}
	for _, file := range plan.Files {
		if file.Path == projectConfigPath {
			return
		}
	}
	t.Fatalf("plan omitted project-level resolution xcconfig %s: %#v", projectConfigPath, plan.Files)
}

func TestOpenSigningStructuredVersionProjectRejectsEscapingSymlinkBeforeParsing(t *testing.T) {
	selectedRoot := t.TempDir()
	outsideRoot := t.TempDir()
	outsideProject := filepath.Join(outsideRoot, "Outside.xcodeproj")
	if err := os.MkdirAll(outsideProject, 0o755); err != nil {
		t.Fatalf("MkdirAll(outside project) error = %v", err)
	}
	// A parser error here proves that the project was opened before its
	// operator-selected root was checked. The preflight must reject the
	// escaping final symlink first.
	if err := os.WriteFile(filepath.Join(outsideProject, "project.pbxproj"), []byte("not a project"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside project) error = %v", err)
	}
	selectedProject := filepath.Join(selectedRoot, "Selected.xcodeproj")
	if err := os.Symlink(outsideProject, selectedProject); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := openSigningStructuredVersionProject(selectedProject)
	if err == nil || !errors.Is(err, rootfs.ErrSymlink) {
		t.Fatalf("openSigningStructuredVersionProject() error = %v, want rooted symlink rejection", err)
	}
}

func TestOpenStructuredVersionProjectPreservesSelectedProjectSymlink(t *testing.T) {
	targetProject := writeStructuredVersionProject(t, false)
	selectedRoot := t.TempDir()
	selectedProject := filepath.Join(selectedRoot, "Selected.xcodeproj")
	if err := os.Symlink(targetProject, selectedProject); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	opened, err := openStructuredVersionProject(selectedProject)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v, want stable symlink compatibility", err)
	}
	selectedAbsolute, err := filepath.Abs(selectedProject)
	if err != nil {
		t.Fatalf("Abs(selected project) error = %v", err)
	}
	if opened.projectPath != filepath.Clean(selectedAbsolute) {
		t.Fatalf("opened project path = %q, want selected spelling %q", opened.projectPath, selectedAbsolute)
	}
}

func TestSigningPlanProtectsConditionalOnlyEntitlementReference(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "Widget.xcconfig")
	protectedPlanPath := filepath.Join(externalDir, "plan.json")
	if err := os.WriteFile(externalPath, []byte("CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(ENTITLEMENTS_FILE)\nENTITLEMENTS_FILE = "+protectedPlanPath+"\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(external xcconfig) error = %v", err)
	}
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const widgetReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
	fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = \"" + externalPath + "\"; sourceTree = \"<absolute>\"; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(contents, widgetConfiguration) {
		t.Fatalf("project fixture is missing Widget Debug configuration")
	}
	contents = strings.Replace(contents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: protectedPlanPath,
		StateDir: filepath.Join(root, "state"), AllowExternalXCConfig: true,
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want conditional entitlement alias rejection", err)
	}
}

func TestSigningPlanRejectsIncompleteUnselectedInternalXCConfigBeforeArtifactOverwrite(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	attachSigningWidgetXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = .asc/xcode/signing/plan.json\n"+
			"BROKEN = \"unterminated\n")
	artifactDir := filepath.Join(projectRoot, ".asc", "xcode", "signing")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifact dir) error = %v", err)
	}
	planPath := filepath.Join(artifactDir, "plan.json")
	const existingEntitlements = "existing entitlement bytes\n"
	if err := os.WriteFile(planPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlement artifact) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want incomplete internal collection to fail closed", plan, err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingEntitlements {
		t.Fatalf("unread entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsUnselectedInheritedConditionalSuffix(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited) Special.entitlements\n")
	protectedPath := filepath.Join(filepath.Dir(project), "Special.entitlements")
	const existingProtected = "existing entitlement bytes\n"
	if err := os.WriteFile(protectedPath, []byte(existingProtected), 0o600); err != nil {
		t.Fatalf("WriteFile(protected entitlement) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	stateDir := filepath.Join(root, "state")
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: protectedPath, StateDir: stateDir,
	})
	if err == nil || !strings.Contains(err.Error(), "protected project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want inherited suffix alias rejection", err)
	}
	if got := mustReadVersionTestFile(t, protectedPath); got != existingProtected {
		t.Fatalf("protected entitlement changed after alias rejection: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("plan artifact after inherited suffix collision = %v, want absent", statErr)
	}
}

func TestSigningPlanRejectsUnselectedUnknownConditionalEntitlementBeforeArtifactOverwrite(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(UNKNOWN_ENTITLEMENTS)/Live.entitlements\n")
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(root, "plan.json")
	stateDir := filepath.Join(root, "state")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: stateDir,
	})
	if err == nil || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried") {
		t.Fatalf("BuildSigningPlan() error = %v, want unbounded entitlement failure", err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after unbounded entitlement failure: %q", got)
	}
	if _, statErr := os.Lstat(filepath.Join(stateDir, "plan.json")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("state plan artifact after unbounded entitlement failure = %v, want absent", statErr)
	}
}

func TestSigningPlanProtectsUnselectedProjectConditionalEntitlementPath(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = ".asc/xcode/signing/plan.json";`)
	appEntitlements := filepath.Join(filepath.Dir(project), "App.entitlements")
	if err := os.WriteFile(appEntitlements, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	projectRoot := filepath.Dir(project)
	artifactDir := filepath.Join(projectRoot, ".asc", "xcode", "signing")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(artifact dir) error = %v", err)
	}
	planPath := filepath.Join(artifactDir, "plan.json")
	const existingEntitlements = "existing entitlement bytes\n"
	if err := os.WriteFile(planPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlement artifact) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want project conditional entitlement path rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingEntitlements {
		t.Fatalf("project conditional entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsUnselectedProjectConditionalEntitlementExpression(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(PRODUCT_NAME).entitlements";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	for _, name := range []string{"App.entitlements", "Widget.entitlements"} {
		entitlementsPath := filepath.Join(projectRoot, name)
		if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	planPath := filepath.Join(projectRoot, "Widget.entitlements")
	const existingEntitlements = "existing widget entitlement bytes\n"
	if err := os.WriteFile(planPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(widget entitlement artifact) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want target-expanded project entitlement expression rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingEntitlements {
		t.Fatalf("target-expanded project entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsTargetConditionalInheritedEntitlementComposition(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Suffix";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = Base;`)
	projectRoot := filepath.Dir(project)
	appEntitlements := filepath.Join(projectRoot, "App.entitlements")
	if err := os.WriteFile(appEntitlements, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	composedPath := filepath.Join(projectRoot, "BaseSuffix")
	const existingEntitlements = "existing composed entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want same-object inherited composition rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsDivergentInheritedProjectEntitlementSeeds(t *testing.T) {
	project := writeDivergentInheritedProjectEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`, "")
	projectRoot := filepath.Dir(project)
	seedBytes := map[string]string{
		"BaseSuffix":  "existing conditional project seed bytes\n",
		"OtherSuffix": "existing unconditional project seed bytes\n",
	}
	for name, contents := range seedBytes {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	tests := []struct {
		name      string
		seed      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan aliases conditional project seed",
			seed: "BaseSuffix",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt aliases unconditional project seed",
			seed: "OtherSuffix",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, filepath.Join(projectRoot, test.seed))
			plan, err := BuildSigningPlan(opts)
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want %s alias rejection", plan, err, test.seed)
			}
			for name, contents := range seedBytes {
				if got := mustReadVersionTestFile(t, filepath.Join(projectRoot, name)); got != contents {
					t.Fatalf("project entitlement seed %s changed while validating %s: %q", name, test.seed, got)
				}
			}
		})
	}
}

func TestSigningPlanProtectsDivergentInheritedProjectXCConfigEntitlementSeeds(t *testing.T) {
	project := writeDivergentInheritedProjectXCConfigEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	seedBytes := map[string]string{
		"BaseSuffix":  "existing conditional project xcconfig seed bytes\n",
		"OtherSuffix": "existing unconditional project xcconfig seed bytes\n",
	}
	for name, contents := range seedBytes {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(contents), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	tests := []struct {
		name      string
		seed      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan aliases conditional project xcconfig seed",
			seed: "BaseSuffix",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt aliases unconditional project xcconfig seed",
			seed: "OtherSuffix",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, filepath.Join(projectRoot, test.seed))
			plan, err := BuildSigningPlan(opts)
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want %s alias rejection", plan, err, test.seed)
			}
			for name, contents := range seedBytes {
				if got := mustReadVersionTestFile(t, filepath.Join(projectRoot, name)); got != contents {
					t.Fatalf("project xcconfig entitlement seed %s changed while validating %s: %q", name, test.seed, got)
				}
			}
		})
	}
}

func TestSigningPlanExpandsReferencedProjectXCConfigEntitlementSeedInTargetContext(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = Other\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	protectedPath := filepath.Join(projectRoot, "WidgetSuffix")
	const existingEntitlements = "existing target-context entitlement seed bytes\n"
	if err := os.WriteFile(protectedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetSuffix) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: protectedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want target-context project xcconfig alias rejection", err)
	}
	if got := mustReadVersionTestFile(t, protectedPath); got != existingEntitlements {
		t.Fatalf("target-context entitlement seed changed during alias validation: %q", got)
	}
}

func TestSigningPlanSkipsShadowedProjectXCConfigReferenceForTargetOverride(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	attachSigningAppXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = ShadowedTarget.entitlements\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App; CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	for _, name := range []string{"App.entitlements", "ShadowedTarget.entitlements", "Widget"} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte("existing project input bytes\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	artifactPath := filepath.Join(projectRoot, "App")
	const existingArtifact = "existing artifact bytes\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(App artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want shadowed project xcconfig reference ignored", err)
			}
			if plan == nil || !plan.Ready {
				t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("artifact path changed while validating shadowed project reference: %q", got)
			}
		})
	}
}

func TestSigningPlanProtectsProjectXCConfigReferenceForUnshadowedTarget(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App; CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "Widget")
	const existingArtifact = "existing unshadowed target artifact bytes\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err == nil || !strings.Contains(err.Error(), "aliases project input") {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want unshadowed Widget project reference rejection", plan, err)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("unshadowed target artifact changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanProtectsProjectXCConfigReferenceWhenTargetExpansionFails(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App;`)
	projectRoot := filepath.Dir(project)
	const existingEntitlements = "existing unresolved target entitlement bytes\n"
	protectedPath := filepath.Join(projectRoot, "Widget")
	if err := os.WriteFile(protectedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, protectedPath)
			plan, err := BuildSigningPlan(opts)
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input") && !strings.Contains(err.Error(), "cannot be safely inventoried")) {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want unresolved Widget input rejected before artifact overwrite", plan, err)
			}
			if got := mustReadVersionTestFile(t, protectedPath); got != existingEntitlements {
				t.Fatalf("unresolved target entitlement was overwritten while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanSkipsProjectXCConfigExpressionWhenAllTargetsOverride(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = Widget.entitlements;`)
	projectRoot := filepath.Dir(project)
	for _, name := range []string{"App.entitlements", "Widget.entitlements"} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want project xcconfig expression shadowed by all target overrides", err)
	}
	if plan == nil || !plan.Ready {
		t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
	}
}

func TestSigningPlanComposesProjectXCConfigReferenceThroughInheritedTarget(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	const existingEntitlements = "existing composed target entitlement bytes\n"
	composedPath := filepath.Join(projectRoot, "WidgetSuffix")
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetSuffix) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "Widget")
	const existingArtifact = "existing valid artifact bytes\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want uncomposed project reference ignored", err)
			}
			if plan == nil || !plan.Ready {
				t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("valid artifact path changed while validating %s: %q", test.name, got)
			}
			if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
				t.Fatalf("composed entitlement input changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanComposesProjectXCConfigReferenceThroughInheritedTargetXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	const existingEntitlements = "existing target xcconfig entitlement bytes\n"
	composedPath := filepath.Join(projectRoot, "WidgetSuffix")
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetSuffix) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "Widget")
	const existingArtifact = "existing valid target xcconfig artifact bytes\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want target xcconfig inherited suffix composed", err)
			}
			if plan == nil || !plan.Ready {
				t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("valid artifact changed while validating %s: %q", test.name, got)
			}
			if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
				t.Fatalf("composed entitlement input changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanSkipsShadowedProjectXCConfigWhenProjectPBXEntitlementOverrides(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = Project.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "Project.entitlementsSuffix"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(Project.entitlementsSuffix) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "WidgetSuffix")
	const existingArtifact = "existing artifact bytes for shadowed project xcconfig\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetSuffix artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want project PBX entitlement to shadow xcconfig expression", err)
			}
			if plan == nil || !plan.Ready {
				t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("valid artifact changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanComposesProjectAndTargetInheritedEntitlements(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)ProjectSuffix";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)TargetSuffix";`)
	projectRoot := filepath.Dir(project)
	tests := []struct {
		name          string
		path          string
		wantAlias     bool
		existingBytes string
	}{
		{
			name:          "bare target suffix is not protected",
			path:          filepath.Join(projectRoot, "WidgetTargetSuffix"),
			wantAlias:     false,
			existingBytes: "existing raw target suffix artifact bytes\n",
		},
		{
			name:          "fully composed target entitlement is protected",
			path:          filepath.Join(projectRoot, "WidgetProjectSuffixTargetSuffix"),
			wantAlias:     true,
			existingBytes: "existing effective target entitlement artifact bytes\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(test.path, []byte(test.existingBytes), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", test.path, err)
			}
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: test.path, StateDir: filepath.Join(root, "state"),
			})
			if test.wantAlias {
				if err == nil || !strings.Contains(err.Error(), "aliases project input") {
					t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want effective inherited entitlement alias rejection", plan, err)
				}
			} else {
				if err != nil {
					t.Fatalf("BuildSigningPlan() error = %v, want bare target suffix accepted", err)
				}
				if plan == nil || !plan.Ready {
					t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
				}
			}
			if got := mustReadVersionTestFile(t, test.path); got != test.existingBytes {
				t.Fatalf("artifact changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanDoesNotDoubleComposeProjectXCConfigThroughProjectPBX(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = Base\n"+
			"CODE_SIGN_ENTITLEMENTS = $(inherited)$(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)ProjectSuffix";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)TargetSuffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	cases := []struct {
		name      string
		path      string
		wantAlias bool
	}{
		{
			name:      "singly composed effective path",
			path:      filepath.Join(projectRoot, "BaseWidgetProjectSuffixTargetSuffix"),
			wantAlias: true,
		},
		{
			name:      "doubly composed phantom path",
			path:      filepath.Join(projectRoot, "BaseWidgetProjectSuffixWidgetProjectSuffixTargetSuffix"),
			wantAlias: false,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			const existingArtifact = "existing artifact bytes for project xcconfig composition\n"
			if err := os.WriteFile(test.path, []byte(existingArtifact), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", test.path, err)
			}
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: test.path, StateDir: filepath.Join(root, "state"),
			})
			if test.wantAlias {
				if err == nil || !strings.Contains(err.Error(), "aliases project input") {
					t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want effective entitlement alias rejection", plan, err)
				}
			} else {
				if err != nil {
					t.Fatalf("BuildSigningPlan() error = %v, want phantom double-composed path ignored", err)
				}
				if plan == nil || !plan.Ready {
					t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
				}
			}
			if got := mustReadVersionTestFile(t, test.path); got != existingArtifact {
				t.Fatalf("artifact changed while validating project xcconfig composition: %q", got)
			}
		})
	}
}

func TestSigningPlanComposesProjectConditionalPBXInheritedEntitlementThroughUnconditional(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Base"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Device";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	composedPath := filepath.Join(projectRoot, "WidgetBaseDevice")
	const existingEntitlements = "existing project conditional inherited entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetBaseDevice) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want conditional PBX inherited entitlement alias rejection", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("conditional inherited entitlement was overwritten during planning: %q", got)
	}
}

func TestSigningProjectPBXInheritedEntitlementValuesComposeConditionalThroughUnconditional(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Base"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Device";`)
	project, err := openSigningStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openSigningStructuredVersionProject() error = %v", err)
	}
	var projectConfiguration, targetConfiguration *versionConfiguration
	for _, configuration := range project.configurations {
		if configuration.projectLevel && configuration.name == "Debug" {
			projectConfiguration = configuration
		}
		if !configuration.projectLevel && configuration.target == "Widget" && configuration.name == "Debug" {
			targetConfiguration = configuration
		}
	}
	if projectConfiguration == nil || targetConfiguration == nil {
		t.Fatal("project fixture is missing Project Debug or Widget Debug configuration")
	}
	resolver := newSigningSettingResolver(project, nil, false, nil)
	values, err := signingProjectPBXInheritedEntitlementValues(projectConfiguration, targetConfiguration, "Widget", resolver)
	if err != nil {
		t.Fatalf("signingProjectPBXInheritedEntitlementValues() error = %v", err)
	}
	want := map[string]bool{"WidgetBase": true, "WidgetBaseDevice": true}
	got := make(map[string]bool, len(values))
	for _, value := range values {
		got[value] = true
	}
	if len(got) != len(want) {
		t.Fatalf("signingProjectPBXInheritedEntitlementValues() = %#v, want %#v", values, want)
	}
	for value := range want {
		if !got[value] {
			t.Errorf("signingProjectPBXInheritedEntitlementValues() = %#v, missing %q", values, value)
		}
	}
}

func TestSigningPlanComposesTargetXCConfigAppendThroughProjectSeed(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS += Suffix\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Base"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Device";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "unconditional project seed", path: filepath.Join(projectRoot, "WidgetBase Suffix")},
		{name: "conditional project seed through unconditional", path: filepath.Join(projectRoot, "WidgetBaseDevice Suffix")},
	} {
		t.Run(test.name, func(t *testing.T) {
			const existingEntitlements = "existing target xcconfig append entitlement bytes\n"
			if err := os.WriteFile(test.path, []byte(existingEntitlements), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", test.path, err)
			}
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: test.path, StateDir: filepath.Join(root, "state"),
			})
			if err == nil || !strings.Contains(err.Error(), "aliases project input") {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want target xcconfig append alias rejection", plan, err)
			}
			if got := mustReadVersionTestFile(t, test.path); got != existingEntitlements {
				t.Fatalf("target xcconfig append entitlement was overwritten during planning: %q", got)
			}
		})
	}
}

func TestSigningPlanComposesConditionalTargetPBXInheritedEntitlementThroughUnconditional(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)Base"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Device";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "WidgetBaseDevice")
	const existingArtifact = "existing conditional target PBX entitlement bytes\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(WidgetBaseDevice) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: artifactPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want conditional target PBX inherited entitlement alias rejection", plan, err)
	}
	if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
		t.Fatalf("conditional target PBX inherited entitlement was overwritten during planning: %q", got)
	}
}

func TestSigningConfigurationInheritedEntitlementValuesComposeConditionalThroughUnconditional(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget; CODE_SIGN_ENTITLEMENTS = "$(inherited)Base"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Device";`)
	project, err := openSigningStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openSigningStructuredVersionProject() error = %v", err)
	}
	_, configFiles, _, _, _, _, lexicalConfigPaths, _, _, err := project.signingXCConfigConsumersWithOptionalMissing(nil, false)
	if err != nil {
		t.Fatalf("signingXCConfigConsumersWithOptionalMissing() error = %v", err)
	}
	resolver := newSigningSettingResolver(project, configFiles, false, lexicalConfigPaths)
	var targetConfiguration *versionConfiguration
	for _, configuration := range project.configurations {
		if !configuration.projectLevel && configuration.target == "Widget" && configuration.name == "Debug" {
			targetConfiguration = configuration
			break
		}
	}
	if targetConfiguration == nil {
		t.Fatal("project fixture is missing Widget Debug configuration")
	}
	values, inherited, err := signingConfigurationInheritedEntitlementValues(targetConfiguration, configFiles, resolver, "Widget")
	if err != nil {
		t.Fatalf("signingConfigurationInheritedEntitlementValues() error = %v", err)
	}
	if !inherited {
		t.Fatal("signingConfigurationInheritedEntitlementValues() inherited = false, want true")
	}
	want := map[string]bool{"WidgetBase": true, "WidgetBaseDevice": true}
	got := make(map[string]bool, len(values))
	for _, value := range values {
		got[value] = true
	}
	if len(got) != len(want) {
		t.Fatalf("signingConfigurationInheritedEntitlementValues() = %#v, want %#v", values, want)
	}
	for value := range want {
		if !got[value] {
			t.Errorf("signingConfigurationInheritedEntitlementValues() = %#v, missing %q", values, value)
		}
	}
}

func TestSigningPlanIgnoresShadowedProjectXCConfigExpression(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = App.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(PRODUCT_NAME)\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = App.entitlements\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "Widget")
	const existingArtifact = "existing artifact bytes for shadowed expression\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget artifact) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt path",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, artifactPath)
			plan, err := BuildSigningPlan(opts)
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v, want shadowed project xcconfig expression ignored", err)
			}
			if plan == nil || !plan.Ready {
				t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
			}
			if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
				t.Fatalf("valid artifact path changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningConfigurationXCConfigInheritedEntitlementIsNotUnconditional(t *testing.T) {
	projectPath := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(projectPath, "project.pbxproj")
	attachSigningProjectXCConfig(t, projectPath, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	attachSigningWidgetXCConfig(t, projectPath, "CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	project, err := openSigningStructuredVersionProject(projectPath)
	if err != nil {
		t.Fatalf("openSigningStructuredVersionProject() error = %v", err)
	}
	_, configFiles, _, _, _, _, lexicalConfigPaths, _, _, err := project.signingXCConfigConsumersWithOptionalMissing(nil, false)
	if err != nil {
		t.Fatalf("signingXCConfigConsumersWithOptionalMissing() error = %v", err)
	}
	resolver := newSigningSettingResolver(project, configFiles, false, lexicalConfigPaths)
	var widgetConfiguration *versionConfiguration
	for _, configuration := range project.configurations {
		if configuration.target == "Widget" && configuration.name == "Debug" {
			widgetConfiguration = configuration
			break
		}
	}
	if widgetConfiguration == nil {
		t.Fatal("project fixture has no Widget Debug configuration")
	}
	if signingConfigurationDefinesUnconditionalEntitlement(project, widgetConfiguration, configFiles, resolver) {
		t.Fatal("target xcconfig CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix was classified as an unconditional override")
	}
}

func TestSigningPlanTreatsRepeatedTargetXCConfigInheritedAfterLiteralAsLocal(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(PRODUCT_NAME)\n")
	attachSigningWidgetXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = Fixed.entitlements\n"+
			"CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	artifactPath := filepath.Join(projectRoot, "Widget")
	const existingArtifact = "existing artifact bytes for local target xcconfig\n"
	if err := os.WriteFile(artifactPath, []byte(existingArtifact), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: artifactPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want local repeated target xcconfig assignment to shadow project expression", err)
	}
	if plan == nil || !plan.Ready {
		t.Fatalf("BuildSigningPlan() plan = %#v, want ready plan", plan)
	}
	if got := mustReadVersionTestFile(t, artifactPath); got != existingArtifact {
		t.Fatalf("artifact changed while validating local repeated target xcconfig assignment: %q", got)
	}
}

func TestSigningPlanProtectsReferencedDivergentInheritedProjectEntitlementSeeds(t *testing.T) {
	project := writeReferencedDivergentInheritedProjectEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`, "")
	protectedPath := filepath.Join(filepath.Dir(project), "WidgetSuffix")
	const existingEntitlements = "existing referenced entitlement seed bytes\n"
	if err := os.WriteFile(protectedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(referenced entitlement) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan aliases referenced project seed",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt aliases referenced project seed",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, protectedPath)
			plan, err := BuildSigningPlan(opts)
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want referenced project seed alias rejection", plan, err)
			}
			if got := mustReadVersionTestFile(t, protectedPath); got != existingEntitlements {
				t.Fatalf("referenced entitlement seed changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanProtectsProjectSeedBelowConditionalTargetXCConfig(t *testing.T) {
	project := writeDivergentInheritedProjectEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`,
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = Target\n")
	protectedPath := filepath.Join(filepath.Dir(project), "BaseSuffix")
	const existingEntitlements = "existing conditional target xcconfig fallback bytes\n"
	if err := os.WriteFile(protectedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(conditional target xcconfig fallback) error = %v", err)
	}

	tests := []struct {
		name      string
		configure func(*SigningPlanOptions, string)
	}{
		{
			name: "plan aliases project fallback",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.PlanPath = path
			},
		},
		{
			name: "receipt aliases project fallback",
			configure: func(opts *SigningPlanOptions, path string) {
				opts.ReceiptPath = path
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			opts := SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				StateDir: filepath.Join(root, "state"),
			}
			test.configure(&opts, protectedPath)
			plan, err := BuildSigningPlan(opts)
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want project fallback alias rejection", plan, err)
			}
			if got := mustReadVersionTestFile(t, protectedPath); got != existingEntitlements {
				t.Fatalf("project fallback entitlement changed while validating %s: %q", test.name, got)
			}
		})
	}
}

func TestSigningPlanDoesNotCrossComposeDivergentProjectEntitlementSeeds(t *testing.T) {
	tests := []struct {
		name           string
		widgetSettings string
		widgetXCConfig string
		actualSeed     string
	}{
		{
			name:           "target xcconfig override",
			widgetSettings: `CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`,
			widgetXCConfig: "CODE_SIGN_ENTITLEMENTS = Target\n",
			actualSeed:     "TargetSuffix",
		},
		{
			name:           "target conditional inherits local slot",
			widgetSettings: `CODE_SIGN_ENTITLEMENTS = Local; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Suffix";`,
			actualSeed:     "LocalSuffix",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeDivergentInheritedProjectEntitlementFixture(t, test.widgetSettings, test.widgetXCConfig)
			projectRoot := filepath.Dir(project)
			actualPath := filepath.Join(projectRoot, test.actualSeed)
			const existingActual = "existing live entitlement seed bytes\n"
			if err := os.WriteFile(actualPath, []byte(existingActual), 0o600); err != nil {
				t.Fatalf("WriteFile(%s) error = %v", test.actualSeed, err)
			}

			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			phantomPath := filepath.Join(projectRoot, "BaseSuffix")
			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: phantomPath, StateDir: filepath.Join(root, "state"),
			})
			if err != nil {
				t.Fatalf("BuildSigningPlan() for phantom project seed = plan=%#v, error=%v; want no alias rejection", plan, err)
			}
			if plan == nil {
				t.Fatal("BuildSigningPlan() returned nil plan for phantom project seed")
			}
			if strings.Contains(strings.Join(plan.Blockers, "\n"), "aliases project input") || strings.Contains(strings.Join(plan.Blockers, "\n"), "protected project input") {
				t.Fatalf("phantom project seed became an artifact alias blocker: %#v", plan.Blockers)
			}

			actualRoot := t.TempDir()
			actualSettingsPath := filepath.Join(actualRoot, "settings.json")
			writeSigningSettingsTestFile(t, actualSettingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			actualPlan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: actualSettingsPath,
				PlanPath: actualPath, StateDir: filepath.Join(actualRoot, "state"),
			})
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan() for live project seed = plan=%#v, error=%v; want actual entitlement alias rejection", actualPlan, err)
			}
			if got := mustReadVersionTestFile(t, actualPath); got != existingActual {
				t.Fatalf("live entitlement seed changed during alias validation: %q", got)
			}
		})
	}
}

func writeDivergentInheritedProjectEntitlementFixture(t *testing.T, widgetSettings, widgetXCConfig string) string {
	return writeInheritedProjectEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = Other; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = Base;`,
		widgetSettings, widgetXCConfig)
}

func writeDivergentInheritedProjectXCConfigEntitlementFixture(t *testing.T, widgetSettings string) string {
	t.Helper()
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	attachSigningProjectXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS = Other\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = Base\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995", widgetSettings)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	return project
}

func writeReferencedDivergentInheritedProjectEntitlementFixture(t *testing.T, widgetSettings, widgetXCConfig string) string {
	project := writeInheritedProjectEntitlementFixture(t,
		`CODE_SIGN_ENTITLEMENTS = Other; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(PRODUCT_NAME)";`,
		widgetSettings, widgetXCConfig)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993", `PRODUCT_NAME = App;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995", `PRODUCT_NAME = Widget;`)
	return project
}

func writeInheritedProjectEntitlementFixture(t *testing.T, projectSettings, widgetSettings, widgetXCConfig string) string {
	t.Helper()
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991", projectSettings)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	if widgetXCConfig != "" {
		attachSigningWidgetXCConfig(t, project, widgetXCConfig)
	}
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995", widgetSettings)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	return project
}

// TestSigningPlanProtectsIdenticalConditionalInheritedEntitlementComposition
// pins Xcode's build-setting evaluation for a conditional assignment whose text
// matches the object's unconditional assignment. Xcode resolves $(inherited) to
// the value the setting has at the next level up for that same assignment slot:
// a conditional CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] inherits the target's own
// unconditional CODE_SIGN_ENTITLEMENTS, which only then inherits the xcconfig
// and project layers. Widget Debug therefore has two live entitlement paths:
// the unconditional "BaseSuffix" and the conditional "BaseSuffixSuffix". A text
// comparison between the two assignments cannot tell the conditional caller
// apart from the unconditional slot it must compose through, so identical
// expressions must not collapse into a single resolution. Planning each path in
// turn keeps both halves pinned: a resolver that gained the conditional path by
// dropping the unconditional slot it composes through would leave the target's
// own entitlements file writable, so the inventory must widen rather than move.
func TestSigningPlanProtectsIdenticalConditionalInheritedEntitlementComposition(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = Base;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Suffix";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	composedNames := []string{"BaseSuffix", "BaseSuffixSuffix"}
	existingByName := map[string]string{
		"BaseSuffix":       "existing unconditional composition bytes\n",
		"BaseSuffixSuffix": "existing identical conditional composition bytes\n",
	}
	for _, name := range composedNames {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte(existingByName[name]), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

	for _, name := range composedNames {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: filepath.Join(projectRoot, name), StateDir: filepath.Join(root, "state"),
			})
			if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
				t.Fatalf("BuildSigningPlan(%s) = plan=%#v, error=%v; want identical conditional composition rejected as an artifact alias", name, plan, err)
			}
			for _, guarded := range composedNames {
				if got := mustReadVersionTestFile(t, filepath.Join(projectRoot, guarded)); got != existingByName[guarded] {
					t.Fatalf("composed entitlement input %s was overwritten while planning %s: %q", guarded, name, got)
				}
			}
		})
	}
}

func TestSigningPlanProtectsInheritedTargetComposedWithProjectConditional(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = Base;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)Suffix";`)
	projectRoot := filepath.Dir(project)
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	composedPath := filepath.Join(projectRoot, "BaseSuffix")
	const existingEntitlements = "existing inherited composition bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want inherited target plus project candidate composition rejected", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanProtectsProjectConditionalInheritedEntitlementComposition(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(inherited)Suffix";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = Base;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	projectRoot := filepath.Dir(project)
	appEntitlements := filepath.Join(projectRoot, "App.entitlements")
	if err := os.WriteFile(appEntitlements, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	composedPath := filepath.Join(projectRoot, "BaseSuffix")
	const existingEntitlements = "existing project composed entitlement bytes\n"
	if err := os.WriteFile(composedPath, []byte(existingEntitlements), 0o600); err != nil {
		t.Fatalf("WriteFile(composed entitlements) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: composedPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() = plan=%#v, error=%v; want project inherited composition rejected as an artifact alias", plan, err)
	}
	if got := mustReadVersionTestFile(t, composedPath); got != existingEntitlements {
		t.Fatalf("project composed entitlement input was overwritten during planning: %q", got)
	}
}

func TestSigningPlanSkipsProjectEntitlementExpressionsShadowedByTargetXCConfig(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(MISSING).entitlements";`)
	projectRoot := filepath.Dir(project)
	appXCConfig := filepath.Join(projectRoot, "App.xcconfig")
	if err := os.WriteFile(appXCConfig, []byte("CODE_SIGN_ENTITLEMENTS = App.entitlements\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "App.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(App.entitlements) error = %v", err)
	}
	contents := mustReadVersionTestFile(t, pbxprojPath)
	fileReference := "\t\tCCCCCCCCCCCCCCCCCCCCCCCC /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = App.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatal("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+marker, 1)
	appConfiguration := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedAppConfiguration := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = CCCCCCCCCCCCCCCCCCCCCCCC;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(contents, appConfiguration) {
		t.Fatal("project fixture is missing App Debug configuration")
	}
	contents = strings.Replace(contents, appConfiguration, updatedAppConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = Widget.entitlements;`)
	if err := os.WriteFile(filepath.Join(projectRoot, "Widget.entitlements"), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(Widget.entitlements) error = %v", err)
	}

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
		t.Fatalf("BuildSigningPlan() error = %v, want target xcconfig to shadow the project entitlement expression", err)
	}
	if !plan.Ready {
		t.Fatalf("target xcconfig-shadowed project entitlement blocked planning: %#v", plan.Blockers)
	}
}

func TestSigningPlanSkipsShadowedProjectEntitlementExpressions(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(MISSING).entitlements";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`CODE_SIGN_ENTITLEMENTS = App.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = Widget.entitlements;`)
	projectRoot := filepath.Dir(project)
	for _, name := range []string{"App.entitlements", "Widget.entitlements"} {
		if err := os.WriteFile(filepath.Join(projectRoot, name), []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

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
		t.Fatalf("BuildSigningPlan() error = %v, want shadowed project entitlement expression ignored", err)
	}
	if !plan.Ready {
		t.Fatalf("shadowed project entitlement expression blocked planning: %#v", plan.Blockers)
	}
}

func TestSigningPlanResolvesProjectEntitlementsInTargetContext(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "$(PRODUCT_NAME).entitlements";`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999993",
		`PRODUCT_NAME = App;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`PRODUCT_NAME = Widget;`)
	for _, name := range []string{"App.entitlements", "Widget.entitlements"} {
		entitlementsPath := filepath.Join(filepath.Dir(project), name)
		if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", name, err)
		}
	}

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
		t.Fatalf("BuildSigningPlan() error = %v, want project entitlement resolved in the target context", err)
	}
	if !plan.Ready {
		t.Fatalf("project entitlement in target context blocked plan: %#v", plan.Blockers)
	}

	protected := filepath.Join(filepath.Dir(project), "App.entitlements")
	_, err = BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: protected, StateDir: filepath.Join(t.TempDir(), "state"),
	})
	if err == nil || (!strings.Contains(err.Error(), "aliases project input") && !strings.Contains(err.Error(), "protected project input")) {
		t.Fatalf("BuildSigningPlan() error = %v, want target-resolved entitlement path protected as an artifact alias", err)
	}
}

func TestSigningPlanRejectsUninventoriableConditionalPBXEntitlements(t *testing.T) {
	tests := []struct {
		name       string
		assignment string
	}{
		{
			name:       "unknown reference",
			assignment: `"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(MISSING_ENTITLEMENTS)";`,
		},
		{
			name: "reference cycle",
			assignment: `ENTITLEMENTS_FILE = "$(OTHER_ENTITLEMENTS)"; OTHER_ENTITLEMENTS = "$(ENTITLEMENTS_FILE)"; ` +
				`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(ENTITLEMENTS_FILE)";`,
		},
		{
			name:       "unsupported modifier",
			assignment: `ENTITLEMENTS_FILE = "App.entitlements"; "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = "$(ENTITLEMENTS_FILE:unsupported)";`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			injectSigningBuildSetting(t, filepath.Join(project, "project.pbxproj"), "999999999999999999999995", test.assignment)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			planPath := filepath.Join(root, "plan.json")
			const existingPlan = "existing plan bytes\n"
			if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
				t.Fatalf("WriteFile(existing plan) error = %v", err)
			}

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: planPath, StateDir: filepath.Join(root, "state"),
			})
			if err == nil || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried") {
				t.Fatalf("BuildSigningPlan() error = %v, want fail-closed unresolved entitlement error", err)
			}
			if plan != nil {
				t.Fatalf("plan = %#v, want no plan when entitlement aliases cannot be bounded", plan)
			}
			if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
				t.Fatalf("existing plan changed after unbounded conditional entitlement failure: %q", got)
			}
		})
	}
}

func TestSigningPlanRejectsUninventoriableConditionalXCConfigEntitlements(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name:   "unknown reference",
			config: "CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(MISSING_ENTITLEMENTS)\n",
		},
		{
			name: "reference cycle",
			config: "ENTITLEMENTS_FILE = $(OTHER_ENTITLEMENTS)\n" +
				"OTHER_ENTITLEMENTS = $(ENTITLEMENTS_FILE)\n" +
				"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(ENTITLEMENTS_FILE)\n",
		},
		{
			name:   "unsupported modifier",
			config: "ENTITLEMENTS_FILE = App.entitlements\nCODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(ENTITLEMENTS_FILE:unsupported)\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			attachSigningWidgetXCConfig(t, project, test.config)
			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
			}`)
			planPath := filepath.Join(root, "plan.json")
			const existingPlan = "existing plan bytes\n"
			if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
				t.Fatalf("WriteFile(existing plan) error = %v", err)
			}

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath,
				PlanPath: planPath, StateDir: filepath.Join(root, "state"),
			})
			if err == nil || !strings.Contains(err.Error(), "CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried") {
				t.Fatalf("BuildSigningPlan() error = %v, want fail-closed unresolved entitlement error", err)
			}
			if plan != nil {
				t.Fatalf("plan = %#v, want no plan when entitlement aliases cannot be bounded", plan)
			}
			if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
				t.Fatalf("existing plan changed after unbounded conditional entitlement failure: %q", got)
			}
		})
	}
}

func TestSigningPlanRejectsUnauthorizedConditionalEntitlementWithoutReadingIt(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	externalDir := t.TempDir()
	externalPath := filepath.Join(externalDir, "Widget.xcconfig")
	const canary = "UNAUTHORIZED_CONDITIONAL_ENTITLEMENT_CANARY"
	if err := os.WriteFile(externalPath, []byte(canary+"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(external xcconfig) error = %v", err)
	}
	previousReader := signingXCConfigReadFileFn
	signingXCConfigReadFileFn = func(path string, limit int64) ([]byte, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig was read: %s", path)
		}
		return previousReader(path, limit)
	}
	t.Cleanup(func() { signingXCConfigReadFileFn = previousReader })

	pbxprojPath := filepath.Join(project, "project.pbxproj")
	contents := mustReadVersionTestFile(t, pbxprojPath)
	const widgetReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
	fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = \"" + externalPath + "\"; sourceTree = \"<absolute>\"; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(contents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	contents = strings.Replace(contents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { \"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]\" = \"$(EXTERNAL_ENTITLEMENTS_FILE)\"; MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(contents, widgetConfiguration) {
		t.Fatalf("project fixture is missing Widget Debug configuration")
	}
	contents = strings.Replace(contents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	planPath := filepath.Join(root, "plan.json")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}

	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), signingUnauthorizedExternalXCConfigMessage) {
		t.Fatalf("BuildSigningPlan() error = %v, want generic unauthorized external failure", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("BuildSigningPlan() error exposed unauthorized xcconfig contents: %v", err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after unauthorized conditional entitlement failure: %q", got)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, "plan.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("project plan artifact after failure = %v, want absent", err)
	}
}

func TestSigningPlanRejectsHiddenConditionalExternalXCConfigWithoutFilesystemAccess(t *testing.T) {
	project, externalDir := externalXCConfigProject(t)
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	root := t.TempDir()
	planPath := filepath.Join(root, "plan.json")
	receiptPath := filepath.Join(root, "receipt.json")
	const existingPlan = "existing plan bytes\n"
	const existingReceipt = "existing receipt bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	if err := os.WriteFile(receiptPath, []byte(existingReceipt), 0o600); err != nil {
		t.Fatalf("WriteFile(existing receipt) error = %v", err)
	}
	if err := os.WriteFile(externalPath, []byte(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(HIDDEN_ENTITLEMENTS)\n"+"HIDDEN_ENTITLEMENTS = "+planPath+"\n"+"HIDDEN_EXTERNAL_CONDITIONAL_CANARY\n",
	), 0o600); err != nil {
		t.Fatalf("WriteFile(external xcconfig) error = %v", err)
	}

	previousReader := signingXCConfigReadFileFn
	previousStat := signingXCConfigStatFileFn
	previousProspective := signingResolveProspectivePathFn
	signingXCConfigReadFileFn = func(path string, limit int64) ([]byte, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig read: %s", path)
		}
		return previousReader(path, limit)
	}
	signingXCConfigStatFileFn = func(path string) (os.FileInfo, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig stat/open: %s", path)
		}
		return previousStat(path)
	}
	signingResolveProspectivePathFn = func(path string) (string, error) {
		if signingLexicalPathEqual(path, externalPath) {
			t.Fatalf("unauthorized external xcconfig prospective path was inspected: %s", path)
		}
		return previousProspective(path)
	}
	t.Cleanup(func() {
		signingXCConfigReadFileFn = previousReader
		signingXCConfigStatFileFn = previousStat
		signingResolveProspectivePathFn = previousProspective
	})

	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, ReceiptPath: receiptPath,
		StateDir: filepath.Join(root, "state"),
	})
	const wantUnauthorizedExternal = "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig"
	if err == nil || !strings.Contains(err.Error(), wantUnauthorizedExternal) {
		t.Fatalf("BuildSigningPlan() error = %v, want generic unauthorized external failure", err)
	}
	if strings.Contains(err.Error(), "conditional CODE_SIGN_ENTITLEMENTS") {
		t.Fatalf("BuildSigningPlan() classified unread external contents as observed conditional input: %v", err)
	}
	if strings.Contains(err.Error(), "HIDDEN_EXTERNAL_CONDITIONAL_CANARY") || strings.Contains(err.Error(), planPath) {
		t.Fatalf("BuildSigningPlan() exposed hidden external content/path: %v", err)
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed after hidden conditional external failure: %q", got)
	}
	if got := mustReadVersionTestFile(t, receiptPath); got != existingReceipt {
		t.Fatalf("existing receipt changed after hidden conditional external failure: %q", got)
	}
}

func attachSigningWidgetXCConfig(t *testing.T, project, contents string) string {
	t.Helper()
	projectRoot := filepath.Dir(project)
	configDir := filepath.Join(projectRoot, "Configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	configPath := filepath.Join(configDir, "Widget.xcconfig")
	if err := os.WriteFile(configPath, []byte(contents), 0o640); err != nil {
		t.Fatalf("WriteFile(widget xcconfig) error = %v", err)
	}

	pbxprojPath := filepath.Join(project, "project.pbxproj")
	projectContents := mustReadVersionTestFile(t, pbxprojPath)
	const widgetReference = "CCCCCCCCCCCCCCCCCCCCCCCC"
	fileReference := "\t\t" + widgetReference + " /* Widget.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Widget.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(projectContents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	projectContents = strings.Replace(projectContents, marker, fileReference+marker, 1)
	widgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedWidgetConfiguration := "999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + widgetReference + "; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(projectContents, widgetConfiguration) {
		t.Fatalf("project fixture is missing Widget Debug configuration")
	}
	projectContents = strings.Replace(projectContents, widgetConfiguration, updatedWidgetConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(projectContents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	return configPath
}

func attachSigningProjectXCConfig(t *testing.T, project, contents string) {
	t.Helper()
	projectRoot := filepath.Dir(project)
	configDir := filepath.Join(projectRoot, "Configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "Project.xcconfig"), []byte(contents), 0o640); err != nil {
		t.Fatalf("WriteFile(project xcconfig) error = %v", err)
	}

	pbxprojPath := filepath.Join(project, "project.pbxproj")
	projectContents := mustReadVersionTestFile(t, pbxprojPath)
	const projectReference = "BBBBBBBBBBBBBBBBBBBBBBBB"
	fileReference := "\t\t" + projectReference + " /* Project.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/Project.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(projectContents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	projectContents = strings.Replace(projectContents, marker, fileReference+marker, 1)
	projectConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; buildSettings = {}; name = Debug; };"
	updatedProjectConfiguration := "999999999999999999999991 /* Project Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + projectReference + "; buildSettings = {}; name = Debug; };"
	if !strings.Contains(projectContents, projectConfiguration) {
		t.Fatalf("project fixture is missing Project Debug configuration")
	}
	projectContents = strings.Replace(projectContents, projectConfiguration, updatedProjectConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(projectContents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
}

func attachSigningAppXCConfig(t *testing.T, project, contents string) {
	t.Helper()
	projectRoot := filepath.Dir(project)
	configDir := filepath.Join(projectRoot, "Configs")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	configPath := filepath.Join(configDir, "App.xcconfig")
	if err := os.WriteFile(configPath, []byte(contents), 0o640); err != nil {
		t.Fatalf("WriteFile(app xcconfig) error = %v", err)
	}

	pbxprojPath := filepath.Join(project, "project.pbxproj")
	projectContents := mustReadVersionTestFile(t, pbxprojPath)
	const appReference = "DDDDDDDDDDDDDDDDDDDDDDDD"
	fileReference := "\t\t" + appReference + " /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = Configs/App.xcconfig; sourceTree = SOURCE_ROOT; };\n"
	marker := "\t\t111111111111111111111111 /* Project object */ = {"
	if !strings.Contains(projectContents, marker) {
		t.Fatalf("project fixture is missing project object marker")
	}
	projectContents = strings.Replace(projectContents, marker, fileReference+marker, 1)
	appDebugConfiguration := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	updatedAppDebugConfiguration := "999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + appReference + ";  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };"
	if !strings.Contains(projectContents, appDebugConfiguration) {
		t.Fatalf("project fixture is missing App Debug configuration")
	}
	projectContents = strings.Replace(projectContents, appDebugConfiguration, updatedAppDebugConfiguration, 1)
	appReleaseConfiguration := "999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration;  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };"
	updatedAppReleaseConfiguration := "999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; baseConfigurationReference = " + appReference + ";  buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };"
	if !strings.Contains(projectContents, appReleaseConfiguration) {
		t.Fatalf("project fixture is missing App Release configuration")
	}
	projectContents = strings.Replace(projectContents, appReleaseConfiguration, updatedAppReleaseConfiguration, 1)
	if err := os.WriteFile(pbxprojPath, []byte(projectContents), 0o644); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
}

func TestSigningPlanProtectsResolvedEntitlementsReference(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningDirectBuildSetting(t, pbxprojPath,
		`ENTITLEMENTS_FILE = App.entitlements; CODE_SIGN_ENTITLEMENTS = "$(ENTITLEMENTS_FILE)";`)
	entitlementsPath := filepath.Join(filepath.Dir(project), "App.entitlements")
	if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlements) error = %v", err)
	}

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
		t.Fatalf("BuildSigningPlan() error = %v, want resolved entitlement reference accepted", err)
	}
	if !plan.Ready {
		t.Fatalf("resolved entitlement reference blocked plan: %#v", plan.Blockers)
	}
	structured, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("openStructuredVersionProject() error = %v", err)
	}
	paths, _, _, err := signingProjectInputPaths(structured, settingsPath, nil, nil, []signingRequest{{
		target: "App", configuration: "Debug",
		settings: []signingDesiredSetting{{key: "CODE_SIGN_STYLE", value: stringPtr("manual")}},
	}}, false, nil)
	if err != nil {
		t.Fatalf("signingProjectInputPaths() error = %v", err)
	}
	foundEntitlements := false
	for _, path := range paths {
		if path == entitlementsPath {
			foundEntitlements = true
		}
		if strings.Contains(path, "$(") || strings.Contains(path, "${") {
			t.Fatalf("input path retained unresolved entitlement expression: %q", path)
		}
	}
	if !foundEntitlements {
		t.Fatalf("input paths omitted resolved entitlement input %s: %#v", entitlementsPath, paths)
	}
}

func TestSigningPlanExpandsInheritedDirectEntitlementForProtectedInputs(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	sharedPath := filepath.Join(projectRoot, "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_ENTITLEMENTS = App.entitlements\n"+shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_ENTITLEMENTS = "$(inherited)";`)
	entitlementsPath := filepath.Join(projectRoot, "App.entitlements")
	if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlements) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: entitlementsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want inherited direct entitlement alias protection", err)
	}
}

func TestSigningPlanExpandsInheritedXCConfigEntitlementForProtectedInputs(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	appPath := filepath.Join(projectRoot, "Configs", "App.xcconfig")
	sharedPath := filepath.Join(projectRoot, "Configs", "Shared.xcconfig")
	app := mustReadVersionTestFile(t, appPath)
	app = strings.Replace(app, "#include \"Shared.xcconfig\"", "#include \"Shared.xcconfig\"\nCODE_SIGN_ENTITLEMENTS = $(inherited)", 1)
	if err := os.WriteFile(appPath, []byte(app), 0o640); err != nil {
		t.Fatalf("WriteFile(App xcconfig) error = %v", err)
	}
	shared := mustReadVersionTestFile(t, sharedPath)
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_ENTITLEMENTS = App.entitlements\n"+shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}
	entitlementsPath := filepath.Join(projectRoot, "App.entitlements")
	if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlements) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)
	_, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: entitlementsPath, StateDir: filepath.Join(root, "state"),
	})
	if err == nil || !strings.Contains(err.Error(), "aliases project input") {
		t.Fatalf("BuildSigningPlan() error = %v, want inherited xcconfig entitlement alias protection", err)
	}
}

func TestSigningPlanKeepsSuccessfulInheritedDirectEntitlementReady(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	// An explicitly empty lower-layer value makes the inherited suffix a
	// concrete target path without leaving the resolver uncertain.
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991", `CODE_SIGN_ENTITLEMENTS = "";`)
	injectSigningDirectBuildSetting(t, pbxprojPath, `CODE_SIGN_ENTITLEMENTS = "$(inherited)App.entitlements";`)
	entitlementsPath := filepath.Join(filepath.Dir(project), "App.entitlements")
	if err := os.WriteFile(entitlementsPath, []byte("<?xml version=\"1.0\"?><plist version=\"1.0\"><dict/></plist>\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(entitlements) error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want successful inherited expansion", err)
	}
	if !plan.Ready {
		t.Fatalf("successful inherited expansion blocked plan: %#v", plan.Blockers)
	}
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "external CODE_SIGN_ENTITLEMENTS input") {
			t.Fatalf("successful inherited expansion invented an external input: %#v", plan.Blockers)
		}
	}
}

func TestSigningPlanKeepsSuccessfulInheritedXCConfigEntitlementReady(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	widgetPath := attachSigningWidgetXCConfig(t, project, "#include \"Shared.xcconfig\"\nCODE_SIGN_ENTITLEMENTS = $(inherited)App.entitlements\n")
	sharedPath := filepath.Join(filepath.Dir(widgetPath), "Shared.xcconfig")
	if err := os.WriteFile(sharedPath, []byte("CODE_SIGN_ENTITLEMENTS = \"\"\n"), 0o640); err != nil {
		t.Fatalf("WriteFile(Shared.xcconfig) error = %v", err)
	}
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
		t.Fatalf("BuildSigningPlan() error = %v, want successful inherited expansion", err)
	}
	if !plan.Ready {
		t.Fatalf("successful inherited xcconfig expansion blocked plan: %#v", plan.Blockers)
	}
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "external CODE_SIGN_ENTITLEMENTS input") {
			t.Fatalf("successful inherited xcconfig expansion invented an external input: %#v", plan.Blockers)
		}
	}
}

func TestSigningPlanDoesNotInventDoubleComposedXCConfigEntitlement(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = Base;`)
	// Keep the unselected Widget configuration uncertain while giving it a
	// divergent PBX assignment. Its xcconfig value must inherit from the
	// project layer, not from this conditional PBX slot.
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix\n")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = PBX.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = Conditional.entitlements;`)

	projectRoot := filepath.Dir(project)
	planPath := filepath.Join(projectRoot, "BaseSuffixSuffix")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want non-alias plan path accepted", err)
	}
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "aliases project input") {
			t.Fatalf("double-composed xcconfig candidate blocked plan: %#v", plan.Blockers)
		}
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed during planning: %q", got)
	}
}

func TestSigningPlanPreservesResolvedEmptyXCConfigInheritance(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = "";`)
	attachSigningWidgetXCConfig(t, project, "CODE_SIGN_ENTITLEMENTS = $(inherited)Suffix\n")
	// The divergent conditional PBX value makes Widget's effective setting
	// uncertain, while the explicit empty project value still resolves its
	// xcconfig inheritance to the concrete path "Suffix".
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`CODE_SIGN_ENTITLEMENTS = PBX.entitlements;`)
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999995",
		`"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*]" = Conditional.entitlements;`)

	projectRoot := filepath.Dir(project)
	planPath := filepath.Join(projectRoot, "SuffixSuffix")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want non-alias plan path accepted", err)
	}
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "aliases project input") {
			t.Fatalf("resolved-empty xcconfig candidate blocked plan: %#v", plan.Blockers)
		}
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed during planning: %q", got)
	}
}

func TestSigningPlanPreservesSelectorSpecificEmptyXCConfigInheritance(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningBuildSetting(t, pbxprojPath, "999999999999999999999991",
		`CODE_SIGN_ENTITLEMENTS = Base;`)
	attachSigningWidgetXCConfig(t, project,
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = \"\"\n"+
			"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited)Suffix\n")

	projectRoot := filepath.Dir(project)
	planPath := filepath.Join(projectRoot, "BaseSuffix")
	const existingPlan = "existing plan bytes\n"
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(plan) error = %v", err)
	}
	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath,
		PlanPath: planPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v, want non-alias plan path accepted", err)
	}
	for _, blocker := range plan.Blockers {
		if strings.Contains(blocker, "aliases project input") {
			t.Fatalf("selector-specific empty xcconfig value blocked plan: %#v", plan.Blockers)
		}
	}
	if got := mustReadVersionTestFile(t, planPath); got != existingPlan {
		t.Fatalf("existing plan changed during planning: %q", got)
	}
}

func TestSigningInheritedXCConfigSourceBypassesDivergentPBXSlot(t *testing.T) {
	projectConfiguration := &versionConfiguration{projectLevel: true, name: "Debug", buildSettings: map[string]any{
		"CODE_SIGN_ENTITLEMENTS": "Base",
	}}
	configuration := &versionConfiguration{name: "Debug", target: "App", buildSettings: map[string]any{
		"CODE_SIGN_ENTITLEMENTS": "PBX.entitlements",
	}}
	project := &structuredVersionProject{configurations: []*versionConfiguration{projectConfiguration, configuration}}
	resolver := newSigningSettingResolver(project, nil, false, nil)
	stack := map[string]bool{"CODE_SIGN_ENTITLEMENTS": true}
	got, err := resolver.resolveInheritedSettingValue(configuration, configuration, "CODE_SIGN_ENTITLEMENTS", "CODE_SIGN_ENTITLEMENTS", "$(inherited)Suffix", stack, true)
	if err != nil {
		t.Fatalf("resolveInheritedSettingValue() error = %v", err)
	}
	if got != "Base" {
		t.Fatalf("xcconfig source inherited value = %q, want %q (PBX slot must be bypassed)", got, "Base")
	}
}

func TestSigningPathLexicallyContainedDoesNotTrustCaseFoldedWindowsRel(t *testing.T) {
	previousOS := runtimeGOOS
	previousCaseSemantics := signingCaseInsensitiveVolumeFn
	previousRel := signingPathRelFn
	runtimeGOOS = "windows"
	signingPathRelFn = func(string, string) (string, error) { return `child\\Config.xcconfig`, nil }
	signingCaseInsensitiveVolumeFn = func(string) (bool, bool) { return false, true }
	t.Cleanup(func() {
		runtimeGOOS = previousOS
		signingCaseInsensitiveVolumeFn = previousCaseSemantics
		signingPathRelFn = previousRel
	})
	root := filepath.Join(t.TempDir(), "Project")
	variantPath := filepath.Join(filepath.Dir(root), "project", "Config.xcconfig")
	project := &structuredVersionProject{rootDir: root}
	if signingPathLexicallyContained(project, variantPath) {
		t.Fatalf("case-variant Windows root %q was authorized from a case-folded Rel result", variantPath)
	}
}

// TestSigningPlanConvergesAfterApplyingPromotedDependentNoOp proves the staged
// resolution pass reaches a real fixed point: after the plan it produced is
// applied, replanning the same manifest against the mutated project yields a
// ready plan with no further changes. A pass that over-promoted a no-op, or
// that promoted against a staged view diverging from what apply publishes,
// would keep emitting target-level writes on every replan.
func TestSigningPlanConvergesAfterApplyingPromotedDependentNoOp(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "PRODUCT_BUNDLE_IDENTIFIER = com.example.old\r\nPROVISIONING_PROFILE_SPECIFIER = \"$(PRODUCT_BUNDLE_IDENTIFIER)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[
			{"name":"Debug","settings":{
				"PROVISIONING_PROFILE_SPECIFIER":"com.example.old",
				"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
			}},
			{"name":"Release","settings":{
				"PROVISIONING_PROFILE_SPECIFIER":"com.example.old",
				"PRODUCT_BUNDLE_IDENTIFIER":"com.example.new"
			}}
		]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	promoted := 0
	for _, change := range plan.Changes {
		if change.Setting == "PROVISIONING_PROFILE_SPECIFIER" {
			if change.Source != "pbxproj" {
				t.Fatalf("profile change = %#v, want promoted target-level write", change)
			}
			promoted++
		}
	}
	if promoted != 2 {
		t.Fatalf("promoted profile changes = %d, want one per requested configuration: %#v", promoted, plan.Changes)
	}
	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}

	replan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "replan"),
	})
	if err != nil {
		t.Fatalf("replan BuildSigningPlan() error = %v", err)
	}
	if !replan.Ready {
		t.Fatalf("replan blockers = %#v", replan.Blockers)
	}
	if len(replan.Changes) != 0 {
		t.Fatalf("replan changes = %#v, want a converged plan with no further writes", replan.Changes)
	}
}

// TestSigningPlanPromotesNoOpThroughStagedEscapedXCConfigValue binds the staged
// xcconfig overlay to the editor's quoting rules. The dependent no-op below can
// only be classified correctly if the staged bytes written by editXCConfig parse
// back to the exact requested identity; a lossy round trip would either hide the
// promotion or block the plan with a staged-value mismatch.
func TestSigningPlanPromotesNoOpThroughStagedEscapedXCConfigValue(t *testing.T) {
	const escapedIdentity = `iPhone Distribution: Acme "Corp" \ Ltd`
	project := writeStructuredVersionProject(t, true)
	sharedPath := filepath.Join(filepath.Dir(project), "Configs", "Shared.xcconfig")
	shared := mustReadVersionTestFile(t, sharedPath)
	shared = "CODE_SIGN_IDENTITY = Apple Development\r\nPROVISIONING_PROFILE_SPECIFIER = \"$(CODE_SIGN_IDENTITY)\"\r\n" + shared
	if err := os.WriteFile(sharedPath, []byte(shared), 0o640); err != nil {
		t.Fatalf("WriteFile(shared xcconfig) error = %v", err)
	}

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	manifest, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"targets": []any{map[string]any{
			"name": "App",
			"configurations": []any{
				map[string]any{"name": "Debug", "settings": map[string]any{
					"CODE_SIGN_IDENTITY":             escapedIdentity,
					"PROVISIONING_PROFILE_SPECIFIER": "Apple Development",
				}},
				map[string]any{"name": "Release", "settings": map[string]any{
					"CODE_SIGN_IDENTITY":             escapedIdentity,
					"PROVISIONING_PROFILE_SPECIFIER": "Apple Development",
				}},
			},
		}},
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	writeSigningSettingsTestFile(t, settingsPath, string(manifest))

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got %#v", plan.Blockers)
	}
	for _, change := range plan.Changes {
		switch change.Setting {
		case "CODE_SIGN_IDENTITY":
			if change.Source != "xcconfig" || change.NewValue == nil || *change.NewValue != escapedIdentity {
				t.Fatalf("identity change = %#v, want escaped xcconfig write", change)
			}
		case "PROVISIONING_PROFILE_SPECIFIER":
			if change.Source != "pbxproj" || change.NewValue == nil || *change.NewValue != "Apple Development" {
				t.Fatalf("profile change = %#v, want promoted target-level literal", change)
			}
		default:
			t.Fatalf("unexpected change = %#v", change)
		}
	}

	if err := WriteSigningPlanArtifact(plan, false); err != nil {
		t.Fatalf("WriteSigningPlanArtifact() error = %v", err)
	}
	if _, err := ApplySigningPlan(SigningApplyOptions{PlanPath: plan.PlanPath}); err != nil {
		t.Fatalf("ApplySigningPlan() error = %v", err)
	}
	updated, err := openStructuredVersionProject(project)
	if err != nil {
		t.Fatalf("reopen project: %v", err)
	}
	configuration, err := signingConfigurationFor(updated, "App", "Debug")
	if err != nil {
		t.Fatalf("find updated configuration: %v", err)
	}
	identity, _, err := updated.resolveSetting(configuration, "CODE_SIGN_IDENTITY")
	if err != nil {
		t.Fatalf("resolve applied identity: %v", err)
	}
	if identity != escapedIdentity {
		t.Fatalf("applied identity = %q, want %q", identity, escapedIdentity)
	}
	profile, _, err := updated.resolveSetting(configuration, "PROVISIONING_PROFILE_SPECIFIER")
	if err != nil {
		t.Fatalf("resolve applied profile: %v", err)
	}
	if profile != "Apple Development" {
		t.Fatalf("applied profile = %q, want the preserved no-op value", profile)
	}
}

// TestSigningPlanResolvesImplicitProjectLocationVariables pins the implicit
// build-setting context Xcode supplies for every project. These variables are
// derivable from the selected .xcodeproj without a build, so an entitlements
// path written as $(SRCROOT)/App.entitlements must resolve to the on-disk file
// instead of blocking an unrelated signing change.
func TestSigningPlanResolvesImplicitProjectLocationVariables(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		file       string
		want       func(projectPath string) string
	}{
		{
			name:       "SRCROOT",
			expression: "$(SRCROOT)/App.entitlements",
			file:       "App.entitlements",
			want:       func(projectPath string) string { return filepath.Dir(projectPath) + "/App.entitlements" },
		},
		{
			name:       "SOURCE_ROOT",
			expression: "${SOURCE_ROOT}/App.entitlements",
			file:       "App.entitlements",
			want:       func(projectPath string) string { return filepath.Dir(projectPath) + "/App.entitlements" },
		},
		{
			name:       "PROJECT_DIR subdirectory",
			expression: "$(PROJECT_DIR)/Sub/App.entitlements",
			file:       filepath.Join("Sub", "App.entitlements"),
			want:       func(projectPath string) string { return filepath.Dir(projectPath) + "/Sub/App.entitlements" },
		},
		{
			name:       "PROJECT_NAME",
			expression: "$(SRCROOT)/$(PROJECT_NAME).entitlements",
			file:       "Demo.entitlements",
			want:       func(projectPath string) string { return filepath.Dir(projectPath) + "/Demo.entitlements" },
		},
		{
			name:       "TARGET_NAME",
			expression: "$(SRCROOT)/$(TARGET_NAME).entitlements",
			file:       "App.entitlements",
			want:       func(projectPath string) string { return filepath.Dir(projectPath) + "/App.entitlements" },
		},
		{
			name:       "PROJECT_FILE_PATH",
			expression: "$(PROJECT_FILE_PATH)/../App.entitlements",
			file:       "App.entitlements",
			want:       func(projectPath string) string { return projectPath + "/../App.entitlements" },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			projectRoot := filepath.Dir(project)
			writeSigningTestEntitlements(t, projectRoot, test.file)
			writeSigningTestEntitlements(t, projectRoot, "Renamed.entitlements")
			injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
				`CODE_SIGN_ENTITLEMENTS = "`+test.expression+`";`)

			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
					"DEVELOPMENT_TEAM": "ABCDE12345",
					"CODE_SIGN_ENTITLEMENTS": "Renamed.entitlements"
				}}]}]
			}`)

			plan, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
			})
			if err != nil {
				t.Fatalf("BuildSigningPlan() error = %v", err)
			}
			if !plan.Ready {
				t.Fatalf("expected ready plan, got blockers %#v", plan.Blockers)
			}
			var change *SigningSettingChange
			for index := range plan.Changes {
				if plan.Changes[index].Setting == "CODE_SIGN_ENTITLEMENTS" {
					change = &plan.Changes[index]
				}
			}
			if change == nil {
				t.Fatalf("no CODE_SIGN_ENTITLEMENTS change in %#v", plan.Changes)
			}
			if change.OldValue == nil {
				t.Fatalf("CODE_SIGN_ENTITLEMENTS old value = nil, want the expanded path")
			}
			if want := test.want(project); *change.OldValue != want {
				t.Fatalf("CODE_SIGN_ENTITLEMENTS old value = %q, want %q", *change.OldValue, want)
			}
		})
	}
}

func TestSigningPlanResolvesImplicitXCConfigEntitlement(t *testing.T) {
	project := writeStructuredVersionProject(t, true)
	projectRoot := filepath.Dir(project)
	appXCConfig := filepath.Join(projectRoot, "Configs", "App.xcconfig")
	contents := mustReadVersionTestFile(t, appXCConfig) +
		"PROJECT_DIR[sdk=iphoneos*] ?= /fallback\n" +
		"CODE_SIGN_ENTITLEMENTS = $(PROJECT_DIR)/App.entitlements\n"
	if err := os.WriteFile(appXCConfig, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(App.xcconfig) error = %v", err)
	}
	writeSigningTestEntitlements(t, projectRoot, "App.entitlements")
	writeSigningTestEntitlements(t, projectRoot, "Renamed.entitlements")

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"DEVELOPMENT_TEAM": "ABCDE12345",
			"CODE_SIGN_ENTITLEMENTS": "Renamed.entitlements"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got blockers %#v", plan.Blockers)
	}
	for index := range plan.Changes {
		if plan.Changes[index].Setting != "CODE_SIGN_ENTITLEMENTS" {
			continue
		}
		if plan.Changes[index].OldValue == nil {
			t.Fatalf("CODE_SIGN_ENTITLEMENTS old value = nil, want the expanded path")
		}
		if want := filepath.Join(projectRoot, "App.entitlements"); *plan.Changes[index].OldValue != want {
			t.Fatalf("CODE_SIGN_ENTITLEMENTS old value = %q, want %q", *plan.Changes[index].OldValue, want)
		}
		return
	}
	t.Fatalf("no CODE_SIGN_ENTITLEMENTS change in %#v", plan.Changes)
}

// TestSigningPlanFailsClosedForImplicitVariableEscapingProjectRoot proves an
// implicit variable cannot be used to reach outside the selected project root.
func TestSigningPlanFailsClosedForImplicitVariableEscapingProjectRoot(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	outside := filepath.Join(filepath.Dir(projectRoot), "outside.entitlements")
	if err := os.WriteFile(outside, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(outside) error = %v", err)
	}
	injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
		`CODE_SIGN_ENTITLEMENTS = "$(SRCROOT)/../outside.entitlements";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"DEVELOPMENT_TEAM":"ABCDE12345"}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if plan.Ready {
		t.Fatalf("escaping entitlements path unexpectedly produced a ready plan: %#v", plan)
	}
	blockers := strings.Join(plan.Blockers, "\n")
	if !strings.Contains(blockers, "external CODE_SIGN_ENTITLEMENTS input") || !strings.Contains(blockers, filepath.Clean(outside)) {
		t.Fatalf("blockers = %#v, want an external entitlements blocker naming %s", plan.Blockers, outside)
	}
}

// TestSigningPlanFailsClosedForBuildContextVariables keeps every variable that
// only a build can supply on the existing unresolved-reference path. Guessing a
// configuration, platform, or products directory would silently change which
// file the plan claims to have inventoried.
func TestSigningPlanFailsClosedForBuildContextVariables(t *testing.T) {
	for _, name := range []string{"CONFIGURATION", "PLATFORM_NAME", "SDKROOT", "BUILT_PRODUCTS_DIR", "EFFECTIVE_PLATFORM_NAME"} {
		t.Run(name, func(t *testing.T) {
			project := writeStructuredVersionProject(t, false)
			injectSigningDirectBuildSetting(t, filepath.Join(project, "project.pbxproj"),
				`CODE_SIGN_ENTITLEMENTS = "$(SRCROOT)/$(`+name+`)/App.entitlements";`)

			root := t.TempDir()
			settingsPath := filepath.Join(root, "settings.json")
			writeSigningSettingsTestFile(t, settingsPath, `{
				"schemaVersion": 1,
				"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{"DEVELOPMENT_TEAM":"ABCDE12345"}}]}]
			}`)

			_, err := BuildSigningPlan(SigningPlanOptions{
				ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
			})
			if err == nil || !strings.Contains(err.Error(), "unresolved build-setting reference "+name) {
				t.Fatalf("BuildSigningPlan() error = %v, want unresolved reference for %s", err, name)
			}
		})
	}
}

// TestSigningPlanKeepsExplicitAssignmentOverImplicitVariable proves the
// implicit table is only a fallback: a project that assigns one of these names
// keeps Xcode's own precedence, where the explicit assignment wins.
func TestSigningPlanKeepsExplicitAssignmentOverImplicitVariable(t *testing.T) {
	project := writeStructuredVersionProject(t, false)
	projectRoot := filepath.Dir(project)
	writeSigningTestEntitlements(t, projectRoot, filepath.Join("Nested", "App.entitlements"))
	writeSigningTestEntitlements(t, projectRoot, "Renamed.entitlements")
	pbxprojPath := filepath.Join(project, "project.pbxproj")
	injectSigningDirectBuildSetting(t, pbxprojPath, `CODE_SIGN_ENTITLEMENTS = "$(PROJECT_DIR)/App.entitlements";`)
	injectSigningDirectBuildSetting(t, pbxprojPath, `PROJECT_DIR = "$(SRCROOT)/Nested";`)

	root := t.TempDir()
	settingsPath := filepath.Join(root, "settings.json")
	writeSigningSettingsTestFile(t, settingsPath, `{
		"schemaVersion": 1,
		"targets": [{"name":"App","configurations":[{"name":"Debug","settings":{
			"CODE_SIGN_ENTITLEMENTS": "Renamed.entitlements"
		}}]}]
	}`)

	plan, err := BuildSigningPlan(SigningPlanOptions{
		ProjectPath: project, SettingsFilePath: settingsPath, StateDir: filepath.Join(root, "state"),
	})
	if err != nil {
		t.Fatalf("BuildSigningPlan() error = %v", err)
	}
	if !plan.Ready {
		t.Fatalf("expected ready plan, got blockers %#v", plan.Blockers)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].OldValue == nil {
		t.Fatalf("unexpected changes %#v", plan.Changes)
	}
	if want := projectRoot + "/Nested/App.entitlements"; *plan.Changes[0].OldValue != want {
		t.Fatalf("CODE_SIGN_ENTITLEMENTS old value = %q, want the explicit assignment %q", *plan.Changes[0].OldValue, want)
	}
}

func writeSigningTestEntitlements(t *testing.T, projectRoot, relative string) {
	t.Helper()
	path := filepath.Join(projectRoot, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
