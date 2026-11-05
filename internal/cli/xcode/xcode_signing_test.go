package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeSigningApplyRequiresConfirm(t *testing.T) {
	command := xcodeSigningApplyCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--plan", "plan.json"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if !isUsageError(err) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func TestXcodeSigningPlanRejectsUninventoriableConditionalInputBeforeOverwrite(t *testing.T) {
	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	const existingPlan = "existing plan bytes\n"
	stateDir := t.TempDir()
	planPath := filepath.Join(stateDir, "plan.json")
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		return nil, errors.New("conditional CODE_SIGN_ENTITLEMENTS cannot be safely inventoried")
	}
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		t.Fatal("conditional entitlement failure reached plan publication")
		return nil
	}

	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{
		"--project", "App.xcodeproj", "--settings-file", "settings.json",
		"--state-dir", stateDir, "--overwrite", "--output", "json",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "conditional CODE_SIGN_ENTITLEMENTS cannot be safely inventoried") {
		t.Fatalf("Exec() error = %v, want conditional entitlement failure", err)
	}
	if got, err := os.ReadFile(planPath); err != nil {
		t.Fatalf("ReadFile(existing plan) error = %v", err)
	} else if string(got) != existingPlan {
		t.Fatalf("existing plan changed after conditional entitlement failure: %q", got)
	}
}

func TestXcodeSigningPlanRejectsHiddenConditionalExternalXCConfigBeforePublication(t *testing.T) {
	root := t.TempDir()
	externalDir := t.TempDir()
	planPath := filepath.Join(root, "state", "plan.json")
	receiptPath := filepath.Join(root, "state", "receipt.json")
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	const existingPlan = "existing plan bytes\n"
	const existingReceipt = "existing receipt bytes\n"
	const canary = "HIDDEN_EXTERNAL_CONDITIONAL_CANARY"
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(state) error = %v", err)
	}
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	if err := os.WriteFile(receiptPath, []byte(existingReceipt), 0o600); err != nil {
		t.Fatalf("WriteFile(existing receipt) error = %v", err)
	}
	if err := os.WriteFile(externalPath, []byte(fmt.Sprintf(
		"CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(HIDDEN_ENTITLEMENTS)\nHIDDEN_ENTITLEMENTS = %s\n%s\n",
		planPath, canary,
	)), 0o600); err != nil {
		t.Fatalf("WriteFile(external xcconfig) error = %v", err)
	}
	project := writeCLIHiddenExternalXCConfigProject(t, externalPath)
	settingsPath := filepath.Join(root, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	runBuildSigningPlan = localxcode.BuildSigningPlan
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		t.Fatal("hidden conditional entitlement failure reached plan publication")
		return nil
	}

	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{
		"--project", project, "--settings-file", settingsPath,
		"--state-dir", filepath.Dir(planPath), "--overwrite", "--output", "json",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig") {
		t.Fatalf("Exec() error = %v, want generic unauthorized external failure", err)
	}
	if strings.Contains(err.Error(), canary) || strings.Contains(err.Error(), planPath) {
		t.Fatalf("Exec() exposed hidden external content/path: %v", err)
	}
	if got, err := os.ReadFile(planPath); err != nil {
		t.Fatalf("ReadFile(existing plan) error = %v", err)
	} else if string(got) != existingPlan {
		t.Fatalf("existing plan changed after hidden conditional external failure: %q", got)
	}
	if got, err := os.ReadFile(receiptPath); err != nil {
		t.Fatalf("ReadFile(existing receipt) error = %v", err)
	} else if string(got) != existingReceipt {
		t.Fatalf("existing receipt changed after hidden conditional external failure: %q", got)
	}
}

