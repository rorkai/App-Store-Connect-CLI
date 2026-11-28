package xcode

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/config"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/rootfs"
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
	resolveAppIDWithExactLookupFn = func(ctx context.Context, client *asc.Client, appID string) (string, error) {
		return shared.ResolveAppIDWithExactLookup(ctx, client, appID)
	}
	waitForBuildByNumberOrUploadFailureFn = (*shared.Deps).WaitForBuildByNumberOrUploadFailure
	resolveBuildUploadIDFn                = func(ctx context.Context, client *asc.Client, appID, version, buildNumber, platform string, exportStartedAt, exportCompletedAt time.Time, pollInterval time.Duration) (string, error) {
		return waitForBuildUploadID(ctx, client, appID, version, buildNumber, platform, exportStartedAt, exportCompletedAt, pollInterval)
	}
	waitForBuildProcessingFn = (*shared.Deps).WaitForBuildProcessingWithDetails
)

const (
	xcodeExportWaitDefaultTimeout = 15 * time.Minute
)

// NewXcodeCommand returns the local Xcode command group on d.
func NewXcodeCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "xcode",
		ShortUsage: "asc xcode <subcommand> [flags]",
		ShortHelp:  "Local Xcode build/archive/export and signing-settings helpers.",
		LongHelp: `Local Xcode build/archive/export helpers.

The xcode signing plan/apply helpers only modify local
project build settings; they do not contact App Store Connect.

The build/archive/export commands wrap local xcodebuild flows and are supported
on macOS only. Signing-plan generation is cross-platform. Signing apply
requires native identity-coupled file mutation support and currently fails
closed on Windows before modifying project or receipt files.

Use these commands to compile projects and produce deterministic .xcarchive,
.ipa, and .pkg paths that can be passed directly into asc upload and publish
commands.

Examples:
  asc xcode inject --manifest .asc/deployment.json --set version=1.3.0 --overwrite
  asc xcode build --project App.xcodeproj --scheme App --destination 'platform=iOS Simulator,name=iPhone 17 Pro Max,OS=27.0' --no-code-signing --output json
  asc xcode test-destinations --platform iOS --available-only --output json
  asc xcode test junit --xcresult ./Test.xcresult --report-file ./junit.xml --output json
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive --output json
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --output json
  asc xcode export --archive-path .asc/artifacts/MacApp.xcarchive --pkg-path .asc/artifacts/MacApp.pkg --output json
  asc xcode install --ipa .asc/artifacts/App.ipa --device-id COREDEVICE_IDENTIFIER --output json
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode doctor --output json
  asc xcode version view
  asc xcode version bump --type patch
  asc xcode version edit --version "1.3.0" --build-number "42"
  asc xcode signing plan --project ./App.xcodeproj --profile ./signing/App.mobileprovision --configuration Release`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newXcodeInjectCommand(d),
			newXcodeBuildCommand(d),
			newXcodeTestCommand(d),
			newXcodeTestDestinationsCommand(d),
			newXcodeArchiveCommand(d),
			newXcodeExportCommand(d),
			newXcodeDevicesCommand(d),
			newXcodeInstallCommand(d),
			newXcodeInstallSimulatorCommand(d),
			newXcodeExportOptionsGroupCommand(d),
			newXcodeDoctorCommand(d),
			newXcodeValidateCommand(d),
			newXcodeVersionCommand(d),
			newXcodeSigningCommand(d),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// newXcodeArchiveCommand returns the local archive command.
func newXcodeArchiveCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode archive", flag.ExitOnError)

	workspacePath := fs.String("workspace", "", "Path to .xcworkspace directory")
	projectPath := fs.String("project", "", "Path to .xcodeproj directory")
	scheme := fs.String("scheme", "", "Xcode scheme name (required)")
	configuration := fs.String("configuration", "", "Build configuration (for example Release)")
	destination := fs.String("destination", "", "Xcode destination specifier")
	derivedDataPath := fs.String("derived-data-path", "", "DerivedData directory")
	resultBundlePath := fs.String("result-bundle-path", "", "Destination for a new Xcode result bundle (must not already exist)")
	archivePath := fs.String("archive-path", "", "Destination path for the .xcarchive output (required)")
	clean := fs.Bool("clean", false, "Run clean before archive")
	noCodeSigning := fs.Bool("no-code-signing", false, "Set CODE_SIGNING_ALLOWED=NO explicitly")
	sourceProvenance := fs.Bool("source-provenance", false, "Include Git revision and dirty state observed in the --project checkout at build start")
	overwrite := fs.Bool("overwrite", false, "Replace an existing archive at --archive-path")
	var xcodebuildFlags shared.MultiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "archive",
		ShortUsage: "asc xcode archive [flags]",
		ShortHelp:  "Create an .xcarchive at an exact path.",
		LongHelp: `Create an .xcarchive at an exact path.

Provide exactly one of --workspace or --project, plus --scheme and
--archive-path. When both are omitted, asc uses the only .xcworkspace, or else
the only .xcodeproj, in the current directory. Signing follows Xcode defaults; --no-code-signing explicitly
sets CODE_SIGNING_ALLOWED=NO.

--source-provenance requires --project in a Git checkout with a committed HEAD.
It records that checkout's revision and dirty state observed at build start,
without source paths. It does not attest to all build inputs or reproducibility;
workspace and prebuilt inputs cannot supply this observation.

When --derived-data-path is omitted, xcodebuild uses its own default
DerivedData location. Use --result-bundle-path to write a new .xcresult bundle;
the destination must not already exist.

Examples:
  asc xcode archive --workspace App.xcworkspace --scheme App --archive-path .asc/artifacts/App.xcarchive
  asc xcode archive --project App.xcodeproj --scheme App --configuration Release --clean --archive-path .asc/artifacts/App.xcarchive
  asc xcode archive --workspace App.xcworkspace --scheme App --destination 'generic/platform=iOS' --archive-path .asc/artifacts/App.xcarchive --output json
  asc xcode archive --project App.xcodeproj --scheme App --destination 'generic/platform=iOS' --derived-data-path /tmp/AppDerivedData --result-bundle-path /tmp/AppArchive.xcresult --no-code-signing --archive-path .asc/artifacts/App.xcarchive`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "workspace", "project"); emptyFlag != "" {
				return d.UsageErrorf("--%s must not be empty", emptyFlag)
			}
			if strings.TrimSpace(*scheme) == "" {
				return d.RequiredFlag("--scheme")
			}
			if strings.TrimSpace(*archivePath) == "" {
				return d.RequiredFlag("--archive-path")
			}
			if emptyFlag := firstExplicitlyEmptyFlag(fs, "configuration", "destination", "derived-data-path", "result-bundle-path"); emptyFlag != "" {
				return d.UsageErrorf("--%s must not be empty", emptyFlag)
			}
			containerNotice, err := detectXcodeContainer(d, fs, workspacePath, projectPath, "exactly one of --workspace or --project is required")
			if err != nil {
				return err
			}
			if strings.TrimSpace(*workspacePath) == "" && strings.TrimSpace(*projectPath) == "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: exactly one of --workspace or --project is required")
				return shared.MissingRequiredUsageError("")
			}
			if strings.TrimSpace(*workspacePath) != "" && strings.TrimSpace(*projectPath) != "" {
				fmt.Fprintln(d.IO.ErrOut, "Error: exactly one of --workspace or --project is required")
				return shared.WithDiagnostic(flag.ErrHelp, shared.DiagnosticConflictingInput, "")
			}
			if *sourceProvenance && strings.TrimSpace(*projectPath) == "" {
				return d.UsageError("--source-provenance requires --project; a workspace does not identify the producing checkout")
			}
			opts := localxcode.ArchiveOptions{
				WorkspacePath:    osPath(d, strings.TrimSpace(*workspacePath)),
				ProjectPath:      osPath(d, strings.TrimSpace(*projectPath)),
				Scheme:           strings.TrimSpace(*scheme),
				Configuration:    strings.TrimSpace(*configuration),
				Destination:      strings.TrimSpace(*destination),
				DerivedDataPath:  osPath(d, strings.TrimSpace(*derivedDataPath)),
				ResultBundlePath: osPath(d, strings.TrimSpace(*resultBundlePath)),
				ArchivePath:      osPath(d, strings.TrimSpace(*archivePath)),
				Clean:            *clean,
				NoCodeSigning:    *noCodeSigning,
				SourceProvenance: *sourceProvenance,
				Overwrite:        *overwrite,
				XcodebuildArgs:   []string(xcodebuildFlags),
				LogWriter:        d.IO.ChildStderr(),
			}
			if err := localxcode.ValidateArchiveXcodebuildArgs(opts); err != nil {
				return d.UsageError(err.Error())
			}
			if _, err := os.Stat(osPath(d, strings.TrimSpace(*archivePath))); err == nil && !*overwrite {
				return d.UsageErrorf("--archive-path already exists: %s (use --overwrite to replace it)", strings.TrimSpace(*archivePath))
			}
			if containerNotice != "" {
				fmt.Fprintln(d.IO.ErrOut, containerNotice)
			}

			result, err := runArchive(ctx, opts)
			if err != nil {
				return xcodeCommandError("xcode archive", err)
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTableTo(d.IO.Out, []string{"field", "value"}, archiveResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdownTo(d.IO.Out, []string{"field", "value"}, archiveResultRows(result))
					return nil
				},
			)
		},
	}
}

