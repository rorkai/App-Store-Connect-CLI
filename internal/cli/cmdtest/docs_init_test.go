package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/docs"
)

func TestDocsInitCreatesReferenceAndLinks(t *testing.T) {
	runInitCreatesReferenceAndLinks(t, []string{"docs", "init"})
}

func TestInitCreatesReferenceAndLinks(t *testing.T) {
	runInitCreatesReferenceAndLinks(t, []string{"init"})
}

func runInitCreatesReferenceAndLinks(t *testing.T, args []string) {
	t.Helper()
	root := RootCommand("1.2.3")

	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	subDir := filepath.Join(repoRoot, "subdir")

	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("create repo root error: %v", err)
	}
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir error: %v", err)
	}

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	claudePath := filepath.Join(repoRoot, "CLAUDE.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md error: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("@Agents.md\n"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md error: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir error: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Path        string   `json:"path"`
		Created     bool     `json:"created"`
		Overwritten bool     `json:"overwritten"`
		Linked      []string `json:"linked"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	expectedPath := resolvePath(t, filepath.Join(repoRoot, "ASC.md"))
	actualPath := resolvePath(t, payload.Path)
	if actualPath != expectedPath {
		t.Fatalf("expected path %q, got %q", expectedPath, actualPath)
	}
	if !payload.Created {
		t.Fatal("expected created to be true")
	}
	if payload.Overwritten {
		t.Fatal("expected overwritten to be false")
	}

	linked := map[string]bool{}
	for _, path := range payload.Linked {
		linked[resolvePath(t, path)] = true
	}
	agentsResolved := resolvePath(t, agentsPath)
	claudeResolved := resolvePath(t, claudePath)
	if !linked[agentsResolved] || !linked[claudeResolved] {
		t.Fatalf("expected linked files to include %q and %q, got %v", agentsResolved, claudeResolved, payload.Linked)
	}

	ascData, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("read ASC.md error: %v", err)
	}
	if !strings.Contains(string(ascData), "# asc cli reference") {
		t.Fatalf("expected ASC.md to contain header, got %q", string(ascData))
	}
	if !strings.Contains(string(ascData), "Release (full pipeline)") || !strings.Contains(string(ascData), "asc release run") {
		t.Fatalf("expected ASC.md to contain release-first guidance, got %q", string(ascData))
	}
	if !strings.Contains(string(ascData), `asc status --app "APP_ID"`) {
		t.Fatalf("expected ASC.md to mention release status monitoring, got %q", string(ascData))
	}
	if !strings.Contains(string(ascData), "Submit for review (low-level)") || !strings.Contains(string(ascData), "asc submit create") {
		t.Fatalf("expected ASC.md to retain low-level submit guidance, got %q", string(ascData))
	}

	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md error: %v", err)
	}
	if !strings.Contains(string(agentsData), "ASC.md") {
		t.Fatalf("expected AGENTS.md to include ASC.md reference, got %q", string(agentsData))
	}

	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md error: %v", err)
	}
	if !strings.Contains(string(claudeData), "@ASC.md") {
		t.Fatalf("expected CLAUDE.md to include @ASC.md, got %q", string(claudeData))
	}
}

func TestDocsInitSubdirectoryPathUsesRelativeAgentLinks(t *testing.T) {
	runInitSubdirectoryPathUsesRelativeAgentLinks(t, []string{"docs", "init"})
}

func TestInitSubdirectoryPathUsesRelativeAgentLinks(t *testing.T) {
	runInitSubdirectoryPathUsesRelativeAgentLinks(t, []string{"init"})
}

func runInitSubdirectoryPathUsesRelativeAgentLinks(t *testing.T, args []string) {
	t.Helper()
	root := RootCommand("1.2.3")

	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	targetPath := filepath.Join(repoRoot, "subdir", "ASC.md")

	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("create repo root error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create target directory error: %v", err)
	}

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	claudePath := filepath.Join(repoRoot, "CLAUDE.md")
	if err := os.WriteFile(agentsPath, []byte("# AGENTS.md\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md error: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("@Agents.md\n"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md error: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir error: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	fullArgs := append(append([]string{}, args...), "--path", "./subdir/ASC.md")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(fullArgs); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Path    string   `json:"path"`
		Linked  []string `json:"linked"`
		Created bool     `json:"created"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}

	if resolvePath(t, payload.Path) != resolvePath(t, targetPath) {
		t.Fatalf("expected output path %q, got %q", targetPath, payload.Path)
	}
	if !payload.Created {
		t.Fatal("expected created to be true")
	}

	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md error: %v", err)
	}
	agentsContent := string(agentsData)
	if !strings.Contains(agentsContent, "See `subdir/ASC.md` for the command catalog and workflows.") {
		t.Fatalf("expected AGENTS.md to use subdirectory reference, got %q", agentsContent)
	}
	if strings.Contains(agentsContent, "See `ASC.md` for the command catalog and workflows.") {
		t.Fatalf("expected AGENTS.md not to use legacy root reference, got %q", agentsContent)
	}

	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md error: %v", err)
	}
	claudeContent := string(claudeData)
	if !strings.Contains(claudeContent, "@subdir/ASC.md") {
		t.Fatalf("expected CLAUDE.md to use subdirectory directive, got %q", claudeContent)
	}
	if strings.Contains(claudeContent, "@ASC.md") {
		t.Fatalf("expected CLAUDE.md not to use legacy root directive, got %q", claudeContent)
	}
}

