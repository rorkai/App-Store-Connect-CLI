package cmdtest

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
)

func TestReleaseNotesGenerate_JSON(t *testing.T) {
	unsetGitHookEnv(t)

	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	repo := initTempGitRepo(t)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir repo error: %v", err)
	}

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{"release-notes", "generate", "--since-tag", "v1.0.0", "--output", "json"}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var got struct {
		Since       string `json:"since"`
		Until       string `json:"until"`
		CommitCount int    `json:"commitCount"`
		Truncated   bool   `json:"truncated"`
		Notes       string `json:"notes"`
		Commits     []struct {
			SHA     string `json:"sha"`
			Subject string `json:"subject"`
		} `json:"commits"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("failed to unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
	}

	if got.Since != "v1.0.0" {
		t.Fatalf("since = %q, want %q", got.Since, "v1.0.0")
	}
	if got.Until != "HEAD" {
		t.Fatalf("until = %q, want %q", got.Until, "HEAD")
	}
	if got.CommitCount != 2 {
		t.Fatalf("commitCount = %d, want %d", got.CommitCount, 2)
	}
	if got.Truncated {
		t.Fatalf("expected truncated=false")
	}
	if !strings.Contains(got.Notes, "feat: add thing") || !strings.Contains(got.Notes, "fix: bug") {
		t.Fatalf("expected notes to include commit subjects, got %q", got.Notes)
	}
	if len(got.Commits) != 2 {
		t.Fatalf("commits len = %d, want %d", len(got.Commits), 2)
	}
	if strings.TrimSpace(got.Commits[0].SHA) == "" {
		t.Fatalf("expected commit sha to be present")
	}
}

func TestReleaseNotesGenerate_MissingSinceIsUsage(t *testing.T) {
	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"release-notes", "generate"}, "1.0.0")
		if code != cmd.ExitUsage {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitUsage)
		}
	})
	if !strings.Contains(stderr, "one of --since-tag, --since-ref, --since-tag-pattern, or --max-commits is required") {
		t.Fatalf("expected stderr to contain missing since error, got %q", stderr)
	}
}

func TestReleaseNotesGenerate_RangeStartSelectors(t *testing.T) {
	unsetGitHookEnv(t)

	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	repo := initTempGitRepo(t)
	runGit(t, repo, "tag", "build-7", "HEAD~1")

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir repo error: %v", err)
	}

	const conflict = "--since-tag, --since-ref, --since-tag-pattern, and --max-commits are mutually exclusive"

	tests := []struct {
		name         string
		args         []string
		wantCode     int
		wantSince    string
		wantSubjects []string
		wantStderr   string
	}{
		{
			name:         "pattern picks newest matching tag and skips newer non-matching tag",
			args:         []string{"--since-tag-pattern", "v*"},
			wantCode:     cmd.ExitSuccess,
			wantSince:    "v1.0.0",
			wantSubjects: []string{"feat: add thing", "fix: bug"},
		},
		{
			name:       "pattern with no matching tag is an error",
			args:       []string{"--since-tag-pattern", "ios/*"},
			wantCode:   cmd.ExitError,
			wantStderr: `no tag matching "ios/*" is reachable from HEAD`,
		},
		{
			name:       "pattern with option-like until ref is an error",
			args:       []string{"--since-tag-pattern", "nomatch*", "--until-ref=--always"},
			wantCode:   cmd.ExitError,
			wantStderr: `no tag matching "nomatch*"`,
		},
		{
			name:       "max commits with option-like until ref is an error",
			args:       []string{"--max-commits", "5", "--until-ref=--all"},
			wantCode:   cmd.ExitError,
			wantStderr: "release-notes generate:",
		},
		{
			name:       "since ref with option-like value is an error",
			args:       []string{"--since-ref=--grep=feat"},
			wantCode:   cmd.ExitError,
			wantStderr: "release-notes generate:",
		},
		{
			name:         "max commits returns the last n commits",
			args:         []string{"--max-commits", "2"},
			wantCode:     cmd.ExitSuccess,
			wantSubjects: []string{"feat: add thing", "fix: bug"},
		},
		{
			name:         "max commits larger than history returns all commits",
			args:         []string{"--max-commits", "50"},
			wantCode:     cmd.ExitSuccess,
			wantSubjects: []string{"chore: initial", "feat: add thing", "fix: bug"},
		},
		{
			name:       "max commits zero is usage",
			args:       []string{"--max-commits", "0"},
			wantCode:   cmd.ExitUsage,
			wantStderr: "--max-commits must be greater than 0",
		},
		{
			name:       "pattern with since-tag is usage",
			args:       []string{"--since-tag-pattern", "v*", "--since-tag", "v1.0.0"},
			wantCode:   cmd.ExitUsage,
			wantStderr: conflict,
		},
		{
			name:       "max commits with since-ref is usage",
			args:       []string{"--max-commits", "2", "--since-ref", "HEAD~1"},
			wantCode:   cmd.ExitUsage,
			wantStderr: conflict,
		},
		{
			name:       "pattern with max commits is usage",
			args:       []string{"--since-tag-pattern", "v*", "--max-commits", "2"},
			wantCode:   cmd.ExitUsage,
			wantStderr: conflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var code int
			stdout, stderr := captureOutput(t, func() {
				args := append([]string{"release-notes", "generate", "--output", "json"}, tt.args...)
				code = cmd.Run(args, "1.0.0")
			})
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr=%q", code, tt.wantCode, stderr)
			}
			if tt.wantStderr != "" {
				if !strings.Contains(stderr, tt.wantStderr) {
					t.Fatalf("stderr = %q, want it to contain %q", stderr, tt.wantStderr)
				}
				return
			}

			var got struct {
				Since   string `json:"since"`
				Commits []struct {
					Subject string `json:"subject"`
				} `json:"commits"`
			}
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("failed to unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
			}
			if got.Since != tt.wantSince {
				t.Fatalf("since = %q, want %q", got.Since, tt.wantSince)
			}
			subjects := make([]string, 0, len(got.Commits))
			for _, c := range got.Commits {
				subjects = append(subjects, c.Subject)
			}
			if strings.Join(subjects, "|") != strings.Join(tt.wantSubjects, "|") {
				t.Fatalf("subjects = %q, want %q", subjects, tt.wantSubjects)
			}
		})
	}
}

func TestReleaseNotesGenerate_NotGitRepoReturnsError(t *testing.T) {
	unsetGitHookEnv(t)

	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	dir := t.TempDir()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir error: %v", err)
	}

	_, stderr := captureOutput(t, func() {
		code := cmd.Run([]string{"release-notes", "generate", "--since-ref", "HEAD~1"}, "1.0.0")
		if code != cmd.ExitError {
			t.Fatalf("exit code = %d, want %d", code, cmd.ExitError)
		}
	})
	if !strings.Contains(strings.ToLower(stderr), "not a git repository") {
		t.Fatalf("expected stderr to mention non-git repo, got %q", stderr)
	}
}

func TestReleaseNotesGenerate_TruncatesToMaxChars(t *testing.T) {
	unsetGitHookEnv(t)

	resetDefaultOutput(t)
	t.Setenv("ASC_DEFAULT_OUTPUT", "json")

	repo := initTempGitRepo(t)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd error: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir repo error: %v", err)
	}

	var code int
	stdout, stderr := captureOutput(t, func() {
		code = cmd.Run([]string{
			"release-notes", "generate",
			"--since-tag", "v1.0.0",
			"--max-chars", "10",
			"--output", "json",
		}, "1.0.0")
	})
	if code != cmd.ExitSuccess {
		t.Fatalf("exit code = %d, want %d; stderr=%q", code, cmd.ExitSuccess, stderr)
	}

	var got struct {
		Truncated bool   `json:"truncated"`
		Notes     string `json:"notes"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("failed to unmarshal stdout JSON: %v\nstdout=%q", err, stdout)
	}
	if !got.Truncated {
		t.Fatalf("expected truncated=true")
	}
	if utf8.RuneCountInString(got.Notes) > 10 {
		t.Fatalf("notes rune length = %d, want <= 10; notes=%q", utf8.RuneCountInString(got.Notes), got.Notes)
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "cmdtest@example.com")
	runGit(t, dir, "config", "user.name", "cmdtest")

	// Ensure the repo has at least one real file so tools that rely on working tree
	// state behave consistently across git versions.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatalf("write README error: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "chore: initial")
	runGit(t, dir, "tag", "v1.0.0")

	runGit(t, dir, "commit", "--allow-empty", "-m", "feat: add thing")
	runGit(t, dir, "commit", "--allow-empty", "-m", "fix: bug")

	return dir
}

func unsetGitHookEnv(t *testing.T) {
	t.Helper()

	// When `go test` runs under a git hook, git exports repository-scoped env vars.
	// Clear them so any git invocation in the tests/CLI uses the temp repo.
	keys := []string{
		"GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
	}

	original := map[string]string{}
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			original[k] = v
			_ = os.Unsetenv(k)
		}
	}
	t.Cleanup(func() {
		for _, k := range keys {
			if v, ok := original[k]; ok {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	c := exec.Command("git", args...)
	c.Dir = dir
	// Git sets GIT_DIR/GIT_WORK_TREE for hook processes. If `go test` runs under a
	// hook, these env vars can leak into this helper and cause our git commands
	// to operate on the outer repo instead of the temp repo.
	c.Env = cleanGitRepoEnv(os.Environ())
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(out))
	}
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