func TestXcodeSigningPlanRejectsPlainUnauthorizedExternalXCConfig(t *testing.T) {
	root := t.TempDir()
	externalDir := t.TempDir()
	planPath := filepath.Join(root, "state", "plan.json")
	receiptPath := filepath.Join(root, "state", "receipt.json")
	externalPath := filepath.Join(externalDir, "App.xcconfig")
	const existingPlan = "existing plan bytes\n"
	const existingReceipt = "existing receipt bytes\n"
	if err := os.MkdirAll(filepath.Dir(planPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(state) error = %v", err)
	}
	if err := os.WriteFile(planPath, []byte(existingPlan), 0o600); err != nil {
		t.Fatalf("WriteFile(existing plan) error = %v", err)
	}
	if err := os.WriteFile(receiptPath, []byte(existingReceipt), 0o600); err != nil {
		t.Fatalf("WriteFile(existing receipt) error = %v", err)
	}
	if err := os.WriteFile(externalPath, []byte("CODE_SIGN_STYLE = Automatic\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(external xcconfig) error = %v", err)
	}
	project := writeCLIHiddenExternalXCConfigProject(t, externalPath)
	settingsPath := filepath.Join(root, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(`{
		"schemaVersion": 1,
		"targets": [{"name":"Widget","configurations":[{"name":"Debug","settings":{"CODE_SIGN_STYLE":"manual"}}]}]
	}`), 0o600); err != nil {
		t.Fatalf("WriteFile(settings) error = %v", err)
	}

	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	runBuildSigningPlan = localxcode.BuildSigningPlan
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		t.Fatal("plain unauthorized external xcconfig reached plan publication")
		return nil
	}

	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{
		"--project", project, "--settings-file", settingsPath,
		"--state-dir", filepath.Dir(planPath), "--overwrite", "--output", "json",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	const wantError = "unauthorized external xcconfig cannot be safely inventoried without --allow-external-xcconfig"
	if err == nil || !strings.Contains(err.Error(), wantError) {
		t.Fatalf("Exec() error = %v, want generic unauthorized external xcconfig failure", err)
	}
	if shared.IsReportedUsageError(err) || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want runtime exit-1 classification", err)
	}
	if strings.Contains(err.Error(), externalPath) || strings.Contains(err.Error(), planPath) {
		t.Fatalf("Exec() exposed external or artifact path: %v", err)
	}
	if got, err := os.ReadFile(planPath); err != nil {
		t.Fatalf("ReadFile(existing plan) error = %v", err)
	} else if string(got) != existingPlan {
		t.Fatalf("existing plan changed after unauthorized external failure: %q", got)
	}
	if got, err := os.ReadFile(receiptPath); err != nil {
		t.Fatalf("ReadFile(existing receipt) error = %v", err)
	} else if string(got) != existingReceipt {
		t.Fatalf("existing receipt changed after unauthorized external failure: %q", got)
	}
}

func writeCLIHiddenExternalXCConfigProject(t *testing.T, externalPath string) string {
	t.Helper()
	root := t.TempDir()
	projectPath := filepath.Join(root, "Demo.xcodeproj")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(project) error = %v", err)
	}
	project := `// !$*UTF8*$!
{
	archiveVersion = 1;
	classes = {};
	objectVersion = 77;
	objects = {
		AAAAAAAAAAAAAAAAAAAAAAAA /* App.xcconfig */ = {isa = PBXFileReference; lastKnownFileType = text.xcconfig; path = "` + externalPath + `"; sourceTree = "<absolute>"; };
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
		999999999999999999999993 /* App Debug */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA; buildSettings = {}; name = Debug; };
		999999999999999999999994 /* App Release */ = {isa = XCBuildConfiguration; baseConfigurationReference = AAAAAAAAAAAAAAAAAAAAAAAA; buildSettings = {}; name = Release; };
		999999999999999999999995 /* Widget Debug */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Debug; };
		999999999999999999999996 /* Widget Release */ = {isa = XCBuildConfiguration; buildSettings = { MARKETING_VERSION = 1.2.3; CURRENT_PROJECT_VERSION = 42; }; name = Release; };
		222222222222222222222222 /* Project configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999991, 999999999999999999999992); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		555555555555555555555555 /* App configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999993, 999999999999999999999994); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
		777777777777777777777777 /* Widget configuration list */ = {isa = XCConfigurationList; buildConfigurations = (999999999999999999999995, 999999999999999999999996); defaultConfigurationIsVisible = 0; defaultConfigurationName = Release; };
	};
	rootObject = 111111111111111111111111 /* Project object */;
}
`
	if err := os.WriteFile(filepath.Join(projectPath, "project.pbxproj"), []byte(project), 0o600); err != nil {
		t.Fatalf("WriteFile(project.pbxproj) error = %v", err)
	}
	return projectPath
}

