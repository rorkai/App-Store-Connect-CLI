package install

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	skillsSourceRepositoryURL = "https://github.com/rorkai/app-store-connect-cli-skills.git"
	skillsSourceCommit        = "f52c4f04323bb2dfb21ca8be82e6494e9cd0b4d8"
	expectedSkillsCount       = 25
)

var expectedSkillNames = []string{
	"asc-ad-hoc-distribution",
	"asc-analytics-reports",
	"asc-app-create-ui",
	"asc-apple-ads",
	"asc-aso-audit",
	"asc-build-lifecycle",
	"asc-cli-usage",
	"asc-crash-triage",
	"asc-id-resolver",
	"asc-localize-metadata",
	"asc-metadata-sync",
	"asc-notarization",
	"asc-ppp-pricing",
	"asc-release-flow",
	"asc-revenuecat-catalog-sync",
	"asc-screenshot-resize",
	"asc-shots-pipeline",
	"asc-signing-setup",
	"asc-submission-health",
	"asc-subscription-localization",
	"asc-testflight-orchestration",
	"asc-wall-submit",
	"asc-whats-new-writer",
	"asc-workflow",
	"asc-xcode-build",
}

// expectedSkillTreeHashes are the Git tree object IDs at skillsSourceCommit.
// The external skills CLI uses these hashes for its version 3 lock schema.
var expectedSkillTreeHashes = map[string]string{
	"asc-ad-hoc-distribution":       "1971fddbfc899aa9c9fdbae966ac1ad910c782f8",
	"asc-analytics-reports":         "02e6f26daa9bbe072d0a16b0f66174577dab3786",
	"asc-app-create-ui":             "f0bd798edb26e555c2d1c9a03170839339a9617f",
	"asc-apple-ads":                 "f880071a54da09438dc263e1e713eec82e30a076",
	"asc-aso-audit":                 "82236048a284d1539e8f3f9b3ab674027c0593d6",
	"asc-build-lifecycle":           "33e0a5aa1e7e44614e08c6cfc7be0d90d0c50beb",
	"asc-cli-usage":                 "bc2e86aff174beffc6822f1ff418fde96ecb8406",
	"asc-crash-triage":              "ddd74903f7d5ca6a3ab2adca3a5fe5d16bf91a2c",
	"asc-id-resolver":               "622819676125ec107c2d2080f53c5e8a49a4da66",
	"asc-localize-metadata":         "e62586a64adca119eba2db2b213f53106565a1ad",
	"asc-metadata-sync":             "579a4bf7b7683f394b0f6fbe62e9c44e2a4c20c3",
	"asc-notarization":              "36d8850790528df621beafcd6e511a6d24d685c0",
	"asc-ppp-pricing":               "db3ee0d35c88e71550eeb7462dd623d48f2b97c4",
	"asc-release-flow":              "dc6aa81e498b821d07fa1a56e25ddb6d693c0ba4",
	"asc-revenuecat-catalog-sync":   "85a89894315657d274695157ea7614d8ed2197d0",
	"asc-screenshot-resize":         "5ac1818b9083168aa7156d22f422bef8f4a8255f",
	"asc-shots-pipeline":            "abe751dc2dcbf63518be4337e482485498eb723d",
	"asc-signing-setup":             "26eeab24b875331b2c527cba32928e1c1bbcf44b",
	"asc-submission-health":         "af9d35eb1418ab12d276c605b48098f330201610",
	"asc-subscription-localization": "02b30fc38f23e8d5ce0107f8548096d9ed4902ce",
	"asc-testflight-orchestration":  "a7a62b24e77df5fe9f2297d0bd1cebf409586068",
	"asc-wall-submit":               "80e3a2f914a7f07aa7641d523eadab226f869f41",
	"asc-whats-new-writer":          "95607774f0b4f620cc4a02a33bc4a3281845a76b",
	"asc-workflow":                  "06bd9199031d1b18d7c183adc8685540458351b3",
	"asc-xcode-build":               "c3c4261279a72d0e4d3feeeb837479b4fa5c9c7f",
}

var (
	lookupExecutable      = exec.LookPath
	runGitCommand         = defaultRunGitCommand
	checkoutSkillsSource  = checkoutPinnedSkills
	userHomeDirectory     = os.UserHomeDir
	renameWithinRoot      = func(root *os.Root, oldName, newName string) error { return root.Rename(oldName, newName) }
	currentTime           = time.Now
	errGitNotFound        = errors.New("git not found")
	errIncompleteSkillSet = errors.New("pinned checkout does not contain the complete asc skill pack")
)

