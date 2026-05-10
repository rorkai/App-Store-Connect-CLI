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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// InitResult is the structured output artifact for metadata init.
type InitResult struct {
	Dir       string   `json:"dir"`
	Locale    string   `json:"locale"`
	Version   string   `json:"version,omitempty"`
	FileCount int      `json:"fileCount"`
	Files     []string `json:"files"`
	NextSteps []string `json:"nextSteps,omitempty"`
}

type appInfoLocalizationTemplate struct {
	Name              string `json:"name"`
	Subtitle          string `json:"subtitle"`
	PrivacyPolicyURL  string `json:"privacyPolicyUrl"`
	PrivacyChoicesURL string `json:"privacyChoicesUrl"`
	PrivacyPolicyText string `json:"privacyPolicyText"`
}

type versionLocalizationTemplate struct {
	Description     string `json:"description"`
	Keywords        string `json:"keywords"`
	MarketingURL    string `json:"marketingUrl"`
	PromotionalText string `json:"promotionalText"`
	SupportURL      string `json:"supportUrl"`
	WhatsNew        string `json:"whatsNew"`
}

// MetadataInitCommand returns the metadata init subcommand.
func MetadataInitCommand() *ffcli.Command {
	fs := flag.NewFlagSet("metadata init", flag.ExitOnError)

	dir := fs.String("dir", "", "Metadata root directory (required)")
	version := fs.String("version", "", "Optional app version string; when set, writes a version localization template")
	locale := fs.String("locale", "en-US", "Metadata locale to scaffold")
	force := fs.Bool("force", false, "Overwrite existing metadata template files in --dir")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "init",
		ShortUsage: "asc metadata init --dir \"./metadata\" [--version \"1.2.3\"] [--locale \"en-US\"] [flags]",
		ShortHelp:  "Create canonical metadata template files.",
		LongHelp: `Create canonical metadata template files.

The generated files include blank string values for every supported Phase 1
metadata field. Fill the values you want to apply and remove keys you want to
leave unset before running validate, apply, or push.

Examples:
  asc metadata init --dir "./metadata"
  asc metadata init --dir "./metadata" --locale "en-US"
  asc metadata init --dir "./metadata" --version "1.2.3" --locale "en-US"
  asc metadata init --dir "./metadata" --version "1.2.3" --locale "en-US" --force`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("metadata init does not accept positional arguments")
			}

			dirValue := strings.TrimSpace(*dir)
			if dirValue == "" {
				return shared.UsageError("--dir is required")
			}

			localeValue, err := validateLocale(*locale)
			if err != nil {
				return shared.UsageError(err.Error())
			}

			versionValue := strings.TrimSpace(*version)
			if versionValue != "" {
				var versionErr error
				versionValue, versionErr = validatePathSegment("version", versionValue)
				if versionErr != nil {
					return shared.UsageError(versionErr.Error())
				}
			}

			plans, err := BuildInitWritePlans(dirValue, localeValue, versionValue)
			if err != nil {
				return fmt.Errorf("metadata init: %w", err)
			}
			if !*force {
				if err := ensureNoExistingInitTargets(plans); err != nil {
					return err
				}
			}
			if err := ApplyWritePlans(plans); err != nil {
				return fmt.Errorf("metadata init: %w", err)
			}

			files := make([]string, 0, len(plans))
			for _, plan := range plans {
				files = append(files, plan.Path)
			}
			sort.Strings(files)

			result := InitResult{
				Dir:       dirValue,
				Locale:    localeValue,
				Version:   versionValue,
				FileCount: len(files),
				Files:     files,
				NextSteps: []string{
					"Fill values you want to apply.",
					"Remove keys you want to leave unset.",
					"Run asc metadata validate --dir \"" + dirValue + "\" before applying.",
				},
			}

			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error { return printInitResultTable(result) },
				func() error { return printInitResultMarkdown(result) },
			)
		},
	}
}

// BuildInitWritePlans creates deterministic write plans for blank metadata templates.
func BuildInitWritePlans(rootDir, locale, version string) ([]WritePlan, error) {
	resolvedLocale, err := validateLocale(locale)
	if err != nil {
		return nil, err
	}

	appInfoPath, err := AppInfoLocalizationFilePath(rootDir, resolvedLocale)
	if err != nil {
		return nil, err
	}
	appInfoData, err := encodeCanonicalJSON(appInfoLocalizationTemplate{})
	if err != nil {
		return nil, err
	}

	plans := []WritePlan{{Path: appInfoPath, Contents: appInfoData}}

	versionValue := strings.TrimSpace(version)
	if versionValue != "" {
		resolvedVersion, err := validatePathSegment("version", versionValue)
		if err != nil {
			return nil, err
		}
		versionPath, err := VersionLocalizationFilePath(rootDir, resolvedVersion, resolvedLocale)
		if err != nil {
			return nil, err
		}
		versionData, err := encodeCanonicalJSON(versionLocalizationTemplate{})
		if err != nil {
			return nil, err
		}
		plans = append(plans, WritePlan{Path: versionPath, Contents: versionData})
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Path < plans[j].Path
	})
	return plans, nil
}

func ensureNoExistingInitTargets(plans []WritePlan) error {
	for _, plan := range plans {
		if _, err := os.Lstat(plan.Path); err == nil {
			return shared.UsageErrorf("refusing to overwrite existing file %s (use --force)", plan.Path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("metadata init: failed to inspect %s: %w", plan.Path, err)
		}
	}
	return nil
}

func printInitResultTable(result InitResult) error {
	fmt.Printf("Dir: %s\n", result.Dir)
	fmt.Printf("Locale: %s\n", result.Locale)
	if result.Version != "" {
		fmt.Printf("Version: %s\n", result.Version)
	}
	fmt.Printf("File Count: %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderTable([]string{"file"}, rows)

	if len(result.NextSteps) > 0 {
		fmt.Println()
		stepRows := make([][]string, 0, len(result.NextSteps))
		for i, step := range result.NextSteps {
			stepRows = append(stepRows, []string{fmt.Sprintf("%d", i+1), step})
		}
		asc.RenderTable([]string{"step", "next"}, stepRows)
	}
	return nil
}

func printInitResultMarkdown(result InitResult) error {
	fmt.Printf("**Dir:** %s\n\n", result.Dir)
	fmt.Printf("**Locale:** %s\n\n", result.Locale)
	if result.Version != "" {
		fmt.Printf("**Version:** %s\n\n", result.Version)
	}
	fmt.Printf("**File Count:** %d\n\n", result.FileCount)

	rows := make([][]string, 0, len(result.Files))
	for _, file := range result.Files {
		rows = append(rows, []string{file})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"(none)"})
	}
	asc.RenderMarkdown([]string{"file"}, rows)

	if len(result.NextSteps) > 0 {
		fmt.Println()
		stepRows := make([][]string, 0, len(result.NextSteps))
		for i, step := range result.NextSteps {
			stepRows = append(stepRows, []string{fmt.Sprintf("%d", i+1), step})
		}
		asc.RenderMarkdown([]string{"step", "next"}, stepRows)
	}
	return nil
}