func TestXcodeSigningPlanValidatesOutputBeforeBuildOrWrite(t *testing.T) {
	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	calledBuild := false
	calledWrite := false
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		calledBuild = true
		return &localxcode.SigningPlan{Ready: true, PlanPath: "plan.json"}, nil
	}
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		calledWrite = true
		return nil
	}
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{
		"--project", "App.xcodeproj", "--settings-file", "settings.json", "--output", "table", "--pretty",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if !isUsageError(err) {
		t.Fatalf("expected output usage error, got %v", err)
	}
	if calledBuild || calledWrite {
		t.Fatalf("invalid output reached side effects: build=%t write=%t", calledBuild, calledWrite)
	}
}

func TestXcodeSigningApplyValidatesOutputBeforeApply(t *testing.T) {
	originalApply := runApplySigningPlan
	t.Cleanup(func() { runApplySigningPlan = originalApply })
	calledApply := false
	runApplySigningPlan = func(localxcode.SigningApplyOptions) (*localxcode.SigningApplyResult, error) {
		calledApply = true
		return &localxcode.SigningApplyResult{PlanPath: "plan.json"}, nil
	}
	command := xcodeSigningApplyCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--plan", "plan.json", "--confirm", "--output", "invalid"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if !isUsageError(err) {
		t.Fatalf("expected output usage error, got %v", err)
	}
	if calledApply {
		t.Fatal("invalid output reached apply side effect")
	}
}

func TestXcodeSigningPlanMapsManifestValidationToReportedUsage(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "malformed JSON", message: "decode settings file: unexpected end of JSON input"},
		{name: "wrong schema", message: "settings file schemaVersion must be 1"},
		{name: "unsupported setting", message: "unsupported signing setting OTHER"},
		{name: "invalid team ID", message: "DEVELOPMENT_TEAM must be a 10-character alphanumeric team ID"},
		{name: "invalid entitlements path", message: "CODE_SIGN_ENTITLEMENTS: path must stay within the project"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalBuild := runBuildSigningPlan
			originalWrite := writeSigningPlanArtifact
			t.Cleanup(func() {
				runBuildSigningPlan = originalBuild
				writeSigningPlanArtifact = originalWrite
			})
			inputErr := localxcode.NewSigningInputError(errors.New(test.message))
			calledWrite := false
			runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
				return nil, inputErr
			}
			writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
				calledWrite = true
				return nil
			}
			command := xcodeSigningPlanCommand()
			command.FlagSet.SetOutput(io.Discard)
			if err := command.FlagSet.Parse([]string{
				"--project", "App.xcodeproj", "--settings-file", "settings.json", "--output", "json",
			}); err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			err := command.Exec(context.Background(), nil)
			if !shared.IsReportedUsageError(err) || errors.Is(err, flag.ErrHelp) {
				t.Fatalf("command error = %v, want reported usage without help wrapping", err)
			}
			if calledWrite {
				t.Fatal("manifest validation reached plan write")
			}
		})
	}
}

func TestXcodeSigningPlanLeavesParseAndFilesystemFailuresUnclassified(t *testing.T) {
	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		return nil, errors.New("parse project: permission denied")
	}
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		t.Fatal("filesystem failure reached plan write")
		return nil
	}
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{
		"--project", "App.xcodeproj", "--settings-file", "settings.json", "--output", "json",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || shared.IsReportedUsageError(err) || errors.Is(err, flag.ErrHelp) {
		t.Fatalf("command error = %v, want unclassified runtime error", err)
	}
}

func isUsageError(err error) bool {
	return err != nil && (errors.Is(err, flag.ErrHelp) || shared.IsReportedUsageError(err) || strings.Contains(err.Error(), "required"))
}

func TestXcodeSigningPlanRejectsEmptyStateDir(t *testing.T) {
	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	calledBuild := false
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		calledBuild = true
		return &localxcode.SigningPlan{Ready: true, PlanPath: "plan.json"}, nil
	}
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error { return nil }

	for _, stateDir := range []string{"", "   "} {
		command := xcodeSigningPlanCommand()
		command.FlagSet.SetOutput(io.Discard)
		if err := command.FlagSet.Parse([]string{
			"--project", "App.xcodeproj",
			"--settings-file", "settings.json",
			"--state-dir", stateDir,
		}); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		err := command.Exec(context.Background(), nil)
		if !isUsageError(err) {
			t.Fatalf("--state-dir=%q: expected usage error, got %v", stateDir, err)
		}
		if calledBuild {
			t.Fatalf("--state-dir=%q silently fell back to the default directory", stateDir)
		}
	}
}

