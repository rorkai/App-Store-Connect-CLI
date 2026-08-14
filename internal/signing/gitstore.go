package signing

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

// redactedRepoUserinfo replaces credentials that net/url cannot parse.
const redactedRepoUserinfo = "[REDACTED]"

// RedactRepoURL removes credentials embedded in a repository URL so the remote
// can be named in errors, diagnostics, and structured output without leaking a
// token or password.
func RedactRepoURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	sanitized := urlsanitize.SanitizeURLForLog(trimmed, urlsanitize.DefaultSignedQueryKeys, urlsanitize.DefaultSensitiveQueryKeys)
	if sanitized != trimmed {
		// net/url parsed the remote and replaced any embedded userinfo.
		return sanitized
	}
	return redactRemoteUserinfo(trimmed)
}

// redactRemoteUserinfo strips credentials from remotes that net/url leaves
// untouched, such as scp-style remotes and unescaped userinfo. An scp-style
// remote without a password keeps its user name, because it carries no secret.
func redactRemoteUserinfo(raw string) string {
	scheme, rest, hasScheme := strings.Cut(raw, "://")
	if !hasScheme {
		rest = raw
	}
	authority, remainder := rest, ""
	if index := strings.Index(rest, "/"); index >= 0 {
		authority, remainder = rest[:index], rest[index:]
	}
	credentialsEnd := strings.LastIndex(authority, "@")
	if credentialsEnd < 0 {
		return raw
	}
	if !hasScheme && !strings.Contains(authority[:credentialsEnd], ":") {
		return raw
	}

	redacted := redactedRepoUserinfo + authority[credentialsEnd:] + remainder
	if hasScheme {
		return scheme + "://" + redacted
	}
	return redacted
}

// GitStore manages an encrypted git repository of signing assets.
type GitStore struct {
	RepoURL  string
	LocalDir string
	Branch   string
}

// Clone clones the git repo. If allowCreate is true (push mode), falls back to
// initializing an empty repo when the branch doesn't exist. If false (pull mode),
// fails when the branch is missing.
func (g *GitStore) Clone(ctx context.Context, allowCreate bool) error {
	branch := g.Branch
	if branch == "" {
		branch = "main"
	}

	// Try cloning with the branch first.
	err := g.gitRun(ctx, "", "clone", "--single-branch", "--branch", branch, "--depth", "1", g.RepoURL, g.LocalDir)
	if err == nil {
		return nil
	}

	// A local Git configuration failure says nothing about the remote, so it
	// must not be reported as a missing branch or retried as an empty repo.
	var configErr gitConfigProbeError
	if errors.As(err, &configErr) {
		return err
	}

	if !allowCreate {
		return fmt.Errorf("git clone: branch %q not found in %s: %w", branch, RedactRepoURL(g.RepoURL), err)
	}

	// Push mode: may be empty repo — clone without branch and init.
	if err2 := g.gitRun(ctx, "", "clone", g.RepoURL, g.LocalDir); err2 != nil {
		return fmt.Errorf("git clone: %w", err2)
	}

	// Ensure we're on the target branch.
	if _, err2 := g.gitOutput(ctx, g.LocalDir, "rev-parse", "HEAD"); err2 != nil {
		// Empty repo — create the branch.
		if err3 := g.gitRun(ctx, g.LocalDir, "checkout", "-b", branch); err3 != nil {
			return fmt.Errorf("git checkout -b: %w", err3)
		}
	} else {
		// Non-empty repo — switch to or create the target branch.
		if err3 := g.gitRun(ctx, g.LocalDir, "checkout", branch); err3 != nil {
			if err4 := g.gitRun(ctx, g.LocalDir, "checkout", "-b", branch); err4 != nil {
				return fmt.Errorf("git checkout -b %s: %w", branch, err4)
			}
		}
	}

	return nil
}

// WriteEncryptedFile writes an encrypted file into the repo.
// Validates that the resolved path stays inside LocalDir to prevent symlink escapes.
func (g *GitStore) WriteEncryptedFile(relPath string, plaintext []byte, password string) error {
	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		return err
	}

	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.WriteFile(relPath+".enc", encrypted, 0o600)
}

