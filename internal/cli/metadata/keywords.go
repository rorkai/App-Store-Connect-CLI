package metadata

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/validation"
)

const (
	keywordImportFormatAuto = "auto"
	keywordImportFormatCSV  = "csv"
	keywordImportFormatJSON = "json"
	keywordImportFormatText = "text"
)

var keywordPlanFields = []string{"keywords"}

// MetadataKeywordFileResult describes one local keyword file change.
type MetadataKeywordFileResult struct {
	Locale       string `json:"locale"`
	File         string `json:"file"`
	Action       string `json:"action"`
	Reason       string `json:"reason,omitempty"`
	KeywordField string `json:"keywordField,omitempty"`
	KeywordCount int    `json:"keywordCount,omitempty"`
}

// MetadataKeywordsImportResult describes one import run.
type MetadataKeywordsImportResult struct {
	Dir     string                      `json:"dir"`
	Version string                      `json:"version"`
	Input   string                      `json:"input"`
	Format  string                      `json:"format"`
	DryRun  bool                        `json:"dryRun"`
	Results []MetadataKeywordFileResult `json:"results"`
}

// MetadataKeywordsLocalizeResult describes one localization-copy run.
type MetadataKeywordsLocalizeResult struct {
	Dir        string                      `json:"dir"`
	Version    string                      `json:"version"`
	FromLocale string                      `json:"fromLocale"`
	DryRun     bool                        `json:"dryRun"`
	Results    []MetadataKeywordFileResult `json:"results"`
}

// MetadataKeywordsWarning highlights submit-readiness risk during keyword creates.
type MetadataKeywordsWarning struct {
	Locale        string   `json:"locale"`
	Message       string   `json:"message"`
	MissingFields []string `json:"missingFields,omitempty"`
}

// MetadataKeywordsPlanResult describes keyword-only remote changes.
type MetadataKeywordsPlanResult struct {
	AppID     string                    `json:"appId"`
	Version   string                    `json:"version"`
	VersionID string                    `json:"versionId"`
	Dir       string                    `json:"dir"`
	DryRun    bool                      `json:"dryRun"`
	Applied   bool                      `json:"applied,omitempty"`
	Adds      []PlanItem                `json:"adds"`
	Updates   []PlanItem                `json:"updates"`
	APICalls  []PlanAPICall             `json:"apiCalls,omitempty"`
	Actions   []ApplyAction             `json:"actions,omitempty"`
	Warnings  []MetadataKeywordsWarning `json:"warnings,omitempty"`
}

// MetadataKeywordsSyncResult combines import and remote planning/apply.
type MetadataKeywordsSyncResult struct {
	Import MetadataKeywordsImportResult `json:"import"`
	Plan   MetadataKeywordsPlanResult   `json:"plan"`
}

type metadataKeywordsImportOptions struct {
	Dir           string
	Version       string
	Input         string
	Format        string
	DefaultLocale string
	DryRun        bool
	Overwrite     bool
}

type metadataKeywordsLocalizeOptions struct {
	Dir           string
	Version       string
	FromLocale    string
	TargetLocales []string
	DryRun        bool
	Overwrite     bool
}

type metadataKeywordsPlanOptions struct {
	AppID      string
	Version    string
	Platform   string
	Dir        string
	DryRun     bool
	Apply      bool
	Confirm    bool
	LocalState map[string]keywordLocalState
}

type keywordLocalState struct {
	locale string
	file   string
	full   VersionLocalization
	patch  versionLocalPatch
}

type keywordImportPayload struct {
	states map[string]keywordLocalState
	result MetadataKeywordsImportResult
}