func TestDocsInitRewritesLegacyRootAgentLinks(t *testing.T) {
	runInitRewritesLegacyRootAgentLinks(t, []string{"docs", "init"})
}

func TestInitRewritesLegacyRootAgentLinks(t *testing.T) {
	runInitRewritesLegacyRootAgentLinks(t, []string{"init"})
}

func runInitRewritesLegacyRootAgentLinks(t *testing.T, args []string) {
	t.Helper()
	root := RootCommand("1.2.3")

	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	targetPath := filepath.Join(repoRoot, "subdir", "ASC.md")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("create repo root error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create target directory error: %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("# Existing\n"), 0o644); err != nil {
		t.Fatalf("write ASC.md error: %v", err)
	}

	agentsPath := filepath.Join(repoRoot, "AGENTS.md")
	claudePath := filepath.Join(repoRoot, "CLAUDE.md")
	legacyAgents := "# AGENTS.md\n\n## ASC CLI Reference\n\nSee `ASC.md` for the command catalog and workflows.\n"
	if err := os.WriteFile(agentsPath, []byte(legacyAgents), 0o644); err != nil {
		t.Fatalf("write AGENTS.md error: %v", err)
	}
	if err := os.WriteFile(claudePath, []byte("@Agents.md\n@ASC.md\n"), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md error: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir error: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	fullArgs := append(append([]string{}, args...), "--path", "./subdir/ASC.md", "--force")
	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(fullArgs); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if err := root.Run(context.Background()); err != nil {
			t.Fatalf("run error: %v", err)
		}
	})
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var payload struct {
		Path        string `json:"path"`
		Overwritten bool   `json:"overwritten"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if resolvePath(t, payload.Path) != resolvePath(t, targetPath) {
		t.Fatalf("expected output path %q, got %q", targetPath, payload.Path)
	}
	if !payload.Overwritten {
		t.Fatal("expected overwritten to be true")
	}

	agentsData, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read AGENTS.md error: %v", err)
	}
	agentsContent := string(agentsData)
	if !strings.Contains(agentsContent, "See `subdir/ASC.md` for the command catalog and workflows.") {
		t.Fatalf("expected AGENTS.md to rewrite legacy link, got %q", agentsContent)
	}
	if strings.Contains(agentsContent, "See `ASC.md` for the command catalog and workflows.") {
		t.Fatalf("expected AGENTS.md not to contain legacy link, got %q", agentsContent)
	}

	claudeData, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read CLAUDE.md error: %v", err)
	}
	claudeContent := string(claudeData)
	if !strings.Contains(claudeContent, "@subdir/ASC.md") {
		t.Fatalf("expected CLAUDE.md to rewrite legacy directive, got %q", claudeContent)
	}
	if strings.Contains(claudeContent, "@ASC.md") {
		t.Fatalf("expected CLAUDE.md not to contain legacy directive, got %q", claudeContent)
	}
}

func TestDocsInitRequiresForceToOverwrite(t *testing.T) {
	runInitRequiresForceToOverwrite(t, []string{"docs", "init"})
}

func TestInitRequiresForceToOverwrite(t *testing.T) {
	runInitRequiresForceToOverwrite(t, []string{"init"})
}

func runInitRequiresForceToOverwrite(t *testing.T, args []string) {
	t.Helper()
	root := RootCommand("1.2.3")

	tempDir := t.TempDir()
	repoRoot := filepath.Join(tempDir, "repo")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("create repo root error: %v", err)
	}

	ascPath := filepath.Join(repoRoot, "ASC.md")
	if err := os.WriteFile(ascPath, []byte("# Existing\n"), 0o644); err != nil {
		t.Fatalf("write ASC.md error: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir error: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir error: %v", err)
	}

	stdout, stderr := captureOutput(t, func() {
		if err := root.Parse(args); err != nil {
			t.Fatalf("parse error: %v", err)
		}
		err := root.Run(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, docs.ErrASCReferenceExists) {
			t.Fatalf("expected ErrASCReferenceExists, got %v", err)
		}
	})

	if stdout != "" {
		t.Fatalf("expected empty stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestInitCommandsRejectInvalidOutputPath(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		precreateMD bool
	}{
		{
			name:        "docs init rejects existing non-ASC markdown path",
			args:        []string{"docs", "init", "--path", "README.md", "--force"},
			precreateMD: true,
		},
		{
			name:        "init rejects existing non-ASC markdown path",
			args:        []string{"init", "--path", "README.md", "--force"},
			precreateMD: true,
		},
		{
			name: "docs init rejects non-markdown file-like path",
			args: []string{"docs", "init", "--path", "notes.txt"},
		},
		{
			name: "init rejects non-markdown file-like path",
			args: []string{"init", "--path", "notes.txt"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			root := RootCommand("1.2.3")

			tempDir := t.TempDir()
			repoRoot := filepath.Join(tempDir, "repo")
			if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
				t.Fatalf("create repo root error: %v", err)
			}
			if tc.precreateMD {
				if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# Existing\n"), 0o644); err != nil {
					t.Fatalf("write README.md error: %v", err)
				}
			}

			originalWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("get working dir error: %v", err)
			}
			defer func() {
				_ = os.Chdir(originalWD)
			}()
			if err := os.Chdir(repoRoot); err != nil {
				t.Fatalf("chdir error: %v", err)
			}

			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(tc.args); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				err := root.Run(context.Background())
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !errors.Is(err, docs.ErrInvalidASCReferencePath) {
					t.Fatalf("expected ErrInvalidASCReferencePath, got %v", err)
				}
			})

			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func resolvePath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
