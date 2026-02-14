package docs

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const ascReferenceFile = "ASC.md"

var (
	// ErrASCReferenceExists indicates ASC.md already exists and --force was not set.
	ErrASCReferenceExists = errors.New("ASC.md already exists")
	// ErrInvalidASCReferencePath indicates --path does not target ASC.md or a directory.
	ErrInvalidASCReferencePath = errors.New("path must target ASC.md or a directory")
)

// InitOptions controls ASC reference generation.
type InitOptions struct {
	Path  string
	Force bool
	Link  bool
}

// InitResult describes the output of an init run.
type InitResult struct {
	Path        string   `json:"path"`
	Created     bool     `json:"created"`
	Overwritten bool     `json:"overwritten"`
	Linked      []string `json:"linked,omitempty"`
}

// NewInitReferenceCommand builds an init-style command that writes ASC.md references.
func NewInitReferenceCommand(flagSetName, commandName, shortUsage, shortHelp, longHelp, errorPrefix string) *ffcli.Command {
	fs := flag.NewFlagSet(flagSetName, flag.ExitOnError)

	path := fs.String("path", "", "Output path for ASC.md (default: repo root or current directory)")
	force := fs.Bool("force", false, "Overwrite existing ASC.md")
	link := fs.Bool("link", true, "Update AGENTS.md and CLAUDE.md to reference ASC.md")

	return &ffcli.Command{
		Name:       commandName,
		ShortUsage: shortUsage,
		ShortHelp:  shortHelp,
		LongHelp:   longHelp,
		FlagSet:    fs,
		UsageFunc:  shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := InitReference(InitOptions{
				Path:  *path,
				Force: *force,
				Link:  *link,
			})
			if err != nil {
				return fmt.Errorf("%s: %w", errorPrefix, err)
			}
			return shared.PrintOutput(result, "json", false)
		},
	}
}

// DocsInitCommand returns the docs init subcommand.
func DocsInitCommand() *ffcli.Command {
	return NewInitReferenceCommand(
		"docs init",
		"init",
		"asc docs init [flags]",
		"Create an ASC.md command reference in the current repo.",
		`Create an ASC.md command reference in the current repo.

Examples:
  asc docs init
  asc docs init --path ./ASC.md
  asc docs init --force --link=false`,
		"docs init",
	)
}

// InitReference generates ASC.md in the target repo and links agent files.
func InitReference(opts InitOptions) (InitResult, error) {
	targetPath, linkRoot, err := resolveOutputPath(opts.Path)
	if err != nil {
		return InitResult{}, err
	}

	created, overwritten, err := writeASCReference(targetPath, opts.Force)
	if err != nil {
		return InitResult{}, err
	}

	linked := []string{}
	if opts.Link {
		relRef, err := filepath.Rel(linkRoot, targetPath)
		if err != nil {
			relRef = ascReferenceFile
		}
		relRef = normalizeReferencePath(relRef)
		linked, err = linkAgentFiles(linkRoot, relRef)
		if err != nil {
			return InitResult{}, err
		}
	}

	return InitResult{
		Path:        targetPath,
		Created:     created,
		Overwritten: overwritten,
		Linked:      linked,
	}, nil
}

func resolveOutputPath(path string) (string, string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed != "" {
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return "", "", err
		}
		targetPath := ""
		linkBase := ""
		if info, err := os.Stat(abs); err == nil {
			if info.IsDir() {
				targetPath = filepath.Join(abs, ascReferenceFile)
				linkBase = abs
			} else if looksLikeMarkdown(abs) {
				if !isASCReferencePath(abs) {
					return "", "", fmt.Errorf("%w: %s", ErrInvalidASCReferencePath, abs)
				}
				targetPath = abs
				linkBase = filepath.Dir(abs)
			} else {
				return "", "", fmt.Errorf("%w: %s is not a directory or markdown file", ErrInvalidASCReferencePath, abs)
			}
		} else if !os.IsNotExist(err) {
			return "", "", err
		} else if looksLikeMarkdown(abs) || hasFileExtension(abs) {
			if !isASCReferencePath(abs) {
				return "", "", fmt.Errorf("%w: %s", ErrInvalidASCReferencePath, abs)
			}
			targetPath = abs
			linkBase = filepath.Dir(abs)
		} else {
			targetPath = filepath.Join(abs, ascReferenceFile)
			linkBase = abs
		}
		root, err := findRepoRoot(linkBase)
		if err != nil {
			return "", "", err
		}
		if root == "" {
			root = linkBase
		}
		return targetPath, root, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	root, err := findRepoRoot(cwd)
	if err != nil {
		return "", "", err
	}
	if root == "" {
		root = cwd
	}
	return filepath.Join(root, ascReferenceFile), root, nil
}

