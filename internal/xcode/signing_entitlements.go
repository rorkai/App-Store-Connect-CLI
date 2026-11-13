package xcode

import (
	"fmt"
	"sort"
	"strings"
)

func signingConfigurationDefinesUnconditionalEntitlement(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	configFiles map[string][]string,
	resolver *signingSettingResolver,
) bool {
	value, definesUnconditional := configuration.buildSettings["CODE_SIGN_ENTITLEMENTS"].(string)
	if definesUnconditional && signingValueInherits(value) {
		definesUnconditional = false
	}
	if !definesUnconditional && signingConfigurationSourcesAuthorized(project, configuration, configFiles) {
		if files := configFiles[configuration.id]; len(files) > 0 {
			definesUnconditional = signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(configuration, files, resolver)
		}
	}
	return definesUnconditional
}

func signingConfigurationDefinesUnconditionalPBXEntitlement(configuration *versionConfiguration) bool {
	value, ok := configuration.buildSettings["CODE_SIGN_ENTITLEMENTS"].(string)
	return ok && !signingValueInherits(value)
}

// signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride keeps
// project fallback protection only when the target xcconfig's effective
// unconditional slot is a literal replacement. The sentinel probe rejects
// effective inherited values, while the candidate traversal excludes
// conditional-only assignments without replaying the xcconfig graph in a
// second resolver.
func signingConfigurationXCConfigHasLiveUnconditionalEntitlementOverride(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) bool {
	if len(files) == 0 || resolver.xcconfigDependsOnFallback(configuration, files[0], "CODE_SIGN_ENTITLEMENTS") {
		return false
	}
	candidates, _, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return false
	}
	for _, filePath := range files {
		data, readErr := resolver.readXCConfig(filePath)
		if readErr != nil {
			return false
		}
		document, parseErr := parseXCConfig(data)
		if parseErr != nil {
			return false
		}
		for _, assignment := range document.assignments {
			if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" ||
				assignment.key != "CODE_SIGN_ENTITLEMENTS" ||
				!candidates[normalizeSigningLexicalPath(filePath)][assignment.lineIndex] {
				continue
			}
			if signingValueInherits(assignment.value) {
				continue
			}
			if assignment.operator == "=" {
				return true
			}
		}
	}
	return false
}

// signingXCConfigSelectorIdentity canonicalizes only the order of bracketed
// selector conditions. Selector names and values remain byte-for-byte (and
// therefore case- and wildcard-) sensitive, because those details affect the
// build context in which Xcode applies an assignment.
func signingXCConfigSelectorIdentity(key string) string {
	base := xcconfigBaseKey(key)
	remainder := key[len(base):]
	if remainder == "" {
		return key
	}
	selectors := make([]string, 0, 2)
	for remainder != "" {
		if remainder[0] != '[' {
			return key
		}
		end := strings.IndexByte(remainder, ']')
		if end < 0 {
			return key
		}
		selectors = append(selectors, remainder[:end+1])
		remainder = remainder[end+1:]
	}
	sort.Strings(selectors)
	return base + strings.Join(selectors, "")
}

type signingXCConfigEntitlementAssignmentExpansion struct {
	value             string
	inheritedResolved bool
}

