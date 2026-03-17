package xcode

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var (
	runArchive                    = localxcode.Archive
	runExport                     = localxcode.Export
	runValidate                   = localxcode.Validate
	isDirectUploadExportOptionsFn = localxcode.IsDirectUploadMode
	inferArchivePlatformFn        = localxcode.InferArchivePlatform
	getASCClientFn                = shared.GetASCClient
	resolveAppIDWithExactLookupFn = func(ctx context.Context, client *asc.Client, appID string) (string, error) {
		return shared.ResolveAppIDWithExactLookup(ctx, client, appID)
	}
	waitForBuildByNumberOrUploadFailureFn = shared.WaitForBuildByNumberOrUploadFailure
	resolveBuildUploadIDFn                = func(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time, pollInterval time.Duration) (string, error) {
		return waitForBuildUploadID(ctx, client, appID, version, buildNumber, platform, exportStartedAt, exportCompletedAt, pollInterval)
	}
	waitForBuildProcessingFn = func(ctx context.Context, client *asc.Client, buildID string, pollInterval time.Duration) (*asc.BuildResponse, error) {
		return client.WaitForBuildProcessing(ctx, buildID, pollInterval)
	}
	resolveXcodeExportWaitTimeoutFn = func() time.Duration {
		return asc.ResolveTimeoutWithDefault(xcodeExportWaitDefaultTimeout)
	}
)

const (
	xcodeExportWaitDefaultTimeout     = 15 * time.Minute
	xcodeExportBuildUploadLookupLimit = 200
)

type multiStringFlag []string

func (m *multiStringFlag) String() string {
	if m == nil {
		return ""
	}
	return strings.Join(*m, ",")
}

func (m *multiStringFlag) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("value cannot be empty")
	}
	*m = append(*m, trimmed)
	return nil
}

// XcodeCommand returns the local Xcode command group.
func XcodeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "xcode",
		ShortUsage: "asc xcode <subcommand> [flags]",
		ShortHelp:  "Local Xcode archive/export helpers (macOS only).",
		LongHelp: `Local Xcode archive/export helpers.

These commands wrap local xcodebuild flows and are visible on every platform so
docs and workflows stay consistent, but execution is supported on macOS only.

Use these commands to produce deterministic .xcarchive and .ipa paths that can
be passed directly into asc upload and publish commands.

Examples:
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive --output json
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa --output json
  asc xcode version view
  asc xcode version bump --type patch
  asc xcode version edit --version "1.3.0" --build-number "42"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeArchiveCommand(),
			XcodeExportCommand(),
			XcodeValidateCommand(),
			XcodeVersionCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeArchiveCommand returns the local archive command.
func XcodeArchiveCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode archive", flag.ExitOnError)

	workspacePath := fs.String("workspace", "", "Path to .xcworkspace directory")
	projectPath := fs.String("project", "", "Path to .xcodeproj directory")
	scheme := fs.String("scheme", "", "Xcode scheme name (required)")
	configuration := fs.String("configuration", "", "Build configuration (for example Release)")
	archivePath := fs.String("archive-path", "", "Destination path for the .xcarchive output (required)")
	clean := fs.Bool("clean", false, "Run clean before archive")
	overwrite := fs.Bool("overwrite", false, "Replace an existing archive at --archive-path")
	var xcodebuildFlags multiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "archive",
		ShortUsage: "asc xcode archive [flags]",
		ShortHelp:  "Create an .xcarchive at an exact path.",
		LongHelp: `Create an .xcarchive at an exact path.

Provide exactly one of --workspace or --project, plus --scheme and
--archive-path.

