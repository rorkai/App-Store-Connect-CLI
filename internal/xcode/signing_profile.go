package xcode

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fullsailor/pkcs7"
	"howett.net/plist"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
)

const signingProfileMaxBytes int64 = 16 << 20

// SigningPlanInference records which profile was chosen for one target.
type SigningPlanInference struct {
	Target            string `json:"target"`
	Configuration     string `json:"configuration"`
	BundleID          string `json:"bundleId"`
	ProfilePath       string `json:"profilePath"`
	ProfileUUID       string `json:"profileUuid"`
	CertificateSHA256 string `json:"certificateSha256"`
	Match             string `json:"match"`
}

// SigningPlanExportOptions is the export-options payload embedded in a plan.
type SigningPlanExportOptions struct {
	Method               string            `json:"method,omitempty"`
	SigningStyle         string            `json:"signingStyle,omitempty"`
	TeamID               string            `json:"teamID,omitempty"`
	ProvisioningProfiles map[string]string `json:"provisioningProfiles,omitempty"`
}

type signingProfile struct {
	path       string
	name       string
	uuid       string
	teamID     string
	pattern    string
	wildcard   bool
	expires    time.Time
	identity   string
	certSHA256 string
	// noValidCert is set when no embedded certificate is valid right now
	// (all expired or not yet valid); such a profile cannot sign.
	noValidCert bool
	method      string
	platforms   []string
}

type signingProfileAssignment struct {
	target        string
	configuration string
	bundleID      string
	profile       *signingProfile
	match         string
}

type signingProfileInference struct {
	manifest      *signingSettingsManifest
	paths         []string
	inferences    []SigningPlanInference
	exportOptions *SigningPlanExportOptions
	exportMethod  string
	skipTargets   []string
	blockers      []string
	warnings      []string
}

