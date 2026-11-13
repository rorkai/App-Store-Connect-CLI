package optimize

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

const (
	keywordScoreSchemaVersion = "1"

	// keywordScorePopularityWindow matches the search plan default so the two
	// commands read the same Apple Ads popularity snapshot.
	keywordScorePopularityWindow = "30d"

	keywordSourcePopularity  = "search_term_popularity"
	keywordSourceCompetition = "public_search"
	keywordSourceMetadata    = "competitor_metadata"
	keywordSourceRank        = "app_rank"
)

var collectSearchPopularityForKeywords = ads.CollectSearchPopularity

// KeywordsScoreCommand returns the composed keyword scoring command.
func KeywordsScoreCommand() *ffcli.Command {
	return keywordsScoreCommand(time.Now)
}

func keywordsScoreCommand(now func() time.Time) *ffcli.Command {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	keywords := shared.BindOnceCSVFlag(fs, "keywords", "Comma-separated keyword candidates to score (required)")
	country := fs.String("country", "us", "ISO alpha-2 App Store storefront country or region")
	appID := fs.String("app", "", "App Store app ID; adds this app's rank (or ASC_APP_ID env)")
	genre := fs.String("genre", "", "Apple Ads search popularity genre; enables the popularity source")
	adAccount := fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID/profile default)")
	adsProfile := fs.String("ads-profile", "", "Use named Apple Ads authentication profile")
	workers := fs.Int("workers", 10, "Number of parallel keyword lookups")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "score",
		ShortUsage: "asc optimize keywords score --keywords LIST [flags]",
		ShortHelp:  "Score keyword candidates for difficulty and demand.",
		LongHelp: `Score keyword candidates by composing official Apple sources.

Each keyword is evaluated against three sources, and any source that is
unavailable is reported as unavailable instead of being replaced with an
invented value:

  Competition  Public App Store search plus public app metadata. No
               authentication is required, and this source alone is enough to
               produce a difficulty score.
  Popularity   Official Apple Ads country-and-genre search demand. It requires
               --genre, a selected ad account, and Apple Ads authentication.
               Apple Ads authentication is resolved independently from ad-account selection:
               use --ads-profile or ASC_ADS_PROFILE,
               ASC_ADS_ACCESS_TOKEN, ASC_ADS_CLIENT_ID/ASC_ADS_TEAM_ID/
               ASC_ADS_KEY_ID plus ASC_ADS_PRIVATE_KEY_PATH,
               ASC_ADS_PRIVATE_KEY, or ASC_ADS_PRIVATE_KEY_B64, or the stored
               default Ads profile. The account is selected by
               --ad-account or ASC_ADS_AD_ACCOUNT_ID, the selected profile's
               ad_account_id, or local ads.ad_account_id configuration. It
               reads the latest complete week.
  Rank         This app's position in the public result window, added when
               --app or ASC_APP_ID is set.

Every score is reported next to the named raw inputs it was derived from, so a
caller can re-derive it without re-running the command. The formula, its
inputs, and its limitations are documented in docs/design/optimize-keywords.md.

Examples:
  asc optimize keywords score --keywords "focus timer,habit tracker"
  asc optimize keywords score --keywords "focus timer" --app "1234567890" --output table
  asc optimize keywords score --keywords "focus timer" --app "1234567890" --genre PRODUCTIVITY_UTILITIES --ad-account "987654321" --country US`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
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
			resolvedAppID := ""
			if strings.TrimSpace(shared.ResolveAppID(*appID)) != "" {
				if resolvedAppID, err = normalizeKeywordAppID(*appID); err != nil {
					return err
				}
			}
			normalizedGenre := strings.ToUpper(strings.TrimSpace(*genre))
			if normalizedGenre != "" && !searchPlanGenrePattern.MatchString(normalizedGenre) {
				return shared.UsageError("--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES")
			}

			client := newKeywordsItunesClient()
			competition, sources := collectKeywordCompetition(ctx, client, keywordCompetitionRequest{
				Keywords: normalizedKeywords,
				Country:  normalizedCountry,
				AppID:    resolvedAppID,
				Workers:  effectiveWorkers,
			})

			popularity, popularitySource := collectKeywordPopularity(ctx, keywordPopularityRequest{
				Keywords:   normalizedKeywords,
				Country:    strings.ToUpper(normalizedCountry),
				Genre:      normalizedGenre,
				AdAccount:  *adAccount,
				AdsProfile: *adsProfile,
			})
			sources = append(sources, popularitySource)
			if err := ctx.Err(); err != nil {
				return err
			}

			reportTime := now().UTC()
			report := buildKeywordScoreReport(keywordScoreBuildInput{
				GeneratedAt: reportTime.Format(time.RFC3339),
				AppID:       resolvedAppID,
				Country:     strings.ToUpper(normalizedCountry),
				Genre:       normalizedGenre,
				Workers:     effectiveWorkers,
				Now:         reportTime,
				Sources:     sources,
				Popularity:  popularity,
				Competition: competition,
			})
			if report.Summary.Unavailable == report.Summary.Keywords {
				failures := make([]error, 0, len(competition))
				for _, result := range competition {
					failures = append(failures, result.Err)
				}
				return fmt.Errorf("optimize keywords score: %w", representativeKeywordError(failures))
			}

			return shared.PrintOutput(&report, *output.Output, *output.Pretty)
		},
	}
}

