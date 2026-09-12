package xcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeVersionViewCommandOutputsResult(t *testing.T) {
	originalRunGetVersion := runGetVersion
	t.Cleanup(func() {
		runGetVersion = originalRunGetVersion
	})

	runGetVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (*localxcode.VersionInfo, error) {
		return &localxcode.VersionInfo{
			Version:     "1.2.3",
			BuildNumber: "42",
			ProjectDir:  opts.ProjectDir,
			Target:      opts.Target,
			Modern:      true,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{"view", "--project-dir", "./MyApp", "--target", "App", "--output", "json"})
	if err != nil {
		t.Fatalf("view run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected view stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from view command")
	}
}

func TestXcodeVersionViewCommandSupportsProjectFlag(t *testing.T) {
	originalRunGetVersion := runGetVersion
	t.Cleanup(func() {
		runGetVersion = originalRunGetVersion
	})

	runGetVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (*localxcode.VersionInfo, error) {
		if opts.ProjectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", opts.ProjectDir)
		}
		return &localxcode.VersionInfo{
			Version:     "1.2.3",
			BuildNumber: "42",
			ProjectDir:  "./MyApp",
			Target:      opts.Target,
			Modern:      true,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"view",
		"--project", "./MyApp/App.xcodeproj",
		"--target", "App",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("view run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected view stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from view command")
	}
}

func TestXcodeVersionViewCommandForwardsXcodebuildSettingsLookupPolicy(t *testing.T) {
	originalRunGetVersion := runGetVersion
	t.Cleanup(func() { runGetVersion = originalRunGetVersion })

	runGetVersion = func(_ context.Context, opts localxcode.GetVersionOptions) (*localxcode.VersionInfo, error) {
		if opts.BuildSettingsLookup != localxcode.BuildSettingsLookupNever {
			t.Fatalf("lookup policy = %q, want never", opts.BuildSettingsLookup)
		}
		if opts.BuildSettingsDiagnostic == nil {
			t.Fatal("expected stderr diagnostic writer")
		}
		return &localxcode.VersionInfo{Version: "1.2.3", BuildNumber: "42", ProjectDir: opts.ProjectDir}, nil
	}

	_, stderr, err := runXcodeVersionCommand(t, []string{
		"view", "--xcodebuild-settings-lookup", "never", "--output", "json",
	})
	if err != nil {
		t.Fatalf("view run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected no diagnostic without a fallback, got %q", stderr)
	}
}

func TestXcodeVersionCommandsRejectInvalidXcodebuildSettingsLookupBeforeWork(t *testing.T) {
	originalRunGetVersion := runGetVersion
	originalRunSetVersion := runSetVersion
	originalRunBumpVersion := runBumpVersion
	t.Cleanup(func() {
		runGetVersion = originalRunGetVersion
		runSetVersion = originalRunSetVersion
		runBumpVersion = originalRunBumpVersion
	})

	runGetVersion = func(context.Context, localxcode.GetVersionOptions) (*localxcode.VersionInfo, error) {
		t.Fatal("view work ran before --xcodebuild-settings-lookup validation")
		return nil, nil
	}
	runSetVersion = func(context.Context, localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		t.Fatal("edit work ran before --xcodebuild-settings-lookup validation")
		return nil, nil
	}
	runBumpVersion = func(context.Context, localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		t.Fatal("bump work ran before --xcodebuild-settings-lookup validation")
		return nil, nil
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "view", args: []string{"view", "--xcodebuild-settings-lookup", "sometimes"}},
		{name: "view empty", args: []string{"view", "--xcodebuild-settings-lookup="}},
		{name: "edit", args: []string{"edit", "--version", "1.2.3", "--xcodebuild-settings-lookup", "sometimes"}},
		{name: "bump", args: []string{"bump", "--type", "patch", "--xcodebuild-settings-lookup", "sometimes"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runXcodeVersionCommand(t, test.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("expected usage error, got %v", err)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, "--xcodebuild-settings-lookup must be one of: auto, never") {
				t.Fatalf("unexpected stderr: %q", stderr)
			}
		})
	}
}

