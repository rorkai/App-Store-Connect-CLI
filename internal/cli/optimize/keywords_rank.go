package optimize

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

const keywordRankSchemaVersion = "1"

// KeywordsRankCommand returns the public keyword ranking command.
func KeywordsRankCommand() *ffcli.Command {
	fs := flag.NewFlagSet("rank", flag.ExitOnError)
	appID := fs.String("app", "", "App Store app ID (required, or ASC_APP_ID env)")
	keywords := shared.BindOnceCSVFlag(fs, "keywords", "Comma-separated keyword candidates to rank (required)")
	country := fs.String("country", "us", "ISO alpha-2 App Store storefront country or region")
	platform := fs.String("platform", "IOS", "Public App Store platform: IOS or TV_OS")
	workers := fs.Int("workers", 10, "Number of parallel keyword lookups")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "rank",
		ShortUsage: "asc optimize keywords rank --app APP_ID --keywords LIST [flags]",
		ShortHelp:  "Rank an app for keyword candidates in a public storefront.",
		LongHelp: `Report where an app appears in the public App Store search result window
for each keyword candidate.

No authentication is required. Keywords are normalized to lowercase, collapsed
whitespace, and deduplicated. Each invocation accepts at most 100 keywords of
2-60 characters and at most 4 space-separated words.
The effective worker count is capped at the normalized keyword count and is
recorded in the report.

A keyword whose lookup fails is reported as an unavailable row instead of being
dropped or scored as absent. The command fails only when every keyword lookup
fails. An empty status means the app is absent from the result window Apple
returned; it does not prove the app is absent from every storefront result.

TV_OS ranking depends on a numeric storefront ID, which Apple does not publish
for every country. The command rejects those storefronts before any request.

Examples:
  asc optimize keywords rank --app "1234567890" --keywords "focus timer,habit tracker"
  asc optimize keywords rank --app "1234567890" --keywords "focus timer" --country gb --output table
  asc optimize keywords rank --app "1234567890" --keywords "focus timer,meditation" --platform TV_OS --workers 4`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := normalizeKeywordAppID(*appID)
			if err != nil {
				return err
			}
			if strings.TrimSpace(keywords.String()) == "" {
				return shared.UsageError("--keywords is required")
			}
			normalizedKeywords, err := normalizeKeywordList(keywords.String())
			if err != nil {
				return err
			}
			if *workers < 1 {
				return shared.UsageError("--workers must be at least 1")
			}
			effectiveWorkers := *workers
			if effectiveWorkers > len(normalizedKeywords) {
				effectiveWorkers = len(normalizedKeywords)
			}
			normalizedCountry, err := normalizeKeywordCountry(*country)
			if err != nil {
				return err
			}
			normalizedPlatform, err := normalizeKeywordRankPlatform(*platform)
			if err != nil {
				return err
			}
			if normalizedPlatform == itunes.PublicSearchPlatformTVOS &&
				strings.TrimSpace(itunes.Storefronts[normalizedCountry]) == "" {
				return shared.UsageErrorf(
					"TV_OS ranking is unavailable for storefront %s",
					strings.ToUpper(normalizedCountry),
				)
			}

			client := newKeywordsItunesClient()
			results := fanOutKeywords(ctx, normalizedKeywords, effectiveWorkers, func(ctx context.Context, keyword string) (itunes.PublicRankResult, error) {
				requestCtx, cancel := shared.ContextWithTimeout(ctx)
				defer cancel()
				return client.RankApp(requestCtx, resolvedAppID, keyword, normalizedCountry, normalizedPlatform)
			})
			if err := ctx.Err(); err != nil {
				return err
			}

			report := buildKeywordRankReport(keywordRankBuildInput{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				AppID:       resolvedAppID,
				Country:     strings.ToUpper(normalizedCountry),
				Platform:    string(normalizedPlatform),
				Workers:     effectiveWorkers,
				Results:     results,
			})
			if report.Summary.Unavailable == report.Summary.Keywords {
				failures := make([]error, 0, len(results))
				for _, result := range results {
					failures = append(failures, result.Err)
				}
				return fmt.Errorf("optimize keywords rank: %w", representativeKeywordError(failures))
			}

			return shared.PrintOutput(&report, *output.Output, *output.Pretty)
		},
	}
}

type keywordRankBuildInput struct {
	GeneratedAt string
	AppID       string
	Country     string
	Platform    string
	Workers     int
	Results     []keywordFanOutResult[itunes.PublicRankResult]
}

func buildKeywordRankReport(input keywordRankBuildInput) asc.KeywordRankReport {
	rows := make([]asc.KeywordRankRow, 0, len(input.Results))
	summary := asc.KeywordRankSummary{Keywords: len(input.Results)}

	for _, result := range input.Results {
		row := asc.KeywordRankRow{Keyword: result.Keyword}
		switch {
		case result.Err != nil:
			row.Status = keywordStatusUnavailable
			row.Error = result.Err.Error()
			summary.Unavailable++
		case result.Value.Found:
			totalResults := result.Value.ResultCount
			row.Status = keywordStatusAvailable
			row.Rank = result.Value.Rank
			row.TotalResults = &totalResults
			summary.Ranked++
		default:
			totalResults := result.Value.ResultCount
			row.Status = keywordStatusEmpty
			row.TotalResults = &totalResults
			summary.Absent++
		}
		rows = append(rows, row)
	}

	return asc.KeywordRankReport{
		SchemaVersion: keywordRankSchemaVersion,
		GeneratedAt:   input.GeneratedAt,
		AppID:         input.AppID,
		Country:       input.Country,
		Platform:      input.Platform,
		Workers:       input.Workers,
		Summary:       summary,
		Rows:          rows,
	}
}

func normalizeKeywordRankPlatform(value string) (itunes.PublicSearchPlatform, error) {
	normalized := itunes.PublicSearchPlatform(strings.ToUpper(strings.TrimSpace(value)))
	switch normalized {
	case itunes.PublicSearchPlatformIOS, itunes.PublicSearchPlatformTVOS:
		return normalized, nil
	default:
		return "", shared.UsageError("--platform must be one of: IOS, TV_OS")
	}
}
