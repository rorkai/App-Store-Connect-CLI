package xcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// signingSettingResolver is the signing workflow's authorization-aware
// counterpart to structuredVersionProject.resolveSetting. Its xcconfig reads
// are limited to paths successfully collected for this plan; this keeps a
// later setting-resolution pass from reopening an unselected or redirected
// external include through the generic resolver.
type signingSettingResolver struct {
	project     *structuredVersionProject
	configFiles map[string][]string
	// lexicalConfigPaths retains every path the collector observed for each
	// configuration, including a directly missing optional include. It is used
	// only to authorize the later no-follow existence check; reads still
	// require membership in authorizedPath (the successfully collected files).
	lexicalConfigPaths map[string][]string
	authorizedPath     map[string]bool
	collectedPaths     map[string]signingAuthorizedPathIndex
	lexicalPaths       map[string]map[string]struct{}
	allowExternal      bool
	// stagedXCConfig contains private bytes for xcconfigs changed by the
	// current planning pass. The map is keyed by normalized lexical path;
	// source existence and authorization remain bound to the collected files.
	stagedXCConfig map[string][]byte
}

func newSigningSettingResolver(project *structuredVersionProject, configFiles map[string][]string, allowExternal bool, lexicalConfigPaths map[string][]string) *signingSettingResolver {
	resolver := &signingSettingResolver{
		project:            project,
		configFiles:        configFiles,
		lexicalConfigPaths: lexicalConfigPaths,
		authorizedPath:     make(map[string]bool),
		collectedPaths:     make(map[string]signingAuthorizedPathIndex, len(configFiles)),
		lexicalPaths:       make(map[string]map[string]struct{}, len(lexicalConfigPaths)),
		allowExternal:      allowExternal,
	}
	for configurationID, paths := range configFiles {
		for _, path := range paths {
			resolver.authorizedPath[normalizeSigningLexicalPath(path)] = true
		}
		resolver.collectedPaths[configurationID] = newSigningAuthorizedPathIndex(paths)
	}
	for configurationID, paths := range lexicalConfigPaths {
		observed := make(map[string]struct{}, len(paths))
		for _, path := range paths {
			observed[normalizeSigningLexicalPath(path)] = struct{}{}
		}
		resolver.lexicalPaths[configurationID] = observed
	}
	return resolver
}

func (resolver *signingSettingResolver) readXCConfig(path string) ([]byte, error) {
	absolute := normalizeSigningLexicalPath(path)
	if !resolver.authorizedPath[normalizeSigningLexicalPath(absolute)] {
		return nil, fmt.Errorf("xcconfig %s was not collected for this signing plan", absolute)
	}
	if err := resolver.authorizeXCConfigPath(absolute); err != nil {
		return nil, err
	}
	if resolver.stagedXCConfig != nil {
		if data, ok := resolver.stagedXCConfig[absolute]; ok {
			return append([]byte(nil), data...), nil
		}
	}
	return signingXCConfigReadFileFn(absolute, signingPlanMaxBytes)
}

// configurationXCConfigPath authorizes a path before any case-volume or
// identity probe, then resolves it only against files collected for the
// supplied configuration. A path collected for a different configuration is
// never readable through this boundary.
func (resolver *signingSettingResolver) configurationXCConfigPath(
	configuration *versionConfiguration,
	path string,
) (string, bool, error) {
	absolute := normalizeSigningLexicalPath(path)
	if err := resolver.authorizeXCConfigPath(absolute); err != nil {
		return absolute, false, err
	}
	if configuration == nil {
		return absolute, false, nil
	}
	collectedPaths := resolver.collectedPaths[configuration.id]
	if _, ok := collectedPaths.exact[absolute]; ok {
		return absolute, true, nil
	}
	for _, collected := range collectedPaths.folded[signingPathCaseFoldKey(absolute)] {
		if !signingPathCaseEquivalent(absolute, collected) {
			continue
		}
		candidateInfo, candidateErr := signingXCConfigIdentityFn(absolute)
		collectedInfo, collectedErr := signingXCConfigIdentityFn(collected)
		if candidateErr == nil && collectedErr == nil && candidateInfo != nil && collectedInfo != nil && os.SameFile(candidateInfo, collectedInfo) {
			return collected, true, nil
		}
	}
	return absolute, false, nil
}

