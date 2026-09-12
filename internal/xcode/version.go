package xcode

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// BumpType represents the version component to increment.
type BumpType string

// BuildSettingsLookupPolicy controls whether version commands may fall back to
// xcodebuild when structured project parsing cannot resolve version settings.
type BuildSettingsLookupPolicy string

// BuildSettingsLookupSession shares successful fallback reads and diagnostic
// state across the phases of one version command.
type BuildSettingsLookupSession struct {
	policy     BuildSettingsLookupPolicy
	diagnostic io.Writer
	warned     bool
	cache      map[string]map[string]string
}

const (
	BumpMajor BumpType = "major"
	BumpMinor BumpType = "minor"
	BumpPatch BumpType = "patch"
	BumpBuild BumpType = "build"

	BuildSettingsLookupAuto  BuildSettingsLookupPolicy = "auto"
	BuildSettingsLookupNever BuildSettingsLookupPolicy = "never"
)

// ParseBumpType validates and normalizes a bump type string.
func ParseBumpType(s string) (BumpType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "major":
		return BumpMajor, nil
	case "minor":
		return BumpMinor, nil
	case "patch":
		return BumpPatch, nil
	case "build":
		return BumpBuild, nil
	default:
		return "", fmt.Errorf("--type must be one of: major, minor, patch, build")
	}
}

// ParseBuildSettingsLookupPolicy validates and normalizes the xcodebuild
// build-settings fallback policy. An empty value preserves the default auto
// behavior for programmatic callers.
func ParseBuildSettingsLookupPolicy(value string) (BuildSettingsLookupPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", string(BuildSettingsLookupAuto):
		return BuildSettingsLookupAuto, nil
	case string(BuildSettingsLookupNever):
		return BuildSettingsLookupNever, nil
	default:
		return "", fmt.Errorf("--xcodebuild-settings-lookup must be one of: auto, never")
	}
}

// NewBuildSettingsLookupSession creates command-scoped state for xcodebuild
// build-settings fallback reads.
func NewBuildSettingsLookupSession(policy BuildSettingsLookupPolicy, diagnostic io.Writer) *BuildSettingsLookupSession {
	return &BuildSettingsLookupSession{
		policy:     policy,
		diagnostic: diagnostic,
		cache:      make(map[string]map[string]string),
	}
}

func resolveBuildSettingsLookupSession(
	policy BuildSettingsLookupPolicy,
	diagnostic io.Writer,
	session *BuildSettingsLookupSession,
) (*BuildSettingsLookupSession, error) {
	if session == nil {
		normalized, err := ParseBuildSettingsLookupPolicy(string(policy))
		if err != nil {
			return nil, err
		}
		return NewBuildSettingsLookupSession(normalized, diagnostic), nil
	}

	normalizedSessionPolicy, err := ParseBuildSettingsLookupPolicy(string(session.policy))
	if err != nil {
		return nil, err
	}
	session.policy = normalizedSessionPolicy
	if strings.TrimSpace(string(policy)) != "" {
		normalizedPolicy, err := ParseBuildSettingsLookupPolicy(string(policy))
		if err != nil {
			return nil, err
		}
		if normalizedPolicy != normalizedSessionPolicy {
			return nil, fmt.Errorf("build settings lookup session policy %q conflicts with policy %q", normalizedSessionPolicy, normalizedPolicy)
		}
	}
	if session.cache == nil {
		session.cache = make(map[string]map[string]string)
	}
	return session, nil
}

func (session *BuildSettingsLookupSession) invalidateCache() {
	if session != nil {
		clear(session.cache)
	}
}

// VersionInfo holds the current version and build number from an Xcode project.
type VersionInfo struct {
	Version           string `json:"version"`
	BuildNumber       string `json:"buildNumber"`
	ProjectDir        string `json:"projectDir"`
	Target            string `json:"target,omitempty"`
	Configuration     string `json:"configuration,omitempty"`
	VersionSource     string `json:"versionSource,omitempty"`
	BuildNumberSource string `json:"buildNumberSource,omitempty"`
	Modern            bool   `json:"modern"` // true if project uses MARKETING_VERSION build setting
}

