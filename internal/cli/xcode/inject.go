package xcode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type xcodeInjectManifest struct {
	Values  map[string]any              `json:"values,omitempty"`
	Outputs []xcodeInjectManifestOutput `json:"outputs"`
}

type xcodeInjectManifestOutput struct {
	Type     string         `json:"type"`
	Path     string         `json:"path"`
	Source   string         `json:"source,omitempty"`
	Values   map[string]any `json:"values,omitempty"`
	Contents string         `json:"contents,omitempty"`
}

type xcodeInjectOptions struct {
	ManifestPath string
	SetValues    []string
	Overwrite    bool
	DryRun       bool
}

type xcodeInjectResult struct {
	ManifestPath string                  `json:"manifest_path"`
	DryRun       bool                    `json:"dry_run"`
	Outputs      []xcodeInjectFileResult `json:"outputs"`
}

type xcodeInjectFileResult struct {
	Type   string `json:"type"`
	Path   string `json:"path"`
	Source string `json:"source,omitempty"`
	Action string `json:"action"`
	Bytes  int64  `json:"bytes,omitempty"`
}

type xcodeInjectOutputPlan struct {
	Type    string
	Path    string
	Source  string
	Payload []byte
	// DisplayPath and DisplaySource report the manifest-relative form the
	// operator wrote, keeping the output contract stable while rooted I/O uses
	// the absolute Path and Source.
	DisplayPath   string
	DisplaySource string
}

func XcodeInjectCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode inject", flag.ExitOnError)

	manifestPath := fs.String("manifest", "", "Path to deployment metadata manifest JSON (required)")
	overwrite := fs.Bool("overwrite", false, "Replace existing output files")
	dryRun := fs.Bool("dry-run", false, "Validate and print the files that would be generated without writing")
	var setValues shared.MultiStringFlag
	fs.Var(&setValues, "set", "Override a manifest value as key=value (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "inject",
		ShortUsage: "asc xcode inject --manifest FILE [flags]",
		ShortHelp:  "Generate Xcode deployment metadata files from a manifest.",
		LongHelp: `Generate Xcode deployment metadata files from a manifest.

The manifest is JSON with top-level "values" and "outputs" fields. Outputs can
generate plist, json, or text files, and can copy declared assets such as app
icons into asset catalog paths. Relative output paths and copy sources resolve
from the manifest directory.

Manifest paths must stay inside the manifest project root: the enclosing
repository when one is found, the parent directory when the manifest lives in a
dot-directory such as .asc, or the manifest directory itself. Absolute paths,
volume or UNC-style paths, traversal that leaves the root, and symlinks that
resolve outside the root are rejected before anything is written.

String values support ${key} placeholders from manifest values and repeated
--set key=value overrides.

Example manifest:
  {
    "values": {
      "bundle_id": "com.example.app",
      "app_name": "Example",
      "version": "1.2.3",
      "build_number": "42"
    },
    "outputs": [
      {
        "type": "plist",
        "path": "Generated/Info.generated.plist",
        "values": {
          "CFBundleIdentifier": "${bundle_id}",
          "CFBundleDisplayName": "${app_name}",
          "CFBundleShortVersionString": "${version}",
          "CFBundleVersion": "${build_number}"
        }
      },
      {
        "type": "copy",
        "source": "Assets/AppIcon.appiconset/Contents.json",
        "path": "App/Assets.xcassets/AppIcon.appiconset/Contents.json"
      }
    ]
  }

Examples:
  asc xcode inject --manifest .asc/deployment.json --dry-run --output json
  asc xcode inject --manifest .asc/deployment.json --set version=1.2.4 --set build_number=43 --overwrite`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			result, err := runXcodeInject(xcodeInjectOptions{
				ManifestPath: strings.TrimSpace(*manifestPath),
				SetValues:    []string(setValues),
				Overwrite:    *overwrite,
				DryRun:       *dryRun,
			})
			if err != nil {
				var usageErr xcodeInjectUsageError
				if errors.As(err, &usageErr) {
					return shared.UsageError(usageErr.Error())
				}
				return fmt.Errorf("xcode inject: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"type", "action", "path", "source"}, xcodeInjectResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"type", "action", "path", "source"}, xcodeInjectResultRows(result))
					return nil
				},
			)
		},
	}
}