// MetadataKeywordsCommand returns the canonical metadata keywords subtree.
func MetadataKeywordsCommand() *ffcli.Command {
	fs := flag.NewFlagSet("keywords", flag.ExitOnError)

	return &ffcli.Command{
		Name:       "keywords",
		ShortUsage: "asc metadata keywords <subcommand> [flags]",
		ShortHelp:  "Manage canonical version-localization keyword metadata.",
		LongHelp: `Manage canonical version-localization keyword metadata.

This workflow manages the canonical version-localization ` + "`keywords`" + ` field
inside ` + "`./metadata/version/<version>/<locale>.json`" + ` files.

It does not front the raw App Store Connect ` + "`searchKeywords`" + `
relationship APIs. Those low-level surfaces remain available under:
  - ` + "`asc apps search-keywords ...`" + `
  - ` + "`asc localizations search-keywords ...`" + `

Examples:
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords localize --dir "./metadata" --version "1.2.3" --from-locale "en-US" --to-locales "fr-FR,de-DE"
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --dir "./metadata" --confirm
  asc metadata keywords sync --app "APP_ID" --version "1.2.3" --dir "./metadata" --input "./keywords.json" --format json --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{
			MetadataKeywordsImportCommand(),
			MetadataKeywordsPlanCommand(),
			MetadataKeywordsDiffCommand(),
			MetadataKeywordsLocalizeCommand(),
			MetadataKeywordsApplyCommand(),
			MetadataKeywordsSyncCommand(),
		},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// MetadataKeywordsImportCommand returns the keywords import subcommand.
func MetadataKeywordsImportCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords import", flag.ExitOnError)

	dir := fs.String("dir", "", "Metadata root directory (required)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	input := fs.String("input", "", "Import file path or - for stdin (required)")
	format := fs.String("format", keywordImportFormatAuto, "Input format: auto, csv, json, or text")
	locale := fs.String("locale", "", "Default locale for inputs without a locale column/field")
	dryRun := fs.Bool("dry-run", false, "Preview local file changes without writing files")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "import",
		ShortUsage: "asc metadata keywords import --dir \"./metadata\" --version \"1.2.3\" --input \"./keywords.csv\" [flags]",
		ShortHelp:  "Import provider keyword exports into canonical metadata files.",
		LongHelp: `Import provider keyword exports into canonical metadata files.

Supported input formats:
  - csv: header-based rows with locale + keywords/keyword/term columns
  - json: locale-keyed maps, arrays of localization objects, or a single localization object
  - text: a plain comma/newline-separated keyword list (requires --locale)

Examples:
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --locale "en-US" --input "./keywords.csv"
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --format json --input "./keywords.json"
  asc metadata keywords import --dir "./metadata" --version "1.2.3" --format text --locale "fr-FR" --input "./keywords.txt" --dry-run`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords import does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsImport(metadataKeywordsImportOptions{
				Dir:           *dir,
				Version:       *version,
				Input:         *input,
				Format:        *format,
				DefaultLocale: *locale,
				DryRun:        *dryRun,
				Overwrite:     true,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords import: %w", err)
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					return printMetadataKeywordFileResultTable("Keyword Import", result.Results, result.Dir, result.Version, result.DryRun)
				},
				func() error {
					return printMetadataKeywordFileResultMarkdown("Keyword Import", result.Results, result.Dir, result.Version, result.DryRun)
				},
			)
		},
	}
}

// MetadataKeywordsPlanCommand returns the keywords plan subcommand.
func MetadataKeywordsPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords plan", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc metadata keywords plan --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [flags]",
		ShortHelp:  "Preview keyword-only changes against App Store Connect.",
		LongHelp: `Preview keyword-only changes against App Store Connect.

This command reads local canonical metadata files, looks only at the version
localization ` + "`keywords`" + ` field, and builds a non-mutating plan.

Examples:
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords plan --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords plan does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:    *appID,
				Version:  *version,
				Platform: *platform,
				Dir:      *dir,
				DryRun:   true,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords plan: %w", err)
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsPlanTable(result) },
				func() error { return printMetadataKeywordsPlanMarkdown(result) },
			)
		},
	}
}

// MetadataKeywordsDiffCommand returns the keywords diff subcommand.
func MetadataKeywordsDiffCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords diff", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "diff",
		ShortUsage: "asc metadata keywords diff --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" [flags]",
		ShortHelp:  "Diff local canonical keywords against App Store Connect.",
		LongHelp: `Diff local canonical keywords against App Store Connect.

This is a keyword-focused alias of the planning flow, intended for human review
of local-vs-remote keyword changes before apply.

Examples:
  asc metadata keywords diff --app "APP_ID" --version "1.2.3" --dir "./metadata"
  asc metadata keywords diff --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata"`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords diff does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:    *appID,
				Version:  *version,
				Platform: *platform,
				Dir:      *dir,
				DryRun:   true,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords diff: %w", err)
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsPlanTable(result) },
				func() error { return printMetadataKeywordsPlanMarkdown(result) },
			)
		},
	}
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
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					return printMetadataKeywordFileResultTable("Keyword Localize", result.Results, result.Dir, result.Version, result.DryRun)
				},
				func() error {
					return printMetadataKeywordFileResultMarkdown("Keyword Localize", result.Results, result.Dir, result.Version, result.DryRun)
				},
			)
		},
	}
}

// MetadataKeywordsApplyCommand returns the keywords apply subcommand.
func MetadataKeywordsApplyCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords apply", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	confirm := fs.Bool("confirm", false, "Confirm remote keyword mutations")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "apply",
		ShortUsage: "asc metadata keywords apply --app \"APP_ID\" --version \"1.2.3\" --dir \"./metadata\" --confirm [flags]",
		ShortHelp:  "Apply keyword-only metadata changes to App Store Connect.",
		LongHelp: `Apply keyword-only metadata changes to App Store Connect.

This command mutates only the version-localization ` + "`keywords`" + ` field.
Other version metadata fields remain untouched by updates performed here.

Examples:
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --dir "./metadata" --confirm
  asc metadata keywords apply --app "APP_ID" --version "1.2.3" --platform IOS --dir "./metadata" --confirm`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata keywords apply does not accept positional arguments")
			}
			result, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:    *appID,
				Version:  *version,
				Platform: *platform,
				Dir:      *dir,
				DryRun:   false,
				Apply:    true,
				Confirm:  *confirm,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords apply: %w", err)
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsPlanTable(result) },
				func() error { return printMetadataKeywordsPlanMarkdown(result) },
			)
		},
	}
}