const (
	installLockName = ".asc-install-skills.lock"
	skillLockName   = ".skill-lock.json"
)

// InstallSkillsCommand returns the top-level `install-skills` command.
func InstallSkillsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("install-skills", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "install-skills",
		ShortUsage: "asc install-skills",
		ShortHelp:  "Install the asc skill pack globally for App Store Connect workflows.",
		LongHelp: fmt.Sprintf(`Install the asc skill pack globally for App Store Connect workflows.

Checks out the reviewed skills commit
%s
and installs its %d skills directly into the standard global agent-skills
directory. No Node.js package or installer is executed.

The source pin lives in the asc source, so upgrading the skill pack is an
explicit reviewed change rather than an automatic update.

Requires git.

Examples:
  asc install-skills`, skillsSourceCommit, expectedSkillsCount),
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if err := installSkills(ctx); err != nil {
				return fmt.Errorf("install skills: %w", err)
			}
			return nil
		},
	}
}

func installSkills(ctx context.Context) (resultErr error) {
	ctx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	gitPath, err := lookupExecutable("git")
	if err != nil {
		return fmt.Errorf("%w; git is required to check out the pinned skills commit %s", errGitNotFound, skillsSourceCommit)
	}
	gitPath, err = filepath.Abs(gitPath)
	if err != nil {
		return fmt.Errorf("resolve the git executable path: %w", err)
	}

	homeDir, err := userHomeDirectory()
	if err != nil {
		return fmt.Errorf("resolve the user home directory: %w", err)
	}
	if strings.TrimSpace(homeDir) == "" {
		return errors.New("resolve the user home directory: path is empty")
	}
	homeDir, err = filepath.Abs(homeDir)
	if err != nil {
		return fmt.Errorf("resolve the absolute user home directory: %w", err)
	}

	agentsDir := filepath.Join(homeDir, ".agents")
	if err := os.MkdirAll(agentsDir, 0o700); err != nil {
		return fmt.Errorf("create the global agent directory: %w", err)
	}
	agentsRoot, err := os.OpenRoot(agentsDir)
	if err != nil {
		return fmt.Errorf("open the global agent directory: %w", err)
	}
	defer agentsRoot.Close()

	installLock, err := acquireInstallLock(agentsRoot, filepath.Join(agentsDir, installLockName))
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := installLock.release(); releaseErr != nil {
			resultErr = errors.Join(resultErr, releaseErr)
		}
	}()

	workspaceName, workspaceRoot, err := createPrivateDirectory(agentsRoot, ".asc-install-skills-")
	if err != nil {
		return fmt.Errorf("create a private install workspace: %w", err)
	}
	defer func() {
		_ = workspaceRoot.Close()
		// Removal is relative to the retained .agents handle. If the workspace
		// path is swapped for a symlink, Root.RemoveAll removes the link rather
		// than traversing to and deleting its target.
		_ = agentsRoot.RemoveAll(workspaceName)
	}()

	if err := workspaceRoot.Mkdir("source", 0o700); err != nil {
		return fmt.Errorf("create the pinned checkout directory: %w", err)
	}
	if err := workspaceRoot.Mkdir("git-home", 0o700); err != nil {
		return fmt.Errorf("create the isolated git home directory: %w", err)
	}
	if err := workspaceRoot.Mkdir(filepath.Join("git-home", "config"), 0o700); err != nil {
		return fmt.Errorf("create the isolated git config directory: %w", err)
	}
	if err := workspaceRoot.Mkdir("git-hooks", 0o700); err != nil {
		return fmt.Errorf("create the empty git hooks directory: %w", err)
	}
	if err := workspaceRoot.Mkdir("git-template", 0o700); err != nil {
		return fmt.Errorf("create the empty git template directory: %w", err)
	}
	if err := workspaceRoot.WriteFile(filepath.Join("git-home", "gitconfig"), nil, 0o600); err != nil {
		return fmt.Errorf("create the isolated git configuration: %w", err)
	}

	workspaceDir := filepath.Join(agentsDir, workspaceName)
	sourceDir := filepath.Join(workspaceDir, "source")
	gitIsolation := gitIsolationPaths{
		home:       filepath.Join(workspaceDir, "git-home"),
		globalCfg:  filepath.Join(workspaceDir, "git-home", "gitconfig"),
		hooks:      filepath.Join(workspaceDir, "git-hooks"),
		template:   filepath.Join(workspaceDir, "git-template"),
		xdgConfig:  filepath.Join(workspaceDir, "git-home", "config"),
		sourceDir:  sourceDir,
		workingDir: workspaceDir,
	}
	if err := checkoutSkillsSource(ctx, gitPath, gitIsolation); err != nil {
		return err
	}

	sourceSkills, err := os.OpenRoot(filepath.Join(sourceDir, "skills"))
	if err != nil {
		return fmt.Errorf("open skills in pinned checkout: %w", err)
	}
	defer sourceSkills.Close()

	names, err := validatePinnedSkills(sourceSkills)
	if err != nil {
		return err
	}

	skillsDir := filepath.Join(agentsDir, "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create the global skills directory: %w", err)
	}
	skillsRoot, err := os.OpenRoot(skillsDir)
	if err != nil {
		return fmt.Errorf("open the global skills directory: %w", err)
	}
	defer skillsRoot.Close()

	lockState, err := preparePinnedSkillLock(homeDir)
	if err != nil {
		return err
	}
	defer lockState.close()
	if err := lockState.bind(names); err != nil {
		return err
	}

	if err := installSkillPack(sourceSkills, skillsRoot, names); err != nil {
		var retainedBackup *retainedSkillBackupError
		if errors.As(err, &retainedBackup) {
			if untrackErr := lockState.untrack(names); untrackErr != nil {
				return errors.Join(err, fmt.Errorf("remove inconsistent asc entries from the skill lock: %w", untrackErr))
			}
		} else if !mustKeepPinnedSkillLock(err) {
			if restoreErr := lockState.restore(); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore previous skill lock: %w", restoreErr))
			}
		}
		return err
	}
	fmt.Fprintf(os.Stdout, "Installed %d asc skills from reviewed commit %s.\n", len(names), skillsSourceCommit)
	return nil
}

