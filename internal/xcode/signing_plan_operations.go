package xcode

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/bitrise-io/go-xcode/xcodeproject/serialized"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

func signingConfigurationFor(project *structuredVersionProject, target, configuration string) (*versionConfiguration, error) {
	targetMatches := 0
	for _, candidate := range project.project.Proj.Targets {
		if candidate.Name == target {
			targetMatches++
		}
	}
	if targetMatches > 1 {
		return nil, fmt.Errorf("project contains multiple targets named %q", target)
	}
	var match *versionConfiguration
	for _, candidate := range project.configurations {
		if !candidate.projectLevel && candidate.target == target && candidate.name == configuration {
			if match != nil {
				return nil, fmt.Errorf("target %q contains multiple configurations named %q", target, configuration)
			}
			match = candidate
		}
	}
	if match != nil {
		return match, nil
	}
	return nil, fmt.Errorf("configuration %q not found for target %q", configuration, target)
}

func inspectSigningCandidate(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	setting signingDesiredSetting,
	configFiles map[string][]string,
	fileConsumers map[string]map[string]bool,
	fileIdentities map[string]string,
	requestedSettings map[string]map[string]bool,
	uncertainConsumers bool,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) (signingCandidate, string, string) {
	candidate := signingCandidate{configuration: configuration, setting: setting.key, desired: cloneSigningString(setting.value)}
	if !signingConfigurationSourcesAuthorized(project, configuration, configFiles) {
		return candidate, signingSettingBlocker(configuration, setting.key, errors.New("configuration inherits from an xcconfig that was not authorized or could not be read")), ""
	}
	resolver := newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths)
	if setting.key == "CODE_SIGN_ENTITLEMENTS" && setting.value != nil {
		if err := validateSigningEntitlementsPath(project, *setting.value); err != nil {
			return candidate, signingSettingBlocker(configuration, setting.key, fmt.Errorf("validate path %q: %w", *setting.value, err)), ""
		}
	}
	keys := matchingBuildSettingKeys(configuration.buildSettings, setting.key)
	if len(keys) > 0 {
		old, _, err := resolver.resolveSetting(configuration, setting.key)
		if err != nil {
			return candidate, signingSettingBlocker(configuration, setting.key, err), ""
		}
		candidate.old = stringPtr(old)
		candidate.resolution = "direct"
		if signingValuesEqual(setting.value, candidate.old) {
			// A direct assignment that still defers to $(inherited) keeps a live
			// dependency on the xcconfig supplying that value. Retain it as a
			// no-op consumer so shared-file resolution can see the disagreement
			// when a sibling configuration wants a different value in the same
			// file; otherwise rewriting that file would silently change this
			// configuration's effective value.
			if setting.value != nil && signingDirectValueInherits(configuration, setting.key) {
				assignmentFiles, assignmentErr := xcconfigFilesDefiningWithReader(configFiles[configuration.id], setting.key, resolver.readXCConfig)
				if assignmentErr != nil {
					return candidate, signingSettingBlocker(configuration, setting.key, assignmentErr), ""
				}
				if len(assignmentFiles) > 0 {
					candidate.mode = "xcconfig"
					candidate.paths = append(candidate.paths, assignmentFiles...)
				}
			}
			candidate.noOp = true
			return candidate, "", ""
		}
		candidate.mode = "pbxproj"
		return candidate, "", ""
	}

	assignmentFiles, err := xcconfigFilesDefiningWithReader(configFiles[configuration.id], setting.key, resolver.readXCConfig)
	if err != nil {
		return candidate, signingSettingBlocker(configuration, setting.key, err), ""
	}
	old, _, resolveErr := resolver.resolveSetting(configuration, setting.key)
	if resolveErr == nil {
		candidate.old = stringPtr(old)
		if len(assignmentFiles) > 0 {
			candidate.resolution = "xcconfig"
		} else {
			candidate.resolution = "inherited"
		}
	} else if !errors.Is(resolveErr, errVersionSettingNotFound) {
		return candidate, signingSettingBlocker(configuration, setting.key, resolveErr), ""
	} else {
		candidate.resolution = "missing"
	}
	if signingValuesEqual(setting.value, candidate.old) {
		if len(assignmentFiles) > 0 && setting.value != nil {
			candidate.mode = "xcconfig"
			candidate.paths = append(candidate.paths, assignmentFiles...)
		}
		candidate.noOp = true
		return candidate, "", ""
	}

	if setting.value == nil {
		return candidate, fmt.Sprintf("target %q configuration %q cannot remove inherited %s; only a direct project assignment can be removed", configuration.target, configuration.name, setting.key), ""
	}
	if len(assignmentFiles) > 0 {
		if uncertainConsumers {
			candidate.mode = "pbxproj"
			return candidate, "", "xcconfig consumer scope is uncertain; using a target-level override for " + setting.key
		}
		if !consumersAuthorizeSetting(assignmentFiles, fileConsumers, fileIdentities, requestedSettings, setting.key) {
			candidate.mode = "pbxproj"
			return candidate, "", "shared xcconfig is consumed by a configuration that did not request " + setting.key + "; using a target-level override"
		}
		warning := ""
		for _, path := range assignmentFiles {
			if err := project.checkXCConfigWritable(path, allowExternal); err != nil {
				return candidate, fmt.Sprintf("target %q configuration %q cannot update %s in xcconfig %s: %v", configuration.target, configuration.name, setting.key, path, err), ""
			}
			if err := validateSigningXCConfigPath(project, path, allowExternal); err != nil {
				return candidate, fmt.Sprintf("target %q configuration %q cannot update xcconfig %s: %v", configuration.target, configuration.name, path, err), ""
			}
			if allowExternal && !signingPathContained(project, path) {
				if warning == "" {
					warning = fmt.Sprintf("external xcconfig %s is authorized for %s", path, setting.key)
				}
			}
		}
		candidate.mode = "xcconfig"
		candidate.paths = append(candidate.paths, assignmentFiles...)
		return candidate, "", warning
	}

	// Project-level inheritance is deliberately shadowed at the selected
	// target/configuration. This avoids widening a change to other targets.
	candidate.mode = "pbxproj"
	return candidate, "", ""
}