// MetadataKeywordsSyncCommand returns the keywords sync subcommand.
func MetadataKeywordsSyncCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata keywords sync", flag.ExitOnError)

	appID := fs.String("app", "", "App Store Connect app ID (or ASC_APP_ID env)")
	version := fs.String("version", "", "App version string (for example 1.2.3)")
	platform := fs.String("platform", "", "Optional platform: IOS, MAC_OS, TV_OS, or VISION_OS")
	dir := fs.String("dir", "", "Metadata root directory (required)")
	input := fs.String("input", "", "Import file path or - for stdin (required)")
	format := fs.String("format", keywordImportFormatAuto, "Input format: auto, csv, json, or text")
	locale := fs.String("locale", "", "Default locale for inputs without a locale column/field")
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
			importPayload, err := executeMetadataKeywordsImportWithState(metadataKeywordsImportOptions{
				Dir:           *dir,
				Version:       *version,
				Input:         *input,
				Format:        *format,
				DefaultLocale: *locale,
				DryRun:        *dryRun,
				Overwrite:     true,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords sync: %w", err)
			}

			planResult, err := executeMetadataKeywordsPlan(ctx, metadataKeywordsPlanOptions{
				AppID:      *appID,
				Version:    *version,
				Platform:   *platform,
				Dir:        *dir,
				DryRun:     *dryRun || !*confirm,
				Apply:      !*dryRun && *confirm,
				Confirm:    *confirm,
				LocalState: importPayload.states,
			})
			if err != nil {
				if errors.Is(err, flag.ErrHelp) {
					return err
				}
				return fmt.Errorf("metadata keywords sync: %w", err)
			}

			result := MetadataKeywordsSyncResult{
				Import: importPayload.result,
				Plan:   planResult,
			}
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printMetadataKeywordsSyncTable(result) },
				func() error { return printMetadataKeywordsSyncMarkdown(result) },
			)
		},
	}
}

func executeMetadataKeywordsImport(opts metadataKeywordsImportOptions) (MetadataKeywordsImportResult, error) {
	payload, err := executeMetadataKeywordsImportWithState(opts)
	if err != nil {
		return MetadataKeywordsImportResult{}, err
	}
	return payload.result, nil
}

func executeMetadataKeywordsImportWithState(opts metadataKeywordsImportOptions) (keywordImportPayload, error) {
	dirValue, versionValue, err := validateMetadataKeywordDirVersion(opts.Dir, opts.Version)
	if err != nil {
		return keywordImportPayload{}, err
	}
	inputValue := strings.TrimSpace(opts.Input)
	if inputValue == "" {
		return keywordImportPayload{}, shared.UsageError("--input is required")
	}
	formatValue, err := resolveMetadataKeywordImportFormat(inputValue, opts.Format)
	if err != nil {
		return keywordImportPayload{}, shared.UsageError(err.Error())
	}

	imported, err := readMetadataKeywordImportInput(inputValue, formatValue, strings.TrimSpace(opts.DefaultLocale))
	if err != nil {
		return keywordImportPayload{}, err
	}

	states, results, plans, err := buildMetadataKeywordWriteResults(dirValue, versionValue, imported, opts.Overwrite)
	if err != nil {
		return keywordImportPayload{}, err
	}
	if !opts.DryRun {
		if err := ApplyWritePlans(plans); err != nil {
			return keywordImportPayload{}, err
		}
	}

	return keywordImportPayload{
		states: states,
		result: MetadataKeywordsImportResult{
			Dir:     dirValue,
			Version: versionValue,
			Input:   inputValue,
			Format:  formatValue,
			DryRun:  opts.DryRun,
			Results: results,
		},
	}, nil
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

	sourcePath, err := VersionLocalizationFilePath(dirValue, versionValue, sourceLocale)
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

	_, results, plans, err := buildMetadataKeywordWriteResults(dirValue, versionValue, valuesByLocale, opts.Overwrite)
	if err != nil {
		return MetadataKeywordsLocalizeResult{}, err
	}
	if !opts.DryRun {
		if err := ApplyWritePlans(plans); err != nil {
			return MetadataKeywordsLocalizeResult{}, err
		}
	}

	return MetadataKeywordsLocalizeResult{
		Dir:        dirValue,
		Version:    versionValue,
		FromLocale: sourceLocale,
		DryRun:     opts.DryRun,
		Results:    results,
	}, nil
}

