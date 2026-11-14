package xcode

import (
	"errors"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"
)

func signingProjectInputPaths(
	project *structuredVersionProject,
	settingsPath string,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	requests []signingRequest,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) ([]string, []string, []string, error) {
	inv := newSigningEntitlementInventory(project, settingsPath, configFiles, fileIdentities, requests, allowExternal, lexicalConfigPaths)
	if err := inv.collect(requests); err != nil {
		return nil, inv.externalEntitlementPaths, inv.inputBlockers, err
	}
	return inv.paths, inv.externalEntitlementPaths, inv.inputBlockers, nil
}

// signingEntitlementInventory collects the files a signing plan may read and
// the entitlement paths it must protect. Paths that failed collection or
// authorization are intentionally not added to the readable input set. The
// caller protects their lexical paths separately, so alias validation never
// stats an unauthorized or missing path.
type signingEntitlementInventory struct {
	project                            *structuredVersionProject
	configFiles                        map[string][]string
	fileIdentities                     map[string]string
	resolver                           *signingSettingResolver
	selectedIDs                        map[string]bool
	selectedXCConfigSources            map[string]bool
	paths                              []string
	externalEntitlementPaths           []string
	inputBlockers                      []string
	uncertainEntitlementConfigurations map[string]bool
	resolvedEntitlementConfigurations  map[string]bool
}

func newSigningEntitlementInventory(
	project *structuredVersionProject,
	settingsPath string,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	requests []signingRequest,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) *signingEntitlementInventory {
	inv := &signingEntitlementInventory{
		project:                            project,
		configFiles:                        configFiles,
		fileIdentities:                     fileIdentities,
		resolver:                           newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths),
		selectedIDs:                        make(map[string]bool, len(requests)),
		selectedXCConfigSources:            make(map[string]bool),
		externalEntitlementPaths:           make([]string, 0),
		inputBlockers:                      make([]string, 0),
		uncertainEntitlementConfigurations: make(map[string]bool),
		resolvedEntitlementConfigurations:  make(map[string]bool),
	}
	inv.paths = make([]string, 0, 2)
	if strings.TrimSpace(settingsPath) != "" {
		inv.paths = append(inv.paths, project.projectFilePath, settingsPath)
	} else {
		inv.paths = []string{project.projectFilePath}
	}
	for _, request := range requests {
		configuration, err := signingConfigurationFor(project, request.target, request.configuration)
		if err == nil {
			inv.selectedIDs[configuration.id] = true
		}
	}
	for _, configuration := range project.configurations {
		if !inv.selectedIDs[configuration.id] {
			continue
		}
		for current := configuration; current != nil; {
			for _, filePath := range configFiles[current.id] {
				inv.selectedXCConfigSources[signingXCConfigOperationKey(filePath, fileIdentities)] = true
			}
			if current.projectLevel {
				break
			}
			current = project.projectConfiguration(current.name)
		}
	}
	return inv
}

func (inv *signingEntitlementInventory) appendEntitlements(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// Raw project/xcconfig scans can encounter the expression that the
	// authorization-aware resolver already expanded. Do not turn that
	// expression into a synthetic filesystem path; a selected unresolved
	// expression is still reported by the resolver above.
	if strings.Contains(value, "$(") || strings.Contains(value, "${") {
		return nil
	}
	if validateSigningRelativePath(value) == nil {
		inv.paths = append(inv.paths, filepath.Join(inv.project.rootDir, filepath.FromSlash(value)))
		return nil
	}
	if filepath.IsAbs(value) {
		absolute := filepath.Clean(value)
		if !signingPathLexicallyContained(inv.project, absolute) {
			inv.externalEntitlementPaths = appendUniqueSigningPaths(inv.externalEntitlementPaths, absolute)
			return nil
		}
		inv.paths = append(inv.paths, absolute)
		return nil
	}
	return fmt.Errorf("CODE_SIGN_ENTITLEMENTS path %q is invalid and cannot be protected", value)
}

