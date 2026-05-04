package publish

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const defaultPublishExportOptionsPath = ".asc/export-options-app-store.plist"

var (
	runPublishArchiveFn   = localxcode.Archive
	runPublishExportFn    = localxcode.Export
	getPublishASCClientFn = func(timeout time.Duration) (*asc.Client, error) {
		if timeout > 0 {
			return shared.GetASCClientWithTimeout(timeout)
		}
		return shared.GetASCClient()
	}
	validatePublishIPAPathFn        = shared.ValidateIPAPath
	resolvePublishNextBuildNumberFn = func(ctx context.Context, client *asc.Client, opts shared.NextBuildNumberOptions) (*asc.BuildsNextBuildNumberResult, error) {
		return shared.ResolveNextBuildNumber(ctx, client, opts)
	}
	uploadBuildAndWaitForIDFn       = uploadBuildAndWaitForID
	resolvePublishAppIDWithLookupFn = func(ctx context.Context, client *asc.Client, appID string) (string, error) {
		return shared.ResolveAppIDWithLookup(ctx, client, appID)
	}
	waitForPublishBuildProcessingFn = func(ctx context.Context, client *asc.Client, buildID string, pollInterval time.Duration) (*asc.BuildResponse, error) {
		return client.WaitForBuildProcessing(ctx, buildID, pollInterval)
	}
	applyPublishVersionMetadataFn = applyPublishVersionMetadata
)

type publishLocalBuildFlagValues struct {
	workspacePath        *string
	projectPath          *string
	scheme               *string
	configuration        *string
	exportOptionsPath    *string
	archivePath          *string
	ipaPath              *string
	clean                *bool
	initialBuildNumber   *int
	archiveXcodebuildArg shared.MultiStringFlag
	exportXcodebuildArg  shared.MultiStringFlag
}

type publishLocalBuildConfig struct {
	WorkspacePath         string
	ProjectPath           string
	Scheme                string
	Configuration         string
	ExportOptionsPath     string
	ArchivePath           string
	IPAPath               string
	Clean                 bool
	ArchiveXcodebuildArgs []string
	ExportXcodebuildArgs  []string
}

type publishLocalBuildExecutionResult struct {
	Archive     *asc.PublishArchiveStageResult
	Export      *asc.PublishExportStageResult
	Build       *asc.BuildResponse
	Version     string
	BuildNumber string
	Uploaded    bool
}

func bindPublishLocalBuildFlags(fs *flag.FlagSet) *publishLocalBuildFlagValues {
	values := &publishLocalBuildFlagValues{}
	values.workspacePath = fs.String("workspace", "", "Path to .xcworkspace directory for local-build mode")
	values.projectPath = fs.String("project", "", "Path to .xcodeproj directory for local-build mode")
	values.scheme = fs.String("scheme", "", "Xcode scheme name for local-build mode")
	values.configuration = fs.String("configuration", "", "Build configuration for local-build mode (defaults to Release)")
	values.exportOptionsPath = fs.String("export-options", "", "Path to ExportOptions.plist for local-build mode")
	values.archivePath = fs.String("archive-path", "", "Destination path for the .xcarchive output in local-build mode")
	values.ipaPath = fs.String("ipa-path", "", "Destination path for the .ipa output in local-build mode")
	values.clean = fs.Bool("clean", false, "Run clean before local-build archive")
	values.initialBuildNumber = fs.Int("initial-build-number", 1, "Initial build number when local-build mode auto-resolves a build number")
	fs.Var(&values.archiveXcodebuildArg, "archive-xcodebuild-flag", "Pass a raw argument through to xcodebuild during local-build archive (repeatable)")
	fs.Var(&values.exportXcodebuildArg, "export-xcodebuild-flag", "Pass a raw argument through to xcodebuild during local-build export (repeatable)")
	return values
}

func (v *publishLocalBuildFlagValues) trimmedWorkspacePath() string {
	if v == nil || v.workspacePath == nil {
		return ""
	}
	return strings.TrimSpace(*v.workspacePath)
}