type xcodeInjectUsageError struct {
	message string
}

func (e xcodeInjectUsageError) Error() string {
	return e.message
}

func newXcodeInjectUsageError(format string, args ...any) error {
	return xcodeInjectUsageError{message: fmt.Sprintf(format, args...)}
}

func runXcodeInject(opts xcodeInjectOptions) (xcodeInjectResult, error) {
	opts.ManifestPath = strings.TrimSpace(opts.ManifestPath)
	if opts.ManifestPath == "" {
		return xcodeInjectResult{}, newXcodeInjectUsageError("--manifest is required")
	}

	manifest, err := readXcodeInjectManifest(opts.ManifestPath)
	if err != nil {
		return xcodeInjectResult{}, err
	}
	if len(manifest.Outputs) == 0 {
		return xcodeInjectResult{}, newXcodeInjectUsageError("manifest outputs must include at least one entry")
	}

	values := make(map[string]any, len(manifest.Values)+len(opts.SetValues))
	for key, value := range manifest.Values {
		values[key] = value
	}
	if err := applyXcodeInjectSetValues(values, opts.SetValues); err != nil {
		return xcodeInjectResult{}, err
	}

	root, err := newXcodeInjectRoot(opts.ManifestPath)
	if err != nil {
		return xcodeInjectResult{}, err
	}
	baseDir, err := xcodeInjectBaseDir(opts.ManifestPath)
	if err != nil {
		return xcodeInjectResult{}, err
	}
	if err := validateXcodeInjectOutputDestinations(root, baseDir, manifest.Outputs, opts.Overwrite); err != nil {
		return xcodeInjectResult{}, err
	}

	result := xcodeInjectResult{
		ManifestPath: opts.ManifestPath,
		DryRun:       opts.DryRun,
		Outputs:      make([]xcodeInjectFileResult, 0, len(manifest.Outputs)),
	}
	plans := make([]xcodeInjectOutputPlan, 0, len(manifest.Outputs))

	// Result paths keep the manifest-relative form the operator wrote, so a
	// relative --manifest keeps producing relative output paths.
	displayBase := filepath.Dir(opts.ManifestPath)
	for i, output := range manifest.Outputs {
		plan, err := planXcodeInjectOutput(root, baseDir, displayBase, values, output)
		if err != nil {
			return xcodeInjectResult{}, fmt.Errorf("output %d: %w", i+1, err)
		}
		plans = append(plans, plan)
	}

	for i, plan := range plans {
		fileResult, err := writeXcodeInjectBytes(root, plan, opts)
		if err != nil {
			return xcodeInjectResult{}, fmt.Errorf("output %d: %w", i+1, err)
		}
		result.Outputs = append(result.Outputs, fileResult)
	}

	return result, nil
}

// newXcodeInjectRoot anchors manifest-declared outputs and copy sources to the
// manifest's project root: the enclosing repository when one is found, the parent
// of a dot-directory manifest location such as .asc, or the manifest directory
// itself. Symlinked directories inside that tree remain supported because they
// are a common asset-catalog layout, but a symlink that resolves outside the root
// is refused.
func newXcodeInjectRoot(manifestPath string) (rootfs.Root, error) {
	rootDir, err := xcodeInjectRootDir(manifestPath)
	if err != nil {
		return rootfs.Root{}, err
	}
	root, err := rootfs.New(rootDir)
	if err != nil {
		return rootfs.Root{}, err
	}
	return root.AllowingInternalSymlinks(), nil
}

