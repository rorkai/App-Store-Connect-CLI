package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

type pathSource string

const (
	pathSourceFlag        pathSource = "flag"
	pathSourceDeliverfile pathSource = "deliverfile"
	pathSourceDefault     pathSource = "default"
)

type importInputOptions struct {
	WorkDir         string
	FastlaneDir     string
	MetadataDir     string
	ScreenshotsDir  string
	SkipScreenshots bool
	// MetadataOnly resolves the metadata directory alone. migrate validate
	// reads metadata files and binds no screenshot trust flag, so resolving a
	// screenshots directory it never opens would reject trees that migrate
	// import accepts.
	MetadataOnly bool
	// The allow options preserve legacy Fastlane layouts only when the operator
	// explicitly trusts paths that repository-controlled configuration can
	// redirect outside the selected checkout.
	AllowExternalMetadata     bool
	AllowExternalScreenshots  bool
	AllowSymlinkedDeliverfile bool
}

type importInputs struct {
	DeliverfilePath   string
	DeliverfileConfig DeliverfileConfig
	MetadataDir       string
	ScreenshotsDir    string
	MetadataSource    pathSource
	ScreenshotsSource pathSource
}

func resolveImportInputs(opts importInputOptions) (importInputs, []SkippedItem, error) {
	workDir := opts.WorkDir
	if strings.TrimSpace(workDir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return importInputs{}, nil, fmt.Errorf("resolve import paths: %w", err)
		}
		workDir = cwd
	}

	if opts.FastlaneDir != "" {
		if err := ensureDirExists(opts.FastlaneDir); err != nil {
			return importInputs{}, nil, err
		}
	}

	deliverfilePath, err := discoverDeliverfilePath(workDir, opts.FastlaneDir)
	if err != nil {
		return importInputs{}, nil, err
	}

	var config DeliverfileConfig
	if deliverfilePath != "" {
		config, err = readDeliverfileConfig(workDir, opts.FastlaneDir, deliverfilePath, opts.AllowSymlinkedDeliverfile)
		if err != nil {
			return importInputs{}, nil, err
		}
	}

	inputs := importInputs{
		DeliverfilePath:   deliverfilePath,
		DeliverfileConfig: config,
	}

	metadata, err := resolveImportPath(workDir, opts.FastlaneDir, deliverfilePath, opts.MetadataDir, config.MetadataPath, "metadata", "metadata_path", opts.AllowExternalMetadata)
	if err != nil {
		return importInputs{}, nil, err
	}
	skipScreenshots := opts.MetadataOnly || opts.SkipScreenshots || config.SkipScreenshots
	var screenshots resolvedImportPath
	if !opts.MetadataOnly {
		if skipScreenshots {
			screenshots, err = resolveSkippedImportPath(workDir, opts.FastlaneDir, deliverfilePath, opts.ScreenshotsDir, config.ScreenshotsPath, "screenshots", "screenshots_path")
		} else {
			screenshots, err = resolveImportPath(workDir, opts.FastlaneDir, deliverfilePath, opts.ScreenshotsDir, config.ScreenshotsPath, "screenshots", "screenshots_path", opts.AllowExternalScreenshots)
		}
		if err != nil {
			return importInputs{}, nil, err
		}
	}

	skipped := ignoredConfigKeyItems(deliverfilePath, config.IgnoredKeys)
	skipped = noteOverriddenConventionalDir(skipped, metadata)
	metadataDir, skipped, err := validateResolvedDir(metadata, skipped)
	if err != nil {
		return importInputs{}, nil, err
	}
	screenshotsDir := screenshots.path
	if !skipScreenshots {
		skipped = noteOverriddenConventionalDir(skipped, screenshots)
		screenshotsDir, skipped, err = validateResolvedDir(screenshots, skipped)
		if err != nil {
			return importInputs{}, nil, err
		}
	}

	inputs.MetadataDir = metadataDir
	inputs.ScreenshotsDir = screenshotsDir
	inputs.MetadataSource = metadata.source
	inputs.ScreenshotsSource = screenshots.source

	return inputs, skipped, nil
}