// signingProjectXCConfigEntitlementValues returns the concrete project-level
// entitlement values that an inherited target assignment can consume. The
// source configuration owns the xcconfig graph while the target configuration
// owns build-setting reference expansion; keeping those contexts separate
// prevents a target's xcconfig from being combined with a sibling project
// configuration while still resolving references such as $(PRODUCT_NAME) for
// the target that will inherit the value.
func signingProjectXCConfigEntitlementValues(
	projectConfiguration, targetConfiguration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	_, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		projectConfiguration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0, len(composed))
	seen := make(map[string]bool, len(composed))
	for _, value := range composed {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if strings.Contains(value, "$(") || strings.Contains(value, "${") {
			expanded, _, expandErr := resolver.expandSettingReferences(
				targetConfiguration,
				value,
				map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
			)
			if expandErr != nil {
				// The ordinary project xcconfig scan retains the raw assignment
				// for lexical/fail-closed handling when target context cannot
				// expand a reference. Do not guess a composed path here.
				continue
			}
			value = expanded
		}
		if !seen[value] {
			seen[value] = true
			values = append(values, value)
		}
	}
	if signingConfigurationDefinesUnconditionalPBXEntitlement(projectConfiguration) {
		return nil, nil
	}
	projectComposed := make([]string, 0, len(values))
	seen = make(map[string]bool, len(values))
	for _, value := range values {
		projectValues, err := signingProjectPBXInheritedEntitlementValues(
			projectConfiguration,
			targetConfiguration,
			value,
			resolver,
		)
		if err != nil {
			return nil, err
		}
		for _, projectValue := range projectValues {
			if !seen[projectValue] {
				seen[projectValue] = true
				projectComposed = append(projectComposed, projectValue)
			}
		}
	}
	return projectComposed, nil
}

type signingPBXInheritedEntitlementAssignment struct {
	key   string
	value string
}

// signingPBXInheritedEntitlementValues composes raw PBX inherited
// assignments through a supplied lower-layer value. Xcode evaluates a
// conditional assignment's $(inherited) against the same object's effective
// unconditional slot, so conditional assignments must consume the computed
// unconditional values rather than the lower layer directly. The raw key is
// retained for the resolver call; this preserves its assignment-aware
// recursion and avoids a second syntax-level composition pass.
func signingPBXInheritedEntitlementValues(
	configuration, expansionConfiguration *versionConfiguration,
	lowerValues []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	normalizedLowerValues := make([]string, 0, len(lowerValues))
	seenLower := make(map[string]bool, len(lowerValues))
	for _, value := range lowerValues {
		value = strings.TrimSpace(value)
		if !seenLower[value] {
			seenLower[value] = true
			normalizedLowerValues = append(normalizedLowerValues, value)
		}
	}
	if len(normalizedLowerValues) == 0 {
		normalizedLowerValues = []string{""}
	}

	assignments := make([]signingPBXInheritedEntitlementAssignment, 0, len(configuration.buildSettings))
	for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
		value, ok := configuration.buildSettings[key].(string)
		if ok && signingValueInherits(value) {
			assignments = append(assignments, signingPBXInheritedEntitlementAssignment{key: key, value: value})
		}
	}
	if len(assignments) == 0 {
		return normalizedLowerValues, nil
	}

	expand := func(assignment signingPBXInheritedEntitlementAssignment, lower string) (string, error) {
		seededConfiguration := *configuration
		seededConfiguration.buildSettings = cloneSigningSerializedObject(configuration.buildSettings)
		seededConfiguration.buildSettings["CODE_SIGN_ENTITLEMENTS"] = lower
		expanded, _, err := resolver.expandDirectAssignmentWithContext(
			&seededConfiguration,
			expansionConfiguration,
			"CODE_SIGN_ENTITLEMENTS",
			assignment.key,
			assignment.value,
			map[string]bool{"CODE_SIGN_ENTITLEMENTS": true},
		)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(expanded), nil
	}

	// The unconditional assignment establishes the same-object state that a
	// conditional assignment inherits. With no unconditional assignment, a
	// conditional slot falls through to the supplied lower-layer values.
	unconditionalValues := make([]string, 0, len(normalizedLowerValues))
	seenUnconditional := make(map[string]bool, len(normalizedLowerValues))
	for _, assignment := range assignments {
		if assignment.key != "CODE_SIGN_ENTITLEMENTS" {
			continue
		}
		for _, lower := range normalizedLowerValues {
			expanded, err := expand(assignment, lower)
			if err != nil {
				return nil, err
			}
			if expanded != "" && !seenUnconditional[expanded] {
				seenUnconditional[expanded] = true
				unconditionalValues = append(unconditionalValues, expanded)
			}
		}
	}
	if len(unconditionalValues) == 0 {
		unconditionalValues = normalizedLowerValues
	}

	composed := make([]string, 0, len(assignments)*len(unconditionalValues))
	seen := make(map[string]bool, len(composed))
	for _, assignment := range assignments {
		if assignment.key == "CODE_SIGN_ENTITLEMENTS" {
			for _, value := range unconditionalValues {
				if !seen[value] {
					seen[value] = true
					composed = append(composed, value)
				}
			}
			continue
		}
		for _, lower := range unconditionalValues {
			expanded, err := expand(assignment, lower)
			if err != nil {
				return nil, err
			}
			if expanded != "" && !seen[expanded] {
				seen[expanded] = true
				composed = append(composed, expanded)
			}
		}
	}
	if len(composed) == 0 {
		return normalizedLowerValues, nil
	}
	return composed, nil
}