// newXcodeExportCommand returns the local export command.
func newXcodeExportCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode export", flag.ExitOnError)

	archivePath := fs.String("archive-path", "", "Path to the .xcarchive input (required)")
	exportOptions := fs.String("export-options", "", "Path to ExportOptions.plist (generated automatically when omitted)")
	method := fs.String("method", "app-store-connect", "Method for generated options: app-store-connect, release-testing, debugging, or enterprise")
	signingStyle := fs.String("signing-style", "automatic", "Signing style for generated options: automatic or manual")
	teamID := fs.String("team-id", "", "Apple Developer team ID for generated options (overrides archive metadata)")
	ipaPath := fs.String("ipa-path", "", "Destination path for a local .ipa when one is produced")
	pkgPath := fs.String("pkg-path", "", "Destination path for a local macOS .pkg when one is produced")
	exportDir := fs.String("export-dir", "", "Directory that keeps xcodebuild's other export outputs, such as manifest.plist and DistributionSummary.plist")
	overwrite := fs.Bool("overwrite", false, "Replace an existing local artifact at its destination path, and matching entries in --export-dir")
	wait := fs.Bool("wait", false, "Wait for App Store Connect build discovery and processing when export uploads directly")
	pollInterval := fs.Duration("poll-interval", shared.PublishDefaultPollInterval, "Polling interval for --wait when waiting for uploaded builds")
	timeout := fs.Duration("timeout", 0, "Maximum duration for xcodebuild -exportArchive (0 disables local export timeout)")
	var xcodebuildFlags shared.MultiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass an additional raw argument through to xcodebuild; asc-managed arguments are rejected (repeatable)")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc xcode export [flags]",
		ShortHelp:  "Export an archive to an IPA or PKG path, or use direct upload.",
		LongHelp: `Export an archive to a deterministic IPA or PKG path, or upload directly.

This command runs xcodebuild -exportArchive into a temporary directory.
When --export-options is omitted, asc generates archive-adjacent options. It
uses app-store-connect and automatic signing by default. Use
--method release-testing for a local IPA installable on registered devices,
debugging for a development-signed IPA, or enterprise for in-house
distribution; debugging and enterprise support automatic signing only. Use
--signing-style manual to match locally installed certificates and profiles;
--team-id optionally overrides archive metadata.
Automatic signing may need --xcodebuild-flag=-allowProvisioningUpdates so
xcodebuild can create or update provisioning profiles.
For local exports, provide exactly one destination: --ipa-path for iOS, tvOS,
or visionOS, or --pkg-path for macOS. asc moves the exported artifact to that
exact path and discards the other export outputs unless --export-dir keeps
them. Generated manual signing options support iOS, tvOS, and App Store
macOS archives. Methods that export a .app, such as developer-id and
mac-application, are not supported.
When ExportOptions.plist uses destination=upload, xcodebuild uploads directly
to App Store Connect and asc returns archive metadata without writing a local
artifact or requiring a destination path. Use --wait to generate direct-upload
options and poll until the uploaded build appears and finishes processing.

Examples:
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa
  asc xcode export --archive-path .asc/artifacts/MacApp.xcarchive --pkg-path .asc/artifacts/MacApp.pkg
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --method release-testing --signing-style manual
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --signing-style manual --team-id TEAM_ID
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --timeout 10m
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --ipa-path .asc/artifacts/App.ipa --export-dir .asc/artifacts/export
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options UploadExportOptions.plist --wait
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa --xcodebuild-flag=-allowProvisioningUpdates --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if strings.TrimSpace(*archivePath) == "" {
				return d.RequiredFlag("--archive-path")
			}
			if *wait && *pollInterval <= 0 {
				return d.UsageError("--poll-interval must be greater than 0")
			}
			if *timeout < 0 {
				return d.UsageError("--timeout must be zero or greater")
			}
			if err := localxcode.ValidateExportXcodebuildArgs([]string(xcodebuildFlags)); err != nil {
				return d.UsageError(err.Error())
			}
			generationFlagsSet := false
			signingStyleSet := false
			teamIDSet := false
			fs.Visit(func(f *flag.Flag) {
				switch f.Name {
				case "method":
					generationFlagsSet = true
				case "signing-style":
					generationFlagsSet = true
					signingStyleSet = true
				case "team-id":
					generationFlagsSet = true
					teamIDSet = true
				}
			})
			methodValue, err := localxcode.NormalizeExportOptionsMethod(*method)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if signingStyleSet && strings.TrimSpace(*signingStyle) == "" {
				return d.UsageError("--signing-style must be one of: automatic, manual")
			}
			signingStyleValue, err := localxcode.NormalizeExportOptionsSigningStyle(*signingStyle)
			if err != nil {
				return d.UsageError(err.Error())
			}
			teamIDValue := strings.TrimSpace(*teamID)
			if teamIDSet && teamIDValue == "" {
				return d.UsageError("--team-id must not be empty")
			}
			trimmedArchivePath := osPath(d, strings.TrimSpace(*archivePath))
			trimmedIPAPath := osPath(d, strings.TrimSpace(*ipaPath))
			trimmedPKGPath := osPath(d, strings.TrimSpace(*pkgPath))
			if trimmedIPAPath != "" && trimmedPKGPath != "" {
				return d.UsageError("--ipa-path and --pkg-path are mutually exclusive")
			}
			exportOptionsPath := osPath(d, strings.TrimSpace(*exportOptions))
			if exportOptionsPath != "" && generationFlagsSet {
				return d.UsageError("--export-options cannot be combined with --method, --signing-style, or --team-id")
			}
			if *wait && methodValue != "app-store-connect" {
				return d.UsageErrorf("--wait cannot be combined with --method %s", methodValue)
			}
			if exportOptionsPath == "" && trimmedPKGPath != "" && methodValue != "app-store-connect" {
				return d.UsageErrorf("--pkg-path cannot be combined with --method %s", methodValue)
			}
			if err := localxcode.ValidateExportOptionsMethodSigningStyle(methodValue, signingStyleValue); err != nil {
				return d.UsageError(err.Error())
			}
			directUpload := *wait
			if exportOptionsPath != "" {
				directUpload = isDirectUploadExportOptionsFn(exportOptionsPath)
			}
			if trimmedIPAPath == "" && trimmedPKGPath == "" && !directUpload {
				fmt.Fprintln(d.IO.ErrOut, "Error: --ipa-path or --pkg-path is required for a local export")
				return shared.MissingRequiredUsageError("")
			}
			if trimmedIPAPath != "" {
				if err := localxcode.ValidateExportDestination(trimmedIPAPath, *overwrite, directUpload); err != nil {
					if localxcode.IsExportDestinationUsageError(err) {
						return d.UsageError(err.Error())
					}
					return fmt.Errorf("xcode export: %w", err)
				}
			}
			if trimmedPKGPath != "" {
				if err := localxcode.ValidateExportPKGDestination(trimmedPKGPath, *overwrite, directUpload); err != nil {
					if localxcode.IsExportDestinationUsageError(err) {
						return d.UsageError(err.Error())
					}
					return fmt.Errorf("xcode export: %w", err)
				}
			}
			if err := localxcode.ValidateExportDir(osPath(d, *exportDir), trimmedIPAPath, trimmedPKGPath, *overwrite, directUpload); err != nil {
				if localxcode.IsExportDestinationUsageError(err) {
					return d.UsageError(err.Error())
				}
				return fmt.Errorf("xcode export: %w", err)
			}
			if exportOptionsPath == "" {
				destination := "export"
				if *wait {
					destination = "upload"
				}
				if err := runXcodeExportPreflight(ctx); err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if destination != "upload" {
					var err error
					if trimmedPKGPath != "" {
						err = localxcode.PreflightExportPKGDestination(trimmedPKGPath, *overwrite, false)
					} else {
						err = localxcode.PreflightExportDestination(trimmedIPAPath, *overwrite, false)
					}
					if err == nil {
						err = localxcode.PreflightExportDir(osPath(d, *exportDir))
					}
					if err != nil {
						if localxcode.IsExportDestinationUsageError(err) {
							return d.UsageError(err.Error())
						}
						return fmt.Errorf("xcode export: %w", err)
					}
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
					return xcodeCommandError("xcode export: generate export options", err)
				}
				if generated == nil || strings.TrimSpace(generated.Path) == "" {
					return fmt.Errorf("xcode export: generate export options: generator returned an empty path")
				}
				exportOptionsPath = strings.TrimSpace(generated.Path)
			}
			if *wait && !isDirectUploadExportOptionsFn(exportOptionsPath) {
				return d.UsageError("--wait requires ExportOptions.plist with destination=upload")
			}

			var client *asc.Client
			if *wait {
				client, err = d.Client()
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
			}

			exportCtx := ctx
			var exportCancel context.CancelFunc
			if *timeout > 0 {
				exportCtx, exportCancel = context.WithTimeout(ctx, *timeout)
			}
			if exportCancel != nil {
				defer exportCancel()
			}

			exportStartedAt := d.Now()
			result, err := runExport(exportCtx, localxcode.ExportOptions{
				ArchivePath:    trimmedArchivePath,
				ExportOptions:  exportOptionsPath,
				IPAPath:        trimmedIPAPath,
				PKGPath:        trimmedPKGPath,
				ExportDir:      osPath(d, strings.TrimSpace(*exportDir)),
				Overwrite:      *overwrite,
				XcodebuildArgs: []string(xcodebuildFlags),
				LogWriter:      d.IO.ChildStderr(),
			})
			if err != nil {
				if *timeout > 0 && errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("xcode export: timed out after %s while running xcodebuild -exportArchive: %w", timeout.String(), err)
				}
				return xcodeCommandError("xcode export", err)
			}
			exportCompletedAt := d.Now()
			if exportCompletedAt.Before(exportStartedAt) {
				exportCompletedAt = exportStartedAt
			}
			commandResult := xcodeExportCommandResult{
				ArchivePath:       result.ArchivePath,
				ExportOptionsPath: exportOptionsPath,
				IPAPath:           result.IPAPath,
				PKGPath:           result.PKGPath,
				BundleID:          result.BundleID,
				Version:           result.Version,
				BuildNumber:       result.BuildNumber,
				DisplayName:       result.DisplayName,
				ExportDir:         result.ExportDir,
				ExportFiles:       result.ExportFiles,
			}
			if *wait {
				if strings.TrimSpace(result.BundleID) == "" {
					return fmt.Errorf("xcode export: exported archive is missing bundle ID required for --wait")
				}
				if strings.TrimSpace(result.Version) == "" || strings.TrimSpace(result.BuildNumber) == "" {
					return fmt.Errorf("xcode export: exported archive is missing version/build metadata required for --wait")
				}
				waitCtx, cancel := shared.ContextWithTimeoutDuration(ctx, d.ResolveTimeout(shared.RequestTimeoutSetting, xcodeExportWaitDefaultTimeout))
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
				fmt.Fprintf(d.IO.ErrOut, "Waiting for build %s (%s) to appear in App Store Connect...\n", result.BuildNumber, result.Version)
				buildResp, err := waitForBuildByNumberOrUploadFailureFn(d, waitCtx, client, appID, uploadID, result.Version, result.BuildNumber, platform, *pollInterval)
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if buildResp == nil {
					return fmt.Errorf("xcode export: failed to resolve build for version %q build %q", result.Version, result.BuildNumber)
				}
				discoveredBuildID := strings.TrimSpace(buildResp.Data.ID)
				fmt.Fprintf(d.IO.ErrOut, "Build %s discovered; waiting for processing...\n", discoveredBuildID)
				buildResp, err = waitForBuildProcessingFn(d, waitCtx, client, "", discoveredBuildID, *pollInterval)
				if err != nil {
					return fmt.Errorf("xcode export: %w", err)
				}
				if buildResp == nil {
					return fmt.Errorf("xcode export: failed to resolve processed build state for build %q", discoveredBuildID)
				}
				commandResult.BuildID = strings.TrimSpace(buildResp.Data.ID)
				commandResult.ProcessingState = strings.ToUpper(strings.TrimSpace(buildResp.Data.Attributes.ProcessingState))
			}

			return d.PrintOutputWithRenderers(
				commandResult,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTableTo(d.IO.Out, []string{"field", "value"}, exportResultRows(commandResult))
					return nil
				},
				func() error {
					asc.RenderMarkdownTo(d.IO.Out, []string{"field", "value"}, exportResultRows(commandResult))
					return nil
				},
			)
		},
	}
}

// newXcodeValidateCommand returns the local validate command.
func newXcodeValidateCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode validate", flag.ExitOnError)

	ipaPath := fs.String("ipa", "", "Path to an iOS, tvOS, or visionOS .ipa input")
	pkgPath := fs.String("pkg", "", "Path to a macOS .pkg input")
	apiKey := fs.String("api-key", "", "App Store Connect API key ID for altool")
	apiIssuer := fs.String("api-issuer", "", "App Store Connect API issuer ID for altool")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "validate",
		ShortUsage: "asc xcode validate [flags]",
		ShortHelp:  "Validate an IPA or PKG with Apple before upload.",
		LongHelp: `Validate an IPA or PKG with Apple before upload.

