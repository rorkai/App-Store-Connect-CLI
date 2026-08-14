package xcode

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	runGenerateExportOptions      = localxcode.GenerateExportOptions
	runXcodeExportPreflight       = localxcode.PreflightExport
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
	xcodeExportWaitDefaultTimeout = 15 * time.Minute
)

// XcodeCommand returns the local Xcode command group.
func XcodeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "xcode",
		ShortUsage: "asc xcode <subcommand> [flags]",
		ShortHelp:  "Local Xcode build/archive/export helpers (macOS only).",
		LongHelp: `Local Xcode build/archive/export helpers.

These commands wrap local xcodebuild flows and are visible on every platform so
docs and workflows stay consistent, but execution is supported on macOS only.

Use these commands to compile projects and produce deterministic .xcarchive and
.ipa paths that can be passed directly into asc upload and publish commands.

Examples:
  asc xcode inject --manifest .asc/deployment.json --set version=1.3.0 --overwrite
  asc xcode build --project App.xcodeproj --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0' --no-code-signing --output json
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive --output json
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --output json
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode version view
  asc xcode version bump --type patch
  asc xcode version edit --version "1.3.0" --build-number "42"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeInjectCommand(),
			XcodeBuildCommand(),
			XcodeArchiveCommand(),
			XcodeExportCommand(),
			XcodeExportOptionsGroupCommand(),
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
	var xcodebuildFlags shared.MultiStringFlag
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
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*workspacePath) != "" && strings.TrimSpace(*projectPath) != "" {
				fmt.Fprintln(os.Stderr, "Error: exactly one of --workspace or --project is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*scheme) == "" {
				fmt.Fprintln(os.Stderr, "Error: --scheme is required")
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*archivePath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --archive-path is required")
				return shared.MissingRequiredUsageError()
			}
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "configuration"); emptyFlag != "" {
				return shared.UsageErrorf("--%s must not be empty", emptyFlag)
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
	exportOptions := fs.String("export-options", "", "Path to ExportOptions.plist (generated automatically when omitted)")
	method := fs.String("method", "app-store-connect", "[experimental] Method for generated options: app-store-connect or release-testing")
	signingStyle := fs.String("signing-style", "automatic", "Signing style for generated options: automatic or manual")
	teamID := fs.String("team-id", "", "Apple Developer team ID for generated options (overrides archive metadata)")
	ipaPath := fs.String("ipa-path", "", "Destination path for a local .ipa when one is produced (required)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing IPA at --ipa-path")
	wait := fs.Bool("wait", false, "Wait for App Store Connect build discovery and processing when export uploads directly")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait when waiting for uploaded builds")
	timeout := fs.Duration("timeout", 0, "Maximum duration for xcodebuild -exportArchive (0 disables local export timeout)")
	var xcodebuildFlags shared.MultiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass an additional raw argument through to xcodebuild; asc-managed arguments are rejected (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc xcode export [flags]",
		ShortHelp:  "Export an archive to a deterministic IPA path or direct upload.",
		LongHelp: `Export an archive to a deterministic IPA path or direct upload.

This command runs xcodebuild -exportArchive into a temporary directory.
When --export-options is omitted, asc generates archive-adjacent options. It
uses app-store-connect and automatic signing by default. Use
--method release-testing for a local IPA installable on registered devices. Use
--signing-style manual to match locally installed certificates and profiles;
--team-id optionally overrides archive metadata.
When ExportOptions.plist produces a local IPA, asc moves it to --ipa-path.
When ExportOptions.plist uses destination=upload, xcodebuild uploads directly
to App Store Connect and asc returns archive metadata without writing a local
IPA at --ipa-path. Use --wait to poll until the uploaded build appears and
finishes processing.