func (inv *signingEntitlementInventory) appendLexicalEntitlementCandidate(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// An unresolved inherited expression can still have a concrete path
	// after the inherited token is empty (the Xcode semantics when no lower
	// layer supplies it). Preserve that bounded candidate for lexical alias
	// checks. Any other unresolved reference may expand to an arbitrary path;
	// no finite alias set is sound, so fail before a blocked artifact can be
	// written.
	value = strings.ReplaceAll(value, "$(inherited)", "")
	value = strings.ReplaceAll(value, "${inherited}", "")
	if strings.ContainsRune(value, '\x00') || strings.Contains(value, "$(") || strings.Contains(value, "${") {
		return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried while a build-setting reference is unresolved")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried while its inherited value is unresolved")
	}
	candidate := value
	if !filepath.IsAbs(candidate) && !pathpkg.IsAbs(candidate) && !isWindowsDrivePath(candidate) {
		if err := validateSigningRelativePath(candidate); err != nil {
			return fmt.Errorf("CODE_SIGN_ENTITLEMENTS value cannot be safely inventoried: %w", err)
		}
		candidate = filepath.Join(inv.project.rootDir, filepath.FromSlash(candidate))
	}
	inv.externalEntitlementPaths = appendUniqueSigningPaths(inv.externalEntitlementPaths, filepath.Clean(candidate))
	return nil
}

// appendResolvedEntitlements expands value before appending it. assignmentKey
// names the build-setting key the value was written under. A conditional key
// such as CODE_SIGN_ENTITLEMENTS[sdk=iphoneos*] composes its $(inherited)
// through the object's unconditional assignment; callers holding an
// already-effective value pass "CODE_SIGN_ENTITLEMENTS".
func (inv *signingEntitlementInventory) appendResolvedEntitlements(configuration *versionConfiguration, assignmentKey, value string, xcconfigSource bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.Contains(value, "$(") || strings.Contains(value, "${") {
		if configuration == nil {
			return fmt.Errorf("CODE_SIGN_ENTITLEMENTS reference cannot be resolved without a configuration")
		}
		// Use the same source-aware expansion as direct build settings. In
		// particular, $(inherited) must resolve against the lower project or
		// xcconfig layer rather than being treated as an ordinary setting
		// reference. This keeps raw PBX and xcconfig scans aligned with the
		// effective resolver and prevents an inherited entitlement path from
		// disappearing from the protected-input inventory.
		expanded, _, err := inv.resolver.expandDirectAssignmentWithSourceContext(
			configuration,
			configuration,
			"CODE_SIGN_ENTITLEMENTS",
			assignmentKey,
			value,
			map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
			xcconfigSource,
		)
		if err != nil {
			return err
		}
		value = expanded
	}
	return inv.appendEntitlements(value)
}

func isSigningConditionalEntitlementKey(key string) bool {
	return key != "CODE_SIGN_ENTITLEMENTS" && xcconfigBaseKey(key) == "CODE_SIGN_ENTITLEMENTS"
}

