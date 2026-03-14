package xcode

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	localxcode "github.com/rudrankriyam/App-Store-Connect-CLI/internal/xcode"
)

var (
	runArchive = localxcode.Archive
	runExport  = localxcode.Export
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
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa --output json`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			XcodeArchiveCommand(),
			XcodeExportCommand(),
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
	ipaPath := fs.String("ipa-path", "", "Destination path for the exported .ipa file (required)")
	overwrite := fs.Bool("overwrite", false, "Replace an existing IPA at --ipa-path")
	var xcodebuildFlags multiStringFlag
	fs.Var(&xcodebuildFlags, "xcodebuild-flag", "Pass a raw argument through to xcodebuild (repeatable)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "export",
		ShortUsage: "asc xcode export [flags]",
		ShortHelp:  "Export an .ipa to an exact path.",
		LongHelp: `Export an .ipa to an exact path.

This command runs xcodebuild -exportArchive into a temporary directory and then
moves the produced IPA to --ipa-path so workflows and humans can rely on one
deterministic artifact path.

Examples:
  asc xcode export --archive-path .asc/artifacts/App.xcarchive --export-options ExportOptions.plist --ipa-path .asc/artifacts/App.ipa
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

			result, err := runExport(ctx, localxcode.ExportOptions{
				ArchivePath:    strings.TrimSpace(*archivePath),
				ExportOptions:  strings.TrimSpace(*exportOptions),
				IPAPath:        strings.TrimSpace(*ipaPath),
				Overwrite:      *overwrite,
				XcodebuildArgs: []string(xcodebuildFlags),
				LogWriter:      os.Stderr,
			})
			if err != nil {
				return fmt.Errorf("xcode export: %w", err)
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					asc.RenderTable([]string{"field", "value"}, exportResultRows(result))
					return nil
				},
				func() error {
					asc.RenderMarkdown([]string{"field", "value"}, exportResultRows(result))
					return nil
				},
			)
		},
	}
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

func exportResultRows(result *localxcode.ExportResult) [][]string {
	return [][]string{
		{"archive_path", result.ArchivePath},
		{"ipa_path", result.IPAPath},
		{"bundle_id", result.BundleID},
		{"version", result.Version},
		{"build_number", result.BuildNumber},
	}
}