// signingDirectValueInherits reports whether a direct project assignment for
// setting still defers to a lower-level value through $(inherited). Such a
// configuration keeps depending on the xcconfig that supplies the inherited
// value even when its resolved value already matches the requested one.
func signingDirectValueInherits(configuration *versionConfiguration, setting string) bool {
	for _, key := range matchingBuildSettingKeys(configuration.buildSettings, setting) {
		switch value := configuration.buildSettings[key].(type) {
		case string:
			if signingValueInherits(value) {
				return true
			}
		case []any:
			for _, element := range value {
				if text, ok := element.(string); ok && signingValueInherits(text) {
					return true
				}
			}
		}
	}
	return false
}

func signingValueInherits(value string) bool {
	return strings.Contains(value, "$(inherited)") || strings.Contains(value, "${inherited}")
}

func signingConfigurationSourcesAuthorized(
	project *structuredVersionProject,
	configuration *versionConfiguration,
	configFiles map[string][]string,
) bool {
	for current := configuration; current != nil; {
		if current.baseReferenceID != "" {
			if _, ok := configFiles[current.id]; !ok {
				return false
			}
		}
		if current.projectLevel {
			break
		}
		current = project.projectConfiguration(current.name)
	}
	return true
}

// validateSigningEntitlementsPath binds a non-null CODE_SIGN_ENTITLEMENTS
// value to the selected project root. It requires an existing regular file
// and rejects symlinked parent or final components before a plan is ready.
func validateSigningEntitlementsPath(project *structuredVersionProject, path string) error {
	root, err := rootfs.New(project.rootDir)
	if err != nil {
		return err
	}
	defer root.Close()
	file, err := root.OpenFile(path)
	if err != nil {
		return err
	}
	return file.Close()
}

func resolveSigningSharedCandidates(candidates []signingCandidate, fileIdentities map[string]string) {
	desiredByFile := make(map[string]string)
	conflictByFile := make(map[string]bool)
	changedByFile := make(map[string]bool)
	noOpByFile := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate.mode != "xcconfig" || candidate.desired == nil {
			continue
		}
		for _, path := range candidate.paths {
			// Shared-file conflict detection is identity-based: two consumers of
			// one inode must not receive different values through an xcconfig
			// mutation. Publication and receipt grouping below deliberately use the
			// path-preserving operation key so hard-linked path intents remain
			// separate writes.
			key := signingXCConfigPhysicalKey(path, fileIdentities) + "\x00" + candidate.setting
			if candidate.noOp {
				noOpByFile[key] = true
				continue
			}
			value := *candidate.desired
			if !signingValuesEqual(candidate.old, candidate.desired) {
				changedByFile[key] = true
			}
			if previous, exists := desiredByFile[key]; exists && previous != value {
				conflictByFile[key] = true
				continue
			}
			desiredByFile[key] = value
		}
	}
	for index := range candidates {
		if candidates[index].mode != "xcconfig" {
			continue
		}
		for _, path := range candidates[index].paths {
			key := signingXCConfigPhysicalKey(path, fileIdentities) + "\x00" + candidates[index].setting
			if !candidates[index].noOp && noOpByFile[key] && changedByFile[key] {
				// A no-op direct/inherited consumer is still semantically tied to
				// this shared setting. Keep the shared source unchanged and
				// materialize the requested value at each changing target instead;
				// this preserves the no-op consumer without inventing a lower-layer
				// value or relying on algebraic inherited-value assumptions.
				candidates[index].mode = "pbxproj"
				candidates[index].paths = nil
				candidates[index].noOp = false
				break
			}
			if conflictByFile[key] {
				candidates[index].mode = "pbxproj"
				candidates[index].paths = nil
				candidates[index].noOp = false
				break
			}
		}
	}
}

