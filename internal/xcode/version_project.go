package xcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/bitrise-io/go-plist"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcodeproj"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/secureopen"
)

const (
	marketingVersionSetting = "MARKETING_VERSION"
	currentProjectSetting   = "CURRENT_PROJECT_VERSION"
)

var (
	errStructuredVersionUnavailable = errors.New("structured Xcode version settings unavailable")
	errVersionSettingNotFound       = errors.New("version build setting not found")
)

type versionConfiguration struct {
	id              string
	target          string
	name            string
	defaultName     string
	projectLevel    bool
	buildSettings   serialized.Object
	baseReferenceID string
}

type structuredVersionProject struct {
	project        xcodeproj.XcodeProj
	projectPath    string
	pbxprojPath    string
	rootDir        string
	objects        serialized.Object
	configurations []*versionConfiguration
	parentByChild  map[string]string
}

type preparedVersionWrite struct {
	path             string
	original         []byte
	updated          []byte
	mode             os.FileMode
	root             rootfs.Root
	name             string
	ownsRoot         bool
	preserveMetadata bool
	// createOnly marks an artifact that must be published with no-replace
	// semantics. It is used for receipts, which are part of a multi-file
	// transaction but must never overwrite an operator or concurrent writer's
	// file.
	createOnly bool
	// createdIdentity identifies a create-only file after successful publication
	// using the descriptor retained by the rooted publication. Rollback must use
	// this token rather than reopening the receipt path.
	createdIdentity *rootfs.FileIdentity
	// committedIdentity identifies the inode installed by an ordinary write. It
	// is retained by the same Root and consumed by conditional rollback, so a
	// same-content replacement cannot be mistaken for our update.
	committedIdentity *rootfs.FileIdentity
	// committedInfo is the historical metadata snapshot returned by a stable
	// compatibility write. Strict signing writes use committedIdentity instead.
	committedInfo os.FileInfo
	// originalIdentity pins the source inode and bytes observed while preparing
	// an ordinary write. The commit path must use this token rather than a
	// caller-supplied os.FileInfo snapshot.
	originalIdentity *rootfs.FileIdentity
	// originalInfo retains the historical metadata snapshot used by stable
	// structured-version writes. Strict signing writes always use
	// originalIdentity and its bounded content snapshot instead.
	originalInfo os.FileInfo
	// strictIdentity selects the bounded descriptor-backed signing path. Stable
	// version edit/bump operations preserve their historical unrestricted
	// os.FileInfo compatibility behavior.
	strictIdentity bool
	// portableFallback preserves stable version-command writes on filesystems
	// without atomic no-replace rename. Signing transactions leave it disabled.
	portableFallback bool
}

type xcconfigMutation struct {
	setting        string
	value          string
	configurations []*versionConfiguration
}

// GetVersionScoped reads version values from a selected Xcode target and configuration.
func GetVersionScoped(ctx context.Context, opts GetVersionOptions) (*VersionInfo, error) {
	lookupSession, err := resolveBuildSettingsLookupSession(
		opts.BuildSettingsLookup,
		opts.BuildSettingsDiagnostic,
		opts.BuildSettingsSession,
	)
	if err != nil {
		return nil, err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	structured, err := project.hasStructuredVersionSettingsForView(opts.Target, opts.Configuration)
	if err != nil {
		return nil, err
	}
	if structured {
		return project.versionInfo(opts)
	}
	err = fmt.Errorf("%w: selected Xcode configuration does not resolve both %s and %s", errStructuredVersionUnavailable, marketingVersionSetting, currentProjectSetting)
	if strings.TrimSpace(opts.Configuration) != "" {
		return nil, fmt.Errorf("--configuration requires structured MARKETING_VERSION and CURRENT_PROJECT_VERSION settings: %w", err)
	}
	return getVersionLegacy(ctx, opts.ProjectDir, opts.Target, lookupSession)
}

// GetConsistentMarketingVersion resolves one marketing version across the full selected mutation scope.
func GetConsistentMarketingVersion(ctx context.Context, opts GetVersionOptions) (string, error) {
	lookupSession, err := resolveBuildSettingsLookupSession(
		opts.BuildSettingsLookup,
		opts.BuildSettingsDiagnostic,
		opts.BuildSettingsSession,
	)
	if err != nil {
		return "", err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return "", err
	}
	structured, err := project.hasStructuredSettingsForMutation(opts.Target, opts.Configuration, marketingVersionSetting)
	if err != nil {
		return "", err
	}
	if structured {
		return project.bumpBaseline(BumpVersionOptions{
			ProjectDir:    opts.ProjectDir,
			Target:        opts.Target,
			Configuration: opts.Configuration,
		}, marketingVersionSetting, true)
	}
	err = fmt.Errorf("%w: selected Xcode configurations do not resolve both %s and %s", errStructuredVersionUnavailable, marketingVersionSetting, currentProjectSetting)
	if strings.TrimSpace(opts.Configuration) != "" {
		return "", fmt.Errorf("--configuration requires structured %s settings: %w", marketingVersionSetting, err)
	}
	legacy, err := getVersionLegacy(ctx, opts.ProjectDir, opts.Target, lookupSession)
	if err != nil {
		return "", err
	}
	return legacy.Version, nil
}

func openStructuredVersionProject(projectInput string) (*structuredVersionProject, error) {
	return openStructuredVersionProjectWithPolicy(projectInput, false)
}

func openSigningStructuredVersionProject(projectInput string) (*structuredVersionProject, error) {
	return openStructuredVersionProjectWithPolicy(projectInput, true)
}

func openStructuredVersionProjectWithPolicy(projectInput string, signingSafety bool) (*structuredVersionProject, error) {
	projectPath, err := findXcodeproj(projectInput)
	if err != nil {
		return nil, err
	}
	if signingSafety {
		if err := validateStructuredVersionProjectPath(projectPath); err != nil {
			return nil, err
		}
	}
	parsed, err := xcodeproj.Open(projectPath)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Join(projectPath, "project.pbxproj"), err)
	}
	objects, err := parsed.RawProj.Object("objects")
	if err != nil {
		return nil, fmt.Errorf("parse Xcode project objects: %w", err)
	}
	absolutePath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, fmt.Errorf("resolve Xcode project path: %w", err)
	}
	project := &structuredVersionProject{
		project:       parsed,
		projectPath:   absolutePath,
		pbxprojPath:   filepath.Join(absolutePath, "project.pbxproj"),
		rootDir:       filepath.Dir(absolutePath),
		objects:       objects,
		parentByChild: make(map[string]string),
	}
	project.indexGroupParents()
	if err := project.indexConfigurations(); err != nil {
		return nil, err
	}
	return project, nil
}

// validateStructuredVersionProjectPath performs the signing-only rooted
// no-follow checks before the plist parser opens project.pbxproj. The parser
// accepts ordinary filesystem paths and would otherwise follow an explicitly
// selected .xcodeproj or project.pbxproj symlink before signing's later
// mutation preflight had a chance to reject it. Stable version commands retain
// their historical selected-path symlink behavior through openStructuredVersionProject.
func validateStructuredVersionProjectPath(projectPath string) error {
	absolute, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("resolve Xcode project path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	root, err := rootfs.New(filepath.Dir(absolute))
	if err != nil {
		return fmt.Errorf("open Xcode project root: %w", err)
	}
	defer root.Close()
	if err := root.CheckContained(absolute); err != nil {
		return fmt.Errorf("refusing to parse Xcode project %s: %w", absolute, err)
	}
	pbxprojPath := filepath.Join(absolute, "project.pbxproj")
	if err := root.CheckContained(pbxprojPath); err != nil {
		return fmt.Errorf("refusing to parse Xcode project file %s: %w", pbxprojPath, err)
	}
	return nil
}

func (project *structuredVersionProject) hasStructuredVersionSettingsForView(target, configuration string) (bool, error) {
	selected, err := project.selectViewConfiguration(target, configuration)
	if err != nil {
		return false, err
	}
	return project.configurationsResolveStructuredSettings([]*versionConfiguration{selected})
}

func (project *structuredVersionProject) hasStructuredVersionSettingsForMutation(target, configuration string) (bool, error) {
	return project.hasStructuredSettingsForMutation(target, configuration, marketingVersionSetting, currentProjectSetting)
}

func (project *structuredVersionProject) hasStructuredSettingsForMutation(target, configuration string, settings ...string) (bool, error) {
	anyProjectSetting, err := project.hasAnyStructuredSetting(settings...)
	if err != nil {
		return false, err
	}
	if !anyProjectSetting {
		return false, nil
	}
	selected, err := project.selectMutationConfigurations(target, configuration)
	if err != nil {
		return false, err
	}
	anyDefined := false
	allDefined := len(settings) > 0
	for _, candidate := range effectiveMutationConfigurations(selected) {
		for _, setting := range settings {
			defined, err := project.configurationDefinesSetting(candidate, setting)
			if err != nil {
				return false, err
			}
			anyDefined = anyDefined || defined
			allDefined = allDefined && defined
		}
	}
	if anyDefined && !allDefined {
		return false, fmt.Errorf(
			"selected Xcode configurations only partially define structured build settings (%s); narrow the scope or edit structured and legacy settings separately",
			strings.Join(settings, ", "),
		)
	}
	return allDefined, nil
}

func (project *structuredVersionProject) hasAnyStructuredSetting(settings ...string) (bool, error) {
	var firstInspectionError error
	for _, configuration := range project.configurations {
		if configuration.projectLevel {
			continue
		}
		for _, setting := range settings {
			defined, err := project.configurationDefinesSetting(configuration, setting)
			if err != nil {
				if firstInspectionError == nil {
					firstInspectionError = err
				}
				continue
			}
			if defined {
				return true, nil
			}
		}
	}
	if firstInspectionError != nil {
		return false, firstInspectionError
	}
	return false, nil
}

func (project *structuredVersionProject) configurationDefinesSetting(configuration *versionConfiguration, setting string) (bool, error) {
	if len(matchingBuildSettingKeys(configuration.buildSettings, setting)) > 0 {
		return true, nil
	}
	if configuration.baseReferenceID != "" {
		root, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return false, err
		}
		paths, err := collectStableXCConfigFiles(root)
		if err != nil {
			return false, err
		}
		defining, err := xcconfigFilesDefining(paths, setting)
		if err != nil {
			return false, err
		}
		if len(defining) > 0 {
			return true, nil
		}
	}
	if !configuration.projectLevel {
		if fallback := project.projectConfiguration(configuration.name); fallback != nil {
			return project.configurationDefinesSetting(fallback, setting)
		}
	}
	return false, nil
}