func (resolver *signingSettingResolver) configurationLexicallyObservedPath(configuration *versionConfiguration, path string) bool {
	if configuration == nil {
		return false
	}
	_, ok := resolver.lexicalPaths[configuration.id][normalizeSigningLexicalPath(path)]
	return ok
}

func (resolver *signingSettingResolver) statXCConfigFor(configuration *versionConfiguration, path string) (os.FileInfo, error) {
	absolute := normalizeSigningLexicalPath(path)
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, absolute)
	if err != nil {
		return nil, err
	}
	if !collected {
		if !resolver.configurationLexicallyObservedPath(configuration, absolute) {
			return nil, fmt.Errorf("xcconfig %s was not collected for this signing plan", absolute)
		}
		info, err := signingXCConfigStatFileFn(absolute)
		if err != nil {
			return nil, err
		}
		if info != nil {
			return nil, fmt.Errorf("xcconfig %s appeared after configuration collection", absolute)
		}
		return nil, os.ErrNotExist
	}
	return signingXCConfigStatFileFn(canonical)
}

func (resolver *signingSettingResolver) readXCConfigFor(configuration *versionConfiguration, path string) ([]byte, error) {
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, path)
	if err != nil {
		return nil, err
	}
	if !collected {
		return nil, fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
	}
	return resolver.readXCConfig(canonical)
}

func (resolver *signingSettingResolver) authorizeXCConfigPath(path string) error {
	path = normalizeSigningLexicalPath(path)
	if !resolver.allowExternal {
		root := normalizeSigningLexicalPath(resolver.project.rootDir)
		if !signingNormalizedPathContained(root, path) {
			// A case-folded prefix is only a candidate for the platform-aware
			// containment check. Reject clearly external paths first so an
			// unauthorized Darwin/Windows path cannot trigger case-volume
			// metadata probes.
			if !signingPathCaseFoldedPrefixContained(root, path) || !signingPathLexicallyContained(resolver.project, path) {
				return fmt.Errorf("external xcconfig %s requires --allow-external-xcconfig", path)
			}
		}
	}
	return validateSigningXCConfigPath(resolver.project, path, resolver.allowExternal)
}

func (resolver *signingSettingResolver) resolveSetting(configuration *versionConfiguration, setting string) (string, string, error) {
	return resolver.resolveSettingWithContext(configuration, configuration, setting)
}

// resolveSettingWithContext locates a value in configuration while expanding
// references in expansionConfiguration. A target configuration can inherit a
// value from its project-level configuration, but Xcode evaluates references
// in that inherited value against the target's effective settings.
func (resolver *signingSettingResolver) resolveSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return resolver.expandDirectSettingWithContext(configuration, expansionConfiguration, setting, value, map[string]bool{setting: true})
	}
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, map[string]bool{setting: true})
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if projectConfiguration := resolver.project.projectConfiguration(configuration.name); projectConfiguration != nil {
			return resolver.resolveSettingWithContext(projectConfiguration, expansionConfiguration, setting)
		}
	}
	return "", "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func (resolver *signingSettingResolver) expandDirectSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, value string,
	stack map[string]bool,
) (string, string, error) {
	return resolver.expandDirectAssignmentWithContext(configuration, expansionConfiguration, setting, setting, value, stack)
}

// expandDirectAssignmentWithContext expands one specific assignment of setting.
// assignmentKey names the build-setting key the value was written under, so a
// conditional assignment such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] stays
// distinguishable from the unconditional CODE_SIGN_ENTITLEMENTS slot it
// inherits from. A caller holding an already-effective value, or one that
// cannot name the originating slot, passes setting itself.
func (resolver *signingSettingResolver) expandDirectAssignmentWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, value string,
	stack map[string]bool,
) (string, string, error) {
	return resolver.expandDirectAssignmentWithSourceContext(configuration, expansionConfiguration, setting, assignmentKey, value, stack, false)
}

func (resolver *signingSettingResolver) expandDirectAssignmentWithSourceContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, value string,
	stack map[string]bool,
	xcconfigSource bool,
) (string, string, error) {
	if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
		inherited, err := resolver.resolveInheritedSettingValue(configuration, expansionConfiguration, setting, assignmentKey, value, stack, xcconfigSource)
		if err != nil {
			return "", "", fmt.Errorf("resolve inherited %s: %w", setting, err)
		}
		value = strings.ReplaceAll(value, "$(inherited)", inherited)
		value = strings.ReplaceAll(value, "${inherited}", inherited)
	}
	return resolver.expandSettingReferences(expansionConfiguration, value, stack)
}