// resolveSkippedImportPath selects the path that would have supplied
// screenshots without touching it. A skipped stage still reports its selected
// path, but missing, external, or symlinked paths must not fail an import that
// will never open them.
func resolveSkippedImportPath(workDir, fastlaneDir, deliverfilePath, explicitPath, deliverfilePathValue, defaultDir, directive string) (resolvedImportPath, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return resolvedImportPath{path: explicitPath, source: pathSourceFlag, label: defaultDir}, nil
	}
	base := workDir
	if strings.TrimSpace(fastlaneDir) != "" {
		base = fastlaneDir
	}
	if deliverfilePath != "" {
		base = filepath.Dir(deliverfilePath)
	}
	if strings.TrimSpace(deliverfilePathValue) != "" {
		path := deliverfilePathValue
		if !filepath.IsAbs(path) {
			absoluteBase, err := filepath.Abs(base)
			if err != nil {
				return resolvedImportPath{}, err
			}
			path = filepath.Join(absoluteBase, path)
		}
		path = filepath.Clean(path)
		return resolvedImportPath{
			path:         path,
			source:       pathSourceDeliverfile,
			label:        defaultDir,
			directive:    directive,
			value:        deliverfilePathValue,
			conventional: overriddenConventionalDir(base, defaultDir, path),
		}, nil
	}
	if strings.TrimSpace(fastlaneDir) != "" {
		return resolvedImportPath{path: filepath.Join(fastlaneDir, defaultDir), source: pathSourceFlag, label: defaultDir}, nil
	}
	return resolvedImportPath{path: filepath.Join(base, defaultDir), source: pathSourceDefault, label: defaultDir}, nil
}

// resolvedImportPath records where an import directory came from so callers can
// report an overridden conventional directory and fail with the directive that
// selected a missing one.
type resolvedImportPath struct {
	path   string
	source pathSource
	label  string
	// directive and value describe the Deliverfile entry that selected the
	// path; they are empty for every other source.
	directive string
	value     string
	// conventional is the fastlane default directory that exists but was not
	// selected, empty when the default was chosen or does not exist.
	conventional string
}

func resolveImportPath(workDir, fastlaneDir, deliverfilePath, explicitPath, deliverfilePathValue, defaultDir, directive string, allowExternal bool) (resolvedImportPath, error) {
	if strings.TrimSpace(explicitPath) != "" {
		return resolvedImportPath{path: explicitPath, source: pathSourceFlag, label: defaultDir}, nil
	}
	base := workDir
	if strings.TrimSpace(fastlaneDir) != "" {
		base = fastlaneDir
	}
	if deliverfilePath != "" {
		base = filepath.Dir(deliverfilePath)
	}
	// A Deliverfile's own metadata_path/screenshots_path wins over the
	// conventional layout in both modes: --fastlane-dir selects which
	// Deliverfile is read, it does not discard the directives inside it.
	if strings.TrimSpace(deliverfilePathValue) != "" {
		root := workDir
		if strings.TrimSpace(fastlaneDir) != "" {
			root = fastlaneDir
		}
		resolved, err := containDeliverfilePath(root, base, deliverfilePathValue)
		if err != nil {
			if !allowExternal {
				return resolvedImportPath{}, err
			}
			resolved, err = resolveTrustedDeliverfilePath(base, deliverfilePathValue)
			if err != nil {
				return resolvedImportPath{}, err
			}
		}
		return resolvedImportPath{
			path:         resolved,
			source:       pathSourceDeliverfile,
			label:        defaultDir,
			directive:    directive,
			value:        deliverfilePathValue,
			conventional: overriddenConventionalDir(base, defaultDir, resolved),
		}, nil
	}
	if strings.TrimSpace(fastlaneDir) != "" {
		resolved, err := resolveFastlaneChild(fastlaneDir, defaultDir, allowExternal)
		if err != nil {
			return resolvedImportPath{}, err
		}
		return resolvedImportPath{path: resolved, source: pathSourceFlag, label: defaultDir}, nil
	}
	return resolvedImportPath{path: filepath.Join(base, defaultDir), source: pathSourceDefault, label: defaultDir}, nil
}

func overriddenConventionalDir(base, defaultDir, resolved string) string {
	conventional := filepath.Join(base, defaultDir)
	if conventional == filepath.Clean(resolved) {
		return ""
	}
	return conventional
}

// noteOverriddenConventionalDir reports a conventional fastlane directory that
// exists but is not read because the Deliverfile selected another one, so the
// precedence is visible instead of silently changing which files are published.
func noteOverriddenConventionalDir(skipped []SkippedItem, resolved resolvedImportPath) []SkippedItem {
	if resolved.source != pathSourceDeliverfile || resolved.conventional == "" {
		return skipped
	}
	if err := ensureDirExists(resolved.conventional); err != nil {
		return skipped
	}
	return append(skipped, SkippedItem{
		Path:   resolved.conventional,
		Reason: fmt.Sprintf("unused because Deliverfile %s %q selects another directory", resolved.directive, resolved.value),
	})
}

func resolveFastlaneChild(fastlaneDir, child string, allowExternal bool) (string, error) {
	resolved, err := containFastlaneChild(fastlaneDir, child)
	if err == nil {
		return resolved, nil
	}
	if !allowExternal {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Join(fastlaneDir, child))
}

func resolveTrustedDeliverfilePath(base, value string) (string, error) {
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(resolved), nil
}

// containFastlaneChild resolves a conventional child directory beneath the
// operator-selected fastlane root and refuses a symlinked child, because the
// fastlane checkout's contents are repository-controlled even when the root
// itself was chosen by the operator.
func containFastlaneChild(fastlaneDir, child string) (string, error) {
	root, err := rootfs.New(fastlaneDir)
	if err != nil {
		return "", err
	}
	if err := root.CheckContained(child); err != nil {
		return "", err
	}
	return filepath.Join(fastlaneDir, child), nil
}