func (project *structuredVersionProject) configurationsResolveStructuredSettings(configurations []*versionConfiguration) (bool, error) {
	for _, configuration := range configurations {
		for _, setting := range []string{marketingVersionSetting, currentProjectSetting} {
			if _, _, err := project.resolveSetting(configuration, setting); err != nil {
				if errors.Is(err, errVersionSettingNotFound) {
					return false, nil
				}
				return false, fmt.Errorf("resolve %s for target %q configuration %q: %w", setting, configuration.target, configuration.name, err)
			}
		}
	}
	return len(configurations) > 0, nil
}

func (project *structuredVersionProject) indexConfigurations() error {
	appendConfigurations := func(target, defaultName string, projectLevel bool, configurations []xcodeproj.BuildConfiguration) error {
		for _, configuration := range configurations {
			raw, err := project.objects.Object(configuration.ID)
			if err != nil {
				return fmt.Errorf("read build configuration %s: %w", configuration.ID, err)
			}
			baseReferenceID, err := raw.String("baseConfigurationReference")
			if err != nil && !serialized.IsKeyNotFoundError(err) {
				return fmt.Errorf("read base configuration for %s: %w", configuration.Name, err)
			}
			project.configurations = append(project.configurations, &versionConfiguration{
				id:              configuration.ID,
				target:          target,
				name:            configuration.Name,
				defaultName:     defaultName,
				projectLevel:    projectLevel,
				buildSettings:   configuration.BuildSettings,
				baseReferenceID: baseReferenceID,
			})
		}
		return nil
	}

	if err := appendConfigurations("", project.project.Proj.BuildConfigurationList.DefaultConfigurationName, true, project.project.Proj.BuildConfigurationList.BuildConfigurations); err != nil {
		return err
	}
	for _, target := range project.project.Proj.Targets {
		if err := appendConfigurations(target.Name, target.BuildConfigurationList.DefaultConfigurationName, false, target.BuildConfigurationList.BuildConfigurations); err != nil {
			return err
		}
	}
	return nil
}

func (project *structuredVersionProject) indexGroupParents() {
	for parentID, value := range project.objects {
		object, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		children, ok := object["children"].([]interface{})
		if !ok {
			continue
		}
		for _, child := range children {
			childID, ok := child.(string)
			if ok {
				project.parentByChild[childID] = parentID
			}
		}
	}
}

func (project *structuredVersionProject) versionInfo(opts GetVersionOptions) (*VersionInfo, error) {
	configuration, err := project.selectViewConfiguration(opts.Target, opts.Configuration)
	if err != nil {
		return nil, err
	}
	version, versionSource, err := project.resolveSetting(configuration, marketingVersionSetting)
	if err != nil {
		return nil, fmt.Errorf("resolve %s for target %q configuration %q: %w", marketingVersionSetting, configuration.target, configuration.name, err)
	}
	build, buildSource, err := project.resolveSetting(configuration, currentProjectSetting)
	if err != nil {
		return nil, fmt.Errorf("resolve %s for target %q configuration %q: %w", currentProjectSetting, configuration.target, configuration.name, err)
	}
	return &VersionInfo{
		Version:           version,
		BuildNumber:       build,
		ProjectDir:        project.rootDir,
		Target:            configuration.target,
		Configuration:     configuration.name,
		VersionSource:     versionSource,
		BuildNumberSource: buildSource,
		Modern:            true,
	}, nil
}

func (project *structuredVersionProject) selectViewConfiguration(targetName, configurationName string) (*versionConfiguration, error) {
	targetName = strings.TrimSpace(targetName)
	configurationName = strings.TrimSpace(configurationName)
	if targetName == "" {
		var appTargets []string
		for _, target := range project.project.Proj.Targets {
			if target.IsAppProduct() {
				appTargets = append(appTargets, target.Name)
			}
		}
		if len(appTargets) == 1 {
			targetName = appTargets[0]
		} else {
			var targetNames []string
			for _, target := range project.project.Proj.Targets {
				targetNames = append(targetNames, target.Name)
			}
			if len(targetNames) == 1 {
				targetName = targetNames[0]
			} else {
				sort.Strings(targetNames)
				return nil, fmt.Errorf("multiple Xcode targets found (%s); use --target", strings.Join(targetNames, ", "))
			}
		}
	}

	var candidates []*versionConfiguration
	for _, configuration := range project.configurations {
		if configuration.projectLevel || configuration.target != targetName {
			continue
		}
		candidates = append(candidates, configuration)
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("target %q not found", targetName)
	}
	if configurationName == "" {
		configurationName = candidates[0].defaultName
		if configurationName == "" && len(candidates) == 1 {
			configurationName = candidates[0].name
		}
		if configurationName == "" {
			return nil, fmt.Errorf("multiple configurations found for target %q; use --configuration", targetName)
		}
	}
	for _, configuration := range candidates {
		if configuration.name == configurationName {
			return configuration, nil
		}
	}
	return nil, fmt.Errorf("configuration %q not found for target %q", configurationName, targetName)
}

func (project *structuredVersionProject) selectMutationConfigurations(targetName, configurationName string) ([]*versionConfiguration, error) {
	targetName = strings.TrimSpace(targetName)
	configurationName = strings.TrimSpace(configurationName)
	if targetName != "" {
		found := false
		for _, target := range project.project.Proj.Targets {
			if target.Name == targetName {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("target %q not found", targetName)
		}
	}

	var selected []*versionConfiguration
	for _, configuration := range project.configurations {
		if targetName != "" && (configuration.projectLevel || configuration.target != targetName) {
			continue
		}
		if configurationName != "" && configuration.name != configurationName {
			continue
		}
		selected = append(selected, configuration)
	}
	if len(selected) == 0 {
		if targetName != "" {
			return nil, fmt.Errorf("configuration %q not found for target %q", configurationName, targetName)
		}
		return nil, fmt.Errorf("configuration %q not found", configurationName)
	}
	return selected, nil
}

func (project *structuredVersionProject) resolveSetting(configuration *versionConfiguration, setting string) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return project.expandDirectSetting(configuration, setting, value, map[string]bool{setting: true})
	}
	if configuration.baseReferenceID != "" {
		path, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := project.resolveConfigurationXCConfig(configuration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := project.expandSettingReferences(configuration, resolved.value, map[string]bool{setting: true})
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if projectConfiguration := project.projectConfiguration(configuration.name); projectConfiguration != nil {
			return project.resolveSetting(projectConfiguration, setting)
		}
	}
	return "", "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func directBuildSetting(settings serialized.Object, setting string) (string, bool, error) {
	keys := matchingBuildSettingKeys(settings, setting)
	if value, ok := settings[setting].(string); ok {
		for _, key := range keys {
			if key == setting {
				continue
			}
			conditionalValue, conditionalOK := settings[key].(string)
			if !conditionalOK || strings.TrimSpace(conditionalValue) != strings.TrimSpace(value) {
				return "", false, fmt.Errorf(
					"%s has differing conditional build setting %s=%q (unconditional value %q); narrow the scope or use Xcode-aware resolution",
					setting, key, conditionalValue, value,
				)
			}
		}
		return value, true, nil
	}
	if len(keys) > 0 {
		return "", false, fmt.Errorf(
			"%s is defined only by conditional build settings (%s); SDK-aware resolution requires Xcode",
			setting, strings.Join(keys, ", "),
		)
	}
	return "", false, nil
}

func (project *structuredVersionProject) expandDirectSetting(
	configuration *versionConfiguration,
	setting string,
	value string,
	stack map[string]bool,
) (string, string, error) {
	if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
		inherited, _, err := project.resolveLowerSetting(configuration, setting)
		if err != nil {
			return "", "", fmt.Errorf("resolve inherited %s: %w", setting, err)
		}
		value = strings.ReplaceAll(value, "$(inherited)", inherited)
		value = strings.ReplaceAll(value, "${inherited}", inherited)
	}
	return project.expandSettingReferences(configuration, value, stack)
}

func (project *structuredVersionProject) resolveLowerSetting(configuration *versionConfiguration, setting string) (string, string, error) {
	if configuration.baseReferenceID != "" {
		path, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := project.resolveConfigurationXCConfig(configuration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := project.expandSettingReferences(configuration, resolved.value, map[string]bool{setting: true})
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if fallback := project.projectConfiguration(configuration.name); fallback != nil {
			return project.resolveSetting(fallback, setting)
		}
	}
	return "", "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func (project *structuredVersionProject) expandSettingReferences(configuration *versionConfiguration, value string, stack map[string]bool) (string, string, error) {
	referencePattern := regexp.MustCompile(`\$\(([^):]+)(?::[^)]*)?\)|\$\{([^}:]+)(?::[^}]*)?\}`)
	resolved := value
	for iteration := 0; iteration < 32; iteration++ {
		match := referencePattern.FindStringSubmatchIndex(resolved)
		if match == nil {
			return strings.TrimSpace(resolved), project.pbxprojPath, nil
		}
		name := ""
		if match[2] >= 0 {
			name = resolved[match[2]:match[3]]
		} else {
			name = resolved[match[4]:match[5]]
		}
		if stack[name] {
			return "", "", fmt.Errorf("build-setting reference cycle at %s", name)
		}
		nextStack := make(map[string]bool, len(stack)+1)
		for key, set := range stack {
			nextStack[key] = set
		}
		nextStack[name] = true
		replacement, _, err := project.resolveSettingReference(configuration, name, nextStack)
		if err != nil {
			return "", "", fmt.Errorf("unresolved build-setting reference %s", name)
		}
		resolved = resolved[:match[0]] + replacement + resolved[match[1]:]
	}
	return "", "", fmt.Errorf("too many nested build-setting references")
}

func (project *structuredVersionProject) resolveSettingReference(configuration *versionConfiguration, setting string, stack map[string]bool) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return project.expandDirectSetting(configuration, setting, value, stack)
	}
	if configuration.baseReferenceID != "" {
		path, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := project.resolveConfigurationXCConfig(configuration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := project.expandSettingReferences(configuration, resolved.value, stack)
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if fallback := project.projectConfiguration(configuration.name); fallback != nil {
			return project.resolveSettingReference(fallback, setting, stack)
		}
	}
	return "", "", fmt.Errorf("setting not found")
}