// GetVersionOptions configures a structured version read.
type GetVersionOptions struct {
	ProjectDir              string
	Target                  string
	Configuration           string
	BuildSettingsLookup     BuildSettingsLookupPolicy
	BuildSettingsDiagnostic io.Writer
	BuildSettingsSession    *BuildSettingsLookupSession
}

// SetVersionOptions configures what to set.
type SetVersionOptions struct {
	ProjectDir    string
	Target        string
	Configuration string
	Version       string
	BuildNumber   string
	// AllowExternalXCConfig authorizes rewriting xcconfig files that the project
	// references outside its own directory. Without it, such a mutation fails.
	AllowExternalXCConfig bool
}

// VersionChange describes one concrete build-setting mutation.
type VersionChange struct {
	Setting       string `json:"setting"`
	OldValue      string `json:"oldValue,omitempty"`
	NewValue      string `json:"newValue"`
	Target        string `json:"target,omitempty"`
	Configuration string `json:"configuration,omitempty"`
	Path          string `json:"path"`
	Source        string `json:"source"`
}

// SetVersionResult holds the result of a set operation.
type SetVersionResult struct {
	Version       string          `json:"version,omitempty"`
	BuildNumber   string          `json:"buildNumber,omitempty"`
	ProjectDir    string          `json:"projectDir"`
	Target        string          `json:"target,omitempty"`
	Configuration string          `json:"configuration,omitempty"`
	ChangedFiles  []string        `json:"changedFiles"`
	Changes       []VersionChange `json:"changes"`
}

// BumpVersionOptions configures the bump operation.
type BumpVersionOptions struct {
	ProjectDir              string
	Target                  string
	Configuration           string
	BumpType                BumpType
	BuildNumber             string
	BuildSettingsLookup     BuildSettingsLookupPolicy
	BuildSettingsDiagnostic io.Writer
	BuildSettingsSession    *BuildSettingsLookupSession
	// AllowExternalXCConfig authorizes rewriting xcconfig files that the project
	// references outside its own directory. Without it, such a mutation fails.
	AllowExternalXCConfig bool
}

// BumpVersionResult holds the result of a bump operation.
type BumpVersionResult struct {
	BumpType      string          `json:"bumpType"`
	OldVersion    string          `json:"oldVersion,omitempty"`
	NewVersion    string          `json:"newVersion,omitempty"`
	OldBuild      string          `json:"oldBuild,omitempty"`
	NewBuild      string          `json:"newBuild,omitempty"`
	ProjectDir    string          `json:"projectDir"`
	Target        string          `json:"target,omitempty"`
	Configuration string          `json:"configuration,omitempty"`
	ChangedFiles  []string        `json:"changedFiles"`
	Changes       []VersionChange `json:"changes"`
}

func resolvedProjectDir(projectDir string) string {
	projectDir = trimTrailingPathSeparators(projectDir)
	if projectDir == "" {
		return "."
	}
	if strings.HasSuffix(projectDir, ".xcodeproj") {
		if resolved, err := resolveProjectParentTraversal(projectDir); err == nil {
			projectDir = resolved
		}
		return filepath.Dir(projectDir)
	}
	return projectDir
}

// GetVersion reads the current marketing version and build number.
func GetVersion(ctx context.Context, projectDir, target string) (*VersionInfo, error) {
	return GetVersionScoped(ctx, GetVersionOptions{ProjectDir: projectDir, Target: target})
}