func (resolver *signingSettingResolver) resolveInheritedSettingValue(
	configuration, expansionConfiguration *versionConfiguration,
	setting, assignmentKey, currentValue string,
	stack map[string]bool,
	xcconfigSource bool,
) (string, error) {
	// Xcode resolves $(inherited) to the value the setting holds at the next
	// level up for the assignment being expanded. A conditional PBX assignment
	// such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] = $(inherited)Suffix
	// therefore composes through the same object's unconditional
	// CODE_SIGN_ENTITLEMENTS, which only then falls through to the xcconfig and
	// project layers.
	//
	// A conditional assignmentKey names that unconditional slot directly, so
	// identical expression text at different conditions no longer collapses into
	// one resolution. The text inequality stays as the conservative fallback for
	// a caller that cannot name the originating slot; without a key it is the
	// only available guard against recursing on the assignment being expanded.
	// Recursion terminates either way because the nested expansion names the
	// unconditional slot and passes its own text as currentValue.
	if !xcconfigSource {
		if raw, ok := configuration.buildSettings[setting].(string); ok &&
			(assignmentKey != setting || strings.TrimSpace(raw) != strings.TrimSpace(currentValue)) {
			expanded, _, err := resolver.expandDirectAssignmentWithContext(configuration, expansionConfiguration, setting, setting, raw, stack)
			if err != nil {
				return "", err
			}
			return expanded, nil
		}
	}
	inherited, err := resolver.resolveLowerSettingWithContext(configuration, expansionConfiguration, setting)
	if err == nil {
		return inherited, nil
	}
	if errors.Is(err, errVersionSettingNotFound) {
		if implicit, ok := signingImplicitSettingValue(resolver.project, expansionConfiguration, setting); ok {
			return implicit, nil
		}
	}
	if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil && fallback != configuration {
		for _, key := range matchingBuildSettingKeys(fallback.buildSettings, setting) {
			literal, ok := fallback.buildSettings[key].(string)
			if ok && strings.TrimSpace(literal) != "" && !strings.Contains(literal, "$(") && !strings.Contains(literal, "${") {
				return literal, nil
			}
		}
	}
	return "", err
}

func (resolver *signingSettingResolver) resolveLowerSettingWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
) (string, error) {
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, map[string]bool{setting: true})
			return value, err
		}
	}
	if !configuration.projectLevel {
		if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
			value, _, err := resolver.resolveSettingWithContext(fallback, expansionConfiguration, setting)
			return value, err
		}
	}
	return "", fmt.Errorf("%s: %w", setting, errVersionSettingNotFound)
}

func (resolver *signingSettingResolver) resolveSettingReference(configuration *versionConfiguration, setting string, stack map[string]bool) (string, string, error) {
	return resolver.resolveSettingReferenceWithContext(configuration, configuration, setting, stack)
}

func (resolver *signingSettingResolver) resolveSettingReferenceWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	setting string,
	stack map[string]bool,
) (string, string, error) {
	value, ok, err := directBuildSetting(configuration.buildSettings, setting)
	if err != nil {
		return "", "", err
	}
	if ok {
		return resolver.expandDirectSettingWithContext(configuration, expansionConfiguration, setting, value, stack)
	}
	if configuration.baseReferenceID != "" {
		path, err := resolver.project.fileReferencePath(configuration.baseReferenceID)
		if err != nil {
			return "", "", err
		}
		resolved, err := resolver.resolveConfigurationXCConfigWithContext(configuration, expansionConfiguration, path, setting)
		if err != nil {
			return "", "", err
		}
		if resolved.found {
			value, _, err := resolver.expandSettingReferences(expansionConfiguration, resolved.value, stack)
			return value, resolved.path, err
		}
	}
	if !configuration.projectLevel {
		if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
			return resolver.resolveSettingReferenceWithContext(fallback, expansionConfiguration, setting, stack)
		}
	}
	// Only after every explicit pbxproj and xcconfig layer has been searched
	// does the implicit context apply, so a project that assigns one of these
	// names keeps Xcode's precedence.
	if value, ok := signingImplicitSettingValue(resolver.project, expansionConfiguration, setting); ok {
		return value, resolver.project.projectFilePath, nil
	}
	return "", "", fmt.Errorf("setting not found")
}