func xcodeInjectRootDir(manifestPath string) (string, error) {
	absolute, err := filepath.Abs(manifestPath)
	if err != nil {
		return "", err
	}
	manifestDir := filepath.Dir(absolute)
	if repositoryRoot := xcodeInjectRepositoryRoot(manifestDir); repositoryRoot != "" {
		return repositoryRoot, nil
	}
	if strings.HasPrefix(filepath.Base(manifestDir), ".") {
		if parent := filepath.Dir(manifestDir); parent != manifestDir {
			return parent, nil
		}
	}
	return manifestDir, nil
}

func xcodeInjectRepositoryRoot(dir string) string {
	current := dir
	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// xcodeInjectBaseDir is the directory manifest-relative paths resolve from.
func xcodeInjectBaseDir(manifestPath string) (string, error) {
	absolute, err := filepath.Abs(manifestPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absolute), nil
}

func validateXcodeInjectOutputDestinations(root rootfs.Root, baseDir string, outputs []xcodeInjectManifestOutput, overwrite bool) error {
	seen := map[string]int{}
	paths := make([]string, 0, len(outputs))
	pathKeys := make([]string, 0, len(outputs))
	for i, output := range outputs {
		if strings.TrimSpace(output.Path) == "" {
			continue
		}
		targetPath, err := resolveXcodeInjectPath(root, baseDir, output.Path)
		if err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		if err := validateXcodeInjectDestinationParents(root, targetPath); err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		if err := validateXcodeInjectDestination(targetPath, overwrite); err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		targetKey, err := xcodeInjectPathConflictKey(targetPath)
		if err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		if first, exists := seen[targetKey]; exists {
			return newXcodeInjectUsageError("duplicate output path %q in outputs %d and %d", targetPath, first+1, i+1)
		}
		seen[targetKey] = i
		for index, existingPath := range paths {
			existingKey := pathKeys[index]
			if xcodeInjectPathContains(existingKey, targetKey) {
				return newXcodeInjectUsageError("output path %q conflicts with nested output path %q", existingPath, targetPath)
			}
			if xcodeInjectPathContains(targetKey, existingKey) {
				return newXcodeInjectUsageError("output path %q conflicts with nested output path %q", targetPath, existingPath)
			}
		}
		paths = append(paths, targetPath)
		pathKeys = append(pathKeys, targetKey)
	}
	for i, output := range outputs {
		if strings.ToLower(strings.TrimSpace(output.Type)) != "copy" || strings.TrimSpace(output.Source) == "" {
			continue
		}
		sourcePath, err := resolveXcodeInjectPath(root, baseDir, output.Source)
		if err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		sourceKey, err := xcodeInjectPathConflictKey(sourcePath)
		if err != nil {
			return fmt.Errorf("output %d: %w", i+1, err)
		}
		if first, exists := seen[sourceKey]; exists {
			return newXcodeInjectUsageError("copy source %q in output %d is produced by output %d", sourcePath, i+1, first+1)
		}
	}
	return nil
}

func validateXcodeInjectDestinationParents(root rootfs.Root, path string) error {
	if err := root.CheckParents(path); err != nil {
		return err
	}

	parent := filepath.Dir(path)
	for {
		if parent == "." || parent == "" {
			return nil
		}
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return fmt.Errorf("output parent path %q is not a directory", parent)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return nil
		}
		parent = next
	}
}

func xcodeInjectPathConflictKey(path string) (string, error) {
	cleaned := filepath.Clean(path)
	current := filepath.Dir(cleaned)
	missing := []string{filepath.Base(cleaned)}
	for {
		if _, err := os.Lstat(current); err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			parts := append([]string{resolved}, missing...)
			return strings.ToLower(filepath.Clean(filepath.Join(parts...))), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(current)
		missing = append([]string{filepath.Base(current)}, missing...)
		if parent == current {
			return strings.ToLower(cleaned), nil
		}
		current = parent
	}
}