func executeMetadataKeywordsPlan(ctx context.Context, opts metadataKeywordsPlanOptions) (MetadataKeywordsPlanResult, error) {
	resolvedAppID := shared.ResolveAppID(opts.AppID)
	if resolvedAppID == "" {
		return MetadataKeywordsPlanResult{}, shared.UsageError("--app is required (or set ASC_APP_ID)")
	}

	dirValue, versionValue, err := validateMetadataKeywordDirVersion(opts.Dir, opts.Version)
	if err != nil {
		return MetadataKeywordsPlanResult{}, err
	}
	platformValue := strings.TrimSpace(opts.Platform)
	if platformValue != "" {
		normalizedPlatform, platformErr := shared.NormalizeAppStoreVersionPlatform(platformValue)
		if platformErr != nil {
			return MetadataKeywordsPlanResult{}, shared.UsageError(platformErr.Error())
		}
		platformValue = normalizedPlatform
	}

	localState := opts.LocalState
	if len(localState) == 0 {
		localState, err = loadMetadataKeywordLocalState(dirValue, versionValue)
		if err != nil {
			return MetadataKeywordsPlanResult{}, err
		}
	}

	client, err := shared.GetASCClient()
	if err != nil {
		return MetadataKeywordsPlanResult{}, fmt.Errorf("auth: %w", err)
	}

	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()

	versionIDValue, _, err := resolveVersionID(requestCtx, client, resolvedAppID, versionValue, platformValue)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return MetadataKeywordsPlanResult{}, err
		}
		return MetadataKeywordsPlanResult{}, err
	}

	remoteVersionItems, err := fetchVersionLocalizations(requestCtx, client, versionIDValue)
	if err != nil {
		return MetadataKeywordsPlanResult{}, err
	}

	localPatches := keywordLocalStateToPatches(localState)
	filteredRemoteItems := filterKeywordRemoteItems(remoteVersionItems, localState)
	remoteVersion := keywordRemoteToVersionMap(filteredRemoteItems)
	adds, updates, _, versionCalls := buildScopePlan(
		versionDirName,
		versionValue,
		keywordPlanFields,
		versionToPlanFields(localPatches),
		versionToFieldMap(remoteVersion),
	)
	sortPlanItems(adds)
	sortPlanItems(updates)
	warnings := buildMetadataKeywordWarnings(localState, remoteVersion)

	result := MetadataKeywordsPlanResult{
		AppID:     resolvedAppID,
		Version:   versionValue,
		VersionID: versionIDValue,
		Dir:       dirValue,
		DryRun:    !opts.Apply || opts.DryRun,
		Adds:      adds,
		Updates:   updates,
		APICalls:  buildAPICallSummary(scopeCallCounts{}, versionCalls),
		Warnings:  warnings,
	}

	if !opts.Apply {
		return result, nil
	}
	if !opts.Confirm {
		return MetadataKeywordsPlanResult{}, shared.UsageError("--confirm is required")
	}

	actions, err := applyVersionChanges(requestCtx, client, versionIDValue, versionValue, localPatches, filteredRemoteItems, false)
	if err != nil {
		return MetadataKeywordsPlanResult{}, err
	}
	result.Applied = true
	result.DryRun = false
	result.Actions = actions
	return result, nil
}

func validateMetadataKeywordDirVersion(dir string, version string) (string, string, error) {
	dirValue := strings.TrimSpace(dir)
	if dirValue == "" {
		return "", "", shared.UsageError("--dir is required")
	}
	versionValue := strings.TrimSpace(version)
	if versionValue == "" {
		return "", "", shared.UsageError("--version is required")
	}
	return dirValue, versionValue, nil
}

func validateMetadataKeywordLocale(locale string) (string, error) {
	resolvedLocale, err := validateLocale(locale)
	if err != nil {
		return "", err
	}
	if resolvedLocale == DefaultLocale {
		return "", fmt.Errorf("default locale is not supported for metadata keywords")
	}
	return resolvedLocale, nil
}

func resolveMetadataKeywordImportFormat(inputPath string, format string) (string, error) {
	formatValue := strings.ToLower(strings.TrimSpace(format))
	if formatValue == "" {
		formatValue = keywordImportFormatAuto
	}
	switch formatValue {
	case keywordImportFormatCSV, keywordImportFormatJSON, keywordImportFormatText:
		return formatValue, nil
	case keywordImportFormatAuto:
		if strings.TrimSpace(inputPath) == "-" {
			return "", fmt.Errorf("--format is required when --input is -")
		}
		switch strings.ToLower(filepath.Ext(inputPath)) {
		case ".csv":
			return keywordImportFormatCSV, nil
		case ".json":
			return keywordImportFormatJSON, nil
		case ".txt", ".text":
			return keywordImportFormatText, nil
		default:
			return "", fmt.Errorf("could not infer input format from %q; use --format", inputPath)
		}
	default:
		return "", fmt.Errorf("--format must be one of auto, csv, json, or text")
	}
}

func readMetadataKeywordImportInput(inputPath, format, defaultLocale string) (map[string][]string, error) {
	data, err := readMetadataKeywordInputBytes(inputPath)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, shared.UsageError("import input is empty")
	}

	var raw map[string][]string
	switch format {
	case keywordImportFormatCSV:
		raw, err = parseMetadataKeywordCSV(data, defaultLocale)
	case keywordImportFormatJSON:
		raw, err = parseMetadataKeywordJSON(data, defaultLocale)
	case keywordImportFormatText:
		raw, err = parseMetadataKeywordText(data, defaultLocale)
	default:
		err = fmt.Errorf("unsupported import format %q", format)
	}
	if err != nil {
		return nil, err
	}
	return normalizeImportedMetadataKeywords(raw, defaultLocale)
}

func readMetadataKeywordInputBytes(inputPath string) ([]byte, error) {
	inputValue := strings.TrimSpace(inputPath)
	if inputValue == "-" {
		return io.ReadAll(os.Stdin)
	}
	return readFileNoFollow(inputValue)
}

func parseMetadataKeywordText(data []byte, defaultLocale string) (map[string][]string, error) {
	if strings.TrimSpace(defaultLocale) == "" {
		return nil, shared.UsageError("--locale is required for text input")
	}
	return map[string][]string{
		defaultLocale: splitMetadataKeywordTokens(string(data)),
	}, nil
}

