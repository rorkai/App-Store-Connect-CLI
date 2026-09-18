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
	method     string
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
	skipped := make(map[string]bool, len(opts.SkipTargets))
	skipTargets := make([]string, 0, len(opts.SkipTargets))
	for _, target := range opts.SkipTargets {
		name := strings.TrimSpace(target)
		if name == "" || skipped[name] {
			continue
		}
		skipped[name] = true
		skipTargets = append(skipTargets, name)
	}
	sort.Strings(skipTargets)

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

	assigned := make([]signingProfileAssignment, 0)
	blockers := make([]string, 0)
	warnings := make([]string, 0)
	covered := signingManifestCoverage(overrides)
	seenConfiguration := configurationFilter == ""
	for _, scope := range scopes {
		if scope.name == configurationFilter {
			seenConfiguration = true
		}
		if skipped[scope.target] {
			continue
		}
		bundleID, bundleErr := signingBundleID(project, scope)
		if bundleErr != nil || bundleID == "" {
			if covered[scope.target+"\x00"+scope.name] {
				continue
			}
			detail := "PRODUCT_BUNDLE_IDENTIFIER is not set"
			if bundleErr != nil {
				detail = bundleErr.Error()
			}
			blockers = append(blockers, fmt.Sprintf("unmatched signing target %s/%s: %s", scope.target, scope.name, detail))
			continue
		}
		selected, discarded, match := selectSigningProfile(profiles, bundleID)
		if selected == nil {
			if covered[scope.target+"\x00"+scope.name] {
				continue
			}
			blockers = append(blockers, fmt.Sprintf("unmatched signing target %s/%s bundle ID %s", scope.target, scope.name, bundleID))
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
		method, warnings = inferSigningExportMethod(assigned, warnings)
	}
	exportOptions := signingExportOptions(method, assigned)
	if len(assigned) > 1 {
		teams := make(map[string]struct{})
		for _, item := range assigned {
			teams[item.profile.teamID] = struct{}{}
		}
		if len(teams) > 1 {
			blockers = append(blockers, "selected profiles use more than one development team")
		}
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
			covered[strings.TrimSpace(target.Name)+"\x00"+strings.TrimSpace(configuration.Name)] = true
		}
	}
	return covered
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

func inferSigningExportMethod(assigned []signingProfileAssignment, warnings []string) (string, []string) {
	if len(assigned) == 0 {
		return "", warnings
	}
	method := assigned[0].profile.method
	for _, item := range assigned[1:] {
		if item.profile.method != method {
			warnings = append(warnings, fmt.Sprintf("selected profiles imply different export methods; using %s", method))
			break
		}
	}
	return method, warnings
}

func signingExportOptions(method string, assigned []signingProfileAssignment) *SigningPlanExportOptions {
	if method == "" || len(assigned) == 0 {
		return nil
	}
	profiles := make(map[string]string)
	teamID := assigned[0].profile.teamID
	for _, item := range assigned {
		profiles[item.bundleID] = item.profile.name
	}
	return &SigningPlanExportOptions{
		Method:               method,
		SigningStyle:         "manual",
		TeamID:               teamID,
		ProvisioningProfiles: profiles,
	}
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
	}
	if _, err := plist.Unmarshal(signed.Content, &payload); err != nil {
		return signingProfile{}, fmt.Errorf("decode profile %s: %w", path, err)
	}
	teamID := firstSigningProfileString(payload.TeamIdentifier)
	prefix := firstSigningProfileString(payload.ApplicationIdentifierPrefix)
	applicationID, _ := payload.Entitlements["application-identifier"].(string)
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
	identity, certSHA, err := signingProfileIdentity(payload.DeveloperCertificates)
	if err != nil {
		return signingProfile{}, fmt.Errorf("profile %s: %w", path, err)
	}
	return signingProfile{
		path:       path,
		name:       strings.TrimSpace(payload.Name),
		uuid:       strings.TrimSpace(payload.UUID),
		teamID:     strings.ToUpper(teamID),
		pattern:    pattern,
		wildcard:   wildcard,
		expires:    payload.ExpirationDate,
		identity:   identity,
		certSHA256: certSHA,
		method:     signingProfileExportMethod(payload.ProvisionsAllDevices, len(payload.ProvisionedDevices) > 0, entitlementBool(payload.Entitlements["get-task-allow"])),
	}, nil
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
	if pattern == "" || strings.Contains(pattern, "*") && !strings.HasSuffix(pattern, ".*") {
		return "", false, fmt.Errorf("unsupported application identifier %q", applicationID)
	}
	return pattern, strings.HasSuffix(pattern, ".*"), nil
}

func signingProfileIdentity(certificates [][]byte) (string, string, error) {
	if len(certificates) == 0 {
		return "", "", fmt.Errorf("missing developer certificate")
	}
	certificate, err := x509.ParseCertificate(certificates[0])
	if err != nil {
		return "", "", fmt.Errorf("parse developer certificate: %w", err)
	}
	sum := sha256.Sum256(certificates[0])
	identity := strings.TrimSpace(certificate.Subject.CommonName)
	if identity == "" {
		identity = "Apple Distribution"
	}
	return identity, hex.EncodeToString(sum[:]), nil
}

func signingProfileExportMethod(enterprise, hasDevices, debuggable bool) string {
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

// WriteSigningExportOptions writes an ExportOptions.plist for a planned profile set.
func WriteSigningExportOptions(path string, options *SigningPlanExportOptions) error {
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
	absolute, err := canonicalSigningPath(path, "export options")
	if err != nil {
		return err
	}
	parent := filepath.Dir(absolute)
	parentInfo, statErr := os.Lstat(parent)
	if statErr == nil && parentInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("write export options %s: %w", absolute, rootfs.ErrSymlink)
	}
	root, err := rootfs.New(parent)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.CreateNewFile(filepath.Base(absolute), data, 0o600); err != nil {
		return fmt.Errorf("write export options %s: %w", absolute, err)
	}
	return nil
}