func (project *structuredVersionProject) projectConfiguration(name string) *versionConfiguration {
	for _, configuration := range project.configurations {
		if configuration.projectLevel && configuration.name == name {
			return configuration
		}
	}
	return nil
}

func (project *structuredVersionProject) resolveConfigurationXCConfig(
	configuration *versionConfiguration,
	path string,
	setting string,
) (xcconfigResolvedValue, error) {
	base := xcconfigResolvedValue{}
	if !configuration.projectLevel {
		if fallback := project.projectConfiguration(configuration.name); fallback != nil {
			value, source, err := project.resolveSetting(fallback, setting)
			if err == nil {
				base = xcconfigResolvedValue{value: value, path: source, found: true}
			}
		}
	}
	return resolveXCConfigSettingWithBase(path, setting, base)
}

func (project *structuredVersionProject) bumpBaseline(opts BumpVersionOptions, setting string, requireConsistent bool) (string, error) {
	selected, err := project.selectMutationConfigurations(opts.Target, opts.Configuration)
	if err != nil {
		return "", err
	}
	selected, err = project.baselineMutationConfigurations(selected, setting)
	if err != nil {
		return "", err
	}
	baseline := ""
	baselineSet := false
	for _, configuration := range selected {
		value, _, err := project.resolveSetting(configuration, setting)
		if err != nil {
			return "", fmt.Errorf("resolve %s for target %q configuration %q: %w", setting, configuration.target, configuration.name, err)
		}
		if !baselineSet {
			baseline = value
			baselineSet = true
			continue
		}
		if requireConsistent && value != baseline {
			return "", fmt.Errorf(
				"cannot bump %s across differing values (%q for target %q configuration %q, baseline %q); narrow with --target and --configuration",
				setting, value, configuration.target, configuration.name, baseline,
			)
		}
	}
	if !baselineSet {
		return "", fmt.Errorf("%s not found in selected Xcode configurations", setting)
	}
	return baseline, nil
}

func (project *structuredVersionProject) baselineMutationConfigurations(selected []*versionConfiguration, setting string) ([]*versionConfiguration, error) {
	effective := effectiveMutationConfigurations(selected)
	included := make(map[string]bool, len(effective))
	result := make([]*versionConfiguration, 0, len(selected))
	for _, configuration := range effective {
		included[configuration.id] = true
		result = append(result, configuration)
	}
	for _, configuration := range selected {
		if included[configuration.id] || !configuration.projectLevel {
			continue
		}
		if _, _, err := project.resolveSetting(configuration, setting); err != nil {
			if errors.Is(err, errVersionSettingNotFound) {
				continue
			}
			return nil, fmt.Errorf("resolve %s for project configuration %q: %w", setting, configuration.name, err)
		}
		result = append(result, configuration)
	}
	return result, nil
}

func effectiveMutationConfigurations(selected []*versionConfiguration) []*versionConfiguration {
	var targetConfigurations []*versionConfiguration
	for _, configuration := range selected {
		if !configuration.projectLevel {
			targetConfigurations = append(targetConfigurations, configuration)
		}
	}
	if len(targetConfigurations) > 0 {
		return targetConfigurations
	}
	return selected
}

func (project *structuredVersionProject) fileReferencePath(referenceID string) (string, error) {
	reference, err := project.objects.Object(referenceID)
	if err != nil {
		return "", fmt.Errorf("read base configuration reference %s: %w", referenceID, err)
	}
	path, err := reference.String("path")
	if err != nil {
		path, err = reference.String("name")
		if err != nil {
			return "", fmt.Errorf("base configuration reference %s has no path", referenceID)
		}
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	sourceTree, _ := reference.String("sourceTree")
	if sourceTree == "SOURCE_ROOT" || sourceTree == "<absolute>" {
		return filepath.Join(project.rootDir, path), nil
	}

	var groupPaths []string
	seen := make(map[string]bool)
	current := referenceID
	for {
		parentID, ok := project.parentByChild[current]
		if !ok || seen[parentID] {
			break
		}
		seen[parentID] = true
		parent, err := project.objects.Object(parentID)
		if err != nil {
			return "", err
		}
		groupPath, _ := parent.String("path")
		if groupPath != "" {
			groupPaths = append([]string{groupPath}, groupPaths...)
		}
		parentSourceTree, _ := parent.String("sourceTree")
		current = parentID
		if parentSourceTree == "SOURCE_ROOT" || parentSourceTree == "<absolute>" {
			break
		}
	}
	parts := append([]string(nil), groupPaths...)
	if len(parts) == 0 || !filepath.IsAbs(parts[0]) {
		parts = append([]string{project.rootDir}, parts...)
	}
	parts = append(parts, path)
	return filepath.Clean(filepath.Join(parts...)), nil
}

type setVersionValidation struct {
	selected                   []*versionConfiguration
	selectedIDs                map[string]bool
	fileConsumers              map[string]map[string]bool
	fileIdentities             map[string]string
	configFiles                map[string][]string
	uncertainXCConfigConsumers bool
}

func (project *structuredVersionProject) validateSetVersion(opts SetVersionOptions) (*setVersionValidation, error) {
	if err := validateVersionMutationValue("--version", opts.Version); err != nil {
		return nil, err
	}
	if err := validateVersionMutationValue("--build-number", opts.BuildNumber); err != nil {
		return nil, err
	}
	selected, err := project.selectMutationConfigurations(opts.Target, opts.Configuration)
	if err != nil {
		return nil, err
	}
	selectedIDs := make(map[string]bool, len(selected))
	for _, configuration := range selected {
		selectedIDs[configuration.id] = true
	}

	fileConsumers, configFiles, fileIdentities, uncertainXCConfigConsumers, err := project.xcconfigConsumers(selectedIDs)
	if err != nil {
		return nil, err
	}
	for _, requested := range []struct {
		name  string
		value string
	}{
		{name: marketingVersionSetting, value: strings.TrimSpace(opts.Version)},
		{name: currentProjectSetting, value: strings.TrimSpace(opts.BuildNumber)},
	} {
		if requested.value == "" {
			continue
		}
		for _, configuration := range effectiveMutationConfigurations(selected) {
			mutable, err := project.configurationCanMutateSetting(
				configuration,
				requested.name,
				configFiles,
				fileConsumers,
				fileIdentities,
				selectedIDs,
				uncertainXCConfigConsumers,
				strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "",
			)
			if err != nil {
				return nil, err
			}
			if !mutable {
				return nil, fmt.Errorf("%s not found for target %q configuration %q", requested.name, configuration.target, configuration.name)
			}
			if err := project.validateXCConfigMutationTargets(
				configuration,
				requested.name,
				configFiles,
				fileConsumers,
				fileIdentities,
				selectedIDs,
				uncertainXCConfigConsumers,
				opts.AllowExternalXCConfig,
			); err != nil {
				return nil, err
			}
		}
	}
	return &setVersionValidation{
		selected:                   selected,
		selectedIDs:                selectedIDs,
		fileConsumers:              fileConsumers,
		fileIdentities:             fileIdentities,
		configFiles:                configFiles,
		uncertainXCConfigConsumers: uncertainXCConfigConsumers,
	}, nil
}

// validateXCConfigMutationTargets refuses to rewrite an xcconfig file the project
// references outside its own directory unless the operator authorized it. The
// check runs during validation so it fails before any local mutation or remote
// build-number lookup.
func (project *structuredVersionProject) validateXCConfigMutationTargets(
	configuration *versionConfiguration,
	setting string,
	configFiles map[string][]string,
	fileConsumers map[string]map[string]bool,
	fileIdentities map[string]string,
	selectedIDs map[string]bool,
	uncertainXCConfigConsumers bool,
	allowExternal bool,
) error {
	if len(matchingBuildSettingKeys(configuration.buildSettings, setting)) > 0 {
		return nil
	}
	assignmentFiles, err := xcconfigFilesDefining(configFiles[configuration.id], setting)
	if err != nil {
		return err
	}
	if len(assignmentFiles) == 0 {
		return nil
	}
	if uncertainXCConfigConsumers || !consumersSelected(assignmentFiles, fileConsumers, fileIdentities, selectedIDs) {
		return nil
	}
	for _, path := range assignmentFiles {
		if err := project.checkXCConfigWritable(path, allowExternal); err != nil {
			return err
		}
	}
	return nil
}

func (project *structuredVersionProject) checkXCConfigWritable(path string, allowExternal bool) error {
	if allowExternal {
		return nil
	}
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.AllowingInternalSymlinks().CheckContained(path); err != nil {
		if errors.Is(err, rootfs.ErrSymlink) {
			if info, lstatErr := os.Lstat(path); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
				// The write path refuses a symlinked final component even when
				// --allow-external-xcconfig is set, so do not present the flag
				// as a remedy here.
				return fmt.Errorf(
					"refusing to modify xcconfig %s through a symlink: %w; "+
						"replace the symlink with a regular file",
					path,
					err,
				)
			}
			// A symlinked parent directory that resolves outside the project is
			// supported once the operator authorizes external rewrites.
			return fmt.Errorf(
				"refusing to modify xcconfig %s reached through a symlinked directory that resolves outside project directory %s: %w; "+
					"move the file inside the project or rerun with --allow-external-xcconfig",
				path,
				project.rootDir,
				err,
			)
		}
		return fmt.Errorf(
			"refusing to modify xcconfig %s outside project directory %s: %w; "+
				"move the file inside the project or rerun with --allow-external-xcconfig",
			path,
			project.rootDir,
			err,
		)
	}
	return nil
}