type activeInstallLock struct {
	file *os.File
	path string
}

func acquireInstallLock(root *os.Root, path string) (*activeInstallLock, error) {
	file, err := secureopen.OpenLockFileNoFollowInRoot(root, installLockName, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create cross-process install lock %s: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, fmt.Errorf("cross-process install lock %s must be a regular file: %w", path, err)
	}
	if err := lockInstallFile(file); err != nil {
		_ = file.Close()
		if errors.Is(err, errInstallLockHeld) {
			return nil, fmt.Errorf("another asc skill installation is already running (lock %s): %w", path, err)
		}
		return nil, fmt.Errorf("acquire cross-process install lock %s: %w", path, err)
	}
	return &activeInstallLock{file: file, path: path}, nil
}

func (lock *activeInstallLock) release() error {
	unlockErr := unlockInstallFile(lock.file)
	closeErr := lock.file.Close()
	if unlockErr != nil || closeErr != nil {
		return fmt.Errorf("release install lock %s: %w", lock.path, errors.Join(unlockErr, closeErr))
	}
	return nil
}

type pinnedSkillLock struct {
	root         *os.Root
	path         string
	original     []byte
	originalMode os.FileMode
	existed      bool
	bound        bool
}

func preparePinnedSkillLock(homeDir string) (*pinnedSkillLock, error) {
	lockDir := filepath.Join(homeDir, ".agents")
	if stateHome := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); stateHome != "" {
		absoluteStateHome, err := filepath.Abs(stateHome)
		if err != nil {
			return nil, fmt.Errorf("resolve XDG_STATE_HOME: %w", err)
		}
		lockDir = filepath.Join(absoluteStateHome, "skills")
	}
	if err := os.MkdirAll(lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create skills lock directory: %w", err)
	}
	root, err := os.OpenRoot(lockDir)
	if err != nil {
		return nil, fmt.Errorf("open skills lock directory: %w", err)
	}
	state := &pinnedSkillLock{
		root:         root,
		path:         filepath.Join(lockDir, skillLockName),
		originalMode: 0o600,
	}
	info, err := root.Lstat(skillLockName)
	if errors.Is(err, fs.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("inspect skills lock %s: %w", state.path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		root.Close()
		return nil, fmt.Errorf("skills lock %s must be a regular file, not a symlink or special file", state.path)
	}
	state.original, err = root.ReadFile(skillLockName)
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("read skills lock %s: %w", state.path, err)
	}
	state.originalMode = info.Mode().Perm()
	state.existed = true
	return state, nil
}