func inferSigningSettings(project *structuredVersionProject, opts SigningPlanOptions, overrides *signingSettingsManifest) (*signingProfileInference, error) {
	profiles, paths, err := readSigningProfiles(opts.ProfilePaths)
	if err != nil {
		return nil, err
	}
	projectTargets := signingTargetProductTypes(project)
	skipped := make(map[string]bool, len(opts.SkipTargets))
	skipTargets := make([]string, 0, len(opts.SkipTargets))
	for _, target := range opts.SkipTargets {
		name := strings.TrimSpace(target)
		if name == "" || skipped[name] {
			continue
		}
		if _, ok := projectTargets[name]; !ok {
			// A misspelled target must not be accepted and silently ignored.
			return nil, newSigningInputError(fmt.Errorf("--skip-target %q is not a target in the project", name))
		}
		skipped[name] = true
		skipTargets = append(skipTargets, name)
	}
	sort.Strings(skipTargets)

	// An expired profile can no longer sign or export, so it is never an
	// inference candidate; it is reported only when nothing else matches.
	active, expired := partitionExpiredSigningProfiles(profiles, signingProfileNow())

	// An explicit --export-method limits candidates to profiles that can
	// export that way, so a development profile never backs an App Store
	// export (or the reverse).
	requestedMethod := strings.TrimSpace(opts.ExportMethod)

	configurationFilter := strings.TrimSpace(opts.Configuration)
	scopes := signingInferenceScopes(project, configurationFilter)
	if configurationFilter != "" && len(scopes) == 0 {
		return &signingProfileInference{
			paths:        paths,
			skipTargets:  skipTargets,
			exportMethod: strings.TrimSpace(opts.ExportMethod),
			blockers:     []string{fmt.Sprintf("configuration %q was not found", configurationFilter)},
		}, nil
	}

	productTypes := signingTargetProductTypes(project)
	assigned := make([]signingProfileAssignment, 0)
	// settingsOnly holds profile-embedding targets that no supplied profile
	// matched but the settings file covers; export options must still name
	// their profile and team.
	settingsOnly := make([]signingProfileAssignment, 0)
	blockers := make([]string, 0)
	warnings := make([]string, 0)
	covered := signingManifestCoverage(overrides)
	seenConfiguration := configurationFilter == ""
	for _, scope := range scopes {
		if scope.name == configurationFilter {
			seenConfiguration = true
		}
		if skipped[scope.target] || !signingProductEmbedsProfile(productTypes[scope.target]) {
			continue
		}
		bundleID, bundleErr := signingBundleID(project, scope)
		if bundleErr != nil || bundleID == "" {
			if covered[scope.target+"\x00"+scope.name] {
				settingsOnly = append(settingsOnly, signingProfileAssignment{target: scope.target, configuration: scope.name})
				continue
			}
			detail := "PRODUCT_BUNDLE_IDENTIFIER is not set"
			if bundleErr != nil {
				detail = bundleErr.Error()
			}
			blockers = append(blockers, fmt.Sprintf("unmatched signing target %s/%s: %s", scope.target, scope.name, detail))
			continue
		}
		sdk := signingTargetSDK(project, scope)
		platformProfiles := signingProfilesForSDK(active, sdk)
		selected, discarded, match := selectSigningProfile(signingProfilesForMethod(platformProfiles, requestedMethod), bundleID)
		if selected == nil {
			if covered[scope.target+"\x00"+scope.name] {
				settingsOnly = append(settingsOnly, signingProfileAssignment{target: scope.target, configuration: scope.name, bundleID: bundleID})
				continue
			}
			detail := ""
			if other, _, _ := selectSigningProfile(platformProfiles, bundleID); other != nil {
				detail = fmt.Sprintf("; profile %s is for %s, not --export-method %s", other.name, other.method, requestedMethod)
			} else if stale, _, _ := selectSigningProfile(signingProfilesForSDK(expired, sdk), bundleID); stale != nil {
				detail = fmt.Sprintf("; profile %s expired at %s", stale.name, stale.expires.UTC().Format(time.RFC3339))
				if stale.expires.After(signingProfileNow()) {
					detail = fmt.Sprintf("; profile %s has no currently valid signing certificate", stale.name)
				}
			}
			blockers = append(blockers, fmt.Sprintf("unmatched signing target %s/%s bundle ID %s%s", scope.target, scope.name, bundleID, detail))
			continue
		}
		if len(discarded) > 0 {
			names := make([]string, 0, len(discarded))
			for _, profile := range discarded {
				names = append(names, profile.name)
			}
			sort.Strings(names)
			warnings = append(warnings, fmt.Sprintf("selected profile %s for %s/%s; discarded %s", selected.name, scope.target, scope.name, strings.Join(names, ", ")))
		}
		assigned = append(assigned, signingProfileAssignment{
			target:        scope.target,
			configuration: scope.name,
			bundleID:      bundleID,
			profile:       selected,
			match:         match,
		})
	}
	if configurationFilter != "" && !seenConfiguration {
		blockers = append(blockers, fmt.Sprintf("configuration %q was not found", configurationFilter))
	}

	manifest := inferredSigningManifest(assigned)
	if overrides != nil {
		if manifest == nil || len(manifest.Targets) == 0 {
			manifest = cloneSigningManifest(overrides)
		} else {
			warnings = append(warnings, overlaySigningManifest(manifest, overrides)...)
		}
	}
	inferences := make([]SigningPlanInference, 0, len(assigned))
	for _, item := range assigned {
		inferences = append(inferences, SigningPlanInference{
			Target:            item.target,
			Configuration:     item.configuration,
			BundleID:          item.bundleID,
			ProfilePath:       item.profile.path,
			ProfileUUID:       item.profile.uuid,
			CertificateSHA256: item.profile.certSHA256,
			Match:             item.match,
		})
	}
	sort.Slice(inferences, func(left, right int) bool {
		if inferences[left].Target != inferences[right].Target {
			return inferences[left].Target < inferences[right].Target
		}
		return inferences[left].Configuration < inferences[right].Configuration
	})

	method := strings.TrimSpace(opts.ExportMethod)
	if method == "" {
		var methodBlocker string
		method, methodBlocker = inferSigningExportMethod(assigned)
		if methodBlocker != "" {
			blockers = append(blockers, methodBlocker)
		}
	}
	exportOptions, teams := signingExportOptions(method, assigned, settingsOnly, manifest)
	if len(teams) > 1 {
		blockers = append(blockers, "selected profiles use more than one development team")
	}
	if manifest != nil && len(manifest.Targets) == 0 {
		manifest = nil
	}
	return &signingProfileInference{
		manifest:      manifest,
		paths:         paths,
		inferences:    inferences,
		exportOptions: exportOptions,
		exportMethod:  method,
		skipTargets:   skipTargets,
		blockers:      blockers,
		warnings:      warnings,
	}, nil
}

// signingProfileNow is the clock used to exclude expired profiles.
var signingProfileNow = time.Now

func partitionExpiredSigningProfiles(profiles []signingProfile, now time.Time) ([]signingProfile, []signingProfile) {
	active := make([]signingProfile, 0, len(profiles))
	expired := make([]signingProfile, 0)
	for _, profile := range profiles {
		if profile.noValidCert || !profile.expires.IsZero() && !profile.expires.After(now) {
			expired = append(expired, profile)
			continue
		}
		active = append(active, profile)
	}
	return active, expired
}