// signingImplicitSettingValue supplies the build settings Xcode defines for
// every project from the project's own location. Xcode sets them before any
// pbxproj or xcconfig assignment is read, so a reference such as
// $(SRCROOT)/App.entitlements is valid in a project that never assigns SRCROOT
// itself and must not block an otherwise resolvable signing plan.
//
// Only values derivable from the selected .xcodeproj without running a build
// are returned. Anything that depends on a build context - CONFIGURATION,
// PLATFORM_NAME, SDKROOT, BUILT_PRODUCTS_DIR, and every other
// xcodebuild-supplied setting - stays unresolved so the plan fails closed
// instead of guessing which file it inventoried. A resolved path is still
// bound to the project root by the caller's rooted, no-follow containment and
// artifact-alias checks, so an implicit variable cannot widen the plan's
// reach.
func signingImplicitSettingValue(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	setting string,
) (string, bool) {
	if project == nil {
		return "", false
	}
	switch setting {
	case "SRCROOT", "SOURCE_ROOT", "PROJECT_DIR":
		return project.rootDir, project.rootDir != ""
	case "PROJECT_FILE_PATH":
		return project.projectPath, project.projectPath != ""
	case "PROJECT_NAME":
		base := filepath.Base(project.projectPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		if name == "" || name == "." || name == string(filepath.Separator) {
			return "", false
		}
		return name, true
	case "TARGET_NAME":
		// A project-level configuration is shared by every target, so Xcode
		// has no single TARGET_NAME to define there.
		if configuration == nil || configuration.projectLevel || configuration.target == "" {
			return "", false
		}
		return configuration.target, true
	}
	return "", false
}

// resolveXCConfigBaseWithContext returns the lower-layer state that the
// target xcconfig resolver will use. Keeping this lookup shared with the
// resolver prevents the conservative entitlement-inventory pass from
// treating a target-level ?= as live when a project-level exact value already
// supplies the setting.
func (resolver *signingSettingResolver) resolveXCConfigBaseWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	path, setting string,
) (xcconfigResolvedValue, error) {
	base := xcconfigResolvedValue{}
	if configuration == nil || configuration.projectLevel {
		return base, nil
	}
	if fallback := resolver.project.projectConfiguration(configuration.name); fallback != nil {
		value, source, err := resolver.resolveSettingWithContext(fallback, expansionConfiguration, setting)
		if err == nil {
			// resolveSettingWithContext only returns a value after the lower
			// layer has produced a usable result. Preserve the resolver's
			// existing exact bit while exposing the real found state to the
			// conservative assignment scanner.
			base = xcconfigResolvedValue{value: value, path: source, found: true}
		} else if !errors.Is(err, errVersionSettingNotFound) {
			// A direct '=' assignment in the target xcconfig overrides the
			// project-level value and does not depend on it. Probe with and
			// without a private sentinel so only ?=/+=/inherited resolution
			// paths retain a fallback error as a blocker.
			if resolver.xcconfigDependsOnFallback(configuration, path, setting) {
				return xcconfigResolvedValue{}, fmt.Errorf("resolve project-level fallback for %s: %w", setting, err)
			}
		}
	}
	return base, nil
}

func (resolver *signingSettingResolver) identifyXCConfigFor(configuration *versionConfiguration, path string) (os.FileInfo, error) {
	canonical, collected, err := resolver.configurationXCConfigPath(configuration, path)
	if err != nil {
		return nil, err
	}
	if !collected {
		if resolver.configurationLexicallyObservedPath(configuration, path) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
	}
	return signingXCConfigIdentityFn(canonical)
}

func (resolver *signingSettingResolver) resolveXCConfigSettingStateWithContext(
	configuration *versionConfiguration,
	path, setting string,
	observe xcconfigAssignmentObserver,
	base xcconfigResolvedValue,
) (xcconfigResolvedValue, bool, error) {
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		identify = func(includePath string) (os.FileInfo, error) {
			return resolver.identifyXCConfigFor(configuration, includePath)
		}
	}
	return resolveXCConfigSettingState(path, setting, xcconfigResolveOptions{base: base, read: read, stat: stat, identify: identify, observe: observe})
}