func getVersionLegacy(
	ctx context.Context,
	projectDir string,
	target string,
	lookupSession *BuildSettingsLookupSession,
) (*VersionInfo, error) {
	if err := requireMacOS(); err != nil {
		return nil, err
	}
	if err := requireAgvtool(ctx); err != nil {
		return nil, err
	}

	version, err := runAgvtool(ctx, projectDir, "what-marketing-version", "-terse1")
	if err != nil {
		return nil, fmt.Errorf("failed to read marketing version: %w", err)
	}

	buildNumber, err := runAgvtool(ctx, projectDir, "what-version", "-terse")
	if err != nil {
		return nil, fmt.Errorf("failed to read build number: %w", err)
	}

	trimmedTarget := strings.TrimSpace(target)
	parsedVersion, err := parseAgvtoolVersionOutput(version, trimmedTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse marketing version: %w", err)
	}
	parsedBuild, err := parseAgvtoolBuildOutput(buildNumber, trimmedTarget)
	if err != nil {
		return nil, fmt.Errorf("failed to parse build number: %w", err)
	}
	modern := isVariableReference(parsedVersion)

	// Modern project: agvtool returns $(MARKETING_VERSION). Resolve via xcodebuild.
	if modern {
		resolved, err := readBuildSettings(ctx, projectDir, target, lookupSession)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve build settings: %w", err)
		}
		if v := resolved["MARKETING_VERSION"]; v != "" {
			parsedVersion = v
		}
		if b := resolved["CURRENT_PROJECT_VERSION"]; b != "" {
			parsedBuild = b
		}
	}

	return &VersionInfo{
		Version:     parsedVersion,
		BuildNumber: parsedBuild,
		ProjectDir:  resolvedProjectDir(projectDir),
		Target:      target,
		Modern:      modern,
	}, nil
}