This command wraps xcrun altool --validate-app to check whether an IPA or PKG passes
Apple's server-side validation before you upload or submit it.

Exactly one of --ipa or --pkg is required. IPA platform metadata is detected
from the artifact; PKG validation uses the macOS platform.

By default, altool receives asc's resolved credentials (from 'asc auth login'
or ASC_* environment variables), including the private key via --p8-file-path.
With --api-key and --api-issuer, altool looks up AuthKey_<KEY_ID>.p8 in its own
private_keys directories instead.

Examples:
  asc xcode validate --ipa .asc/artifacts/App.ipa
  asc xcode validate --pkg .asc/artifacts/MacApp.pkg
  asc xcode validate --ipa .asc/artifacts/App.ipa --api-key KEY123ABC --api-issuer 00000000-0000-0000-0000-000000000000 --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedIPAPath := osPath(d, strings.TrimSpace(*ipaPath))
			trimmedPKGPath := osPath(d, strings.TrimSpace(*pkgPath))
			trimmedAPIKey := strings.TrimSpace(*apiKey)
			trimmedAPIIssuer := strings.TrimSpace(*apiIssuer)

			if emptyFlag := firstExplicitlyEmptyFlag(fs, "ipa", "pkg"); emptyFlag != "" {
				return d.UsageErrorf("--%s must not be empty", emptyFlag)
			}
			hasIPA := trimmedIPAPath != ""
			hasPKG := trimmedPKGPath != ""
			if !hasIPA && !hasPKG {
				fmt.Fprintln(d.IO.ErrOut, "Error: --ipa or --pkg is required")
				return shared.MissingRequiredUsageError("")
			}
			if hasIPA && hasPKG {
				return d.UsageError("--ipa and --pkg are mutually exclusive")
			}
			if hasIPA && !strings.EqualFold(filepath.Ext(trimmedIPAPath), ".ipa") {
				return d.UsageError("--ipa must end with .ipa")
			}
			if hasPKG && !strings.EqualFold(filepath.Ext(trimmedPKGPath), ".pkg") {
				return d.UsageError("--pkg must end with .pkg")
			}
			if (trimmedAPIKey == "") != (trimmedAPIIssuer == "") {
				return d.UsageError("--api-key and --api-issuer must be provided together")
			}
			p8FilePath := ""
			if trimmedAPIKey == "" {
				var err error
				trimmedAPIKey, trimmedAPIIssuer, p8FilePath, err = resolveAltoolCredentials()
				if err != nil {
					return classifyXcodeLocalFailure(fmt.Errorf("xcode validate: %w", err))
				}
			}

			result, err := runValidate(ctx, localxcode.ValidateOptions{
				IPAPath:    trimmedIPAPath,
				PKGPath:    trimmedPKGPath,
				APIKey:     trimmedAPIKey,
				APIIssuer:  trimmedAPIIssuer,
				P8FilePath: p8FilePath,
				LogWriter:  d.IO.ChildStderr(),
			})
			if err != nil {
				return xcodeCommandError("xcode validate", err)
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTableTo(d.IO.Out, []string{"field", "value"}, validateResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdownTo(d.IO.Out, []string{"field", "value"}, validateResultRows(result))
					return nil
				},
			)
		},
	}
}

