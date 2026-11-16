package xcode

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

const (
	defaultXcodeExportOptionsPath = ".asc/export-options-app-store.plist"
)

// newXcodeExportOptionsGroupCommand returns the export-options command group.
func newXcodeExportOptionsGroupCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode export-options", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "export-options",
		ShortUsage: "asc xcode export-options <subcommand> [flags]",
		ShortHelp:  "Generate Xcode ExportOptions.plist files.",
		LongHelp: `Generate Xcode ExportOptions.plist files.

Use generate to create app-store-connect, release-testing, debugging, or
enterprise export options from an existing .xcarchive. Automatic signing lets Xcode resolve signing for the
app and any embedded targets; provide --signing-style manual to resolve local
profiles.

Manual App Store macOS generation also resolves an installed package-installer
identity. Profile mappings are emitted for executable bundles that carry an
embedded provisioning profile or whose entitlements require one.

Examples:
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --method release-testing
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --destination upload --output-path .asc/UploadExportOptions.plist`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			newXcodeExportOptionsCommand(d),
		},
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}
}

// newXcodeExportOptionsCommand returns the export-options generate command.
//
// It intentionally returns the leaf command so callers can exercise generate
// directly without building the parent command tree.
func newXcodeExportOptionsCommand(d *shared.Deps) *ffcli.Command {
	fs := d.NewFlagSet("xcode export-options generate", flag.ExitOnError)

	archivePath := fs.String("archive-path", "", "Path to the .xcarchive input (required)")
	outputPath := fs.String("output-path", "", "Destination path for the generated ExportOptions.plist (defaults to a method-specific .asc path)")
	method := fs.String("method", "app-store-connect", "Xcode export method: app-store-connect, release-testing, debugging, or enterprise")
	destination := fs.String("destination", "export", "Xcode export destination: export or upload")
	signingStyle := fs.String("signing-style", "automatic", "Signing style: automatic or manual")
	teamID := fs.String("team-id", "", "Apple Developer team ID (overrides archive metadata)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing file at --output-path")
	output := d.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "asc xcode export-options generate [flags]",
		ShortHelp:  "Generate an ExportOptions.plist from an Xcode archive.",
		LongHelp: `Generate an ExportOptions.plist from an Xcode archive.

The generated plist uses app-store-connect by default. Use
--method release-testing to create a local IPA for registered devices,
debugging for a development-signed IPA, or enterprise for in-house
distribution. Xcode's deprecated ad-hoc and development names are not
accepted. Methods other than app-store-connect require destination=export, and
debugging and enterprise support automatic signing only.

Examples:
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --method release-testing --signing-style manual
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --destination upload --output-path .asc/UploadExportOptions.plist
  asc xcode export-options generate --archive-path .asc/artifacts/App.xcarchive --signing-style manual --team-id TEAM_ID --overwrite --output json`,
		FlagSet:   fs,
		UsageFunc: d.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			trimmedDestination := strings.TrimSpace(*destination)
			if trimmedDestination != "export" && trimmedDestination != "upload" {
				return d.UsageError("--destination must be one of: export, upload")
			}
			trimmedSigningStyle, err := localxcode.NormalizeExportOptionsSigningStyle(*signingStyle)
			if err != nil {
				return d.UsageError(err.Error())
			}
			trimmedMethod, err := localxcode.NormalizeExportOptionsMethod(*method)
			if err != nil {
				return d.UsageError(err.Error())
			}
			if trimmedMethod != "app-store-connect" && trimmedDestination != "export" {
				return d.UsageErrorf("%s requires destination=export", trimmedMethod)
			}
			if err := localxcode.ValidateExportOptionsMethodSigningStyle(trimmedMethod, trimmedSigningStyle); err != nil {
				return d.UsageError(err.Error())
			}

			trimmedArchivePath := osPath(d, strings.TrimSpace(*archivePath))
			if trimmedArchivePath == "" {
				return d.RequiredFlag("--archive-path")
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
				if trimmedMethod != "app-store-connect" {
					trimmedOutputPath = ".asc/export-options-" + trimmedMethod + ".plist"
				}
			}
			if trimmedOutputPath == "" {
				return d.RequiredFlag("--output-path")
			}
			if err := localxcode.ValidateExportOptionsArchivePath(trimmedArchivePath); err != nil {
				return d.UsageErrorf("xcode export-options generate: %v", err)
			}

			result, err := runGenerateExportOptions(ctx, localxcode.ExportOptionsGenerateOptions{
				ArchivePath:  trimmedArchivePath,
				OutputPath:   osPath(d, trimmedOutputPath),
				Method:       trimmedMethod,
				Destination:  trimmedDestination,
				SigningStyle: trimmedSigningStyle,
				TeamID:       strings.TrimSpace(*teamID),
				Overwrite:    *overwrite,
			})
			if err != nil {
				return fmt.Errorf("xcode export-options generate: %w", err)
			}

			return d.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTableTo(d.IO.Out, []string{"field", "value"}, exportOptionsResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdownTo(d.IO.Out, []string{"field", "value"}, exportOptionsResultRows(result))
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
	if installerSigningCertificate := strings.TrimSpace(result.InstallerSigningCertificate); installerSigningCertificate != "" {
		rows = append(rows, []string{"installer_signing_certificate", installerSigningCertificate})
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
