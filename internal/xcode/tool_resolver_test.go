package xcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResolveTrustedXcodeToolUsesTrustedXcrunAndSelectedDeveloperDir(t *testing.T) {
	developerDir := filepath.Join(t.TempDir(), "Xcode.app", "Contents", "Developer")
	for _, tool := range []string{"xcodebuild", "agvtool"} {
		pathValue := filepath.Join(developerDir, "usr", "bin", tool)
		if err := os.MkdirAll(filepath.Dir(pathValue), 0o755); err != nil {
			t.Fatalf("MkdirAll() error: %v", err)
		}
		if err := os.WriteFile(pathValue, []byte("tool"), 0o755); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
	}

	trustedPath := filepath.Join(t.TempDir(), "trusted-xcrun")
	var calls []struct {
		name string
		args []string
		cmd  *exec.Cmd
	}
	previousXcrunPath := trustedXcrunPathFn
	previousCommandContext := commandContextFn
	trustedXcrunPathFn = func() (string, error) { return trustedPath, nil }
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		calls = append(calls, struct {
			name string
			args []string
			cmd  *exec.Cmd
		}{name: name, args: append([]string(nil), args...)})
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestTrustedToolResolverHelperProcess", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1", "TRUSTED_TOOL_RESOLVER_OUTPUT="+filepath.Join(developerDir, "usr", "bin", args[len(args)-1]))
		calls[len(calls)-1].cmd = cmd
		return cmd
	}
	t.Cleanup(func() {
		trustedXcrunPathFn = previousXcrunPath
		commandContextFn = previousCommandContext
	})

	pathValue, err := resolveTrustedXcodeToolPath(context.Background(), "xcodebuild", []string{
		"PATH=/tmp/path-shadow",
		"DEVELOPER_DIR=" + developerDir,
		"GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1",
		"TRUSTED_TOOL_RESOLVER_OUTPUT=" + filepath.Join(developerDir, "usr", "bin", "xcodebuild"),
	})
	if err != nil {
		t.Fatalf("resolveTrustedXcodeToolPath() error: %v", err)
	}
	wantPath := filepath.Join(developerDir, "usr", "bin", "xcodebuild")
	if pathValue != wantPath {
		t.Fatalf("resolved path = %q, want %q", pathValue, wantPath)
	}
	if len(calls) != 1 || calls[0].name != trustedPath {
		t.Fatalf("resolver calls = %#v, want one call through %q", calls, trustedPath)
	}
	if len(calls[0].args) != 2 || calls[0].args[0] != "--find" || calls[0].args[1] != "xcodebuild" {
		t.Fatalf("resolver args = %#v, want --find xcodebuild", calls[0].args)
	}
	if !containsDeveloperDirectoryValue(calls[0].cmd.Env, developerDir) {
		t.Fatalf("resolver environment = %#v, want selected developer directory", calls[0].cmd.Env)
	}
	agvtoolPath, err := resolveTrustedXcodeToolPath(context.Background(), "agvtool", []string{
		"PATH=/tmp/path-shadow",
		"DEVELOPER_DIR=" + developerDir,
		"GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1",
		"TRUSTED_TOOL_RESOLVER_OUTPUT=" + filepath.Join(developerDir, "usr", "bin", "agvtool"),
	})
	if err != nil {
		t.Fatalf("resolveTrustedXcodeToolPath(agvtool) error: %v", err)
	}
	if want := filepath.Join(developerDir, "usr", "bin", "agvtool"); agvtoolPath != want {
		t.Fatalf("resolved agvtool path = %q, want %q", agvtoolPath, want)
	}
}

func TestTrustedToolUnavailableClassificationPreservesToolchainConfigurationErrors(t *testing.T) {
	configurationErr := errors.New(`selected developer directory "/missing/Xcode.app" is unavailable`)
	if isTrustedXcodeToolUnavailable(configurationErr, "xcodebuild") {
		t.Fatal("selected developer directory error was classified as a missing xcodebuild")
	}
	if !isTrustedXcodeToolUnavailable(fmt.Errorf("resolve xcodebuild: %w", errTrustedXcodeToolUnavailable), "xcodebuild") {
		t.Fatal("typed trusted-tool error was not classified as unavailable")
	}
	if !isTrustedXcodeToolUnavailable(exec.ErrNotFound, "xcodebuild") {
		t.Fatal("exec.ErrNotFound was not classified as unavailable")
	}
}