// WriteEncryptedFileWithMetadata writes a versioned encrypted file whose
// non-secret metadata is authenticated with the ciphertext.
func (g *GitStore) WriteEncryptedFileWithMetadata(relPath string, plaintext []byte, password string, metadata EncryptedFileMetadata) error {
	metadata.RelativePath = canonicalEncryptedRepositoryPath(relPath)
	encrypted, err := EncryptFile(plaintext, password, metadata)
	if err != nil {
		return err
	}
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.CreateNewFile(relPath+".enc", encrypted, 0o600)
}

// ReplaceEncryptedFileWithMetadata atomically creates or replaces a versioned
// encrypted non-secret index artifact after the caller has validated its scope.
func (g *GitStore) ReplaceEncryptedFileWithMetadata(relPath string, plaintext []byte, password string, metadata EncryptedFileMetadata) error {
	metadata.RelativePath = canonicalEncryptedRepositoryPath(relPath)
	encrypted, err := EncryptFile(plaintext, password, metadata)
	if err != nil {
		return err
	}
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.WriteFilePreservingMode(relPath+".enc", encrypted, 0o600)
}

// CheckNewEncryptedFile performs the rooted, non-mutating destination checks
// used before publishing a new versioned encrypted artifact.
func (g *GitStore) CheckNewEncryptedFile(relPath string) error {
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.CheckCreateNewFile(relPath + ".enc")
}

// CheckWriteEncryptedFile performs the non-mutating rooted checks used before
// replacing or creating a legacy encrypted signing artifact.
func (g *GitStore) CheckWriteEncryptedFile(relPath string) error {
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.CheckWriteFilePreservingMode(relPath + ".enc")
}

// CheckEncryptedFileParent validates an encrypted artifact's future parent
// layout without assuming its final profile-specific name is known yet.
func (g *GitStore) CheckEncryptedFileParent(relPath string) error {
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return err
	}
	return root.CheckFileParent(relPath + ".enc")
}