func (inv *signingEntitlementInventory) collect(requests []signingRequest) error {
	for _, files := range inv.configFiles {
		inv.paths = append(inv.paths, files...)
	}
	for _, request := range requests {
		for _, setting := range request.settings {
			if setting.key == "CODE_SIGN_ENTITLEMENTS" && setting.value != nil {
				if err := inv.appendEntitlements(*setting.value); err != nil {
					return err
				}
			}
		}
	}
	for _, configuration := range inv.project.configurations {
		selected := inv.selectedIDs[configuration.id]
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			if _, ok := configuration.buildSettings[key].(string); !ok {
				// Xcode project values can be arbitrary serialized objects, but
				// signing can only safely inventory a scalar entitlement path. Do
				// not silently drop a list/object value: it may name a protected
				// input that would otherwise be overwritten by an artifact.
				return fmt.Errorf("target %q configuration %q has a non-string CODE_SIGN_ENTITLEMENTS value", configuration.target, configuration.name)
			}
		}
		authorized := signingConfigurationSourcesAuthorized(inv.project, configuration, inv.configFiles)
		if configuration.projectLevel {
			// Do not resolve the project configuration itself: it cannot see
			// target-supplied settings such as PRODUCT_NAME. Still inventory
			// literal project assignments, including conditional-only paths,
			// before artifact publication. Expressions are expanded later
			// through each inheriting target configuration.
			for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
				value, ok := configuration.buildSettings[key].(string)
				if !ok || strings.Contains(value, "$(") || strings.Contains(value, "${") {
					continue
				}
				if err := inv.appendEntitlements(value); err != nil {
					return err
				}
			}
			continue
		}
		if authorized {
			value, _, err := inv.resolver.resolveSetting(configuration, "CODE_SIGN_ENTITLEMENTS")
			if err == nil {
				if err := inv.appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", value, false); err != nil {
					if lexicalErr := inv.appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return lexicalErr
					}
					if selected {
						return err
					}
					inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				} else {
					inv.resolvedEntitlementConfigurations[configuration.id] = true
				}
			} else if !errors.Is(err, errVersionSettingNotFound) {
				resolutionErr := fmt.Errorf("resolve CODE_SIGN_ENTITLEMENTS for target %q configuration %q: %w", configuration.target, configuration.name, err)
				if selected {
					return resolutionErr
				}
				// An unselected configuration whose effective entitlement value
				// cannot be resolved may still have more than one semantically
				// possible assignment (for example, an unconditional value and a
				// divergent SDK-conditional value). The later assignment scan must
				// retain every live candidate for alias protection instead of
				// assuming the conditional assignment is the whole input set.
				inv.uncertainEntitlementConfigurations[configuration.id] = true
				inv.inputBlockers = append(inv.inputBlockers, resolutionErr.Error())
			}
		}
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			value, ok := configuration.buildSettings[key].(string)
			if ok {
				if !authorized && !selected && (strings.Contains(value, "$(") || strings.Contains(value, "${")) {
					inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has unresolved CODE_SIGN_ENTITLEMENTS reference; signing scope is uncertain", configuration.target, configuration.name))
					continue
				}
				if err := inv.appendResolvedEntitlements(configuration, key, value, false); err != nil {
					if lexicalErr := inv.appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return lexicalErr
					}
					if isSigningConditionalEntitlementKey(key) {
						if !selected {
							inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
							continue
						}
						return newSigningConditionalEntitlementError(err)
					}
					if selected {
						return err
					}
					inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				}
			}
		}
		// An unconditional inherited target assignment can consume any
		// project-level entitlement assignment when the project fallback is
		// genuinely uncertain. The effective resolver intentionally reports one
		// value (or falls back to its first literal) for callers that need a
		// single setting, but the artifact alias inventory must retain every
		// concrete composition that Xcode may select. Same-object conditional
		// assignments inherit through the target's unconditional slot, and a
		// target xcconfig can provide the lower value, so neither case should be
		// cross-composed with project seeds here.
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			if key != "CODE_SIGN_ENTITLEMENTS" {
				continue
			}
			value, ok := configuration.buildSettings[key].(string)
			if !ok || !signingValueInherits(value) {
				continue
			}
			if _, err := inv.resolver.resolveLowerSettingWithContext(configuration, configuration, "CODE_SIGN_ENTITLEMENTS"); err == nil {
				continue
			}
			if files := inv.configFiles[configuration.id]; len(files) > 0 {
				if signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(configuration, files, inv.resolver) {
					continue
				}
			}
			for _, projectCfg := range inv.project.configurations {
				if !projectCfg.projectLevel || projectCfg.name != configuration.name {
					continue
				}
				for _, projectKey := range matchingBuildSettingKeys(projectCfg.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
					projectValue, ok := projectCfg.buildSettings[projectKey].(string)
					if !ok {
						continue
					}
					if strings.Contains(projectValue, "$(") || strings.Contains(projectValue, "${") {
						expanded, _, expandErr := inv.resolver.expandDirectAssignmentWithContext(
							projectCfg,
							configuration,
							"CODE_SIGN_ENTITLEMENTS",
							projectKey,
							projectValue,
							map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
						)
						if expandErr != nil {
							// The later project-expression scan applies the same
							// fail-closed lexical handling for an unresolved seed.
							// Do not invent a composed path here when the target
							// context cannot resolve the project reference.
							continue
						}
						projectValue = expanded
					}
					composed := strings.ReplaceAll(value, "$(inherited)", projectValue)
					composed = strings.ReplaceAll(composed, "${inherited}", projectValue)
					if err := inv.appendResolvedEntitlements(configuration, key, composed, false); err != nil {
						if lexicalErr := inv.appendLexicalEntitlementCandidate(composed); lexicalErr != nil {
							return lexicalErr
						}
						if selected {
							return err
						}
						inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
					}
				}
				projectFiles := inv.configFiles[projectCfg.id]
				if signingConfigurationDefinesUnconditionalPBXEntitlement(projectCfg) {
					continue
				}
				if len(projectFiles) == 0 {
					continue
				}
				projectValues, projectErr := signingProjectXCConfigEntitlementValues(projectCfg, configuration, projectFiles, inv.resolver)
				if projectErr != nil {
					return fmt.Errorf("resolve project-level CODE_SIGN_ENTITLEMENTS for target %q configuration %q: %w", configuration.target, configuration.name, projectErr)
				}
				for _, projectValue := range projectValues {
					composed := strings.ReplaceAll(value, "$(inherited)", projectValue)
					composed = strings.ReplaceAll(composed, "${inherited}", projectValue)
					if err := inv.appendResolvedEntitlements(configuration, key, composed, false); err != nil {
						if lexicalErr := inv.appendLexicalEntitlementCandidate(composed); lexicalErr != nil {
							return lexicalErr
						}
						if selected {
							return err
						}
						inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
					}
				}
			}
		}
		targetDefinesUnconditional := signingConfigurationDefinesUnconditionalEntitlement(inv.project, configuration, inv.configFiles, inv.resolver)
		if targetDefinesUnconditional {
			continue
		}
		for _, projectCfg := range inv.project.configurations {
			if !projectCfg.projectLevel || projectCfg.name != configuration.name {
				continue
			}
			for _, key := range matchingBuildSettingKeys(projectCfg.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
				value, ok := projectCfg.buildSettings[key].(string)
				if !ok || (!strings.Contains(value, "$(") && !strings.Contains(value, "${")) {
					continue
				}
				expanded, _, err := inv.resolver.expandDirectAssignmentWithContext(
					projectCfg,
					configuration,
					"CODE_SIGN_ENTITLEMENTS",
					key,
					value,
					map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
				)
				if err == nil {
					err = inv.appendEntitlements(expanded)
				}
				if err != nil {
					if lexicalErr := inv.appendLexicalEntitlementCandidate(value); lexicalErr != nil {
						return lexicalErr
					}
					if isSigningConditionalEntitlementKey(key) {
						if !selected {
							inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited project CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
							continue
						}
						return newSigningConditionalEntitlementError(err)
					}
					if selected {
						return err
					}
					inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved inherited project CODE_SIGN_ENTITLEMENTS input: %v", configuration.target, configuration.name, err))
				}
			}
		}
	}
	entitlementCandidatesByConfiguration := make(map[string]map[string]map[int]bool)
	entitlementAssignmentExpansionsByConfiguration := make(map[string]map[string]map[int]signingXCConfigEntitlementAssignmentExpansion)
	configurationsByID := make(map[string]*versionConfiguration, len(inv.project.configurations))
	for _, configuration := range inv.project.configurations {
		configurationsByID[configuration.id] = configuration
	}
	for configurationID := range inv.uncertainEntitlementConfigurations {
		files := inv.configFiles[configurationID]
		if len(files) == 0 {
			continue
		}
		base := xcconfigResolvedValue{}
		if configuration := configurationsByID[configurationID]; configuration != nil {
			// Reuse the same lower-layer resolution state as the effective
			// resolver. If that state is itself uncertain, retaining all
			// candidates is the fail-closed choice.
			resolvedBase, baseErr := inv.resolver.resolveXCConfigBaseWithContext(
				configuration,
				configuration,
				files[0],
				"CODE_SIGN_ENTITLEMENTS",
			)
			if baseErr == nil {
				base = resolvedBase
			}
		}
		candidates, composed, expansions, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
			configurationsByID[configurationID], files, inv.resolver, base,
		)
		if err != nil {
			return err
		}
		entitlementCandidatesByConfiguration[configurationID] = candidates
		entitlementAssignmentExpansionsByConfiguration[configurationID] = expansions
		configuration := configurationsByID[configurationID]
		for _, value := range composed {
			if err := inv.appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", value, true); err != nil {
				if lexicalErr := inv.appendLexicalEntitlementCandidate(value); lexicalErr != nil {
					return lexicalErr
				}
			}
		}
	}
	knownConfigurationIDs := make(map[string]bool, len(inv.project.configurations))
	projectEntitlementCandidatesByConfiguration := make(map[string]map[string]map[int]bool)
	for _, configuration := range inv.project.configurations {
		knownConfigurationIDs[configuration.id] = true
		files, ok := inv.configFiles[configuration.id]
		if !ok {
			continue
		}
		for _, filePath := range files {
			// These paths came from the successful collector, but keep the
			// membership and authorization checks on this later read as well.
			// This prevents a future caller from turning configFiles into an
			// ambient path list that bypasses the signing resolver's rooted,
			// no-follow policy.
			data, err := inv.resolver.readXCConfig(filePath)
			if err != nil {
				return fmt.Errorf("read xcconfig %s: %w", filePath, err)
			}
			document, err := parseXCConfig(data)
			if err != nil {
				return fmt.Errorf("parse xcconfig %s: %w", filePath, err)
			}
			for _, assignment := range document.assignments {
				selected := inv.selectedIDs[configuration.id]
				selectedSource := inv.selectedXCConfigSources[signingXCConfigOperationKey(filePath, inv.fileIdentities)]
				if assignment.continued && allowedSigningSetting(assignment.baseKey) &&
					(assignment.baseKey == "CODE_SIGN_ENTITLEMENTS" || selectedSource) {
					return fmt.Errorf("xcconfig %s uses a line continuation for signing setting %s", filePath, assignment.baseKey)
				}
				if inv.resolvedEntitlementConfigurations[configuration.id] && assignment.baseKey == "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if configuration.projectLevel && signingConfigurationDefinesUnconditionalPBXEntitlement(configuration) {
					continue
				}
				if configuration.projectLevel {
					candidates, cached := projectEntitlementCandidatesByConfiguration[configuration.id]
					if !cached {
						var candidateErr error
						candidates, _, _, candidateErr = signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
							configuration,
							files,
							inv.resolver,
							xcconfigResolvedValue{},
						)
						if candidateErr != nil {
							return candidateErr
						}
						projectEntitlementCandidatesByConfiguration[configuration.id] = candidates
					}
					if !candidates[normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
						continue
					}
				}
				if configuration.projectLevel && (strings.Contains(assignment.value, "$(") || strings.Contains(assignment.value, "${")) {
					expandedForTarget := false
					targetExpansionFailed := false
					matchingTargetCount := 0
					shadowedTargetCount := 0
					for _, targetConfiguration := range inv.project.configurations {
						if targetConfiguration.projectLevel || targetConfiguration.name != configuration.name {
							continue
						}
						matchingTargetCount++
						if signingConfigurationDefinesUnconditionalEntitlement(inv.project, targetConfiguration, inv.configFiles, inv.resolver) {
							shadowedTargetCount++
							continue
						}
						expanded, _, expandErr := inv.resolver.expandDirectAssignmentWithSourceContext(
							configuration,
							targetConfiguration,
							"CODE_SIGN_ENTITLEMENTS",
							assignment.key,
							assignment.value,
							map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
							true,
						)
						if expandErr != nil {
							targetExpansionFailed = true
							continue
						}
						projectSeeds, projectSeedErr := signingProjectPBXInheritedEntitlementValues(
							configuration,
							targetConfiguration,
							expanded,
							inv.resolver,
						)
						if projectSeedErr != nil {
							targetExpansionFailed = true
							continue
						}
						for _, projectSeed := range projectSeeds {
							targetValues, targetInherited, targetValueErr := signingConfigurationInheritedEntitlementValues(
								targetConfiguration,
								inv.configFiles,
								inv.resolver,
								projectSeed,
							)
							if targetValueErr != nil {
								targetExpansionFailed = true
								continue
							}
							if !targetInherited {
								targetValues = []string{projectSeed}
							}
							for _, targetValue := range targetValues {
								if err := inv.appendEntitlements(targetValue); err != nil {
									return err
								}
							}
						}
						expandedForTarget = true
					}
					if matchingTargetCount > 0 && shadowedTargetCount == matchingTargetCount {
						continue
					}
					if targetExpansionFailed {
						if lexicalErr := inv.appendLexicalEntitlementCandidate(assignment.value); lexicalErr != nil {
							return lexicalErr
						}
						continue
					}
					if expandedForTarget {
						continue
					}
				}
				if !isSigningConditionalEntitlementKey(assignment.key) {
					if !inv.uncertainEntitlementConfigurations[configuration.id] {
						continue
					}
					if !entitlementCandidatesByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
						continue
					}
				} else if inv.uncertainEntitlementConfigurations[configuration.id] &&
					!entitlementCandidatesByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
					continue
				}
				// The xcconfig layer sits below these PBX settings, so the
				// object's unconditional assignment is not this assignment's
				// inherited base. Selector-aware composition inside an xcconfig
				// is handled by signingXCConfigEntitlementAssignmentCandidates.
				// For an uncertain configuration, that pass has already expanded
				// every live assignment against the preceding xcconfig/project
				// state. Re-resolving an inherited raw assignment through the
				// configuration's whole xcconfig would use the final value of this
				// same file as its base and compose the suffix twice.
				rawValue := assignment.value
				if expansion := entitlementAssignmentExpansionsByConfiguration[configuration.id][normalizeSigningLexicalPath(filePath)][assignment.lineIndex]; expansion.inheritedResolved {
					rawValue = expansion.value
				}
				if err := inv.appendResolvedEntitlements(configuration, "CODE_SIGN_ENTITLEMENTS", rawValue, true); err != nil {
					if lexicalErr := inv.appendLexicalEntitlementCandidate(assignment.value); lexicalErr != nil {
						return lexicalErr
					}
					if !selected {
						inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("target %q configuration %q has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", configuration.target, configuration.name))
						continue
					}
					return newSigningConditionalEntitlementError(err)
				}
			}
		}
	}
	// configFiles is normally keyed only by configurations parsed from the
	// project. Keep this boundary fail-closed for injected or future collector
	// results as well: an unassociated source cannot be safely classified as an
	// unselected consumer, so it must remain readable and representable rather
	// than being silently omitted from the protected-input inventory.
	for configurationID, files := range inv.configFiles {
		if knownConfigurationIDs[configurationID] {
			continue
		}
		for _, filePath := range files {
			data, err := inv.resolver.readXCConfig(filePath)
			if err != nil {
				return fmt.Errorf("read xcconfig %s: %w", filePath, err)
			}
			document, err := parseXCConfig(data)
			if err != nil {
				return fmt.Errorf("parse xcconfig %s: %w", filePath, err)
			}
			for _, assignment := range document.assignments {
				if assignment.continued && allowedSigningSetting(assignment.baseKey) {
					return fmt.Errorf("xcconfig %s uses a line continuation for signing setting %s", filePath, assignment.baseKey)
				}
				if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
					continue
				}
				if strings.Contains(assignment.value, "$(") || strings.Contains(assignment.value, "${") {
					if isSigningConditionalEntitlementKey(assignment.key) {
						inv.inputBlockers = append(inv.inputBlockers, fmt.Sprintf("xcconfig %s has an unresolved conditional CODE_SIGN_ENTITLEMENTS input; signing scope is uncertain", filePath))
						continue
					}
				}
				if err := inv.appendEntitlements(assignment.value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