// SetVersion sets the marketing version and/or build number.
func SetVersion(ctx context.Context, opts SetVersionOptions) (*SetVersionResult, error) {
	if err := validateVersionMutationValue("--version", opts.Version); err != nil {
		return nil, err
	}
	if err := validateVersionMutationValue("--build-number", opts.BuildNumber); err != nil {
		return nil, err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	structured, err := project.hasStructuredSettingsForMutation(opts.Target, opts.Configuration, setVersionRequestedSettings(opts)...)
	if err != nil {
		return nil, err
	}
	if structured {
		return project.setVersion(opts)
	}
	err = fmt.Errorf("%w: selected Xcode configurations do not resolve both MARKETING_VERSION and CURRENT_PROJECT_VERSION", errStructuredVersionUnavailable)
	if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "" {
		return nil, fmt.Errorf("scoped edits require structured Xcode build settings: %w", err)
	}
	return setVersionLegacy(ctx, opts)
}

// ValidateSetVersion verifies that a version mutation is locally valid and
// editable without changing any files. Callers can use it before remote work.
func ValidateSetVersion(ctx context.Context, opts SetVersionOptions) error {
	if err := validateVersionMutationValue("--version", opts.Version); err != nil {
		return err
	}
	if err := validateVersionMutationValue("--build-number", opts.BuildNumber); err != nil {
		return err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return err
	}
	structured, err := project.hasStructuredSettingsForMutation(opts.Target, opts.Configuration, setVersionRequestedSettings(opts)...)
	if err != nil {
		return err
	}
	if structured {
		_, err := project.validateSetVersion(opts)
		return err
	}
	structuredErr := fmt.Errorf("%w: selected Xcode configurations do not resolve both MARKETING_VERSION and CURRENT_PROJECT_VERSION", errStructuredVersionUnavailable)
	if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "" {
		return fmt.Errorf("scoped edits require structured Xcode build settings: %w", structuredErr)
	}
	return validateSetVersionLegacy(ctx)
}

func setVersionLegacy(ctx context.Context, opts SetVersionOptions) (*SetVersionResult, error) {
	if err := validateSetVersionLegacy(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.Target) != "" {
		return nil, fmt.Errorf("--target is only supported by xcode version view; edit updates the whole project")
	}

	result := &SetVersionResult{ProjectDir: resolvedProjectDir(opts.ProjectDir)}

	if v := strings.TrimSpace(opts.Version); v != "" {
		if _, err := runAgvtool(ctx, opts.ProjectDir, "new-marketing-version", v); err != nil {
			return nil, fmt.Errorf("failed to set marketing version: %w", err)
		}
		result.Version = v
	}

	if b := strings.TrimSpace(opts.BuildNumber); b != "" {
		if _, err := runAgvtool(ctx, opts.ProjectDir, "new-version", "-all", b); err != nil {
			return nil, fmt.Errorf("failed to set build number: %w", err)
		}
		result.BuildNumber = b
	}

	return result, nil
}

func validateSetVersionLegacy(ctx context.Context) error {
	if err := requireMacOS(); err != nil {
		return err
	}
	return requireAgvtool(ctx)
}

func attachBuildSettingsLookupSession(opts *BumpVersionOptions) error {
	lookupSession, err := resolveBuildSettingsLookupSession(
		opts.BuildSettingsLookup,
		opts.BuildSettingsDiagnostic,
		opts.BuildSettingsSession,
	)
	if err != nil {
		return err
	}
	opts.BuildSettingsSession = lookupSession
	return nil
}

// BumpVersion increments the version or build number.
func BumpVersion(ctx context.Context, opts BumpVersionOptions) (*BumpVersionResult, error) {
	if err := attachBuildSettingsLookupSession(&opts); err != nil {
		return nil, err
	}
	if err := validateBumpVersionOptions(opts); err != nil {
		return nil, err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return nil, err
	}
	structured, err := project.hasStructuredSettingsForMutation(opts.Target, opts.Configuration, bumpVersionSetting(opts))
	if err != nil {
		return nil, err
	}
	if structured {
		result, setOptions, err := project.prepareBump(opts)
		if err != nil {
			return nil, err
		}
		updated, err := project.setVersion(setOptions)
		if err != nil {
			return nil, err
		}
		result.ChangedFiles = updated.ChangedFiles
		result.Changes = updated.Changes
		return result, nil
	}
	err = fmt.Errorf("%w: selected Xcode configurations do not resolve both MARKETING_VERSION and CURRENT_PROJECT_VERSION", errStructuredVersionUnavailable)
	if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "" {
		return nil, fmt.Errorf("scoped bumps require structured Xcode build settings: %w", err)
	}
	return bumpVersionLegacy(ctx, opts)
}

// ValidateBumpVersion verifies the complete local bump, including a consistent
// baseline across the selected configurations, without changing any files.
// Callers can use it before remote work such as resolving a build number.
func ValidateBumpVersion(ctx context.Context, opts BumpVersionOptions) error {
	if err := attachBuildSettingsLookupSession(&opts); err != nil {
		return err
	}
	if err := validateBumpVersionOptions(opts); err != nil {
		return err
	}
	project, err := openStructuredVersionProject(opts.ProjectDir)
	if err != nil {
		return err
	}
	structured, err := project.hasStructuredSettingsForMutation(opts.Target, opts.Configuration, bumpVersionSetting(opts))
	if err != nil {
		return err
	}
	if structured {
		_, setOptions, err := project.prepareBump(opts)
		if err != nil {
			return err
		}
		_, err = project.validateSetVersion(setOptions)
		return err
	}
	structuredErr := fmt.Errorf("%w: selected Xcode configurations do not resolve both MARKETING_VERSION and CURRENT_PROJECT_VERSION", errStructuredVersionUnavailable)
	if strings.TrimSpace(opts.Target) != "" || strings.TrimSpace(opts.Configuration) != "" {
		return fmt.Errorf("scoped bumps require structured Xcode build settings: %w", structuredErr)
	}
	if err := validateSetVersionLegacy(ctx); err != nil {
		return err
	}
	current, err := getVersionLegacy(ctx, opts.ProjectDir, "", opts.BuildSettingsSession)
	if err != nil {
		return err
	}
	if opts.BumpType == BumpBuild {
		if strings.TrimSpace(opts.BuildNumber) != "" {
			return nil
		}
		_, err = incrementBuildString(current.BuildNumber)
		if err != nil {
			return fmt.Errorf("failed to increment build number: %w", err)
		}
		return nil
	}
	_, err = bumpVersionString(current.Version, opts.BumpType)
	return err
}

func validateBumpVersionOptions(opts BumpVersionOptions) error {
	if err := validateVersionMutationValue("--build-number", opts.BuildNumber); err != nil {
		return err
	}
	if strings.TrimSpace(opts.BuildNumber) != "" && opts.BumpType != BumpBuild {
		return fmt.Errorf("--build-number is only supported for build bumps")
	}
	return nil
}

func setVersionRequestedSettings(opts SetVersionOptions) []string {
	settings := make([]string, 0, 2)
	if strings.TrimSpace(opts.Version) != "" {
		settings = append(settings, marketingVersionSetting)
	}
	if strings.TrimSpace(opts.BuildNumber) != "" {
		settings = append(settings, currentProjectSetting)
	}
	return settings
}

func bumpVersionSetting(opts BumpVersionOptions) string {
	if opts.BumpType == BumpBuild {
		return currentProjectSetting
	}
	return marketingVersionSetting
}

func (project *structuredVersionProject) prepareBump(opts BumpVersionOptions) (*BumpVersionResult, SetVersionOptions, error) {
	result := &BumpVersionResult{
		BumpType:      string(opts.BumpType),
		ProjectDir:    project.rootDir,
		Target:        strings.TrimSpace(opts.Target),
		Configuration: strings.TrimSpace(opts.Configuration),
	}
	setOptions := SetVersionOptions{
		ProjectDir:            opts.ProjectDir,
		Target:                opts.Target,
		Configuration:         opts.Configuration,
		AllowExternalXCConfig: opts.AllowExternalXCConfig,
	}
	if opts.BumpType == BumpBuild {
		currentBuild, err := project.bumpBaseline(opts, currentProjectSetting, true)
		if err != nil {
			return nil, SetVersionOptions{}, err
		}
		result.OldBuild = currentBuild
		newBuild := strings.TrimSpace(opts.BuildNumber)
		if newBuild == "" {
			newBuild, err = incrementBuildString(currentBuild)
			if err != nil {
				return nil, SetVersionOptions{}, fmt.Errorf("failed to increment build number: %w", err)
			}
		}
		setOptions.BuildNumber = newBuild
		result.NewBuild = newBuild
		return result, setOptions, nil
	}

	currentVersion, err := project.bumpBaseline(opts, marketingVersionSetting, true)
	if err != nil {
		return nil, SetVersionOptions{}, err
	}
	result.OldVersion = currentVersion
	newVersion, err := bumpVersionString(currentVersion, opts.BumpType)
	if err != nil {
		return nil, SetVersionOptions{}, err
	}
	setOptions.Version = newVersion
	result.NewVersion = newVersion
	return result, setOptions, nil
}

func bumpVersionLegacy(ctx context.Context, opts BumpVersionOptions) (*BumpVersionResult, error) {
	if err := requireMacOS(); err != nil {
		return nil, err
	}
	if err := requireAgvtool(ctx); err != nil {
		return nil, err
	}
	trimmedTarget := strings.TrimSpace(opts.Target)

	current, err := getVersionLegacy(ctx, opts.ProjectDir, trimmedTarget, opts.BuildSettingsSession)
	if err != nil {
		return nil, err
	}

	result := &BumpVersionResult{
		BumpType:   string(opts.BumpType),
		ProjectDir: resolvedProjectDir(opts.ProjectDir),
	}

	if opts.BumpType == BumpBuild {
		result.OldBuild = current.BuildNumber
		if requestedBuild := strings.TrimSpace(opts.BuildNumber); requestedBuild != "" {
			if _, err := runAgvtool(ctx, opts.ProjectDir, "new-version", "-all", requestedBuild); err != nil {
				return nil, fmt.Errorf("failed to set build number: %w", err)
			}
			result.NewBuild = requestedBuild
			return result, nil
		}
		if _, err := runAgvtool(ctx, opts.ProjectDir, "next-version", "-all"); err != nil {
			return nil, fmt.Errorf("failed to increment build number: %w", err)
		}
		opts.BuildSettingsSession.invalidateCache()
		updated, err := getVersionLegacy(ctx, opts.ProjectDir, trimmedTarget, opts.BuildSettingsSession)
		if err != nil {
			return nil, fmt.Errorf("failed to read updated build number: %w", err)
		}
		result.NewBuild = updated.BuildNumber
		return result, nil
	}

	// Version bump (major/minor/patch).
	result.OldVersion = current.Version
	newVersion, err := bumpVersionString(current.Version, opts.BumpType)
	if err != nil {
		return nil, err
	}

	if _, err := runAgvtool(ctx, opts.ProjectDir, "new-marketing-version", newVersion); err != nil {
		return nil, fmt.Errorf("failed to set marketing version: %w", err)
	}
	result.NewVersion = newVersion

	return result, nil
}

func requireMacOS() error {
	if runtimeGOOS != "darwin" {
		return fmt.Errorf("xcode version commands require macOS")
	}
	return nil
}

func requireAgvtool(ctx context.Context) error {
	_, err := trustedXcodeToolPathFn(ctx, "agvtool", nil)
	if err != nil {
		if contextErr := contextError(ctx); contextErr != nil {
			return contextErr
		}
		if !isTrustedXcodeToolUnavailable(err, "agvtool") {
			return fmt.Errorf("resolve agvtool: %w", err)
		}
		return fmt.Errorf("agvtool not found: install Xcode command-line tools")
	}
	return nil
}

func runAgvtool(ctx context.Context, projectDir string, args ...string) (string, error) {
	cmd, err := trustedXcodeCommand(ctx, "agvtool", args, nil)
	if err != nil {
		return "", err
	}
	cmd.Dir = resolvedProjectDir(projectDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runXcodeCommand(cmd); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return "", fmt.Errorf("%w: %s", err, stderrText)
		}
		return "", err
	}

	return stdout.String(), nil
}