// keywordCompetition is the public-search evidence for one keyword.
type keywordCompetition struct {
	Keyword  string
	Err      error
	AppCount int
	Apps     []competitorApp
	Rank     *int
}

type keywordCompetitionRequest struct {
	Keywords []string
	Country  string
	AppID    string
	Workers  int
}

// collectKeywordCompetition searches the public App Store once per keyword,
// then hydrates the top competitors' release dates in deduplicated batches.
func collectKeywordCompetition(
	ctx context.Context,
	client *itunes.Client,
	request keywordCompetitionRequest,
) ([]keywordCompetition, []ads.SearchOptimizationSourceStatus) {
	searchResults := fanOutKeywords(ctx, request.Keywords, request.Workers, func(ctx context.Context, keyword string) ([]itunes.SearchResult, error) {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		return client.SearchApps(requestCtx, keyword, request.Country, 200)
	})

	competition := make([]keywordCompetition, 0, len(searchResults))
	searchedKeywords := 0
	for _, result := range searchResults {
		entry := keywordCompetition{Keyword: result.Keyword, Err: result.Err}
		if result.Err == nil {
			searchedKeywords++
			entry.AppCount = len(result.Value)
			entry.Apps = topCompetitorApps(result.Value, keywordCompetitorSampleSize)
			entry.Rank = rankInSearchResults(result.Value, request.AppID)
		}
		competition = append(competition, entry)
	}

	metadata, metadataStatus := hydrateCompetitorMetadata(ctx, client, competition, request.Country, request.Workers)
	for index := range competition {
		for appIndex := range competition[index].Apps {
			hydrated, ok := metadata[competition[index].Apps[appIndex].AppID]
			if !ok {
				continue
			}
			competition[index].Apps[appIndex].ReleaseDate = hydrated.ReleaseDate
			competition[index].Apps[appIndex].CurrentVersionReleaseDate = hydrated.CurrentVersionReleaseDate
			if hydrated.PublisherName != "" {
				competition[index].Apps[appIndex].PublisherName = hydrated.PublisherName
			}
		}
	}

	sources := []ads.SearchOptimizationSourceStatus{
		keywordPartialSourceStatus(
			keywordSourceCompetition,
			searchedKeywords,
			len(competition)-searchedKeywords,
			competitionSourceError(competition),
		),
		metadataStatus,
	}
	if strings.TrimSpace(request.AppID) != "" {
		ranked := 0
		for _, entry := range competition {
			if entry.Rank != nil {
				ranked++
			}
		}
		sources = append(sources, keywordPartialSourceStatus(
			keywordSourceRank,
			ranked,
			len(competition)-searchedKeywords,
			competitionSourceError(competition),
		))
	} else {
		sources = append(sources, ads.SearchOptimizationSourceStatus{
			Name:   keywordSourceRank,
			Status: keywordStatusUnavailable,
			Error:  "--app was not provided, so this app's rank was not requested",
		})
	}
	return competition, sources
}