func parseMetadataKeywordCSV(data []byte, defaultLocale string) (map[string][]string, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, shared.UsageErrorf("invalid csv input: %v", err)
	}
	if len(rows) == 0 {
		return nil, shared.UsageError("csv input is empty")
	}

	headerIndex := make(map[string]int, len(rows[0]))
	for idx, header := range rows[0] {
		headerIndex[normalizeMetadataKeywordHeader(header)] = idx
	}

	localeIdx, hasLocale := metadataKeywordHeaderIndex(headerIndex, "locale", "lang", "language")
	keywordsIdx, hasKeywords := metadataKeywordHeaderIndex(headerIndex, "keywords", "keywordfield", "keywordlist")
	keywordIdx, hasKeyword := metadataKeywordHeaderIndex(headerIndex, "keyword", "term", "searchterm", "searchkeyword")
	if !hasKeywords && !hasKeyword {
		return nil, shared.UsageError(`csv input requires a "keywords" or "keyword" column`)
	}

	result := make(map[string][]string)
	for rowIndex, row := range rows[1:] {
		if metadataKeywordCSVRowEmpty(row) {
			continue
		}

		rowLocale := strings.TrimSpace(defaultLocale)
		if hasLocale && localeIdx < len(row) {
			rowLocale = strings.TrimSpace(row[localeIdx])
			if rowLocale == "" {
				rowLocale = strings.TrimSpace(defaultLocale)
			}
		}
		if rowLocale == "" {
			return nil, shared.UsageErrorf("csv row %d is missing a locale (set --locale or add a locale column)", rowIndex+2)
		}

		var tokens []string
		if hasKeywords && keywordsIdx < len(row) {
			tokens = append(tokens, splitMetadataKeywordTokens(row[keywordsIdx])...)
		}
		if len(tokens) == 0 && hasKeyword && keywordIdx < len(row) {
			tokens = append(tokens, splitMetadataKeywordTokens(row[keywordIdx])...)
		}
		if len(tokens) == 0 {
			continue
		}
		result[rowLocale] = append(result[rowLocale], tokens...)
	}
	return result, nil
}

func parseMetadataKeywordJSON(data []byte, defaultLocale string) (map[string][]string, error) {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, shared.UsageErrorf("invalid json input: %v", err)
	}
	result, err := collectMetadataKeywordJSON(payload, strings.TrimSpace(defaultLocale))
	if err != nil {
		return nil, err
	}
	return result, nil
}

func collectMetadataKeywordJSON(payload any, defaultLocale string) (map[string][]string, error) {
	switch value := payload.(type) {
	case []any:
		result := make(map[string][]string)
		for idx, item := range value {
			locale, keywords, err := parseMetadataKeywordJSONObject(item, defaultLocale)
			if err != nil {
				return nil, shared.UsageErrorf("json item %d: %v", idx, err)
			}
			result[locale] = append(result[locale], keywords...)
		}
		return result, nil
	case map[string]any:
		if nested, ok := value["localizations"]; ok {
			return collectMetadataKeywordJSON(nested, defaultLocale)
		}
		if looksLikeMetadataKeywordLocalizationObject(value) {
			locale, keywords, err := parseMetadataKeywordJSONObject(value, defaultLocale)
			if err != nil {
				return nil, err
			}
			return map[string][]string{locale: keywords}, nil
		}

		result := make(map[string][]string)
		for rawLocale, rawKeywords := range value {
			if object, ok := rawKeywords.(map[string]any); ok && looksLikeMetadataKeywordLocalizationObject(object) {
				locale, keywords, err := parseMetadataKeywordJSONObject(object, rawLocale)
				if err != nil {
					return nil, shared.UsageErrorf("json locale %q: %v", rawLocale, err)
				}
				result[locale] = append(result[locale], keywords...)
				continue
			}
			keywords, err := decodeMetadataKeywordValue(rawKeywords)
			if err != nil {
				return nil, shared.UsageErrorf("json locale %q: %v", rawLocale, err)
			}
			result[rawLocale] = append(result[rawLocale], keywords...)
		}
		return result, nil
	default:
		return nil, shared.UsageError("json input must be an object or array")
	}
}

func looksLikeMetadataKeywordLocalizationObject(value map[string]any) bool {
	if _, ok := value["locale"]; ok {
		return true
	}
	for key := range value {
		switch normalizeMetadataKeywordHeader(key) {
		case "keywords", "keywordfield", "keywordlist", "keyword", "term", "searchterm", "searchkeyword":
			return true
		}
	}
	return false
}

func parseMetadataKeywordJSONObject(raw any, defaultLocale string) (string, []string, error) {
	object, ok := raw.(map[string]any)
	if !ok {
		return "", nil, fmt.Errorf("expected object")
	}

	localeValue := strings.TrimSpace(defaultLocale)
	if localeRaw, ok := object["locale"]; ok {
		if localeString, ok := localeRaw.(string); ok {
			localeValue = strings.TrimSpace(localeString)
		} else {
			return "", nil, fmt.Errorf(`field "locale" must be a string`)
		}
	}
	if localeValue == "" {
		return "", nil, fmt.Errorf("locale is required")
	}

	for _, key := range []string{"keywords", "keywordField", "keywordList", "keyword", "term", "searchTerm", "searchKeyword"} {
		if rawValue, ok := object[key]; ok {
			keywords, err := decodeMetadataKeywordValue(rawValue)
			if err != nil {
				return "", nil, err
			}
			return localeValue, keywords, nil
		}
	}
	for rawKey, rawValue := range object {
		switch normalizeMetadataKeywordHeader(rawKey) {
		case "keywords", "keywordfield", "keywordlist", "keyword", "term", "searchterm", "searchkeyword":
			keywords, err := decodeMetadataKeywordValue(rawValue)
			if err != nil {
				return "", nil, err
			}
			return localeValue, keywords, nil
		}
	}
	return "", nil, fmt.Errorf("keywords field is required")
}