func TestXcodeSigningPlanRejectsUnknownExportMethod(t *testing.T) {
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--export-method", "side-load", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if !isUsageError(err) {
		t.Fatalf("expected usage error, got %v", err)
	}
}

func stubXcodeSigningPlanSideEffects(t *testing.T) (*bool, *bool) {
	t.Helper()
	originalBuild := runBuildSigningPlan
	originalWrite := writeSigningPlanArtifact
	t.Cleanup(func() {
		runBuildSigningPlan = originalBuild
		writeSigningPlanArtifact = originalWrite
	})
	calledBuild := false
	calledWrite := false
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		calledBuild = true
		return &localxcode.SigningPlan{
			Ready:         true,
			PlanPath:      "plan.json",
			ExportOptions: &localxcode.SigningPlanExportOptions{Method: "app-store", SigningStyle: "manual"},
		}, nil
	}
	writeSigningPlanArtifact = func(*localxcode.SigningPlan, bool) error {
		calledWrite = true
		return nil
	}
	return &calledBuild, &calledWrite
}

func TestXcodeSigningPlanRejectsInferenceFlagsWithoutProfile(t *testing.T) {
	for _, extra := range [][]string{
		{"--configuration", "Release"},
		{"--export-method", "app-store"},
		{"--export-options-out", "ExportOptions.plist"},
		{"--skip-target", "Widget"},
	} {
		calledBuild, calledWrite := stubXcodeSigningPlanSideEffects(t)
		command := xcodeSigningPlanCommand()
		command.FlagSet.SetOutput(io.Discard)
		args := append([]string{"--project", "App.xcodeproj", "--settings-file", "settings.json"}, extra...)
		if err := command.FlagSet.Parse(args); err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		var err error
		_, stderr := captureCommandOutput(t, func() error { err = command.Exec(context.Background(), nil); return err })
		if !isUsageError(err) {
			t.Fatalf("%v: expected usage error, got %v", extra, err)
		}
		if !strings.Contains(err.Error()+stderr, extra[0]+" requires --profile") {
			t.Fatalf("%v: error = %v stderr = %q", extra, err, stderr)
		}
		if *calledBuild || *calledWrite {
			t.Fatalf("%v: plan side effects ran before validation", extra)
		}
	}
}

func TestXcodeSigningPlanMissingInputNamesProfile(t *testing.T) {
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj"}); err != nil {
		t.Fatal(err)
	}
	var err error
	_, stderr := captureCommandOutput(t, func() error { err = command.Exec(context.Background(), nil); return err })
	if !isUsageError(err) || !strings.Contains(err.Error(), "--settings-file") {
		t.Fatalf("expected --settings-file usage error, got %v", err)
	}
	if !strings.Contains(stderr, "--settings-file or --profile is required") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestXcodeSigningPlanRejectsExistingExportOptionsBeforeWritingPlan(t *testing.T) {
	calledBuild, calledWrite := stubXcodeSigningPlanSideEffects(t)
	existing := filepath.Join(t.TempDir(), "ExportOptions.plist")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--export-options-out", existing, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	err := command.Exec(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want existing export options rejection", err)
	}
	if *calledBuild || *calledWrite {
		t.Fatalf("plan was built or written before the export options destination was validated")
	}
	data, readErr := os.ReadFile(existing)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("existing export options changed: %q %v", data, readErr)
	}
}