func topCompetitorApps(results []itunes.SearchResult, size int) []competitorApp {
	apps := make([]competitorApp, 0, size)
	for _, result := range results {
		if len(apps) >= size {
			break
		}
		apps = append(apps, competitorApp{
			AppID:             formatPublicAppID(result.AppID),
			Name:              result.Name,
			PublisherName:     result.SellerName,
			AverageUserRating: result.AverageRating,
			UserRatingCount:   result.RatingCount,
		})
	}
	return apps
}

func rankInSearchResults(results []itunes.SearchResult, appID string) *int {
	target := strings.TrimSpace(appID)
	if target == "" {
		return nil
	}
	for index, result := range results {
		if formatPublicAppID(result.AppID) != target {
			continue
		}
		rank := index + 1
		return &rank
	}
	return nil
}

// hydrateCompetitorMetadata fetches release dates for every competitor across
// every keyword in deduplicated, bounded batches.
func hydrateCompetitorMetadata(
	ctx context.Context,
	client *itunes.Client,
	competition []keywordCompetition,
	country string,
	workers int,
) (map[string]publicAppMetadata, ads.SearchOptimizationSourceStatus) {
	seen := make(map[string]struct{})
	ids := make([]string, 0)
	for _, entry := range competition {
		for _, app := range entry.Apps {
			if app.AppID == "" {
				continue
			}
			if _, duplicate := seen[app.AppID]; duplicate {
				continue
			}
			seen[app.AppID] = struct{}{}
			ids = append(ids, app.AppID)
		}
	}
	if len(ids) == 0 {
		return map[string]publicAppMetadata{}, ads.SearchOptimizationSourceStatus{
			Name:   keywordSourceMetadata,
			Status: keywordStatusEmpty,
		}
	}

	chunks := chunkAppIDs(ids, keywordLookupChunkSize)
	chunkKeys := make([]string, len(chunks))
	for index := range chunks {
		chunkKeys[index] = strings.Join(chunks[index], ",")
	}
	results := fanOutKeywords(ctx, chunkKeys, workers, func(ctx context.Context, chunk string) (map[string]publicAppMetadata, error) {
		requestCtx, cancel := shared.ContextWithTimeout(ctx)
		defer cancel()
		return fetchPublicAppMetadata(requestCtx, client, strings.Split(chunk, ","), country)
	})

	metadata := make(map[string]publicAppMetadata, len(ids))
	failures := make([]error, 0, len(results))
	failedChunks := 0
	omittedIDs := 0
	incompleteMetadata := 0
	completeMetadata := 0
	for _, result := range results {
		if result.Err != nil {
			failedChunks++
			failures = append(failures, result.Err)
			continue
		}
		for appID, app := range result.Value {
			metadata[appID] = app
			if app.ReleaseDate != "" && app.CurrentVersionReleaseDate != "" {
				completeMetadata++
			} else {
				incompleteMetadata++
			}
		}
		for _, appID := range strings.Split(result.Keyword, ",") {
			if _, ok := result.Value[appID]; !ok {
				omittedIDs++
			}
		}
	}
	coverageErr := representativeKeywordError(failures)
	incompleteBatches := failedChunks
	incompleteIDs := omittedIDs + incompleteMetadata
	if incompleteIDs > 0 {
		incompleteBatches++
		incompleteErr := fmt.Errorf(
			"lookup returned incomplete required release metadata (both releaseDate and currentVersionReleaseDate must be valid) for %d of %d requested app IDs",
			incompleteIDs,
			len(ids),
		)
		if coverageErr == nil {
			coverageErr = incompleteErr
		} else {
			coverageErr = fmt.Errorf("%w; %w", incompleteErr, coverageErr)
		}
	}
	return metadata, keywordPartialSourceStatus(
		keywordSourceMetadata,
		completeMetadata,
		incompleteBatches,
		coverageErr,
	)
}

