package xcode

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcodeproj"
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
	path     string
	original []byte
	updated  []byte
	mode     os.FileMode
}

type xcconfigMutation struct {
	setting        string
	value          string
	configurations []*versionConfiguration
}

// GetVersionScoped reads version values from a selected Xcode target and configuration.
func GetVersionScoped(ctx context.Context, opts GetVersionOptions) (*VersionInfo, error) {
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
	return getVersionLegacy(ctx, opts.ProjectDir, opts.Target)
}

// GetConsistentMarketingVersion resolves one marketing version across the full selected mutation scope.
func GetConsistentMarketingVersion(ctx context.Context, opts GetVersionOptions) (string, error) {
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
	legacy, err := getVersionLegacy(ctx, opts.ProjectDir, opts.Target)
	if err != nil {
		return "", err
	}
	return legacy.Version, nil
}

func openStructuredVersionProject(projectInput string) (*structuredVersionProject, error) {
	projectPath, err := findXcodeproj(projectInput)
	if err != nil {
		return nil, err
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
		paths, err := collectXCConfigFiles(root)
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
						if xcconfigMutations[path] == nil {
							xcconfigMutations[path] = make(map[string]xcconfigMutation)
						}
						mutation := xcconfigMutations[path][requested.name]
						mutation.setting = requested.name
						mutation.value = requested.value
						mutation.configurations = appendVersionConfigurationOnce(mutation.configurations, configuration)
						xcconfigMutations[path][requested.name] = mutation
					}
					continue
				}
			}

			if !configuration.projectLevel {
				ancestor := project.projectConfiguration(configuration.name)
				if ancestor != nil && selectedIDs[ancestor.id] && configurationProvidesScheduledSetting(ancestor, requested.name, configFiles, xcconfigMutations) {
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

	var writes []preparedVersionWrite
	if pbxprojChanged {
		write, err := project.preparePBXProjWrite()
		if err != nil {
			return nil, err
		}
		writes = append(writes, write)
	}

	var xcconfigPaths []string
	for path := range xcconfigMutations {
		xcconfigPaths = append(xcconfigPaths, path)
	}
	sort.Strings(xcconfigPaths)
	for _, path := range xcconfigPaths {
		write, fileChanges, changed, err := prepareXCConfigWrite(path, xcconfigMutations[path])
		if err != nil {
			return nil, err
		}
		if changed {
			writes = append(writes, write)
			changes = append(changes, fileChanges...)
		}
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
	mutations map[string]map[string]xcconfigMutation,
) bool {
	if len(matchingBuildSettingKeys(configuration.buildSettings, setting)) > 0 {
		return true
	}
	for _, path := range configFiles[configuration.id] {
		mutation, ok := mutations[path][setting]
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

func (index *xcconfigFileIdentityIndex) identity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	for _, entry := range index.entries {
		if os.SameFile(info, entry.info) {
			return entry.key, nil
		}
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	key := filepath.Clean(absolutePath)
	index.entries = append(index.entries, xcconfigFileIdentity{key: key, info: info})
	return key, nil
}

func (project *structuredVersionProject) xcconfigConsumers(selectedIDs map[string]bool) (map[string]map[string]bool, map[string][]string, map[string]string, bool, error) {
	consumers := make(map[string]map[string]bool)
	configFiles := make(map[string][]string)
	fileIdentities := make(map[string]string)
	identityIndex := xcconfigFileIdentityIndex{}
	uncertainConsumers := false
	for _, configuration := range project.configurations {
		if configuration.baseReferenceID == "" {
			continue
		}
		root, err := project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			if selectedIDs[configuration.id] {
				return nil, nil, nil, false, err
			}
			uncertainConsumers = true
			continue
		}
		files, err := collectXCConfigFiles(root)
		if err != nil {
			if selectedIDs[configuration.id] {
				return nil, nil, nil, false, fmt.Errorf("resolve xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err)
			}
			uncertainConsumers = true
			continue
		}
		configFiles[configuration.id] = files
		for _, path := range files {
			identity, err := identityIndex.identity(path)
			if err != nil {
				if selectedIDs[configuration.id] {
					return nil, nil, nil, false, fmt.Errorf("identify xcconfig for target %q configuration %q: %w", configuration.target, configuration.name, err)
				}
				uncertainConsumers = true
				continue
			}
			fileIdentities[path] = identity
			if consumers[identity] == nil {
				consumers[identity] = make(map[string]bool)
			}
			consumers[identity][configuration.id] = true
		}
	}
	return consumers, configFiles, fileIdentities, uncertainConsumers, nil
}

func xcconfigFilesDefining(paths []string, setting string) ([]string, error) {
	var defining []string
	for _, path := range paths {
		data, err := os.ReadFile(path)
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
		identity, ok := identities[path]
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

func (project *structuredVersionProject) preparePBXProjWrite() (preparedVersionWrite, error) {
	original, mode, err := readRegularVersionFile(project.pbxprojPath)
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
	stagedProject := project.project
	stagedProject.Path = stagedProjectPath
	if err := stagedProject.Save(); err != nil {
		return preparedVersionWrite{}, fmt.Errorf("serialize Xcode project: %w", err)
	}
	updated, err := os.ReadFile(filepath.Join(stagedProjectPath, "project.pbxproj"))
	if err != nil {
		return preparedVersionWrite{}, err
	}
	if _, err := xcodeproj.Open(stagedProjectPath); err != nil {
		return preparedVersionWrite{}, fmt.Errorf("validate staged Xcode project: %w", err)
	}
	return preparedVersionWrite{path: project.pbxprojPath, original: original, updated: updated, mode: mode}, nil
}

func prepareXCConfigWrite(path string, mutations map[string]xcconfigMutation) (preparedVersionWrite, []VersionChange, bool, error) {
	original, mode, err := readRegularVersionFile(path)
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
			return preparedVersionWrite{}, nil, false, fmt.Errorf("edit %s: %w", path, err)
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
				Path:          path,
				Source:        "xcconfig",
			})
		}
	}
	if _, err := parseXCConfig(updated); err != nil {
		return preparedVersionWrite{}, nil, false, fmt.Errorf("validate %s: %w", path, err)
	}
	return preparedVersionWrite{path: path, original: original, updated: updated, mode: mode}, changes, changed, nil
}

func readRegularVersionFile(path string) ([]byte, os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, 0, fmt.Errorf("refusing to replace symlink: %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	return data, info.Mode().Perm(), nil
}

var atomicWriteVersionFileFn = atomicWriteVersionFile

func commitVersionWrites(writes []preparedVersionWrite) error {
	sort.Slice(writes, func(left, right int) bool { return writes[left].path < writes[right].path })
	var committed []preparedVersionWrite
	for _, write := range writes {
		if string(write.original) == string(write.updated) {
			continue
		}
		if err := atomicWriteVersionFileFn(write.path, write.updated, write.mode); err != nil {
			var rollbackErrors []error
			for index := len(committed) - 1; index >= 0; index-- {
				if rollbackErr := atomicWriteVersionFileFn(committed[index].path, committed[index].original, committed[index].mode); rollbackErr != nil {
					rollbackErrors = append(rollbackErrors, fmt.Errorf("restore %s: %w", committed[index].path, rollbackErr))
				}
			}
			writeErr := fmt.Errorf("write %s: %w", write.path, err)
			if len(rollbackErrors) > 0 {
				return errors.Join(writeErr, fmt.Errorf("rollback failed: %w", errors.Join(rollbackErrors...)))
			}
			return writeErr
		}
		committed = append(committed, write)
	}
	return nil
}

func atomicWriteVersionFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".asc-version-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	removeTemporary = false
	if directoryHandle, err := os.Open(directory); err == nil {
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}

func sortVersionChanges(changes []VersionChange) {
	sort.SliceStable(changes, func(left, right int) bool {
		leftKey := changes[left].Path + "\x00" + changes[left].Target + "\x00" + changes[left].Configuration + "\x00" + changes[left].Setting
		rightKey := changes[right].Path + "\x00" + changes[right].Target + "\x00" + changes[right].Configuration + "\x00" + changes[right].Setting
		return leftKey < rightKey
	})
}