func (project *structuredVersionProject) versionFileTarget(projectRoot rootfs.Root, path string, allowExternal bool) (preparedVersionWrite, error) {
	if err := projectRoot.CheckContained(path); err == nil {
		return preparedVersionWrite{path: path, root: projectRoot, name: path}, nil
	} else if !allowExternal {
		return preparedVersionWrite{}, err
	}

	// --allow-external-xcconfig deliberately authorizes the referenced parent
	// directory as a separate trusted root. The final file must still be a
	// regular, non-symlink entry below that root.
	externalRoot, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return preparedVersionWrite{}, err
	}
	name := filepath.Base(path)
	if err := externalRoot.CheckContained(name); err != nil {
		_ = externalRoot.Close()
		return preparedVersionWrite{}, err
	}
	return preparedVersionWrite{path: path, root: externalRoot, name: name, ownsRoot: true}, nil
}

func (project *structuredVersionProject) containedVersionFileTarget(projectRoot rootfs.Root, path string) (preparedVersionWrite, error) {
	if err := projectRoot.CheckContained(path); err != nil {
		return preparedVersionWrite{}, err
	}
	return preparedVersionWrite{path: path, root: projectRoot, name: path}, nil
}

func (project *structuredVersionProject) setVersion(opts SetVersionOptions) (*SetVersionResult, error) {
	validation, err := project.validateSetVersion(opts)
	if err != nil {
		return nil, err
	}
	selected := validation.selected
	selectedIDs := validation.selectedIDs
	fileConsumers := validation.fileConsumers
	fileIdentities := validation.fileIdentities
	configFiles := validation.configFiles
	uncertainXCConfigConsumers := validation.uncertainXCConfigConsumers
	xcconfigMutations := make(map[string]map[string]xcconfigMutation)
	xcconfigPathSpelling := make(map[string]string)
	changes := make([]VersionChange, 0)
	pbxprojChanged := false

	settings := []struct {
		name  string
		value string
	}{
		{name: marketingVersionSetting, value: strings.TrimSpace(opts.Version)},
		{name: currentProjectSetting, value: strings.TrimSpace(opts.BuildNumber)},
	}
	for _, requested := range settings {
		if requested.value == "" {
			continue
		}
		handled := make(map[string]bool)
		handlingErrors := make(map[string]error)
		for _, configuration := range selected {
			keys := matchingBuildSettingKeys(configuration.buildSettings, requested.name)
			if len(keys) > 0 {
				handled[configuration.id] = true
				if len(keys) == 1 && keys[0] == requested.name {
					resolved, _, resolveErr := project.resolveSetting(configuration, requested.name)
					if resolveErr == nil && resolved == requested.value {
						continue
					}
				}
				for _, key := range keys {
					oldValue, _ := configuration.buildSettings[key].(string)
					if oldValue == requested.value {
						continue
					}
					configuration.buildSettings[key] = requested.value
					pbxprojChanged = true
					changes = append(changes, versionChangeForConfiguration(configuration, requested.name, oldValue, requested.value, project.pbxprojPath, "pbxproj"))
				}
				continue
			}

			assignmentFiles, err := xcconfigFilesDefining(configFiles[configuration.id], requested.name)
			if err != nil {
				return nil, err
			}
			if len(assignmentFiles) > 0 {
				if !uncertainXCConfigConsumers && consumersSelected(assignmentFiles, fileConsumers, fileIdentities, selectedIDs) {
					handled[configuration.id] = true
					for _, path := range assignmentFiles {
						pathKey := signingXCConfigOperationKey(path, fileIdentities)
						if xcconfigMutations[pathKey] == nil {
							xcconfigMutations[pathKey] = make(map[string]xcconfigMutation)
							xcconfigPathSpelling[pathKey] = path
						}
						mutation := xcconfigMutations[pathKey][requested.name]
						mutation.setting = requested.name
						mutation.value = requested.value
						mutation.configurations = appendVersionConfigurationOnce(mutation.configurations, configuration)
						xcconfigMutations[pathKey][requested.name] = mutation
					}
					continue
				}
			}

			if !configuration.projectLevel {
				ancestor := project.projectConfiguration(configuration.name)
				if ancestor != nil && selectedIDs[ancestor.id] && configurationProvidesScheduledSetting(ancestor, requested.name, configFiles, fileIdentities, xcconfigMutations) {
					handled[configuration.id] = true
					continue
				}
			}

			if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "" {
				oldValue, _, resolveErr := project.resolveSetting(configuration, requested.name)
				if resolveErr == nil {
					handled[configuration.id] = true
					if oldValue == requested.value {
						continue
					}
					configuration.buildSettings[requested.name] = requested.value
					pbxprojChanged = true
					changes = append(changes, versionChangeForConfiguration(configuration, requested.name, oldValue, requested.value, project.pbxprojPath, "pbxproj"))
				} else {
					handlingErrors[configuration.id] = resolveErr
				}
			}
		}
		for _, configuration := range effectiveMutationConfigurations(selected) {
			if !handled[configuration.id] {
				if handlingErr := handlingErrors[configuration.id]; handlingErr != nil {
					return nil, fmt.Errorf("%s could not be updated for target %q configuration %q: %w", requested.name, configuration.target, configuration.name, handlingErr)
				}
				return nil, fmt.Errorf("%s could not be updated for target %q configuration %q", requested.name, configuration.target, configuration.name)
			}
		}
	}

	projectRoot, err := rootfs.New(project.rootDir)
	if err != nil {
		return nil, err
	}
	projectRoot = projectRoot.AllowingInternalSymlinks()
	defer projectRoot.Close()
	var writes []preparedVersionWrite
	defer func() { _ = closeVersionWrites(writes) }()
	if pbxprojChanged {
		write, err := project.preparePBXProjWrite(projectRoot, false)
		if err != nil {
			return nil, err
		}
		writes = append(writes, write)
	}

	var xcconfigPaths []string
	for pathKey := range xcconfigMutations {
		xcconfigPaths = append(xcconfigPaths, pathKey)
	}
	sort.Strings(xcconfigPaths)
	for _, pathKey := range xcconfigPaths {
		path := xcconfigPathSpelling[pathKey]
		if err := project.checkXCConfigWritable(path, opts.AllowExternalXCConfig); err != nil {
			return nil, err
		}
		target, err := project.versionFileTarget(projectRoot, path, opts.AllowExternalXCConfig)
		if err != nil {
			return nil, fmt.Errorf("prepare xcconfig %s: %w", path, err)
		}
		write, fileChanges, changed, err := prepareXCConfigWrite(target, xcconfigMutations[pathKey])
		if err != nil {
			_ = target.root.Close()
			return nil, err
		}
		if changed {
			writes = append(writes, write)
			changes = append(changes, fileChanges...)
		} else if target.ownsRoot {
			_ = target.root.Close()
		}
	}

	for index := range writes {
		writes[index].portableFallback = true
	}
	if err := commitVersionWrites(writes); err != nil {
		return nil, err
	}
	sortVersionChanges(changes)
	changedFiles := make([]string, 0, len(writes))
	for _, write := range writes {
		changedFiles = append(changedFiles, write.path)
	}
	sort.Strings(changedFiles)
	return &SetVersionResult{
		Version:       strings.TrimSpace(opts.Version),
		BuildNumber:   strings.TrimSpace(opts.BuildNumber),
		ProjectDir:    project.rootDir,
		Target:        strings.TrimSpace(opts.Target),
		Configuration: strings.TrimSpace(opts.Configuration),
		ChangedFiles:  changedFiles,
		Changes:       changes,
	}, nil
}

func (project *structuredVersionProject) configurationCanMutateSetting(
	configuration *versionConfiguration,
	setting string,
	configFiles map[string][]string,
	fileConsumers map[string]map[string]bool,
	fileIdentities map[string]string,
	selectedIDs map[string]bool,
	uncertainXCConfigConsumers bool,
	scoped bool,
) (bool, error) {
	if len(matchingBuildSettingKeys(configuration.buildSettings, setting)) > 0 {
		return true, nil
	}
	defining, err := xcconfigFilesDefining(configFiles[configuration.id], setting)
	if err != nil {
		return false, err
	}
	if len(defining) > 0 {
		if !scoped || (!uncertainXCConfigConsumers && consumersSelected(defining, fileConsumers, fileIdentities, selectedIDs)) {
			return true, nil
		}
		_, _, resolveErr := project.resolveSetting(configuration, setting)
		if resolveErr != nil {
			return false, resolveErr
		}
		return true, nil
	}
	if !configuration.projectLevel {
		if ancestor := project.projectConfiguration(configuration.name); ancestor != nil {
			if len(matchingBuildSettingKeys(ancestor.buildSettings, setting)) > 0 && (!scoped || selectedIDs[ancestor.id]) {
				return true, nil
			}
			defining, err := xcconfigFilesDefining(configFiles[ancestor.id], setting)
			if err != nil {
				return false, err
			}
			if len(defining) > 0 && (!scoped || (selectedIDs[ancestor.id] && !uncertainXCConfigConsumers && consumersSelected(defining, fileConsumers, fileIdentities, selectedIDs))) {
				return true, nil
			}
		}
	}
	if scoped {
		_, _, resolveErr := project.resolveSetting(configuration, setting)
		if resolveErr != nil {
			return false, resolveErr
		}
		return true, nil
	}
	return false, nil
}