func TestSelectedProjectInputPreservesExactPathBytes(t *testing.T) {
	tests := []struct {
		name       string
		projectDir string
		project    string
		want       string
	}{
		{
			name:       "explicit project with trailing whitespace",
			projectDir: "./ignored",
			project:    "./MyApp/Foo.xcodeproj ",
			want:       "./MyApp/Foo.xcodeproj ",
		},
		{
			name:       "project directory with surrounding whitespace",
			projectDir: " ./My App ",
			want:       " ./My App ",
		},
		{
			name:       "all-whitespace explicit project remains a path",
			projectDir: "./ignored",
			project:    " ",
			want:       " ",
		},
		{
			name: "empty values keep default",
			want: ".",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectedProjectInput(test.projectDir, test.project); got != test.want {
				t.Fatalf("selectedProjectInput(%q, %q) = %q, want %q", test.projectDir, test.project, got, test.want)
			}
		})
	}
}

func TestXcodeVersionEditCommandOutputsResult(t *testing.T) {
	originalRunSetVersion := runSetVersion
	t.Cleanup(func() {
		runSetVersion = originalRunSetVersion
	})

	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		return &localxcode.SetVersionResult{
			Version:     opts.Version,
			BuildNumber: opts.BuildNumber,
			ProjectDir:  opts.ProjectDir,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{"edit", "--project-dir", "./MyApp", "--version", "1.3.0", "--build-number", "42", "--output", "json"})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected edit stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from edit command")
	}
}

func TestXcodeVersionEditCommandSupportsProjectFlag(t *testing.T) {
	originalRunSetVersion := runSetVersion
	t.Cleanup(func() {
		runSetVersion = originalRunSetVersion
	})

	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		if opts.ProjectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", opts.ProjectDir)
		}
		return &localxcode.SetVersionResult{
			Version:     opts.Version,
			BuildNumber: opts.BuildNumber,
			ProjectDir:  "./MyApp",
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"edit",
		"--project", "./MyApp/App.xcodeproj",
		"--version", "1.3.0",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected edit stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from edit command")
	}
}

func TestXcodeVersionBumpCommandSupportsTargetFlag(t *testing.T) {
	originalRunBumpVersion := runBumpVersion
	t.Cleanup(func() {
		runBumpVersion = originalRunBumpVersion
	})

	runBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		if opts.ProjectDir != "./MyApp/App.xcodeproj" {
			t.Fatalf("expected explicit project path, got %q", opts.ProjectDir)
		}
		if opts.Target != "Extension" {
			t.Fatalf("expected bump target Extension, got %q", opts.Target)
		}
		return &localxcode.BumpVersionResult{
			BumpType:   string(opts.BumpType),
			OldVersion: "2.0.0",
			NewVersion: "2.0.1",
			ProjectDir: opts.ProjectDir,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"bump",
		"--project", "./MyApp/App.xcodeproj",
		"--target", "Extension",
		"--type", "patch",
		"--output", "json",
	})
	if err != nil {
		t.Fatalf("bump run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected bump stderr to be empty, got %q", stderr)
	}
	if stdout == "" {
		t.Fatal("expected JSON output from bump command")
	}
}

func TestXcodeVersionEditCommandExposesTargetFlag(t *testing.T) {
	if xcodeVersionEditCommand().FlagSet.Lookup("target") == nil {
		t.Fatal("expected edit command to expose --target")
	}
}

func TestXcodeVersionCommandsExposeProjectFlag(t *testing.T) {
	if xcodeVersionViewCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected view command to expose --project")
	}
	if xcodeVersionEditCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected edit command to expose --project")
	}
	if xcodeVersionBumpCommand().FlagSet.Lookup("project") == nil {
		t.Fatal("expected bump command to expose --project")
	}
}