// readBuildSettings runs xcodebuild -showBuildSettings and extracts key=value pairs.
// If target is non-empty, scopes to that target for deterministic results in
// multi-target projects.
func readBuildSettings(
	ctx context.Context,
	projectDir string,
	target string,
	lookupSession *BuildSettingsLookupSession,
) (map[string]string, error) {
	xcodeproj, err := findXcodeproj(projectDir)
	if err != nil {
		return nil, err
	}

	args := []string{"-showBuildSettings", "-project", filepath.Base(xcodeproj)}
	if t := strings.TrimSpace(target); t != "" {
		args = append(args, "-target", t)
	}
	if lookupSession == nil {
		return nil, fmt.Errorf("build settings lookup session is required")
	}
	if lookupSession.policy == BuildSettingsLookupNever {
		return nil, fmt.Errorf("xcodebuild -showBuildSettings fallback is disabled by --xcodebuild-settings-lookup never; define MARKETING_VERSION and CURRENT_PROJECT_VERSION in the project or referenced xcconfig files")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cacheKey := filepath.Clean(xcodeproj) + "\x00" + strings.TrimSpace(target)
	if cached, ok := lookupSession.cache[cacheKey]; ok {
		return cached, nil
	}
	if !lookupSession.warned {
		if lookupSession.diagnostic != nil {
			fmt.Fprintln(lookupSession.diagnostic, "Warning: structured project parsing could not resolve MARKETING_VERSION and CURRENT_PROJECT_VERSION; running xcodebuild -showBuildSettings. Define both settings in the project or referenced xcconfig files, or pass --xcodebuild-settings-lookup never to fail without running xcodebuild.")
		}
		lookupSession.warned = true
	}
	cmd, err := trustedXcodeCommand(ctx, "xcodebuild", args, nil)
	if err != nil {
		return nil, err
	}
	cmd.Dir = resolvedProjectDir(projectDir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := runXcodeCommand(cmd); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return nil, fmt.Errorf("%w: %s", err, stderrText)
		}
		return nil, err
	}

	buildSettingsOutput := stdout.String()
	if strings.TrimSpace(target) == "" {
		targets := buildSettingsTargetNames(buildSettingsOutput)
		if len(targets) > 1 {
			return nil, fmt.Errorf("multiple Xcode targets found in build settings (%s); use --target", strings.Join(targets, ", "))
		}
	}

	settings := make(map[string]string)
	for _, line := range strings.Split(buildSettingsOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if idx := strings.Index(trimmed, " = "); idx > 0 {
			key := strings.TrimSpace(trimmed[:idx])
			value := strings.TrimSpace(trimmed[idx+3:])
			// Keep the first occurrence only — in multi-target projects,
			// the first target block is typically the main app target.
			if _, exists := settings[key]; !exists {
				settings[key] = value
			}
		}
	}
	lookupSession.cache[cacheKey] = settings
	return settings, nil
}