// signingProjectPBXInheritedEntitlementValues composes a project xcconfig
// seed through the project's raw PBX entitlement assignment. Project PBX
// inherited suffixes sit above the project xcconfig and therefore must be
// applied before a target's inherited suffix is considered.
func signingProjectPBXInheritedEntitlementValues(
	projectConfiguration, targetConfiguration *versionConfiguration,
	projectSeed string,
	resolver *signingSettingResolver,
) ([]string, error) {
	if signingConfigurationDefinesUnconditionalPBXEntitlement(projectConfiguration) {
		return nil, nil
	}
	return signingPBXInheritedEntitlementValues(
		projectConfiguration,
		targetConfiguration,
		[]string{projectSeed},
		resolver,
	)
}

// signingXCConfigInheritedEntitlementValues returns raw target-xcconfig
// entitlement assignments that remain live and still consume a lower-layer
// value. The candidate traversal supplies the same replacement semantics used
// by the uncertain-configuration inventory, so a shadowed assignment cannot
// re-enter target-context composition merely because it was parsed.
func signingXCConfigInheritedEntitlementValues(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	candidates, _, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration,
		files,
		resolver,
		xcconfigResolvedValue{},
	)
	if err != nil {
		return nil, err
	}
	values := make([]string, 0)
	seen := make(map[string]bool)
	for _, filePath := range files {
		data, readErr := resolver.readXCConfig(filePath)
		if readErr != nil {
			return nil, readErr
		}
		document, parseErr := parseXCConfig(data)
		if parseErr != nil {
			return nil, fmt.Errorf("parse %s: %w", filePath, parseErr)
		}
		path := normalizeSigningLexicalPath(filePath)
		for _, assignment := range document.assignments {
			if assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" ||
				(!signingValueInherits(assignment.value) && assignment.operator != "+=") ||
				!candidates[path][assignment.lineIndex] {
				continue
			}
			if !seen[assignment.value] {
				seen[assignment.value] = true
				values = append(values, assignment.value)
			}
		}
	}
	return values, nil
}

// signingConfigurationInheritedEntitlementValues composes one concrete
// project seed through the target's raw inherited entitlement sources. The
// target xcconfig graph is resolved with the supplied seed as its lower layer
// before any PBX inherited suffix is appended, which keeps a target-xcconfig
// suffix and a PBX suffix in their actual order and avoids protecting the bare
// project reference as a synthetic path.
func signingConfigurationInheritedEntitlementValues(
	configuration *versionConfiguration,
	configFiles map[string][]string,
	resolver *signingSettingResolver,
	projectSeed string,
) ([]string, bool, error) {
	values := []string{strings.TrimSpace(projectSeed)}
	targetXCConfigValues := values
	if files := configFiles[configuration.id]; len(files) > 0 {
		_, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
			configuration,
			files,
			resolver,
			xcconfigResolvedValue{value: projectSeed, found: true, exact: true},
		)
		if err != nil {
			return nil, false, err
		}
		targetXCConfigValues = make([]string, 0, len(composed))
		seen := make(map[string]bool, len(composed))
		for _, value := range composed {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if strings.Contains(value, "$(") || strings.Contains(value, "${") {
				value, _, err = resolver.expandSettingReferences(configuration, value, map[string]bool{"CODE_SIGN_ENTITLEMENTS": true})
				if err != nil {
					return nil, false, err
				}
			}
			if !seen[value] {
				seen[value] = true
				targetXCConfigValues = append(targetXCConfigValues, value)
			}
		}
		if len(targetXCConfigValues) == 0 {
			targetXCConfigValues = values
		}
	}

	if targetValues, err := signingXCConfigInheritedEntitlementValues(configuration, configFiles[configuration.id], resolver); err != nil {
		return nil, false, err
	} else {
		pbxInherited := make([]string, 0)
		for _, key := range matchingBuildSettingKeys(configuration.buildSettings, "CODE_SIGN_ENTITLEMENTS") {
			value, ok := configuration.buildSettings[key].(string)
			if ok && signingValueInherits(value) {
				pbxInherited = append(pbxInherited, value)
			}
		}
		if len(pbxInherited) == 0 {
			return targetXCConfigValues, len(targetValues) > 0, nil
		}
		composed, err := signingPBXInheritedEntitlementValues(
			configuration,
			configuration,
			targetXCConfigValues,
			resolver,
		)
		if err != nil {
			return nil, true, err
		}
		return composed, true, nil
	}
}