func (state *pinnedSkillLock) close() {
	if state != nil && state.root != nil {
		_ = state.root.Close()
	}
}

func (state *pinnedSkillLock) bind(names []string) error {
	data, err := buildPinnedSkillLock(state.original, names, currentTime().UTC())
	if err != nil {
		return fmt.Errorf("prepare pinned skills lock %s: %w", state.path, err)
	}
	if err := atomicWriteRootFile(state.root, skillLockName, data, state.originalMode); err != nil {
		return fmt.Errorf("write pinned skills lock %s: %w", state.path, err)
	}
	state.bound = true
	return nil
}

func (state *pinnedSkillLock) restore() error {
	if !state.bound {
		return nil
	}
	if !state.existed {
		if err := state.root.Remove(skillLockName); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		state.bound = false
		return nil
	}
	if err := atomicWriteRootFile(state.root, skillLockName, state.original, state.originalMode); err != nil {
		return err
	}
	state.bound = false
	return nil
}

func (state *pinnedSkillLock) untrack(names []string) error {
	if !state.existed {
		if err := state.root.Remove(skillLockName); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		state.bound = false
		return nil
	}
	data, err := buildUntrackedSkillLock(state.original, names)
	if err != nil {
		return err
	}
	if err := atomicWriteRootFile(state.root, skillLockName, data, state.originalMode); err != nil {
		return err
	}
	state.bound = false
	return nil
}

