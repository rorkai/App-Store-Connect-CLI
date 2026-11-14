package signing

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
	"golang.org/x/text/unicode/norm"
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
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Opaque == "" {
		// net/url parsed a hierarchical URL, so the sanitizer sees any
		// userinfo and sensitive query values. An scp-style remote parses
		// as an opaque URL whose credentials the sanitizer cannot see,
		// even when rendering it changes the text (net/url lowercases the
		// "scheme" that is really the user name).
		return urlsanitize.SanitizeURLForLog(trimmed, urlsanitize.DefaultSignedQueryKeys, urlsanitize.DefaultSensitiveQueryKeys)
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
		return withoutQueryOrFragment(raw)
	}
	if !hasScheme && !strings.Contains(authority[:credentialsEnd], ":") {
		return withoutQueryOrFragment(raw)
	}

	redacted := redactedRepoUserinfo + withoutQueryOrFragment(authority[credentialsEnd:]+remainder)
	if hasScheme {
		return scheme + "://" + redacted
	}
	return redacted
}

// withoutQueryOrFragment drops a query or fragment, which can carry a token
// that the fallback above cannot parse and redact by key.
func withoutQueryOrFragment(raw string) string {
	if index := strings.IndexAny(raw, "?#"); index >= 0 {
		return raw[:index]
	}
	return raw
}

// GitStore manages an encrypted git repository of signing assets.
type GitStore struct {
	RepoURL  string
	LocalDir string
	Branch   string

	rootMu sync.Mutex
	root   *rootfs.Root
}

func (g *GitStore) filesystemRoot() (rootfs.Root, error) {
	absolute, err := filepath.Abs(g.LocalDir)
	if err != nil {
		return rootfs.Root{}, err
	}
	absolute = filepath.Clean(absolute)

	g.rootMu.Lock()
	defer g.rootMu.Unlock()
	if g.root != nil && g.root.Path() == absolute {
		return *g.root, nil
	}
	if g.root != nil {
		if err := g.root.Close(); err != nil {
			return rootfs.Root{}, err
		}
		g.root = nil
	}
	root, err := rootfs.New(absolute)
	if err != nil {
		return rootfs.Root{}, err
	}
	g.root = &root
	return root, nil
}

func (g *GitStore) closeFilesystemRoot() error {
	g.rootMu.Lock()
	defer g.rootMu.Unlock()
	if g.root == nil {
		return nil
	}
	err := g.root.Close()
	g.root = nil
	return err
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
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		return err
	}

	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.WriteFile(relPath+".enc", encrypted, 0o600)
}

// ReplaceEncryptedFile creates or replaces a legacy encrypted artifact while
// preserving its existing file mode when present. Replacement is atomic on
// platforms where rename replaces an existing destination; on Windows, the
// original is restored if publishing the replacement fails.
func (g *GitStore) ReplaceEncryptedFile(relPath string, plaintext []byte, password string) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	encrypted, err := Encrypt(plaintext, password)
	if err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.WriteFilePreservingMode(relPath+".enc", encrypted, 0o600)
}

// WriteEncryptedFileWithMetadata writes a versioned encrypted file whose
// non-secret metadata is authenticated with the ciphertext.
func (g *GitStore) WriteEncryptedFileWithMetadata(relPath string, plaintext []byte, password string, metadata EncryptedFileMetadata) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	metadata.RelativePath = canonicalEncryptedRepositoryPath(relPath)
	encrypted, err := EncryptFile(plaintext, password, metadata)
	if err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.CreateNewFileAtomic(relPath+".enc", encrypted, 0o600)
}

// ReplaceEncryptedFileWithMetadata atomically creates or replaces a versioned
// encrypted artifact after the caller has validated its scope.
func (g *GitStore) ReplaceEncryptedFileWithMetadata(relPath string, plaintext []byte, password string, metadata EncryptedFileMetadata) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	metadata.RelativePath = canonicalEncryptedRepositoryPath(relPath)
	encrypted, err := EncryptFile(plaintext, password, metadata)
	if err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.WriteFilePreservingMode(relPath+".enc", encrypted, 0o600)
}

// CheckNewEncryptedFile performs the rooted, non-mutating destination checks
// used before publishing a new versioned encrypted artifact.
func (g *GitStore) CheckNewEncryptedFile(relPath string) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.CheckCreateNewFile(relPath + ".enc")
}

// CheckWriteEncryptedFile performs the non-mutating rooted checks used before
// replacing or creating a legacy encrypted signing artifact.
func (g *GitStore) CheckWriteEncryptedFile(relPath string) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.CheckWriteFilePreservingMode(relPath + ".enc")
}