// resolveAltoolCredentials returns asc's own API key, issuer, and .p8 path so
// altool does not depend on its private_keys search directories.
func resolveAltoolCredentials() (keyID, issuerID, p8FilePath string, err error) {
	creds, err := shared.ResolveAuthCredentials("")
	if err != nil {
		return "", "", "", err
	}
	if config.IsEnterpriseCredentialKeyType(creds.KeyType) {
		return "", "", "", fmt.Errorf("altool: %w", asc.ErrEnterpriseProgramAPIUnsupported)
	}
	p8FilePath, err = shared.AltoolPrivateKeyPath(creds)
	if err != nil {
		return "", "", "", fmt.Errorf("prepare private key for altool: %w", err)
	}
	if strings.TrimSpace(creds.IssuerID) == "" || p8FilePath == "" {
		return "", "", "", fmt.Errorf("%w: altool needs a team API key with an issuer ID and a readable .p8 file; run 'asc auth login' or pass --api-key and --api-issuer", shared.ErrMissingAuth)
	}
	return strings.TrimSpace(creds.KeyID), strings.TrimSpace(creds.IssuerID), p8FilePath, nil
}

// xcodeCommandError prefixes err with command and classifies a missing input
// path as a validation failure instead of an internal error.
func xcodeCommandError(command string, err error) error {
	wrapped := fmt.Errorf("%s: %w", command, err)
	var notFound *localxcode.InputPathNotFoundError
	if errors.As(err, &notFound) {
		return shared.WithDiagnostic(shared.NewValidationError(wrapped), shared.DiagnosticFileNotFound, notFound.Flag)
	}
	return classifyXcodeLocalFailure(wrapped)
}