func TestXcodeSigningPlanOverwriteReplacesExportOptions(t *testing.T) {
	stubXcodeSigningPlanSideEffects(t)
	existing := filepath.Join(t.TempDir(), "ExportOptions.plist")
	if err := os.WriteFile(existing, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--export-options-out", existing, "--overwrite", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var execErr error
	captureCommandOutput(t, func() error { execErr = command.Exec(context.Background(), nil); return execErr })
	if execErr != nil {
		t.Fatalf("Exec() error = %v", execErr)
	}
	data, err := os.ReadFile(existing)
	if err != nil || !strings.Contains(string(data), "<string>app-store</string>") {
		t.Fatalf("export options were not replaced: %q %v", data, err)
	}
}

func TestXcodeSigningPlanWritesBlockedPlanWithoutExportOptions(t *testing.T) {
	_, calledWrite := stubXcodeSigningPlanSideEffects(t)
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		return &localxcode.SigningPlan{
			Ready:         false,
			PlanPath:      "plan.json",
			Blockers:      []string{"unmatched signing target Widget/Release"},
			ExportOptions: &localxcode.SigningPlanExportOptions{Method: "app-store", SigningStyle: "manual"},
		}, nil
	}
	destination := filepath.Join(t.TempDir(), "ExportOptions.plist")
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--export-options-out", destination, "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var execErr error
	_, stderr := captureCommandOutput(t, func() error { execErr = command.Exec(context.Background(), nil); return execErr })
	if execErr != nil {
		t.Fatalf("Exec() error = %v", execErr)
	}
	if !*calledWrite {
		t.Fatal("blocked plan was not written")
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("export options were written for a blocked plan: %v", err)
	}
	if !strings.Contains(stderr, "export options were not written") {
		t.Fatalf("stderr = %q, want a not-written warning", stderr)
	}
}

func TestXcodeSigningPlanRejectsExportOptionsAliasingPlanInputs(t *testing.T) {
	root := t.TempDir()
	pbxproj := filepath.Join(root, "project.pbxproj")
	if err := os.WriteFile(pbxproj, []byte("project"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, destination := range []string{filepath.Join(root, "plan.json"), pbxproj} {
		_, calledWrite := stubXcodeSigningPlanSideEffects(t)
		runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
			return &localxcode.SigningPlan{
				Ready:         true,
				PlanPath:      filepath.Join(root, "plan.json"),
				ReceiptPath:   filepath.Join(root, "receipt.json"),
				Files:         []localxcode.SigningPlanFile{{Path: pbxproj, Source: "pbxproj"}},
				ExportOptions: &localxcode.SigningPlanExportOptions{Method: "app-store", SigningStyle: "manual"},
			}, nil
		}
		command := xcodeSigningPlanCommand()
		command.FlagSet.SetOutput(io.Discard)
		if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--export-options-out", destination, "--overwrite", "--output", "json"}); err != nil {
			t.Fatal(err)
		}
		err := command.Exec(context.Background(), nil)
		if err == nil || !strings.Contains(err.Error(), "must not be") {
			t.Fatalf("%s: error = %v, want an alias rejection", destination, err)
		}
		if *calledWrite {
			t.Fatalf("%s: plan artifact was written before the alias was rejected", destination)
		}
		if data, _ := os.ReadFile(pbxproj); string(data) != "project" {
			t.Fatalf("project file was overwritten: %q", data)
		}
	}
}

func TestXcodeSigningPlanSanitizesWarningsWithoutChangingJSON(t *testing.T) {
	stubXcodeSigningPlanSideEffects(t)
	warning := "discarded Profile\x1b[31mName"
	runBuildSigningPlan = func(localxcode.SigningPlanOptions) (*localxcode.SigningPlan, error) {
		return &localxcode.SigningPlan{Ready: true, PlanPath: "plan.json", Warnings: []string{warning}}, nil
	}
	command := xcodeSigningPlanCommand()
	command.FlagSet.SetOutput(io.Discard)
	if err := command.FlagSet.Parse([]string{"--project", "App.xcodeproj", "--profile", "App.mobileprovision", "--output", "json"}); err != nil {
		t.Fatal(err)
	}
	var execErr error
	stdout, stderr := captureCommandOutput(t, func() error { execErr = command.Exec(context.Background(), nil); return execErr })
	if execErr != nil {
		t.Fatal(execErr)
	}
	if strings.ContainsRune(stderr, '\x1b') || !strings.Contains(stderr, shared.SanitizeTerminal(warning)) {
		t.Fatalf("unsafe terminal warning: %q", stderr)
	}
	var output struct {
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatal(err)
	}
	if len(output.Warnings) != 1 || output.Warnings[0] != warning {
		t.Fatalf("JSON warning changed: %#v", output.Warnings)
	}
}
