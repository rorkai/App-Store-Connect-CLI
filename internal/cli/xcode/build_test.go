package xcode

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

func TestXcodeBuildPassesTypedAndRawOptionsAndPrintsJSON(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })

	var gotOpts localxcode.BuildOptions
	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		gotOpts = opts
		exitStatus := 0
		return &localxcode.BuildResult{
			ProjectPath:       opts.ProjectPath,
			Scheme:            opts.Scheme,
			Configuration:     opts.Configuration,
			Destination:       opts.Destination,
			DerivedDataPath:   opts.DerivedDataPath,
			ResultBundlePath:  opts.ResultBundlePath,
			BuildProductsPath: "/tmp/Derived Data/Build/Products",
			Clean:             opts.Clean,
			NoCodeSigning:     opts.NoCodeSigning,
			Success:           true,
			DurationMS:        1250,
			ExitStatus:        &exitStatus,
		}, nil
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{
		"--project", "Demo App.xcodeproj",
		"--scheme", "Demo App",
		"--configuration", "Debug",
		"--destination", "platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0",
		"--derived-data-path", "/tmp/Derived Data",
		"--result-bundle-path", "/tmp/Results/Demo.xcresult",
		"--clean",
		"--no-code-signing",
		"--xcodebuild-flag=-quiet",
		"--xcodebuild-flag=OTHER_SWIFT_FLAGS=-D ASC_BUILD",
		"--output", "json",
	}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}

	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	if gotOpts.ProjectPath != "Demo App.xcodeproj" || gotOpts.WorkspacePath != "" || gotOpts.Scheme != "Demo App" {
		t.Fatalf("unexpected selector options: %+v", gotOpts)
	}
	if !gotOpts.Clean || !gotOpts.NoCodeSigning {
		t.Fatalf("expected clean and no-code-signing: %+v", gotOpts)
	}
	if gotOpts.ResultBundlePath != "/tmp/Results/Demo.xcresult" {
		t.Fatalf("ResultBundlePath = %q, want typed result bundle path", gotOpts.ResultBundlePath)
	}
	wantRaw := []string{"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_BUILD"}
	if len(gotOpts.XcodebuildArgs) != len(wantRaw) || gotOpts.XcodebuildArgs[0] != wantRaw[0] || gotOpts.XcodebuildArgs[1] != wantRaw[1] {
		t.Fatalf("XcodebuildArgs = %#v, want %#v", gotOpts.XcodebuildArgs, wantRaw)
	}
	var payload localxcode.BuildResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if !payload.Success || payload.DurationMS != 1250 || !payload.NoCodeSigning || payload.ExitStatus == nil || *payload.ExitStatus != 0 {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
	if payload.ResultBundlePath != "/tmp/Results/Demo.xcresult" {
		t.Fatalf("unexpected result bundle path: %+v", payload)
	}
}

func TestXcodeBuildValidationErrorsAreUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing selector", args: []string{"--scheme", "Demo"}, want: "exactly one of --workspace or --project"},
		{name: "both selectors", args: []string{"--project", "Demo.xcodeproj", "--workspace", "Demo.xcworkspace", "--scheme", "Demo"}, want: "exactly one of --workspace or --project"},
		{name: "missing scheme", args: []string{"--project", "Demo.xcodeproj"}, want: "--scheme is required"},
		{name: "bad project suffix", args: []string{"--project", "Demo.txt", "--scheme", "Demo"}, want: "--project must end with .xcodeproj"},
		{name: "reserved raw flag", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--xcodebuild-flag=-derivedDataPath"}, want: "cannot override asc-managed argument"},
		{name: "reserved result bundle raw flag", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--xcodebuild-flag=-resultBundlePath=/tmp/elsewhere.xcresult"}, want: "cannot override asc-managed argument"},
		{name: "conditional signing override", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--no-code-signing", "--xcodebuild-flag=CODE_SIGNING_ALLOWED[sdk=iphoneos*]=YES"}, want: `cannot override asc-managed argument "CODE_SIGNING_ALLOWED"`},
		{name: "action build setting override", args: []string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--xcodebuild-flag=ACTION=archive"}, want: `cannot override asc-managed argument "ACTION"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := newXcodeTestDeps(nil)
			cmd := newXcodeBuildCommand(d)
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(test.args); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			var runErr error
			stdout, stderr := captureCommandOutput(t, d, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("Exec() error = %v, want usage error", runErr)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestXcodeBuildRejectsExplicitlyEmptyOptionalValues(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "configuration",
			args: []string{"--configuration", ""},
			want: "--configuration must not be empty",
		},
		{
			name: "destination",
			args: []string{"--destination", ""},
			want: "--destination must not be empty",
		},
		{
			name: "derived data path",
			args: []string{"--derived-data-path", "  "},
			want: "--derived-data-path must not be empty",
		},
		{
			name: "result bundle path",
			args: []string{"--result-bundle-path", ""},
			want: "--result-bundle-path must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			originalRunBuild := runBuild
			t.Cleanup(func() { runBuild = originalRunBuild })
			runBuild = func(context.Context, localxcode.BuildOptions) (*localxcode.BuildResult, error) {
				t.Fatal("runBuild must not be called for an explicitly empty value")
				return nil, nil
			}

			d := newXcodeTestDeps(nil)
			cmd := newXcodeBuildCommand(d)
			cmd.FlagSet.SetOutput(io.Discard)
			args := append([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo"}, test.args...)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}

			var runErr error
			stdout, stderr := captureCommandOutput(t, d, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if !errors.Is(runErr, flag.ErrHelp) {
				t.Fatalf("Exec() error = %v, want usage error", runErr)
			}
			if runErr.Error() != test.want {
				t.Fatalf("Exec() error = %q, want %q", runErr, test.want)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, test.want) {
				t.Fatalf("stderr = %q, want %q", stderr, test.want)
			}
		})
	}
}

func TestXcodeBuildAcceptsOmittedOptionalValues(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })

	called := false
	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		called = true
		if opts.Configuration != "" || opts.Destination != "" || opts.DerivedDataPath != "" || opts.ResultBundlePath != "" {
			t.Fatalf("omitted flags must stay empty, got %+v", opts)
		}
		exitStatus := 0
		return &localxcode.BuildResult{
			ProjectPath:     opts.ProjectPath,
			Scheme:          opts.Scheme,
			DerivedDataPath: "/tmp/derived",
			Success:         true,
			DurationMS:      1,
			ExitStatus:      &exitStatus,
		}, nil
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	_, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr != nil {
		t.Fatalf("Exec() error = %v", runErr)
	}
	if !called {
		t.Fatal("runBuild was not called for omitted optional flags")
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestXcodeBuildRejectsInvalidOutputBeforeStartingBuild(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })
	runBuild = func(context.Context, localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		t.Fatal("runBuild must not be called for invalid output")
		return nil, nil
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "yaml"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if !errors.Is(runErr, flag.ErrHelp) {
		t.Fatalf("Exec() error = %v, want usage error", runErr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, `(got "yaml")`) {
		t.Fatalf("stderr = %q, want unsupported format error", stderr)
	}
}

func TestXcodeBuildPrintsStructuredFailureBeforeReturningError(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })

	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		_, _ = io.WriteString(opts.LogWriter, "compile failed\n")
		exitStatus := 65
		return &localxcode.BuildResult{
			ProjectPath:     opts.ProjectPath,
			Scheme:          opts.Scheme,
			DerivedDataPath: "/tmp/derived",
			NoCodeSigning:   false,
			Success:         false,
			DurationMS:      400,
			ExitStatus:      &exitStatus,
		}, errors.New("xcodebuild build failed: compile failed")
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	if runErr == nil || !strings.Contains(runErr.Error(), "xcodebuild build failed: compile failed") {
		t.Fatalf("Exec() error = %v, want wrapped build failure", runErr)
	}
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("Exec() error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := strings.Count(stderr, "compile failed"); got != 1 {
		t.Fatalf("stderr = %q, compile diagnostic count = %d, want 1", stderr, got)
	}
	if !strings.Contains(stderr, "Error: xcode build failed with exit status 65") {
		t.Fatalf("stderr = %q, want concise final build error", stderr)
	}
	var payload localxcode.BuildResult
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if payload.Success || payload.ExitStatus == nil || *payload.ExitStatus != 65 {
		t.Fatalf("unexpected failure payload: %+v", payload)
	}
}

func TestXcodeBuildPrintsPreflightFailureReason(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })

	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		return &localxcode.BuildResult{
			ProjectPath:     opts.ProjectPath,
			Scheme:          opts.Scheme,
			DerivedDataPath: "/tmp/derived",
			Success:         false,
			DurationMS:      1,
		}, errors.New("xcodebuild not usable: xcodebuild version failed: exit status 72")
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("Exec() error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := strings.Count(stderr, "xcodebuild not usable"); got != 1 {
		t.Fatalf("stderr = %q, preflight reason count = %d, want 1", stderr, got)
	}
	if strings.Contains(stderr, "xcode build failed with exit status 72") {
		t.Fatalf("stderr = %q, version-probe status must not be reported as build status", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if _, exists := payload["exit_status"]; exists {
		t.Fatalf("unexpected exit_status for preflight failure: %s", stdout)
	}
}

func TestXcodeBuildPrintsSignaledFailureReason(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process termination does not expose Unix signal status")
	}
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })
	signalErr := exec.Command("sh", "-c", "kill -KILL $$").Run()
	if signalErr == nil {
		t.Fatal("signal helper returned nil error")
	}
	var signalExitErr *exec.ExitError
	if !errors.As(signalErr, &signalExitErr) || signalExitErr.ExitCode() != -1 {
		t.Fatalf("signal helper error = %T %v, want signaled *exec.ExitError", signalErr, signalErr)
	}

	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		_, _ = io.WriteString(opts.LogWriter, "compile started\n")
		return &localxcode.BuildResult{
			ProjectPath:     opts.ProjectPath,
			Scheme:          opts.Scheme,
			DerivedDataPath: "/tmp/derived",
			Success:         false,
			DurationMS:      2,
		}, fmt.Errorf("xcodebuild build failed: compile started: %w", signalErr)
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("Exec() error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := strings.Count(stderr, "signal: killed"); got != 1 {
		t.Fatalf("stderr = %q, signal reason count = %d, want 1", stderr, got)
	}
	if got := strings.Count(stderr, "compile started"); got != 1 {
		t.Fatalf("stderr = %q, streamed build log count = %d, want 1", stderr, got)
	}
	if strings.Contains(stderr, "exit status -1") {
		t.Fatalf("stderr = %q, signaled process must not report a numeric exit status", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if _, exists := payload["exit_status"]; exists {
		t.Fatalf("unexpected exit_status for signaled failure: %s", stdout)
	}
}

func TestXcodeBuildPrintsCancellationReasonWithoutReplayingLog(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })

	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		_, _ = io.WriteString(opts.LogWriter, "compile interrupted\n")
		return &localxcode.BuildResult{
			ProjectPath:     opts.ProjectPath,
			Scheme:          opts.Scheme,
			DerivedDataPath: "/tmp/derived",
			Success:         false,
			DurationMS:      2,
		}, fmt.Errorf("xcodebuild build timed out or was canceled: compile interrupted: %w", context.Canceled)
	}

	d := newXcodeTestDeps(nil)
	cmd := newXcodeBuildCommand(d)
	cmd.FlagSet.SetOutput(io.Discard)
	if err := cmd.FlagSet.Parse([]string{"--project", "Demo.xcodeproj", "--scheme", "Demo", "--output", "json"}); err != nil {
		t.Fatalf("FlagSet.Parse() error = %v", err)
	}
	var runErr error
	stdout, stderr := captureCommandOutput(t, d, func() error {
		runErr = cmd.Exec(context.Background(), nil)
		return runErr
	})
	var reportedErr shared.ReportedError
	if !errors.As(runErr, &reportedErr) {
		t.Fatalf("Exec() error = %T %v, want ReportedError", runErr, runErr)
	}
	if got := strings.Count(stderr, "context canceled"); got != 1 {
		t.Fatalf("stderr = %q, cancellation reason count = %d, want 1", stderr, got)
	}
	if got := strings.Count(stderr, "compile interrupted"); got != 1 {
		t.Fatalf("stderr = %q, streamed build log count = %d, want 1", stderr, got)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v\nstdout=%s", err, stdout)
	}
	if _, exists := payload["exit_status"]; exists {
		t.Fatalf("unexpected exit_status for cancellation: %s", stdout)
	}
}