func TestXcodeVersionBumpCommandExposesTargetFlag(t *testing.T) {
	if xcodeVersionBumpCommand().FlagSet.Lookup("target") == nil {
		t.Fatal("expected bump command to expose --target")
	}
}

func TestXcodeVersionCommandsExposeStructuredScopeAndRemoteFlags(t *testing.T) {
	for name, command := range map[string]*ffcli.Command{
		"view": xcodeVersionViewCommand(),
		"edit": xcodeVersionEditCommand(),
		"bump": xcodeVersionBumpCommand(),
	} {
		if command.FlagSet.Lookup("configuration") == nil {
			t.Fatalf("expected %s command to expose --configuration", name)
		}
	}

	if xcodeVersionEditCommand().FlagSet.Lookup("target") == nil {
		t.Fatal("expected edit command to expose --target")
	}
	for _, command := range []*ffcli.Command{xcodeVersionEditCommand(), xcodeVersionBumpCommand()} {
		for _, flagName := range []string{"next-build-number", "app", "platform", "processing-state", "exclude-expired", "initial-build-number"} {
			if command.FlagSet.Lookup(flagName) == nil {
				t.Fatalf("expected %s command to expose --%s", command.Name, flagName)
			}
		}
	}
}

func TestXcodeVersionEditCommandForwardsTargetAndConfiguration(t *testing.T) {
	originalRunSetVersion := runSetVersion
	t.Cleanup(func() { runSetVersion = originalRunSetVersion })

	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		if opts.Target != "App" || opts.Configuration != "Release" {
			t.Fatalf("unexpected edit scope: %#v", opts)
		}
		return &localxcode.SetVersionResult{
			Version:       opts.Version,
			ProjectDir:    opts.ProjectDir,
			Target:        opts.Target,
			Configuration: opts.Configuration,
		}, nil
	}

	_, stderr, err := runXcodeVersionCommand(t, []string{
		"edit", "--project", "./Demo.xcodeproj", "--target", "App",
		"--configuration", "Release", "--version", "2.0.0", "--output", "json",
	})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestXcodeVersionEditRejectsExplicitAndRemoteBuildNumbers(t *testing.T) {
	_, stderr, err := runXcodeVersionCommand(t, []string{
		"edit", "--version", "2.0.0", "--build-number", "42",
		"--next-build-number", "--app", "123456789",
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.HasPrefix(stderr, "Error: --build-number and --next-build-number are mutually exclusive\n") {
		t.Fatalf("unexpected diagnostic: %q", stderr)
	}
}

func TestXcodeVersionEditResolvesAndAppliesRemoteSafeBuildNumber(t *testing.T) {
	originalConsistent := runGetConsistentMarketingVersion
	originalValidate := runValidateSetVersion
	originalSet := runSetVersion
	originalResolve := runResolveXcodeNextBuildNumber
	t.Cleanup(func() {
		runGetConsistentMarketingVersion = originalConsistent
		runValidateSetVersion = originalValidate
		runSetVersion = originalSet
		runResolveXcodeNextBuildNumber = originalResolve
	})
	runValidateSetVersion = func(context.Context, localxcode.SetVersionOptions) error { return nil }

	runGetConsistentMarketingVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (string, error) {
		if opts.BuildSettingsLookup != localxcode.BuildSettingsLookupNever || opts.BuildSettingsDiagnostic == nil {
			t.Fatalf("unexpected lookup policy options: %#v", opts)
		}
		return "2.4.0", nil
	}
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		if opts.AppID != "com.example.demo" || opts.Version != "2.4.0" || opts.Platform != "IOS" {
			t.Fatalf("unexpected remote selection options: %#v", opts)
		}
		return "108", nil
	}
	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		if opts.BuildNumber != "108" || opts.Target != "App" || opts.Configuration != "Release" {
			t.Fatalf("unexpected remote-safe edit: %#v", opts)
		}
		return &localxcode.SetVersionResult{BuildNumber: opts.BuildNumber, Target: opts.Target, Configuration: opts.Configuration}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"edit", "--project", "Demo.xcodeproj", "--target", "App", "--configuration", "Release",
		"--next-build-number", "--app", "com.example.demo", "--platform", "IOS",
		"--xcodebuild-settings-lookup", "never", "--output", "json",
	})
	if err != nil {
		t.Fatalf("edit run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("unexpected output stdout=%q stderr=%q", stdout, stderr)
	}
	var result localxcode.SetVersionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode structured output: %v; output=%q", err, stdout)
	}
	if result.BuildNumber != "108" || result.Target != "App" || result.Configuration != "Release" {
		t.Fatalf("unexpected structured output: %#v", result)
	}
}

