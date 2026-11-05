package docs

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestInitReference_RefusesSymlinkedASCReference(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.md")
	writeDocsContainmentFile(t, sentinelPath, "# Sentinel\n")

	if err := os.Symlink(sentinelPath, filepath.Join(repo, ascReferenceFile)); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	_, err := InitReference(InitOptions{Path: repo, Force: true, Link: false})
	if err == nil {
		t.Fatal("InitReference() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("InitReference() error = %v, want symlink rejection", err)
	}
	if got := readDocsContainmentFile(t, sentinelPath); got != "# Sentinel\n" {
		t.Fatalf("sentinel content = %q, want unchanged", got)
	}
}

func TestInitReference_AllowsASCReferenceSymlinkToContainedRegularFile(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	targetName := "ASC.actual.md"
	targetPath := filepath.Join(repo, targetName)
	writeDocsContainmentFile(t, targetPath, "# Existing\n")
	linkPath := filepath.Join(repo, ascReferenceFile)
	if err := os.Symlink(targetName, linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := InitReference(InitOptions{Path: repo, Force: true, Link: false})
	if err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}
	if !result.Overwritten {
		t.Fatalf("InitReference() result = %#v, want overwritten", result)
	}
	if got := readDocsContainmentFile(t, targetPath); !strings.Contains(got, "asc") {
		t.Fatalf("contained target content = %q, want generated reference", got)
	}
	if info, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("Lstat(ASC.md) error = %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("ASC.md mode = %v, want symlink preserved", info.Mode())
	}
}

func TestInitReference_AllowsNestedASCReferenceSymlinkToRepositoryRoot(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	docsDir := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(docs) error = %v", err)
	}
	targetPath := filepath.Join(repo, ascReferenceFile)
	writeDocsContainmentFile(t, targetPath, "# Existing\n")
	linkPath := filepath.Join(docsDir, ascReferenceFile)
	if err := os.Symlink(filepath.Join("..", ascReferenceFile), linkPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := InitReference(InitOptions{Path: linkPath, Force: true, Link: false})
	if err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}
	if !result.Overwritten {
		t.Fatalf("InitReference() result = %#v, want overwritten", result)
	}
	if got := readDocsContainmentFile(t, targetPath); !strings.Contains(got, "asc") {
		t.Fatalf("contained target content = %q, want generated reference", got)
	}
	if info, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("Lstat(docs/ASC.md) error = %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("docs/ASC.md mode = %v, want symlink preserved", info.Mode())
	}
}

func TestInitReference_RejectsASCReferenceSymlinkToRepositoryMetadata(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	configPath := filepath.Join(repo, ".git", "config")
	writeDocsContainmentFile(t, configPath, "[core]\n")
	if err := os.Symlink(filepath.Join(".git", "config"), filepath.Join(repo, ascReferenceFile)); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := InitReference(InitOptions{Path: repo, Force: true}); err == nil {
		t.Fatal("InitReference() error = nil, want repository-metadata rejection")
	}
	if got := readDocsContainmentFile(t, configPath); got != "[core]\n" {
		t.Fatalf(".git/config content = %q, want unchanged", got)
	}
}

func TestInitReference_RejectsDestinationAliasCollisionBeforeWriting(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	agentsPath := filepath.Join(repo, "AGENTS.md")
	writeDocsContainmentFile(t, agentsPath, "# Agents\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(repo, ascReferenceFile)); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := InitReference(InitOptions{Path: repo, Force: true, Link: true}); err == nil {
		t.Fatal("InitReference() error = nil, want alias collision rejection")
	}
	if got := readDocsContainmentFile(t, agentsPath); got != "# Agents\n" {
		t.Fatalf("AGENTS.md content = %q, want unchanged", got)
	}
}

