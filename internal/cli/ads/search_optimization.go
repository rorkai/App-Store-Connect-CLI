package ads

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

// SearchOptimizationRequest identifies the official Apple Ads data window and
// market used by the cross-API search optimization workflow.
type SearchOptimizationRequest struct {
	AppID           string
	Country         string
	Genre           string
	Start           string
	End             string
	PopularityStart string
	PopularityEnd   string
	// Limit bounds the suggestions-only collection used by keyword discovery.
	// The broader optimization plan leaves it unset and retains its existing
	// pagination behavior.
	Limit int
}

// SearchOptimizationSourceStatus describes the availability of one official
// Apple Ads source. Empty responses are distinct from unavailable sources.
type SearchOptimizationSourceStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Count  int    `json:"count"`
	Error  string `json:"error,omitempty"`
}

// SearchSuggestion is one keyword or phrase returned by Apple's suggestion
// endpoints. Popularity is kept separate from market-scoped popularity.
type SearchSuggestion struct {
	Text       string `json:"text"`
	Popularity *int   `json:"popularity,omitempty"`
	Kind       string `json:"kind"`
}

// SearchPopularity is one country-and-genre search demand snapshot.
type SearchPopularity struct {
	Term              string `json:"term"`
	Country           string `json:"country,omitempty"`
	Genre             string `json:"genre,omitempty"`
	Week              string `json:"week,omitempty"`
	Month             string `json:"month,omitempty"`
	RankInGenre       *int   `json:"rankInGenre,omitempty"`
	PopularityInGenre *int   `json:"popularityInGenre,omitempty"`
	Popularity100     *int   `json:"popularity100,omitempty"`
	Popularity5       *int   `json:"popularity5,omitempty"`
}

// SearchImpressionShare is one app-specific paid reach row.
type SearchImpressionShare struct {
	Term        string   `json:"term"`
	Country     string   `json:"country,omitempty"`
	Day         string   `json:"day,omitempty"`
	Week        string   `json:"week,omitempty"`
	Low         *float64 `json:"low,omitempty"`
	High        *float64 `json:"high,omitempty"`
	Rank        *int     `json:"rank,omitempty"`
	Popularity5 *int     `json:"popularity5,omitempty"`
}

// SearchTermPerformance captures the actual user query and paid outcome from
// Apple's app Search Terms report.
type SearchTermPerformance struct {
	Term          string `json:"term"`
	KeywordText   string `json:"keywordText,omitempty"`
	MatchType     string `json:"matchType,omitempty"`
	Country       string `json:"country,omitempty"`
	CampaignID    int64  `json:"campaignId,omitempty"`
	AdGroupID     int64  `json:"adGroupId,omitempty"`
	Impressions   int64  `json:"impressions"`
	Taps          int64  `json:"taps"`
	TapInstalls   int64  `json:"tapInstalls"`
	TotalInstalls int64  `json:"totalInstalls"`
	SpendAmount   string `json:"spendAmount,omitempty"`
	SpendCurrency string `json:"spendCurrency,omitempty"`
}

// SearchCampaign is the app campaign context used to scope dependent queries.
type SearchCampaign struct {
	ID            int64    `json:"id"`
	Name          string   `json:"name,omitempty"`
	Status        string   `json:"status,omitempty"`
	DisplayStatus string   `json:"displayStatus,omitempty"`
	Countries     []string `json:"countries,omitempty"`
}

// SearchTargetingKeyword is an existing paid targeting keyword.
type SearchTargetingKeyword struct {
	Text       string `json:"text"`
	MatchType  string `json:"matchType"`
	Status     string `json:"status,omitempty"`
	CampaignID int64  `json:"campaignId"`
	AdGroupID  int64  `json:"adGroupId"`
}

// SearchNegativeKeyword is an existing paid exclusion.
type SearchNegativeKeyword struct {
	Text       string `json:"text"`
	MatchType  string `json:"matchType"`
	Status     string `json:"status,omitempty"`
	CampaignID int64  `json:"campaignId"`
	AdGroupID  int64  `json:"adGroupId,omitempty"`
}

// SearchEligibility describes one placement/storefront eligibility row.
type SearchEligibility struct {
	Country         string   `json:"country,omitempty"`
	SupplyPlacement string   `json:"supplyPlacement,omitempty"`
	DeviceClass     string   `json:"deviceClass,omitempty"`
	State           string   `json:"state"`
	Reasons         []string `json:"reasons,omitempty"`
}