// reportXcodeFailure prints the stderr summary for a failed xcodebuild run:
// its exit status, or the interruption cause when it has none.
func reportXcodeFailure(d *shared.Deps, command string, exitStatus *int, err error) {
	message := command + " failed"
	if exitStatus != nil {
		message = fmt.Sprintf("%s with exit status %d", message, *exitStatus)
	} else {
		message = fmt.Sprintf("%s: %v", message, xcodeInterruptionReason(err))
	}
	fmt.Fprintf(d.IO.ErrOut, "Error: %s\n", message)
}

func xcodeInterruptionReason(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() < 0 {
		return exitErr
	}
	return err
}

// classifyXcodeLocalFailure reports failures of the local Xcode toolchain or
// project selection as validation outcomes instead of asc defects: a project
// that does not compile or sign, or project flags that do not resolve to one
// .xcodeproj. Other errors, including timeouts, are returned unchanged.
func classifyXcodeLocalFailure(err error) error {
	if err == nil || shared.IsValidationError(err) {
		return err
	}
	var project *localxcode.ProjectSelectionError
	if errors.As(err, &project) {
		code := shared.DiagnosticInvalidInput
		if project.Missing {
			code = shared.DiagnosticFileNotFound
		}
		return shared.WithDiagnostic(shared.NewValidationError(err), code, project.Flag)
	}
	var toolFailure *localxcode.CommandFailureError
	if errors.As(err, &toolFailure) {
		return shared.WithDiagnostic(shared.NewValidationError(err), shared.DiagnosticDependencyFailed, "")
	}
	return err
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
	if result.DisplayName != "" {
		rows = append(rows, []string{"display_name", result.DisplayName})
	}
	if result.ProjectCheckout != nil {
		rows = append(rows, []string{"project_checkout_revision", result.ProjectCheckout.Revision}, []string{"project_checkout_dirty", fmt.Sprint(result.ProjectCheckout.Dirty)}, []string{"project_checkout_observed_at", result.ProjectCheckout.ObservedAt})
	}
	return rows
}