func TestXcodeBuildRendersTableAndMarkdown(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })
	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		exitStatus := 0
		return &localxcode.BuildResult{
			WorkspacePath:   opts.WorkspacePath,
			Scheme:          opts.Scheme,
			Destination:     opts.Destination,
			DerivedDataPath: "/tmp/derived",
			NoCodeSigning:   false,
			Success:         true,
			DurationMS:      10,
			ExitStatus:      &exitStatus,
		}, nil
	}

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			d := newXcodeTestDeps(nil)
			cmd := newXcodeBuildCommand(d)
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse([]string{
				"--workspace", "Demo.xcworkspace", "--scheme", "Demo",
				"--destination", "generic/platform=iOS", "--output", format,
			}); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			var runErr error
			stdout, _ := captureCommandOutput(t, d, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})
			if runErr != nil {
				t.Fatalf("Exec() error = %v", runErr)
			}
			for _, want := range []string{"workspace", "Demo.xcworkspace", "destination", "generic/platform=iOS", "success", "exit_status", "0"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output = %q, want %q", format, stdout, want)
				}
			}
		})
	}
}

func TestXcodeBuildResolvesPathsInDepsWorkingDirectory(t *testing.T) {
	originalRunBuild := runBuild
	t.Cleanup(func() { runBuild = originalRunBuild })
	var gotWorkspace, gotProject string
	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		gotWorkspace, gotProject = opts.WorkspacePath, opts.ProjectPath
		return &localxcode.BuildResult{Success: true}, nil
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "App.xcworkspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name          string
		args          []string
		wantWorkspace string
		wantProject   string
	}{
		{name: "detected workspace", wantWorkspace: filepath.Join(dir, "App.xcworkspace")},
		{name: "explicit project", args: []string{"--project", "App.xcodeproj"}, wantProject: filepath.Join(dir, "App.xcodeproj")},
	} {
		t.Run(test.name, func(t *testing.T) {
			gotWorkspace, gotProject = "", ""
			d := newXcodeTestDeps(nil)
			d.Getwd = func() (string, error) { return dir, nil }
			cmd := newXcodeBuildCommand(d)
			if err := cmd.FlagSet.Parse(append([]string{"--scheme", "App", "--output", "json"}, test.args...)); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			if err := cmd.Exec(context.Background(), nil); err != nil {
				_, stderr := depsOutput(d)
				t.Fatalf("Exec() error = %v; stderr = %q", err, stderr)
			}
			if gotWorkspace != test.wantWorkspace || gotProject != test.wantProject {
				t.Fatalf("workspace=%q project=%q, want workspace=%q project=%q", gotWorkspace, gotProject, test.wantWorkspace, test.wantProject)
			}
		})
	}
}

