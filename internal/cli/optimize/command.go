package optimize

import (
	"context"
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

var searchPlanGenrePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var (
	resolveSearchMetadataForPlan = resolveSearchMetadata
	collectSearchDataForPlan     = ads.CollectSearchOptimizationData
)

// OptimizeCommand returns the cross-API optimization group.
func OptimizeCommand() *ffcli.Command {
	fs := flag.NewFlagSet("optimize", flag.ExitOnError)
	return &ffcli.Command{
		Name:        "optimize",
		ShortUsage:  "asc optimize <subcommand> [flags]",
		ShortHelp:   "Build cross-API optimization plans.",
		LongHelp:    "Build read-only cross-API optimization plans from official Apple APIs.",
		FlagSet:     fs,
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{SearchCommand(), KeywordsCommand()},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SearchCommand returns the search optimization group.
func SearchCommand() *ffcli.Command {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	return &ffcli.Command{
		Name:        "search",
		ShortUsage:  "asc optimize search <subcommand> [flags]",
		ShortHelp:   "Build App Store search optimization plans.",
		LongHelp:    "Build App Store search optimization plans from official Apple APIs.",
		FlagSet:     fs,
		UsageFunc:   shared.DefaultUsageFunc,
		Subcommands: []*ffcli.Command{SearchPlanCommand()},
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
	}
}

// SearchPlanCommand returns the official-only search optimization workflow.
func SearchPlanCommand() *ffcli.Command {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	appID := fs.String("app", "", "App Store Connect app ID, bundle ID, or exact app name (required, or ASC_APP_ID env)")
	version := fs.String("version", "", "App Store version string (required)")
	platform := fs.String("platform", "IOS", "App Store platform: IOS, MAC_OS, TV_OS, VISION_OS")
	appInfoID := shared.BindResourceIDFlag(fs, "app-info", "appInfos", "App Info ID override when multiple records cannot be auto-resolved")
	adAccount := fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID/profile default)")
	adsProfile := fs.String("ads-profile", "", "Use named Apple Ads authentication profile")
	country := fs.String("country", "", "ISO alpha-2 Apple Ads country or region (required)")
	genre := fs.String("genre", "", "Apple Ads search popularity genre (required)")
	locale := fs.String("locale", "", "App Store metadata locale, for example en-US (required)")
	windowValue := fs.String("window", "30d", "Paid performance window: 2d through 30d")
	outDir := fs.String("out-dir", "", "Write reviewable plan artifacts to this directory")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "plan",
		ShortUsage: "asc optimize search plan [flags]",
		ShortHelp:  "Join official Apple Ads and App Store metadata into a search plan.",
		LongHelp: `Join the official Apple Ads Platform API v1 with App Store Connect metadata.

This read-only workflow does not mutate campaigns or App Store metadata. It
keeps keyword demand, app-specific paid reach, actual search-term outcomes,
and metadata coverage distinct. Unavailable official sources are reported
instead of replaced with invented values.

When --out-dir is set, the command writes report.json, metadata-candidates.csv,
exact-keywords.json, and negative-keywords.json as reviewable plans.

Examples:
  asc optimize search plan --app "123456789" --version "1.2.0" --ad-account "987654321" --country US --genre PRODUCTIVITY_UTILITIES --locale en-US
  asc optimize search plan --app "123456789" --version "1.2.0" --ad-account "987654321" --country US --genre PRODUCTIVITY_UTILITIES --locale en-US --out-dir .asc/optimization/1.2.0 --output markdown`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppSelector := shared.ResolveAppID(*appID)
			if resolvedAppSelector == "" {
				return shared.UsageError("--app is required (or set ASC_APP_ID)")
			}
			if strings.TrimSpace(*version) == "" {
				return shared.UsageError("--version is required")
			}
			normalizedPlatform, err := shared.NormalizeAppStoreVersionPlatform(*platform)
			if err != nil {
				return shared.UsageError(err.Error())
			}
			normalizedCountry := strings.ToUpper(strings.TrimSpace(*country))
			if len(normalizedCountry) != 2 || normalizedCountry[0] < 'A' || normalizedCountry[0] > 'Z' || normalizedCountry[1] < 'A' || normalizedCountry[1] > 'Z' {
				return shared.UsageError("--country must be an ISO alpha-2 code")
			}
			normalizedGenre := strings.ToUpper(strings.TrimSpace(*genre))
			if normalizedGenre == "" || !searchPlanGenrePattern.MatchString(normalizedGenre) {
				return shared.UsageError("--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES")
			}
			normalizedLocale, err := shared.NormalizeAppStoreLocalizationLocale(*locale)
			if err != nil {
				return shared.UsageError("--locale " + err.Error())
			}
			window, err := resolveSearchPlanWindow(*windowValue, time.Now())
			if err != nil {
				return shared.UsageError(err.Error())
			}

			metadata, err := resolveSearchMetadataForPlan(ctx, resolvedAppSelector, strings.TrimSpace(*version), normalizedPlatform, strings.TrimSpace(*appInfoID), normalizedLocale)
			if err != nil {
				return fmt.Errorf("optimize search plan: App Store Connect metadata: %w", err)
			}
			adsData, err := collectSearchDataForPlan(ctx, *adsProfile, *adAccount, ads.SearchOptimizationRequest{
				AppID:           metadata.AppID,
				Country:         normalizedCountry,
				Genre:           normalizedGenre,
				Start:           window.Start,
				End:             window.End,
				PopularityStart: window.PopularityStart,
				PopularityEnd:   window.PopularityEnd,
			})
			if err != nil {
				return fmt.Errorf("optimize search plan: Apple Ads: %w", err)
			}

			report := buildSearchPlan(searchPlanBuildInput{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				AppID:       metadata.AppID,
				Version:     strings.TrimSpace(*version),
				VersionID:   metadata.VersionID,
				AppInfoID:   metadata.AppInfoID,
				Platform:    metadata.Platform,
				Country:     normalizedCountry,
				Genre:       normalizedGenre,
				Locale:      normalizedLocale,
				Window:      window,
				Metadata:    metadata.Metadata,
				Ads:         adsData,
			})
			if strings.TrimSpace(*outDir) != "" {
				artifacts, artifactErr := writeSearchPlanArtifacts(strings.TrimSpace(*outDir), report)
				if artifactErr != nil {
					return fmt.Errorf("optimize search plan: %w", artifactErr)
				}
				report.Artifacts = artifacts
			}
			return shared.PrintOutputWithRenderers(
				report,
				*output.Output,
				*output.Pretty,
				func() error { return renderSearchPlanTable(report) },
				func() error { return renderSearchPlanMarkdown(report) },
			)
		},
	}
}
