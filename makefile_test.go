package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeCleanRemovesReleaseDirectory(t *testing.T) {
	workspaceDir := t.TempDir()
	releaseDir := filepath.Join(workspaceDir, "release")
	staleArtifact := filepath.Join(releaseDir, "stale-artifact")
	if err := os.MkdirAll(releaseDir, 0o755); err != nil {
		t.Fatalf("mkdir release dir: %v", err)
	}
	if err := os.WriteFile(staleArtifact, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale artifact: %v", err)
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "clean")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make clean failed: %v\n%s", err, output)
	}

	if _, err := os.Stat(staleArtifact); !os.IsNotExist(err) {
		t.Fatalf("expected make clean to remove %s, stat err=%v\n%s", staleArtifact, err, output)
	}
}

// makeDryRun returns the commands make would execute for a target without
// running them.
func makeDryRun(t *testing.T, target string) string {
	t.Helper()

	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	cmd := exec.Command("make", "-n", "-f", filepath.Join(repoRoot, "Makefile"), "-C", t.TempDir(), target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n %s failed: %v\n%s", target, err, output)
	}
	return string(output)
}

func TestMakeBuildAllUsesReleaseFlagsAndPerTargetCGO(t *testing.T) {
	output := makeDryRun(t, "build-all")
	for _, want := range []string{
		"-trimpath",
		`-ldflags "-s -w `,
		`if [ "$os" = "darwin" ]; then cgo="1"; else cgo="0"; fi`,
		`CGO_ENABLED="$cgo" GOOS="$os" GOARCH="$arch"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected build-all recipe to contain %q, got:\n%s", want, output)
		}
	}
}

func TestMakeBuildAllFailsWhenAnyTargetFails(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	workspaceDir := t.TempDir()
	fakeGo := filepath.Join(workspaceDir, "fake-go")
	script := `#!/bin/sh
if [ "$1" = "env" ]; then
	exit 0
fi
if [ "$GOOS" = "darwin" ]; then
	echo "simulated Darwin Cgo failure" >&2
	exit 42
fi
exit 0
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}

	cmd := exec.Command(
		"make",
		"-f",
		filepath.Join(repoRoot, "Makefile"),
		"-C",
		workspaceDir,
		"build-all",
		"GO="+fakeGo,
		"VERSION=1.2.3",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("make build-all succeeded despite a failed target:\n%s", output)
	}
	if strings.Contains(string(output), "Release binaries written") {
		t.Fatalf("make build-all reported success after a failed target:\n%s", output)
	}
}

func TestMakeBuildAllQuotesCustomReleaseDirectory(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	workspaceDir := t.TempDir()
	releaseDir := filepath.Join(workspaceDir, "release output")

	cmd := exec.Command(
		"make",
		"-n",
		"-f",
		filepath.Join(repoRoot, "Makefile"),
		"-C",
		workspaceDir,
		"build-all",
		"VERSION=1.2.3",
		"RELEASE_DIR="+releaseDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n build-all with custom output failed: %v\n%s", err, output)
	}
	commands := string(output)
	for _, want := range []string{
		`rm -rf build dist "` + releaseDir + `"`,
		`mkdir -p "` + releaseDir + `"`,
		`-o "` + releaseDir + `/asc_1.2.3_`,
	} {
		if !strings.Contains(commands, want) {
			t.Fatalf("custom release directory is not safely quoted in %q; got:\n%s", want, commands)
		}
	}
}

func TestMakeBuildKeepsDebugSymbolsForDevBuilds(t *testing.T) {
	output := makeDryRun(t, "build")
	for _, unwanted := range []string{"-trimpath", "-s -w"} {
		if strings.Contains(output, unwanted) {
			t.Fatalf("expected dev build recipe to stay unstripped, found %q in:\n%s", unwanted, output)
		}
	}
	if !strings.Contains(output, "-X main.version=") {
		t.Fatalf("expected dev build recipe to inject version metadata, got:\n%s", output)
	}
}

func TestMakeBuildRebuildsBinaryWhenSourceChanges(t *testing.T) {
	workspaceDir := t.TempDir()
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	writeWorkspaceFile := func(path, contents string) {
		t.Helper()
		fullPath := filepath.Join(workspaceDir, path)
		if err := os.WriteFile(fullPath, []byte(contents), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	writeWorkspaceFile("go.mod", "module example.com/makebuildtest\n\ngo 1.24.0\n")
	writeWorkspaceFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Print(\"first\") }\n")

	runMakeBuild := func() {
		t.Helper()
		cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "build")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("make build failed: %v\n%s", err, output)
		}
	}

	runBinary := func() string {
		t.Helper()
		cmd := exec.Command(filepath.Join(workspaceDir, "asc"))
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run built binary failed: %v\n%s", err, output)
		}
		return string(output)
	}

	runMakeBuild()
	if got := runBinary(); got != "first" {
		t.Fatalf("expected initial binary output %q, got %q", "first", got)
	}

	writeWorkspaceFile("main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Print(\"second\") }\n")

	runMakeBuild()
	if got := runBinary(); got != "second" {
		t.Fatalf("expected rebuilt binary output %q, got %q", "second", got)
	}
}