func (v *publishLocalBuildFlagValues) trimmedProjectPath() string {
	if v == nil || v.projectPath == nil {
		return ""
	}
	return strings.TrimSpace(*v.projectPath)
}

func (v *publishLocalBuildFlagValues) trimmedScheme() string {
	if v == nil || v.scheme == nil {
		return ""
	}
	return strings.TrimSpace(*v.scheme)
}

func (v *publishLocalBuildFlagValues) localBuildMode() bool {
	return v.trimmedWorkspacePath() != "" || v.trimmedProjectPath() != ""
}

func validateLocalBuildFlagUsage(localBuildMode bool, setFlags map[string]bool) error {
	if localBuildMode {
		return nil
	}
	for _, flagName := range []string{
		"archive-path",
		"ipa-path",
		"export-options",
		"configuration",
		"clean",
		"initial-build-number",
		"archive-xcodebuild-flag",
		"export-xcodebuild-flag",
		"scheme",
	} {
		if setFlags[flagName] {
			return shared.UsageErrorf("--%s requires --workspace or --project", flagName)
		}
	}
	return nil
}

func validateLocalBuildSelectors(values *publishLocalBuildFlagValues) error {
	if values == nil {
		return shared.UsageError("exactly one of --workspace or --project is required")
	}
	workspacePath := values.trimmedWorkspacePath()
	projectPath := values.trimmedProjectPath()
	if workspacePath == "" && projectPath == "" {
		return shared.UsageError("exactly one of --workspace or --project is required")
	}
	if workspacePath != "" && projectPath != "" {
		return shared.UsageError("exactly one of --workspace or --project is required")
	}
	if values.trimmedScheme() == "" {
		return shared.UsageError("--scheme is required")
	}
	return nil
}

func resolveLocalBuildConfig(values *publishLocalBuildFlagValues, platform, version, buildNumber string) (publishLocalBuildConfig, error) {
	config := publishLocalBuildConfig{
		WorkspacePath:         values.trimmedWorkspacePath(),
		ProjectPath:           values.trimmedProjectPath(),
		Scheme:                values.trimmedScheme(),
		Configuration:         "Release",
		Clean:                 values.clean != nil && *values.clean,
		ArchiveXcodebuildArgs: append([]string(nil), values.archiveXcodebuildArg...),
		ExportXcodebuildArgs:  append([]string(nil), values.exportXcodebuildArg...),
	}
	if trimmedConfiguration := strings.TrimSpace(*values.configuration); trimmedConfiguration != "" {
		config.Configuration = trimmedConfiguration
	}

	exportOptionsPath, err := resolvePublishExportOptionsPath(strings.TrimSpace(*values.exportOptionsPath))
	if err != nil {
		return publishLocalBuildConfig{}, err
	}
	if localxcode.IsDirectUploadMode(exportOptionsPath) {
		return publishLocalBuildConfig{}, shared.UsageError("--export-options with destination=upload is not supported by publish; use export options that produce a local IPA")
	}
	config.ExportOptionsPath = exportOptionsPath

	config.ArchivePath = strings.TrimSpace(*values.archivePath)
	if config.ArchivePath == "" {
		config.ArchivePath = defaultPublishArchivePath(config.Scheme, platform, version, buildNumber)
	}

	config.IPAPath = strings.TrimSpace(*values.ipaPath)
	if config.IPAPath == "" {
		config.IPAPath = defaultPublishIPAPath(config.Scheme, platform, version, buildNumber)
	}

	return config, nil
}