func configurationProvidesScheduledSetting(
	configuration *versionConfiguration,
	setting string,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	mutations map[string]map[string]xcconfigMutation,
) bool {
	if len(matchingBuildSettingKeys(configuration.buildSettings, setting)) > 0 {
		return true
	}
	for _, path := range configFiles[configuration.id] {
		mutation, ok := mutations[signingXCConfigOperationKey(path, fileIdentities)][setting]
		if !ok {
			continue
		}
		for _, affected := range mutation.configurations {
			if affected.id == configuration.id {
				return true
			}
		}
	}
	return false
}

func appendVersionConfigurationOnce(configurations []*versionConfiguration, candidate *versionConfiguration) []*versionConfiguration {
	for _, configuration := range configurations {
		if configuration.id == candidate.id {
			return configurations
		}
	}
	return append(configurations, candidate)
}

func matchingBuildSettingKeys(settings serialized.Object, setting string) []string {
	var keys []string
	for key := range settings {
		if xcconfigBaseKey(key) == setting {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func versionChangeForConfiguration(configuration *versionConfiguration, setting, oldValue, newValue, path, source string) VersionChange {
	return VersionChange{
		Setting:       setting,
		OldValue:      oldValue,
		NewValue:      newValue,
		Target:        configuration.target,
		Configuration: configuration.name,
		Path:          path,
		Source:        source,
	}
}

func validateVersionMutationValue(flagName, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain a newline", flagName)
	}
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("%s must not contain NUL", flagName)
	}
	if strings.Contains(value, "//") || strings.Contains(value, "/*") || strings.Contains(value, "*/") {
		return fmt.Errorf("%s must not contain comment syntax", flagName)
	}
	if strings.Contains(value, "$(") || strings.Contains(value, "${") {
		return fmt.Errorf("%s must be a static value without build-setting references", flagName)
	}
	return nil
}

type xcconfigFileIdentity struct {
	key  string
	info os.FileInfo
}

type xcconfigFileIdentityIndex struct {
	entries []xcconfigFileIdentity
}

// identity records a file identity only after the collector has successfully
// authorized and read the path. Keeping that invariant explicit prevents a
// later identity pass from reopening an unselected or unauthorized path.
func (index *xcconfigFileIdentityIndex) identity(
	path string,
	collected map[string]bool,
	identifiers ...func(string) (os.FileInfo, error),
) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absolutePath = filepath.Clean(absolutePath)
	if !collected[normalizeSigningLexicalPath(absolutePath)] && !collected[signingLexicalPathKey(absolutePath)] {
		return "", fmt.Errorf("xcconfig path %s was not collected for this signing plan", absolutePath)
	}
	identify := signingXCConfigIdentityFn
	if len(identifiers) > 0 && identifiers[0] != nil {
		identify = identifiers[0]
	}
	info, err := identify(absolutePath)
	if err != nil {
		return "", err
	}
	for _, entry := range index.entries {
		if os.SameFile(info, entry.info) {
			return entry.key, nil
		}
	}
	key := absolutePath
	index.entries = append(index.entries, xcconfigFileIdentity{key: key, info: info})
	return key, nil
}

func (project *structuredVersionProject) xcconfigConsumers(selectedIDs map[string]bool) (map[string]map[string]bool, map[string][]string, map[string]string, bool, error) {
	return project.xcconfigConsumersWithCollectorAndIdentity(selectedIDs, collectStableXCConfigFiles, os.Stat)
}

func (project *structuredVersionProject) signingXCConfigConsumers(selectedIDs map[string]bool, allowExternal bool) (map[string]map[string]bool, map[string][]string, map[string]string, bool, []string, []string, map[string][]string, bool, error) {
	consumers, configFiles, identities, uncertain, protected, blocked, lexical, unauthorized, _, err := project.signingXCConfigConsumersWithOptionalMissing(selectedIDs, allowExternal)
	return consumers, configFiles, identities, uncertain, protected, blocked, lexical, unauthorized, err
}

const signingPlanMaxMissingOptionalIncludes = 256