func TestXcodeVersionBumpCommandForwardsXcodebuildSettingsLookupPolicy(t *testing.T) {
	originalRunBumpVersion := runBumpVersion
	t.Cleanup(func() { runBumpVersion = originalRunBumpVersion })

	runBumpVersion = func(_ context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		if opts.BuildSettingsLookup != localxcode.BuildSettingsLookupNever {
			t.Fatalf("lookup policy = %q, want never", opts.BuildSettingsLookup)
		}
		if opts.BuildSettingsDiagnostic == nil {
			t.Fatal("expected stderr diagnostic writer")
		}
		return &localxcode.BumpVersionResult{BumpType: string(opts.BumpType), NewVersion: "1.2.4"}, nil
	}

	_, stderr, err := runXcodeVersionCommand(t, []string{
		"bump", "--type", "patch", "--xcodebuild-settings-lookup", "never", "--output", "json",
	})
	if err != nil {
		t.Fatalf("bump run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected no diagnostic without a fallback, got %q", stderr)
	}
}

func TestXcodeVersionEditRemoteNumberRejectsDivergentLocalVersions(t *testing.T) {
	originalConsistent := runGetConsistentMarketingVersion
	originalValidate := runValidateSetVersion
	originalResolve := runResolveXcodeNextBuildNumber
	originalSet := runSetVersion
	t.Cleanup(func() {
		runGetConsistentMarketingVersion = originalConsistent
		runValidateSetVersion = originalValidate
		runResolveXcodeNextBuildNumber = originalResolve
		runSetVersion = originalSet
	})
	runValidateSetVersion = func(context.Context, localxcode.SetVersionOptions) error { return nil }

	runGetConsistentMarketingVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (string, error) {
		return "", errors.New("MARKETING_VERSION has differing values")
	}
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		t.Fatal("remote lookup ran with a divergent local version scope")
		return "", nil
	}
	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		t.Fatal("mutation ran with a divergent local version scope")
		return nil, nil
	}

	_, _, err := runXcodeVersionCommand(t, []string{
		"edit", "--target", "App", "--next-build-number", "--app", "123456789",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected divergent version error, got %v", err)
	}
}

func TestXcodeVersionEditValidatesLocalMutationBeforeRemoteLookup(t *testing.T) {
	originalResolve := runResolveXcodeNextBuildNumber
	originalSet := runSetVersion
	t.Cleanup(func() {
		runResolveXcodeNextBuildNumber = originalResolve
		runSetVersion = originalSet
	})

	remoteCalled := false
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		remoteCalled = true
		return "108", nil
	}
	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		return nil, errors.New("--version must be a static value without build-setting references")
	}

	_, _, err := runXcodeVersionCommand(t, []string{
		"edit", "--project", "Missing.xcodeproj", "--version", "$(FOO)",
		"--next-build-number", "--app", "123456789",
	})
	if err == nil || !strings.Contains(err.Error(), "static value") {
		t.Fatalf("expected local validation error, got %v", err)
	}
	if remoteCalled {
		t.Fatal("remote lookup ran before the local mutation was validated")
	}
}