func xcodeInjectPathContains(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func readXcodeInjectManifest(path string) (xcodeInjectManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return xcodeInjectManifest{}, err
	}

	var manifest xcodeInjectManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return xcodeInjectManifest{}, newXcodeInjectUsageError("failed to parse --manifest: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return xcodeInjectManifest{}, newXcodeInjectUsageError("failed to parse --manifest: multiple JSON values")
	} else if !errors.Is(err, io.EOF) {
		return xcodeInjectManifest{}, newXcodeInjectUsageError("failed to parse --manifest: multiple JSON values")
	}
	return manifest, nil
}

func applyXcodeInjectSetValues(values map[string]any, setValues []string) error {
	for _, entry := range setValues {
		key, value, ok := strings.Cut(entry, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return newXcodeInjectUsageError("--set values must use key=value")
		}
		values[key] = strings.TrimSpace(value)
	}
	return nil
}

func planXcodeInjectOutput(root rootfs.Root, baseDir, displayBase string, values map[string]any, output xcodeInjectManifestOutput) (xcodeInjectOutputPlan, error) {
	outputType := strings.ToLower(strings.TrimSpace(output.Type))
	if strings.TrimSpace(output.Path) == "" {
		return xcodeInjectOutputPlan{}, newXcodeInjectUsageError("path is required")
	}
	targetPath, err := resolveXcodeInjectPath(root, baseDir, output.Path)
	if err != nil {
		return xcodeInjectOutputPlan{}, err
	}
	displayPath := xcodeInjectDisplayPath(displayBase, output.Path)

	switch outputType {
	case "plist":
		renderedValues, err := renderXcodeInjectValue(output.Values, values)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		payload, err := plist.Marshal(renderedValues, plist.XMLFormat)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		return xcodeInjectOutputPlan{Type: outputType, Path: targetPath, DisplayPath: displayPath, Payload: payload}, nil
	case "json":
		renderedValues, err := renderXcodeInjectValue(output.Values, values)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		payload, err := json.MarshalIndent(renderedValues, "", "  ")
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		payload = append(payload, '\n')
		return xcodeInjectOutputPlan{Type: outputType, Path: targetPath, DisplayPath: displayPath, Payload: payload}, nil
	case "text":
		renderedContents, err := renderXcodeInjectString(output.Contents, values)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		return xcodeInjectOutputPlan{Type: outputType, Path: targetPath, DisplayPath: displayPath, Payload: []byte(renderedContents)}, nil
	case "copy":
		if strings.TrimSpace(output.Source) == "" {
			return xcodeInjectOutputPlan{}, newXcodeInjectUsageError("source is required for copy outputs")
		}
		sourcePath, err := resolveXcodeInjectPath(root, baseDir, output.Source)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		payload, err := root.ReadFile(sourcePath)
		if err != nil {
			return xcodeInjectOutputPlan{}, err
		}
		return xcodeInjectOutputPlan{
			Type:          outputType,
			Path:          targetPath,
			Source:        sourcePath,
			Payload:       payload,
			DisplayPath:   displayPath,
			DisplaySource: xcodeInjectDisplayPath(displayBase, output.Source),
		}, nil
	default:
		return xcodeInjectOutputPlan{}, newXcodeInjectUsageError("type must be one of plist, json, text, copy")
	}
}

func renderXcodeInjectValue(value any, values map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		return renderXcodeInjectString(typed, values)
	case []any:
		rendered := make([]any, 0, len(typed))
		for _, item := range typed {
			renderedItem, err := renderXcodeInjectValue(item, values)
			if err != nil {
				return nil, err
			}
			rendered = append(rendered, renderedItem)
		}
		return rendered, nil
	case map[string]any:
		rendered := make(map[string]any, len(typed))
		for key, item := range typed {
			renderedItem, err := renderXcodeInjectValue(item, values)
			if err != nil {
				return nil, err
			}
			rendered[key] = renderedItem
		}
		return rendered, nil
	default:
		return value, nil
	}
}

