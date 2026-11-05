package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

func preserveInstallerGlobals(t *testing.T) {
	t.Helper()
	originalLookup := lookupExecutable
	originalRunGit := runGitCommand
	originalCheckout := checkoutSkillsSource
	originalHome := userHomeDirectory
	originalRename := renameWithinRoot
	originalTime := currentTime
	t.Cleanup(func() {
		lookupExecutable = originalLookup
		runGitCommand = originalRunGit
		checkoutSkillsSource = originalCheckout
		userHomeDirectory = originalHome
		renameWithinRoot = originalRename
		currentTime = originalTime
	})
}

func installFixture(t *testing.T, checkout func(gitIsolationPaths) error) string {
	t.Helper()
	preserveInstallerGlobals(t)
	t.Setenv("XDG_STATE_HOME", "")

	home := t.TempDir()
	currentTime = func() time.Time {
		return time.Date(2026, time.July, 30, 12, 0, 0, 0, time.UTC)
	}
	userHomeDirectory = func() (string, error) { return home, nil }
	lookupExecutable = func(name string) (string, error) {
		if name != "git" {
			t.Fatalf("looked up unexpected executable %q", name)
		}
		return "/test/bin/git", nil
	}
	checkoutSkillsSource = func(_ context.Context, gitPath string, paths gitIsolationPaths) error {
		if gitPath != "/test/bin/git" {
			t.Fatalf("git path = %q", gitPath)
		}
		return checkout(paths)
	}
	return home
}

func populatePinnedSkillsFixture(t *testing.T, sourceDir string, names []string) {
	t.Helper()
	for _, name := range names {
		skillDir := filepath.Join(sourceDir, "skills", name)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			t.Fatalf("create %s fixture: %v", name, err)
		}
		content := []byte("---\nname: " + name + "\n---\n")
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), content, 0o644); err != nil {
			t.Fatalf("write %s fixture: %v", name, err)
		}
	}
	referenceDir := filepath.Join(sourceDir, "skills", "asc-release-flow", "references")
	if err := os.MkdirAll(referenceDir, 0o755); err != nil {
		t.Fatalf("create nested reference fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(referenceDir, "release.md"), []byte("reviewed reference\n"), 0o644); err != nil {
		t.Fatalf("write nested reference fixture: %v", err)
	}
}

func runInstallCommand(t *testing.T, args ...string) error {
	t.Helper()
	cmd := InstallSkillsCommand()
	if err := cmd.Parse(args); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return cmd.Run(context.Background())
}

func TestSkillsSourceIsPinnedToImmutableInput(t *testing.T) {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(skillsSourceCommit) {
		t.Fatalf("skillsSourceCommit = %q, want a full 40-character commit SHA", skillsSourceCommit)
	}
	if skillsSourceRepositoryURL != "https://github.com/rorkai/app-store-connect-cli-skills.git" {
		t.Fatalf("skillsSourceRepositoryURL = %q, want the reviewed ASC skills repository", skillsSourceRepositoryURL)
	}
	if len(expectedSkillNames) != expectedSkillsCount {
		t.Fatalf("expectedSkillNames has %d entries, want %d", len(expectedSkillNames), expectedSkillsCount)
	}
	if len(expectedSkillTreeHashes) != expectedSkillsCount {
		t.Fatalf("expectedSkillTreeHashes has %d entries, want %d", len(expectedSkillTreeHashes), expectedSkillsCount)
	}
	seen := make(map[string]struct{}, len(expectedSkillNames))
	for _, name := range expectedSkillNames {
		if _, duplicate := seen[name]; duplicate {
			t.Fatalf("duplicate expected skill %q", name)
		}
		seen[name] = struct{}{}
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(expectedSkillTreeHashes[name]) {
			t.Fatalf("tree hash for %s = %q, want a Git object ID", name, expectedSkillTreeHashes[name])
		}
	}
}

func TestInstallSkillsRejectsPositionalArgumentsBeforeSideEffects(t *testing.T) {
	preserveInstallerGlobals(t)
	lookupExecutable = func(name string) (string, error) {
		t.Fatalf("positional argument caused executable lookup for %q", name)
		return "", nil
	}

	err := runInstallCommand(t, "unexpected")
	if err == nil {
		t.Fatal("expected positional argument error")
	}
	if !strings.Contains(err.Error(), "unexpected arguments: unexpected") {
		t.Fatalf("error = %q, want unexpected argument diagnostic", err)
	}
}