func signingProfilesForMethod(profiles []signingProfile, method string) []signingProfile {
	if method == "" {
		return profiles
	}
	filtered := make([]signingProfile, 0, len(profiles))
	for _, profile := range profiles {
		if profile.method == method {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

func signingInferenceScopes(project *structuredVersionProject, configuration string) []*versionConfiguration {
	scopes := make([]*versionConfiguration, 0)
	for _, item := range project.configurations {
		if item == nil || item.projectLevel || item.target == "" {
			continue
		}
		if configuration != "" && item.name != configuration {
			continue
		}
		scopes = append(scopes, item)
	}
	sort.Slice(scopes, func(left, right int) bool {
		if scopes[left].target != scopes[right].target {
			return scopes[left].target < scopes[right].target
		}
		return scopes[left].name < scopes[right].name
	})
	return scopes
}

func signingTargetProductTypes(project *structuredVersionProject) map[string]string {
	productTypes := make(map[string]string, len(project.project.Proj.Targets))
	for _, target := range project.project.Proj.Targets {
		productTypes[target.Name] = target.ProductType
	}
	return productTypes
}

// signingProductEmbedsProfile reports whether a product type is signed with
// an embedded provisioning profile. Frameworks, libraries, test bundles, and
// tools are signed without one, and Xcode rejects a manual
// PROVISIONING_PROFILE_SPECIFIER on them, so inference leaves them alone.
func signingProductEmbedsProfile(productType string) bool {
	switch {
	case strings.HasPrefix(productType, "com.apple.product-type.application"),
		strings.HasPrefix(productType, "com.apple.product-type.app-extension"):
		return true
	}
	switch productType {
	case "com.apple.product-type.extensionkit-extension",
		"com.apple.product-type.watchkit-extension",
		"com.apple.product-type.watchkit2-extension",
		"com.apple.product-type.tv-app-extension",
		"com.apple.product-type.system-extension",
		"com.apple.product-type.driver-extension":
		return true
	default:
		return false
	}
}

func signingTargetSDK(project *structuredVersionProject, configuration *versionConfiguration) string {
	value, _, err := project.resolveSetting(configuration, "SDKROOT")
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// signingProfilesForSDK keeps the profiles usable for a target's SDK. A
// universal-purchase app can share one bundle ID across iOS, tvOS, and macOS
// profiles, so each target only considers profiles whose Platform list
// covers its SDK family. watchOS apps are signed with iOS-platform profiles
// and visionOS apps with iOS profiles that list xrOS/visionOS, so those
// families accept either. An unknown SDK (for example SDKROOT=auto on a
// multiplatform target) or a profile without a Platform entry is not
// filtered.
func signingProfilesForSDK(profiles []signingProfile, sdk string) []signingProfile {
	accepted := signingSDKProfilePlatforms(sdk)
	if accepted == nil {
		return profiles
	}
	filtered := make([]signingProfile, 0, len(profiles))
	for _, profile := range profiles {
		keep := len(profile.platforms) == 0
		for _, platform := range profile.platforms {
			if accepted[strings.ToLower(strings.TrimSpace(platform))] {
				keep = true
				break
			}
		}
		if keep {
			filtered = append(filtered, profile)
		}
	}
	return filtered
}

// signingSDKProfilePlatforms maps an SDKROOT to the lowercase profile
// Platform values that can sign it, or nil when the SDK is not recognized.
func signingSDKProfilePlatforms(sdk string) map[string]bool {
	switch {
	case strings.HasPrefix(sdk, "macosx"):
		return map[string]bool{"osx": true}
	case strings.HasPrefix(sdk, "iphoneos"):
		return map[string]bool{"ios": true}
	case strings.HasPrefix(sdk, "appletvos"):
		return map[string]bool{"tvos": true}
	case strings.HasPrefix(sdk, "watchos"):
		return map[string]bool{"ios": true, "watchos": true}
	case strings.HasPrefix(sdk, "xros"):
		return map[string]bool{"xros": true, "visionos": true, "ios": true}
	default:
		return nil
	}
}

func signingProfileMacOnly(profile signingProfile) bool {
	for _, platform := range profile.platforms {
		if !strings.EqualFold(platform, "OSX") {
			return false
		}
	}
	return len(profile.platforms) > 0
}

func signingBundleID(project *structuredVersionProject, configuration *versionConfiguration) (string, error) {
	value, _, err := project.resolveSetting(configuration, "PRODUCT_BUNDLE_IDENTIFIER")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func signingManifestCoverage(manifest *signingSettingsManifest) map[string]bool {
	covered := make(map[string]bool)
	if manifest == nil {
		return covered
	}
	for _, target := range manifest.Targets {
		for _, configuration := range target.Configurations {
			if signingOverrideCoversProfile(configuration.Settings) {
				covered[strings.TrimSpace(target.Name)+"\x00"+strings.TrimSpace(configuration.Name)] = true
			}
		}
	}
	return covered
}

// signingOverrideCoversProfile reports whether a settings-file entry decides
// how an unmatched target is signed: it names a provisioning profile or
// switches the target to automatic signing. An entry that only touches an
// unrelated setting (for example CODE_SIGN_IDENTITY) leaves the target
// without a profile, so its unmatched blocker must stay.
func signingOverrideCoversProfile(settings map[string]json.RawMessage) bool {
	for _, key := range []string{"PROVISIONING_PROFILE_SPECIFIER", "PROVISIONING_PROFILE"} {
		var value string
		if raw, ok := settings[key]; ok && json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			return true
		}
	}
	return signingSettingsAutomatic(settings)
}

func signingSettingsAutomatic(settings map[string]json.RawMessage) bool {
	var style string
	if raw, ok := settings["CODE_SIGN_STYLE"]; ok && json.Unmarshal(raw, &style) == nil {
		return strings.EqualFold(strings.TrimSpace(style), "Automatic")
	}
	return false
}

func selectSigningProfile(profiles []signingProfile, bundleID string) (*signingProfile, []signingProfile, string) {
	exact := make([]signingProfile, 0)
	wild := make([]signingProfile, 0)
	for _, profile := range profiles {
		if !profile.wildcard && profile.pattern == bundleID {
			exact = append(exact, profile)
			continue
		}
		if profile.wildcard && signingWildcardMatch(profile.pattern, bundleID) {
			wild = append(wild, profile)
		}
	}
	chosen := exact
	match := "exact"
	if len(chosen) == 0 {
		chosen = wild
		match = "wildcard"
	}
	if len(chosen) == 0 {
		return nil, nil, ""
	}
	sort.SliceStable(chosen, func(left, right int) bool {
		// A narrower wildcard (com.example.*) is more specific than the team
		// wildcard (*), so it wins before expiration is considered.
		if len(chosen[left].pattern) != len(chosen[right].pattern) {
			return len(chosen[left].pattern) > len(chosen[right].pattern)
		}
		if !chosen[left].expires.Equal(chosen[right].expires) {
			return chosen[left].expires.After(chosen[right].expires)
		}
		if chosen[left].uuid != chosen[right].uuid {
			return chosen[left].uuid < chosen[right].uuid
		}
		return chosen[left].path < chosen[right].path
	})
	selected := chosen[0]
	return &selected, chosen[1:], match
}

func signingWildcardMatch(pattern, bundleID string) bool {
	if pattern == "*" {
		return bundleID != ""
	}
	prefix := strings.TrimSuffix(pattern, "*")
	if prefix == pattern || !strings.HasSuffix(prefix, ".") {
		return false
	}
	return strings.HasPrefix(bundleID, prefix) && len(bundleID) > len(prefix)
}

func inferredSigningManifest(assigned []signingProfileAssignment) *signingSettingsManifest {
	grouped := make(map[string][]signingManifestConfiguration)
	order := make([]string, 0)
	for _, item := range assigned {
		if _, ok := grouped[item.target]; !ok {
			order = append(order, item.target)
		}
		grouped[item.target] = append(grouped[item.target], signingManifestConfiguration{
			Name: item.configuration,
			Settings: map[string]json.RawMessage{
				"CODE_SIGN_STYLE":                mustRawJSON("Manual"),
				"DEVELOPMENT_TEAM":               mustRawJSON(item.profile.teamID),
				"CODE_SIGN_IDENTITY":             mustRawJSON(item.profile.identity),
				"PROVISIONING_PROFILE_SPECIFIER": mustRawJSON(item.profile.name),
			},
		})
	}
	sort.Strings(order)
	manifest := &signingSettingsManifest{SchemaVersion: signingPlanSchemaVersion, Targets: make([]signingManifestTarget, 0, len(order))}
	for _, name := range order {
		configs := grouped[name]
		sort.Slice(configs, func(left, right int) bool { return configs[left].Name < configs[right].Name })
		manifest.Targets = append(manifest.Targets, signingManifestTarget{Name: name, Configurations: configs})
	}
	return manifest
}

func overlaySigningManifest(inferred, overrides *signingSettingsManifest) []string {
	if inferred == nil || overrides == nil {
		return nil
	}
	warnings := make([]string, 0)
	index := make(map[string]int, len(inferred.Targets))
	for i, target := range inferred.Targets {
		index[target.Name] = i
	}
	for _, override := range overrides.Targets {
		name := strings.TrimSpace(override.Name)
		position, ok := index[name]
		if !ok {
			inferred.Targets = append(inferred.Targets, cloneSigningManifestTarget(override))
			index[name] = len(inferred.Targets) - 1
			continue
		}
		configIndex := make(map[string]int, len(inferred.Targets[position].Configurations))
		for i, configuration := range inferred.Targets[position].Configurations {
			configIndex[configuration.Name] = i
		}
		for _, configuration := range override.Configurations {
			configName := strings.TrimSpace(configuration.Name)
			configPosition, found := configIndex[configName]
			if !found {
				inferred.Targets[position].Configurations = append(inferred.Targets[position].Configurations, cloneSigningManifestConfiguration(configuration))
				continue
			}
			existing := inferred.Targets[position].Configurations[configPosition].Settings
			for key, value := range configuration.Settings {
				if _, present := existing[key]; present && string(existing[key]) != string(value) {
					warnings = append(warnings, fmt.Sprintf("settings file overrides inferred %s for %s/%s", key, name, configName))
				}
				cloned := make(json.RawMessage, len(value))
				copy(cloned, value)
				existing[key] = cloned
			}
			if signingSettingsAutomatic(configuration.Settings) {
				// Automatic signing picks its own profile and identity, so the
				// inferred manual profile and certificate are removed unless
				// the settings file sets them explicitly.
				for _, key := range []string{"PROVISIONING_PROFILE_SPECIFIER", "CODE_SIGN_IDENTITY"} {
					if _, explicit := configuration.Settings[key]; !explicit {
						existing[key] = json.RawMessage("null")
					}
				}
			}
		}
	}
	sort.Strings(warnings)
	return warnings
}

func cloneSigningManifest(manifest *signingSettingsManifest) *signingSettingsManifest {
	if manifest == nil {
		return nil
	}
	cloned := &signingSettingsManifest{
		SchemaVersion: manifest.SchemaVersion,
		Targets:       make([]signingManifestTarget, len(manifest.Targets)),
	}
	for index, target := range manifest.Targets {
		cloned.Targets[index] = cloneSigningManifestTarget(target)
	}
	return cloned
}

func cloneSigningManifestTarget(target signingManifestTarget) signingManifestTarget {
	cloned := signingManifestTarget{Name: target.Name, Configurations: make([]signingManifestConfiguration, len(target.Configurations))}
	for i, configuration := range target.Configurations {
		cloned.Configurations[i] = cloneSigningManifestConfiguration(configuration)
	}
	return cloned
}

func cloneSigningManifestConfiguration(configuration signingManifestConfiguration) signingManifestConfiguration {
	cloned := signingManifestConfiguration{Name: configuration.Name, Settings: make(map[string]json.RawMessage, len(configuration.Settings))}
	for key, value := range configuration.Settings {
		copied := make(json.RawMessage, len(value))
		copy(copied, value)
		cloned.Settings[key] = copied
	}
	return cloned
}

// inferSigningExportMethod returns the one export method every selected
// profile implies. One ExportOptions.plist applies a single method to every
// target, so profiles that imply different methods cannot be exported
// together; that is a blocker unless the caller names --export-method.
func inferSigningExportMethod(assigned []signingProfileAssignment) (string, string) {
	if len(assigned) == 0 {
		return "", ""
	}
	methods := make(map[string]bool)
	for _, item := range assigned {
		methods[item.profile.method] = true
	}
	if len(methods) == 1 {
		return assigned[0].profile.method, ""
	}
	names := make([]string, 0, len(methods))
	for method := range methods {
		names = append(names, method)
	}
	sort.Strings(names)
	return names[0], fmt.Sprintf("selected profiles imply different export methods (%s); pass --export-method to choose one", strings.Join(names, ", "))
}

// signingExportOptions builds export options from the final settings, after
// any --settings-file override, so the plist names the same bundle ID,
// profile, and team that the plan writes into the project. It also returns
// the distinct development teams those settings use.
func signingExportOptions(method string, assigned, settingsOnly []signingProfileAssignment, manifest *signingSettingsManifest) (*SigningPlanExportOptions, []string) {
	profiles := make(map[string]string)
	teamSet := make(map[string]bool)
	items := make([]signingProfileAssignment, 0, len(assigned)+len(settingsOnly))
	items = append(items, assigned...)
	items = append(items, settingsOnly...)
	for _, item := range items {
		bundleID := item.bundleID
		if value, found := signingManifestString(manifest, item.target, item.configuration, "PRODUCT_BUNDLE_IDENTIFIER"); found && value != "" {
			bundleID = value
		}
		if style, found := signingManifestString(manifest, item.target, item.configuration, "CODE_SIGN_STYLE"); found && strings.EqualFold(style, "Automatic") {
			// Automatic signing resolves its own profile; a manual export
			// options entry would contradict it.
			continue
		}
		name, team := "", ""
		if item.profile != nil {
			name, team = item.profile.name, item.profile.teamID
		}
		if value, found := signingManifestString(manifest, item.target, item.configuration, "PROVISIONING_PROFILE_SPECIFIER"); found {
			name = value
		}
		if name == "" {
			// ExportOptions accepts a profile UUID as well as a name, so a
			// target pinned only by PROVISIONING_PROFILE still maps.
			if value, found := signingManifestString(manifest, item.target, item.configuration, "PROVISIONING_PROFILE"); found {
				name = value
			}
		}
		if value, found := signingManifestString(manifest, item.target, item.configuration, "DEVELOPMENT_TEAM"); found {
			team = value
		}
		if name != "" && bundleID != "" {
			profiles[bundleID] = name
		}
		if team != "" {
			teamSet[team] = true
		}
	}
	teams := make([]string, 0, len(teamSet))
	for team := range teamSet {
		teams = append(teams, team)
	}
	sort.Strings(teams)
	// Export options are manual-signing mappings; with no manually signed
	// target left (all automatic, or none selected) there is nothing to map.
	if method == "" || len(profiles) == 0 {
		return nil, teams
	}
	options := &SigningPlanExportOptions{
		Method:               method,
		SigningStyle:         "manual",
		ProvisioningProfiles: profiles,
	}
	if len(teams) > 0 {
		options.TeamID = teams[0]
	}
	return options, teams
}

// signingManifestString returns a string setting from the final manifest. A
// JSON null (a removal) is reported as found with an empty value.
func signingManifestString(manifest *signingSettingsManifest, target, configuration, key string) (string, bool) {
	if manifest == nil {
		return "", false
	}
	for _, item := range manifest.Targets {
		if strings.TrimSpace(item.Name) != target {
			continue
		}
		for _, config := range item.Configurations {
			if strings.TrimSpace(config.Name) != configuration {
				continue
			}
			raw, ok := config.Settings[key]
			if !ok {
				continue
			}
			var value *string
			if err := json.Unmarshal(raw, &value); err != nil {
				continue
			}
			if value == nil {
				return "", true
			}
			return strings.TrimSpace(*value), true
		}
	}
	return "", false
}

func readSigningProfiles(paths []string) ([]signingProfile, []string, error) {
	canonical := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		absolute, err := canonicalSigningPath(path, "profile")
		if err != nil {
			return nil, nil, err
		}
		if seen[absolute] {
			continue
		}
		seen[absolute] = true
		canonical = append(canonical, absolute)
	}
	sort.Strings(canonical)
	profiles := make([]signingProfile, 0, len(canonical))
	for _, path := range canonical {
		profile, err := parseSigningProfile(path)
		if err != nil {
			return nil, nil, err
		}
		profiles = append(profiles, profile)
	}
	return profiles, canonical, nil
}

func parseSigningProfile(path string) (signingProfile, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".mobileprovision" && extension != ".provisionprofile" {
		return signingProfile{}, fmt.Errorf("profile %s must be .mobileprovision or .provisionprofile", path)
	}
	data, err := readSigningRegularFile(path, signingProfileMaxBytes)
	if err != nil {
		return signingProfile{}, fmt.Errorf("read profile %s: %w", path, err)
	}
	signed, err := pkcs7.Parse(data)
	if err != nil {
		return signingProfile{}, fmt.Errorf("parse profile %s: %w", path, err)
	}
	// Check the CMS signature over the payload so a modified or corrupted
	// profile is rejected instead of trusted for inference.
	if err := signed.Verify(); err != nil {
		return signingProfile{}, fmt.Errorf("verify profile %s signature: %w", path, err)
	}
	var payload struct {
		UUID                        string         `plist:"UUID"`
		Name                        string         `plist:"Name"`
		TeamIdentifier              []string       `plist:"TeamIdentifier"`
		ApplicationIdentifierPrefix []string       `plist:"ApplicationIdentifierPrefix"`
		ExpirationDate              time.Time      `plist:"ExpirationDate"`
		Entitlements                map[string]any `plist:"Entitlements"`
		DeveloperCertificates       [][]byte       `plist:"DeveloperCertificates"`
		ProvisionsAllDevices        bool           `plist:"ProvisionsAllDevices"`
		ProvisionedDevices          []string       `plist:"ProvisionedDevices"`
		Platform                    []string       `plist:"Platform"`
	}
	if _, err := plist.Unmarshal(signed.Content, &payload); err != nil {
		return signingProfile{}, fmt.Errorf("decode profile %s: %w", path, err)
	}
	teamID := firstSigningProfileString(payload.TeamIdentifier)
	prefix := firstSigningProfileString(payload.ApplicationIdentifierPrefix)
	if prefix == "" {
		prefix = teamID
	}
	// macOS profiles carry com.apple.application-identifier instead of the
	// iOS-family application-identifier key.
	applicationID, _ := payload.Entitlements["application-identifier"].(string)
	if strings.TrimSpace(applicationID) == "" {
		applicationID, _ = payload.Entitlements["com.apple.application-identifier"].(string)
	}
	pattern, wildcard, err := signingProfilePattern(applicationID, prefix)
	if err != nil {
		return signingProfile{}, fmt.Errorf("profile %s: %w", path, err)
	}
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.UUID) == "" {
		return signingProfile{}, fmt.Errorf("profile %s is missing a name or UUID", path)
	}
	if !signingTeamIDPattern.MatchString(strings.ToUpper(teamID)) {
		return signingProfile{}, fmt.Errorf("profile %s has invalid team ID %q", path, teamID)
	}
	identity, certSHA, certExpires, certValid, err := signingProfileIdentity(payload.DeveloperCertificates, signingProfileNow())
	if err != nil {
		return signingProfile{}, fmt.Errorf("profile %s: %w", path, err)
	}
	profile := signingProfile{
		path:        path,
		name:        strings.TrimSpace(payload.Name),
		uuid:        strings.TrimSpace(payload.UUID),
		teamID:      strings.ToUpper(teamID),
		pattern:     pattern,
		wildcard:    wildcard,
		expires:     earliestSigningExpiry(payload.ExpirationDate, certExpires),
		noValidCert: !certValid,
		identity:    identity,
		certSHA256:  certSHA,
		platforms:   append([]string(nil), payload.Platform...),
	}
	profile.method = signingProfileExportMethod(signingProfileMacOnly(profile), payload.ProvisionsAllDevices, len(payload.ProvisionedDevices) > 0, entitlementBool(payload.Entitlements["get-task-allow"]))
	return profile, nil
}

func signingProfilePattern(applicationID, prefix string) (string, bool, error) {
	applicationID = strings.TrimSpace(applicationID)
	prefix = strings.TrimSpace(prefix)
	if applicationID == "" || prefix == "" {
		return "", false, fmt.Errorf("missing application identifier")
	}
	qualified := prefix + "."
	if !strings.HasPrefix(applicationID, qualified) {
		return "", false, fmt.Errorf("application identifier %q does not use prefix %s", applicationID, prefix)
	}
	pattern := strings.TrimPrefix(applicationID, qualified)
	if pattern == "*" {
		// TEAM.* is the team-wide wildcard App ID used by Xcode-managed and
		// generic development profiles; it matches any bundle identifier.
		return pattern, true, nil
	}
	if pattern == "" || strings.Count(pattern, "*") > 1 || strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, ".*") {
		return "", false, fmt.Errorf("unsupported application identifier %q", applicationID)
	}
	return pattern, strings.HasSuffix(pattern, ".*"), nil
}

// signingProfileIdentity picks the developer certificate the plan signs
// with: the one valid at now with the latest expiration, falling back to the
// latest-expiring certificate when none is currently valid. It returns that
// certificate's expiration and whether it is valid now, so a profile with no
// currently valid certificate is never selected.
func signingProfileIdentity(certificates [][]byte, now time.Time) (string, string, time.Time, bool, error) {
	if len(certificates) == 0 {
		return "", "", time.Time{}, false, fmt.Errorf("missing developer certificate")
	}
	var chosen *x509.Certificate
	var chosenDER []byte
	chosenValid := false
	validKinds := make(map[string]bool)
	validCount := 0
	for _, der := range certificates {
		certificate, err := x509.ParseCertificate(der)
		if err != nil {
			return "", "", time.Time{}, false, fmt.Errorf("parse developer certificate: %w", err)
		}
		valid := !now.Before(certificate.NotBefore) && now.Before(certificate.NotAfter)
		if valid {
			validCount++
			validKinds[signingCertificateKind(certificate.Subject.CommonName)] = true
		}
		switch {
		case chosen == nil,
			valid && !chosenValid,
			valid == chosenValid && certificate.NotAfter.After(chosen.NotAfter):
			chosen, chosenDER, chosenValid = certificate, der, valid
		}
	}
	sum := sha256.Sum256(chosenDER)
	identity := strings.TrimSpace(chosen.Subject.CommonName)
	if validCount > 1 && len(validKinds) == 1 {
		// A team profile embeds several members' certificates, and only one
		// may have its private key on this machine. Name the certificate
		// kind (for example "Apple Development") so Xcode picks whichever
		// matching identity is installed, instead of pinning one member's.
		identity = signingCertificateKind(identity)
	}
	if identity == "" {
		identity = "Apple Distribution"
	}
	return identity, hex.EncodeToString(sum[:]), chosen.NotAfter, chosenValid, nil
}

// signingCertificateKind returns the generic identity for a certificate
// common name: the part before ": ", such as "Apple Development".
func signingCertificateKind(commonName string) string {
	kind, _, _ := strings.Cut(strings.TrimSpace(commonName), ":")
	return strings.TrimSpace(kind)
}

// earliestSigningExpiry is when a profile stops being usable: the profile
// or its signing certificate, whichever expires first.
func earliestSigningExpiry(profile, certificate time.Time) time.Time {
	if profile.IsZero() || !certificate.IsZero() && certificate.Before(profile) {
		return certificate
	}
	return profile
}

func signingProfileExportMethod(mac, allDevices, hasDevices, debuggable bool) string {
	if mac {
		// macOS has no ad-hoc or enterprise distribution: a profile that
		// provisions all devices is Developer ID, a device list is development.
		switch {
		case allDevices:
			return "developer-id"
		case hasDevices:
			return "development"
		default:
			return "app-store"
		}
	}
	enterprise := allDevices
	switch {
	case enterprise:
		return "enterprise"
	case hasDevices && debuggable:
		return "development"
	case hasDevices:
		return "ad-hoc"
	default:
		return "app-store"
	}
}

func entitlementBool(value any) bool {
	enabled, _ := value.(bool)
	return enabled
}

func firstSigningProfileString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func mustRawJSON(value string) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return encoded
}