func TestInitReference_AllowsAgentsSymlinkToContainedMarkdownFile(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	docsDir := filepath.Join(repo, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(docs) error = %v", err)
	}
	targetPath := filepath.Join(docsDir, "INSTRUCTIONS.md")
	writeDocsContainmentFile(t, targetPath, "# Shared agent instructions\n")
	agentsPath := filepath.Join(repo, "AGENTS.md")
	if err := os.Symlink(filepath.Join("docs", "INSTRUCTIONS.md"), agentsPath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := InitReference(InitOptions{Path: repo, Link: true})
	if err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}
	if len(result.Linked) != 1 || result.Linked[0] != agentsPath {
		t.Fatalf("InitReference() linked = %v, want AGENTS.md", result.Linked)
	}
	if got := readDocsContainmentFile(t, targetPath); !strings.Contains(got, "See `ASC.md`") {
		t.Fatalf("shared agent content = %q, want ASC.md reference", got)
	}
	if info, err := os.Lstat(agentsPath); err != nil {
		t.Fatalf("Lstat(AGENTS.md) error = %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("AGENTS.md mode = %v, want symlink preserved", info.Mode())
	}
}

func TestInitReference_RejectsAgentsSymlinkToRepositoryMetadata(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	targetPath := filepath.Join(repo, ".git", "INSTRUCTIONS.md")
	writeDocsContainmentFile(t, targetPath, "# Repository metadata\n")
	if err := os.Symlink(filepath.Join(".git", "INSTRUCTIONS.md"), filepath.Join(repo, "AGENTS.md")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	if _, err := InitReference(InitOptions{Path: repo, Link: true}); err == nil {
		t.Fatal("InitReference() error = nil, want repository-metadata rejection")
	}
	if got := readDocsContainmentFile(t, targetPath); got != "# Repository metadata\n" {
		t.Fatalf(".git/INSTRUCTIONS.md content = %q, want unchanged", got)
	}
	if _, err := os.Lstat(filepath.Join(repo, ascReferenceFile)); !os.IsNotExist(err) {
		t.Fatalf("ASC.md exists after failed preflight: %v", err)
	}
}

func TestInitReference_AllowsClaudeSymlinkToContainedAgentsFile(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	agentsPath := filepath.Join(repo, "AGENTS.md")
	writeDocsContainmentFile(t, agentsPath, "# Shared agent instructions\n")
	claudePath := filepath.Join(repo, "CLAUDE.md")
	if err := os.Symlink("AGENTS.md", claudePath); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	result, err := InitReference(InitOptions{Path: repo, Link: true})
	if err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}
	if len(result.Linked) != 2 {
		t.Fatalf("InitReference() linked = %v, want both logical agent files", result.Linked)
	}
	shared := readDocsContainmentFile(t, agentsPath)
	if !strings.Contains(shared, "See `ASC.md`") || !strings.Contains(shared, "@ASC.md") {
		t.Fatalf("shared agent content = %q, want both reference forms", shared)
	}
	if info, err := os.Lstat(claudePath); err != nil {
		t.Fatalf("Lstat(CLAUDE.md) error = %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CLAUDE.md mode = %v, want symlink preserved", info.Mode())
	}
}

func TestInitReference_RefusesLowercaseSymlinkedAgentsFile(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	sentinelPath := filepath.Join(t.TempDir(), "sentinel.md")
	writeDocsContainmentFile(t, sentinelPath, "# Sentinel\n")

	if err := os.Symlink(sentinelPath, filepath.Join(repo, "Agents.md")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}

	_, err := InitReference(InitOptions{Path: repo, Force: true, Link: true})
	if err == nil {
		t.Fatal("InitReference() error = nil, want symlink rejection")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("InitReference() error = %v, want symlink rejection", err)
	}
	if got := readDocsContainmentFile(t, sentinelPath); got != "# Sentinel\n" {
		t.Fatalf("sentinel content = %q, want unchanged", got)
	}
}

func TestInitReference_WritesNothingWhenAgentFileIsSymlinked(t *testing.T) {
	for _, test := range []struct {
		symlinked      string
		sibling        string
		siblingContent string
	}{
		{symlinked: "AGENTS.md", sibling: "CLAUDE.md", siblingContent: "# Claude\n"},
		{symlinked: "CLAUDE.md", sibling: "AGENTS.md", siblingContent: "# Agents\n"},
	} {
		t.Run(test.symlinked, func(t *testing.T) {
			repo := newDocsContainmentRepo(t)
			sentinelPath := filepath.Join(t.TempDir(), "sentinel.md")
			writeDocsContainmentFile(t, sentinelPath, "# Sentinel\n")
			writeDocsContainmentFile(t, filepath.Join(repo, test.sibling), test.siblingContent)

			if err := os.Symlink(sentinelPath, filepath.Join(repo, test.symlinked)); err != nil {
				t.Fatalf("Symlink() error = %v", err)
			}

			_, err := InitReference(InitOptions{Path: repo, Force: true, Link: true})
			if err == nil {
				t.Fatal("InitReference() error = nil, want symlink rejection")
			}
			if !strings.Contains(err.Error(), "symlink") {
				t.Fatalf("InitReference() error = %v, want symlink rejection", err)
			}
			if _, statErr := os.Lstat(filepath.Join(repo, ascReferenceFile)); !os.IsNotExist(statErr) {
				t.Fatalf("Lstat(ASC.md) error = %v, want IsNotExist: a failed init must not leave ASC.md behind", statErr)
			}
			if got := readDocsContainmentFile(t, filepath.Join(repo, test.sibling)); got != test.siblingContent {
				t.Fatalf("%s content = %q, want unchanged", test.sibling, got)
			}
			if got := readDocsContainmentFile(t, sentinelPath); got != "# Sentinel\n" {
				t.Fatalf("sentinel content = %q, want unchanged", got)
			}
		})
	}
}

func TestInitReference_PreflightsHardlinkedAgentFileBeforeWritingASC(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	agentsPath := filepath.Join(repo, "AGENTS.md")
	writeDocsContainmentFile(t, agentsPath, "# Agents\n")
	if err := os.Link(agentsPath, filepath.Join(t.TempDir(), "external-agent.md")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}

	if _, err := InitReference(InitOptions{Path: repo, Link: true}); err == nil {
		t.Fatal("InitReference() error = nil, want hard-link preflight rejection")
	}
	if _, err := os.Lstat(filepath.Join(repo, ascReferenceFile)); !os.IsNotExist(err) {
		t.Fatalf("ASC.md exists after failed preflight: %v", err)
	}
	if got := readDocsContainmentFile(t, agentsPath); got != "# Agents\n" {
		t.Fatalf("AGENTS.md content = %q, want unchanged", got)
	}
}

func TestInitReference_WritesOrdinaryRepositoryFiles(t *testing.T) {
	repo := newDocsContainmentRepo(t)
	writeDocsContainmentFile(t, filepath.Join(repo, "AGENTS.md"), "# Agents\n")
	writeDocsContainmentFile(t, filepath.Join(repo, "CLAUDE.md"), "# Claude\n")

	result, err := InitReference(InitOptions{Path: repo, Force: false, Link: true})
	if err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}
	if !result.Created {
		t.Fatalf("InitReference() result = %#v, want created", result)
	}
	if len(result.Linked) != 2 {
		t.Fatalf("InitReference() linked = %v, want AGENTS.md and CLAUDE.md", result.Linked)
	}
	if got := readDocsContainmentFile(t, filepath.Join(repo, ascReferenceFile)); !strings.Contains(got, "asc") {
		t.Fatalf("ASC.md content = %q", got)
	}
	if got := readDocsContainmentFile(t, filepath.Join(repo, "AGENTS.md")); !strings.Contains(got, "ASC.md") {
		t.Fatalf("AGENTS.md content = %q, want ASC.md reference", got)
	}
	if got := readDocsContainmentFile(t, filepath.Join(repo, "CLAUDE.md")); !strings.Contains(got, "@ASC.md") {
		t.Fatalf("CLAUDE.md content = %q, want @ASC.md directive", got)
	}

	// A rerun with --force must still overwrite the ordinary file in place.
	rerun, err := InitReference(InitOptions{Path: repo, Force: true, Link: true})
	if err != nil {
		t.Fatalf("InitReference() rerun error = %v", err)
	}
	if !rerun.Overwritten {
		t.Fatalf("InitReference() rerun result = %#v, want overwritten", rerun)
	}
}