// SearchOptimizationData is the normalized official Apple Ads evidence used
// by the top-level optimizer.
type SearchOptimizationData struct {
	Sources                        []SearchOptimizationSourceStatus `json:"sources"`
	Suggestions                    []SearchSuggestion               `json:"suggestions,omitempty"`
	SuggestionsTruncated           bool                             `json:"-"`
	Popularities                   []SearchPopularity               `json:"popularities,omitempty"`
	ImpressionShares               []SearchImpressionShare          `json:"impressionShares,omitempty"`
	SearchTerms                    []SearchTermPerformance          `json:"searchTerms,omitempty"`
	Campaigns                      []SearchCampaign                 `json:"campaigns,omitempty"`
	Keywords                       []SearchTargetingKeyword         `json:"keywords,omitempty"`
	NegativeKeywords               []SearchNegativeKeyword          `json:"negativeKeywords,omitempty"`
	Eligibilities                  []SearchEligibility              `json:"eligibilities,omitempty"`
	DailyBudgetRecommendationItems []json.RawMessage                `json:"dailyBudgetRecommendationItems,omitempty"`
	TargetCPARecommendationItems   []json.RawMessage                `json:"targetCpaRecommendationItems,omitempty"`
	TargetCPASuggestion            json.RawMessage                  `json:"targetCpaSuggestion,omitempty"`
	DailyBudgetRecommendations     int                              `json:"dailyBudgetRecommendations"`
	TargetCPARecommendations       int                              `json:"targetCpaRecommendations"`
}

// CollectSearchOptimizationData resolves official Apple Ads authentication and
// gathers the read-only evidence needed by `asc optimize search plan`.
func CollectSearchOptimizationData(ctx context.Context, adsProfile, adAccount string, request SearchOptimizationRequest) (SearchOptimizationData, error) {
	profile := strings.TrimSpace(adsProfile)
	account := strings.TrimSpace(adAccount)
	client, _, err := resolvePlatformClientAndAdAccountID(ctx, commonFlags{
		AdsProfile: &profile,
		AdAccount:  &account,
	}, appleads.ContextAdAccount)
	if err != nil {
		return SearchOptimizationData{}, err
	}
	return fetchSearchOptimizationData(ctx, client, request)
}

// CollectSearchSuggestions resolves official Apple Ads authentication and
// gathers only the keyword and phrase suggestions needed by
// `asc optimize keywords discover`. The request limit is applied to each
// documented suggestion endpoint before pagination, so discovery never walks
// the broader optimization data set or downloads a full 1000-item page just
// to truncate it locally.
func CollectSearchSuggestions(ctx context.Context, adsProfile, adAccount string, request SearchOptimizationRequest) (SearchOptimizationData, error) {
	profile := strings.TrimSpace(adsProfile)
	account := strings.TrimSpace(adAccount)
	client, _, err := resolvePlatformClientAndAdAccountID(ctx, commonFlags{
		AdsProfile: &profile,
		AdAccount:  &account,
	}, appleads.ContextAdAccount)
	if err != nil {
		return SearchOptimizationData{}, err
	}
	return fetchSearchSuggestions(ctx, client, request)
}

// CollectSearchPopularity resolves official Apple Ads authentication and reads
// only country-and-genre search demand. Unlike the broader optimization plan,
// this source is not scoped to a promoted app.
func CollectSearchPopularity(ctx context.Context, adsProfile, adAccount string, request SearchOptimizationRequest) ([]SearchPopularity, error) {
	profile := strings.TrimSpace(adsProfile)
	account := strings.TrimSpace(adAccount)
	client, _, err := resolvePlatformClientAndAdAccountID(ctx, commonFlags{
		AdsProfile: &profile,
		AdAccount:  &account,
	}, appleads.ContextAdAccount)
	if err != nil {
		return nil, err
	}
	return fetchOptimizationPopularity(ctx, client, request)
}