// copyTestRunner gives a temporary workspace the script that make test runs.
func copyTestRunner(t *testing.T, repoRoot, workspaceDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "scripts", "go_test_shard.py"))
	if err != nil {
		t.Fatalf("read test runner: %v", err)
	}
	if err := os.Mkdir(filepath.Join(workspaceDir, "scripts"), 0o700); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceDir, "scripts", "go_test_shard.py"), data, 0o600); err != nil {
		t.Fatalf("write test runner: %v", err)
	}
}

func TestMakeTestTargetsIsolateDeveloperEnvironment(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for _, target := range []string{"test", "test-short", "test-parallel", "test-coverage"} {
		t.Run(target, func(t *testing.T) {
			workspaceDir := t.TempDir()
			tempDir := filepath.Join(workspaceDir, "tmp")
			if err := os.Mkdir(tempDir, 0o700); err != nil {
				t.Fatalf("mkdir tmp: %v", err)
			}
			envLog := filepath.Join(workspaceDir, "test-env")
			stateLog := filepath.Join(workspaceDir, "test-state")
			copyTestRunner(t, repoRoot, workspaceDir)
			fakeGo := filepath.Join(workspaceDir, "fake-go")
			script := `#!/bin/sh
if [ "$1" = "test" ]; then
	env > "$FAKE_GO_ENV_LOG"
	config_dir=$(dirname "$ASC_CONFIG_PATH")
	[ -d "$config_dir" ] && echo "config-dir-exists" >> "$FAKE_GO_STATE_LOG"
	[ -e "$ASC_CONFIG_PATH" ] && echo "config-exists" >> "$FAKE_GO_STATE_LOG"
	[ -w "$config_dir" ] && echo "config-dir-writable" >> "$FAKE_GO_STATE_LOG"
fi
exit 0
`
			if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
				t.Fatalf("write fake go: %v", err)
			}

			cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, target, "GO="+fakeGo)
			cmd.Env = append(
				os.Environ(),
				"TMPDIR="+tempDir,
				"FAKE_GO_ENV_LOG="+envLog,
				"FAKE_GO_STATE_LOG="+stateLog,
				"ASC_APP_ID=developer-app",
				"ASC_CONFIG_PATH=/developer/.asc/config.json",
				"ASC_PROFILE=developer-profile",
				"ASC_TELEMETRY_DISABLED=1",
				"ASC_BYPASS_KEYCHAIN=0",
				"DO_NOT_TRACK=1",
				"ASC_UPDATE_GOLDEN=1",
				"ASC_SIGNING_RUN_LIVE_TEST=1",
				"ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST=1",
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("make %s failed: %v\n%s", target, err, output)
			}

			data, err := os.ReadFile(envLog)
			if err != nil {
				t.Fatalf("read test environment: %v", err)
			}
			got := map[string]string{}
			for _, line := range strings.Split(string(data), "\n") {
				if name, value, ok := strings.Cut(line, "="); ok {
					got[name] = value
				}
			}

			for name, want := range map[string]string{
				"ASC_BYPASS_KEYCHAIN": "1",
				// Opt-in test switches pass through so an explicitly requested
				// golden update or gated live test still runs under make.
				"ASC_UPDATE_GOLDEN":                      "1",
				"ASC_SIGNING_RUN_LIVE_TEST":              "1",
				"ASC_SIGNING_KEYCHAIN_INSTALL_LIVE_TEST": "1",
			} {
				if got[name] != want {
					t.Errorf("%s = %q, want %q", name, got[name], want)
				}
			}
			for _, name := range []string{"ASC_APP_ID", "ASC_PROFILE", "ASC_TELEMETRY_DISABLED", "DO_NOT_TRACK"} {
				if value, ok := got[name]; ok {
					t.Errorf("%s leaked into the test environment as %q", name, value)
				}
			}

			// The config path is a missing file in a fresh per-run directory
			// that tests cannot write to and that is removed afterwards.
			configPath := got["ASC_CONFIG_PATH"]
			configDir := filepath.Dir(configPath)
			if filepath.Base(configPath) != "config.json" || filepath.Dir(configDir) != tempDir ||
				!strings.HasPrefix(filepath.Base(configDir), "asc-test-config.") {
				t.Fatalf("ASC_CONFIG_PATH = %q, want config.json in a fresh directory under %q", configPath, tempDir)
			}
			state, err := os.ReadFile(stateLog)
			if err != nil {
				t.Fatalf("read test state: %v", err)
			}
			want := "config-dir-exists\n"
			if os.Geteuid() == 0 {
				// Root bypasses the read-only mode; the directory is still
				// fresh, empty, and private to this run.
				want += "config-dir-writable\n"
			}
			if string(state) != want {
				t.Fatalf("config state during the run = %q, want %q", state, want)
			}
			if _, err := os.Stat(configDir); !os.IsNotExist(err) {
				t.Fatalf("config directory %q still exists after the run (stat error %v)", configDir, err)
			}
		})
	}
}