func (project *structuredVersionProject) signingXCConfigConsumersWithOptionalMissing(selectedIDs map[string]bool, allowExternal bool) (map[string]map[string]bool, map[string][]string, map[string]string, bool, []string, []string, map[string][]string, bool, []string, error) {
	var protectedConfigPaths []string
	var blockedExternalPaths []string
	protectedPathSet := make(map[string]struct{})
	blockedPathSet := make(map[string]struct{})
	var missingOptionalIncludes []string
	missingOptionalOverflow := false
	unauthorizedExternal := false
	lexicalConfigPaths := make(map[string][]string)
	observedPathsByRoot := make(map[string][]string)
	var unselectedCollectionError error
	sourceBudget := &xcconfigSourceBudget{}
	addProtectedPath := func(path string) {
		absolute := normalizeSigningLexicalPath(path)
		if _, exists := protectedPathSet[absolute]; exists {
			return
		}
		protectedPathSet[absolute] = struct{}{}
		protectedConfigPaths = append(protectedConfigPaths, absolute)
	}
	addBlockedPath := func(path string) {
		absolute := normalizeSigningLexicalPath(path)
		if _, exists := blockedPathSet[absolute]; exists {
			return
		}
		blockedPathSet[absolute] = struct{}{}
		blockedExternalPaths = append(blockedExternalPaths, absolute)
	}
	addMissingOptionalInclude := func(path string) {
		path = normalizeSigningLexicalPath(path)
		if len(path) > signingPlanMaxMissingOptionalIncludePathBytes {
			missingOptionalOverflow = true
			return
		}
		for _, existing := range missingOptionalIncludes {
			if existing == path {
				return
			}
		}
		if len(missingOptionalIncludes) >= signingPlanMaxMissingOptionalIncludes {
			missingOptionalOverflow = true
			return
		}
		missingOptionalIncludes = append(missingOptionalIncludes, path)
	}
	collect := func(path string) ([]string, error) {
		var collectionErrorPath string
		var collectionErrorExternal bool
		observedPaths := make([]string, 0)
		observedPathSet := make(map[string]struct{})
		var identify func(string) (os.FileInfo, error)
		if xcconfigUsesIdentityTraversal() {
			identify = signingXCConfigIdentityFn
		}
		files, err := collectXCConfigFilesWithHooksAndIdentityAndOptionalMissingLimitWithBudget(
			path,
			func(filePath string) ([]byte, error) {
				return signingXCConfigReadFileFn(filePath, signingPlanMaxBytes)
			},
			func(filePath string) error {
				if !allowExternal && signingPathDefinitelyExternal(project, filePath) {
					unauthorizedExternal = true
					return &signingXCConfigAccessError{path: filePath, err: errors.New("path is outside the project directory"), external: true}
				}
				if err := validateSigningXCConfigPath(project, filePath, allowExternal); err != nil {
					// A rejected symlink is not readable through the signing
					// root, even when its lexical spelling is inside the project.
					// Treat it as untrusted external content: its target may hide
					// an entitlement expression that cannot be inventoried here.
					// ErrEscapesRoot also covers a case-variant spelling rejected
					// on a modeled case-sensitive Darwin/Windows directory. That
					// path was not proven to be an internal source, so it must take
					// the same no-opt-in fail-closed path as other external inputs.
					pathRejectedAsExternal := !allowExternal &&
						(errors.Is(err, rootfs.ErrEscapesRoot) || errors.Is(err, rootfs.ErrSymlink))
					if pathRejectedAsExternal {
						unauthorizedExternal = true
					}
					return &signingXCConfigAccessError{
						path:     filePath,
						err:      err,
						external: pathRejectedAsExternal || signingPathDefinitelyExternal(project, filePath),
					}
				}
				return nil
			},
			func(filePath string) {
				// This runs before authorization, stat, open, read, and parse.
				// An external path remains protected from artifact writes even
				// when the operator opted into reading it.
				addProtectedPath(filePath)
				absolute := normalizeSigningLexicalPath(filePath)
				if _, exists := observedPathSet[absolute]; !exists {
					observedPathSet[absolute] = struct{}{}
					observedPaths = append(observedPaths, absolute)
				}
			},
			func(filePath string, _ error) {
				collectionErrorPath = filePath
			},
			identify,
			addMissingOptionalInclude,
			signingPlanMaxFiles,
			signingXCConfigStatFileFn,
			sourceBudget,
		)
		observedPathsByRoot[normalizeSigningLexicalPath(path)] = observedPaths
		if err == nil {
			return files, nil
		}
		if collectionErrorPath != "" {
			var accessErr *signingXCConfigAccessError
			if errors.As(err, &accessErr) {
				collectionErrorExternal = accessErr.external
			}
			return nil, &signingXCConfigAccessError{
				path:     collectionErrorPath,
				err:      err,
				external: collectionErrorExternal,
			}
		}
		return nil, err
	}
	consumers, configFiles, identities, uncertain, err := project.xcconfigConsumersWithCollectorAndErrorHook(selectedIDs, collect, func(configuration *versionConfiguration, err error) {
		var accessErr *signingXCConfigAccessError
		if errors.As(err, &accessErr) {
			addBlockedPath(accessErr.path)
		}
		if !selectedIDs[configuration.id] && isXCConfigSourceGraphLimitError(err) {
			unselectedCollectionError = errors.Join(
				unselectedCollectionError,
				fmt.Errorf("resolve xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err),
			)
		}
	})
	// Unselected collection failures are normally represented by an uncertain
	// consumer scope so stable target-level planning can continue. A source
	// graph limit is different: the collector did not inventory the complete
	// graph, so serializing any plan could omit a source that aliases an output
	// artifact. Preserve this fatal cause for BuildSigningPlan even when the
	// overflowing configuration was not selected.
	err = errors.Join(err, unselectedCollectionError)
	for _, configuration := range project.configurations {
		if configuration.baseReferenceID == "" {
			continue
		}
		root, rootErr := project.fileReferencePath(configuration.baseReferenceID)
		if rootErr != nil {
			continue
		}
		if observed := observedPathsByRoot[normalizeSigningLexicalPath(root)]; len(observed) > 0 {
			lexicalConfigPaths[configuration.id] = append([]string(nil), observed...)
		}
	}
	if missingOptionalOverflow {
		err = errors.Join(err, fmt.Errorf("too many missing optional xcconfig includes; refusing to publish a signing plan"))
	}
	sort.Strings(missingOptionalIncludes)
	return consumers, configFiles, identities, uncertain, protectedConfigPaths, blockedExternalPaths, lexicalConfigPaths, unauthorizedExternal, missingOptionalIncludes, err
}

func (project *structuredVersionProject) xcconfigConsumersWithCollectorAndErrorHook(
	selectedIDs map[string]bool,
	collect func(string) ([]string, error),
	onUnselectedError func(*versionConfiguration, error),
) (map[string]map[string]bool, map[string][]string, map[string]string, bool, error) {
	return project.xcconfigConsumersWithCollectorAndIdentityAndErrorHook(selectedIDs, collect, signingXCConfigIdentityFn, onUnselectedError)
}

func (project *structuredVersionProject) xcconfigConsumersWithCollectorAndIdentity(
	selectedIDs map[string]bool,
	collect func(string) ([]string, error),
	identify func(string) (os.FileInfo, error),
) (map[string]map[string]bool, map[string][]string, map[string]string, bool, error) {
	return project.xcconfigConsumersWithCollectorAndIdentityAndErrorHook(selectedIDs, collect, identify, nil)
}

func (project *structuredVersionProject) xcconfigConsumersWithCollectorAndIdentityAndErrorHook(
	selectedIDs map[string]bool,
	collect func(string) ([]string, error),
	identify func(string) (os.FileInfo, error),
	onUnselectedError func(*versionConfiguration, error),
) (map[string]map[string]bool, map[string][]string, map[string]string, bool, error) {
	consumers := make(map[string]map[string]bool)
	configFiles := make(map[string][]string)
	fileIdentities := make(map[string]string)
	identityIndex := xcconfigFileIdentityIndex{}
	uncertainConsumers := false
	var selectedError error
	for _, configuration := range project.configurations {
		if configuration.baseReferenceID == "" {
			continue
		}
		root, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			if selectedIDs[configuration.id] {
				selectedError = errors.Join(selectedError, fmt.Errorf("resolve xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err))
				continue
			}
			uncertainConsumers = true
			continue
		}
		files, err := collect(root)
		if err != nil {
			if onUnselectedError != nil {
				onUnselectedError(configuration, err)
			}
			if selectedIDs[configuration.id] {
				selectedError = errors.Join(selectedError, fmt.Errorf("resolve xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err))
				continue
			}
			uncertainConsumers = true
			continue
		}
		configFiles[configuration.id] = files
		collected := make(map[string]bool, len(files))
		for _, path := range files {
			absolute, err := filepath.Abs(path)
			if err != nil {
				if selectedIDs[configuration.id] {
					selectedError = errors.Join(selectedError, fmt.Errorf("resolve collected xcconfig path %q: %w", path, err))
					continue
				}
				uncertainConsumers = true
				continue
			}
			collected[normalizeSigningLexicalPath(absolute)] = true
		}
		for _, path := range files {
			normalizedPath := normalizeSigningLexicalPath(path)
			identity, ok := fileIdentities[normalizedPath]
			if !ok {
				identity, err = identityIndex.identity(path, collected, identify)
				if err != nil {
					if selectedIDs[configuration.id] {
						selectedError = errors.Join(selectedError, fmt.Errorf("identify xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err))
						continue
					}
					uncertainConsumers = true
					continue
				}
				fileIdentities[normalizedPath] = identity
			}
			// Keep the operator spelling as the map key. Windows can enable
			// case-sensitive semantics per directory, so lowercasing here would
			// discard a distinct file before operation keys can use its proven
			// identity.
			fileIdentities[normalizedPath] = identity
			if consumers[identity] == nil {
				consumers[identity] = make(map[string]bool)
			}
			consumers[identity][configuration.id] = true
		}
	}
	return consumers, configFiles, fileIdentities, uncertainConsumers, selectedError
}

func xcconfigFilesDefining(paths []string, setting string) ([]string, error) {
	return xcconfigFilesDefiningWithReader(paths, setting, os.ReadFile)
}

func xcconfigFilesDefiningWithReader(paths []string, setting string, read func(string) ([]byte, error)) ([]string, error) {
	var defining []string
	for _, path := range paths {
		data, err := read(path)
		if err != nil {
			return nil, err
		}
		document, err := parseXCConfig(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		for _, assignment := range document.assignments {
			if assignment.baseKey == setting {
				defining = append(defining, path)
				break
			}
		}
	}
	return defining, nil
}

func consumersSelected(paths []string, consumers map[string]map[string]bool, identities map[string]string, selected map[string]bool) bool {
	for _, path := range paths {
		identity, ok := signingFileIdentity(path, identities)
		if !ok {
			return false
		}
		for configurationID := range consumers[identity] {
			if !selected[configurationID] {
				return false
			}
		}
	}
	return true
}

// consumersAuthorizeSetting reports whether every configuration that consumes
// one of paths explicitly requested the same setting. A shared xcconfig may
// only be edited when all of its consumers authorize that specific key; merely
// selecting a configuration is insufficient because another selected consumer
// may have requested a different signing setting.
func consumersAuthorizeSetting(
	paths []string,
	consumers map[string]map[string]bool,
	identities map[string]string,
	requestedSettings map[string]map[string]bool,
	setting string,
) bool {
	for _, path := range paths {
		identity, ok := signingFileIdentity(path, identities)
		if !ok {
			return false
		}
		for configurationID := range consumers[identity] {
			if !requestedSettings[configurationID][setting] {
				return false
			}
		}
	}
	return true
}

func (project *structuredVersionProject) preparePBXProjWrite(projectRoot rootfs.Root, strictIdentity bool) (result preparedVersionWrite, resultErr error) {
	target, err := project.containedVersionFileTarget(projectRoot, project.pbxprojPath)
	if err != nil {
		return preparedVersionWrite{}, fmt.Errorf("prepare Xcode project file: %w", err)
	}
	target.strictIdentity = strictIdentity
	defer func() {
		if resultErr != nil {
			_ = target.root.Close()
		}
	}()
	original, mode, err := readRegularVersionFile(&target)
	if err != nil {
		return preparedVersionWrite{}, err
	}
	tempRoot, err := os.MkdirTemp("", "asc-xcode-project-stage-*")
	if err != nil {
		return preparedVersionWrite{}, fmt.Errorf("create Xcode project staging directory: %w", err)
	}
	defer os.RemoveAll(tempRoot)
	stagedProjectPath := filepath.Join(tempRoot, filepath.Base(project.projectPath))
	if err := os.MkdirAll(stagedProjectPath, 0o755); err != nil {
		return preparedVersionWrite{}, err
	}
	if err := os.WriteFile(filepath.Join(stagedProjectPath, "project.pbxproj"), original, mode); err != nil {
		return preparedVersionWrite{}, err
	}
	var updated []byte
	if project.project.Format == plist.OpenStepFormat {
		updated, err = editPBXProjBuildSettings(original, project)
		if err != nil {
			return preparedVersionWrite{}, fmt.Errorf("edit Xcode project build settings: %w", err)
		}
		if err := os.WriteFile(filepath.Join(stagedProjectPath, "project.pbxproj"), updated, mode); err != nil {
			return preparedVersionWrite{}, err
		}
	} else {
		stagedProject := project.project
		stagedProject.Path = stagedProjectPath
		if err := stagedProject.Save(); err != nil {
			return preparedVersionWrite{}, fmt.Errorf("serialize Xcode project: %w", err)
		}
		updated, err = os.ReadFile(filepath.Join(stagedProjectPath, "project.pbxproj"))
		if err != nil {
			return preparedVersionWrite{}, err
		}
	}
	staged, err := xcodeproj.Open(stagedProjectPath)
	if err != nil {
		return preparedVersionWrite{}, fmt.Errorf("validate staged Xcode project: %w", err)
	}
	// go-xcode retains the root source-position annotation; it is not project data.
	stagedRaw, intendedRaw := maps.Clone(staged.RawProj), maps.Clone(project.project.RawProj)
	delete(stagedRaw, plist.CustomAnnotationKey)
	delete(intendedRaw, plist.CustomAnnotationKey)
	if !reflect.DeepEqual(stagedRaw, intendedRaw) {
		return preparedVersionWrite{}, fmt.Errorf("staged Xcode project differs from intended build settings")
	}
	target.original = original
	target.updated = updated
	target.mode = mode
	return target, nil
}

func prepareXCConfigWrite(target preparedVersionWrite, mutations map[string]xcconfigMutation) (preparedVersionWrite, []VersionChange, bool, error) {
	original, mode, err := readRegularVersionFile(&target)
	if err != nil {
		return preparedVersionWrite{}, nil, false, err
	}
	updated := original
	var changes []VersionChange
	var settings []string
	for setting := range mutations {
		settings = append(settings, setting)
	}
	sort.Strings(settings)
	changed := false
	for _, setting := range settings {
		mutation := mutations[setting]
		next, oldValues, settingChanged, err := editXCConfig(updated, setting, mutation.value)
		if err != nil {
			return preparedVersionWrite{}, nil, false, fmt.Errorf("edit %s: %w", target.path, err)
		}
		updated = next
		if !settingChanged {
			continue
		}
		changed = true
		oldValue := ""
		if len(oldValues) > 0 {
			oldValue = oldValues[len(oldValues)-1]
		}
		for _, configuration := range mutation.configurations {
			changes = append(changes, VersionChange{
				Setting:       setting,
				OldValue:      oldValue,
				NewValue:      mutation.value,
				Target:        configuration.target,
				Configuration: configuration.name,
				Path:          target.path,
				Source:        "xcconfig",
			})
		}
	}
	if _, err := parseXCConfig(updated); err != nil {
		return preparedVersionWrite{}, nil, false, fmt.Errorf("validate %s: %w", target.path, err)
	}
	target.original = original
	target.updated = updated
	target.mode = mode
	return target, changes, changed, nil
}

func readRegularVersionFile(target *preparedVersionWrite) ([]byte, os.FileMode, error) {
	if target == nil {
		return nil, 0, fmt.Errorf("prepared version write is nil")
	}
	if target.strictIdentity {
		if target.originalIdentity == nil {
			identity, err := target.root.CaptureFileLimited(target.name, signingPlanMaxBytes)
			if err != nil {
				return nil, 0, err
			}
			target.originalIdentity = identity
			return identity.Data(), identity.Mode(), nil
		}
	}
	if !target.strictIdentity {
		info, data, err := readRegularVersionFileCompatibility(target)
		if err != nil {
			return nil, 0, err
		}
		if target.originalInfo == nil {
			target.originalInfo = info
		}
		return data, info.Mode().Perm(), nil
	}

	file, err := target.root.OpenFile(target.name)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, 0, err
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file: %s", target.path)
	}
	data, err := io.ReadAll(io.LimitReader(file, signingPlanMaxBytes+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > signingPlanMaxBytes {
		return nil, 0, fmt.Errorf("%s exceeds the %d-byte version file size limit", target.path, signingPlanMaxBytes)
	}
	return data, info.Mode().Perm(), nil
}

func readRegularVersionFileCompatibility(target *preparedVersionWrite) (os.FileInfo, []byte, error) {
	file, err := target.root.OpenFile(target.name)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("not a regular file: %s", target.path)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}
	return info, data, nil
}

var (
	atomicWriteVersionFileFn    = atomicWritePreparedVersionFile
	afterVersionSourceCaptureFn func([]preparedVersionWrite) error
)

var atomicWriteVersionFileInfoFn = atomicWritePreparedVersionFileInfo

var atomicCreateVersionFileFn = atomicCreatePreparedVersionFile

var removeCreatedVersionFileFn = removeCreatedPreparedVersionFile

// removeCreatedVersionFileBeforeFinalCheckForTest injects a replacement or
// disappearance between content verification and the final identity check.
// It is intentionally test-only and nil in production.
var removeCreatedVersionFileBeforeFinalCheckForTest func()

// rollbackOrdinaryVersionBeforeConditionalWriteForTest injects a replacement
// after the transaction has captured a committed identity but before the
// rooted compare-and-publish primitive runs. It is intentionally test-only.
var rollbackOrdinaryVersionBeforeConditionalWriteForTest func()

// rollbackPublishedVersionBeforeConditionalWriteForTest injects a
// replacement after a failed write has returned its installed identity, but
// before the rooted compare-and-publish rollback.
// It is intentionally test-only and nil in production.
var rollbackPublishedVersionBeforeConditionalWriteForTest func()

// rollbackPublishedVersionWriteInfoFn is replaceable only so tests can cover
// the stable-command fallback used on platforms without identity mutation.
var rollbackPublishedVersionWriteInfoFn = func(write preparedVersionWrite) error {
	return write.root.WriteFileIfSame(
		write.name,
		write.original,
		write.mode,
		write.committedInfo,
		write.updated,
		write.preserveMetadata,
	)
}

func commitVersionWrites(writes []preparedVersionWrite) (resultErr error) {
	return commitVersionWritesWithCreateCheck(writes, nil)
}

// commitVersionWritesWithCreateCheck commits ordinary files before
// create-only artifacts and optionally revalidates the transaction immediately
// before the create-only phase. Signing apply uses that final check to ensure
// the receipt cannot certify source bytes that changed after the ordinary
// writes completed. Existing version-setting callers retain the original
// behavior by calling commitVersionWrites without a check.
func commitVersionWritesWithCreateCheck(
	writes []preparedVersionWrite,
	beforeCreate func([]preparedVersionWrite) error,
) (resultErr error) {
	return commitVersionWritesWithCreateChecks(writes, beforeCreate, nil)
}

// commitVersionWritesWithCreateChecks adds an optional post-create validation
// point. Signing apply uses both checks so a source replacement that races the
// receipt publication causes identity-checked removal of the receipt and
// rollback of the ordinary writes before the transaction returns.
func commitVersionWritesWithCreateChecks(
	writes []preparedVersionWrite,
	beforeCreate func([]preparedVersionWrite) error,
	afterCreate func([]preparedVersionWrite) error,
) (resultErr error) {
	var retainedIdentities []os.FileInfo
	defer func() {
		closeErr := errors.Join(closeVersionWritesFn(writes), closeVersionIdentities(retainedIdentities))
		if resultErr != nil {
			resultErr = errors.Join(resultErr, closeErr)
			return
		}
		// Publication already succeeded. A later close must not retract a
		// completed receipt or block apply retry.
		_ = closeErr
	}()
	sort.Slice(writes, func(left, right int) bool {
		// Finalize create-only artifacts after ordinary project files so a
		// publication failure exercises the rollback path for every mutation.
		if writes[left].createOnly != writes[right].createOnly {
			return !writes[left].createOnly
		}
		return writes[left].path < writes[right].path
	})
	// Capture source identities on the transaction-owned entries before the
	// commit loop starts copying them by value.  A strict write may arrive from
	// a caller that has only prepared its byte snapshot; in that case the
	// descriptor must be retained through publication and rollback rather than
	// reopening the path (which permits unlink-and-recreate identity reuse).
	for index := range writes {
		if writes[index].createOnly || !writes[index].strictIdentity || writes[index].originalIdentity != nil || bytes.Equal(writes[index].original, writes[index].updated) {
			continue
		}
		if _, _, err := readRegularVersionFile(&writes[index]); err != nil {
			return fmt.Errorf("verify source before commit: %w", err)
		}
	}
	if afterVersionSourceCaptureFn != nil {
		if err := afterVersionSourceCaptureFn(writes); err != nil {
			return err
		}
	}
	var committed []preparedVersionWrite
	for _, write := range writes {
		if !write.createOnly && string(write.original) == string(write.updated) {
			continue
		}
		if !write.createOnly {
			current, _, err := readRegularVersionFile(&write)
			if err != nil {
				return commitVersionWriteFailure(
					write,
					fmt.Errorf("verify source before commit: %w", err),
					committed,
				)
			}
			if !bytes.Equal(current, write.original) {
				return commitVersionWriteFailure(
					write,
					errors.New("source changed before commit"),
					committed,
				)
			}
		}
		var err error
		if write.createOnly && beforeCreate != nil {
			if err := beforeCreate(committed); err != nil {
				return commitVersionWriteFailure(
					write,
					fmt.Errorf("verify sources before receipt: %w", err),
					committed,
				)
			}
		}
		if write.createOnly {
			write.createdIdentity, err = atomicCreateVersionFileFn(write, write.updated)
		} else if write.strictIdentity {
			write.committedIdentity, err = atomicWriteVersionFileFn(write, write.updated)
		} else {
			write.committedInfo, err = atomicWriteVersionFileInfoFn(write, write.updated)
			if write.committedInfo == nil && write.portableFallback && identityPublicationUnsupported(err) {
				if checkErr := verifyPortableVersionWriteSource(write, write.original); checkErr != nil {
					err = errors.Join(err, fmt.Errorf("verify source before portable fallback: %w", checkErr))
				} else {
					write.committedInfo, err = portableWritePreparedVersionFile(write, write.updated)
				}
			}
			if write.committedInfo != nil {
				retainedIdentities = append(retainedIdentities, write.committedInfo)
			}
		}
		if err != nil {
			publicationMaySurvive := errors.Is(err, rootfs.ErrFilePublicationUncertain)
			if !write.createOnly && write.strictIdentity && write.committedIdentity == nil {
				err = errors.Join(err, errors.New("installed identity unavailable; rollback uncertain"))
			}
			rollbackErrors := make([]error, 0)
			receiptMaySurvive := false
			if write.createOnly && write.createdIdentity != nil {
				if rollbackErr := removeCreatedVersionFileFn(write); rollbackErr != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", write.path, rollbackErr))
					// The target is already gone when the cleanup primitive reports
					// ErrFileIdentityRemoved. A later durability or handle-close
					// failure still matters, but it does not justify leaving earlier
					// project writes in place for a receipt that cannot survive.
					receiptMaySurvive = !errors.Is(rollbackErr, rootfs.ErrFileIdentityRemoved)
				}
			}
			if !write.createOnly {
				if rollbackErr := rollbackPublishedVersionWriteAfterError(write); rollbackErr != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", write.path, rollbackErr))
				}
			}
			// A retained create-only identity is the only proof that a receipt can
			// be removed without touching a concurrent replacement. If any file
			// remains at the receipt path, it may be a concurrently published
			// Completed receipt. Keep earlier project writes in place so that file
			// cannot certify settings that rollback has removed.
			if !receiptMaySurvive && !publicationMaySurvive {
				rollbackErrors = append(rollbackErrors, rollbackVersionWrites(committed)...)
			}
			writeErr := fmt.Errorf("write %s: %w", write.path, err)
			if len(rollbackErrors) > 0 {
				return errors.Join(writeErr, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrors...)))
			}
			return writeErr
		}
		if !write.createOnly && write.strictIdentity && write.committedIdentity == nil {
			committedWithWrite := append(append([]preparedVersionWrite(nil), committed...), write)
			return commitVersionWriteFailure(
				write,
				fmt.Errorf("verify written file: installed identity unavailable"),
				committedWithWrite,
			)
		}
		if !write.createOnly && !write.strictIdentity && write.committedInfo == nil {
			return commitVersionWriteFailure(
				write,
				fmt.Errorf("verify written file: installed metadata unavailable"),
				append(append([]preparedVersionWrite(nil), committed...), write),
			)
		}
		if write.createOnly && write.createdIdentity == nil {
			return commitVersionWriteFailure(
				write,
				errors.New("created identity unavailable; rollback uncertain"),
				committed,
			)
		}
		committed = append(committed, write)
		if write.createOnly && afterCreate != nil {
			if err := afterCreate(committed); err != nil {
				return commitVersionWriteFailure(
					write,
					fmt.Errorf("verify sources after receipt: %w", err),
					committed,
				)
			}
		}
		if write.createOnly {
			if err := write.root.CheckFileIdentity(write.name, write.createdIdentity); err != nil {
				return commitVersionWriteFailure(
					write,
					fmt.Errorf("verify created file: %w", err),
					committed,
				)
			}
		}
	}
	return nil
}

func commitVersionWriteFailure(write preparedVersionWrite, writeErr error, committed []preparedVersionWrite) error {
	rollbackErrors := rollbackVersionWrites(committed)
	wrapped := fmt.Errorf("write %s: %w", write.path, writeErr)
	if len(rollbackErrors) == 0 {
		return wrapped
	}
	return errors.Join(wrapped, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrors...)))
}

func rollbackVersionWrites(committed []preparedVersionWrite) []error {
	var rollbackErrors []error
	for index := len(committed) - 1; index >= 0; index-- {
		var rollbackErr error
		if committed[index].createOnly {
			rollbackErr = removeCreatedVersionFileFn(committed[index])
		} else {
			rollbackErr = rollbackOrdinaryVersionWrite(committed[index])
		}
		if rollbackErr != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", committed[index].path, rollbackErr))
			if committed[index].createOnly && !errors.Is(rollbackErr, rootfs.ErrFileIdentityRemoved) {
				// Do not roll back any earlier project writes after a receipt
				// cleanup failure unless the primitive proves the receipt was
				// removed before a later durability or close failure. Their state
				// must remain consistent with a receipt whose removal is uncertain.
				break
			}
		}
	}
	return rollbackErrors
}