// buildSigningPlanOperations materializes the current candidate decisions
// into concrete operations. It deliberately runs shared-file arbitration on
// every pass: a candidate promoted to a target-level override must no longer
// participate as an xcconfig write in a later pass.
func buildSigningPlanOperations(
	project *structuredVersionProject,
	candidates []signingCandidate,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) ([]signingPlanOperation, []string) {
	resolveSigningSharedCandidates(candidates, fileIdentities)
	resolver := newSigningSettingResolver(project, configFiles, allowExternal, lexicalConfigPaths)
	operations := make([]signingPlanOperation, 0, len(candidates))
	var blockers []string
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.mode == "" || candidate.noOp {
			continue
		}
		if candidate.mode == "xcconfig" {
			var validationErr error
			for _, path := range candidate.paths {
				if err := validateSigningXCConfigWrite(resolver, path, candidate.setting, candidate.desired); err != nil {
					validationErr = err
					break
				}
			}
			if validationErr != nil {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, validationErr))
				continue
			}
			for _, path := range candidate.paths {
				operations = append(operations, signingPlanOperation{
					SigningSettingChange: signingChange(candidate, path, "xcconfig"),
					configuration:        candidate.configuration,
				})
			}
			continue
		}
		operations = append(operations, signingPlanOperation{
			SigningSettingChange: signingChange(candidate, project.projectFilePath, project.projectSource),
			configuration:        candidate.configuration,
		})
	}
	sortSigningPlanOperations(operations)
	sort.Strings(blockers)
	return operations, blockers
}

// stageSigningPlan applies all concrete operations to a private project copy
// and xcconfig byte overlay. The source project and files are never mutated;
// the resulting resolver therefore observes the same state that apply will
// publish if the plan converges.
func stageSigningPlan(
	project *structuredVersionProject,
	operations []signingPlanOperation,
	configFiles map[string][]string,
	fileIdentities map[string]string,
	allowExternal bool,
	lexicalConfigPaths map[string][]string,
) (*structuredVersionProject, *signingSettingResolver, error) {
	stagedProject := cloneSigningStructuredVersionProject(project)
	configurations := make(map[string]*versionConfiguration, len(stagedProject.configurations))
	for _, configuration := range stagedProject.configurations {
		if configuration != nil {
			configurations[configuration.id] = configuration
		}
	}
	for _, operation := range operations {
		if operation.Source != project.projectSource {
			continue
		}
		if operation.configuration == nil {
			return nil, nil, fmt.Errorf("missing configuration for %s", operation.Setting)
		}
		configuration := configurations[operation.configuration.id]
		if configuration == nil {
			return nil, nil, fmt.Errorf("configuration %s is missing from staged project", operation.configuration.id)
		}
		stagedOperation := operation
		stagedOperation.configuration = configuration
		if err := applySigningPBXOperation(stagedOperation); err != nil {
			return nil, nil, err
		}
	}

	if stagedProject.xcproj != nil {
		if _, err := stagedProject.editXCProjBuildSettings(stagedProject.xcproj.data); err != nil {
			return nil, nil, fmt.Errorf("edit %s: %w", xcprojFileName, err)
		}
	}

	resolver := newSigningSettingResolver(stagedProject, configFiles, allowExternal, lexicalConfigPaths)
	overlayByIdentity := make(map[string][]byte)
	for _, operation := range operations {
		if operation.Source != "xcconfig" {
			continue
		}
		if operation.NewValue == nil {
			return nil, nil, fmt.Errorf("xcconfig removal is not supported for %s", operation.Setting)
		}
		identityKey := signingXCConfigOperationKey(operation.Path, fileIdentities)
		data, ok := overlayByIdentity[identityKey]
		if !ok {
			var err error
			data, err = resolver.readXCConfig(operation.Path)
			if err != nil {
				return nil, nil, fmt.Errorf("read xcconfig %s while staging: %w", operation.Path, err)
			}
		}
		updated, _, _, err := editXCConfig(data, operation.Setting, *operation.NewValue)
		if err != nil {
			return nil, nil, fmt.Errorf("edit xcconfig %s while staging: %w", operation.Path, err)
		}
		if _, err := parseXCConfig(updated); err != nil {
			return nil, nil, fmt.Errorf("validate xcconfig %s while staging: %w", operation.Path, err)
		}
		overlayByIdentity[identityKey] = append([]byte(nil), updated...)
	}

	// A case-insensitive filesystem may expose one collected file under more
	// than one lexical spelling. Mirror the staged bytes to every collected
	// spelling that has the same identity so resolver reads cannot escape the
	// overlay on the next pass.
	overlay := make(map[string][]byte, len(overlayByIdentity))
	for _, paths := range configFiles {
		for _, path := range paths {
			identityKey := signingXCConfigOperationKey(path, fileIdentities)
			data, ok := overlayByIdentity[identityKey]
			if !ok {
				continue
			}
			overlay[normalizeSigningLexicalPath(path)] = append([]byte(nil), data...)
		}
	}
	resolver.stagedXCConfig = overlay
	return stagedProject, resolver, nil
}

