package signing

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

	if !allowCreate {
		return fmt.Errorf("git clone: branch %q not found in %s: %w", branch, g.RepoURL, err)
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

	fullPath := filepath.Join(g.LocalDir, relPath+".enc")
	if err := EnsureInsideDir(g.LocalDir, fullPath); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return err
	}
	if err := RejectSymlinkIfExists(fullPath); err != nil {
		return err
	}

	return os.WriteFile(fullPath, encrypted, 0o600)
}

// ReadEncryptedFile reads and decrypts a file from the repo.
// Rejects symlinks to prevent reading outside the clone directory.
func (g *GitStore) ReadEncryptedFile(relPath string, password string) ([]byte, error) {
	fullPath := filepath.Join(g.LocalDir, relPath+".enc")
	if err := EnsureInsideDir(g.LocalDir, fullPath); err != nil {
		return nil, err
	}
	if err := rejectSymlink(fullPath); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	return Decrypt(data, password)
}

// ListEncryptedFiles returns relative paths (without .enc) of all encrypted files.
func (g *GitStore) ListEncryptedFiles() ([]string, error) {
	var files []string
	err := filepath.Walk(g.LocalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			// Skip symlinked directories to prevent escape.
			if info.Mode()&os.ModeSymlink != 0 {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip symlinked files.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if strings.HasSuffix(info.Name(), ".enc") {
			rel, err := filepath.Rel(g.LocalDir, path)
			if err != nil {
				return err
			}
			files = append(files, strings.TrimSuffix(rel, ".enc"))
		}
		return nil
	})
	return files, err
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

// rejectSymlink checks that path is not a symlink.
func rejectSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to read symlink %q (potential escape)", path)
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

func (g *GitStore) gitRun(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdout = os.Stderr // progress to stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *GitStore) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	return stdout.String(), err
}
