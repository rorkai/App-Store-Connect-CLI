package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/testguard"
)

type accountHomeFixture struct {
	accountHome   string
	accountConfig string
	projectDir    string
	scratchHome   string
	warnings      *bytes.Buffer
}

// newAccountHomeFixture builds an account home that holds its own
// .asc/config.json, a project directory nested inside it, and a separate
// scratch directory to use as an overridden HOME. It points the account home
// lookup at the fixture and captures the warning output.
func newAccountHomeFixture(t *testing.T) accountHomeFixture {
	t.Helper()

	// Resolve the temp root so the fixture paths are spelled the way os.Getwd
	// reports them (macOS temp dirs live behind the /var -> /private/var link).
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	accountHome := filepath.Join(root, "Users", "someone")
	accountConfig := filepath.Join(accountHome, ".asc", "config.json")
	projectDir := filepath.Join(accountHome, "Developer", "project", "sub")
	scratchHome := filepath.Join(root, "scratch-home")
	for _, dir := range []string{filepath.Dir(accountConfig), projectDir, scratchHome} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}
	if err := os.WriteFile(accountConfig, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", accountConfig, err)
	}

	previousAccountHomeDir := accountHomeDir
	accountHomeDir = func() (string, error) { return accountHome, nil }
	warnings := &bytes.Buffer{}
	previousOutput := accountHomeWarningOutput
	accountHomeWarningOutput = warnings
	resetAccountHomeWarning()
	t.Cleanup(func() {
		accountHomeDir = previousAccountHomeDir
		accountHomeWarningOutput = previousOutput
		resetAccountHomeWarning()
	})

	return accountHomeFixture{
		accountHome:   accountHome,
		accountConfig: accountConfig,
		projectDir:    projectDir,
		scratchHome:   scratchHome,
		warnings:      warnings,
	}
}

func resetAccountHomeWarning() {
	accountHomeWarningMu.Lock()
	accountHomeWarningWritten = false
	accountHomeWarningMu.Unlock()
}

func assertResolvedPath(t *testing.T, want string) {
	t.Helper()
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	gotInfo, gotErr := os.Stat(got)
	wantInfo, wantErr := os.Stat(want)
	if got != want && (gotErr != nil || wantErr != nil || !os.SameFile(gotInfo, wantInfo)) {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestPathWarnsWhenUpwardSearchReachesAccountHomeUnderOverriddenHome(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", fixture.scratchHome)
	t.Chdir(fixture.projectDir)

	// Compatibility: the account home config is still selected for now.
	assertResolvedPath(t, fixture.accountConfig)

	warning := fixture.warnings.String()
	for _, want := range []string{
		"Warning:",
		filepath.Join(".asc", "config.json"),
		"HOME is " + fixture.scratchHome,
		"ASC_CONFIG_PATH",
		"future release",
	} {
		if !strings.Contains(warning, want) {
			t.Fatalf("warning %q does not contain %q", warning, want)
		}
	}

	// The warning is printed once per process, not once per lookup.
	assertResolvedPath(t, fixture.accountConfig)
	if count := strings.Count(fixture.warnings.String(), "Warning:"); count != 1 {
		t.Fatalf("warning printed %d times, want 1: %q", count, fixture.warnings.String())
	}
}

func TestPathDoesNotWarnWhenHomeIsAccountHome(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", fixture.accountHome)
	t.Chdir(fixture.projectDir)

	assertResolvedPath(t, fixture.accountConfig)
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}

func TestPathDoesNotWarnWhenHomeIsSymlinkToAccountHome(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	link := filepath.Join(t.TempDir(), "home-link")
	if err := os.Symlink(fixture.accountHome, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", link)
	t.Chdir(fixture.projectDir)

	assertResolvedPath(t, fixture.accountConfig)
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}

func TestPathPrefersProjectConfigInsideAccountHomeWithoutWarning(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	projectRoot := filepath.Dir(fixture.projectDir)
	projectConfig := filepath.Join(projectRoot, ".asc", "config.json")
	if err := os.MkdirAll(filepath.Dir(projectConfig), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(projectConfig, []byte("{}"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", fixture.scratchHome)
	t.Chdir(fixture.projectDir)

	assertResolvedPath(t, projectConfig)
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}

func TestPathEnvOverrideWinsOverAccountHomeConfig(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	override := filepath.Join(t.TempDir(), "missing", "config.json")
	t.Setenv("ASC_CONFIG_PATH", override)
	t.Setenv("HOME", fixture.scratchHome)
	t.Chdir(fixture.projectDir)

	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if got != override {
		t.Fatalf("Path() = %q, want %q", got, override)
	}
	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}

func TestPathDoesNotWarnOutsideAccountHome(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	outside := t.TempDir()
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", fixture.scratchHome)
	t.Chdir(outside)

	assertResolvedPath(t, filepath.Join(fixture.scratchHome, ".asc", "config.json"))
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}

func TestPathDoesNotWarnAboutAccountHomeConfigTheTestGuardHides(t *testing.T) {
	fixture := newAccountHomeFixture(t)
	t.Cleanup(testguard.HideForTest(filepath.Dir(fixture.accountConfig)))
	t.Setenv("ASC_CONFIG_PATH", "")
	t.Setenv("HOME", fixture.scratchHome)
	t.Chdir(fixture.projectDir)

	assertResolvedPath(t, fixture.accountConfig)
	if _, err := Load(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load() error = %v, want ErrNotFound", err)
	}
	if got := fixture.warnings.String(); got != "" {
		t.Fatalf("unexpected warning: %q", got)
	}
}