func decodeMetadataKeywordValue(raw any) ([]string, error) {
	switch value := raw.(type) {
	case string:
		return splitMetadataKeywordTokens(value), nil
	case []any:
		tokens := make([]string, 0, len(value))
		for idx, item := range value {
			itemString, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("keyword item %d must be a string", idx)
			}
			tokens = append(tokens, splitMetadataKeywordTokens(itemString)...)
		}
		return tokens, nil
	default:
		return nil, fmt.Errorf("keywords must be a string or string array")
	}
}

func normalizeImportedMetadataKeywords(raw map[string][]string, defaultLocale string) (map[string][]string, error) {
	result := make(map[string][]string, len(raw))
	for locale, keywords := range raw {
		resolvedLocale := strings.TrimSpace(locale)
		if resolvedLocale == "" {
			resolvedLocale = strings.TrimSpace(defaultLocale)
		}
		if resolvedLocale == "" {
			return nil, shared.UsageError("locale is required for imported keywords")
		}

		canonicalLocale, err := validateMetadataKeywordLocale(resolvedLocale)
		if err != nil {
			return nil, shared.UsageError(err.Error())
		}

		normalizedKeywords, err := normalizeMetadataKeywordList(keywords)
		if err != nil {
			return nil, shared.UsageErrorf("locale %q: %v", canonicalLocale, err)
		}
		result[canonicalLocale] = append(result[canonicalLocale], normalizedKeywords...)
	}

	for locale, keywords := range result {
		normalizedKeywords, err := normalizeMetadataKeywordList(keywords)
		if err != nil {
			return nil, shared.UsageErrorf("locale %q: %v", locale, err)
		}
		result[locale] = normalizedKeywords
	}
	if len(result) == 0 {
		return nil, shared.UsageError("no keywords were found in the import input")
	}
	return result, nil
}

func buildMetadataKeywordWriteResults(dir, version string, valuesByLocale map[string][]string, overwrite bool) (map[string]keywordLocalState, []MetadataKeywordFileResult, []WritePlan, error) {
	locales := make([]string, 0, len(valuesByLocale))
	for locale := range valuesByLocale {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	states := make(map[string]keywordLocalState, len(locales))
	results := make([]MetadataKeywordFileResult, 0, len(locales))
	plans := make([]WritePlan, 0, len(locales))

	for _, locale := range locales {
		path, err := VersionLocalizationFilePath(dir, version, locale)
		if err != nil {
			return nil, nil, nil, err
		}
		keywordField, keywordCount, err := buildMetadataKeywordField(valuesByLocale[locale])
		if err != nil {
			return nil, nil, nil, shared.UsageErrorf("locale %q: %v", locale, err)
		}

		existing, exists, err := readExistingVersionLocalization(path)
		if err != nil {
			return nil, nil, nil, err
		}

		result := MetadataKeywordFileResult{
			Locale:       locale,
			File:         path,
			KeywordField: keywordField,
			KeywordCount: keywordCount,
		}

		switch {
		case exists && strings.TrimSpace(existing.Keywords) == keywordField:
			result.Action = "noop"
			result.Reason = "keywords already match canonical file"
			states[locale] = buildMetadataKeywordLocalState(locale, path, existing)
		case exists && strings.TrimSpace(existing.Keywords) != "" && !overwrite:
			result.Action = "skip"
			result.Reason = "existing keywords preserved (use --overwrite to replace)"
			states[locale] = buildMetadataKeywordLocalState(locale, path, existing)
		default:
			next := existing
			next.Keywords = keywordField
			result.Action = "update"
			if !exists {
				result.Action = "create"
			}
			result.Reason = "canonical keyword field updated"
			states[locale] = buildMetadataKeywordLocalState(locale, path, next)
			data, err := EncodeVersionLocalization(next)
			if err != nil {
				return nil, nil, nil, err
			}
			plans = append(plans, WritePlan{Path: path, Contents: data})
		}
		results = append(results, result)
	}
	return states, results, plans, nil
}

func buildMetadataKeywordLocalState(locale, path string, localization VersionLocalization) keywordLocalState {
	normalized := NormalizeVersionLocalization(localization)
	return keywordLocalState{
		locale: locale,
		file:   path,
		full:   normalized,
		patch: versionLocalPatch{
			localization: NormalizeVersionLocalization(VersionLocalization{Keywords: normalized.Keywords}),
			setFields:    map[string]string{"keywords": normalized.Keywords},
		},
	}
}

func readExistingVersionLocalization(path string) (VersionLocalization, bool, error) {
	localization, err := ReadVersionLocalizationFile(path)
	if err == nil {
		return localization, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return VersionLocalization{}, false, nil
	}
	return VersionLocalization{}, false, err
}

func loadMetadataKeywordLocalState(dir, version string) (map[string]keywordLocalState, error) {
	resolvedVersion, err := validatePathSegment("version", version)
	if err != nil {
		return nil, shared.UsageError(err.Error())
	}
	versionPath := filepath.Join(strings.TrimSpace(dir), versionDirName, resolvedVersion)
	entries, err := os.ReadDir(versionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, shared.UsageErrorf("no version localization files found in %s", versionPath)
		}
		return nil, fmt.Errorf("failed to read %s: %w", versionPath, err)
	}

	states := make(map[string]keywordLocalState)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		rawLocale := strings.TrimSuffix(entry.Name(), ".json")
		if strings.EqualFold(rawLocale, DefaultLocale) {
			continue
		}
		locale, localeErr := validateMetadataKeywordLocale(rawLocale)
		if localeErr != nil {
			return nil, shared.UsageErrorf("invalid metadata keywords file %q: %v", entry.Name(), localeErr)
		}

		path := filepath.Join(versionPath, entry.Name())
		localization, readErr := ReadVersionLocalizationFile(path)
		if readErr != nil {
			return nil, shared.UsageErrorf("invalid metadata schema in %s: %v", path, readErr)
		}
		if strings.TrimSpace(localization.Keywords) == "" {
			continue
		}
		states[locale] = buildMetadataKeywordLocalState(locale, path, localization)
	}
	if len(states) == 0 {
		return nil, shared.UsageErrorf("no keyword metadata files with a non-empty keywords field were found in %s", versionPath)
	}
	return states, nil
}