func TestResolveTrustedXcodeToolRejectsPathOutsideSelectedDeveloperDir(t *testing.T) {
	developerDir := filepath.Join(t.TempDir(), "Selected.app", "Contents", "Developer")
	if err := os.MkdirAll(developerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	outsidePath := filepath.Join(t.TempDir(), "Other.app", "Contents", "Developer", "usr", "bin", "xcodebuild")
	if err := os.MkdirAll(filepath.Dir(outsidePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := os.WriteFile(outsidePath, []byte("shadow"), 0o755); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	previousXcrunPath := trustedXcrunPathFn
	previousCommandContext := commandContextFn
	trustedXcrunPathFn = func() (string, error) { return "/trusted/xcrun", nil }
	commandContextFn = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestTrustedToolResolverHelperProcess", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1", "TRUSTED_TOOL_RESOLVER_OUTPUT="+outsidePath)
		return cmd
	}
	t.Cleanup(func() {
		trustedXcrunPathFn = previousXcrunPath
		commandContextFn = previousCommandContext
	})

	_, err := resolveTrustedXcodeToolPath(context.Background(), "xcodebuild", []string{
		"DEVELOPER_DIR=" + developerDir,
		"GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1",
		"TRUSTED_TOOL_RESOLVER_OUTPUT=" + outsidePath,
	})
	if err == nil || !strings.Contains(err.Error(), "outside selected developer directory") {
		t.Fatalf("resolveTrustedXcodeToolPath() error = %v, want containment rejection", err)
	}
}

func TestParseTrustedToolPathRejectsMalformedResolverOutput(t *testing.T) {
	for _, test := range []struct {
		name   string
		output string
	}{
		{name: "empty", output: ""},
		{name: "relative", output: "usr/bin/xcodebuild"},
		{name: "multiple paths", output: "/Xcode/usr/bin/xcodebuild\n/tmp/shadow/xcodebuild"},
		{name: "nul", output: "/Xcode/usr/bin/xcodebuild\x00"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseTrustedToolPath(test.output, "xcodebuild"); err == nil {
				t.Fatalf("parseTrustedToolPath(%q) succeeded, want rejection", test.output)
			}
		})
	}
}

func TestTrustedXcodeCommandUsesResolvedAbsolutePath(t *testing.T) {
	const wantPath = "/selected/Xcode.app/Contents/Developer/usr/bin/xcodebuild"
	previousResolver := trustedXcodeToolPathFn
	previousCommandContext := commandContextFn
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		if tool != "xcodebuild" {
			return "", fmt.Errorf("unexpected tool %q", tool)
		}
		return wantPath, nil
	}
	var gotPath string
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		gotPath = name
		return exec.CommandContext(ctx, os.Args[0], "-test.run=TestTrustedToolResolverHelperProcess", "--")
	}
	t.Cleanup(func() {
		trustedXcodeToolPathFn = previousResolver
		commandContextFn = previousCommandContext
	})

	cmd, err := trustedXcodeCommand(context.Background(), "xcodebuild", []string{"-version"}, nil)
	if err != nil {
		t.Fatalf("trustedXcodeCommand() error: %v", err)
	}
	if gotPath != wantPath || cmd == nil {
		t.Fatalf("trustedXcodeCommand() path = %q, want %q", gotPath, wantPath)
	}
}

func TestTrustedXcodeCommandNormalizesRelativeDeveloperDirectoryBeforeChildChangesDirectory(t *testing.T) {
	developerDir := filepath.Join(t.TempDir(), "Developer")
	if err := os.MkdirAll(developerDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeDeveloperDir, err := filepath.Rel(workingDir, developerDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("DEVELOPER_DIR", relativeDeveloperDir)

	previousResolver := trustedXcodeToolPathFn
	trustedXcodeToolPathFn = func(context.Context, string, []string) (string, error) {
		return "/usr/bin/xcodebuild", nil
	}
	t.Cleanup(func() { trustedXcodeToolPathFn = previousResolver })

	cmd, err := trustedXcodeCommand(t.Context(), "xcodebuild", []string{"-version"}, nil)
	if err != nil {
		t.Fatalf("trustedXcodeCommand() error: %v", err)
	}
	cmd.Dir = t.TempDir()
	got, ok := environmentValue(cmd.Env, "DEVELOPER_DIR")
	if !ok || got != developerDir {
		t.Fatalf("child DEVELOPER_DIR = %q, %t; want %q", got, ok, developerDir)
	}
}

func TestRunXcodebuildUsesTrustedResolvedPath(t *testing.T) {
	const wantPath = "/selected/Xcode.app/Contents/Developer/usr/bin/xcodebuild"
	previousResolver := trustedXcodeToolPathFn
	previousCommandContext := commandContextFn
	trustedXcodeToolPathFn = func(_ context.Context, tool string, _ []string) (string, error) {
		if tool != "xcodebuild" {
			return "", fmt.Errorf("unexpected tool %q", tool)
		}
		return wantPath, nil
	}
	var gotPath string
	commandContextFn = func(ctx context.Context, name string, _ ...string) *exec.Cmd {
		gotPath = name
		return exec.CommandContext(ctx, "true")
	}
	t.Cleanup(func() {
		trustedXcodeToolPathFn = previousResolver
		commandContextFn = previousCommandContext
	})

	if err := runXcodebuild(context.Background(), []string{"-version"}, nil); err != nil {
		t.Fatalf("runXcodebuild() error: %v", err)
	}
	if gotPath != wantPath {
		t.Fatalf("runXcodebuild() path = %q, want %q", gotPath, wantPath)
	}
}

func TestResolveTrustedXcrunPathDoesNotConsultPATH(t *testing.T) {
	previousStatPath := statPathFn
	previousLookPath := lookPathFn
	statPathFn = func(path string) (os.FileInfo, error) {
		if path != trustedXcrunPath {
			return nil, errors.New("unexpected stat path")
		}
		return fakeExecutableInfo{}, nil
	}
	lookPathFn = func(string) (string, error) {
		t.Fatal("lookPathFn must not be consulted for trusted xcrun")
		return "", nil
	}
	t.Cleanup(func() {
		statPathFn = previousStatPath
		lookPathFn = previousLookPath
	})

	pathValue, err := resolveToolchainXcrunPath()
	if err != nil {
		t.Fatalf("resolveToolchainXcrunPath() error: %v", err)
	}
	if pathValue != trustedXcrunPath {
		t.Fatalf("resolveToolchainXcrunPath() = %q, want %q", pathValue, trustedXcrunPath)
	}
}

func TestActiveDeveloperDirFromTrustedSelectUsesExactEnvironment(t *testing.T) {
	t.Setenv("DEVELOPER_DIR", "/ambient/Xcode.app/Contents/Developer")
	const want = "/selected/Xcode.app/Contents/Developer"
	exactEnvironment := []string{
		"GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1",
		"TRUSTED_TOOL_RESOLVER_OUTPUT=" + want,
	}
	previousCommandContext := commandContextFn
	var command *exec.Cmd
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != trustedXcodeSelectPath || len(args) != 1 || args[0] != "-p" {
			t.Fatalf("xcode-select command = %q %#v", name, args)
		}
		command = exec.CommandContext(ctx, os.Args[0], "-test.run=TestTrustedToolResolverHelperProcess", "--")
		return command
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })

	got, err := activeDeveloperDirFromTrustedSelect(context.Background(), exactEnvironment)
	if err != nil {
		t.Fatalf("activeDeveloperDirFromTrustedSelect() error: %v", err)
	}
	if got != want {
		t.Fatalf("active developer directory = %q, want %q", got, want)
	}
	if command == nil || containsDeveloperDirectoryValue(command.Env, "/ambient/Xcode.app/Contents/Developer") {
		t.Fatalf("xcode-select inherited ambient DEVELOPER_DIR: %#v", command)
	}
	if len(command.Env) != len(exactEnvironment) {
		t.Fatalf("xcode-select environment = %#v, want exact %#v", command.Env, exactEnvironment)
	}
}