func buildPinnedSkillLock(original []byte, names []string, now time.Time) ([]byte, error) {
	document := make(map[string]json.RawMessage)
	if len(strings.TrimSpace(string(original))) != 0 {
		if err := json.Unmarshal(original, &document); err != nil {
			return nil, fmt.Errorf("invalid version 3 lock JSON: %w", err)
		}
	}

	if rawVersion, ok := document["version"]; ok {
		var version int
		if err := json.Unmarshal(rawVersion, &version); err != nil || version != 3 {
			return nil, fmt.Errorf("unsupported skills lock version %s; expected version 3", strings.TrimSpace(string(rawVersion)))
		}
	}
	document["version"] = json.RawMessage("3")

	entries := make(map[string]json.RawMessage)
	if rawSkills, ok := document["skills"]; ok {
		if err := json.Unmarshal(rawSkills, &entries); err != nil {
			return nil, fmt.Errorf("invalid skills object: %w", err)
		}
	}
	timestamp := now.Format(time.RFC3339Nano)
	for _, name := range names {
		entry := make(map[string]json.RawMessage)
		if rawEntry, ok := entries[name]; ok {
			if err := json.Unmarshal(rawEntry, &entry); err != nil {
				return nil, fmt.Errorf("invalid lock entry for %s: %w", name, err)
			}
		}
		installedAt := timestamp
		if rawInstalledAt, ok := entry["installedAt"]; ok {
			var existing string
			if json.Unmarshal(rawInstalledAt, &existing) == nil && strings.TrimSpace(existing) != "" {
				installedAt = existing
			}
		}
		fields := map[string]string{
			"source":          "rorkai/app-store-connect-cli-skills",
			"sourceType":      "github",
			"sourceUrl":       skillsSourceRepositoryURL,
			"ref":             skillsSourceCommit,
			"skillPath":       filepath.ToSlash(filepath.Join("skills", name, "SKILL.md")),
			"skillFolderHash": expectedSkillTreeHashes[name],
			"installedAt":     installedAt,
			"updatedAt":       timestamp,
		}
		for key, value := range fields {
			encoded, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			entry[key] = encoded
		}
		encodedEntry, err := json.Marshal(entry)
		if err != nil {
			return nil, err
		}
		entries[name] = encodedEntry
	}
	encodedSkills, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	document["skills"] = encodedSkills
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildUntrackedSkillLock(original []byte, names []string) ([]byte, error) {
	document := make(map[string]json.RawMessage)
	if err := json.Unmarshal(original, &document); err != nil {
		return nil, fmt.Errorf("invalid version 3 lock JSON: %w", err)
	}
	rawSkills, ok := document["skills"]
	if !ok {
		return append(append([]byte(nil), original...), '\n'), nil
	}
	entries := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rawSkills, &entries); err != nil {
		return nil, fmt.Errorf("invalid skills object: %w", err)
	}
	for _, name := range names {
		delete(entries, name)
	}
	encodedSkills, err := json.Marshal(entries)
	if err != nil {
		return nil, err
	}
	document["skills"] = encodedSkills
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func atomicWriteRootFile(root *os.Root, name string, data []byte, mode os.FileMode) error {
	file, temporaryName, err := secureopen.CreateTempNoFollowInRoot(root, ".", ".asc-skill-lock-*.tmp", mode)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		_ = file.Close()
		if cleanup {
			_ = root.Remove(temporaryName)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := root.Rename(temporaryName, name); err != nil {
		return err
	}
	cleanup = false
	return nil
}

type gitIsolationPaths struct {
	home       string
	globalCfg  string
	hooks      string
	template   string
	xdgConfig  string
	sourceDir  string
	workingDir string
}

// checkoutPinnedSkills fetches exactly skillsSourceCommit into an empty
// directory. Git configuration, hooks, credential helpers, filters, alternate
// object stores, and askpass helpers are isolated so caller- or
// repository-controlled configuration cannot execute code during checkout.
func checkoutPinnedSkills(ctx context.Context, gitPath string, paths gitIsolationPaths) error {
	common := []string{
		"-c", "credential.helper=",
		"-c", "core.hooksPath=" + paths.hooks,
		"-c", "init.templateDir=" + paths.template,
		"-c", "core.fsmonitor=false",
		"-c", "protocol.file.allow=never",
		"-c", "protocol.ext.allow=never",
		"-c", "protocol.ssh.allow=never",
		"-c", "protocol.git.allow=never",
		"-c", "protocol.http.allow=never",
		"-c", "protocol.https.allow=always",
	}
	steps := [][]string{
		append(append([]string{}, common...), "-C", paths.sourceDir, "init", "--quiet"),
		append(append([]string{}, common...), "-C", paths.sourceDir, "fetch", "--quiet", "--depth=1", "--no-tags", skillsSourceRepositoryURL, skillsSourceCommit),
		append(append([]string{}, common...), "-C", paths.sourceDir, "checkout", "--quiet", "--detach", skillsSourceCommit),
	}
	env := isolatedGitEnvironment(os.Environ(), paths)

	for _, args := range steps {
		if err := runGitCommand(ctx, paths.workingDir, env, gitPath, args...); err != nil {
			return fmt.Errorf("failed to check out pinned skills commit %s from %s: %w", skillsSourceCommit, skillsSourceRepositoryURL, err)
		}
	}
	return nil
}

func isolatedGitEnvironment(base []string, paths gitIsolationPaths) []string {
	env := make([]string, 0, len(base)+8)
	safeGitEnvironment := make(map[string]string)
	for _, entry := range base {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(key)
		switch {
		case strings.HasPrefix(upper, "GIT_"):
			// Corporate Git installations commonly require a private CA. These
			// two values configure certificate lookup but cannot select a Git
			// directory, helper, hook, filter, or executable.
			if upper == "GIT_SSL_CAINFO" || upper == "GIT_SSL_CAPATH" {
				safeGitEnvironment[upper] = value
			}
			continue
		case upper == "HOME", upper == "USERPROFILE", upper == "HOMEDRIVE", upper == "HOMEPATH":
			continue
		case upper == "XDG_CONFIG_HOME", upper == "SSH_ASKPASS", upper == "GCM_INTERACTIVE":
			continue
		}
		env = append(env, entry)
	}
	for _, key := range []string{"GIT_SSL_CAINFO", "GIT_SSL_CAPATH"} {
		if value, ok := safeGitEnvironment[key]; ok {
			env = append(env, key+"="+value)
		}
	}
	env = append(
		env,
		"HOME="+paths.home,
		"USERPROFILE="+paths.home,
		"XDG_CONFIG_HOME="+paths.xdgConfig,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_ATTR_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+paths.globalCfg,
		"GIT_TERMINAL_PROMPT=0",
		"GCM_INTERACTIVE=never",
	)
	return env
}

func defaultRunGitCommand(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	return cmd.Run()
}

func validatePinnedSkills(root *os.Root) ([]string, error) {
	entries, err := readRootDir(root, ".")
	if err != nil {
		return nil, fmt.Errorf("read skills in pinned checkout: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: unexpected entry %q", errIncompleteSkillSet, entry.Name())
		}
		if _, err := root.Stat(filepath.Join(entry.Name(), "SKILL.md")); err != nil {
			return nil, fmt.Errorf("%w: %s has no readable SKILL.md", errIncompleteSkillSet, entry.Name())
		}
		if _, err := snapshotTree(root, entry.Name()); err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errIncompleteSkillSet, entry.Name(), err)
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	want := append([]string(nil), expectedSkillNames...)
	sort.Strings(want)
	if len(names) != expectedSkillsCount || !reflect.DeepEqual(names, want) {
		return nil, fmt.Errorf("%w: found %d skills (%s), want exactly %d (%s)",
			errIncompleteSkillSet, len(names), strings.Join(names, ", "), expectedSkillsCount, strings.Join(want, ", "))
	}
	return names, nil
}

func installSkillPack(source, destination *os.Root, names []string) error {
	stageName, stageRoot, err := createPrivateDirectory(destination, ".asc-stage-")
	if err != nil {
		return fmt.Errorf("create skill staging directory: %w", err)
	}
	defer func() {
		_ = stageRoot.Close()
		_ = destination.RemoveAll(stageName)
	}()

	backupName, backupRoot, err := createPrivateDirectory(destination, ".asc-backup-")
	if err != nil {
		return fmt.Errorf("create skill backup directory: %w", err)
	}
	preserveBackup := false
	defer func() {
		_ = backupRoot.Close()
		if !preserveBackup {
			_ = destination.RemoveAll(backupName)
		}
	}()

	for _, name := range names {
		if err := copyTree(source, name, stageRoot, name); err != nil {
			return fmt.Errorf("stage skill %s: %w", name, err)
		}
		sourceSnapshot, err := snapshotTree(source, name)
		if err != nil {
			return fmt.Errorf("verify source skill %s: %w", name, err)
		}
		stagedSnapshot, err := snapshotTree(stageRoot, name)
		if err != nil {
			return fmt.Errorf("verify staged skill %s: %w", name, err)
		}
		if !reflect.DeepEqual(sourceSnapshot, stagedSnapshot) {
			return fmt.Errorf("verify staged skill %s: copied files do not match the pinned checkout", name)
		}
	}

	type replacement struct {
		name        string
		hadExisting bool
		installed   bool
	}
	replaced := make([]replacement, 0, len(names))

	rollback := func(cause error) error {
		var rollbackErrs []error
		for i := len(replaced) - 1; i >= 0; i-- {
			item := replaced[i]
			if item.installed {
				if err := destination.RemoveAll(item.name); err != nil {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("remove replacement %s: %w", item.name, err))
				}
			}
			if item.hadExisting {
				if err := renameWithinRoot(destination, filepath.Join(backupName, item.name), item.name); err != nil {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("restore previous %s: %w", item.name, err))
				}
			}
		}
		if len(rollbackErrs) == 0 {
			return cause
		}
		preserveBackup = true
		backupPath := filepath.Join(destination.Name(), backupName)
		return &retainedSkillBackupError{
			cause: errors.Join(cause, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrs...))),
			path:  backupPath,
		}
	}

	for _, name := range names {
		item := replacement{name: name}
		if _, err := destination.Lstat(name); err == nil {
			if err := renameWithinRoot(destination, name, filepath.Join(backupName, name)); err != nil {
				return rollback(fmt.Errorf("back up existing skill %s: %w", name, err))
			}
			item.hadExisting = true
		} else if !errors.Is(err, fs.ErrNotExist) {
			return rollback(fmt.Errorf("inspect existing skill %s: %w", name, err))
		}
		replaced = append(replaced, item)

		if err := renameWithinRoot(destination, filepath.Join(stageName, name), name); err != nil {
			return rollback(fmt.Errorf("install skill %s: %w", name, err))
		}
		replaced[len(replaced)-1].installed = true
	}

	for _, name := range names {
		sourceSnapshot, err := snapshotTree(source, name)
		if err != nil {
			return rollback(fmt.Errorf("verify source skill %s after install: %w", name, err))
		}
		installedSnapshot, err := snapshotTree(destination, name)
		if err != nil {
			return rollback(fmt.Errorf("verify installed skill %s: %w", name, err))
		}
		if !reflect.DeepEqual(sourceSnapshot, installedSnapshot) {
			return rollback(fmt.Errorf("verify installed skill %s: installed files do not match the pinned checkout", name))
		}
	}

	if err := backupRoot.Close(); err != nil {
		return &committedSkillInstallError{cause: fmt.Errorf("close previous asc skill backup directory: %w", err)}
	}
	if err := destination.RemoveAll(backupName); err != nil {
		preserveBackup = true
		return &committedSkillInstallError{cause: fmt.Errorf("remove previous asc skill backups at %s: %w", filepath.Join(destination.Name(), backupName), err)}
	}
	if err := stageRoot.Close(); err != nil {
		return &committedSkillInstallError{cause: fmt.Errorf("close skill staging directory: %w", err)}
	}
	if err := destination.RemoveAll(stageName); err != nil {
		return &committedSkillInstallError{cause: fmt.Errorf("remove skill staging directory: %w", err)}
	}
	return nil
}