func fetchSearchOptimizationData(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) (SearchOptimizationData, error) {
	data := SearchOptimizationData{}
	successfulIntelligenceSources := 0
	record := func(name string, count int, err error, intelligence bool) {
		status := SearchOptimizationSourceStatus{Name: name, Count: count}
		switch {
		case err != nil:
			status.Status = "unavailable"
			status.Error = err.Error()
		case count == 0:
			status.Status = "empty"
			if intelligence {
				successfulIntelligenceSources++
			}
		default:
			status.Status = "available"
			if intelligence {
				successfulIntelligenceSources++
			}
		}
		data.Sources = append(data.Sources, status)
	}

	appID := strings.TrimSpace(request.AppID)
	appIDNumber, err := strconv.ParseInt(appID, 10, 64)
	if err != nil || appIDNumber <= 0 {
		return data, fmt.Errorf("app ID must be a positive integer Adam ID")
	}

	data.Campaigns, err = fetchOptimizationCampaigns(ctx, client, request)
	campaignErr := err
	record("campaigns", len(data.Campaigns), campaignErr, false)

	keywordSuggestions, keywordErr := fetchOptimizationSuggestions(ctx, client, request, false)
	data.Suggestions = append(data.Suggestions, keywordSuggestions...)
	record("keyword_suggestions", len(keywordSuggestions), keywordErr, true)
	phraseSuggestions, phraseErr := fetchOptimizationSuggestions(ctx, client, request, true)
	data.Suggestions = append(data.Suggestions, phraseSuggestions...)
	record("phrase_suggestions", len(phraseSuggestions), phraseErr, true)

	var targetCPASuggestionErr error
	data.TargetCPASuggestion, targetCPASuggestionErr = fetchOptimizationTargetCPASuggestion(ctx, client, request)
	record("target_cpa_suggestion", optimizationRawObjectCount(data.TargetCPASuggestion), targetCPASuggestionErr, true)

	data.Popularities, err = fetchOptimizationPopularity(ctx, client, request)
	record("search_term_popularity", len(data.Popularities), err, true)
	data.ImpressionShares, err = fetchOptimizationImpressionShare(ctx, client, request)
	record("impression_share", len(data.ImpressionShares), err, true)
	data.Eligibilities, err = fetchOptimizationEligibility(ctx, client, request, appIDNumber)
	record("eligibility", len(data.Eligibilities), err, true)

	var dailyErr error
	data.DailyBudgetRecommendationItems, dailyErr = fetchOptimizationRecommendations(ctx, client, request, "daily-budgets")
	data.DailyBudgetRecommendations = len(data.DailyBudgetRecommendationItems)
	record("daily_budget_recommendations", data.DailyBudgetRecommendations, dailyErr, true)
	var targetErr error
	data.TargetCPARecommendationItems, targetErr = fetchOptimizationRecommendations(ctx, client, request, "target-cpas")
	data.TargetCPARecommendations = len(data.TargetCPARecommendationItems)
	record("target_cpa_recommendations", data.TargetCPARecommendations, targetErr, true)

	var keywordErrors, negativeErrors, searchTermErrors []string
	if campaignErr != nil {
		dependencyError := "campaign scope unavailable: " + campaignErr.Error()
		keywordErrors = append(keywordErrors, dependencyError)
		negativeErrors = append(negativeErrors, dependencyError)
		searchTermErrors = append(searchTermErrors, dependencyError)
	}
	for _, campaign := range data.Campaigns {
		keywords, keywordErr := fetchOptimizationKeywords(ctx, client, campaign.ID)
		data.Keywords = append(data.Keywords, keywords...)
		if keywordErr != nil {
			keywordErrors = append(keywordErrors, fmt.Sprintf("campaign %d: %v", campaign.ID, keywordErr))
		}

		negatives, negativeErr := fetchOptimizationNegatives(ctx, client, campaign.ID)
		data.NegativeKeywords = append(data.NegativeKeywords, negatives...)
		if negativeErr != nil {
			negativeErrors = append(negativeErrors, fmt.Sprintf("campaign %d: %v", campaign.ID, negativeErr))
		}

		terms, searchTermErr := fetchOptimizationSearchTerms(ctx, client, request, campaign.ID)
		data.SearchTerms = append(data.SearchTerms, terms...)
		if searchTermErr != nil {
			searchTermErrors = append(searchTermErrors, fmt.Sprintf("campaign %d: %v", campaign.ID, searchTermErr))
		}
	}
	record("targeting_keywords", len(data.Keywords), joinedOptimizationError(keywordErrors), false)
	record("negative_keywords", len(data.NegativeKeywords), joinedOptimizationError(negativeErrors), false)
	record("search_term_performance", len(data.SearchTerms), joinedOptimizationError(searchTermErrors), true)

	if successfulIntelligenceSources == 0 {
		return data, fmt.Errorf("all official Apple Ads optimization sources are unavailable")
	}
	return data, nil
}