func appendSigningProfileInputs(paths []string, inference *signingProfileInference) []string {
	if inference == nil {
		return paths
	}
	return append(paths, inference.paths...)
}

func recordSigningProfileInference(plan *SigningPlan, inference *signingProfileInference, opts SigningPlanOptions) {
	attachSigningProfileInference(plan, inference, opts)
	sort.Strings(plan.Blockers)
	sort.Strings(plan.Warnings)
}

func blockedSigningProfilePlan(opts SigningPlanOptions, project *structuredVersionProject, settingsPath, planPath, receiptPath string, inference *signingProfileInference) (*signingPlanBuild, error) {
	plan := &SigningPlan{
		SchemaVersion:         signingPlanSchemaVersion,
		Command:               signingPlanCommand,
		GeneratedAt:           time.Now().UTC().Format(time.RFC3339Nano),
		Ready:                 false,
		ProjectPath:           project.projectPath,
		SettingsFilePath:      settingsPath,
		PlanPath:              planPath,
		ReceiptPath:           receiptPath,
		AllowExternalXCConfig: opts.AllowExternalXCConfig,
		Desired:               []SigningPlanTarget{},
		Files:                 []SigningPlanFile{},
		Changes:               []SigningSettingChange{},
		Blockers:              []string{"no signing targets were inferred"},
		Warnings:              []string{},
	}
	recordSigningProfileInference(plan, inference, opts)
	if len(plan.Blockers) == 0 {
		plan.Blockers = []string{"no signing targets were inferred"}
	}
	plan.Ready = false
	plan.PlanHash = signingPlanHash(plan)
	return &signingPlanBuild{plan: plan, project: project}, nil
}