func buildSettingsTargetNames(output string) []string {
	seen := make(map[string]struct{})
	var targets []string
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "Build settings for action ") || !strings.HasSuffix(trimmed, ":") {
			continue
		}
		idx := strings.LastIndex(trimmed, " target ")
		if idx < 0 {
			continue
		}
		target := strings.TrimSpace(strings.TrimSuffix(trimmed[idx+len(" target "):], ":"))
		if target == "" {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

// findXcodeproj resolves an explicit .xcodeproj path or finds one in a project dir.
// Returns an error if zero or multiple .xcodeproj directories are found.
func findXcodeproj(projectDir string) (string, error) {
	if projectDir == "" {
		projectDir = "."
	}
	projectDir = trimTrailingPathSeparators(projectDir)
	if strings.HasSuffix(projectDir, ".xcodeproj") {
		info, err := os.Stat(projectDir)
		if err != nil {
			return "", fmt.Errorf("failed to read Xcode project %s: %w", projectDir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%s is not an .xcodeproj directory", projectDir)
		}
		resolved, err := resolveProjectParentTraversal(projectDir)
		if err != nil {
			return "", fmt.Errorf("resolve Xcode project path %s: %w", projectDir, err)
		}
		return resolved, nil
	}

	resolvedProjectDir, err := resolveProjectParentTraversal(projectDir)
	if err != nil {
		return "", fmt.Errorf("resolve Xcode project directory %s: %w", projectDir, err)
	}
	projectDir = resolvedProjectDir
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("failed to read project directory: %w", err)
	}
	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasSuffix(entry.Name(), ".xcodeproj") {
			matches = append(matches, entry.Name())
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no .xcodeproj found in %s", projectDir)
	case 1:
		return filepath.Join(projectDir, matches[0]), nil
	default:
		return "", fmt.Errorf("multiple .xcodeproj found in %s (%s); use --project to pick one", projectDir, strings.Join(matches, ", "))
	}
}

func resolveProjectParentTraversal(path string) (string, error) {
	slashPath := filepath.ToSlash(path)
	components := strings.Split(slashPath, "/")
	lastParent := -1
	for index, component := range components {
		if component == ".." {
			lastParent = index
		}
	}
	if lastParent == -1 {
		return path, nil
	}

	prefix := filepath.FromSlash(strings.Join(components[:lastParent+1], "/"))
	resolvedPrefix, err := filepath.EvalSymlinks(prefix)
	if err != nil {
		return "", err
	}
	suffix := filepath.FromSlash(strings.Join(components[lastParent+1:], "/"))
	if suffix == "" {
		return resolvedPrefix, nil
	}
	return filepath.Join(resolvedPrefix, suffix), nil
}

func trimTrailingPathSeparators(path string) string {
	minimumLength := len(filepath.VolumeName(path)) + 1
	for len(path) > minimumLength && os.IsPathSeparator(path[len(path)-1]) {
		path = path[:len(path)-1]
	}
	return path
}

// isVariableReference checks if a value is an Xcode variable like $(MARKETING_VERSION).
func isVariableReference(value string) bool {
	return strings.Contains(value, "$(")
}

// parseAgvtoolVersionOutput extracts the version from agvtool output.
// `agvtool what-marketing-version -terse1` outputs lines like "=1.2.3" or "TargetName=1.2.3".
func parseAgvtoolVersionOutput(output, target string) (string, error) {
	return parseAgvtoolValueOutput(output, target)
}

// parseAgvtoolBuildOutput extracts the build number from agvtool output.
// `agvtool what-version -terse` outputs just the number or target-scoped lines.
func parseAgvtoolBuildOutput(output, target string) (string, error) {
	return parseAgvtoolValueOutput(output, target)
}

func parseAgvtoolValueOutput(output, target string) (string, error) {
	lines := strings.Split(output, "\n")
	trimmedTarget := strings.TrimSpace(target)

	var fallbackValues []string
	seenTargets := make(map[string]struct{})
	valuesByTarget := make(map[string][]string)
	var targetNames []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, "="); idx >= 0 {
			name := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			if name != "" {
				if _, exists := seenTargets[name]; !exists {
					seenTargets[name] = struct{}{}
					targetNames = append(targetNames, name)
				}
				valuesByTarget[name] = append(valuesByTarget[name], value)
				continue
			}
			fallbackValues = append(fallbackValues, value)
			continue
		}
		fallbackValues = append(fallbackValues, line)
	}

	if trimmedTarget != "" {
		if values, ok := valuesByTarget[trimmedTarget]; ok {
			return consistentAgvtoolValue(values, fmt.Sprintf("target %q", trimmedTarget))
		}
		if len(targetNames) > 0 {
			return "", fmt.Errorf("target %q not found in agvtool output", trimmedTarget)
		}
		return consistentAgvtoolValue(fallbackValues, fmt.Sprintf("target %q", trimmedTarget))
	}

	if len(targetNames) > 1 {
		return "", fmt.Errorf("multiple target values found (%s); use --target", strings.Join(targetNames, ", "))
	}
	if len(targetNames) == 1 {
		return consistentAgvtoolValue(valuesByTarget[targetNames[0]], fmt.Sprintf("target %q", targetNames[0]))
	}

	return consistentAgvtoolValue(fallbackValues, "selected project")
}