func fetchSearchSuggestions(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) (SearchOptimizationData, error) {
	data := SearchOptimizationData{}
	appID := strings.TrimSpace(request.AppID)
	appIDNumber, err := strconv.ParseInt(appID, 10, 64)
	if err != nil || appIDNumber <= 0 {
		return data, fmt.Errorf("app ID must be a positive integer Adam ID")
	}
	if request.Limit < 1 {
		return data, fmt.Errorf("suggestion limit must be at least 1")
	}

	suggestionRequest := SearchOptimizationRequest{AppID: appID, Country: request.Country, Limit: request.Limit}
	record := func(name string, suggestions []SearchSuggestion, err error) {
		status := SearchOptimizationSourceStatus{Name: name, Count: len(suggestions)}
		switch {
		case err != nil:
			status.Status = "unavailable"
			status.Error = err.Error()
		case len(suggestions) == 0:
			status.Status = "empty"
		default:
			status.Status = "available"
		}
		data.Sources = append(data.Sources, status)
	}

	var keywordSuggestions []SearchSuggestion
	var keywordMore bool
	keywordSuggestions, keywordMore, err = fetchOptimizationSuggestionsLimitedWithMore(ctx, client, suggestionRequest, false, request.Limit)
	keywordErr := err
	data.Suggestions = append(data.Suggestions, keywordSuggestions...)
	record("keyword_suggestions", keywordSuggestions, keywordErr)

	var phraseMore bool
	var phraseSuggestions []SearchSuggestion
	phraseSuggestions, phraseMore, err = fetchOptimizationSuggestionsLimitedWithMore(ctx, client, suggestionRequest, true, request.Limit)
	phraseErr := err
	data.Suggestions = append(data.Suggestions, phraseSuggestions...)
	record("phrase_suggestions", phraseSuggestions, phraseErr)
	data.SuggestionsTruncated = keywordMore || phraseMore

	return data, nil
}

func fetchOptimizationCampaigns(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) ([]SearchCampaign, error) {
	spec := mustOptimizationEndpoint("campaigns", "find")
	body := map[string]any{"filters": []any{
		optimizationFilter("promotedObjectType", "EQUALS", "APPSTORE_APP"),
		optimizationFilter("promotedObjectId", "EQUALS", strings.TrimSpace(request.AppID)),
	}}
	var rows []campaignResponse
	items, err := queryOptimizationList[campaignResponse](ctx, client, spec, body, 1000)
	if err != nil {
		return nil, err
	}
	rows = append(rows, items...)
	result := make([]SearchCampaign, 0, len(rows))
	for _, row := range rows {
		countries := row.Targeting.CountryOrRegion.Include
		if len(countries) > 0 && !containsFold(countries, request.Country) {
			continue
		}
		result = append(result, SearchCampaign{
			ID:            row.ID.Int64(),
			Name:          row.Name,
			Status:        row.Status,
			DisplayStatus: row.DisplayStatus,
			Countries:     countries,
		})
	}
	return result, nil
}

func fetchOptimizationSuggestions(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest, phrases bool) ([]SearchSuggestion, error) {
	items, _, err := fetchOptimizationSuggestionsLimitedWithMore(ctx, client, request, phrases, 0)
	return items, err
}

func fetchOptimizationSuggestionsLimitedWithMore(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest, phrases bool, limit int) ([]SearchSuggestion, bool, error) {
	path := []string{"suggestions", "keywords", "find"}
	kind := "keyword"
	if phrases {
		path = []string{"suggestions", "phrases", "find"}
		kind = "phrase"
	}
	spec := mustOptimizationEndpoint(path...)
	filters := []any{
		optimizationFilter("promotedObjectId", "EQUALS", []string{strings.TrimSpace(request.AppID)}),
		optimizationFilter("promotedObjectType", "EQUALS", []string{"APPSTORE_APP"}),
	}
	if phrases {
		filters = append(filters, optimizationFilter("queryType", "EQUALS", []string{"SUGGESTION"}))
	} else {
		filters = append(filters, optimizationFilter("countriesOrRegions", "IN", []string{strings.ToUpper(strings.TrimSpace(request.Country))}))
	}
	// Omit sorting: Apple's suggestions endpoints reject sorting by
	// "popularity" (INVALID_FIELD_ATTRIBUTE: Field 'popularity' is not
	// queryable). Official starter payloads for these endpoints also omit
	// sorting. Callers that need popularity order sort locally after merge.
	body := map[string]any{
		"filters": filters,
	}
	items, more, err := queryOptimizationListBoundedWithMore[suggestionResponse](ctx, client, spec, body, 1000, limit)
	result := make([]SearchSuggestion, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if phrases {
			text = strings.TrimSpace(item.Phrase)
		}
		if text != "" {
			result = append(result, SearchSuggestion{Text: text, Popularity: item.Popularity, Kind: kind})
		}
	}
	return result, more, err
}

