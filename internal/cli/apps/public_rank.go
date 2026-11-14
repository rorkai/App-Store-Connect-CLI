package apps

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

type publicRankOutput struct {
	AppID       int64  `json:"appId"`
	Term        string `json:"term"`
	Country     string `json:"country"`
	Platform    string `json:"platform"`
	Found       bool   `json:"found"`
	Rank        *int   `json:"rank,omitempty"`
	ResultCount int    `json:"resultCount"`
}

// AppsPublicRankCommand returns the public app ranking command.
func AppsPublicRankCommand() *ffcli.Command {
	fs := flag.NewFlagSet("apps public rank", flag.ExitOnError)

	appID := fs.String("app", "", "Public App Store app ID")
	term := fs.String("term", "", "Search term")
	platform := fs.String("platform", "", "Search platform: IOS or TV_OS")
	country := fs.String("country", "us", "Storefront country code (ISO alpha-2, e.g. us, gb, de)")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "rank",
		ShortUsage: "asc apps public rank --app APP_ID --term QUERY --platform PLATFORM [--country CODE]",
		ShortHelp:  "Report an app's rank in a public App Store search window.",
		LongHelp: `Report an app's rank in a public App Store search result window.

No authentication is required.

Supported platforms are IOS and TV_OS. A not-found result means the app is
absent from the result window returned by Apple; it does not prove the app is
absent from every possible storefront result.

Examples:
  asc apps public rank --app "1234567890" --term "focus timer" --country us --platform TV_OS
  asc apps public rank --app "1234567890" --term "meditation" --country gb --platform tv_os --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := resolvePublicAppID(*appID)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}
			termValue := strings.TrimSpace(*term)
			if termValue == "" {
				return shared.RequiredFlag("--term")
			}
			platformValue, err := normalizePublicRankPlatform(*platform)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}
			normalizedCountry, err := normalizePublicCountry(*country)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error: "+err.Error())
				return flag.ErrHelp
			}
			if platformValue == itunes.PublicSearchPlatformTVOS && strings.TrimSpace(itunes.Storefronts[normalizedCountry]) == "" {
				fmt.Fprintf(os.Stderr, "Error: TV_OS ranking is unavailable for storefront %s\n", strings.ToUpper(normalizedCountry))
				return flag.ErrHelp
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()

			client := newPublicItunesClient()
			rankResult, err := client.RankApp(requestCtx, resolvedAppID, termValue, normalizedCountry, platformValue)
			if err != nil {
				return fmt.Errorf("apps public rank: %w", err)
			}
			numericAppID, err := strconv.ParseInt(resolvedAppID, 10, 64)
			if err != nil {
				return fmt.Errorf("apps public rank: parse app ID: %w", err)
			}

			payload := publicRankOutput{
				AppID:       numericAppID,
				Term:        termValue,
				Country:     strings.ToUpper(normalizedCountry),
				Platform:    string(platformValue),
				Found:       rankResult.Found,
				Rank:        rankResult.Rank,
				ResultCount: rankResult.ResultCount,
			}

			return shared.PrintOutputWithRenderers(payload, *output.Output, *output.Pretty, func() error {
				return renderPublicRankTable(payload)
			}, func() error {
				return renderPublicRankMarkdown(payload)
			})
		},
	}
}

func normalizePublicRankPlatform(value string) (itunes.PublicSearchPlatform, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	if normalized == "" {
		return "", fmt.Errorf("--platform is required")
	}
	switch itunes.PublicSearchPlatform(normalized) {
	case itunes.PublicSearchPlatformIOS:
		return itunes.PublicSearchPlatformIOS, nil
	case itunes.PublicSearchPlatformTVOS:
		return itunes.PublicSearchPlatformTVOS, nil
	default:
		return "", fmt.Errorf("--platform must be one of: IOS, TV_OS")
	}
}

func renderPublicRankTable(payload publicRankOutput) error {
	asc.RenderTable(publicRankHeaders(), [][]string{publicRankRow(payload)})
	return nil
}

func renderPublicRankMarkdown(payload publicRankOutput) error {
	asc.RenderMarkdown(publicRankHeaders(), [][]string{publicRankRow(payload)})
	return nil
}

func publicRankHeaders() []string {
	return []string{"App ID", "Term", "Country", "Platform", "Found", "Rank", "Result Count"}
}

func publicRankRow(payload publicRankOutput) []string {
	rank := "-"
	if payload.Rank != nil {
		rank = strconv.Itoa(*payload.Rank)
	}
	return []string{
		strconv.FormatInt(payload.AppID, 10),
		payload.Term,
		payload.Country,
		payload.Platform,
		strconv.FormatBool(payload.Found),
		rank,
		strconv.Itoa(payload.ResultCount),
	}
}
