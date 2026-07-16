package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MetadataKeywordsSyncResult combines import and remote planning/apply.
type MetadataKeywordsSyncResult struct {
	Import MetadataKeywordsImportResult `json:"import"`
	Plan   *MetadataKeywordsPlanResult  `json:"plan,omitempty"`
}

// MetadataKeywordsSyncCommand returns the keywords sync subcommand.
func MetadataKeywordsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords sync", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	input := fs.String("input", "", "Import file path or - for stdin (required)")
	format := fs.String("format", keywordImportFormatAuto, "Input format: auto, csv, json, text, or astro-csv")
	locale := fs.String("locale", "", "Default locale for inputs without a locale column/field")
	sideDataReportFile := fs.String("side-data-report-file", "", "Optional path to write side-data report JSON when research fields are present")
	dryRun := fs.Bool("dry-run", false, "Preview import and remote keyword changes without writing or mutating")
	confirm := fs.Bool("confirm", false, "Confirm remote keyword mutations after import")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "sync",
		ShortUsage: "asc metadata keywords sync --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" --input \"./keywords.csv\" [flags]",
		ShortHelp:  "Import keyword input and sync the resulting keyword plan.",
		LongHelp: `Import keyword input and sync the resulting keyword plan.

Workflow:
  1. normalize provider input into canonical metadata keyword files
  2. build a keyword-only remote plan for the imported locales
  3. apply the remote changes only when --confirm is provided

Without ` + "`--confirm`" + `, sync writes local files (unless ` + "`--dry-run`" + `)
and returns a non-mutating remote plan.

Examples:
  asc metadata keywords sync --app "APP_ID" --version "1.2.3" --dir "./metadata" --input "./keywords.csv"
  asc metadata keywords sync --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --input "./keywords.json" --format json --confirm
  asc metadata keywords sync --app "APP_ID" --version "1.2.3" --dir "./metadata" --format text --locale "en-US" --input "./keywords.txt" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords sync does not accept positional arguments")
			}
			resolvedAppID, versionValue, dirValue, platformValue, err := validateMetadataKeywordsRemoteInputs(*appID, *version, *dir, *platform)
			if err != nil {
				return fmt.Errorf("metadata keywords sync: %w", err)
			}
			importPayload, err := executeMetadataKeywordsImportWithState(metadataKeywordsImportOptions{
				Dir:                *dir,
				Version:            *version,
				Input:              *input,
				Format:             *format,
				DefaultLocale:      *locale,
				DryRun:             *dryRun,
				Overwrite:          true,
				SideDataReportFile: *sideDataReportFile,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords sync: %w", err)
			}
			if !importPayload.result.Valid {
				result := MetadataKeywordsSyncResult{Import: importPayload.result}
				if err := shared.PrintOutputWithRenderers(
					result,
					*output.Output,
					*output.Pretty,
					func() error { return printMetadataKeywordsSyncTable(result) },
					func() error { return printMetadataKeywordsSyncMarkdown(result) },
				); err != nil {
					return err
				}
				return shared.NewReportedError(fmt.Errorf("metadata keywords sync: found %d import issue(s)", len(importPayload.result.Issues)))
			}

			planResult, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:                resolvedAppID,
				Version:              versionValue,
				Platform:             platformValue,
				Dir:                  dirValue,
				DryRun:               *dryRun || !*confirm,
				Apply:                !*dryRun && *confirm,
				Confirm:              *confirm,
				FailureArtifactScope: "metadata-keywords-sync",
				LocalState:           importPayload.states,
			})
			if err != nil && errors.Is(err, flag.ErrHelp) {
				return err
			}
			if !shouldPrintMetadataKeywordsPlanResult(planResult, err) {
				return fmt.Errorf("metadata keywords sync: %w", err)
			}

			result := MetadataKeywordsSyncResult{
				Import: importPayload.result,
				Plan:   &planResult,
			}
			if printErr := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsSyncTable(result) },
				func() error { return printMetadataKeywordsSyncMarkdown(result) },
			); printErr != nil {
				return printErr
			}
			if err != nil {
				return fmt.Errorf("metadata keywords sync: %w", err)
			}
			if planResult.Failed > 0 {
				return shared.NewReportedError(fmt.Errorf("metadata keywords sync: %d locale(s) failed", planResult.Failed))
			}
			return nil
		},
	}
}

func printMetadataKeywordsSyncTable(result MetadataKeywordsSyncResult) error {
	if err := printMetadataKeywordFileResultTable("Keyword Import", result.Import.Results, result.Import.DetectedLocales, result.Import.Issues, result.Import.Dir, result.Import.Version, result.Import.DryRun, result.Import.SideDataRecordCount, result.Import.SideDataReportPath); err != nil {
		return err
	}
	if result.Plan == nil {
		return nil
	}
	fmt.Println()
	return printMetadataKeywordsPlanTable(*result.Plan)
}

func printMetadataKeywordsSyncMarkdown(result MetadataKeywordsSyncResult) error {
	if err := printMetadataKeywordFileResultMarkdown("Keyword Import", result.Import.Results, result.Import.DetectedLocales, result.Import.Issues, result.Import.Dir, result.Import.Version, result.Import.DryRun, result.Import.SideDataRecordCount, result.Import.SideDataReportPath); err != nil {
		return err
	}
	if result.Plan == nil {
		return nil
	}
	fmt.Println()
	return printMetadataKeywordsPlanMarkdown(*result.Plan)
}