type keywordPopularityRequest struct {
	Keywords   []string
	Country    string
	Genre      string
	AdAccount  string
	AdsProfile string
}

// collectKeywordPopularity reads the official Apple Ads country-and-genre
// demand snapshot. It degrades to unavailable with a reason when the required
// genre is missing.
func collectKeywordPopularity(
	ctx context.Context,
	request keywordPopularityRequest,
) (map[string]asc.KeywordPopularity, ads.SearchOptimizationSourceStatus) {
	if strings.TrimSpace(request.Genre) == "" {
		return nil, ads.SearchOptimizationSourceStatus{
			Name:   keywordSourcePopularity,
			Status: keywordStatusUnavailable,
			Error:  "Apple Ads search popularity needs --genre; popularity was not requested",
		}
	}

	window, err := resolveSearchPlanWindow(keywordScorePopularityWindow, time.Now())
	if err != nil {
		return nil, ads.SearchOptimizationSourceStatus{
			Name:   keywordSourcePopularity,
			Status: keywordStatusUnavailable,
			Error:  err.Error(),
		}
	}

	rows, err := collectSearchPopularityForKeywords(ctx, request.AdsProfile, request.AdAccount, ads.SearchOptimizationRequest{
		Country:         request.Country,
		Genre:           request.Genre,
		PopularityStart: window.PopularityStart,
		PopularityEnd:   window.PopularityEnd,
	})
	if err != nil {
		return nil, ads.SearchOptimizationSourceStatus{
			Name:   keywordSourcePopularity,
			Status: keywordStatusUnavailable,
			Error:  err.Error(),
		}
	}

	wanted := make(map[string][]string, len(request.Keywords))
	for _, keyword := range request.Keywords {
		normalized := normalizeKeywordText(keyword)
		if normalized != "" {
			wanted[normalized] = append(wanted[normalized], keyword)
		}
	}
	popularity := make(map[string]asc.KeywordPopularity)
	for _, row := range rows {
		term := normalizeKeywordText(row.Term)
		keywords := wanted[term]
		if len(keywords) == 0 {
			continue
		}
		for _, keyword := range keywords {
			// Apple publishes one row per week; keep the most recent one.
			if existing, ok := popularity[keyword]; ok && existing.Week >= row.Week {
				continue
			}
			popularity[keyword] = asc.KeywordPopularity{
				Country:           row.Country,
				Genre:             row.Genre,
				Week:              row.Week,
				RankInGenre:       row.RankInGenre,
				PopularityInGenre: row.PopularityInGenre,
				Popularity100:     row.Popularity100,
				Popularity5:       row.Popularity5,
			}
		}
	}

	return popularity, keywordSourceStatus(keywordSourcePopularity, len(popularity), nil)
}

type keywordScoreBuildInput struct {
	GeneratedAt string
	AppID       string
	Country     string
	Genre       string
	Workers     int
	Now         time.Time
	Sources     []ads.SearchOptimizationSourceStatus
	Popularity  map[string]asc.KeywordPopularity
	Competition []keywordCompetition
}