// containDeliverfilePath resolves a repository-controlled Deliverfile path
// value against the Deliverfile's own directory and requires the result to stay
// inside the trusted root for the run (the working directory, or the selected
// Fastlane directory), because the Deliverfile ships with the checkout and must
// not select files outside it.
func containDeliverfilePath(rootPath, base, value string) (string, error) {
	if err := rootfs.ValidateRelativeAllowingTraversal(value); err != nil {
		return "", fmt.Errorf("deliverfile path %q: %w", value, err)
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return "", err
	}
	// The Deliverfile directory may itself be relative to the process working
	// directory; make it absolute so joining below cannot re-resolve it against
	// the trusted root and duplicate the leading components.
	absoluteBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	resolved, err := root.Resolve(filepath.Clean(filepath.Join(absoluteBase, value)))
	if err != nil {
		return "", fmt.Errorf("deliverfile path %q must stay inside %s: %w", value, root.Path(), err)
	}
	return resolved, nil
}

func validateResolvedDir(resolved resolvedImportPath, skipped []SkippedItem) (string, []SkippedItem, error) {
	if strings.TrimSpace(resolved.path) == "" {
		return "", skipped, nil
	}
	if err := ensureDirExists(resolved.path); err != nil {
		switch resolved.source {
		case pathSourceDefault:
			skipped = append(skipped, SkippedItem{
				Path:   resolved.path,
				Reason: fmt.Sprintf("default %s directory not found", resolved.label),
			})
			return "", skipped, nil
		case pathSourceDeliverfile:
			// Name the directive so an operator can tell a stale Deliverfile
			// path apart from a missing conventional directory.
			return "", skipped, fmt.Errorf("deliverfile %s %q resolves to %s: %w%s", resolved.directive, resolved.value, resolved.path, err, projectRootRelativeHint(resolved))
		default:
			return "", skipped, err
		}
	}
	return resolved.path, skipped, nil
}

// projectRootRelativeHint explains the common fastlane collision behind a
// missing Deliverfile directory: deliver resolves metadata_path and
// screenshots_path from the project root, so a Deliverfile inside fastlane/
// often carries "./fastlane/metadata", which lands one level too deep here
// while the conventional directory sits next to the Deliverfile. The hint is
// added only when that conventional directory exists, so a plain typo keeps the
// bare failure.
func projectRootRelativeHint(resolved resolvedImportPath) string {
	if resolved.conventional == "" || resolved.label == "" {
		return ""
	}
	if err := ensureDirExists(resolved.conventional); err != nil {
		return ""
	}
	return fmt.Sprintf(
		" (paths in a Deliverfile resolve relative to the Deliverfile; %s exists, so a project-root-relative value like %q should be %q)",
		resolved.conventional,
		"./fastlane/"+resolved.label,
		"./"+resolved.label,
	)
}

func discoverDeliverfilePath(workDir, fastlaneDir string) (string, error) {
	if strings.TrimSpace(fastlaneDir) != "" {
		path := filepath.Join(fastlaneDir, "Deliverfile")
		if exists, err := fileExists(path); err != nil {
			return "", err
		} else if exists {
			return path, nil
		}
		return "", nil
	}

	candidates := []string{
		filepath.Join(workDir, "Deliverfile"),
		filepath.Join(workDir, "fastlane", "Deliverfile"),
	}
	for _, path := range candidates {
		if exists, err := fileExists(path); err != nil {
			return "", err
		} else if exists {
			return path, nil
		}
	}
	return "", nil
}

// readDeliverfileConfig reads repository-controlled Deliverfiles through a
// rooted no-follow handle. The legacy symlink-following behavior remains
// available only behind an explicit trust decision.
func readDeliverfileConfig(workDir, fastlaneDir, path string, allowSymlink bool) (DeliverfileConfig, error) {
	if allowSymlink {
		return parseDeliverfile(path)
	}
	rootPath := workDir
	if strings.TrimSpace(fastlaneDir) != "" {
		rootPath = fastlaneDir
	}
	root, err := rootfs.New(rootPath)
	if err != nil {
		return DeliverfileConfig{}, err
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return DeliverfileConfig{}, err
	}
	relative, err := filepath.Rel(root.Path(), absolutePath)
	if err != nil {
		return DeliverfileConfig{}, err
	}
	file, err := root.OpenFile(relative)
	if err != nil {
		return DeliverfileConfig{}, fmt.Errorf("read Deliverfile: %w", err)
	}
	defer file.Close()
	return parseDeliverfileReader(path, file)
}

func ensureDirExists(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("expected directory: %s", path)
	}
	return nil
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.Mode().IsRegular(), nil
}