// CheckEncryptedFileParent validates an encrypted artifact's future parent
// layout without assuming its final profile-specific name is known yet.
func (g *GitStore) CheckEncryptedFileParent(relPath string) error {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return err
	}
	root, err := g.filesystemRoot()
	if err != nil {
		return err
	}
	return root.CheckFileParent(relPath + ".enc")
}

// EncryptedFileSize returns the size of a regular no-follow encrypted artifact.
func (g *GitStore) EncryptedFileSize(relPath string) (int64, error) {
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return 0, err
	}
	root, err := g.filesystemRoot()
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
	if err := validateEncryptedRepositoryPath(filepath.ToSlash(relPath)); err != nil {
		return nil, EncryptedFileMetadata{}, err
	}
	root, err := g.filesystemRoot()
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

// ValidateEncryptedRepositoryPaths rejects path sets that cannot coexist on a
// Windows case-insensitive or normalization-insensitive macOS checkout. It
// normalizes canonically equivalent paths before applying the Unicode
// simple-fold classes behind strings.EqualFold.
// Exact duplicate canonical paths retain their existing update semantics.
func ValidateEncryptedRepositoryPaths(paths []string) error {
	seen := make(map[string]string, len(paths))
	for _, rawPath := range paths {
		portablePath := filepath.ToSlash(rawPath)
		if err := validateEncryptedRepositoryPath(portablePath); err != nil {
			return err
		}
		key := windowsUnicodeCaseFoldKey(portablePath)
		if existing, ok := seen[key]; ok && existing != portablePath {
			return errors.New("encrypted repository paths collide under Windows Unicode case folding")
		}
		seen[key] = portablePath
	}
	return nil
}

// CheckEncryptedRepositoryPaths validates planned paths together with every
// existing encrypted artifact before a push writes any repository files.
func (g *GitStore) CheckEncryptedRepositoryPaths(planned []string) error {
	existing, err := g.ListEncryptedFiles()
	if err != nil {
		return err
	}
	combined := make([]string, 0, len(existing)+len(planned))
	combined = append(combined, existing...)
	combined = append(combined, planned...)
	return ValidateEncryptedRepositoryPaths(combined)
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
	if err != nil {
		return nil, err
	}
	if err := ValidateEncryptedRepositoryPaths(files); err != nil {
		return nil, err
	}
	return files, nil
}

func validateEncryptedRepositoryPath(path string) error {
	if !utf8.ValidString(path) {
		return errors.New("encrypted repository path is not valid UTF-8")
	}
	if strings.ContainsRune(path, '\\') {
		return fmt.Errorf("encrypted repository path %q contains a non-portable backslash", path)
	}
	for _, r := range path {
		if unicode.IsControl(r) || isBidiControl(r) {
			return fmt.Errorf("encrypted repository path contains control characters")
		}
	}
	for _, component := range strings.Split(path, "/") {
		if err := validateWindowsPortablePathComponent(component); err != nil {
			return fmt.Errorf("encrypted repository path %q has a Windows-incompatible component: %w", path, err)
		}
	}
	return nil
}

func windowsUnicodeCaseFoldKey(path string) string {
	path = norm.NFC.String(path)
	var key strings.Builder
	key.Grow(len(path))
	for _, r := range path {
		representative := r
		for folded := unicode.SimpleFold(r); folded != r; folded = unicode.SimpleFold(folded) {
			if folded < representative {
				representative = folded
			}
		}
		key.WriteRune(representative)
	}
	return key.String()
}

func validateWindowsPortablePathComponent(component string) error {
	if component == "" {
		return fmt.Errorf("path component is empty")
	}
	// Rooted filesystem validation reports traversal components with its
	// established ErrEscapesRoot contract.
	if component == "." || component == ".." {
		return nil
	}
	if strings.ContainsAny(component, `<>:"|?*`) {
		return fmt.Errorf("path component %q contains a reserved character", component)
	}
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return fmt.Errorf("path component %q ends with a dot or space", component)
	}

	stem := component
	if dot := strings.IndexByte(stem, '.'); dot >= 0 {
		stem = stem[:dot]
	}
	upperStem := strings.ToUpper(stem)
	switch upperStem {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		"COM¹", "COM²", "COM³", "LPT¹", "LPT²", "LPT³":
		return fmt.Errorf("path component %q uses a reserved device name", component)
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
	if err := readonly.Check(ctx, http.MethodPost, RedactRepoURL(g.RepoURL)); err != nil {
		return err
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
	closeErr := g.closeFilesystemRoot()
	var removeErr error
	if g.LocalDir != "" {
		removeErr = os.RemoveAll(g.LocalDir)
	}
	return errors.Join(closeErr, removeErr)
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
	configureGitProcess(cmd)
	return cmd, nil
}

var gitSigningSyncPasswordEnvironmentKeys = []string{
	"ASC_SIGNING_SYNC_PASSWORD",
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
	configureGitProcess(cmd)
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
	output := newRepoRedactingWriter(os.Stderr, g.RepoURL)
	cmd.Stdout = output // progress to stderr
	cmd.Stderr = output
	err = cmd.Run()
	_ = output.Flush()
	return err
}

func (g *GitStore) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd, err := newGitCommand(ctx, dir, args...)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	output := newRepoRedactingWriter(os.Stderr, g.RepoURL)
	cmd.Stderr = output
	err = cmd.Run()
	_ = output.Flush()
	return stdout.String(), err
}