func fetchOptimizationPopularity(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) ([]SearchPopularity, error) {
	spec := mustOptimizationEndpoint("insights", "search-term-popularity", "find")
	body := map[string]any{
		"filters": []any{
			optimizationFilter("countryOrRegion", "EQUALS", strings.ToUpper(strings.TrimSpace(request.Country))),
			optimizationFilter("genre", "EQUALS", strings.ToUpper(strings.TrimSpace(request.Genre))),
		},
		"timeRange": map[string]any{"start": request.PopularityStart, "end": request.PopularityEnd, "timeZone": "UTC", "granularity": "WEEKLY_SUN_SAT"},
		"sorting":   []any{map[string]any{"field": "rankInGenre", "sortOrder": "ASC"}},
	}
	items, err := queryOptimizationRows[popularityResponse](ctx, client, spec, body, 5000)
	if err != nil {
		return nil, err
	}
	result := make([]SearchPopularity, 0, len(items))
	for _, item := range items {
		if term := strings.TrimSpace(item.SearchTerm); term != "" {
			result = append(result, SearchPopularity{
				Term: term, Country: item.Country, Genre: item.Genre, Week: item.Week, Month: item.Month,
				RankInGenre: item.RankInGenre, PopularityInGenre: item.PopularityInGenre,
				Popularity100: item.Popularity100, Popularity5: item.Popularity5,
			})
		}
	}
	return result, nil
}

func fetchOptimizationImpressionShare(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) ([]SearchImpressionShare, error) {
	spec := mustOptimizationEndpoint("insights", "impression-share", "find")
	body := map[string]any{
		"filters": []any{
			optimizationFilter("promotedObjectId", "EQUALS", strings.TrimSpace(request.AppID)),
			optimizationFilter("countryOrRegion", "EQUALS", strings.ToUpper(strings.TrimSpace(request.Country))),
		},
		"options":   map[string]any{"impressionShareReportType": "ALL_SLOTS"},
		"timeRange": map[string]any{"start": request.Start, "end": request.End, "timeZone": "UTC", "granularity": "DAILY"},
	}
	items, err := queryOptimizationRows[impressionShareResponse](ctx, client, spec, body, 5000)
	if err != nil {
		return nil, err
	}
	result := make([]SearchImpressionShare, 0, len(items))
	for _, item := range items {
		if term := strings.TrimSpace(item.SearchTerm); term != "" {
			result = append(result, SearchImpressionShare{
				Term: term, Country: item.Country, Day: item.Day, Week: item.Week,
				Low: item.Low, High: item.High, Rank: item.Rank, Popularity5: item.Popularity5,
			})
		}
	}
	return result, nil
}

func fetchOptimizationEligibility(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest, appID int64) ([]SearchEligibility, error) {
	spec := mustOptimizationEndpoint("apps", "eligibility", "find")
	body := map[string]any{"filters": []any{optimizationFilter("adamId", "EQUALS", appID)}}
	items, err := queryOptimizationList[eligibilityResponse](ctx, client, spec, body, 1000)
	if err != nil {
		return nil, err
	}
	result := make([]SearchEligibility, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(item.Country, request.Country) || item.SupplyPlacement != "APPSTORE_SEARCH_RESULTS" {
			continue
		}
		result = append(result, SearchEligibility(item))
	}
	return result, nil
}

func fetchOptimizationRecommendations(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest, kind string) ([]json.RawMessage, error) {
	spec := mustOptimizationEndpoint("recommendations", kind, "find")
	body := map[string]any{"filters": []any{
		optimizationFilter("promotedObjectId", "EQUALS", []string{strings.TrimSpace(request.AppID)}),
		optimizationFilter("promotedObjectType", "EQUALS", []string{"APPSTORE_APP"}),
		optimizationFilter("state", "EQUALS", []string{"AVAILABLE"}),
	}}
	items, err := queryOptimizationList[json.RawMessage](ctx, client, spec, body, 1000)
	return items, err
}

func fetchOptimizationTargetCPASuggestion(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest) (json.RawMessage, error) {
	spec := mustOptimizationEndpoint("suggestions", "target-cpas", "find")
	body := map[string]any{"filters": []any{
		optimizationFilter("promotedObjectId", "EQUALS", []string{strings.TrimSpace(request.AppID)}),
		optimizationFilter("promotedObjectType", "EQUALS", []string{"APPSTORE_APP"}),
		optimizationFilter("countryOrRegion", "IN", []string{strings.ToUpper(strings.TrimSpace(request.Country))}),
	}}
	raw, err := executeOptimizationQuery(ctx, client, spec, body)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", strings.Join(spec.CommandPath, " "), err)
	}
	if optimizationRawObjectCount(envelope.Result) == 0 {
		return nil, nil
	}
	return envelope.Result, nil
}

func fetchOptimizationKeywords(ctx context.Context, client *appleads.Client, campaignID int64) ([]SearchTargetingKeyword, error) {
	spec := mustOptimizationEndpoint("targeting-keywords", "find")
	body := map[string]any{"filters": []any{optimizationFilter("campaignId", "EQUALS", campaignID)}}
	items, err := queryOptimizationList[keywordResponse](ctx, client, spec, body, 1000)
	if err != nil {
		return nil, err
	}
	result := make([]SearchTargetingKeyword, 0, len(items))
	for _, item := range items {
		result = append(result, SearchTargetingKeyword{Text: item.Text, MatchType: item.MatchType, Status: item.Status, CampaignID: item.CampaignID.Int64(), AdGroupID: item.AdGroupID.Int64()})
	}
	return result, nil
}