func renderXcodeInjectString(input string, values map[string]any) (string, error) {
	rendered := input
	for range len(values) + 1 {
		before := rendered
		for _, key := range sortedXcodeInjectValueKeys(values) {
			placeholder := "${" + key + "}"
			if !strings.Contains(rendered, placeholder) {
				continue
			}
			rendered = strings.ReplaceAll(rendered, placeholder, fmt.Sprint(values[key]))
		}
		if !strings.Contains(rendered, "${") {
			return rendered, nil
		}
		if rendered == before {
			break
		}
	}
	if start := strings.Index(rendered, "${"); start != -1 {
		if end := strings.Index(rendered[start+2:], "}"); end != -1 {
			name := rendered[start+2 : start+2+end]
			return "", newXcodeInjectUsageError("missing value for placeholder %q", name)
		}
		return "", newXcodeInjectUsageError("unclosed placeholder in %q", rendered[start:])
	}
	return rendered, nil
}

func sortedXcodeInjectValueKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return len(keys[i]) > len(keys[j]) || (len(keys[i]) == len(keys[j]) && keys[i] < keys[j])
	})
	return keys
}

func writeXcodeInjectBytes(root rootfs.Root, plan xcodeInjectOutputPlan, opts xcodeInjectOptions) (xcodeInjectFileResult, error) {
	result := xcodeInjectFileResult{
		Type:   plan.Type,
		Path:   plan.DisplayPath,
		Source: plan.DisplaySource,
		Bytes:  int64(len(plan.Payload)),
	}
	if err := validateXcodeInjectDestination(plan.Path, opts.Overwrite); err != nil {
		return xcodeInjectFileResult{}, err
	}
	if opts.DryRun {
		if plan.Type == "copy" {
			result.Action = "would_copy"
		} else {
			result.Action = "would_write"
		}
		return result, nil
	}
	if _, err := root.WriteFrom(plan.Path, bytes.NewReader(plan.Payload), 0o644); err != nil {
		return xcodeInjectFileResult{}, err
	}
	if plan.Type == "copy" {
		result.Action = "copied"
	} else {
		result.Action = "written"
	}
	return result, nil
}

func validateXcodeInjectDestination(path string, overwrite bool) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !overwrite {
		return newXcodeInjectUsageError("output path %q already exists; use --overwrite", path)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to overwrite symlink %q", path)
	}
	if info.IsDir() {
		return fmt.Errorf("output path %q is a directory", path)
	}
	return nil
}

// xcodeInjectDisplayPath reports a manifest-declared path the way the operator
// wrote it: resolved from the manifest directory as typed, so a relative
// --manifest keeps relative result paths and an absolute one stays absolute.
func xcodeInjectDisplayPath(displayBase, path string) string {
	return filepath.Clean(filepath.Join(displayBase, path))
}

// resolveXcodeInjectPath resolves a manifest-declared path from the manifest
// directory and confirms it stays inside the manifest project root. Absolute
// paths, volume or UNC-style changes, and traversal that leaves the root are
// refused because the manifest is repository-controlled.
func resolveXcodeInjectPath(root rootfs.Root, baseDir string, path string) (string, error) {
	if path == "" {
		return "", newXcodeInjectUsageError("path is required")
	}
	if err := rootfs.ValidateRelativeAllowingTraversal(path); err != nil {
		return "", fmt.Errorf(
			"%w; manifest paths must be relative to the manifest directory and stay inside %s",
			err,
			root.Path(),
		)
	}
	resolved, err := root.Resolve(filepath.Clean(filepath.Join(baseDir, path)))
	if err != nil {
		return "", fmt.Errorf("%w; manifest paths must stay inside %s", err, root.Path())
	}
	return resolved, nil
}

func xcodeInjectResultRows(result xcodeInjectResult) [][]string {
	rows := make([][]string, 0, len(result.Outputs))
	for _, output := range result.Outputs {
		rows = append(rows, []string{output.Type, output.Action, output.Path, output.Source})
	}
	if len(rows) == 0 {
		return [][]string{{"", "", "", ""}}
	}
	return rows
}
