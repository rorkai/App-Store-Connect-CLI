package xcode

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const (
	defaultXcodeExportOptionsPath          = ".asc/export-options-app-store.plist"
	defaultReleaseTestingExportOptionsPath = ".asc/export-options-release-testing.plist"
)

// XcodeExportOptionsGroupCommand returns the export-options command group.
func XcodeExportOptionsGroupCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode export-options", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "export-options",
		ShortUsage: "asc xcode export-options <subcommand> [flags]",
		ShortHelp:  "Generate Xcode ExportOptions.plist files.",
		LongHelp: `Generate Xcode ExportOptions.plist files.

Use generate to create App Store Connect or release-testing export options from
an existing .xcarchive. Automatic signing lets Xcode resolve signing for the
app and any embedded targets; provide --signing-style manual to resolve local
profiles.

Examples:
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --method release-testing
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --destination upload --output-path .asc/UploadExportOptions.plist`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeExportOptionsCommand(),
		},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}
}

// XcodeExportOptionsCommand returns the export-options generate command.
//
// It intentionally returns the leaf command so callers can exercise generate
// directly without building the parent command tree.
func XcodeExportOptionsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("xcode export-options generate", flag.ExitOnError)

	archivePath := fs.String("archive-path", "", "Path to the .xcarchive input (required)")
	outputPath := fs.String("output-path", "", "Destination path for the generated ExportOptions.plist (defaults to a method-specific .asc path)")
	method := fs.String("method", "app-store-connect", "[experimental] Xcode export method: app-store-connect or release-testing")
	destination := fs.String("destination", "export", "Xcode export destination: export or upload")
	signingStyle := fs.String("signing-style", "automatic", "Signing style: automatic or manual")
	teamID := fs.String("team-id", "", "Apple Developer team ID (overrides archive metadata)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing file at --output-path")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc xcode export-options generate [flags]",
		ShortHelp:  "Generate an ExportOptions.plist from an Xcode archive.",
		LongHelp: `Generate an ExportOptions.plist from an Xcode archive.

The generated plist uses app-store-connect by default. Use
--method release-testing to create a local IPA for registered devices. Xcode's
older ad-hoc method name is deprecated and is not accepted. Release-testing
requires destination=export.

Examples:
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --method release-testing --signing-style manual
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --destination upload --output-path .asc/UploadExportOptions.plist
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --signing-style manual --team-id TEAM_ID --overwrite --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				fmt.Fprintln(os.Stderr, "Error: xcode export-options generate does not accept positional arguments")
				return flag.ErrHelp
			}

			trimmedDestination := strings.TrimSpace(*destination)
			if trimmedDestination != "export" && trimmedDestination != "upload" {
				return shared.UsageError("--destination must be one of: export, upload")
			}
			trimmedSigningStyle, err := localxcode.NormalizeExportOptionsSigningStyle(*signingStyle)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			trimmedMethod, err := localxcode.NormalizeExportOptionsMethod(*method)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			if trimmedMethod == "release-testing" && trimmedDestination != "export" {
				return shared.UsageError("release-testing requires destination=export")
			}

			trimmedArchivePath := strings.TrimSpace(*archivePath)
			if trimmedArchivePath == "" {
				fmt.Fprintln(os.Stderr, "Error: --archive-path is required")
				return shared.MissingRequiredUsageError()
			}
			outputPathSet := false
			fs.Visit(func(f *flag.Flag) {
				if f.Name == "output-path" {
					outputPathSet = true
				}
			})
			trimmedOutputPath := strings.TrimSpace(*outputPath)
			if !outputPathSet {
				trimmedOutputPath = defaultXcodeExportOptionsPath
				if trimmedMethod == "release-testing" {
					trimmedOutputPath = defaultReleaseTestingExportOptionsPath
				}
			}
			if trimmedOutputPath == "" {
				fmt.Fprintln(os.Stderr, "Error: --output-path is required")
				return shared.MissingRequiredUsageError()
			}

			result, err := runGenerateExportOptions(ctx, localxcode.ExportOptionsGenerateOptions{
				ArchivePath:  trimmedArchivePath,
				OutputPath:   trimmedOutputPath,
				Method:       trimmedMethod,
				Destination:  trimmedDestination,
				SigningStyle: trimmedSigningStyle,
				TeamID:       strings.TrimSpace(*teamID),
				Overwrite:    *overwrite,
			})
			if err != nil {
				return fmt.Errorf("xcode export-options generate: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, exportOptionsResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, exportOptionsResultRows(result))
					return nil
				},
			)
		},
	}
}

func exportOptionsResultRows(result *localxcode.ExportOptionsGenerateResult) [][]string {
	rows := [][]string{
		{"path", result.Path},
		{"archive_path", result.ArchivePath},
		{"method", result.Method},
		{"destination", result.Destination},
		{"signing_style", result.SigningStyle},
	}
	if teamID := strings.TrimSpace(result.TeamID); teamID != "" {
		rows = append(rows, []string{"team_id", teamID})
	}
	if signingCertificate := strings.TrimSpace(result.SigningCertificate); signingCertificate != "" {
		rows = append(rows, []string{"signing_certificate", signingCertificate})
	}
	bundleIDs := make([]string, 0, len(result.ProvisioningProfiles))
	for bundleID := range result.ProvisioningProfiles {
		bundleIDs = append(bundleIDs, bundleID)
	}
	sort.Strings(bundleIDs)
	for _, bundleID := range bundleIDs {
		rows = append(rows, []string{"provisioning_profile." + bundleID, result.ProvisioningProfiles[bundleID]})
	}
	rows = append(rows, []string{"overwritten", fmt.Sprintf("%t", result.Overwritten)})
	return rows
}