func consistentAgvtoolValue(values []string, scope string) (string, error) {
	var first string
	seen := make(map[string]struct{})
	var distinct []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		distinct = append(distinct, value)
		if first == "" {
			first = value
		}
	}
	if len(distinct) > 1 {
		return "", fmt.Errorf("agvtool reported differing values for %s (%s)", scope, strings.Join(distinct, ", "))
	}
	return first, nil
}

// bumpVersionString increments a semver-style version string.
func bumpVersionString(current string, bumpType BumpType) (string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		return "", fmt.Errorf("current version is empty")
	}

	parts := strings.Split(current, ".")
	components := make([]int, len(parts))
	for i, p := range parts {
		val, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return "", fmt.Errorf("version %q is not a valid numeric version", current)
		}
		components[i] = val
	}

	switch bumpType {
	case BumpMajor:
		if len(components) < 1 {
			return "", fmt.Errorf("version %q has no major component", current)
		}
		components[0]++
		for i := 1; i < len(components); i++ {
			components[i] = 0
		}
	case BumpMinor:
		if len(components) < 2 {
			return "", fmt.Errorf("version %q needs at least major.minor format for minor bump", current)
		}
		components[1]++
		for i := 2; i < len(components); i++ {
			components[i] = 0
		}
	case BumpPatch:
		if len(components) < 3 {
			return "", fmt.Errorf("version %q needs major.minor.patch format for patch bump", current)
		}
		components[2]++
	default:
		return "", fmt.Errorf("unsupported bump type %q for version bump", bumpType)
	}

	result := make([]string, len(components))
	for i, v := range components {
		result[i] = strconv.Itoa(v)
	}
	return strings.Join(result, "."), nil
}

// incrementBuildString increments a numeric build string by 1.
func incrementBuildString(current string) (string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		return "", fmt.Errorf("build number is empty")
	}

	// Support dotted build numbers (e.g. 1.2.3 → 1.2.4).
	parts := strings.Split(current, ".")
	last := parts[len(parts)-1]
	val, err := strconv.Atoi(last)
	if err != nil {
		return "", fmt.Errorf("build number %q is not numeric", current)
	}
	parts[len(parts)-1] = strconv.Itoa(val + 1)
	return strings.Join(parts, "."), nil
}