func fetchOptimizationNegatives(ctx context.Context, client *appleads.Client, campaignID int64) ([]SearchNegativeKeyword, error) {
	spec := mustOptimizationEndpoint("negative-keywords", "find")
	queries := []map[string]any{
		{"filters": []any{optimizationFilter("campaignId", "EQUALS", campaignID), map[string]any{"field": "adGroupId", "operator": "IS_NULL"}}},
		{"filters": []any{optimizationFilter("campaignId", "EQUALS", campaignID), map[string]any{"field": "adGroupId", "operator": "IS_NOT_NULL"}}},
	}
	result := make([]SearchNegativeKeyword, 0)
	for _, body := range queries {
		items, err := queryOptimizationList[negativeKeywordResponse](ctx, client, spec, body, 1000)
		if err != nil {
			return result, err
		}
		for _, item := range items {
			result = append(result, SearchNegativeKeyword{Text: item.Text, MatchType: item.MatchType, Status: item.Status, CampaignID: item.CampaignID.Int64(), AdGroupID: item.AdGroupID.Int64()})
		}
	}
	return result, nil
}

func fetchOptimizationSearchTerms(ctx context.Context, client *appleads.Client, request SearchOptimizationRequest, campaignID int64) ([]SearchTermPerformance, error) {
	spec := mustOptimizationEndpoint("reports", "apps", "search-terms")
	body := map[string]any{
		"filters":   []any{optimizationFilter("campaignId", "EQUALS", campaignID)},
		"fields":    []string{"impressions", "taps", "localSpend", "tapInstalls", "totalInstalls"},
		"groupBy":   []string{"countryOrRegion"},
		"timeRange": map[string]any{"start": request.Start, "end": request.End, "timeZone": "ORTZ", "granularity": "DAILY"},
	}
	items, err := queryOptimizationRows[searchTermReportResponse](ctx, client, spec, body, 5000)
	if err != nil {
		return nil, err
	}
	result := make([]SearchTermPerformance, 0, len(items))
	for _, item := range items {
		country := strings.TrimSpace(item.Metadata.Country)
		if country != "" && !strings.EqualFold(country, request.Country) {
			continue
		}
		if term := strings.TrimSpace(item.Metadata.SearchTermText); term != "" {
			result = append(result, SearchTermPerformance{
				Term: term, KeywordText: item.Metadata.Keyword.Text, MatchType: item.Metadata.Keyword.MatchType,
				Country: country, CampaignID: item.Metadata.CampaignID.Int64(), AdGroupID: item.Metadata.AdGroupID.Int64(),
				Impressions: item.TotalMetrics.Impressions, Taps: item.TotalMetrics.Taps,
				TapInstalls: item.TotalMetrics.TapInstalls, TotalInstalls: item.TotalMetrics.TotalInstalls,
				SpendAmount: item.TotalMetrics.LocalSpend.Amount, SpendCurrency: item.TotalMetrics.LocalSpend.Currency,
			})
		}
	}
	return result, nil
}

func queryOptimizationList[T any](ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any, pageSize int) ([]T, error) {
	return queryOptimizationListBounded[T](ctx, client, spec, body, pageSize, 0)
}

// queryOptimizationListBounded keeps the normal pagination behavior while
// allowing callers that only need a bounded prefix to stop as soon as that
// prefix has been collected. A non-positive maxItems retains the unbounded
// behavior used by the broader optimization plan.
func queryOptimizationListBounded[T any](ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any, pageSize, maxItems int) ([]T, error) {
	items, _, err := queryOptimizationListBoundedWithMore[T](ctx, client, spec, body, pageSize, maxItems)
	return items, err
}