Examples:
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive
  asc xcode archive --project App.xcodeproj --scheme App --configuration Release --clean --archive-path .asc/artifacts/App.xcarchive
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive --xcodebuild-flag=-destination --xcodebuild-flag=generic/platform=iOS --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode archive does not accept positional arguments")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*workspacePath) == "" && strings.TrimSpace(*projectPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: exactly one of --workspace or --project is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*workspacePath) != "" && strings.TrimSpace(*projectPath) != "" {
				fmt.Fprintln(os.Stderr, "Error: exactly one of --workspace or --project is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*scheme) == "" {
				fmt.Fprintln(os.Stderr, "Error: --scheme is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*archivePath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --archive-path is required")
				return flag.ErrHelp
			}

			result, err := runArchive(ctx, localxcode.ArchiveOptions{
				WorkspacePath:  strings.TrimSpace(*workspacePath),
				ProjectPath:    strings.TrimSpace(*projectPath),
				Scheme:         strings.TrimSpace(*scheme),
				Configuration:  strings.TrimSpace(*configuration),
				ArchivePath:    strings.TrimSpace(*archivePath),
				Clean:          *clean,
				Overwrite:      *overwrite,
				XcodebuildArgs: []string(xcodebuildFlags),
				LogWriter:      os.Stderr,
			})
			if err != nil {
				return fmt.Errorf("xcode archive: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, archiveResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, archiveResultRows(result))
					return nil
				},
			)
		},
	}
}

// XcodeExportCommand returns the local export command.
func XcodeExportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode export", flag.ExitOnError)

	archivePath := fs.String("archive-path", "", "Path to the .xcarchive input (required)")
	exportOptions := fs.String("export-options", "", "Path to ExportOptions.plist (required)")
	ipaPath := fs.String("ipa-path", "", "Destination path for a local .ipa when one is produced (required)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing IPA at --ipa-path")
	wait := fs.Bool("wait", false, "Wait for App Store Connect build discovery and processing when export uploads directly")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait when waiting for uploaded builds")
	var xcodebuildFlags multiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc xcode export [flags]",
		ShortHelp:  "Export an archive to a deterministic IPA path or direct upload.",
		LongHelp: `Export an archive to a deterministic IPA path or direct upload.

This command runs xcodebuild -exportArchive into a temporary directory.
When ExportOptions.plist produces a local IPA, asc moves it to --ipa-path.
When ExportOptions.plist uses destination=upload, xcodebuild uploads directly
to App Store Connect and asc returns archive metadata without writing a local
IPA at --ipa-path. Use --wait to poll until the uploaded build appears and
finishes processing.

Examples:
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options UploadExportOptions.plist --ipa-path .asc/artifacts/App.ipa --wait
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa --xcodebuild-flag=-allowProvisioningUpdates --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode export does not accept positional arguments")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*archivePath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --archive-path is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*exportOptions) == "" {
				fmt.Fprintln(os.Stderr, "Error: --export-options is required")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*ipaPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --ipa-path is required")
				return flag.ErrHelp
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			exportOptionsPath := strings.TrimSpace(*exportOptions)
			if *wait && !isDirectUploadExportOptionsFn(exportOptionsPath) {
				return shared.UsageError("--wait requires ExportOptions.plist with destination=upload")
			}

			exportStartedAt := time.Now()
			result, err := runExport(ctx, localxcode.ExportOptions{
				ArchivePath:    strings.TrimSpace(*archivePath),
				ExportOptions:  exportOptionsPath,
				IPAPath:        strings.TrimSpace(*ipaPath),
				Overwrite:      *overwrite,
				XcodebuildArgs: []string(xcodebuildFlags),
				LogWriter:      os.Stderr,
			})
			if err != nil {
				return fmt.Errorf("xcode export: %w", err)
			}
			exportCompletedAt := time.Now()
			if exportCompletedAt.Before(exportStartedAt) {
				exportCompletedAt = exportStartedAt
			}
			commandResult := xcodeExportCommandResult{
				ArchivePath: result.ArchivePath,
				IPAPath:     result.IPAPath,
				BundleID:    result.BundleID,
				Version:     result.Version,
				BuildNumber: result.BuildNumber,
			}
			if *wait {
				if strings.TrimSpace(result.BundleID) == "" {
					return fmt.Errorf("xcode export: exported archive is missing bundle ID required for --wait")
				}
				if strings.TrimSpace(result.Version) == "" || strings.TrimSpace(result.BuildNumber) == "" {
					return fmt.Errorf("xcode export: exported archive is missing version/build metadata required for --wait")
				}
				client, err := getASCClientFn()
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				waitCtx, cancel := shared.ContextWithTimeoutDuration(ctx, resolveXcodeExportWaitTimeoutFn())
				defer cancel()
				appID, err := resolveAppIDWithExactLookupFn(waitCtx, client, result.BundleID)
				if err != nil {
					return fmt.Errorf("xcode export: resolve app for exported bundle ID %q: %w", result.BundleID, err)
				}
				platform, err := inferArchivePlatformFn(result.ArchivePath)
				if err != nil {
					return fmt.Errorf("xcode export: infer archive platform for --wait: %w", err)
				}
				uploadID, err := resolveBuildUploadIDFn(waitCtx, client, appID, result.Version, result.BuildNumber, platform, exportStartedAt, exportCompletedAt, *pollInterval)
				if err != nil {
					return fmt.Errorf("xcode export: resolve build upload for --wait: %w", err)
				}
				if strings.TrimSpace(uploadID) == "" {
					return fmt.Errorf("xcode export: failed to resolve build upload for version %q build %q", result.Version, result.BuildNumber)
				}
				fmt.Fprintf(os.Stderr, "Waiting for build %s (%s) to appear in App Store Connect...\n", result.BuildNumber, result.Version)
				buildResp, err := waitForBuildByNumberOrUploadFailureFn(waitCtx, client, appID, uploadID, result.Version, result.BuildNumber, platform, *pollInterval)
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if buildResp == nil {
					return fmt.Errorf("xcode export: failed to resolve build for version %q build %q", result.Version, result.BuildNumber)
				}
				discoveredBuildID := strings.TrimSpace(buildResp.Data.ID)
				fmt.Fprintf(os.Stderr, "Build %s discovered; waiting for processing...\n", discoveredBuildID)
				buildResp, err = waitForBuildProcessingFn(waitCtx, client, discoveredBuildID, *pollInterval)
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if buildResp == nil {
					return fmt.Errorf("xcode export: failed to resolve processed build state for build %q", discoveredBuildID)
				}
				commandResult.BuildID = strings.TrimSpace(buildResp.Data.ID)
				commandResult.ProcessingState = strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
			}

			return shared.PrintOutputWithRenderers(
				commandResult,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, exportResultRows(commandResult))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, exportResultRows(commandResult))
					return nil
				},
			)
		},
	}
}

// XcodeValidateCommand returns the local validate command.
func XcodeValidateCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode validate", flag.ExitOnError)

	ipaPath := fs.String("ipa", "", "Path to the .ipa input (required)")
	apiKey := fs.String("api-key", "", "App Store Connect API key ID for altool")
	apiIssuer := fs.String("api-issuer", "", "App Store Connect API issuer ID for altool")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc xcode validate [flags]",
		ShortHelp:  "Validate an IPA with Apple before upload.",
		LongHelp: `Validate an IPA with Apple before upload.