// reclassifySigningNoOps validates every requested setting against the fully
// staged project. A no-op whose effective value changed is promoted to a
// target-level literal operation; preserving the original candidate metadata
// keeps the public plan's old-value and resolution fields stable.
func reclassifySigningNoOps(
	candidates []signingCandidate,
	originalProject *structuredVersionProject,
	stagedProject *structuredVersionProject,
	resolver *signingSettingResolver,
	baselineResolver *signingSettingResolver,
) (int, []string) {
	configurations := make(map[string]*versionConfiguration, len(stagedProject.configurations))
	for _, configuration := range stagedProject.configurations {
		if configuration != nil {
			configurations[configuration.id] = configuration
		}
	}
	reclassified := 0
	var blockers []string
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.configuration == nil {
			blockers = append(blockers, fmt.Sprintf("cannot stage signing setting %s without a configuration", candidate.setting))
			continue
		}
		configuration := configurations[candidate.configuration.id]
		if configuration == nil {
			blockers = append(blockers, fmt.Sprintf("target %q configuration %q is missing from staged signing project", candidate.configuration.target, candidate.configuration.name))
			continue
		}
		if candidate.desired == nil {
			if len(matchingBuildSettingKeys(configuration.buildSettings, candidate.setting)) > 0 {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, errors.New("staged project still has a direct assignment")))
				continue
			}
			resolved, _, err := resolver.resolveSetting(configuration, candidate.setting)
			if err != nil && !errors.Is(err, errVersionSettingNotFound) {
				blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, err))
			} else if err == nil && baselineResolver != nil {
				baselineProject := cloneSigningStructuredVersionProject(originalProject)
				var baselineConfiguration *versionConfiguration
				for _, candidateConfiguration := range baselineProject.configurations {
					if candidateConfiguration != nil && candidateConfiguration.id == candidate.configuration.id {
						baselineConfiguration = candidateConfiguration
						break
					}
				}
				if baselineConfiguration == nil {
					continue
				}
				for _, key := range matchingBuildSettingKeys(baselineConfiguration.buildSettings, candidate.setting) {
					delete(baselineConfiguration.buildSettings, key)
				}
				baselineResolver = newSigningSettingResolver(baselineProject, baselineResolver.configFiles, baselineResolver.allowExternal, baselineResolver.lexicalConfigPaths)
				baseline, _, baselineErr := baselineResolver.resolveSetting(baselineConfiguration, candidate.setting)
				if signingRemovalFallbackChanged(resolved, err, baseline, baselineErr) && baselineErr == nil {
					blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q differs from value after removal alone %q; another operation in this plan would change the fallback", resolved, baseline)))
				} else if signingRemovalFallbackChanged(resolved, err, baseline, baselineErr) {
					blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q appears after removal alone left the setting unresolved; another operation in this plan would create the fallback", resolved)))
				}
			}
			continue
		}

		resolved, _, err := resolver.resolveSetting(configuration, candidate.setting)
		if err != nil {
			if candidate.noOp {
				// A previously resolvable no-op can become unresolved when a
				// staged dependency is removed (for example, a direct setting
				// that expands a setting removed by the same plan). Materialize
				// the requested value at the target level so the no-op's
				// effective value remains stable. Do this for every staged
				// resolution failure: the initial pass already proved that the
				// candidate resolved, so a new error necessarily comes from the
				// staged dependency state.
				candidate.mode = "pbxproj"
				candidate.paths = nil
				candidate.noOp = false
				reclassified++
				continue
			}
			blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged resolution failed: %w", err)))
			continue
		}
		if resolved == *candidate.desired {
			continue
		}
		if candidate.noOp {
			candidate.mode = "pbxproj"
			candidate.paths = nil
			candidate.noOp = false
			reclassified++
			continue
		}
		blockers = append(blockers, signingSettingBlocker(candidate.configuration, candidate.setting, fmt.Errorf("staged value %q does not match desired value %q", resolved, *candidate.desired)))
	}
	sort.Strings(blockers)
	return reclassified, blockers
}