// signingXCConfigEntitlementAssignmentCandidates returns the assignments that
// can still contribute to CODE_SIGN_ENTITLEMENTS when effective resolution is
// uncertain. It uses the configuration-scoped resolver's event traversal and
// rooted read/stat/identity callbacks; it does not replay a global path list or
// implement a second include walker.
func signingXCConfigEntitlementAssignmentCandidates(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
	base xcconfigResolvedValue,
) (map[string]map[int]bool, []string, error) {
	candidates, composed, _, err := signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
		configuration, files, resolver, base,
	)
	return candidates, composed, err
}

func signingXCConfigEntitlementAssignmentCandidatesWithExpansions(
	configuration *versionConfiguration,
	files []string,
	resolver *signingSettingResolver,
	base xcconfigResolvedValue,
) (map[string]map[int]bool, []string, map[string]map[int]signingXCConfigEntitlementAssignmentExpansion, error) {
	type assignmentReference struct {
		path        string
		assignment  xcconfigAssignment
		selectorKey string
	}
	var active []assignmentReference
	// A found lower-layer value shadows a target-level ?= even when the
	// resolver's exact bit is false (for example, a value inherited from a
	// project xcconfig). A lower-layer resolution error intentionally supplies
	// an empty state so every target assignment remains protected.
	hasExactValue := base.found
	if len(files) == 0 {
		return map[string]map[int]bool{}, nil, nil, nil
	}
	var observerErr error
	observe := func(path string, assignment xcconfigAssignment) {
		if observerErr != nil || assignment.baseKey != "CODE_SIGN_ENTITLEMENTS" {
			return
		}
		canonicalPath, collected, err := resolver.configurationXCConfigPath(configuration, path)
		if err != nil {
			observerErr = err
			return
		}
		if !collected {
			observerErr = fmt.Errorf("xcconfig %s was not collected for this configuration", normalizeSigningLexicalPath(path))
			return
		}
		candidate := assignmentReference{
			path:        canonicalPath,
			assignment:  assignment,
			selectorKey: signingXCConfigSelectorIdentity(assignment.key),
		}
		if assignment.key == "CODE_SIGN_ENTITLEMENTS" {
			switch assignment.operator {
			case "?=":
				if hasExactValue {
					return
				}
			case "=":
				if !strings.Contains(assignment.value, "$(inherited)") && !strings.Contains(assignment.value, "${inherited}") {
					filtered := active[:0]
					for _, existing := range active {
						if existing.assignment.key == "CODE_SIGN_ENTITLEMENTS" || existing.assignment.operator == "?=" {
							continue
						}
						filtered = append(filtered, existing)
					}
					active = filtered
				}
			case "+=":
				// The prior exact value remains part of an append operation;
				// retain both path intents for conservative alias protection.
			}
			hasExactValue = true
			active = append(active, candidate)
			return
		}
		if assignment.operator == "?=" {
			if hasExactValue {
				return
			}
			for _, existing := range active {
				if existing.selectorKey == candidate.selectorKey {
					// A prior same-selector assignment wins over a later
					// conditional default; ?= never replaces a live value.
					return
				}
			}
		} else if assignment.operator != "+=" && !strings.Contains(assignment.value, "$(inherited)") && !strings.Contains(assignment.value, "${inherited}") {
			// An exact same-selector assignment replaces the earlier value.
			// Reordered conditions share selector identity, while different
			// selectors remain possible in other build contexts.
			filtered := active[:0]
			for _, existing := range active {
				if existing.selectorKey == candidate.selectorKey {
					continue
				}
				filtered = append(filtered, existing)
			}
			active = filtered
		}
		active = append(active, candidate)
	}
	resolved, _, err := resolver.resolveXCConfigSettingStateWithContext(
		configuration,
		files[0],
		"CODE_SIGN_ENTITLEMENTS",
		observe,
		base,
	)
	if err != nil {
		return nil, nil, nil, err
	}
	if observerErr != nil {
		return nil, nil, nil, observerErr
	}
	candidates := make(map[string]map[int]bool)
	expansions := make(map[string]map[int]signingXCConfigEntitlementAssignmentExpansion)
	for _, candidate := range active {
		path := normalizeSigningLexicalPath(candidate.path)
		if candidates[path] == nil {
			candidates[path] = make(map[int]bool)
		}
		candidates[path][candidate.assignment.lineIndex] = true
		if expansions[path] == nil {
			expansions[path] = make(map[int]signingXCConfigEntitlementAssignmentExpansion)
		}
	}
	composed := make([]string, 0)
	if strings.TrimSpace(resolved.value) != "" {
		composed = append(composed, resolved.value)
	}
	accumulated := map[string]string{}
	accumulatedKnown := map[string]bool{}
	if base.found {
		accumulated["CODE_SIGN_ENTITLEMENTS"] = strings.TrimSpace(base.value)
		accumulatedKnown["CODE_SIGN_ENTITLEMENTS"] = true
	}
	lookupAccumulated := func(selector string) (string, bool) {
		previous, known := accumulated[selector], accumulatedKnown[selector]
		if !known && previous == "" && selector != "CODE_SIGN_ENTITLEMENTS" {
			previous = accumulated["CODE_SIGN_ENTITLEMENTS"]
			known = accumulatedKnown["CODE_SIGN_ENTITLEMENTS"]
		}
		return previous, known
	}
	for _, candidate := range active {
		selector := candidate.selectorKey
		value := strings.TrimSpace(candidate.assignment.value)
		inheritedResolved := false
		switch candidate.assignment.operator {
		case "+=":
			previous, previousKnown := lookupAccumulated(selector)
			if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
				inheritedResolved = previousKnown
				value = strings.ReplaceAll(value, "$(inherited)", previous)
				value = strings.ReplaceAll(value, "${inherited}", previous)
				accumulated[selector] = strings.TrimSpace(value)
				accumulatedKnown[selector] = true
				break
			}
			accumulated[selector] = strings.TrimSpace(strings.TrimSpace(previous) + " " + value)
			accumulatedKnown[selector] = true
		case "?=":
			if !accumulatedKnown[selector] {
				accumulated[selector] = value
				accumulatedKnown[selector] = true
			}
		default:
			if strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}") {
				previous, previousKnown := lookupAccumulated(selector)
				inheritedResolved = previousKnown
				value = strings.ReplaceAll(value, "$(inherited)", previous)
				value = strings.ReplaceAll(value, "${inherited}", previous)
			}
			accumulated[selector] = strings.TrimSpace(value)
			accumulatedKnown[selector] = true
		}
		path := normalizeSigningLexicalPath(candidate.path)
		if expansions[path] == nil {
			expansions[path] = make(map[int]signingXCConfigEntitlementAssignmentExpansion)
		}
		expansions[path][candidate.assignment.lineIndex] = signingXCConfigEntitlementAssignmentExpansion{
			value:             strings.TrimSpace(value),
			inheritedResolved: inheritedResolved,
		}
	}
	for _, value := range accumulated {
		if strings.TrimSpace(value) != "" {
			composed = append(composed, value)
		}
	}
	return candidates, composed, expansions, nil
}