func queryOptimizationListBoundedWithMore[T any](ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any, pageSize, maxItems int) ([]T, bool, error) {
	if pageSize <= 0 {
		pageSize = 1000
	}
	if maxItems > 0 && pageSize > maxItems {
		pageSize = maxItems
	}
	items := make([]T, 0)
	offset := 0
	for pages := 0; pages < appleads.MaxPlatformPaginationPages; pages++ {
		pageBody := cloneOptimizationBody(body)
		pageBody["pagination"] = optimizationRequestPagination(spec, offset, pageSize)
		raw, err := executeOptimizationQuery(ctx, client, spec, pageBody)
		if err != nil {
			if maxItems > 0 && len(items) > 0 {
				if len(items) > maxItems {
					items = items[:maxItems]
				}
				return items, true, err
			}
			return items, false, err
		}
		var envelope struct {
			Result     []T                    `json:"result"`
			Pagination optimizationPagination `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			if maxItems > 0 && len(items) > 0 {
				if len(items) > maxItems {
					items = items[:maxItems]
				}
				return items, true, fmt.Errorf("decode %s response: %w", strings.Join(spec.CommandPath, " "), err)
			}
			return items, false, fmt.Errorf("decode %s response: %w", strings.Join(spec.CommandPath, " "), err)
		}
		items = append(items, envelope.Result...)
		if maxItems > 0 && len(items) >= maxItems {
			more := len(items) > maxItems
			if !more && envelope.Pagination.TotalCountPresent {
				more = envelope.Pagination.TotalCount > offset+len(envelope.Result)
			}
			if !more && !envelope.Pagination.TotalCountPresent && len(envelope.Result) >= pageSize {
				more, err = probeOptimizationListHasMore(ctx, client, spec, body, offset+len(envelope.Result))
				if err != nil {
					return items[:maxItems], true, fmt.Errorf("pagination truncation probe for %s failed: %w", strings.Join(spec.CommandPath, " "), err)
				}
			}
			return items[:maxItems], more, nil
		}
		if optimizationPageComplete(offset, pageSize, len(envelope.Result), envelope.Pagination.TotalCount) {
			return items, false, nil
		}
		offset += len(envelope.Result)
	}
	if maxItems > 0 && len(items) > 0 {
		if len(items) > maxItems {
			items = items[:maxItems]
		}
		return items, true, optimizationPageLimitError(spec)
	}
	return items, false, optimizationPageLimitError(spec)
}

// probeOptimizationListHasMore checks one record beyond a bounded prefix when
// Apple omits pagination.totalCount. The probe is deliberately one item wide
// so bounded callers do not silently turn into an unbounded collection.
func probeOptimizationListHasMore(ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any, offset int) (bool, error) {
	pageBody := cloneOptimizationBody(body)
	pageBody["pagination"] = optimizationRequestPagination(spec, offset, 1)
	raw, err := executeOptimizationQuery(ctx, client, spec, pageBody)
	if err != nil {
		return false, err
	}
	var envelope struct {
		Result []json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return false, fmt.Errorf("decode %s probe response: %w", strings.Join(spec.CommandPath, " "), err)
	}
	return len(envelope.Result) > 0, nil
}

func queryOptimizationRows[T any](ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any, pageSize int) ([]T, error) {
	items := make([]T, 0)
	offset := 0
	for pages := 0; pages < appleads.MaxPlatformPaginationPages; pages++ {
		pageBody := cloneOptimizationBody(body)
		pageBody["pagination"] = optimizationRequestPagination(spec, offset, pageSize)
		raw, err := executeOptimizationQuery(ctx, client, spec, pageBody)
		if err != nil {
			return items, err
		}
		var envelope struct {
			Result struct {
				Rows []T `json:"rows"`
			} `json:"result"`
			Pagination optimizationPagination `json:"pagination"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return items, fmt.Errorf("decode %s response: %w", strings.Join(spec.CommandPath, " "), err)
		}
		items = append(items, envelope.Result.Rows...)
		if optimizationPageComplete(offset, pageSize, len(envelope.Result.Rows), envelope.Pagination.TotalCount) {
			return items, nil
		}
		offset += len(envelope.Result.Rows)
	}
	return items, optimizationPageLimitError(spec)
}

// optimizationPageLimitError reports that a body-paginated Apple Ads query kept
// returning full pages past the shared safety bound. The pages already fetched
// are returned with it so callers can still record partial evidence.
func optimizationPageLimitError(spec appleads.EndpointSpec) error {
	return fmt.Errorf("platform API v1 pagination for %s exceeded the %d-page safety limit; narrow the request filters or time range", strings.Join(spec.CommandPath, " "), appleads.MaxPlatformPaginationPages)
}

func executeOptimizationQuery(ctx context.Context, client *appleads.Client, spec appleads.EndpointSpec, body map[string]any) (appleads.RawResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	requestCtx, cancel := shared.ContextWithTimeout(ctx)
	defer cancel()
	return client.Do(requestCtx, spec, nil, nil, payload)
}

func optimizationPageComplete(offset, pageSize, count, total int) bool {
	if count == 0 {
		return true
	}
	if total > 0 {
		return offset+count >= total
	}
	return count < pageSize
}

func optimizationRequestPagination(spec appleads.EndpointSpec, offset, pageSize int) map[string]any {
	pagination := map[string]any{"offset": offset, "pageSize": pageSize}
	if spec.BodyType == "QueryRequest" {
		pagination["fetchTotalCount"] = true
	}
	return pagination
}