func signingRemovalFallbackChanged(staged string, stagedErr error, baseline string, baselineErr error) bool {
	if stagedErr != nil {
		return false
	}
	if baselineErr != nil {
		return true
	}
	return staged != baseline
}

func cloneSigningStructuredVersionProject(project *structuredVersionProject) *structuredVersionProject {
	if project == nil {
		return nil
	}
	clone := *project
	clone.objects = cloneSigningSerializedObject(project.objects)
	clone.project.RawProj = cloneSigningSerializedObject(project.project.RawProj)
	clone.parentByChild = make(map[string]string, len(project.parentByChild))
	for child, parent := range project.parentByChild {
		clone.parentByChild[child] = parent
	}
	clone.configurations = make([]*versionConfiguration, 0, len(project.configurations))
	for _, configuration := range project.configurations {
		if configuration == nil {
			clone.configurations = append(clone.configurations, nil)
			continue
		}
		configurationClone := *configuration
		configurationClone.buildSettings = cloneSigningSerializedObject(configuration.buildSettings)
		clone.configurations = append(clone.configurations, &configurationClone)
	}
	return &clone
}

func cloneSigningSerializedObject(source serialized.Object) serialized.Object {
	if source == nil {
		return nil
	}
	clone := make(serialized.Object, len(source))
	for key, value := range source {
		clone[key] = cloneSigningValue(value)
	}
	return clone
}

func cloneSigningValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case serialized.Object:
		return cloneSigningSerializedObject(typed)
	case map[string]interface{}:
		clone := make(map[string]interface{}, len(typed))
		for key, nested := range typed {
			clone[key] = cloneSigningValue(nested)
		}
		return clone
	case []interface{}:
		clone := make([]interface{}, len(typed))
		for index, nested := range typed {
			clone[index] = cloneSigningValue(nested)
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

func signingSettingBlocker(configuration *versionConfiguration, setting string, err error) string {
	return fmt.Sprintf("target %q configuration %q cannot resolve %s: %v", configuration.target, configuration.name, setting, err)
}

func validateSigningXCConfigWrite(resolver *signingSettingResolver, path, setting string, desired *string) error {
	if desired == nil {
		return nil
	}
	data, err := resolver.readXCConfig(path)
	if err != nil {
		return err
	}
	document, err := parseXCConfig(data)
	if err != nil {
		return err
	}
	if !xcconfigValueHasLineContinuation(*desired) {
		return nil
	}
	for _, assignment := range document.assignments {
		if assignment.baseKey != setting {
			continue
		}
		if assignment.quote == "" {
			return fmt.Errorf("desired value has a trailing backslash that would continue the xcconfig assignment")
		}
	}
	return nil
}

func signingValuesEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func signingChange(candidate *signingCandidate, path, source string) SigningSettingChange {
	operation := "set"
	if candidate.desired == nil {
		operation = "remove"
	}
	return SigningSettingChange{
		Target:        candidate.configuration.target,
		Configuration: candidate.configuration.name,
		Setting:       candidate.setting,
		Operation:     operation,
		OldValue:      cloneSigningString(candidate.old),
		NewValue:      cloneSigningString(candidate.desired),
		Path:          path,
		Source:        source,
		Resolution:    candidate.resolution,
	}
}

func sortSigningPlanOperations(operations []signingPlanOperation) {
	sort.Slice(operations, func(left, right int) bool {
		first, second := operations[left], operations[right]
		if first.Path != second.Path {
			return first.Path < second.Path
		}
		if first.Target != second.Target {
			return first.Target < second.Target
		}
		if first.Configuration != second.Configuration {
			return first.Configuration < second.Configuration
		}
		return first.Setting < second.Setting
	})
}