func attachSigningProfileInference(plan *SigningPlan, inference *signingProfileInference, opts SigningPlanOptions) {
	if plan == nil || inference == nil {
		return
	}
	plan.ProfilePaths = append([]string(nil), inference.paths...)
	plan.Inferences = append([]SigningPlanInference(nil), inference.inferences...)
	plan.ExportOptions = cloneSigningExportOptions(inference.exportOptions)
	plan.Configuration = strings.TrimSpace(opts.Configuration)
	plan.ExportMethod = inference.exportMethod
	plan.SkipTargets = append([]string(nil), inference.skipTargets...)
	plan.Blockers = append(plan.Blockers, inference.blockers...)
	plan.Warnings = append(plan.Warnings, inference.warnings...)
}

func cloneSigningExportOptions(options *SigningPlanExportOptions) *SigningPlanExportOptions {
	if options == nil {
		return nil
	}
	cloned := *options
	if options.ProvisioningProfiles != nil {
		cloned.ProvisioningProfiles = make(map[string]string, len(options.ProvisioningProfiles))
		for key, value := range options.ProvisioningProfiles {
			cloned.ProvisioningProfiles[key] = value
		}
	}
	return &cloned
}

// CheckSigningExportOptionsDestination verifies, without writing, that
// WriteSigningExportOptions can publish path: an existing file is accepted
// only when overwrite is set.
func CheckSigningExportOptionsDestination(path string, overwrite bool) error {
	root, name, err := openSigningExportOptionsRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	if overwrite {
		err = root.CheckWriteFile(name)
	} else {
		err = root.CheckCreateNewFile(name)
	}
	if err != nil {
		return fmt.Errorf("export options %s: %w", filepath.Join(root.Path(), name), err)
	}
	return nil
}