// rollbackPublishedVersionWriteAfterError restores a write whose replacement
// was already published even though the write call reported an error. The
// writer returns the installed identity from the same rooted publication
// operation, backed by a held descriptor, so rollback does not reopen the
// destination by path and mistake a later same-content replacement for the
// transaction's update. WriteFileIfSame then couples that identity to the
// expected updated bytes before restoring the original.
//
// The concrete case is the Windows replacement path, which moves the original
// aside, renames the staged file into place, and then fails while removing its
// backup copy. Publication succeeded, so without this the transaction would
// abort while leaving the project file modified and no receipt written.
//
// The backup file the primitive could not remove is deliberately not deleted
// here: its name is known only to the primitive and reaches this layer inside
// the error text. Callers join that error into the returned failure, so the
// stranded path is reported to the operator rather than silently discarded.
func rollbackPublishedVersionWriteAfterError(write preparedVersionWrite) error {
	if rollbackPublishedVersionBeforeConditionalWriteForTest != nil {
		rollbackPublishedVersionBeforeConditionalWriteForTest()
	}
	var err error
	if write.strictIdentity {
		if write.committedIdentity == nil {
			return nil
		}
		_, err = write.root.ReplaceFileIfSame(
			write.name,
			write.committedIdentity,
			write.original,
			write.mode,
			write.preserveMetadata,
		)
	} else {
		if write.committedInfo == nil {
			return nil
		}
		err = rollbackPublishedVersionWriteInfoFn(write)
	}
	if write.portableFallback && identityPublicationUnsupported(err) {
		if checkErr := verifyPortableVersionWriteSource(write, write.updated); checkErr != nil {
			err = errors.Join(err, checkErr)
		} else if info, fallbackErr := portableWritePreparedVersionFile(write, write.original); fallbackErr != nil {
			err = fallbackErr
		} else {
			write.committedInfo = info
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("preserve current file after failed write: %w", err)
	}
	return nil
}

func rollbackOrdinaryVersionWrite(write preparedVersionWrite) error {
	if write.strictIdentity && write.committedIdentity == nil {
		return fmt.Errorf("preserve current file after concurrent change: committed file identity unavailable")
	}
	if !write.strictIdentity && write.committedInfo == nil {
		return fmt.Errorf("preserve current file after concurrent change: committed file metadata unavailable")
	}
	if rollbackOrdinaryVersionBeforeConditionalWriteForTest != nil {
		rollbackOrdinaryVersionBeforeConditionalWriteForTest()
	}
	var err error
	if write.strictIdentity {
		_, err = write.root.ReplaceFileIfSame(
			write.name,
			write.committedIdentity,
			write.original,
			write.mode,
			write.preserveMetadata,
		)
	} else {
		err = write.root.WriteFileIfSame(
			write.name,
			write.original,
			write.mode,
			write.committedInfo,
			write.updated,
			write.preserveMetadata,
		)
	}
	if write.portableFallback && identityPublicationUnsupported(err) {
		if checkErr := verifyPortableVersionWriteSource(write, write.updated); checkErr != nil {
			err = errors.Join(err, checkErr)
		} else {
			write.committedInfo, err = portableWritePreparedVersionFile(write, write.original)
		}
	}
	if err != nil {
		return fmt.Errorf("preserve current file after concurrent change: %w", err)
	}
	return nil
}

func identityPublicationUnsupported(err error) bool {
	return errors.Is(err, secureopen.ErrRenameNoReplaceUnsupported) || errors.Is(err, rootfs.ErrFileIdentityMutationUnsupported)
}

var closeVersionWritesFn = closeVersionWrites

func closeVersionWrites(writes []preparedVersionWrite) error {
	var closeErrors []error
	for _, write := range writes {
		if err := write.root.Close(); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close root for %s: %w", write.path, err))
		}
	}
	return errors.Join(closeErrors...)
}

