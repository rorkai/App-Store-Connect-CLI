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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
		"Create an ASC.md command reference for the asc cli in the current repo.",
		`Create an ASC.md command reference for the asc cli in the current repo.

Examples:
  asc docs init
  asc docs init --path ./ASC.md
  asc docs init --force --link=false`,
		"docs init",
	)
}

// InitReference generates ASC.md in the target repo and links agent files.
// Every validation, including the symlink containment checks on the ASC.md
// destination and the agent files, runs before the first write, so a failed
// init leaves the repository untouched.
func InitReference(opts InitOptions) (InitResult, error) {
	targetPath, linkRoot, err := resolveOutputPath(opts.Path)
	if err != nil {
		return InitResult{}, err
	}

	ascPlan, err := planASCReference(linkRoot, targetPath, opts.Force)
	if err != nil {
		return InitResult{}, err
	}

	linkPlan := agentLinkPlan{}
	if opts.Link {
		relRef, err := filepath.Rel(linkRoot, targetPath)
		if err != nil {
			relRef = ascReferenceFile
		}
		relRef = normalizeReferencePath(relRef)
		linkPlan, err = planAgentFileLinks(linkRoot, relRef)
		if err != nil {
			return InitResult{}, err
		}
	}
	if err := validateInitReferencePlan(ascPlan, linkPlan); err != nil {
		return InitResult{}, err
	}

	created, overwritten, err := writeASCReference(ascPlan)
	if err != nil {
		return InitResult{}, err
	}

	linked, err := applyAgentFileLinks(linkPlan)
	if err != nil {
		return InitResult{}, err
	}

	return InitResult{
		Path:        targetPath,
		Created:     created,
		Overwritten: overwritten,
		Linked:      linked,
	}, nil
}

