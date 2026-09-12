package xcode

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestValidateBuildOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    BuildOptions
		wantErr string
	}{
		{name: "project", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo"}},
		{name: "workspace", opts: BuildOptions{WorkspacePath: "Demo.xcworkspace", Scheme: "Demo"}},
		{name: "non-managed prefix remains available", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-destination-timeout", "60"}}},
		{name: "ordinary build setting remains available", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"test=YES"}}},
		{name: "ordinary conditional build setting remains available", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"OTHER_SWIFT_FLAGS[sdk=iphonesimulator*]=-DASC_BUILD"}}},
		{name: "blank passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{""}}, wantErr: "--xcodebuild-flag cannot be empty"},
		{name: "whitespace passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{" \t"}}, wantErr: "--xcodebuild-flag cannot be empty"},
		{name: "missing selector", opts: BuildOptions{Scheme: "Demo"}, wantErr: "exactly one of --workspace or --project"},
		{name: "both selectors", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", WorkspacePath: "Demo.xcworkspace", Scheme: "Demo"}, wantErr: "exactly one of --workspace or --project"},
		{name: "missing scheme", opts: BuildOptions{ProjectPath: "Demo.xcodeproj"}, wantErr: "--scheme is required"},
		{name: "wrong project suffix", opts: BuildOptions{ProjectPath: "Demo.txt", Scheme: "Demo"}, wantErr: "--project must end with .xcodeproj"},
		{name: "wrong workspace suffix", opts: BuildOptions{WorkspacePath: "Demo.txt", Scheme: "Demo"}, wantErr: "--workspace must end with .xcworkspace"},
		{name: "reserved typed flag passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-destination"}}, wantErr: `cannot override asc-managed argument "-destination"`},
		{name: "reserved equals flag passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-derivedDataPath=/tmp/elsewhere"}}, wantErr: `cannot override asc-managed argument "-derivedDataPath"`},
		{name: "reserved result bundle flag passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-resultBundlePath"}}, wantErr: `cannot override asc-managed argument "-resultBundlePath"`},
		{name: "reserved result bundle passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-resultBundlePath=/tmp/elsewhere.xcresult"}}, wantErr: `cannot override asc-managed argument "-resultBundlePath"`},
		{name: "reserved equals selector passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"-PROJECT=Other.xcodeproj"}}, wantErr: `cannot override asc-managed argument "-PROJECT"`},
		{name: "reserved signing passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"CODE_SIGNING_ALLOWED=NO"}}, wantErr: `cannot override asc-managed argument "CODE_SIGNING_ALLOWED"`},
		{name: "reserved conditional signing passthrough", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"CODE_SIGNING_ALLOWED[sdk=iphoneos*]=YES"}}, wantErr: `cannot override asc-managed argument "CODE_SIGNING_ALLOWED"`},
		{name: "reserved conditional signing passthrough case insensitive", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"code_signing_allowed[config=Debug]=YES"}}, wantErr: `cannot override asc-managed argument "code_signing_allowed"`},
		{name: "reserved action build setting", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"ACTION=archive"}}, wantErr: `cannot override asc-managed argument "ACTION"`},
		{name: "reserved conditional action build setting", opts: BuildOptions{ProjectPath: "Demo.xcodeproj", Scheme: "Demo", XcodebuildArgs: []string{"ACTION[sdk=macosx*]=install"}}, wantErr: `cannot override asc-managed argument "ACTION"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateBuildOptions(test.opts)
			if test.wantErr == "" && err != nil {
				t.Fatalf("ValidateBuildOptions() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("ValidateBuildOptions() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateBuildOptionsRejectsEveryXcodebuildAction(t *testing.T) {
	actions := []string{
		"build",
		"build-for-testing",
		"analyze",
		"archive",
		"test",
		"test-without-building",
		"docbuild",
		"installsrc",
		"installhdrs",
		"install",
		"clean",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			err := ValidateBuildOptions(BuildOptions{
				ProjectPath:    "Demo.xcodeproj",
				Scheme:         "Demo",
				XcodebuildArgs: []string{action},
			})
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("cannot override asc-managed argument %q", action)) {
				t.Fatalf("ValidateBuildOptions() error = %v, want rejected action %q", err, action)
			}
		})
	}
}

func TestValidateBuildOptionsRejectsXcodebuildOperationModes(t *testing.T) {
	operations := []string{
		"-dry-run",
		"-usage",
		"-help",
		"-license",
		"-checkFirstLaunchStatus",
		"-runFirstLaunch",
		"-prepareDeviceSupport",
		"-downloadPlatform",
		"-downloadAllPlatforms",
		"-importPlatform",
		"-downloadComponent",
		"-importComponent",
		"-deleteComponent",
		"-showComponent",
		"-showsdks",
		"-showdestinations",
		"-showTestPlans",
		"-showBuildSettings",
		"-showBuildSettingsForIndex",
		"-list",
		"-find-executable",
		"-find-library",
		"-version",
		"-exportArchive",
		"-exportNotarizedApp",
		"-exportLocalizations",
		"-importLocalizations",
		"-resolvePackageDependencies",
		"-create-xcframework",
		"-target",
		"-alltargets",
	}

	for _, operation := range operations {
		for _, raw := range []string{operation, operation + "=value"} {
			t.Run(raw, func(t *testing.T) {
				err := ValidateBuildOptions(BuildOptions{
					ProjectPath:    "Demo.xcodeproj",
					Scheme:         "Demo",
					XcodebuildArgs: []string{raw},
				})
				want := strings.SplitN(raw, "=", 2)[0]
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("cannot override asc-managed argument %q", want)) {
					t.Fatalf("ValidateBuildOptions() error = %v, want rejected operation %q", err, raw)
				}
			})
		}
	}
}

// TestValidateBuildOptionsRejectsEveryArgumentBuildEmits derives its
// expectations from the shipped command instead of copying the blocklist, so a
// newly managed xcodebuild argument that nobody added to
// reservedBuildPassthroughArgument fails here rather than silently letting
// --xcodebuild-flag override it and make the structured result untruthful.
func TestValidateBuildOptionsRejectsEveryArgumentBuildEmits(t *testing.T) {
	opts := BuildOptions{
		WorkspacePath:    "AscManagedWorkspace.xcworkspace",
		Scheme:           "AscManagedScheme",
		Configuration:    "AscManagedConfiguration",
		Destination:      "AscManagedDestination",
		DerivedDataPath:  "/tmp/asc-managed-derived-data",
		ResultBundlePath: "/tmp/asc-managed-result.xcresult",
		Clean:            true,
		NoCodeSigning:    true,
	}
	suppliedValues := map[string]struct{}{
		opts.WorkspacePath:    {},
		opts.Scheme:           {},
		opts.Configuration:    {},
		opts.Destination:      {},
		opts.DerivedDataPath:  {},
		opts.ResultBundlePath: {},
	}

	// --project is mutually exclusive with --workspace, so it never appears in
	// the same command and needs its own pass.
	projectOpts := opts
	projectOpts.WorkspacePath = ""
	projectOpts.ProjectPath = "AscManagedProject.xcodeproj"
	suppliedValues[projectOpts.ProjectPath] = struct{}{}

	for _, args := range [][]string{buildBuildCommand(opts), buildBuildCommand(projectOpts)} {
		for _, arg := range args {
			if _, isValue := suppliedValues[arg]; isValue {
				continue
			}
			t.Run(arg, func(t *testing.T) {
				if reserved := reservedBuildPassthroughArgument([]string{arg}); reserved == "" {
					t.Fatalf("asc emits %q but --xcodebuild-flag can still override it", arg)
				}
			})
		}
	}
}

func TestBuildCommandUsesTypedOptionsAndPreservesRawArguments(t *testing.T) {
	opts := BuildOptions{
		WorkspacePath:    "Demo App.xcworkspace",
		Scheme:           "Demo App",
		Configuration:    "Release Candidate",
		Destination:      "platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0",
		DerivedDataPath:  "/tmp/Derived Data/Demo",
		ResultBundlePath: "/tmp/Results/Demo.xcresult",
		Clean:            true,
		NoCodeSigning:    true,
		XcodebuildArgs:   []string{"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_BUILD"},
	}

	want := []string{
		"-workspace", "Demo App.xcworkspace",
		"-scheme", "Demo App",
		"-configuration", "Release Candidate",
		"-destination", "platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0",
		"-derivedDataPath", "/tmp/Derived Data/Demo",
		"-resultBundlePath", "/tmp/Results/Demo.xcresult",
		"-quiet", "OTHER_SWIFT_FLAGS=-D ASC_BUILD",
		"CODE_SIGNING_ALLOWED=NO",
		"clean", "build",
	}
	if got := buildBuildCommand(opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("buildBuildCommand() = %#v\nwant %#v", got, want)
	}
}

func TestBuildCommandDoesNotChangeSigningByDefault(t *testing.T) {
	opts := BuildOptions{
		ProjectPath:     "Demo.xcodeproj",
		Scheme:          "Demo",
		Destination:     "generic/platform=iOS",
		DerivedDataPath: "/tmp/DemoDerivedData",
	}
	args := buildBuildCommand(opts)
	if containsArg(args, "CODE_SIGNING_ALLOWED=NO") {
		t.Fatalf("default build unexpectedly disabled signing: %#v", args)
	}
	if got, want := args[len(args)-1], "build"; got != want {
		t.Fatalf("last argument = %q, want %q", got, want)
	}
}

func TestResolveBuildDerivedDataPathIsStableAndOutsideProject(t *testing.T) {
	cacheDir := t.TempDir()
	originalUserCacheDir := userCacheDirFn
	userCacheDirFn = func() (string, error) { return cacheDir, nil }
	t.Cleanup(func() { userCacheDirFn = originalUserCacheDir })

	projectDir := t.TempDir()
	projectPath := filepath.Join(projectDir, "Demo.xcodeproj")
	opts := BuildOptions{
		ProjectPath:   projectPath,
		Scheme:        "Demo App",
		Configuration: "Debug",
		Destination:   "platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0",
	}
	first, err := resolveBuildDerivedDataPath(opts)
	if err != nil {
		t.Fatalf("resolveBuildDerivedDataPath() error = %v", err)
	}
	second, err := resolveBuildDerivedDataPath(opts)
	if err != nil {
		t.Fatalf("resolveBuildDerivedDataPath() second error = %v", err)
	}
	if first != second {
		t.Fatalf("derived-data path is not stable: %q != %q", first, second)
	}
	if !strings.HasPrefix(first, filepath.Join(cacheDir, "asc", "xcode-build")+string(os.PathSeparator)) {
		t.Fatalf("derived-data path %q is not in asc cache %q", first, cacheDir)
	}
	if strings.HasPrefix(first, projectDir+string(os.PathSeparator)) {
		t.Fatalf("derived-data path %q is inside source checkout %q", first, projectDir)
	}

	opts.Destination = "generic/platform=iOS"
	third, err := resolveBuildDerivedDataPath(opts)
	if err != nil {
		t.Fatalf("resolveBuildDerivedDataPath() changed destination error = %v", err)
	}
	if third == first {
		t.Fatalf("destination change did not change derived-data path: %q", third)
	}
}

func TestResolveBuildDerivedDataPathMakesExplicitPathAbsolute(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	got, err := resolveBuildDerivedDataPath(BuildOptions{DerivedDataPath: "Derived Data"})
	if err != nil {
		t.Fatalf("resolveBuildDerivedDataPath() error = %v", err)
	}
	want := filepath.Join(workingDirectory, "Derived Data")
	if got != want {
		t.Fatalf("resolveBuildDerivedDataPath() = %q, want %q", got, want)
	}
}

func TestResolveBuildResultBundlePathMakesExplicitPathAbsolute(t *testing.T) {
	workingDirectory := t.TempDir()
	t.Chdir(workingDirectory)

	got, err := resolveBuildResultBundlePath("Results/Demo.xcresult")
	if err != nil {
		t.Fatalf("resolveBuildResultBundlePath() error = %v", err)
	}
	want := filepath.Join(workingDirectory, "Results", "Demo.xcresult")
	if got != want {
		t.Fatalf("resolveBuildResultBundlePath() = %q, want %q", got, want)
	}
}

func TestSafeBuildPathComponentTruncatesOnRuneBoundary(t *testing.T) {
	got := safeBuildPathComponent(strings.Repeat("界", 60))
	if !utf8.ValidString(got) {
		t.Fatalf("safeBuildPathComponent() returned invalid UTF-8: %q", got)
	}
	if gotRunes := utf8.RuneCountInString(got); gotRunes != 48 {
		t.Fatalf("safeBuildPathComponent() rune count = %d, want 48", gotRunes)
	}
}

func TestBuildReturnsStructuredSuccessAndProductsDirectory(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	derivedDataPath := filepath.Join(t.TempDir(), "Derived Data")
	productsPath := filepath.Join(derivedDataPath, "Build", "Products")
	restore := overrideBuildProcess(t, "success", productsPath)
	defer restore()

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:      projectPath,
		Scheme:           "Demo",
		Configuration:    "Debug",
		Destination:      "platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0",
		DerivedDataPath:  derivedDataPath,
		ResultBundlePath: filepath.Join(t.TempDir(), "Demo.xcresult"),
		NoCodeSigning:    true,
		LogWriter:        io.Discard,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("Build() result = %+v, want success", result)
	}
	if result.BuildProductsPath != productsPath {
		t.Fatalf("BuildProductsPath = %q, want %q", result.BuildProductsPath, productsPath)
	}
	if result.DurationMS < 0 {
		t.Fatalf("DurationMS = %d, want non-negative", result.DurationMS)
	}
	if result.ExitStatus == nil || *result.ExitStatus != 0 {
		t.Fatalf("ExitStatus = %v, want pointer to 0", result.ExitStatus)
	}
	if !filepath.IsAbs(result.ResultBundlePath) || !strings.HasSuffix(result.ResultBundlePath, "Demo.xcresult") {
		t.Fatalf("ResultBundlePath = %q, want resolved requested path", result.ResultBundlePath)
	}
}

func TestBuildOmitsPreexistingBuildProductsDirectory(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	derivedDataPath := filepath.Join(t.TempDir(), "DerivedData")
	productsPath := filepath.Join(derivedDataPath, "Build", "Products")
	if err := os.MkdirAll(productsPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() products error = %v", err)
	}
	restore := overrideBuildProcess(t, "success")
	defer restore()

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: derivedDataPath,
		XcodebuildArgs:  []string{"CONFIGURATION_BUILD_DIR=/tmp/redirected"},
		LogWriter:       io.Discard,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if result.BuildProductsPath != "" {
		t.Fatalf("BuildProductsPath = %q, want omitted preexisting directory", result.BuildProductsPath)
	}
}

func TestBuildPreservesFailureExitStatus(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	restore := overrideBuildProcess(t, "failure")
	defer restore()

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
		LogWriter:       io.Discard,
	})
	if err == nil {
		t.Fatal("Build() error = nil, want subprocess failure")
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want structured failure", result)
	}
	if result.ExitStatus == nil || *result.ExitStatus != 65 {
		t.Fatalf("ExitStatus = %v, want pointer to 65", result.ExitStatus)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Build() error = %T %v, want wrapped *exec.ExitError", err, err)
	}
}

func TestBuildOmitsExitStatusForSignaledFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows process termination does not expose Unix signal status")
	}
	projectPath := createBuildTestContainer(t)
	restore := overrideBuildProcess(t, "signal")
	defer restore()

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
		LogWriter:       io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "signal:") {
		t.Fatalf("Build() error = %v, want signal failure", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want structured failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil for signaled process", result.ExitStatus)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != -1 {
		t.Fatalf("Build() error = %T %v, want signaled *exec.ExitError", err, err)
	}
}

func TestBuildPreservesContextCancellation(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	restore := overrideBuildProcess(t, "wait")
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	result, err := Build(ctx, BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
		LogWriter:       io.Discard,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Build() error = %v, want context deadline exceeded", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want canceled failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil for cancellation", result.ExitStatus)
	}
}

func TestBuildOmitsExitStatusForPreflightFailure(t *testing.T) {
	result, err := Build(context.Background(), BuildOptions{
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one of --workspace or --project") {
		t.Fatalf("Build() error = %v, want selector preflight error", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want structured preflight failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil before xcodebuild starts", result.ExitStatus)
	}
}

func TestBuildRejectsExistingResultBundleBeforeStartingProcess(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
	}{
		{name: "directory", setup: func(t *testing.T, path string) {
			t.Helper()
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatalf("MkdirAll() result bundle error = %v", err)
			}
		}},
		{name: "dangling symlink", setup: func(t *testing.T, path string) {
			t.Helper()
			if runtime.GOOS == "windows" {
				t.Skip("Windows symlink creation requires elevated privileges")
			}
			if err := os.Symlink(filepath.Join(filepath.Dir(path), "missing-target"), path); err != nil {
				t.Fatalf("Symlink() result bundle error = %v", err)
			}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectPath := createBuildTestContainer(t)
			resultBundlePath := filepath.Join(t.TempDir(), "Existing.xcresult")
			test.setup(t, resultBundlePath)

			originalGOOS := runtimeGOOS
			originalLookPath := lookPathFn
			runtimeGOOS = "darwin"
			lookPathFn = func(string) (string, error) {
				t.Fatal("xcodebuild lookup must not run for an existing result bundle path")
				return "", nil
			}
			t.Cleanup(func() {
				runtimeGOOS = originalGOOS
				lookPathFn = originalLookPath
			})

			result, err := Build(context.Background(), BuildOptions{
				ProjectPath:      projectPath,
				Scheme:           "Demo",
				DerivedDataPath:  filepath.Join(t.TempDir(), "DerivedData"),
				ResultBundlePath: resultBundlePath,
			})
			if err == nil || !strings.Contains(err.Error(), "--result-bundle-path already exists") {
				t.Fatalf("Build() error = %v, want existing result bundle error", err)
			}
			if result == nil || result.ResultBundlePath != resultBundlePath {
				t.Fatalf("Build() result = %+v, want resolved result bundle path", result)
			}
			if result.ExitStatus != nil {
				t.Fatalf("ExitStatus = %v, want nil before xcodebuild starts", result.ExitStatus)
			}
		})
	}
}

func TestBuildOmitsExitStatusForSilentVersionProbeFailure(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	restore := overrideBuildProcess(t, "version-failure")
	defer restore()

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
		LogWriter:       io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "xcodebuild not usable") {
		t.Fatalf("Build() error = %v, want unusable xcodebuild preflight error", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want structured preflight failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil for version probe failure", result.ExitStatus)
	}
}

func TestBuildRejectsUnsupportedHostBeforeStartingProcess(t *testing.T) {
	projectPath := createBuildTestContainer(t)
	originalGOOS := runtimeGOOS
	runtimeGOOS = "linux"
	t.Cleanup(func() { runtimeGOOS = originalGOOS })

	result, err := Build(context.Background(), BuildOptions{
		ProjectPath:     projectPath,
		Scheme:          "Demo",
		DerivedDataPath: filepath.Join(t.TempDir(), "DerivedData"),
	})
	if err == nil || !strings.Contains(err.Error(), "supported on macOS only") {
		t.Fatalf("Build() error = %v, want macOS-only error", err)
	}
	if result == nil || result.Success {
		t.Fatalf("Build() result = %+v, want structured failure", result)
	}
	if result.ExitStatus != nil {
		t.Fatalf("ExitStatus = %v, want nil before xcodebuild starts", result.ExitStatus)
	}
}

func createBuildTestContainer(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Demo.xcodeproj")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return path
}

func overrideBuildProcess(t *testing.T, mode string, productsPaths ...string) func() {
	t.Helper()
	productsPath := ""
	if len(productsPaths) > 0 {
		productsPath = productsPaths[0]
	}
	originalGOOS := runtimeGOOS
	originalLookPath := lookPathFn
	originalCommandContext := commandContextFn
	originalTrustedXcodeToolPath := trustedXcodeToolPathFn
	runtimeGOOS = "darwin"
	lookPathFn = func(string) (string, error) { return "/usr/bin/xcodebuild", nil }
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		return lookPathFn(tool)
	}
	commandContextFn = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		commandArgs := []string{"-test.run=TestBuildHelperProcess", "--"}
		commandArgs = append(commandArgs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], commandArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_BUILD_HELPER_PROCESS=1", "ASC_BUILD_HELPER_MODE="+mode, "ASC_BUILD_PRODUCTS_PATH="+productsPath)
		return cmd
	}
	return func() {
		runtimeGOOS = originalGOOS
		lookPathFn = originalLookPath
		commandContextFn = originalCommandContext
		trustedXcodeToolPathFn = originalTrustedXcodeToolPath
	}
}

func TestBuildHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_BUILD_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	separator := -1
	for index, arg := range args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator == -1 {
		os.Exit(2)
	}
	commandArgs := args[separator+1:]
	if len(commandArgs) == 1 && commandArgs[0] == "-version" {
		if os.Getenv("ASC_BUILD_HELPER_MODE") == "version-failure" {
			os.Exit(72)
		}
		os.Exit(0)
	}
	switch os.Getenv("ASC_BUILD_HELPER_MODE") {
	case "success":
		if productsPath := os.Getenv("ASC_BUILD_PRODUCTS_PATH"); productsPath != "" {
			if err := os.MkdirAll(productsPath, 0o755); err != nil {
				os.Exit(2)
			}
		}
		os.Exit(0)
	case "failure":
		_, _ = io.WriteString(os.Stderr, "compile failed\n")
		os.Exit(65)
	case "signal":
		process, err := os.FindProcess(os.Getpid())
		if err != nil || process.Kill() != nil {
			os.Exit(2)
		}
		os.Exit(2)
	case "wait":
		time.Sleep(2 * time.Second)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