func TestInitReference_PreservesExistingFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not reported faithfully on Windows")
	}
	repo := newDocsContainmentRepo(t)
	agentsPath := filepath.Join(repo, "AGENTS.md")
	writeDocsContainmentFile(t, agentsPath, "# Agents\n")
	if err := os.Chmod(agentsPath, 0o600); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}
	ascPath := filepath.Join(repo, ascReferenceFile)
	writeDocsContainmentFile(t, ascPath, "# Existing\n")
	if err := os.Chmod(ascPath, 0o600); err != nil {
		t.Fatalf("Chmod() error = %v", err)
	}

	if _, err := InitReference(InitOptions{Path: repo, Force: true, Link: true}); err != nil {
		t.Fatalf("InitReference() error = %v", err)
	}

	agentsInfo, err := os.Lstat(agentsPath)
	if err != nil {
		t.Fatalf("Lstat(AGENTS.md) error = %v", err)
	}
	if agentsInfo.Mode().Perm() != 0o600 {
		t.Fatalf("AGENTS.md mode = %v, want preserved 0600", agentsInfo.Mode().Perm())
	}
	ascInfo, err := os.Lstat(ascPath)
	if err != nil {
		t.Fatalf("Lstat(ASC.md) error = %v", err)
	}
	if ascInfo.Mode().Perm() != 0o600 {
		t.Fatalf("ASC.md mode = %v, want preserved 0600", ascInfo.Mode().Perm())
	}
}

func newDocsContainmentRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll(.git) error = %v", err)
	}
	return repo
}

func writeDocsContainmentFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func readDocsContainmentFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