func TestTrustedDeveloperDirectoryTreatsEmptyOverrideAsUnset(t *testing.T) {
	t.Setenv("DEVELOPER_DIR", "/ambient/Xcode.app/Contents/Developer")
	const want = "/selected/Xcode.app/Contents/Developer"
	exactEnvironment := []string{
		"DEVELOPER_DIR=",
		"GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER=1",
		"TRUSTED_TOOL_RESOLVER_OUTPUT=" + want,
	}
	previousCommandContext := commandContextFn
	var command *exec.Cmd
	commandContextFn = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name != trustedXcodeSelectPath || len(args) != 1 || args[0] != "-p" {
			t.Fatalf("xcode-select command = %q %#v", name, args)
		}
		command = exec.CommandContext(ctx, os.Args[0], "-test.run=TestTrustedToolResolverHelperProcess", "--")
		return command
	}
	t.Cleanup(func() { commandContextFn = previousCommandContext })

	got, err := trustedDeveloperDirectory(context.Background(), exactEnvironment)
	if err != nil {
		t.Fatalf("trustedDeveloperDirectory() error: %v", err)
	}
	if got != want {
		t.Fatalf("trusted developer directory = %q, want %q", got, want)
	}
	if command == nil || !containsDeveloperDirectoryValue(command.Env, "") {
		t.Fatalf("xcode-select environment = %#v, want explicit empty DEVELOPER_DIR", command)
	}
	if containsDeveloperDirectoryValue(command.Env, "/ambient/Xcode.app/Contents/Developer") {
		t.Fatalf("xcode-select inherited ambient DEVELOPER_DIR: %#v", command.Env)
	}
}

type fakeExecutableInfo struct{}

func (fakeExecutableInfo) Name() string       { return "xcrun" }
func (fakeExecutableInfo) Size() int64        { return 1 }
func (fakeExecutableInfo) Mode() os.FileMode  { return 0o755 }
func (fakeExecutableInfo) ModTime() time.Time { return time.Time{} }
func (fakeExecutableInfo) IsDir() bool        { return false }
func (fakeExecutableInfo) Sys() any           { return nil }

func containsDeveloperDirectoryValue(environment []string, want string) bool {
	const prefix = "DEVELOPER_DIR="
	for _, entry := range environment {
		if strings.HasPrefix(entry, prefix) && strings.TrimPrefix(entry, prefix) == want {
			return true
		}
	}
	return false
}

func TestTrustedToolResolverHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_TRUSTED_TOOL_RESOLVER_HELPER") != "1" {
		return
	}
	if output := os.Getenv("TRUSTED_TOOL_RESOLVER_OUTPUT"); output != "" {
		fmt.Fprintln(os.Stdout, output)
	}
	os.Exit(0)
}