func (resolver *signingSettingResolver) resolveConfigurationXCConfigWithContext(
	configuration, expansionConfiguration *versionConfiguration,
	path, setting string,
) (xcconfigResolvedValue, error) {
	base, err := resolver.resolveXCConfigBaseWithContext(configuration, expansionConfiguration, path, setting)
	if err != nil {
		return xcconfigResolvedValue{}, err
	}
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	var identify func(string) (os.FileInfo, error)
	if xcconfigUsesIdentityTraversal() {
		identify = func(includePath string) (os.FileInfo, error) {
			return resolver.identifyXCConfigFor(configuration, includePath)
		}
	}
	return resolveXCConfigSetting(path, setting, xcconfigResolveOptions{
		base:     base,
		read:     read,
		stat:     stat,
		identify: identify,
		lookup: func(name string) (string, bool) {
			return signingImplicitSettingValue(resolver.project, expansionConfiguration, name)
		},
	})
}

// xcconfigDependsOnFallback distinguishes a target xcconfig that semantically
// consumes its project-level base from one that replaces it with a direct
// assignment. The probe is read-only and uses the same authorization-aware
// callbacks as normal resolution. A sentinel is intentionally private: it is
// only compared inside this helper and is never surfaced in a plan or error.
func (resolver *signingSettingResolver) xcconfigDependsOnFallback(configuration *versionConfiguration, path, setting string) bool {
	stat := func(includePath string) (os.FileInfo, error) {
		return resolver.statXCConfigFor(configuration, includePath)
	}
	read := func(includePath string) ([]byte, error) {
		return resolver.readXCConfigFor(configuration, includePath)
	}
	withoutBase, withoutErr := resolveXCConfigSetting(path, setting, xcconfigResolveOptions{read: read, stat: stat})
	const sentinel = "__asc_signing_project_fallback_sentinel__"
	withBase, withErr := resolveXCConfigSetting(path, setting, xcconfigResolveOptions{
		base: xcconfigResolvedValue{value: sentinel, path: resolver.project.projectFilePath, found: true, exact: true},
		read: read,
		stat: stat,
	})
	if withoutErr != nil || withErr != nil {
		// If one probe succeeds and the other does not, the base changes the
		// result. When both fail, the target's own error is returned by the
		// normal resolution below and the fallback error is not needed.
		return (withoutErr == nil) != (withErr == nil)
	}
	return withoutBase.value != withBase.value ||
		withoutBase.found != withBase.found ||
		withoutBase.exact != withBase.exact ||
		withoutBase.missingInherited != withBase.missingInherited
}

func (resolver *signingSettingResolver) expandSettingReferences(configuration *versionConfiguration, value string, stack map[string]bool) (string, string, error) {
	resolved := value
	for iteration := 0; iteration < 32; iteration++ {
		match := signingReferencePattern.FindStringSubmatchIndex(resolved)
		if match == nil {
			if strings.Contains(resolved, "$(") || strings.Contains(resolved, "${") {
				return "", "", fmt.Errorf("incomplete build-setting reference")
			}
			return strings.TrimSpace(resolved), resolver.project.projectFilePath, nil
		}
		name := ""
		if match[2] >= 0 {
			name = resolved[match[2]:match[3]]
		} else {
			name = resolved[match[6]:match[7]]
		}
		if match[4] >= 0 || match[8] >= 0 {
			return "", "", fmt.Errorf("build-setting reference modifier for %s is unsupported", name)
		}
		if stack[name] {
			return "", "", fmt.Errorf("build-setting reference cycle at %s", name)
		}
		nextStack := make(map[string]bool, len(stack)+1)
		for key, set := range stack {
			nextStack[key] = set
		}
		nextStack[name] = true
		replacement, _, err := resolver.resolveSettingReference(configuration, name, nextStack)
		if err != nil {
			return "", "", fmt.Errorf("unresolved build-setting reference %s", name)
		}
		resolved = resolved[:match[0]] + replacement + resolved[match[1]:]
	}
	return "", "", fmt.Errorf("too many nested build-setting references")
}