func resolveOutputPath(path string) (string, string, error) {
	if path != "" {
		abs, err := filepath.Abs(path)
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
	normalized := filepath.ToSlash(path)
	if normalized == "" || normalized == "." {
		return ascReferenceFile
	}
	return normalized
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

const defaultDocsFileMode os.FileMode = 0o644

// ascReferencePlan is a fully validated, not-yet-applied ASC.md write.
type ascReferencePlan struct {
	root   rootfs.Root
	name   string
	exists bool
	perm   os.FileMode
}

// planASCReference validates the ASC.md destination without writing anything.
func planASCReference(rootDir, path string, force bool) (ascReferencePlan, error) {
	root, err := rootfs.New(rootDir)
	if err != nil {
		return ascReferencePlan{}, err
	}
	name, err := filepath.Rel(root.Path(), path)
	if err != nil {
		return ascReferencePlan{}, err
	}

	// Lstat, not Stat, so a dangling symlink still counts as an existing entry.
	// A final symlink may be followed only after proving that its physical target
	// is still a regular file beneath the repository root.
	exists := false
	perm := defaultDocsFileMode
	if info, err := os.Lstat(path); err == nil {
		exists = true
		if info.Mode().IsRegular() {
			// Preserve the operator's chosen mode when replacing the file.
			perm = info.Mode().Perm()
		} else if info.Mode()&os.ModeSymlink != 0 {
			name, err = root.ResolveContainedFinalSymlink(name)
			if err != nil {
				return ascReferencePlan{}, fmt.Errorf("%w: ASC.md target is not contained: %w", rootfs.ErrSymlink, err)
			}
			if err := validateContainedMarkdownTarget(name); err != nil {
				return ascReferencePlan{}, err
			}
			targetInfo, err := os.Lstat(filepath.Join(root.Path(), name))
			if err != nil {
				return ascReferencePlan{}, err
			}
			if !targetInfo.Mode().IsRegular() {
				return ascReferencePlan{}, fmt.Errorf("%q is not a regular file", path)
			}
			perm = targetInfo.Mode().Perm()
		}
	} else if !os.IsNotExist(err) {
		return ascReferencePlan{}, err
	}

	if exists && !force {
		return ascReferencePlan{}, fmt.Errorf("%w: %s (use --force to overwrite)", ErrASCReferenceExists, path)
	}

	// The name now denotes either the ordinary destination or the already
	// resolved target of a safe final symlink. The rooted write re-checks every
	// component at write time.
	if err := root.CheckContained(name); err != nil {
		return ascReferencePlan{}, err
	}

	return ascReferencePlan{root: root, name: name, exists: exists, perm: perm}, nil
}

func writeASCReference(plan ascReferencePlan) (bool, bool, error) {
	content := ascTemplate
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	if err := plan.root.WriteFilePreservingMode(plan.name, []byte(content), plan.perm); err != nil {
		return false, false, err
	}

	if plan.exists {
		return false, true, nil
	}
	return true, false, nil
}

// agentFileUpdate is one planned agent-file rewrite.
type agentFileUpdate struct {
	name        string
	content     string
	perm        os.FileMode
	reportPaths []string
}

// agentLinkPlan holds every agent-file update computed during planning.
type agentLinkPlan struct {
	root    rootfs.Root
	rootDir string
	updates []agentFileUpdate
}

// planAgentFileLinks computes every agent-file update up front, so a symlinked
// or unreadable agent file is rejected before anything is written.
func planAgentFileLinks(rootDir string, relRef string) (agentLinkPlan, error) {
	root, err := rootfs.New(rootDir)
	if err != nil {
		return agentLinkPlan{}, err
	}
	plan := agentLinkPlan{root: root, rootDir: rootDir}
	pending := make(map[string]agentFileUpdate)

	agentsName := "AGENTS.md"
	if !entryExists(filepath.Join(rootDir, agentsName)) {
		agentsName = "Agents.md"
	}
	agentsTarget, err := resolveContainedDocsEntry(root, agentsName)
	if err != nil {
		return agentLinkPlan{}, err
	}
	agentsContent, agentsFound, err := readPendingAgentFile(root, pending, agentsTarget)
	if err != nil {
		return agentLinkPlan{}, err
	}
	if agentsFound {
		agentsContent, agentsChanged, err := planAgentsLinkContent(agentsContent, relRef)
		if err != nil {
			return agentLinkPlan{}, err
		}
		if agentsChanged {
			update, err := plannedAgentFileUpdate(rootDir, agentsTarget, agentsContent)
			if err != nil {
				return agentLinkPlan{}, err
			}
			update.reportPaths = append(update.reportPaths, agentsName)
			pending[agentsTarget] = update
		}
	}

	claudeName := "CLAUDE.md"
	claudeTarget, err := resolveContainedDocsEntry(root, claudeName)
	if err != nil {
		return agentLinkPlan{}, err
	}
	claudeContent, claudeFound, err := readPendingAgentFile(root, pending, claudeTarget)
	if err != nil {
		return agentLinkPlan{}, err
	}
	if claudeFound {
		claudeContent, claudeChanged, err := planClaudeLinkContent(claudeContent, relRef)
		if err != nil {
			return agentLinkPlan{}, err
		}
		if claudeChanged {
			update, ok := pending[claudeTarget]
			if !ok {
				update, err = plannedAgentFileUpdate(rootDir, claudeTarget, claudeContent)
				if err != nil {
					return agentLinkPlan{}, err
				}
			} else {
				update.content = claudeContent
			}
			update.reportPaths = append(update.reportPaths, claudeName)
			pending[claudeTarget] = update
		}
	}

	for _, name := range []string{agentsTarget, claudeTarget} {
		update, ok := pending[name]
		if !ok {
			continue
		}
		plan.updates = append(plan.updates, update)
		delete(pending, name)
	}

	return plan, nil
}

// resolveContainedDocsEntry keeps ordinary files unchanged while permitting a
// final symlink whose resolved target remains inside the repository. External
// and dangling links are rejected.
func resolveContainedDocsEntry(root rootfs.Root, name string) (string, error) {
	if !entryExists(filepath.Join(root.Path(), name)) {
		return name, nil
	}
	if err := root.CheckContained(name); err == nil {
		return name, nil
	} else if !errors.Is(err, rootfs.ErrSymlink) {
		return "", err
	}
	target, err := root.ResolveContainedFinalSymlink(name)
	if err != nil {
		return "", fmt.Errorf("%w: agent-file target is not contained: %w", rootfs.ErrSymlink, err)
	}
	if err := validateContainedMarkdownTarget(target); err != nil {
		return "", err
	}
	return target, nil
}

func validateContainedMarkdownTarget(name string) error {
	if !strings.EqualFold(filepath.Ext(name), ".md") {
		return fmt.Errorf("%w: contained target %q is not a Markdown file", rootfs.ErrSymlink, name)
	}
	for _, component := range strings.FieldsFunc(filepath.Clean(name), func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		switch strings.ToLower(component) {
		case ".git", ".hg", ".svn":
			return fmt.Errorf("%w: contained target %q is repository metadata", rootfs.ErrSymlink, name)
		}
	}
	return nil
}

func validateInitReferencePlan(ascPlan ascReferencePlan, linkPlan agentLinkPlan) error {
	paths := make(map[string]string, len(linkPlan.updates)+1)
	check := func(root rootfs.Root, name, label string) error {
		if err := root.CheckWriteFilePreservingMode(name); err != nil {
			return fmt.Errorf("preflight %s: %w", label, err)
		}
		physicalRoot := filepath.Clean(root.Path())
		if resolvedRoot, err := filepath.EvalSymlinks(root.Path()); err == nil {
			physicalRoot = resolvedRoot
		}
		physicalPath := filepath.Join(physicalRoot, name)
		if previous, exists := paths[physicalPath]; exists {
			return fmt.Errorf("%s and %s resolve to the same destination %q", previous, label, physicalPath)
		}
		paths[physicalPath] = label
		return nil
	}

	if err := check(ascPlan.root, ascPlan.name, ascReferenceFile); err != nil {
		return err
	}
	for _, update := range linkPlan.updates {
		label := update.name
		if len(update.reportPaths) > 0 {
			label = strings.Join(update.reportPaths, "/")
		}
		if err := check(linkPlan.root, update.name, label); err != nil {
			return err
		}
	}
	return nil
}

func readPendingAgentFile(root rootfs.Root, pending map[string]agentFileUpdate, name string) (string, bool, error) {
	if update, ok := pending[name]; ok {
		return update.content, true, nil
	}
	data, found, err := root.ReadFileOptional(name)
	return string(data), found, err
}

// plannedAgentFileUpdate records the rewrite along with the file's current
// mode, so applying the plan preserves the operator's chosen permissions. The
// rooted read that produced content already rejected symlinked entries.
func plannedAgentFileUpdate(rootDir, name, content string) (agentFileUpdate, error) {
	update := agentFileUpdate{name: name, content: content, perm: defaultDocsFileMode}
	info, err := os.Lstat(filepath.Join(rootDir, name))
	switch {
	case err == nil:
		if info.Mode().IsRegular() {
			update.perm = info.Mode().Perm()
		}
	case !os.IsNotExist(err):
		return agentFileUpdate{}, err
	}
	return update, nil
}

func applyAgentFileLinks(plan agentLinkPlan) ([]string, error) {
	linked := []string{}
	for _, update := range plan.updates {
		if err := plan.root.WriteFilePreservingMode(update.name, []byte(update.content), update.perm); err != nil {
			return nil, err
		}
		for _, reportPath := range update.reportPaths {
			linked = append(linked, filepath.Join(plan.rootDir, reportPath))
		}
	}
	return linked, nil
}

// entryExists reports whether path exists without following a final symlink, so
// a symlinked agent file is still selected and then rejected by the rooted read.
func entryExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Lstat(path); err == nil {
		return true
	}
	return false
}

// planAgentsLink computes the updated AGENTS.md content without writing it.
func planAgentsLink(root rootfs.Root, name string, relRef string) (string, bool, error) {
	data, found, err := root.ReadFileOptional(name)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return planAgentsLinkContent(string(data), relRef)
}

func planAgentsLinkContent(content string, relRef string) (string, bool, error) {
	desiredLine := fmt.Sprintf("See `%s` for the command catalog and workflows.", relRef)

	lines := strings.Split(content, "\n")
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
			return "", false, nil
		}
		return plannedContent(content, strings.Join(lines, "\n"))
	}

	section := fmt.Sprintf("## asc cli reference\n\n%s", desiredLine)
	return plannedContent(content, appendSection(content, section))
}

// planClaudeLink computes the updated CLAUDE.md content without writing it.
func planClaudeLink(root rootfs.Root, name string, relRef string) (string, bool, error) {
	data, found, err := root.ReadFileOptional(name)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	return planClaudeLinkContent(string(data), relRef)
}

func planClaudeLinkContent(content string, relRef string) (string, bool, error) {
	desiredLine := "@" + relRef

	lines := strings.Split(content, "\n")
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
			return "", false, nil
		}
		return plannedContent(content, strings.Join(updatedLines, "\n"))
	}

	updated := strings.TrimRight(content, "\n")
	if updated != "" {
		updated += "\n"
	}
	updated += desiredLine + "\n"

	return plannedContent(content, updated)
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

// plannedContent reports updated as a pending write when it differs from the
// existing content.
func plannedContent(existing, updated string) (string, bool, error) {
	if existing == updated {
		return "", false, nil
	}
	return updated, true, nil
}