// closeVersionIdentities releases descriptor-retaining os.FileInfo values
// returned by the compatibility publication path. Strict identity tokens are
// released by Root.Close instead.
func closeVersionIdentities(identities []os.FileInfo) error {
	var closeErrors []error
	for _, identity := range identities {
		closer, ok := identity.(interface{ Close() error })
		if !ok {
			continue
		}
		if err := closer.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func atomicWritePreparedVersionFile(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
	if write.originalIdentity != nil {
		return write.root.ReplaceFileIfSame(
			write.name,
			write.originalIdentity,
			data,
			write.mode,
			write.preserveMetadata,
		)
	}
	return nil, fmt.Errorf("prepared write source identity unavailable")
}

func atomicWritePreparedVersionFileInfo(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
	if write.originalInfo != nil {
		return write.root.WriteFileIfSameWithInfo(
			write.name,
			data,
			write.mode,
			write.originalInfo,
			write.original,
			write.preserveMetadata,
		)
	}
	return nil, fmt.Errorf("prepared write source metadata unavailable")
}

func portableWritePreparedVersionFile(write preparedVersionWrite, data []byte) (os.FileInfo, error) {
	var err error
	if write.preserveMetadata {
		err = write.root.WriteFilePreservingMode(write.name, data, write.mode)
	} else {
		err = write.root.WriteFile(write.name, data, write.mode)
	}
	if err != nil {
		return nil, err
	}
	file, err := write.root.OpenFile(write.name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	contents, err := io.ReadAll(io.LimitReader(file, int64(len(data))+1))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(contents, data) {
		return nil, fmt.Errorf("portable version write verification failed")
	}
	return info, nil
}

func verifyPortableVersionWriteSource(write preparedVersionWrite, expected []byte) error {
	current, _, err := readRegularVersionFile(&write)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, expected) {
		return fmt.Errorf("source changed before portable fallback")
	}
	return nil
}

func atomicCreatePreparedVersionFile(write preparedVersionWrite, data []byte) (*rootfs.FileIdentity, error) {
	// CreateNewFileAtomicWithIdentity returns the retained staging identity on
	// every supported post-publication failure. If retention itself fails, the
	// rootfs error carries ErrFilePublicationUncertain so callers keep earlier
	// writes in place instead of attempting path-based rollback.
	return write.root.CreateNewFileAtomicWithIdentity(write.name, data, write.mode)
}

func removeCreatedPreparedVersionFile(write preparedVersionWrite) error {
	if write.createdIdentity == nil {
		return fmt.Errorf("created file identity is unavailable")
	}
	if removeCreatedVersionFileBeforeFinalCheckForTest != nil {
		removeCreatedVersionFileBeforeFinalCheckForTest()
	}
	if err := write.root.RemoveFileIfSameIdentity(write.name, write.createdIdentity); err != nil {
		return fmt.Errorf("remove created path safely: %w", err)
	}
	return nil
}

// atomicWriteVersionFile is retained for other Xcode outputs that already
// select an operator-trusted destination path. Version project writes use
// atomicWritePreparedVersionFile so their project-root anchor survives from
// preparation through commit.
func atomicWriteVersionFile(path string, data []byte, mode os.FileMode) error {
	root, err := rootfs.New(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	return root.WriteFile(filepath.Base(path), data, mode)
}

func sortVersionChanges(changes []VersionChange) {
	sort.SliceStable(changes, func(left, right int) bool {
		leftKey := changes[left].Path + "\x00" + changes[left].Target + "\x00" + changes[left].Configuration + "\x00" + changes[left].Setting
		rightKey := changes[right].Path + "\x00" + changes[right].Target + "\x00" + changes[right].Configuration + "\x00" + changes[right].Setting
		return leftKey < rightKey
	})
}