// CheckSigningExportOptionsAliases rejects an export-options destination that
// names the plan, its receipt, the project, or any file the plan read, so
// writing the plist can never replace a plan artifact or project input.
func CheckSigningExportOptionsAliases(path string, plan *SigningPlan) error {
	if plan == nil {
		return nil
	}
	destination, err := canonicalSigningPath(path, "export options")
	if err != nil {
		return err
	}
	protected := []string{plan.PlanPath, plan.ReceiptPath, plan.SettingsFilePath}
	if plan.ProjectPath != "" {
		protected = append(protected, plan.ProjectPath, filepath.Join(plan.ProjectPath, "project.pbxproj"))
	}
	protected = append(protected, plan.ProfilePaths...)
	for _, file := range plan.Files {
		protected = append(protected, file.Path)
	}
	destinationInfo, destinationErr := os.Stat(destination)
	for _, candidate := range protected {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		absolute, err := canonicalSigningPath(candidate, "plan file")
		if err != nil {
			continue
		}
		same := strings.EqualFold(absolute, destination)
		if !same && destinationErr == nil {
			if info, statErr := os.Stat(absolute); statErr == nil && os.SameFile(info, destinationInfo) {
				same = true
			}
		}
		if same {
			return fmt.Errorf("--export-options-out must not be %s, which the signing plan writes or reads", absolute)
		}
	}
	return nil
}