This command wraps xcrun altool --validate-app to check whether an IPA passes
Apple's server-side validation before you upload or submit it.

Examples:
  asc xcode validate --ipa .asc/artifacts/App.ipa
  asc xcode validate --ipa .asc/artifacts/App.ipa --api-key KEY123ABC --api-issuer 00000000-0000-0000-0000-000000000000 --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode validate does not accept positional arguments")
				return flag.ErrHelp
			}
			if strings.TrimSpace(*ipaPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --ipa is required")
				return flag.ErrHelp
			}

			result, err := runValidate(ctx, localxcode.ValidateOptions{
				IPAPath:   strings.TrimSpace(*ipaPath),
				APIKey:    strings.TrimSpace(*apiKey),
				APIIssuer: strings.TrimSpace(*apiIssuer),
				LogWriter: os.Stderr,
			})
			if err != nil {
				return fmt.Errorf("xcode validate: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, validateResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, validateResultRows(result))
					return nil
				},
			)
		},
	}
}

func waitForBuildUploadID(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time, pollInterval time.Duration) (string, error) {
	if client == nil {
		return "", fmt.Errorf("client is required")
	}
	if pollInterval <= 0 {
		pollInterval = shared.PublishDefaultPollInterval
	}

	return asc.PollUntil(ctx, pollInterval, func(ctx context.Context) (string, bool, error) {
		return findRecentBuildUploadID(ctx, client, appID, version, buildNumber, platform, exportStartedAt, exportCompletedAt)
	})
}