// EncryptedFileSize returns the size of a regular no-follow encrypted artifact.
func (g *GitStore) EncryptedFileSize(relPath string) (int64, error) {
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return 0, err
	}
	file, err := root.OpenFile(relPath + ".enc")
	if err != nil {
		return 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// ReadEncryptedFile reads and decrypts a file from the repo.
// Rejects symlinks to prevent reading outside the clone directory.
func (g *GitStore) ReadEncryptedFile(relPath string, password string) ([]byte, error) {
	plaintext, _, err := g.ReadEncryptedFileWithMetadata(relPath, password)
	return plaintext, err
}

// ReadEncryptedFileWithMetadata reads either a versioned envelope or a legacy
// encrypted file.
func (g *GitStore) ReadEncryptedFileWithMetadata(relPath string, password string) ([]byte, EncryptedFileMetadata, error) {
	root, err := rootfs.New(g.LocalDir)
	if err != nil {
		return nil, EncryptedFileMetadata{}, err
	}
	data, err := root.ReadFileLimited(relPath+".enc", maxEncryptedFileSize)
	if err != nil {
		return nil, EncryptedFileMetadata{}, err
	}
	plaintext, metadata, err := DecryptFile(data, password)
	if err != nil {
		return nil, EncryptedFileMetadata{}, err
	}
	if metadata.Version != 0 && canonicalEncryptedRepositoryPath(metadata.RelativePath) != canonicalEncryptedRepositoryPath(relPath) {
		return nil, EncryptedFileMetadata{}, fmt.Errorf("encrypted file metadata path %q does not match %q", metadata.RelativePath, relPath)
	}
	return plaintext, metadata, nil
}

func canonicalEncryptedRepositoryPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), `\`, "/")
}

// ListEncryptedFiles returns relative paths (without .enc) of all encrypted files.
func (g *GitStore) ListEncryptedFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(g.LocalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("encrypted signing repository contains symlink %q", path)
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".enc") {
			rel, err := filepath.Rel(g.LocalDir, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if err := validateEncryptedRepositoryPath(rel); err != nil {
				return err
			}
			files = append(files, strings.TrimSuffix(rel, ".enc"))
		}
		return nil
	})
	return files, err
}

func validateEncryptedRepositoryPath(path string) error {
	if strings.ContainsRune(path, '\\') {
		return fmt.Errorf("encrypted repository path %q contains a non-portable backslash", path)
	}
	for _, r := range path {
		if unicode.IsControl(r) || isBidiControl(r) {
			return fmt.Errorf("encrypted repository path contains control characters")
		}
	}
	return nil
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		(r >= '\u202a' && r <= '\u202e') || (r >= '\u2066' && r <= '\u2069')
}

// CommitAndPush stages all changes, commits, and pushes.
func (g *GitStore) CommitAndPush(ctx context.Context, message string) error {
	if err := g.gitRun(ctx, g.LocalDir, "add", "-A"); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	// Check if there are changes to commit.
	status, err := g.gitOutput(ctx, g.LocalDir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil // nothing to commit
	}

	if err := g.gitRun(ctx, g.LocalDir, "commit", "-m", message); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	branch := g.Branch
	if branch == "" {
		branch = "main"
	}
	if err := g.gitRun(ctx, g.LocalDir, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// Cleanup removes the local clone directory.
func (g *GitStore) Cleanup() error {
	if g.LocalDir == "" {
		return nil
	}
	return os.RemoveAll(g.LocalDir)
}

// EnsureInsideDir checks that target stays inside baseDir and does not traverse
// any symlinked parent directories.
func EnsureInsideDir(baseDir, target string) error {
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("resolve base dir: %w", err)
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve target path: %w", err)
	}
	if !strings.HasPrefix(absTarget, absBase+string(filepath.Separator)) && absTarget != absBase {
		return fmt.Errorf("path %q escapes base directory %q", target, baseDir)
	}

	if absTarget == absBase {
		return nil
	}

	parent := filepath.Dir(absTarget)
	relParent, err := filepath.Rel(absBase, parent)
	if err != nil {
		return fmt.Errorf("resolve target parent: %w", err)
	}

	current := absBase
	for _, component := range strings.Split(relParent, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}

		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return fmt.Errorf("inspect path %q: %w", current, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("path %q uses symlink component %q", target, current)
		}
	}

	return nil
}

// RejectSymlinkIfExists rejects writes through an existing symlink path.
func RejectSymlinkIfExists(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write symlink %q (potential escape)", path)
	}
	return nil
}

// gitConfigProbeError marks a failure of the local Git configuration probe, so
// callers can tell it apart from a failure of the Git command they asked for.
type gitConfigProbeError struct {
	err error
}

func (e gitConfigProbeError) Error() string { return e.err.Error() }

func (e gitConfigProbeError) Unwrap() error { return e.err }

func newGitCommand(ctx context.Context, dir string, args ...string) (*exec.Cmd, error) {
	environment := gitEnvironmentWithoutRepositorySelectors(os.Environ(), runtime.GOOS)
	environment = gitEnvironmentWithoutSigningSyncPasswords(environment, runtime.GOOS)
	coreSSHCommandConfigured := false
	if gitCommandMayUseSSH(args) && !hasGitSSHEnvironmentOverride(environment, runtime.GOOS) {
		var err error
		includeRepositoryConfig := args[0] != "clone"
		coreSSHCommandConfigured, err = hasConfiguredGitSSHCommand(ctx, dir, environment, runtime.GOOS, includeRepositoryConfig)
		if err != nil {
			return nil, err
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = gitCommandEnvironmentWithConfig(environment, runtime.GOOS, coreSSHCommandConfigured)
	return cmd, nil
}

var gitSigningSyncPasswordEnvironmentKeys = []string{
	"ASC_SIGNING_SYNC_PASSWORD",
	"ASC_MATCH_PASSWORD",
}

func gitEnvironmentWithoutSigningSyncPasswords(environment []string, goos string) []string {
	caseInsensitive := goos == "windows"
	for _, key := range gitSigningSyncPasswordEnvironmentKeys {
		environment = removeCommandEnvironmentValue(environment, key, caseInsensitive)
	}
	return environment
}

var gitRepositorySelectorEnvironmentKeys = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_OBJECT_DIRECTORY",
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_IMPLICIT_WORK_TREE",
	"GIT_GRAFT_FILE",
	"GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS",
	"GIT_REPLACE_REF_BASE",
	"GIT_PREFIX",
	"GIT_INTERNAL_SUPER_PREFIX",
	"GIT_SHALLOW_FILE",
	"GIT_COMMON_DIR",
}

func gitEnvironmentWithoutRepositorySelectors(environment []string, goos string) []string {
	caseInsensitive := goos == "windows"
	for _, key := range gitRepositorySelectorEnvironmentKeys {
		environment = removeCommandEnvironmentValue(environment, key, caseInsensitive)
	}
	return environment
}

func gitCommandEnvironmentWithConfig(environment []string, goos string, coreSSHCommandConfigured bool) []string {
	caseInsensitive := goos == "windows"
	environment = replaceCommandEnvironmentValue(environment, "GIT_TERMINAL_PROMPT", "0", caseInsensitive)

	sshCommand, ok := commandEnvironmentValue(environment, "GIT_SSH_COMMAND", caseInsensitive)
	if ok && strings.TrimSpace(sshCommand) != "" {
		// A caller-provided command may contain shell quoting or invoke a wrapper.
		// Preserve it verbatim instead of trying to append or rewrite SSH options.
		return replaceCommandEnvironmentValue(environment, "GIT_SSH_COMMAND", sshCommand, caseInsensitive)
	}
	environment = removeCommandEnvironmentValue(environment, "GIT_SSH_COMMAND", caseInsensitive)

	gitSSH, ok := commandEnvironmentValue(environment, "GIT_SSH", caseInsensitive)
	if ok && strings.TrimSpace(gitSSH) != "" {
		return environment
	}
	environment = removeCommandEnvironmentValue(environment, "GIT_SSH", caseInsensitive)

	if coreSSHCommandConfigured {
		return environment
	}
	return replaceCommandEnvironmentValue(environment, "GIT_SSH_COMMAND", "ssh -o BatchMode=yes", caseInsensitive)
}

func hasGitSSHEnvironmentOverride(environment []string, goos string) bool {
	caseInsensitive := goos == "windows"
	for _, key := range []string{"GIT_SSH_COMMAND", "GIT_SSH"} {
		value, ok := commandEnvironmentValue(environment, key, caseInsensitive)
		if ok && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

func gitCommandMayUseSSH(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "clone", "fetch", "ls-remote", "pull", "push", "submodule":
		return true
	default:
		return false
	}
}

func hasConfiguredGitSSHCommand(
	ctx context.Context,
	dir string,
	environment []string,
	goos string,
	includeRepositoryConfig bool,
) (bool, error) {
	queryDir := dir
	queryEnvironment := replaceCommandEnvironmentValue(environment, "GIT_TERMINAL_PROMPT", "0", goos == "windows")
	if !includeRepositoryConfig {
		neutralRoot, err := os.MkdirTemp("", "asc-git-config-")
		if err != nil {
			return false, gitConfigProbeError{fmt.Errorf("create neutral Git config probe: %w", err)}
		}
		defer func() {
			_ = os.Remove(neutralRoot)
		}()

		queryDir = neutralRoot
		queryEnvironment = replaceCommandEnvironmentValue(
			queryEnvironment,
			"GIT_DIR",
			filepath.Join(neutralRoot, "nonexistent.git"),
			goos == "windows",
		)
	}

	cmd := exec.CommandContext(ctx, "git", "config", "--get", "core.sshCommand")
	cmd.Dir = queryDir
	cmd.Env = queryEnvironment
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
			return false, nil
		}
		return false, gitConfigProbeError{fmt.Errorf("check Git core.sshCommand: %w", err)}
	}
	return strings.TrimSpace(string(output)) != "", nil
}

func commandEnvironmentValue(environment []string, key string, caseInsensitive bool) (string, bool) {
	for i := len(environment) - 1; i >= 0; i-- {
		if value, ok := commandEnvironmentEntryValue(environment[i], key, caseInsensitive); ok {
			return value, true
		}
	}
	return "", false
}

func replaceCommandEnvironmentValue(environment []string, key, value string, caseInsensitive bool) []string {
	return append(removeCommandEnvironmentValue(environment, key, caseInsensitive), key+"="+value)
}

func removeCommandEnvironmentValue(environment []string, key string, caseInsensitive bool) []string {
	updated := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if _, ok := commandEnvironmentEntryValue(entry, key, caseInsensitive); ok {
			continue
		}
		updated = append(updated, entry)
	}
	return updated
}

func commandEnvironmentEntryValue(entry, key string, caseInsensitive bool) (string, bool) {
	entryKey, value, ok := strings.Cut(entry, "=")
	if !ok {
		return "", false
	}
	if entryKey != key && (!caseInsensitive || !strings.EqualFold(entryKey, key)) {
		return "", false
	}
	return value, true
}

func (g *GitStore) gitRun(ctx context.Context, dir string, args ...string) error {
	cmd, err := newGitCommand(ctx, dir, args...)
	if err != nil {
		return err
	}
	cmd.Stdout = os.Stderr // progress to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *GitStore) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd, err := newGitCommand(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	return stdout.String(), err
}