type retainedSkillBackupError struct {
	cause error
	path  string
}

func (err *retainedSkillBackupError) Error() string {
	return fmt.Sprintf("%v; previous skill data was preserved at %s; after resolving target conflicts, restore needed entries from that directory and then remove it", err.cause, err.path)
}

func (err *retainedSkillBackupError) Unwrap() error {
	return err.cause
}

type committedSkillInstallError struct {
	cause error
}

func (err *committedSkillInstallError) Error() string {
	return err.cause.Error()
}

func (err *committedSkillInstallError) Unwrap() error {
	return err.cause
}

func mustKeepPinnedSkillLock(err error) bool {
	var committed *committedSkillInstallError
	return errors.As(err, &committed)
}

type treeEntry struct {
	mode   fs.FileMode
	digest [sha256.Size]byte
}

func snapshotTree(root *os.Root, path string) (map[string]treeEntry, error) {
	snapshot := make(map[string]treeEntry)
	if err := snapshotTreeAt(root, path, ".", snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func snapshotTreeAt(root *os.Root, path, relative string, snapshot map[string]treeEntry) error {
	info, err := root.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic link %q is not permitted in the reviewed skill pack", path)
	}
	if info.IsDir() {
		snapshot[relative] = treeEntry{mode: info.Mode().Perm()}
		entries, err := readRootDir(root, path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			childRelative := entry.Name()
			if relative != "." {
				childRelative = filepath.Join(relative, entry.Name())
			}
			if err := snapshotTreeAt(root, filepath.Join(path, entry.Name()), childRelative, snapshot); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular file %q is not permitted in the reviewed skill pack", path)
	}
	file, err := root.Open(path)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, copyErr := io.Copy(hash, file)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	snapshot[relative] = treeEntry{mode: info.Mode().Perm(), digest: digest}
	return nil
}

func copyTree(source *os.Root, sourcePath string, destination *os.Root, destinationPath string) error {
	info, err := source.Lstat(sourcePath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symbolic link %q is not permitted", sourcePath)
	}
	if info.IsDir() {
		if err := destination.Mkdir(destinationPath, info.Mode().Perm()); err != nil {
			return err
		}
		if err := destination.Chmod(destinationPath, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := readRootDir(source, sourcePath)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(
				source,
				filepath.Join(sourcePath, entry.Name()),
				destination,
				filepath.Join(destinationPath, entry.Name()),
			); err != nil {
				return err
			}
		}
		return nil
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("non-regular file %q is not permitted", sourcePath)
	}
	input, err := source.Open(sourcePath)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := destination.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return destination.Chmod(destinationPath, info.Mode().Perm())
}

func readRootDir(root *os.Root, path string) ([]os.DirEntry, error) {
	dir, err := root.Open(path)
	if err != nil {
		return nil, err
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return entries, nil
}

func createPrivateDirectory(root *os.Root, prefix string) (string, *os.Root, error) {
	for range 100 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return "", nil, err
		}
		name := fmt.Sprintf("%s%x", prefix, random[:])
		if err := root.Mkdir(name, 0o700); err != nil {
			if errors.Is(err, fs.ErrExist) {
				continue
			}
			return "", nil, err
		}
		child, err := root.OpenRoot(name)
		if err != nil {
			_ = root.RemoveAll(name)
			return "", nil, err
		}
		return name, child, nil
	}
	return "", nil, errors.New("could not allocate a unique private directory")
}