func TestXcodeCommandsDetectContainerInWorkingDirectory(t *testing.T) {
	originalRunBuild, originalRunArchive, originalRunTest := runBuild, runArchive, runTest
	t.Cleanup(func() { runBuild, runArchive, runTest = originalRunBuild, originalRunArchive, originalRunTest })

	var gotWorkspace, gotProject string
	runBuild = func(_ context.Context, opts localxcode.BuildOptions) (*localxcode.BuildResult, error) {
		gotWorkspace, gotProject = opts.WorkspacePath, opts.ProjectPath
		return &localxcode.BuildResult{Success: true}, nil
	}
	runArchive = func(_ context.Context, opts localxcode.ArchiveOptions) (*localxcode.ArchiveResult, error) {
		gotWorkspace, gotProject = opts.WorkspacePath, opts.ProjectPath
		return &localxcode.ArchiveResult{}, nil
	}
	runTest = func(_ context.Context, opts localxcode.TestOptions) (*localxcode.TestResult, error) {
		gotWorkspace, gotProject = opts.WorkspacePath, opts.ProjectPath
		return &localxcode.TestResult{Success: true}, nil
	}

	tests := []struct {
		name          string
		command       string
		dirs          []string
		args          []string
		wantWorkspace string
		wantProject   string
		wantStderr    string
		wantUsage     string
		noScheme      bool
	}{
		{name: "lone workspace", command: "build", dirs: []string{"App.xcworkspace"}, wantWorkspace: "App.xcworkspace", wantStderr: "Using workspace App.xcworkspace\n"},
		{name: "lone project", command: "archive", dirs: []string{"App.xcodeproj"}, wantProject: "App.xcodeproj", wantStderr: "Using project App.xcodeproj\n"},
		{name: "workspace wins over project", command: "test", dirs: []string{"App.xcodeproj", "App.xcodeproj/project.xcworkspace", "App.xcworkspace"}, wantWorkspace: "App.xcworkspace", wantStderr: "Using workspace App.xcworkspace\n"},
		{name: "two workspaces", command: "build", dirs: []string{"A.xcworkspace", "B.xcworkspace"}, wantUsage: "exactly one of --workspace or --project is required; found 2 workspaces: A.xcworkspace, B.xcworkspace; pass --workspace"},
		{name: "empty directory", command: "test", wantUsage: "Error: exactly one of --workspace, --project, or --package-path is required\n"},
		{name: "explicitly empty workspace", command: "build", dirs: []string{"App.xcworkspace"}, args: []string{"--workspace", ""}, wantUsage: "--workspace must not be empty"},
		{name: "missing scheme before detection", command: "archive", dirs: []string{"App.xcworkspace"}, noScheme: true, wantUsage: "Error: --scheme is required"},
		{name: "explicit project beside workspace", command: "build", dirs: []string{"App.xcworkspace", "App.xcodeproj"}, args: []string{"--project", "App.xcodeproj"}, wantProject: "App.xcodeproj"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, name := range test.dirs {
				if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			gotWorkspace, gotProject = "", ""

			d := newXcodeTestDeps(nil)
			var cmd *ffcli.Command
			args := append([]string{"--output", "json"}, test.args...)
			if !test.noScheme {
				args = append(args, "--scheme", "App")
			}
			switch test.command {
			case "build":
				cmd = newXcodeBuildCommand(d)
			case "archive":
				cmd = newXcodeArchiveCommand(d)
				args = append(args, "--archive-path", filepath.Join(dir, "out", "App.xcarchive"))
			case "test":
				cmd = newXcodeTestCommand(d)
				args = append(args, "--destination", "generic/platform=iOS")
			}
			cmd.FlagSet.SetOutput(io.Discard)
			if err := cmd.FlagSet.Parse(args); err != nil {
				t.Fatalf("FlagSet.Parse() error = %v", err)
			}
			var runErr error
			_, stderr := captureCommandOutput(t, d, func() error {
				runErr = cmd.Exec(context.Background(), nil)
				return runErr
			})

			if test.wantUsage != "" {
				if !errors.Is(runErr, flag.ErrHelp) {
					t.Fatalf("Exec() error = %v, want usage error", runErr)
				}
				if !strings.Contains(stderr, test.wantUsage) || strings.Contains(stderr, "Using ") {
					t.Fatalf("stderr = %q, want %q and no Using line", stderr, test.wantUsage)
				}
				return
			}
			if runErr != nil {
				t.Fatalf("Exec() error = %v; stderr = %q", runErr, stderr)
			}
			if gotWorkspace != test.wantWorkspace || gotProject != test.wantProject {
				t.Fatalf("workspace=%q project=%q, want workspace=%q project=%q", gotWorkspace, gotProject, test.wantWorkspace, test.wantProject)
			}
			if stderr != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr, test.wantStderr)
			}
		})
	}
}