func findRecentBuildUploadID(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time) (string, bool, error) {
	if !exportStartedAt.IsZero() && !exportCompletedAt.IsZero() && exportCompletedAt.Before(exportStartedAt) {
		exportCompletedAt = exportStartedAt
	}
	resp, err := client.GetBuildUploads(ctx, appID,
		asc.WithBuildUploadsCFBundleShortVersionStrings([]string{version}),
		asc.WithBuildUploadsCFBundleVersions([]string{buildNumber}),
		asc.WithBuildUploadsPlatforms([]string{platform}),
		asc.WithBuildUploadsSort("-uploadedDate"),
		asc.WithBuildUploadsLimit(xcodeExportBuildUploadLookupLimit),
	)
	if err != nil {
		return "", false, err
	}

	for {
		for _, upload := range resp.Data {
			associationAt, hasAssociationAt := buildUploadAssociationTime(upload.Attributes)
			if !hasAssociationAt {
				if !exportStartedAt.IsZero() {
					continue
				}
				return strings.TrimSpace(upload.ID), true, nil
			}
			if !exportCompletedAt.IsZero() && associationAt.After(exportCompletedAt) {
				continue
			}
			if !exportStartedAt.IsZero() && associationAt.Before(exportStartedAt) {
				continue
			}
			return strings.TrimSpace(upload.ID), true, nil
		}

		nextURL := strings.TrimSpace(resp.Links.Next)
		if nextURL == "" {
			return "", false, nil
		}
		resp, err = client.GetBuildUploads(ctx, appID, asc.WithBuildUploadsNextURL(nextURL))
		if err != nil {
			return "", false, err
		}
	}
}

func buildUploadAssociationTime(attr asc.BuildUploadAttributes) (time.Time, bool) {
	for _, candidate := range []*string{attr.CreatedDate, attr.UploadedDate} {
		parsed, ok := parseBuildUploadTime(candidate)
		if ok {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseBuildUploadTime(value *string) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	candidate := strings.TrimSpace(*value)
	if candidate == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		parsed, err := time.Parse(layout, candidate)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func archiveResultRows(result *localxcode.ArchiveResult) [][]string {
	rows := [][]string{
		{"archive_path", result.ArchivePath},
		{"bundle_id", result.BundleID},
		{"version", result.Version},
		{"build_number", result.BuildNumber},
		{"scheme", result.Scheme},
	}
	if strings.TrimSpace(result.Configuration) != "" {
		rows = append(rows, []string{"configuration", result.Configuration})
	}
	return rows
}

type xcodeExportCommandResult struct {
	ArchivePath     string `json:"archive_path"`
	IPAPath         string `json:"ipa_path"`
	BundleID        string `json:"bundle_id,omitempty"`
	Version         string `json:"version,omitempty"`
	BuildNumber     string `json:"build_number,omitempty"`
	BuildID         string `json:"build_id,omitempty"`
	ProcessingState string `json:"processing_state,omitempty"`
}

func exportResultRows(result xcodeExportCommandResult) [][]string {
	rows := [][]string{
		{"archive_path", result.ArchivePath},
	}
	if result.IPAPath != "" {
		rows = append(rows, []string{"ipa_path", result.IPAPath})
	} else {
		rows = append(rows, []string{"ipa_path", "(direct upload — no local artifact)"})
	}
	rows = append(rows,
		[]string{"bundle_id", result.BundleID},
		[]string{"version", result.Version},
		[]string{"build_number", result.BuildNumber},
	)
	if strings.TrimSpace(result.BuildID) != "" {
		rows = append(rows, []string{"build_id", result.BuildID})
	}
	if strings.TrimSpace(result.ProcessingState) != "" {
		rows = append(rows, []string{"processing_state", result.ProcessingState})
	}
	return rows
}

func validateResultRows(result *localxcode.ValidateResult) [][]string {
	return [][]string{
		{"ipa_path", result.IPAPath},
		{"validated", fmt.Sprintf("%t", result.Validated)},
	}
}