func openSigningExportOptionsRoot(path string) (rootfs.Root, string, error) {
	absolute, err := canonicalSigningPath(path, "export options")
	if err != nil {
		return rootfs.Root{}, "", err
	}
	parent := filepath.Dir(absolute)
	parentInfo, statErr := os.Lstat(parent)
	if statErr == nil && parentInfo.Mode()&os.ModeSymlink != 0 {
		return rootfs.Root{}, "", fmt.Errorf("export options %s: %w", absolute, rootfs.ErrSymlink)
	}
	root, err := rootfs.New(parent)
	if err != nil {
		return rootfs.Root{}, "", fmt.Errorf("export options %s: %w", absolute, err)
	}
	return root, filepath.Base(absolute), nil
}

// WriteSigningExportOptions writes an ExportOptions.plist for a planned
// profile set. An existing file is replaced only when overwrite is set.
func WriteSigningExportOptions(path string, options *SigningPlanExportOptions, overwrite bool) error {
	if options == nil || strings.TrimSpace(options.Method) == "" {
		return fmt.Errorf("export options were not inferred")
	}
	payload := map[string]any{
		"method":       options.Method,
		"signingStyle": options.SigningStyle,
	}
	if options.TeamID != "" {
		payload["teamID"] = options.TeamID
	}
	if len(options.ProvisioningProfiles) > 0 {
		payload["provisioningProfiles"] = options.ProvisioningProfiles
	}
	data, err := plist.Marshal(payload, plist.XMLFormat)
	if err != nil {
		return fmt.Errorf("encode export options: %w", err)
	}
	root, name, err := openSigningExportOptionsRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	if overwrite {
		err = root.WriteFile(name, data, 0o600)
	} else {
		err = root.CreateNewFile(name, data, 0o600)
	}
	if err != nil {
		return fmt.Errorf("write export options %s: %w", filepath.Join(root.Path(), name), err)
	}
	return nil
}