func optimizationRawObjectCount(raw json.RawMessage) int {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("{}")) {
		return 0
	}
	return 1
}

func cloneOptimizationBody(body map[string]any) map[string]any {
	clone := make(map[string]any, len(body)+1)
	for key, value := range body {
		clone[key] = value
	}
	return clone
}

func mustOptimizationEndpoint(path ...string) appleads.EndpointSpec {
	spec, ok := appleads.PlatformEndpointByCommandPath(path...)
	if !ok {
		panic("missing Platform API endpoint: " + strings.Join(path, " "))
	}
	return spec
}

func optimizationFilter(field, operator string, value any) map[string]any {
	return map[string]any{"field": field, "operator": operator, "value": value}
}

func containsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func joinedOptimizationError(messages []string) error {
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

type flexibleInt64 int64

func (value *flexibleInt64) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if trimmed == "" || trimmed == "null" {
		*value = 0
		return nil
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return err
	}
	*value = flexibleInt64(parsed)
	return nil
}

func (value flexibleInt64) Int64() int64 { return int64(value) }

type optimizationPagination struct {
	TotalCount        int  `json:"totalCount"`
	TotalCountPresent bool `json:"-"`
}

func (pagination *optimizationPagination) UnmarshalJSON(data []byte) error {
	var raw struct {
		TotalCount *int `json:"totalCount"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	pagination.TotalCount = 0
	pagination.TotalCountPresent = raw.TotalCount != nil
	if raw.TotalCount != nil {
		pagination.TotalCount = *raw.TotalCount
	}
	return nil
}

type suggestionResponse struct {
	Text       string `json:"text"`
	Phrase     string `json:"phrase"`
	Popularity *int   `json:"popularity"`
}

type popularityResponse struct {
	Week              string `json:"week"`
	Month             string `json:"month"`
	Country           string `json:"countryOrRegion"`
	Genre             string `json:"genre"`
	SearchTerm        string `json:"searchTerm"`
	RankInGenre       *int   `json:"rankInGenre"`
	PopularityInGenre *int   `json:"searchPopularityInGenre"`
	Popularity100     *int   `json:"searchPopularity1to100"`
	Popularity5       *int   `json:"searchPopularity1to5"`
}

type impressionShareResponse struct {
	Day         string   `json:"day"`
	Week        string   `json:"week"`
	Country     string   `json:"countryOrRegion"`
	SearchTerm  string   `json:"searchTerm"`
	Low         *float64 `json:"lowImpressionShare"`
	High        *float64 `json:"highImpressionShare"`
	Rank        *int     `json:"rank"`
	Popularity5 *int     `json:"searchPopularity1to5"`
}

type campaignResponse struct {
	ID            flexibleInt64 `json:"id"`
	Name          string        `json:"name"`
	Status        string        `json:"status"`
	DisplayStatus string        `json:"displayStatus"`
	Targeting     struct {
		CountryOrRegion struct {
			Include []string `json:"include"`
		} `json:"countryOrRegion"`
	} `json:"targeting"`
}

type keywordResponse struct {
	CampaignID flexibleInt64 `json:"campaignId"`
	AdGroupID  flexibleInt64 `json:"adGroupId"`
	Text       string        `json:"text"`
	MatchType  string        `json:"matchType"`
	Status     string        `json:"status"`
}

type negativeKeywordResponse struct {
	CampaignID flexibleInt64 `json:"campaignId"`
	AdGroupID  flexibleInt64 `json:"adGroupId"`
	Text       string        `json:"text"`
	MatchType  string        `json:"matchType"`
	Status     string        `json:"status"`
}

type eligibilityResponse struct {
	Country         string   `json:"countryOrRegion"`
	SupplyPlacement string   `json:"supplyPlacement"`
	DeviceClass     string   `json:"deviceClass"`
	State           string   `json:"state"`
	Reasons         []string `json:"reasons"`
}

type searchTermReportResponse struct {
	Metadata struct {
		SearchTermText string        `json:"searchTermText"`
		CampaignID     flexibleInt64 `json:"campaignId"`
		AdGroupID      flexibleInt64 `json:"adGroupId"`
		Country        string        `json:"countryOrRegion"`
		Keyword        struct {
			Text      string `json:"text"`
			MatchType string `json:"matchType"`
		} `json:"keyword"`
	} `json:"metadata"`
	TotalMetrics struct {
		LocalSpend struct {
			Amount   string `json:"amount"`
			Currency string `json:"currency"`
		} `json:"localSpend"`
		Impressions   int64 `json:"impressions"`
		Taps          int64 `json:"taps"`
		TapInstalls   int64 `json:"tapInstalls"`
		TotalInstalls int64 `json:"totalInstalls"`
	} `json:"totalMetrics"`
}