type xcodeExportCommandResult struct {
	ArchivePath       string `json:"archive_path"`
	ExportOptionsPath string `json:"export_options_path"`
	IPAPath           string `json:"ipa_path"`
	PKGPath           string `json:"pkg_path,omitempty"`
	BundleID          string `json:"bundle_id,omitempty"`
	Version           string `json:"version,omitempty"`
	BuildNumber       string `json:"build_number,omitempty"`
	DisplayName       string `json:"displayName,omitempty"`
	BuildID           string `json:"build_id,omitempty"`
	ProcessingState   string `json:"processing_state,omitempty"`

	ExportDir   string   `json:"exportDir,omitempty"`
	ExportFiles []string `json:"exportFiles,omitempty"`
}

func exportResultRows(result xcodeExportCommandResult) [][]string {
	rows := [][]string{
		{"archive_path", result.ArchivePath},
		{"export_options_path", result.ExportOptionsPath},
	}
	if result.IPAPath != "" {
		rows = append(rows, []string{"ipa_path", result.IPAPath})
	} else if result.PKGPath != "" {
		rows = append(rows, []string{"pkg_path", result.PKGPath})
	} else {
		rows = append(rows, []string{"ipa_path", "(direct upload — no local artifact)"})
	}
	if result.ExportDir != "" {
		rows = append(rows, []string{"export_dir", result.ExportDir})
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
	if result.DisplayName != "" {
		rows = append(rows, []string{"display_name", result.DisplayName})
	}
	if strings.TrimSpace(result.ProcessingState) != "" {
		rows = append(rows, []string{"processing_state", result.ProcessingState})
	}
	return rows
}

func validateResultRows(result *localxcode.ValidateResult) [][]string {
	rows := make([][]string, 0, 2)
	if strings.TrimSpace(result.PKGPath) != "" {
		rows = append(rows, []string{"pkg_path", result.PKGPath})
	} else {
		rows = append(rows, []string{"ipa_path", result.IPAPath})
	}
	return append(rows, []string{"validated", fmt.Sprintf("%t", result.Validated)})
}

// detectXcodeContainer fills --workspace or --project, when neither was
// given, with the only workspace, or else the only project, at the top of the
// working directory. It returns the notice to print once the command passes
// validation. When the choice is ambiguous it returns the required usage
// error with what was found.
func detectXcodeContainer(d *shared.Deps, fs *flag.FlagSet, workspacePath, projectPath *string, required string) (string, error) {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "workspace" || f.Name == "project" {
			given = true
		}
	})
	if given {
		return "", nil
	}
	entries, err := readWorkingDirectory(d)
	if err != nil {
		return "", d.UsageErrorf("%s; could not list the current directory: %v", required, err)
	}
	var workspaces, projects []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".xcworkspace":
			workspaces = append(workspaces, entry.Name())
		case ".xcodeproj":
			projects = append(projects, entry.Name())
		}
	}
	slices.Sort(workspaces)
	slices.Sort(projects)
	for _, kind := range []struct {
		name  string
		found []string
		flag  string
		value *string
	}{
		{"workspace", workspaces, "--workspace", workspacePath},
		{"project", projects, "--project", projectPath},
	} {
		switch len(kind.found) {
		case 0:
			continue
		case 1:
			*kind.value = kind.found[0]
			return fmt.Sprintf("Using %s %s", kind.name, kind.found[0]), nil
		default:
			return "", d.UsageErrorf("%s; found %d %ss: %s; pass %s", required, len(kind.found), kind.name, strings.Join(kind.found, ", "), kind.flag)
		}
	}
	return "", nil
}

func readWorkingDirectory(d *shared.Deps) ([]os.DirEntry, error) {
	root, err := rootfs.New(osPath(d, "."))
	if err != nil {
		return nil, err
	}
	defer root.Close()
	dir, err := root.OpenDir(".")
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadDir(-1)
}

// osPath is d.OSPath for a path flag value, keeping an empty value empty.
func osPath(d *shared.Deps, path string) string {
	if path == "" {
		return ""
	}
	return d.OSPath(path)
}

func osPaths(d *shared.Deps, paths []string) []string {
	if paths == nil {
		return nil
	}
	resolved := make([]string, len(paths))
	for i, path := range paths {
		resolved[i] = osPath(d, path)
	}
	return resolved
}