func buildKeywordScoreReport(input keywordScoreBuildInput) asc.KeywordScoreReport {
	rows := make([]asc.KeywordScoreRow, 0, len(input.Competition))
	summary := asc.KeywordScoreSummary{Keywords: len(input.Competition)}

	for _, entry := range input.Competition {
		row := asc.KeywordScoreRow{Keyword: entry.Keyword, Status: keywordStatusAvailable}
		if popularity, ok := input.Popularity[entry.Keyword]; ok {
			value := popularity
			row.Popularity = &value
		}
		if entry.Err != nil {
			row.Status = keywordStatusUnavailable
			row.Error = entry.Err.Error()
			summary.Unavailable++
			rows = append(rows, row)
			continue
		}

		signals := make([]asc.KeywordScoreSignals, 0, len(entry.Apps))
		appScores := make([]float64, 0, len(entry.Apps))
		match := keywordMatchNone
		for _, app := range entry.Apps {
			scored := scoreCompetitorApp(app, entry.Keyword, input.Now)
			if keywordMatchScore(scored.KeywordMatch) > keywordMatchScore(match) {
				match = scored.KeywordMatch
			}
			signals = append(signals, scored)
			appScores = append(appScores, scored.AppScore)
		}

		difficulty := computeKeywordDifficulty(appScores, entry.AppCount)
		brand := isBrandKeyword(entry.Keyword, entry.Apps)
		appCount := entry.AppCount

		row.DifficultyScore = &difficulty.Difficulty
		row.MinDifficultyScore = &difficulty.MinDifficulty
		row.AverageAppScore = &difficulty.AverageAppScore
		row.MinimumAppScore = &difficulty.MinimumAppScore
		row.NormalizedAppCount = &difficulty.NormalizedAppCount
		row.Fallback = difficulty.Fallback
		row.IsBrandKeyword = &brand
		row.AppCount = &appCount
		row.KeywordMatch = match
		row.Rank = entry.Rank
		row.RawSignals = signals

		summary.Scored++
		if brand {
			summary.BrandMatches++
		}
		if entry.Rank != nil {
			summary.WithRank++
		}
		rows = append(rows, row)
	}

	sources := append([]ads.SearchOptimizationSourceStatus(nil), input.Sources...)
	sort.SliceStable(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	outputSources := make([]asc.KeywordScoreSourceStatus, 0, len(sources))
	for _, source := range sources {
		outputSources = append(outputSources, asc.KeywordScoreSourceStatus{
			Name:   source.Name,
			Status: source.Status,
			Count:  source.Count,
			Error:  source.Error,
		})
	}

	return asc.KeywordScoreReport{
		SchemaVersion: keywordScoreSchemaVersion,
		GeneratedAt:   input.GeneratedAt,
		AppID:         input.AppID,
		Country:       input.Country,
		Genre:         input.Genre,
		Workers:       input.Workers,
		Sources:       outputSources,
		Summary:       summary,
		Rows:          rows,
	}
}

func keywordSourceStatus(name string, count int, err error) ads.SearchOptimizationSourceStatus {
	status := ads.SearchOptimizationSourceStatus{Name: name, Count: count}
	switch {
	case err != nil:
		status.Status = keywordStatusUnavailable
		status.Error = err.Error()
	case count == 0:
		status.Status = keywordStatusEmpty
	default:
		status.Status = keywordStatusAvailable
	}
	return status
}

// keywordPartialSourceStatus records a source that can succeed for part of its
// work. It stays available while any evidence was collected and still reports
// the representative failure so partial coverage is visible rather than silent.
func keywordPartialSourceStatus(name string, count, failed int, err error) ads.SearchOptimizationSourceStatus {
	status := keywordSourceStatus(name, count, nil)
	if err == nil || failed == 0 {
		return status
	}
	if count == 0 {
		status.Status = keywordStatusUnavailable
	}
	status.Error = err.Error()
	return status
}

func competitionSourceError(competition []keywordCompetition) error {
	failures := make([]error, 0, len(competition))
	for _, entry := range competition {
		failures = append(failures, entry.Err)
	}
	return representativeKeywordError(failures)
}

func formatPublicAppID(appID int64) string {
	if appID == 0 {
		return ""
	}
	return strconv.FormatInt(appID, 10)
}