func resolvePublishBuildNumber(ctx context.Context, client *asc.Client, appID, version, platform string, values *publishLocalBuildFlagValues, explicitBuildNumber string) (string, error) {
	buildNumber := strings.TrimSpace(explicitBuildNumber)
	if buildNumber != "" {
		return buildNumber, nil
	}

	result, err := resolvePublishNextBuildNumberFn(ctx, client, shared.NextBuildNumberOptions{
		LatestBuildSelectionOptions: shared.LatestBuildSelectionOptions{
			AppID:          appID,
			Version:        strings.TrimSpace(version),
			Platform:       strings.TrimSpace(platform),
			ExcludeExpired: false,
		},
		InitialBuildNumber: *values.initialBuildNumber,
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.NextBuildNumber), nil
}

func runPublishLocalBuild(ctx context.Context, client *asc.Client, appID, platform, version, buildNumber string, pollInterval, timeout time.Duration, timeoutOverride bool, config publishLocalBuildConfig) (*publishLocalBuildExecutionResult, error) {
	archiveResult, err := runPublishArchiveFn(ctx, localxcode.ArchiveOptions{
		WorkspacePath:  config.WorkspacePath,
		ProjectPath:    config.ProjectPath,
		Scheme:         config.Scheme,
		Configuration:  config.Configuration,
		ArchivePath:    config.ArchivePath,
		Clean:          config.Clean,
		Overwrite:      true,
		XcodebuildArgs: publishArchiveXcodebuildArgs(platform, version, buildNumber, config.ArchiveXcodebuildArgs),
		LogWriter:      os.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("archive local build: %w", err)
	}

	exportResult, err := runPublishExportFn(ctx, localxcode.ExportOptions{
		ArchivePath:    archiveResult.ArchivePath,
		ExportOptions:  config.ExportOptionsPath,
		IPAPath:        config.IPAPath,
		Overwrite:      true,
		XcodebuildArgs: publishExportXcodebuildArgs(config.ExportXcodebuildArgs),
		LogWriter:      os.Stderr,
	})
	if err != nil {
		return nil, fmt.Errorf("export local build: %w", err)
	}
	result := &publishLocalBuildExecutionResult{
		Archive: &asc.PublishArchiveStageResult{
			ArchivePath:   strings.TrimSpace(archiveResult.ArchivePath),
			BundleID:      firstNonEmpty(strings.TrimSpace(archiveResult.BundleID), strings.TrimSpace(exportResult.BundleID)),
			Version:       firstNonEmpty(strings.TrimSpace(archiveResult.Version), strings.TrimSpace(exportResult.Version), strings.TrimSpace(version)),
			BuildNumber:   firstNonEmpty(strings.TrimSpace(archiveResult.BuildNumber), strings.TrimSpace(exportResult.BuildNumber), strings.TrimSpace(buildNumber)),
			Scheme:        strings.TrimSpace(archiveResult.Scheme),
			Configuration: strings.TrimSpace(archiveResult.Configuration),
		},
		Export: &asc.PublishExportStageResult{
			ArchivePath:       strings.TrimSpace(exportResult.ArchivePath),
			IPAPath:           strings.TrimSpace(exportResult.IPAPath),
			BundleID:          firstNonEmpty(strings.TrimSpace(exportResult.BundleID), strings.TrimSpace(archiveResult.BundleID)),
			Version:           firstNonEmpty(strings.TrimSpace(exportResult.Version), strings.TrimSpace(archiveResult.Version), strings.TrimSpace(version)),
			BuildNumber:       firstNonEmpty(strings.TrimSpace(exportResult.BuildNumber), strings.TrimSpace(archiveResult.BuildNumber), strings.TrimSpace(buildNumber)),
			ExportOptionsPath: config.ExportOptionsPath,
			DirectUpload:      strings.TrimSpace(exportResult.IPAPath) == "",
		},
		Version:     firstNonEmpty(strings.TrimSpace(exportResult.Version), strings.TrimSpace(archiveResult.Version), strings.TrimSpace(version)),
		BuildNumber: firstNonEmpty(strings.TrimSpace(exportResult.BuildNumber), strings.TrimSpace(archiveResult.BuildNumber), strings.TrimSpace(buildNumber)),
	}

	if strings.TrimSpace(exportResult.IPAPath) == "" {
		return nil, fmt.Errorf("export local build: expected a local IPA artifact for publish upload")
	}

	fileInfo, err := validatePublishIPAPathFn(exportResult.IPAPath)
	if err != nil {
		return nil, fmt.Errorf("validate exported IPA: %w", err)
	}
	uploadRequestCtx, cancel := shared.ContextWithTimeoutDuration(ctx, timeout)
	defer cancel()
	uploadResult, err := uploadBuildAndWaitForIDFn(
		uploadRequestCtx,
		client,
		appID,
		exportResult.IPAPath,
		fileInfo,
		result.Version,
		result.BuildNumber,
		asc.Platform(platform),
		pollInterval,
		timeout,
		timeoutOverride,
	)
	if err != nil {
		return nil, err
	}
	result.Build = uploadResult.Build
	result.Version = uploadResult.Version
	result.BuildNumber = uploadResult.BuildNumber
	result.Uploaded = true
	result.Export.Version = uploadResult.Version
	result.Export.BuildNumber = uploadResult.BuildNumber
	result.Archive.Version = firstNonEmpty(result.Archive.Version, uploadResult.Version)
	result.Archive.BuildNumber = firstNonEmpty(result.Archive.BuildNumber, uploadResult.BuildNumber)
	return result, nil
}

func publishArchiveXcodebuildArgs(platform, version, buildNumber string, extra []string) []string {
	args := []string{
		"-destination", defaultPublishArchiveDestination(platform),
		"MARKETING_VERSION=" + strings.TrimSpace(version),
		"CURRENT_PROJECT_VERSION=" + strings.TrimSpace(buildNumber),
		"-allowProvisioningUpdates",
	}
	return append(args, extra...)
}

func publishExportXcodebuildArgs(extra []string) []string {
	args := []string{"-allowProvisioningUpdates"}
	return append(args, extra...)
}

func resolvePublishExportOptionsPath(explicit string) (string, error) {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		return trimmed, nil
	}
	info, err := os.Stat(defaultPublishExportOptionsPath)
	if err == nil && !info.IsDir() {
		return defaultPublishExportOptionsPath, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("stat %s: %w", defaultPublishExportOptionsPath, err)
	}
	return "", shared.UsageError(fmt.Sprintf("--export-options is required in local-build mode when %s is missing", defaultPublishExportOptionsPath))
}

func defaultPublishArchivePath(scheme, platform, version, buildNumber string) string {
	fileName := fmt.Sprintf(
		"%s-%s-%s-%s.xcarchive",
		sanitizePublishArtifactToken(scheme),
		sanitizePublishArtifactToken(platform),
		sanitizePublishArtifactToken(version),
		sanitizePublishArtifactToken(buildNumber),
	)
	return filepath.Join(".asc", "artifacts", fileName)
}

func defaultPublishIPAPath(scheme, platform, version, buildNumber string) string {
	fileName := fmt.Sprintf(
		"%s-%s-%s-%s.ipa",
		sanitizePublishArtifactToken(scheme),
		sanitizePublishArtifactToken(platform),
		sanitizePublishArtifactToken(version),
		sanitizePublishArtifactToken(buildNumber),
	)
	return filepath.Join(".asc", "artifacts", fileName)
}

func defaultPublishArchiveDestination(platform string) string {
	switch strings.ToUpper(strings.TrimSpace(platform)) {
	case "MAC_OS":
		return "generic/platform=macOS"
	case "TV_OS":
		return "generic/platform=tvOS"
	case "VISION_OS":
		return "generic/platform=visionOS"
	default:
		return "generic/platform=iOS"
	}
}

func sanitizePublishArtifactToken(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "artifact"
	}
	var builder strings.Builder
	lastDash := false
	for _, r := range trimmed {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '_':
			builder.WriteRune(r)
			lastDash = false
		case r == '-' || unicode.IsSpace(r) || r == '/' || r == '\\':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	result := strings.Trim(builder.String(), "-")
	if result == "" {
		return "artifact"
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func collectSetFlags(fs *flag.FlagSet) map[string]bool {
	setFlags := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		setFlags[f.Name] = true
	})
	return setFlags
}