func keywordLocalStateToPatches(states map[string]keywordLocalState) map[string]versionLocalPatch {
	result := make(map[string]versionLocalPatch, len(states))
	for locale, state := range states {
		result[locale] = cloneVersionLocalPatch(state.patch)
	}
	return result
}

func filterKeywordRemoteItems(items []asc.Resource[asc.AppStoreVersionLocalizationAttributes], states map[string]keywordLocalState) []asc.Resource[asc.AppStoreVersionLocalizationAttributes] {
	filtered := make([]asc.Resource[asc.AppStoreVersionLocalizationAttributes], 0, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		if _, ok := states[locale]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func keywordRemoteToVersionMap(items []asc.Resource[asc.AppStoreVersionLocalizationAttributes]) map[string]VersionLocalization {
	result := make(map[string]VersionLocalization, len(items))
	for _, item := range items {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		result[locale] = NormalizeVersionLocalization(VersionLocalization{
			Description:     item.Attributes.Description,
			Keywords:        item.Attributes.Keywords,
			MarketingURL:    item.Attributes.MarketingURL,
			PromotionalText: item.Attributes.PromotionalText,
			SupportURL:      item.Attributes.SupportURL,
			WhatsNew:        item.Attributes.WhatsNew,
		})
	}
	return result
}

func buildMetadataKeywordWarnings(states map[string]keywordLocalState, remote map[string]VersionLocalization) []MetadataKeywordsWarning {
	locales := make([]string, 0, len(states))
	for locale := range states {
		locales = append(locales, locale)
	}
	sort.Strings(locales)

	warnings := make([]MetadataKeywordsWarning, 0)
	for _, locale := range locales {
		if _, exists := remote[locale]; exists {
			continue
		}
		state := states[locale]
		missing := shared.MissingSubmitRequiredLocalizationFields(versionAttributes(locale, state.full, true))
		if len(missing) == 0 {
			continue
		}
		warnings = append(warnings, MetadataKeywordsWarning{
			Locale:        locale,
			Message:       fmt.Sprintf("creating locale %q leaves submit-required fields missing", locale),
			MissingFields: missing,
		})
	}
	return warnings
}

func metadataKeywordHeaderIndex(headers map[string]int, names ...string) (int, bool) {
	for _, name := range names {
		if idx, ok := headers[name]; ok {
			return idx, true
		}
	}
	return 0, false
}

func normalizeMetadataKeywordHeader(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, "_", "")
	return normalized
}

func metadataKeywordCSVRowEmpty(row []string) bool {
	for _, item := range row {
		if strings.TrimSpace(item) != "" {
			return false
		}
	}
	return true
}

func splitMetadataKeywordTokens(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == '\n' || r == '\r'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(part)), " ")
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}

func normalizeMetadataKeywordList(keywords []string) ([]string, error) {
	normalized := make([]string, 0, len(keywords))
	seen := make(map[string]struct{}, len(keywords))
	for _, keyword := range keywords {
		trimmed := strings.Join(strings.Fields(strings.TrimSpace(keyword)), " ")
		if trimmed == "" {
			continue
		}
		folded := strings.ToLower(trimmed)
		if _, ok := seen[folded]; ok {
			continue
		}
		seen[folded] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("no non-empty keywords were found")
	}
	return normalized, nil
}

func buildMetadataKeywordField(keywords []string) (string, int, error) {
	normalized, err := normalizeMetadataKeywordList(keywords)
	if err != nil {
		return "", 0, err
	}
	field := strings.Join(normalized, ",")
	if utf8.RuneCountInString(field) > validation.LimitKeywords {
		return "", 0, fmt.Errorf("keywords exceed %d characters", validation.LimitKeywords)
	}
	return field, len(normalized), nil
}