// repoRedactingWriter hides repository credentials in Git's own output, which
// can echo the remote with its token. It holds back a partial line so a secret
// split across writes is still caught.
type repoRedactingWriter struct {
	out      io.Writer
	replacer *strings.Replacer
	pending  []byte
}

func newRepoRedactingWriter(out io.Writer, repoURL string) *repoRedactingWriter {
	raw := strings.TrimSpace(repoURL)
	pairs := []string{}
	if raw != "" {
		pairs = append(pairs, raw, RedactRepoURL(raw))
	}
	secrets := repoURLSecrets(raw)
	slices.SortFunc(secrets, func(a, b string) int { return cmp.Or(len(b)-len(a), strings.Compare(a, b)) })
	for _, secret := range slices.Compact(secrets) {
		if len(secret) >= 4 {
			pairs = append(pairs, secret, redactedRepoUserinfo)
			continue
		}
		// A short credential is replaced only where it sits in the URL, so a
		// one-letter password does not erase that letter from all output.
		pairs = append(pairs, ":"+secret+"@", ":"+redactedRepoUserinfo+"@", "//"+secret+"@", "//"+redactedRepoUserinfo+"@")
		for _, end := range []string{"&", ";", "#", "/", "'", "\"", " ", "\r", "\n"} {
			pairs = append(pairs, "="+secret+end, "="+redactedRepoUserinfo+end)
		}
	}
	return &repoRedactingWriter{out: out, replacer: strings.NewReplacer(pairs...)}
}

func (w *repoRedactingWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	end := bytes.LastIndexAny(w.pending, "\r\n") + 1
	if end == 0 {
		return len(p), nil
	}
	if _, err := io.WriteString(w.out, w.replacer.Replace(string(w.pending[:end]))); err != nil {
		return 0, err
	}
	w.pending = append(w.pending[:0], w.pending[end:]...)
	return len(p), nil
}

// Flush writes any final partial line once the command has exited.
func (w *repoRedactingWriter) Flush() error {
	if len(w.pending) == 0 {
		return nil
	}
	_, err := io.WriteString(w.out, w.replacer.Replace(string(w.pending)))
	w.pending = w.pending[:0]
	return err
}

// repoURLSecrets returns the credentials embedded in a remote, in raw,
// decoded, and escaped forms: the userinfo password (or a token used as the
// user name) and the values of sensitive query keys.
func repoURLSecrets(raw string) []string {
	var values []string
	_, rest, hasScheme := strings.Cut(raw, "://")
	if !hasScheme {
		rest = raw
	}
	authority := rest
	if index := strings.IndexAny(rest, "/?#"); index >= 0 {
		authority = rest[:index]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		user, password, hasPassword := strings.Cut(authority[:at], ":")
		if hasPassword {
			values = append(values, password)
		} else if hasScheme {
			values = append(values, user)
		}
	}

	_, query, _ := strings.Cut(raw, "?")
	query, _, _ = strings.Cut(query, "#")
	var queryValues []string
	redactAll := false
	for _, pair := range strings.FieldsFunc(query, func(r rune) bool { return r == '&' || r == ';' }) {
		key, value, _ := strings.Cut(pair, "=")
		if decoded, err := url.QueryUnescape(key); err == nil {
			key = decoded
		}
		key = strings.ToLower(key)
		if _, ok := urlsanitize.DefaultSignedQueryKeys[key]; ok && value != "" {
			redactAll = true
		}
		if _, ok := urlsanitize.DefaultSensitiveQueryKeys[key]; ok {
			values = append(values, value)
		}
		queryValues = append(queryValues, value)
	}
	if redactAll {
		values = append(values, queryValues...)
	}

	var secrets []string
	for _, value := range values {
		forms := []string{value, url.QueryEscape(value), url.PathEscape(value)}
		if decoded, err := url.PathUnescape(value); err == nil {
			forms = append(forms, decoded)
		}
		if decoded, err := url.QueryUnescape(value); err == nil {
			forms = append(forms, decoded)
		}
		for _, form := range forms {
			if form != "" {
				secrets = append(secrets, form)
			}
		}
	}
	return secrets
}