Examples:
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --method release-testing --signing-style manual
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --signing-style manual --team-id TEAM_ID
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --timeout 10m
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
				return shared.MissingRequiredUsageError()
			}
			if strings.TrimSpace(*ipaPath) == "" {
				fmt.Fprintln(os.Stderr, "Error: --ipa-path is required")
				return shared.MissingRequiredUsageError()
			}
			if *wait && *pollInterval <= 0 {
				return shared.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout < 0 {
				return shared.UsageError("--timeout must be zero or greater")
			}
			if err := localxcode.ValidateExportXcodebuildArgs([]string(xcodebuildFlags)); err != nil {
				return shared.UsageError(err.Error())
			}
			generationFlagsSet := false
			methodSet := false
			signingStyleSet := false
			teamIDSet := false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "method":
					generationFlagsSet = true
					methodSet = true
				case "signing-style":
					generationFlagsSet = true
					signingStyleSet = true
				case "team-id":
					generationFlagsSet = true
					teamIDSet = true
				}
			})
			if methodSet && strings.TrimSpace(*method) == "" {
				return shared.UsageError("--method must be one of: app-store-connect, release-testing")
			}
			methodValue, err := localxcode.NormalizeExportOptionsMethod(*method)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if signingStyleSet && strings.TrimSpace(*signingStyle) == "" {
				return shared.UsageError("--signing-style must be one of: automatic, manual")
			}
			signingStyleValue, err := localxcode.NormalizeExportOptionsSigningStyle(*signingStyle)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			teamIDValue := strings.TrimSpace(*teamID)
			if teamIDSet && teamIDValue == "" {
				return shared.UsageError("--team-id must not be empty")
			}
			trimmedArchivePath := strings.TrimSpace(*archivePath)
			exportOptionsPath := strings.TrimSpace(*exportOptions)
			if exportOptionsPath != "" && generationFlagsSet {
				return shared.UsageError("--export-options cannot be combined with --method, --signing-style, or --team-id")
			}
			if *wait && methodValue == "release-testing" {
				return shared.UsageError("--wait cannot be combined with --method release-testing")
			}
			if exportOptionsPath == "" {
				destination := "export"
				if *wait {
					destination = "upload"
				}
				if err := localxcode.ValidateExportDestination(strings.TrimSpace(*ipaPath), *overwrite, destination == "upload"); err != nil {
					if localxcode.IsExportDestinationUsageError(err) {
						return shared.UsageError(err.Error())
					}
					return fmt.Errorf("xcode export: %w", err)
				}
				if err := runXcodeExportPreflight(ctx); err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if err := localxcode.PreflightExportDestination(strings.TrimSpace(*ipaPath), *overwrite, destination == "upload"); err != nil {
					if localxcode.IsExportDestinationUsageError(err) {
						return shared.UsageError(err.Error())
					}
					return fmt.Errorf("xcode export: %w", err)
				}
				generatedPath, err := localxcode.UniqueExportOptionsPathForArchive(trimmedArchivePath)
				if err != nil {
					return fmt.Errorf("xcode export: select export options path: %w", err)
				}
				generated, err := runGenerateExportOptions(ctx, localxcode.ExportOptionsGenerateOptions{
					ArchivePath:  trimmedArchivePath,
					OutputPath:   generatedPath,
					Method:       methodValue,
					Destination:  destination,
					SigningStyle: signingStyleValue,
					TeamID:       teamIDValue,
					Overwrite:    false,
				})
				if err != nil {
					return fmt.Errorf("xcode export: generate export options: %w", err)
				}
				if generated == nil || strings.TrimSpace(generated.Path) == "" {
					return fmt.Errorf("xcode export: generate export options: generator returned an empty path")
				}
				exportOptionsPath = strings.TrimSpace(generated.Path)
			}
			if *wait && !isDirectUploadExportOptionsFn(exportOptionsPath) {
				return shared.UsageError("--wait requires ExportOptions.plist with destination=upload")
			}

			exportCtx := ctx
			var exportCancel context.CancelFunc
			if *timeout > 0 {
				exportCtx, exportCancel = context.WithTimeout(ctx, *timeout)
			}
			if exportCancel != nil {
				defer exportCancel()
			}

			exportStartedAt := time.Now()
			result, err := runExport(exportCtx, localxcode.ExportOptions{
				ArchivePath:    trimmedArchivePath,
				ExportOptions:  exportOptionsPath,
				IPAPath:        strings.TrimSpace(*ipaPath),
				Overwrite:      *overwrite,
				XcodebuildArgs: []string(xcodebuildFlags),
				LogWriter:      os.Stderr,
			})
			if err != nil {
				if *timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("xcode export: timed out after %s while running xcodebuild -exportArchive: %w", timeout.String(), err)
				}
				return fmt.Errorf("xcode export: %w", err)
			}
			exportCompletedAt := time.Now()
			if exportCompletedAt.Before(exportStartedAt) {
				exportCompletedAt = exportStartedAt
			}
			commandResult := xcodeExportCommandResult{
				ArchivePath:       result.ArchivePath,
				ExportOptionsPath: exportOptionsPath,
				IPAPath:           result.IPAPath,
				BundleID:          result.BundleID,
				Version:           result.Version,
				BuildNumber:       result.BuildNumber,
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
			trimmedIPAPath := strings.TrimSpace(*ipaPath)
			trimmedAPIKey := strings.TrimSpace(*apiKey)
			trimmedAPIIssuer := strings.TrimSpace(*apiIssuer)

			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode validate does not accept positional arguments")
				return flag.ErrHelp
			}
			if trimmedIPAPath == "" {
				fmt.Fprintln(os.Stderr, "Error: --ipa is required")
				return shared.MissingRequiredUsageError()
			}
			if !strings.EqualFold(filepath.Ext(trimmedIPAPath), ".ipa") {
				return shared.UsageError("--ipa must end with .ipa")
			}
			if (trimmedAPIKey == "") != (trimmedAPIIssuer == "") {
				return shared.UsageError("--api-key and --api-issuer must be provided together")
			}

			result, err := runValidate(ctx, localxcode.ValidateOptions{
				IPAPath:   trimmedIPAPath,
				APIKey:    trimmedAPIKey,
				APIIssuer: trimmedAPIIssuer,
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

// firstExplicitlyEmptyFlag reports the first of names that the caller supplied
// with a blank value, in the order given. Commands use it to reject values they
// would otherwise silently ignore, because a supplied-but-empty flag (typically
// an unset CI variable) is never the same request as an omitted flag.
func firstExplicitlyEmptyFlag(fs *flag.FlagSet, names ...string) string {
	empty := make(map[string]struct{}, len(names))
	fs.Visit(func(f *flag.Flag) {
		if strings.TrimSpace(f.Value.String()) == "" {
			empty[f.Name] = struct{}{}
		}
	})
	for _, name := range names {
		if _, ok := empty[name]; ok {
			return name
		}
	}
	return ""
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
	ArchivePath       string `json:"archive_path"`
	ExportOptionsPath string `json:"export_options_path"`
	IPAPath           string `json:"ipa_path"`
	BundleID          string `json:"bundle_id,omitempty"`
	Version           string `json:"version,omitempty"`
	BuildNumber       string `json:"build_number,omitempty"`
	BuildID           string `json:"build_id,omitempty"`
	ProcessingState   string `json:"processing_state,omitempty"`
}

func exportResultRows(result xcodeExportCommandResult) [][]string {
	rows := [][]string{
		{"archive_path", result.ArchivePath},
		{"export_options_path", result.ExportOptionsPath},
	}
	if result.IPAPath != "" {
		rows = append(rows, []string{"ipa_path", result.IPAPath})
	} else {
		rows = append(rows, []string{"ipa_path", "(direct upload — no local artifact)"})
	}
	rows = append(
		rows,
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
