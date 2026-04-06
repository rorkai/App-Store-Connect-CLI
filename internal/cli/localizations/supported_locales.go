package localizations

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

type supportedLocaleEntry struct {
	Locale         string `json:"locale"`
	Name           string `json:"name,omitempty"`
	Configured     bool   `json:"configured"`
	LocalizationID string `json:"localizationId,omitempty"`
}

type supportedLocalesResult struct {
	VersionID          string                 `json:"versionId"`
	CatalogLocaleCount int                    `json:"catalogLocaleCount"`
	ConfiguredCount    int                    `json:"configuredCount"`
	Locales            []supportedLocaleEntry `json:"locales"`
}

// LocalizationsSupportedLocalesCommand returns the supported-locales subcommand.
func LocalizationsSupportedLocalesCommand() *ffcli.Command {
	fs := flag.NewFlagSet("supported-locales", flag.ExitOnError)

	versionID := fs.String("version", "", "App Store version ID (required)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "supported-locales",
		ShortUsage: "asc localizations supported-locales --version \"VERSION_ID\" [flags]",
		ShortHelp:  "List supported App Store localization locales for a version.",
		LongHelp: `List supported App Store localization locales from the shared CLI catalog for a version and show which ones are already configured.

Examples:
  asc localizations supported-locales --version "VERSION_ID"
  asc localizations supported-locales --version "VERSION_ID" --output json --pretty`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) > 0 {
				return shared.UsageError("localizations supported-locales does not accept positional arguments")
			}

			vid := strings.TrimSpace(*versionID)
			if vid == "" {
				fmt.Fprintln(os.Stderr, "Error: --version is required")
				return flag.ErrHelp
			}

			client, err := shared.GetASCClient()
			if err != nil {
				return fmt.Errorf("localizations supported-locales: %w", err)
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			firstPage, err := client.GetAppStoreVersionLocalizations(requestCtx, vid, asc.WithAppStoreVersionLocalizationsLimit(200))
			if err != nil {
				return fmt.Errorf("localizations supported-locales: failed to fetch configured localizations: %w", err)
			}

			paginated, err := asc.PaginateAll(requestCtx, firstPage, func(ctx context.Context, nextURL string) (asc.PaginatedResponse, error) {
				return client.GetAppStoreVersionLocalizations(ctx, vid, asc.WithAppStoreVersionLocalizationsNextURL(nextURL))
			})
			if err != nil {
				return fmt.Errorf("localizations supported-locales: paginate configured localizations: %w", err)
			}

			resp, ok := paginated.(*asc.AppStoreVersionLocalizationsResponse)
			if !ok {
				return fmt.Errorf("localizations supported-locales: unexpected localization response type %T", paginated)
			}

			result := buildSupportedLocalesResult(vid, resp)
			return shared.PrintOutputWithRenderers(
				result,
				*output.Output,
				*output.Pretty,
				func() error {
					renderSupportedLocales(result, false)
					return nil
				},
				func() error {
					renderSupportedLocales(result, true)
					return nil
				},
			)
		},
	}
}

func buildSupportedLocalesResult(versionID string, resp *asc.AppStoreVersionLocalizationsResponse) *supportedLocalesResult {
	catalog := shared.AppStoreLocalizationCatalog()
	configured := make(map[string]string, len(resp.Data))
	for _, item := range resp.Data {
		locale := strings.TrimSpace(item.Attributes.Locale)
		if locale == "" {
			continue
		}
		normalized, err := shared.NormalizeAppStoreLocalizationLocale(locale)
		if err == nil {
			locale = normalized
		}
		configured[locale] = item.ID
	}

	result := &supportedLocalesResult{
		VersionID: versionID,
		Locales:   make([]supportedLocaleEntry, 0, len(catalog)+len(configured)),
	}

	for _, locale := range catalog {
		localizationID, isConfigured := configured[locale.Code]
		result.Locales = append(result.Locales, supportedLocaleEntry{
			Locale:         locale.Code,
			Name:           locale.Name,
			Configured:     isConfigured,
			LocalizationID: localizationID,
		})
		if isConfigured {
			result.ConfiguredCount++
			delete(configured, locale.Code)
		}
	}

	if len(configured) > 0 {
		unknownLocales := make([]string, 0, len(configured))
		for locale := range configured {
			unknownLocales = append(unknownLocales, locale)
		}
		slices.Sort(unknownLocales)
		for _, locale := range unknownLocales {
			result.Locales = append(result.Locales, supportedLocaleEntry{
				Locale:         locale,
				Configured:     true,
				LocalizationID: configured[locale],
			})
			result.ConfiguredCount++
		}
	}

	result.CatalogLocaleCount = len(catalog)
	return result
}

func renderSupportedLocales(result *supportedLocalesResult, markdown bool) {
	shared.RenderSection(
		"Summary",
		[]string{"Version ID", "Catalog Locales", "Configured Locales"},
		[][]string{{
			result.VersionID,
			strconv.Itoa(result.CatalogLocaleCount),
			strconv.Itoa(result.ConfiguredCount),
		}},
		markdown,
	)

	headers, rows := supportedLocalesRows(result)
	shared.RenderSection("Supported Locales", headers, rows, markdown)
}

func supportedLocalesRows(result *supportedLocalesResult) ([]string, [][]string) {
	headers := []string{"Locale", "Language", "Configured", "Localization ID"}
	rows := make([][]string, 0, len(result.Locales))
	for _, item := range result.Locales {
		rows = append(rows, []string{
			item.Locale,
			shared.OrNA(item.Name),
			strconv.FormatBool(item.Configured),
			shared.OrNA(item.LocalizationID),
		})
	}
	return headers, rows
}