func looksLikeMarkdown(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(strings.ToLower(base), ".md")
}

func hasFileExtension(path string) bool {
	return filepath.Ext(filepath.Base(path)) != ""
}

func isASCReferencePath(path string) bool {
	return strings.EqualFold(filepath.Base(path), ascReferenceFile)
}

func normalizeReferencePath(path string) string {
	trimmed := strings.TrimSpace(filepath.ToSlash(path))
	if trimmed == "" || trimmed == "." {
		return ascReferenceFile
	}
	return trimmed
}

func findRepoRoot(start string) (string, error) {
	dir := start
	for {
		if dir == "" {
			return "", nil
		}
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
}

func writeASCReference(path string, force bool) (bool, bool, error) {
	exists := false
	if _, err := os.Stat(path); err == nil {
		exists = true
	} else if !os.IsNotExist(err) {
		return false, false, err
	}

	if exists && !force {
		return false, false, fmt.Errorf("%w: %s (use --force to overwrite)", ErrASCReferenceExists, path)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, false, err
	}

	content := ascTemplate
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, false, err
	}

	if exists {
		return false, true, nil
	}
	return true, false, nil
}

func linkAgentFiles(root string, relRef string) ([]string, error) {
	linked := []string{}

	agentsPath := filepath.Join(root, "AGENTS.md")
	if !fileExists(agentsPath) {
		agentsPath = filepath.Join(root, "Agents.md")
	}
	agentsUpdated, err := updateAgentsLink(agentsPath, relRef)
	if err != nil {
		return nil, err
	}
	if agentsUpdated {
		linked = append(linked, agentsPath)
	}

	claudePath := filepath.Join(root, "CLAUDE.md")
	claudeUpdated, err := updateClaudeLink(claudePath, relRef)
	if err != nil {
		return nil, err
	}
	if claudeUpdated {
		linked = append(linked, claudePath)
	}

	return linked, nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err == nil {
		return true
	}
	return false
}

func updateAgentsLink(path string, relRef string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	desiredLine := fmt.Sprintf("See `%s` for the command catalog and workflows.", relRef)

	lines := strings.Split(string(data), "\n")
	foundReference := false
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isAgentsReferenceLine(trimmed) {
			continue
		}
		if foundReference {
			lines[i] = ""
			changed = true
			continue
		}
		foundReference = true
		if line != desiredLine {
			lines[i] = desiredLine
			changed = true
		}
	}
	if foundReference {
		if !changed {
			return false, nil
		}
		return writeIfChanged(path, strings.Join(lines, "\n"))
	}

	section := fmt.Sprintf("## ASC CLI Reference\n\n%s", desiredLine)
	updated := appendSection(string(data), section)
	return writeIfChanged(path, updated)
}

func updateClaudeLink(path string, relRef string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	desiredLine := "@" + relRef

	lines := strings.Split(string(data), "\n")
	updatedLines := make([]string, 0, len(lines))
	foundReference := false
	changed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !isASCReferenceDirective(trimmed) {
			updatedLines = append(updatedLines, line)
			continue
		}
		if foundReference {
			changed = true
			continue
		}
		foundReference = true
		if line != desiredLine {
			changed = true
		}
		updatedLines = append(updatedLines, desiredLine)
	}
	if foundReference {
		if !changed {
			return false, nil
		}
		return writeIfChanged(path, strings.Join(updatedLines, "\n"))
	}

	updated := strings.TrimRight(string(data), "\n")
	if updated != "" {
		updated += "\n"
	}
	updated += desiredLine + "\n"

	return writeIfChanged(path, updated)
}

func isAgentsReferenceLine(line string) bool {
	return strings.HasPrefix(line, "See `") &&
		strings.HasSuffix(line, "` for the command catalog and workflows.")
}

func isASCReferenceDirective(line string) bool {
	if !strings.HasPrefix(line, "@") {
		return false
	}
	ref := strings.TrimSpace(strings.TrimPrefix(line, "@"))
	return strings.EqualFold(filepath.Base(ref), ascReferenceFile)
}

func appendSection(content, section string) string {
	trimmed := strings.TrimRight(content, "\n")
	if trimmed == "" {
		return section + "\n"
	}
	return trimmed + "\n\n" + section + "\n"
}

func writeIfChanged(path, content string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if string(existing) == content {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return false, err
	}
	return true, nil
}