func parseMetadataKeywordField(field string) ([]string, error) {
	normalized, err := normalizeMetadataKeywordList(splitMetadataKeywordTokens(field))
	if err != nil {
		return nil, err
	}
	joined := strings.Join(normalized, ",")
	if utf8.RuneCountInString(joined) > validation.LimitKeywords {
		return nil, fmt.Errorf("keywords exceed %d characters", validation.LimitKeywords)
	}
	return normalized, nil
}

func printMetadataKeywordFileResultTable(title string, results []MetadataKeywordFileResult, dir string, version string, dryRun bool) error {
	fmt.Printf("%s\n", title)
	fmt.Printf("Dir: %s\n", dir)
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Dry Run: %t\n\n", dryRun)

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Reason,
			fmt.Sprintf("%d", result.KeywordCount),
			sanitizePlanCell(result.KeywordField),
			result.File,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", "none", "no changes", "0", "", ""})
	}
	asc.RenderTable([]string{"locale", "action", "reason", "count", "keywords", "file"}, rows)
	return nil
}

func printMetadataKeywordFileResultMarkdown(title string, results []MetadataKeywordFileResult, dir string, version string, dryRun bool) error {
	fmt.Printf("## %s\n\n", title)
	fmt.Printf("**Dir:** %s\n\n", dir)
	fmt.Printf("**Version:** %s\n\n", version)
	fmt.Printf("**Dry Run:** %t\n\n", dryRun)

	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			result.Locale,
			result.Action,
			result.Reason,
			fmt.Sprintf("%d", result.KeywordCount),
			sanitizePlanCell(result.KeywordField),
			result.File,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"", "none", "no changes", "0", "", ""})
	}
	asc.RenderMarkdown([]string{"locale", "action", "reason", "count", "keywords", "file"}, rows)
	return nil
}

func printMetadataKeywordsPlanTable(result MetadataKeywordsPlanResult) error {
	fmt.Printf("App ID: %s\n", result.AppID)
	fmt.Printf("Version: %s\n", result.Version)
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Dry Run: %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("Applied: %t\n\n", result.Applied)
	}

	pushResult := PushPlanResult{
		AppID:    result.AppID,
		Version:  result.Version,
		Dir:      result.Dir,
		DryRun:   result.DryRun,
		Applied:  result.Applied,
		Adds:     result.Adds,
		Updates:  result.Updates,
		APICalls: result.APICalls,
		Actions:  result.Actions,
	}
	asc.RenderTable([]string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}, buildPlanRows(pushResult))
	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	if len(result.Warnings) > 0 {
		fmt.Println()
		asc.RenderTable([]string{"locale", "message", "missingFields"}, buildMetadataKeywordWarningRows(result.Warnings))
	}
	return nil
}

func printMetadataKeywordsPlanMarkdown(result MetadataKeywordsPlanResult) error {
	fmt.Printf("**App ID:** %s\n\n", result.AppID)
	fmt.Printf("**Version:** %s\n\n", result.Version)
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Dry Run:** %t\n\n", result.DryRun)
	if result.Applied {
		fmt.Printf("**Applied:** %t\n\n", result.Applied)
	}

	pushResult := PushPlanResult{
		AppID:    result.AppID,
		Version:  result.Version,
		Dir:      result.Dir,
		DryRun:   result.DryRun,
		Applied:  result.Applied,
		Adds:     result.Adds,
		Updates:  result.Updates,
		APICalls: result.APICalls,
		Actions:  result.Actions,
	}
	asc.RenderMarkdown([]string{"change", "key", "scope", "locale", "version", "field", "reason", "from", "to"}, buildPlanRows(pushResult))
	if len(result.APICalls) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"operation", "scope", "count"}, buildAPICallRows(result.APICalls))
	}
	if len(result.Actions) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"scope", "locale", "version", "action", "localizationId"}, buildApplyActionRows(result.Actions))
	}
	if len(result.Warnings) > 0 {
		fmt.Println()
		asc.RenderMarkdown([]string{"locale", "message", "missingFields"}, buildMetadataKeywordWarningRows(result.Warnings))
	}
	return nil
}

func buildMetadataKeywordWarningRows(warnings []MetadataKeywordsWarning) [][]string {
	rows := make([][]string, 0, len(warnings))
	for _, warning := range warnings {
		rows = append(rows, []string{
			warning.Locale,
			warning.Message,
			strings.Join(warning.MissingFields, ","),
		})
	}
	return rows
}

func printMetadataKeywordsSyncTable(result MetadataKeywordsSyncResult) error {
	if err := printMetadataKeywordFileResultTable("Keyword Import", result.Import.Results, result.Import.Dir, result.Import.Version, result.Import.DryRun); err != nil {
		return err
	}
	fmt.Println()
	return printMetadataKeywordsPlanTable(result.Plan)
}

func printMetadataKeywordsSyncMarkdown(result MetadataKeywordsSyncResult) error {
	if err := printMetadataKeywordFileResultMarkdown("Keyword Import", result.Import.Results, result.Import.Dir, result.Import.Version, result.Import.DryRun); err != nil {
		return err
	}
	fmt.Println()
	return printMetadataKeywordsPlanMarkdown(result.Plan)
}