func TestXcodeVersionEditValidatesProjectBeforeRemoteLookup(t *testing.T) {
	originalResolve := runResolveXcodeNextBuildNumber
	originalSet := runSetVersion
	t.Cleanup(func() {
		runResolveXcodeNextBuildNumber = originalResolve
		runSetVersion = originalSet
	})

	remoteCalled := false
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		remoteCalled = true
		return "108", nil
	}
	runSetVersion = func(ctx context.Context, opts localxcode.SetVersionOptions) (*localxcode.SetVersionResult, error) {
		return nil, errors.New("no .xcodeproj found")
	}

	_, _, err := runXcodeVersionCommand(t, []string{
		"edit", "--project", filepath.Join(t.TempDir(), "Missing.xcodeproj"), "--version", "2.4.0",
		"--next-build-number", "--app", "123456789",
	})
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("expected local project error, got %v", err)
	}
	if remoteCalled {
		t.Fatal("remote lookup ran before the local project was validated")
	}
}

func TestXcodeVersionBumpRemoteNumberRequiresBuildType(t *testing.T) {
	_, stderr, err := runXcodeVersionCommand(t, []string{
		"bump", "--type", "patch", "--next-build-number", "--app", "123456789",
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.HasPrefix(stderr, "Error: --next-build-number requires --type build\n") {
		t.Fatalf("unexpected diagnostic: %q", stderr)
	}
}

func TestXcodeVersionBumpResolvesAndAppliesRemoteSafeBuildNumber(t *testing.T) {
	originalConsistent := runGetConsistentMarketingVersion
	originalValidate := runValidateBumpVersion
	originalBump := runBumpVersion
	originalResolve := runResolveXcodeNextBuildNumber
	t.Cleanup(func() {
		runGetConsistentMarketingVersion = originalConsistent
		runValidateBumpVersion = originalValidate
		runBumpVersion = originalBump
		runResolveXcodeNextBuildNumber = originalResolve
	})
	var lookupSession *localxcode.BuildSettingsLookupSession
	runValidateBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) error {
		if opts.BuildSettingsSession == nil {
			t.Fatal("validate phase did not receive a build settings lookup session")
		}
		lookupSession = opts.BuildSettingsSession
		return nil
	}

	runGetConsistentMarketingVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (string, error) {
		if opts.BuildSettingsSession != lookupSession {
			t.Fatal("marketing-version phase did not reuse the validation lookup session")
		}
		return "3.0.0", nil
	}
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		if opts.AppID != "123456789" || opts.Version != "3.0.0" || opts.InitialBuildNumber != 7 {
			t.Fatalf("unexpected remote options: %#v", opts)
		}
		return "301", nil
	}
	runBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		if opts.BuildSettingsSession != lookupSession {
			t.Fatal("mutation phase did not reuse the validation lookup session")
		}
		if opts.BumpType != localxcode.BumpBuild || opts.BuildNumber != "301" || opts.Target != "Widget" || opts.Configuration != "Debug" {
			t.Fatalf("unexpected bump options: %#v", opts)
		}
		return &localxcode.BumpVersionResult{
			BumpType: "build", NewBuild: opts.BuildNumber, Target: opts.Target, Configuration: opts.Configuration,
		}, nil
	}

	stdout, stderr, err := runXcodeVersionCommand(t, []string{
		"bump", "--project", "Demo.xcodeproj", "--target", "Widget", "--configuration", "Debug",
		"--type", "build", "--next-build-number", "--app", "123456789", "--initial-build-number", "7", "--output", "json",
	})
	if err != nil {
		t.Fatalf("bump run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
	var result localxcode.BumpVersionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode structured output: %v; output=%q", err, stdout)
	}
	if result.NewBuild != "301" || result.Target != "Widget" || result.Configuration != "Debug" {
		t.Fatalf("unexpected structured output: %#v", result)
	}
}

