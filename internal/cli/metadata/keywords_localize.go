package metadata

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// MetadataKeywordsLocalizeResult describes one localization-copy run.
type MetadataKeywordsLocalizeResult struct {
	Dir                 string                      `json:"dir"`
	Version             string                      `json:"version"`
	FromLocale          string                      `json:"fromLocale"`
	DryRun              bool                        `json:"dryRun"`
	Valid               bool                        `json:"valid"`
	DetectedLocales     []string                    `json:"detectedLocales"`
	Results             []MetadataKeywordFileResult `json:"results"`
	Issues              []MetadataKeywordIssue      `json:"issues,omitempty"`
	SideDataRecordCount int                         `json:"sideDataRecordCount,omitempty"`
	SideDataReportPath  string                      `json:"sideDataReportPath,omitempty"`
}

type metadataKeywordsLocalizeOptions struct {
	Dir           string
	Version       string
	FromLocale    string
	TargetLocales []string
	DryRun        bool
	Overwrite     bool
}

// MetadataKeywordsLocalizeCommand returns the keywords localize subcommand.
func MetadataKeywordsLocalizeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords localize", flag.ExitOnError)

	dir := fs.String("dir", "", "Metadata root directory (required)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	fromLocale := fs.String("from-locale", "", "Source locale to copy keywords from (required)")
	toLocales := fs.String("to-locales", "", "Target locales (comma-separated, required)")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing target keyword fields")
	dryRun := fs.Bool("dry-run", false, "Preview local file changes without writing files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "localize",
		ShortUsage: "asc metadata keywords localize --dir \"./metadata\" --version \"1.2.3\" --from-locale \"en-US\" --to-locales \"fr-FR,de-DE\" [flags]",
		ShortHelp:  "Copy one locale's canonical keywords into other locales.",
		LongHelp: `Copy one locale's canonical keywords into other locales.

This command copies the canonical keyword field from one locale into one or
more target locale files. It does not translate terms; it seeds target locale
files so they can be reviewed and refined before apply.

Examples:
  asc metadata keywords localize --dir "./metadata" --version "1.2.3" --from-locale "en-US" --to-locales "fr-FR,de-DE"
  asc metadata keywords localize --dir "./metadata" --version "1.2.3" --from-locale "en-US" --to-locales "it,es-MX" --overwrite --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords localize does not accept positional arguments")
			}
			targets := shared.SplitUniqueCSV(*toLocales)
			result, err := executeMetadataKeywordsLocalize(metadataKeywordsLocalizeOptions{
				Dir:           *dir,
				Version:       *version,
				FromLocale:    *fromLocale,
				TargetLocales: targets,
				DryRun:        *dryRun,
				Overwrite:     *overwrite,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords localize: %w", err)
			}
			if err := shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					return printMetadataKeywordFileResultTable("Keyword Localize", result.Results, result.DetectedLocales, result.Issues, result.Dir, result.Version, result.DryRun, 0, "")
				},
				func() error {
					return printMetadataKeywordFileResultMarkdown("Keyword Localize", result.Results, result.DetectedLocales, result.Issues, result.Dir, result.Version, result.DryRun, 0, "")
				},
			); err != nil {
				return err
			}
			if !result.Valid {
				return shared.NewReportedError(fmt.Errorf("metadata keywords localize: found %d issue(s)", len(result.Issues)))
			}
			return nil
		},
	}
}

func executeMetadataKeywordsLocalize(opts metadataKeywordsLocalizeOptions) (MetadataKeywordsLocalizeResult, error) {
	dirValue, versionValue, err := validateMetadataKeywordDirVersion(opts.Dir, opts.Version)
	if err != nil {
		return MetadataKeywordsLocalizeResult{}, err
	}
	sourceLocale, err := validateMetadataKeywordLocale(opts.FromLocale)
	if err != nil {
		return MetadataKeywordsLocalizeResult{}, shared.UsageError(err.Error())
	}
	if len(opts.TargetLocales) == 0 {
		return MetadataKeywordsLocalizeResult{}, shared.UsageError("--to-locales is required")
	}

	targets := make([]string, 0, len(opts.TargetLocales))
	for _, rawLocale := range opts.TargetLocales {
		resolvedLocale, localeErr := validateMetadataKeywordLocale(rawLocale)
		if localeErr != nil {
			return MetadataKeywordsLocalizeResult{}, shared.UsageError(localeErr.Error())
		}
		if strings.EqualFold(resolvedLocale, sourceLocale) {
			return MetadataKeywordsLocalizeResult{}, shared.UsageError("--from-locale must be different from every target locale")
		}
		targets = append(targets, resolvedLocale)
	}
	sort.Strings(targets)

	sourcePath, err := resolveExistingVersionLocalizationPath(dirValue, versionValue, sourceLocale)
	if err != nil {
		return MetadataKeywordsLocalizeResult{}, err
	}
	sourceLocalization, err := ReadVersionLocalizationFile(sourcePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return MetadataKeywordsLocalizeResult{}, shared.UsageErrorf("source locale file %s does not exist", sourcePath)
		}
		return MetadataKeywordsLocalizeResult{}, err
	}
	sourceKeywords, keywordErr := parseMetadataKeywordField(sourceLocalization.Keywords)
	if keywordErr != nil {
		return MetadataKeywordsLocalizeResult{}, shared.UsageErrorf("source locale %q has invalid keywords: %v", sourceLocale, keywordErr)
	}

	valuesByLocale := make(map[string][]string, len(targets))
	for _, locale := range targets {
		valuesByLocale[locale] = append([]string(nil), sourceKeywords...)
	}

	_, results, plans, issues, err := buildMetadataKeywordWriteResults(dirValue, versionValue, valuesByLocale, opts.Overwrite)
	if err != nil {
		return MetadataKeywordsLocalizeResult{}, err
	}
	if !opts.DryRun && len(issues) == 0 {
		if err := ApplyWritePlans(plans); err != nil {
			return MetadataKeywordsLocalizeResult{}, err
		}
	}

	return MetadataKeywordsLocalizeResult{
		Dir:             dirValue,
		Version:         versionValue,
		FromLocale:      sourceLocale,
		DryRun:          opts.DryRun,
		Valid:           len(issues) == 0,
		DetectedLocales: targets,
		Results:         results,
		Issues:          issues,
	}, nil
}