func TestMakeTestFailsWhenTestsWriteSharedConfig(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	workspaceDir := t.TempDir()
	tempDir := filepath.Join(workspaceDir, "tmp")
	if err := os.Mkdir(tempDir, 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	copyTestRunner(t, repoRoot, workspaceDir)
	fakeGo := filepath.Join(workspaceDir, "fake-go")
	// Simulate a test that writes the inherited config path, as a root runner
	// could despite the read-only directory.
	script := `#!/bin/sh
if [ "$1" = "test" ]; then
	chmod 700 "$(dirname "$ASC_CONFIG_PATH")"
	echo "{}" > "$ASC_CONFIG_PATH"
fi
exit 0
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}

	cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "test-short", "GO="+fakeGo)
	cmd.Env = append(os.Environ(), "TMPDIR="+tempDir)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("make test-short succeeded although tests wrote the shared config:\n%s", output)
	}
	if !strings.Contains(string(output), "tests wrote to the shared test config directory") {
		t.Fatalf("make test-short output does not explain the failure:\n%s", output)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("config directory left behind: %v", entries)
	}
}

func TestMakeTestRemovesConfigDirectoryWhenInterrupted(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	workspaceDir := t.TempDir()
	tempDir := filepath.Join(workspaceDir, "tmp")
	if err := os.Mkdir(tempDir, 0o700); err != nil {
		t.Fatalf("mkdir tmp: %v", err)
	}
	copyTestRunner(t, repoRoot, workspaceDir)
	fakeGo := filepath.Join(workspaceDir, "fake-go")
	// Terminate the recipe shell while the test command runs, as Ctrl-C or a
	// cancelled CI job would.
	script := `#!/bin/sh
if [ "$1" = "test" ]; then
	kill -TERM "$PPID"
fi
exit 0
`
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}

	cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspaceDir, "test-short", "GO="+fakeGo)
	cmd.Env = append(os.Environ(), "TMPDIR="+tempDir)
	if output, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("make test-short succeeded although the run was terminated:\n%s", output)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("read tmp: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("config directory left behind after termination: %v", entries)
	}
}

func TestMakeBuildsPinMacOSDeploymentTarget(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"build", "build-debug", "build-all"} {
		for _, source := range []string{"environment", "make-argument"} {
			t.Run(target+"/"+source, func(t *testing.T) {
				workspace := t.TempDir()
				fakeGo := filepath.Join(workspace, "fake-go")
				script := `#!/bin/sh
if [ "$1" = "env" ]; then
  [ "$2" != "GOHOSTOS" ] || echo darwin
  exit 0
fi
if [ "$1" = "build" ]; then
  echo "deployment-target=$MACOSX_DEPLOYMENT_TARGET"
  [ "$MACOSX_DEPLOYMENT_TARGET" = "13.0" ] || exit 42
  [ "$CGO_CFLAGS" = "-O0 -g -mmacosx-version-min=13.0" ] || exit 43
  [ "$CGO_LDFLAGS" = "-O0 -g -mmacosx-version-min=13.0" ] || exit 44
fi
`
				if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
					t.Fatal(err)
				}
				cmd := exec.Command("make", "-f", filepath.Join(repoRoot, "Makefile"), "-C", workspace, target, "GO="+fakeGo)
				settings := []string{"MACOSX_DEPLOYMENT_TARGET=27.0", "CGO_CFLAGS=-O0 -g -mmacosx-version-min=27.0", "CGO_LDFLAGS=-O0 -g -mmacosx-version-min=27.0"}
				if source == "make-argument" {
					cmd.Args = append(cmd.Args, settings...)
				} else {
					cmd.Env = append(os.Environ(), settings...)
				}
				output, err := cmd.CombinedOutput()
				if err != nil || !strings.Contains(string(output), "deployment-target=13.0") {
					t.Fatalf("make %s must pin the supported macOS minimum despite the host default: %v\n%s", target, err, output)
				}
			})
		}
	}
}