func TestInstallSkillsCopiesExactPackWithoutNodeAndPreservesUnrelatedData(t *testing.T) {
	home := installFixture(t, func(paths gitIsolationPaths) error {
		populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
		return nil
	})

	unrelatedDir := filepath.Join(home, ".agents", "skills", "user-owned-skill")
	if err := os.MkdirAll(unrelatedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unrelatedDir, "SKILL.md"), []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(home, ".agents", ".skill-lock.json")
	lockData := []byte(`{
  "version": 3,
  "skills": {
    "user-owned-skill": {
      "source": "local",
      "custom": {"preserve": true}
    },
    "asc-app-create-ui": {
      "source": "rorkai/app-store-connect-cli-skills",
      "sourceType": "github",
      "sourceUrl": "https://github.com/rorkai/app-store-connect-cli-skills.git",
      "skillPath": "skills/asc-app-create-ui/SKILL.md",
      "skillFolderHash": "stale",
      "installedAt": "2026-01-02T03:04:05Z",
      "updatedAt": "2026-01-02T03:04:05Z"
    }
  },
  "dismissed": {"findSkillsPrompt": true},
  "lastSelectedAgents": ["codex"],
  "futureTopLevel": {"keep": 1}
}`)
	if err := os.WriteFile(lockPath, lockData, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := runInstallCommand(t); err != nil {
		t.Fatalf("install: %v", err)
	}

	for _, name := range expectedSkillNames {
		data, err := os.ReadFile(filepath.Join(home, ".agents", "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatalf("%s was not installed: %v", name, err)
		}
		if !strings.Contains(string(data), "name: "+name) {
			t.Fatalf("%s content = %q", name, data)
		}
	}
	for _, name := range []string{"asc-ad-hoc-distribution", "asc-analytics-reports"} {
		if _, err := os.Stat(filepath.Join(home, ".agents", "skills", name, "SKILL.md")); err != nil {
			t.Errorf("current workflow skill %s was not installed: %v", name, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "asc-release-flow", "references", "release.md")); err != nil {
		t.Fatalf("nested skill file was not installed: %v", err)
	} else if string(data) != "reviewed reference\n" {
		t.Fatalf("nested skill content = %q", data)
	}
	if data, err := os.ReadFile(filepath.Join(unrelatedDir, "SKILL.md")); err != nil || string(data) != "keep me\n" {
		t.Fatalf("unrelated skill changed: data=%q err=%v", data, err)
	}
	assertPinnedLock(t, lockPath, "2026-01-02T03:04:05Z")
	assertNoInstallWorkspace(t, filepath.Join(home, ".agents"))
}

func TestInstallSkillsReplacesOnlyPinnedSkills(t *testing.T) {
	home := installFixture(t, func(paths gitIsolationPaths) error {
		populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
		return nil
	})
	oldPath := filepath.Join(home, ".agents", "skills", expectedSkillNames[0], "old-only.txt")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runInstallCommand(t); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old pinned-skill content was not replaced: %v", err)
	}
}

func TestInstallSkillsRejectsIncompletePackBeforeReplacingAnything(t *testing.T) {
	missing := append([]string(nil), expectedSkillNames[:len(expectedSkillNames)-1]...)
	home := installFixture(t, func(paths gitIsolationPaths) error {
		populatePinnedSkillsFixture(t, paths.sourceDir, missing)
		return nil
	})
	existing := filepath.Join(home, ".agents", "skills", expectedSkillNames[0], "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte("existing\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runInstallCommand(t)
	if err == nil {
		t.Fatal("expected incomplete pack error")
	}
	if !errors.Is(err, errIncompleteSkillSet) {
		t.Fatalf("error = %v, want errIncompleteSkillSet", err)
	}
	if data, readErr := os.ReadFile(existing); readErr != nil || string(data) != "existing\n" {
		t.Fatalf("existing skill changed before pack validation: data=%q err=%v", data, readErr)
	}
}

func TestInstallSkillPackRollsBackEveryReplacementOnPartialFailure(t *testing.T) {
	sourceDir := t.TempDir()
	populatePinnedSkillsFixture(t, sourceDir, expectedSkillNames)
	source, err := os.OpenRoot(filepath.Join(sourceDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	destinationDir := t.TempDir()
	for _, name := range expectedSkillNames {
		path := filepath.Join(destinationDir, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("old "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	unrelated := filepath.Join(destinationDir, "user-owned", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(unrelated), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("unrelated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	destination, err := os.OpenRoot(destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()

	preserveInstallerGlobals(t)
	failed := false
	renameWithinRoot = func(root *os.Root, oldName, newName string) error {
		if !failed && strings.Contains(oldName, ".asc-stage-") && filepath.Base(oldName) == expectedSkillNames[2] {
			failed = true
			return errors.New("injected install failure")
		}
		return root.Rename(oldName, newName)
	}

	names := append([]string(nil), expectedSkillNames...)
	sort.Strings(names)
	err = installSkillPack(source, destination, names)
	if err == nil || !strings.Contains(err.Error(), "injected install failure") {
		t.Fatalf("error = %v, want injected partial failure", err)
	}
	for _, name := range expectedSkillNames {
		data, readErr := os.ReadFile(filepath.Join(destinationDir, name, "SKILL.md"))
		if readErr != nil {
			t.Fatalf("%s was not restored: %v", name, readErr)
		}
		if string(data) != "old "+name+"\n" {
			t.Fatalf("%s was not rolled back: %q", name, data)
		}
	}
	if data, readErr := os.ReadFile(unrelated); readErr != nil || string(data) != "unrelated\n" {
		t.Fatalf("unrelated skill changed: data=%q err=%v", data, readErr)
	}
}

func TestInstallSkillPackPreservesBackupWhenRollbackRestoreFails(t *testing.T) {
	sourceDir := t.TempDir()
	populatePinnedSkillsFixture(t, sourceDir, expectedSkillNames)
	source, err := os.OpenRoot(filepath.Join(sourceDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	destinationDir := t.TempDir()
	for _, name := range expectedSkillNames {
		path := filepath.Join(destinationDir, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("old "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination, err := os.OpenRoot(destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()

	preserveInstallerGlobals(t)
	stageFailed := false
	restoreFailed := false
	renameWithinRoot = func(root *os.Root, oldName, newName string) error {
		if !stageFailed && strings.Contains(oldName, ".asc-stage-") && filepath.Base(oldName) == expectedSkillNames[2] {
			stageFailed = true
			return errors.New("injected install failure")
		}
		if !restoreFailed && strings.Contains(oldName, ".asc-backup-") && filepath.Base(oldName) == expectedSkillNames[0] {
			restoreFailed = true
			return errors.New("injected restore failure")
		}
		return root.Rename(oldName, newName)
	}

	names := append([]string(nil), expectedSkillNames...)
	sort.Strings(names)
	err = installSkillPack(source, destination, names)
	if err == nil {
		t.Fatal("expected rollback failure")
	}
	var retained *retainedSkillBackupError
	if !errors.As(err, &retained) {
		t.Fatalf("error = %v, want retainedSkillBackupError", err)
	}
	if !strings.Contains(err.Error(), "previous skill data was preserved at") ||
		!strings.Contains(err.Error(), "restore needed entries") {
		t.Fatalf("error lacks recovery instructions: %v", err)
	}
	data, readErr := os.ReadFile(filepath.Join(retained.path, expectedSkillNames[0], "SKILL.md"))
	if readErr != nil || string(data) != "old "+expectedSkillNames[0]+"\n" {
		t.Fatalf("only recovery backup was lost: data=%q err=%v path=%s", data, readErr, retained.path)
	}
}

func TestInstallSkillsSerializesAcrossProcessesWithAdvisoryLock(t *testing.T) {
	agentsDir := t.TempDir()
	root, err := os.OpenRoot(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	path := filepath.Join(agentsDir, installLockName)
	lockContents := []byte("persistent operator metadata must not be rewritten\n")
	if err := os.WriteFile(path, lockContents, 0o600); err != nil {
		t.Fatal(err)
	}

	first, err := acquireInstallLock(root, path)
	if err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if _, err := acquireInstallLock(root, path); err == nil || !errors.Is(err, errInstallLockHeld) {
		t.Fatalf("second lock error = %v, want held advisory lock", err)
	}
	if err := first.release(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || !reflect.DeepEqual(data, lockContents) {
		t.Fatalf("advisory locking mutated persistent lock inode: data=%q err=%v", data, err)
	}

	// The lock file is intentionally persistent. Advisory ownership is released
	// by the OS on close or process death, so a stale metadata file cannot
	// strand future installs.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persistent lock metadata missing: %v", err)
	}
	second, err := acquireInstallLock(root, path)
	if err != nil {
		t.Fatalf("reacquire persistent lock after release: %v", err)
	}
	if err := second.release(); err != nil {
		t.Fatalf("release second lock: %v", err)
	}
}

func TestInstallLockNeverMutatesPreexistingHardlink(t *testing.T) {
	agentsDir := t.TempDir()
	backing := filepath.Join(t.TempDir(), "operator-data")
	original := []byte("must survive advisory locking\n")
	if err := os.WriteFile(backing, original, 0o600); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(agentsDir, installLockName)
	if err := os.Link(backing, lockPath); err != nil {
		t.Skipf("hardlinks are not available: %v", err)
	}
	root, err := os.OpenRoot(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	lock, err := acquireInstallLock(root, lockPath)
	if err != nil {
		t.Fatalf("acquire hard-linked advisory lock: %v", err)
	}
	if err := lock.release(); err != nil {
		t.Fatalf("release hard-linked advisory lock: %v", err)
	}
	data, err := os.ReadFile(backing)
	if err != nil || !reflect.DeepEqual(data, original) {
		t.Fatalf("hardlink target was mutated: data=%q err=%v", data, err)
	}
}

func TestUntrackedRecoveryLockRemovesOnlyASCEntries(t *testing.T) {
	original := []byte(`{
  "version": 3,
  "skills": {
    "unrelated": {"source": "local", "future": {"keep": true}},
    "asc-app-create-ui": {"source": "rorkai/app-store-connect-cli-skills", "ref": "main"},
    "asc-xcode-build": {"source": "rorkai/app-store-connect-cli-skills"}
  },
  "dismissed": {"findSkillsPrompt": true},
  "futureTopLevel": [1, 2, 3]
}`)
	data, err := buildUntrackedSkillLock(original, expectedSkillNames)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(document["skills"], &entries); err != nil {
		t.Fatal(err)
	}
	if _, ok := entries["asc-app-create-ui"]; ok {
		t.Fatal("ASC entry remained tracked after incomplete rollback")
	}
	if _, ok := entries["asc-xcode-build"]; ok {
		t.Fatal("ASC entry remained tracked after incomplete rollback")
	}
	if _, ok := entries["unrelated"]; !ok {
		t.Fatal("unrelated entry was removed")
	}
	if _, ok := document["dismissed"]; !ok {
		t.Fatal("dismissed metadata was removed")
	}
	if _, ok := document["futureTopLevel"]; !ok {
		t.Fatal("future top-level metadata was removed")
	}
}

func TestInstallSkillsRestoresExactLockBytesAfterSuccessfulRollback(t *testing.T) {
	home := installFixture(t, func(paths gitIsolationPaths) error {
		populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
		return nil
	})
	lockPath := filepath.Join(home, ".agents", skillLockName)
	original := []byte("{\n  \"version\": 3,\n  \"skills\": {\"unrelated\": {\"opaque\": [3,2,1]}}\n}\n")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, original, 0o600); err != nil {
		t.Fatal(err)
	}

	failed := false
	renameWithinRoot = func(root *os.Root, oldName, newName string) error {
		if !failed && strings.Contains(oldName, ".asc-stage-") && filepath.Base(oldName) == expectedSkillNames[2] {
			failed = true
			return errors.New("injected install failure")
		}
		return root.Rename(oldName, newName)
	}
	if err := runInstallCommand(t); err == nil {
		t.Fatal("expected injected installation failure")
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, original) {
		t.Fatalf("lock rollback changed original bytes:\n%s\nwant:\n%s", data, original)
	}
}

func TestInstallSkillsUsesXDGStateSkillsLock(t *testing.T) {
	xdgState := t.TempDir()
	home := installFixture(t, func(paths gitIsolationPaths) error {
		populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
		return nil
	})
	t.Setenv("XDG_STATE_HOME", xdgState)
	if err := runInstallCommand(t); err != nil {
		t.Fatalf("install: %v", err)
	}
	assertPinnedLock(t, filepath.Join(xdgState, "skills", skillLockName), "2026-07-30T12:00:00Z")
	if _, err := os.Stat(filepath.Join(home, ".agents", skillLockName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("HOME lock should not be written when XDG_STATE_HOME is set: %v", err)
	}
}

func TestInstallSkillsCleanupNeverFollowsSwappedWorkspaceSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not permit replacing an open directory; the retained handle is already protective")
	}
	victim := t.TempDir()
	sentinel := filepath.Join(victim, "keep-me")
	if err := os.WriteFile(sentinel, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}

	home := installFixture(t, func(paths gitIsolationPaths) error {
		if err := os.RemoveAll(paths.workingDir); err != nil {
			return err
		}
		if err := os.Symlink(victim, paths.workingDir); err != nil {
			return err
		}
		return errors.New("injected checkout failure after workspace swap")
	})

	err := runInstallCommand(t)
	if err == nil {
		t.Fatal("expected injected checkout failure")
	}
	if data, readErr := os.ReadFile(sentinel); readErr != nil || string(data) != "unrelated" {
		t.Fatalf("cleanup followed swapped workspace symlink: data=%q err=%v", data, readErr)
	}
	assertNoInstallWorkspace(t, filepath.Join(home, ".agents"))
}

func TestInstallSkillsIgnoresRelativeAndSymlinkedTMPDIR(t *testing.T) {
	cases := []struct {
		name  string
		value func(*testing.T) string
	}{
		{name: "relative", value: func(*testing.T) string { return "relative-temp" }},
		{name: "symlink", value: func(t *testing.T) string {
			target := t.TempDir()
			link := filepath.Join(t.TempDir(), "tmp-link")
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlink creation is not permitted: %v", err)
			}
			return link
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := tc.value(t)
			installFixture(t, func(paths gitIsolationPaths) error {
				populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
				return nil
			})
			t.Setenv("TMPDIR", tmpDir)
			if err := runInstallCommand(t); err != nil {
				t.Fatalf("install with %s TMPDIR: %v", tc.name, err)
			}
		})
	}
}

func TestInstallSkillsAppliesASCTimeoutToGit(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1m")
	preserveInstallerGlobals(t)
	home := t.TempDir()
	userHomeDirectory = func() (string, error) { return home, nil }
	lookupExecutable = func(string) (string, error) { return "/test/bin/git", nil }
	checkoutSkillsSource = func(ctx context.Context, _ string, paths gitIsolationPaths) error {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected install context to have a deadline")
		}
		remaining := time.Until(deadline)
		if remaining <= 0 || remaining > time.Minute {
			t.Fatalf("expected ASC_TIMEOUT deadline within 1m, got %s", remaining)
		}
		populatePinnedSkillsFixture(t, paths.sourceDir, expectedSkillNames)
		return nil
	}

	if err := runInstallCommand(t); err != nil {
		t.Fatalf("install: %v", err)
	}
}

func TestInstallSkillsFailsWhenGitMissing(t *testing.T) {
	preserveInstallerGlobals(t)
	lookupExecutable = func(string) (string, error) { return "", errors.New("not found") }

	err := runInstallCommand(t)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, errGitNotFound) {
		t.Fatalf("error = %v, want errGitNotFound", err)
	}
	if !strings.Contains(err.Error(), skillsSourceCommit) {
		t.Fatalf("error does not name pinned commit: %v", err)
	}
}

func TestCheckoutPinnedSkillsUsesOnlyExactSourceAndIsolatedGitState(t *testing.T) {
	preserveInstallerGlobals(t)
	base := t.TempDir()
	paths := gitIsolationPaths{
		home:       filepath.Join(base, "home"),
		globalCfg:  filepath.Join(base, "home", "gitconfig"),
		hooks:      filepath.Join(base, "hooks"),
		template:   filepath.Join(base, "template"),
		xdgConfig:  filepath.Join(base, "home", "config"),
		sourceDir:  filepath.Join(base, "source"),
		workingDir: base,
	}
	for _, dir := range []string{paths.home, paths.hooks, paths.template, paths.xdgConfig, paths.sourceDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}

	t.Setenv("GIT_CONFIG_PARAMETERS", "'alias.checkout=!touch /tmp/pwned'")
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.hooksPath")
	t.Setenv("GIT_CONFIG_VALUE_0", "/tmp/hooks")
	t.Setenv("GIT_DIR", "/tmp/attacker")
	t.Setenv("GIT_ASKPASS", "/tmp/askpass")
	t.Setenv("GIT_SSL_CAINFO", "/corporate/ca.pem")
	t.Setenv("GIT_SSL_CAPATH", "/corporate/certs")
	t.Setenv("HOME", "/tmp/caller-home")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/caller-xdg")

	type call struct {
		dir  string
		env  []string
		name string
		args []string
	}
	var calls []call
	runGitCommand = func(_ context.Context, dir string, env []string, name string, args ...string) error {
		calls = append(calls, call{dir: dir, env: append([]string(nil), env...), name: name, args: append([]string(nil), args...)})
		return nil
	}

	if err := checkoutPinnedSkills(context.Background(), "/test/bin/git", paths); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("git calls = %d, want 3", len(calls))
	}
	for _, call := range calls {
		if call.dir != paths.workingDir || call.name != "/test/bin/git" {
			t.Fatalf("call = %#v", call)
		}
		assertSafeGitEnvironment(t, call.env, paths)
		joined := strings.Join(call.args, "\x00")
		for _, required := range []string{
			"credential.helper=",
			"core.hooksPath=" + paths.hooks,
			"init.templateDir=" + paths.template,
			"protocol.file.allow=never",
			"protocol.ext.allow=never",
			"protocol.http.allow=never",
		} {
			if !strings.Contains(joined, required) {
				t.Fatalf("git args %q do not include %q", call.args, required)
			}
		}
	}
	fetch := strings.Join(calls[1].args, "\x00")
	if !strings.Contains(fetch, skillsSourceRepositoryURL) || !strings.Contains(fetch, skillsSourceCommit) {
		t.Fatalf("fetch args do not bind repository and commit: %q", calls[1].args)
	}
	for _, mutable := range []string{"HEAD", "main", "master", "@latest"} {
		if strings.Contains(fetch, "\x00"+mutable+"\x00") {
			t.Fatalf("fetch uses mutable ref %q: %q", mutable, calls[1].args)
		}
	}
}

func TestValidatePinnedSkillsRejectsSymlinks(t *testing.T) {
	sourceDir := t.TempDir()
	populatePinnedSkillsFixture(t, sourceDir, expectedSkillNames)
	link := filepath.Join(sourceDir, "skills", expectedSkillNames[0], "unsafe")
	if err := os.Symlink(filepath.Join(sourceDir, "outside"), link); err != nil {
		t.Skipf("symlink creation is not permitted: %v", err)
	}
	root, err := os.OpenRoot(filepath.Join(sourceDir, "skills"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	if _, err := validatePinnedSkills(root); err == nil || !errors.Is(err, errIncompleteSkillSet) {
		t.Fatalf("error = %v, want rejected symlink", err)
	}
}

func TestPinnedSkillsDocumentationMatchesDirectInstaller(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "..", "README.md"),
		filepath.Join("..", "..", "..", "website", "src", "content", "docs", "installation.mdx"),
		filepath.Join("..", "..", "..", "website", "src", "content", "docs", "index.mdx"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if strings.Contains(content, "npx --yes skills") || strings.Contains(content, "needs Node.js") {
			t.Errorf("%s still documents executable npm installer code", path)
		}
		if !strings.Contains(content, skillsSourceCommit) {
			t.Errorf("%s does not mention the pinned skills commit %q", path, skillsSourceCommit)
		}
		if !strings.Contains(content, fmt.Sprintf("%d skills", expectedSkillsCount)) {
			t.Errorf("%s does not mention verification of all %d skills", path, expectedSkillsCount)
		}
	}
}

func assertSafeGitEnvironment(t *testing.T, env []string, paths gitIsolationPaths) {
	t.Helper()
	values := make(map[string]string)
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[strings.ToUpper(key)] = value
		}
	}
	for _, key := range []string{
		"GIT_CONFIG_PARAMETERS",
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0",
		"GIT_CONFIG_VALUE_0",
		"GIT_DIR",
		"GIT_ASKPASS",
	} {
		if _, exists := values[key]; exists {
			t.Fatalf("unsafe %s survived git environment isolation", key)
		}
	}
	want := map[string]string{
		"HOME":                paths.home,
		"USERPROFILE":         paths.home,
		"XDG_CONFIG_HOME":     paths.xdgConfig,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_CONFIG_GLOBAL":   paths.globalCfg,
		"GIT_TERMINAL_PROMPT": "0",
		"GCM_INTERACTIVE":     "never",
		"GIT_SSL_CAINFO":      "/corporate/ca.pem",
		"GIT_SSL_CAPATH":      "/corporate/certs",
	}
	for key, value := range want {
		if values[key] != value {
			t.Fatalf("%s = %q, want %q", key, values[key], value)
		}
	}
}

func assertNoInstallWorkspace(t *testing.T, agentsDir string) {
	t.Helper()
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".asc-install-skills-") {
			t.Fatalf("install workspace was not cleaned up: %s", entry.Name())
		}
	}
}

func assertPinnedLock(t *testing.T, path string, preservedInstalledAt string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pinned lock %s: %v", path, err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("parse pinned lock: %v", err)
	}
	var version int
	if err := json.Unmarshal(document["version"], &version); err != nil || version != 3 {
		t.Fatalf("lock version = %d err=%v", version, err)
	}
	var entries map[string]map[string]json.RawMessage
	if err := json.Unmarshal(document["skills"], &entries); err != nil {
		t.Fatalf("parse lock entries: %v", err)
	}
	for _, name := range expectedSkillNames {
		entry, ok := entries[name]
		if !ok {
			t.Fatalf("lock is missing %s", name)
		}
		want := map[string]string{
			"source":          "rorkai/app-store-connect-cli-skills",
			"sourceType":      "github",
			"sourceUrl":       skillsSourceRepositoryURL,
			"ref":             skillsSourceCommit,
			"skillPath":       filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")),
			"skillFolderHash": expectedSkillTreeHashes[name],
			"updatedAt":       "2026-07-30T12:00:00Z",
		}
		for key, wantValue := range want {
			var got string
			if err := json.Unmarshal(entry[key], &got); err != nil || got != wantValue {
				t.Fatalf("%s.%s = %q err=%v, want %q", name, key, got, err, wantValue)
			}
		}
		var installedAt string
		if err := json.Unmarshal(entry["installedAt"], &installedAt); err != nil {
			t.Fatalf("%s.installedAt: %v", name, err)
		}
		if name == "asc-app-create-ui" && preservedInstalledAt != "" {
			if installedAt != preservedInstalledAt {
				t.Fatalf("%s installedAt = %q, want preserved %q", name, installedAt, preservedInstalledAt)
			}
		} else if installedAt == "" {
			t.Fatalf("%s installedAt is empty", name)
		}
	}
	if unrelated, ok := entries["user-owned-skill"]; ok {
		var source string
		if err := json.Unmarshal(unrelated["source"], &source); err != nil || source != "local" {
			t.Fatalf("unrelated skill metadata changed: source=%q err=%v", source, err)
		}
	}
	if raw, ok := document["dismissed"]; ok && !strings.Contains(string(raw), "findSkillsPrompt") {
		t.Fatalf("dismissed metadata changed: %s", raw)
	}
	if raw, ok := document["futureTopLevel"]; ok && !strings.Contains(string(raw), `"keep": 1`) && !strings.Contains(string(raw), `"keep":1`) {
		t.Fatalf("future top-level metadata changed: %s", raw)
	}
}
