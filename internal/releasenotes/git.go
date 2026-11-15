package releasenotes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var errGitNotFound = errors.New("git not found on PATH")

// LatestTagMatching returns the most recent tag reachable from until whose
// name matches the glob pattern.
func LatestTagMatching(ctx context.Context, repoDir, pattern, until string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", errGitNotFound
	}

	cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0", "--match", pattern, "--end-of-options", until)
	if strings.TrimSpace(repoDir) != "" {
		cmd.Dir = repoDir
	}
	cmd.Env = cleanGitRepoEnv(os.Environ())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "No names found") || strings.Contains(msg, "No tags can describe") {
			return "", fmt.Errorf("no tag matching %q is reachable from %s", pattern, until)
		}
		if msg == "" {
			return "", fmt.Errorf("git describe failed: %w", err)
		}
		return "", fmt.Errorf("git describe failed: %s", msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// ListCommits returns commits in (since, until], ordered oldest -> newest.
//
// since and until are git revs (tag, branch, SHA, etc). An empty since means
// the whole history up to until. maxCommits > 0 keeps only the newest
// maxCommits commits.
func ListCommits(ctx context.Context, repoDir, since, until string, includeMerges bool, maxCommits int) ([]Commit, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errGitNotFound
	}

	since = strings.TrimSpace(since)
	until = strings.TrimSpace(until)
	if until == "" || (since == "" && maxCommits < 1) {
		return nil, fmt.Errorf("since and until are required")
	}

	// Use NUL between sha and subject so parsing is robust even if subjects contain tabs.
	const pretty = "--pretty=format:%h%x00%s"

	args := []string{
		"log",
		"--reverse",
		"--no-color",
		"--no-decorate",
		pretty,
	}
	if !includeMerges {
		args = append(args, "--no-merges")
	}
	if maxCommits > 0 {
		args = append(args, fmt.Sprintf("--max-count=%d", maxCommits))
	}
	args = append(args, "--end-of-options")
	if since == "" {
		args = append(args, until)
	} else {
		args = append(args, fmt.Sprintf("%s..%s", since, until))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if strings.TrimSpace(repoDir) != "" {
		cmd.Dir = repoDir
	}
	// Git hooks (especially in worktrees) can export GIT_* repo override variables.
	// Remove them so the command consistently targets cmd.Dir / cwd.
	cmd.Env = cleanGitRepoEnv(os.Environ())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("git log failed: %w", err)
		}
		return nil, fmt.Errorf("git log failed: %s", msg)
	}

	out := stdout.Bytes()
	out = bytes.TrimSuffix(out, []byte{'\n'})
	if len(out) == 0 {
		return nil, nil
	}

	rawLines := bytes.Split(out, []byte{'\n'})
	commits := make([]Commit, 0, len(rawLines))
	for _, line := range rawLines {
		if len(line) == 0 {
			continue
		}
		parts := bytes.SplitN(line, []byte{0}, 2)
		if len(parts) != 2 {
			// Defensive: if parsing fails, avoid losing data entirely.
			commits = append(commits, Commit{Subject: string(bytes.TrimSpace(line))})
			continue
		}
		commits = append(commits, Commit{
			SHA:     string(bytes.TrimSpace(parts[0])),
			Subject: string(bytes.TrimSpace(parts[1])),
		})
	}
	return commits, nil
}

func cleanGitRepoEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "GIT_DIR="):
			continue
		case strings.HasPrefix(kv, "GIT_WORK_TREE="):
			continue
		case strings.HasPrefix(kv, "GIT_INDEX_FILE="):
			continue
		case strings.HasPrefix(kv, "GIT_COMMON_DIR="):
			continue
		}
		out = append(out, kv)
	}
	return out
}