func TestXcodeVersionBumpRemoteNumberRejectsDivergentLocalVersions(t *testing.T) {
	originalConsistent := runGetConsistentMarketingVersion
	originalValidate := runValidateBumpVersion
	originalResolve := runResolveXcodeNextBuildNumber
	originalBump := runBumpVersion
	t.Cleanup(func() {
		runGetConsistentMarketingVersion = originalConsistent
		runValidateBumpVersion = originalValidate
		runResolveXcodeNextBuildNumber = originalResolve
		runBumpVersion = originalBump
	})
	runValidateBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) error { return nil }

	runGetConsistentMarketingVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (string, error) {
		return "", errors.New("MARKETING_VERSION has differing values")
	}
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		t.Fatal("remote lookup ran with a divergent local version scope")
		return "", nil
	}
	runBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		t.Fatal("mutation ran with a divergent local version scope")
		return nil, nil
	}

	_, _, err := runXcodeVersionCommand(t, []string{
		"bump", "--type", "build", "--target", "App", "--next-build-number", "--app", "123456789",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected divergent version error, got %v", err)
	}
}

func TestXcodeVersionBumpValidatesBuildBaselineBeforeRemoteLookup(t *testing.T) {
	originalValidate := runValidateBumpVersion
	originalConsistent := runGetConsistentMarketingVersion
	originalResolve := runResolveXcodeNextBuildNumber
	originalBump := runBumpVersion
	t.Cleanup(func() {
		runValidateBumpVersion = originalValidate
		runGetConsistentMarketingVersion = originalConsistent
		runResolveXcodeNextBuildNumber = originalResolve
		runBumpVersion = originalBump
	})

	runValidateBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) error {
		if opts.BumpType != localxcode.BumpBuild || opts.BuildNumber != "1" || opts.Target != "App" {
			t.Fatalf("unexpected bump preflight options: %#v", opts)
		}
		return errors.New("CURRENT_PROJECT_VERSION has differing values")
	}
	runGetConsistentMarketingVersion = func(ctx context.Context, opts localxcode.GetVersionOptions) (string, error) {
		t.Fatal("marketing-version lookup ran before build-baseline validation")
		return "", nil
	}
	runResolveXcodeNextBuildNumber = func(ctx context.Context, opts xcodeRemoteBuildNumberOptions) (string, error) {
		t.Fatal("remote lookup ran before build-baseline validation")
		return "", nil
	}
	runBumpVersion = func(ctx context.Context, opts localxcode.BumpVersionOptions) (*localxcode.BumpVersionResult, error) {
		t.Fatal("mutation ran after failed build-baseline validation")
		return nil, nil
	}

	_, _, err := runXcodeVersionCommand(t, []string{
		"bump", "--type", "build", "--target", "App", "--next-build-number", "--app", "123456789",
	})
	if err == nil || !strings.Contains(err.Error(), "differing values") {
		t.Fatalf("expected divergent build baseline error, got %v", err)
	}
}

func TestXcodeVersionEditRemoteNumberRequiresApp(t *testing.T) {
	t.Setenv("ASC_APP_ID", "")

	_, stderr, err := runXcodeVersionCommand(t, []string{
		"edit", "--version", "2.4.0", "--next-build-number",
	})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("expected usage error, got %v", err)
	}
	if !strings.HasPrefix(stderr, "Error: --app is required (or set ASC_APP_ID)\n") {
		t.Fatalf("unexpected diagnostic: %q", stderr)
	}
}

func runXcodeVersionCommand(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	cmd := XcodeVersionCommand()
	var runErr error
	stdout, stderr := captureXcodeVersionOutput(t, func() {
		if err := cmd.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		runErr = cmd.Run(context.Background())
	})
	return stdout, stderr, runErr
}

func captureXcodeVersionOutput(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe error: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe error: %v", err)
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	stdoutCh := make(chan string, 1)
	stderrCh := make(chan string, 1)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutR)
		_ = stdoutR.Close()
		stdoutCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrR)
		_ = stderrR.Close()
		stderrCh <- buf.String()
	}()

	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		_ = stdoutW.Close()
		_ = stderrW.Close()
	}()

	fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	return <-stdoutCh, <-stderrCh
}
