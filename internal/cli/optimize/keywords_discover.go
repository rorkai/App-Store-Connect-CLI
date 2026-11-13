package optimize

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

const (
	keywordDiscoverSchemaVersion = "1"
	keywordDiscoverDefaultLimit  = 50

	keywordSuggestionSourceKeyword = "keyword_suggestions"
	keywordSuggestionSourcePhrase  = "phrase_suggestions"
)

var collectSearchDataForDiscover = ads.CollectSearchSuggestions

// KeywordsDiscoverCommand returns the official Apple Ads keyword suggestion
// command.
func KeywordsDiscoverCommand() *ffcli.Command {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	appID := fs.String("app", "", "App Store app ID (required, or ASC_APP_ID env)")
	country := fs.String("country", "us", "ISO alpha-2 Apple Ads country or region")
	genre := fs.String("genre", "", "Apple Ads genre; optional report label (does not affect suggestions)")
	adAccount := fs.String("ad-account", "", "Apple Ads ad account ID (or ASC_ADS_AD_ACCOUNT_ID/profile default)")
	adsProfile := fs.String("ads-profile", "", "Use named Apple Ads authentication profile")
	limit := fs.Int("limit", keywordDiscoverDefaultLimit, "Maximum suggestions to return")
	output := shared.BindOutputFlags(fs)

	return &ffcli.Command{
		Name:       "discover",
		ShortUsage: "asc optimize keywords discover --app APP_ID [flags]",
		ShortHelp:  "List official Apple Ads keyword suggestions for an app.",
		LongHelp: `List keyword candidates from Apple's official Ads suggestion endpoints.

Suggestions come only from Apple's documented keyword and phrase suggestion
endpoints. No campaign is required. Undocumented endpoints, including the
iTunes search autocomplete endpoint, are deliberately out of scope.

--country scopes keyword suggestions. Apple's phrase suggestion endpoint is
country-unscoped; the report's country is the requested keyword market and
report context, and each source declares its scope.

Terms are normalized, deduplicated across both endpoints, and reported with the
endpoint each one came from. Apple Ads is the only source for this command, so
unlike the other keyword commands it fails when that source is unavailable
instead of degrading. Credentials come from --ad-account, --ads-profile, the
ASC_ADS_AD_ACCOUNT_ID environment variable, or a stored Apple Ads profile.

The report also carries scoreKeywords, a comma-separated list of the
suggestions that satisfy keyword hygiene, ready to pass straight to
` + "`asc optimize keywords score --keywords`" + `. Suggestions that are too long, too
short, too many words, or contain the comma delimiter remain listed but are
left out of that field.

Examples:
  asc optimize keywords discover --app "1234567890" --country US
  asc optimize keywords discover --app "1234567890" --country US --ad-account "987654321" --limit 25
  asc optimize keywords discover --app "1234567890" --country US --output table`,
		FlagSet:   fs,
		UsageFunc: shared.DefaultUsageFunc,
		Exec: func(ctx context.Context, args []string) error {
			resolvedAppID, err := normalizeKeywordAppID(*appID)
			if err != nil {
				return err
			}
			normalizedCountry, err := normalizeKeywordCountry(*country)
			if err != nil {
				return err
			}
			normalizedGenre := strings.ToUpper(strings.TrimSpace(*genre))
			if normalizedGenre != "" && !searchPlanGenrePattern.MatchString(normalizedGenre) {
				return shared.UsageError("--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES")
			}
			if *limit < 1 {
				return shared.UsageError("--limit must be at least 1")
			}

			requestCtx, cancel := shared.ContextWithTimeout(ctx)
			defer cancel()
			data, err := collectSearchDataForDiscover(requestCtx, *adsProfile, *adAccount, ads.SearchOptimizationRequest{
				AppID:   resolvedAppID,
				Country: strings.ToUpper(normalizedCountry),
				Genre:   normalizedGenre,
				Limit:   *limit,
			})
			if err != nil {
				return keywordDiscoverUnavailableError(err.Error())
			}

			report := buildKeywordDiscoverReport(keywordDiscoverBuildInput{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				AppID:       resolvedAppID,
				Country:     strings.ToUpper(normalizedCountry),
				Genre:       normalizedGenre,
				Limit:       *limit,
				Sources:     suggestionSources(data.Sources),
				Suggestions: data.Suggestions,
				Truncated:   data.SuggestionsTruncated,
			})
			if len(report.Keywords) == 0 {
				if cause := unavailableSuggestionCause(report.Sources); cause != "" {
					return keywordDiscoverUnavailableError(cause)
				}
			}

			return shared.PrintOutput(&report, *output.Output, *output.Pretty)
		},
	}
}

type keywordDiscoverBuildInput struct {
	GeneratedAt string
	AppID       string
	Country     string
	Genre       string
	Limit       int
	Sources     []asc.KeywordDiscoverSourceStatus
	Suggestions []ads.SearchSuggestion
	Truncated   bool
}

func buildKeywordDiscoverReport(input keywordDiscoverBuildInput) asc.KeywordDiscoverReport {
	seen := make(map[string]int, len(input.Suggestions))
	suggestions := make([]asc.KeywordSuggestion, 0, len(input.Suggestions))
	duplicates := 0

	for _, suggestion := range input.Suggestions {
		keyword := strings.ToLower(strings.Join(strings.Fields(suggestion.Text), " "))
		if keyword == "" {
			continue
		}
		candidate := asc.KeywordSuggestion{
			Keyword:    keyword,
			Source:     strings.TrimSpace(suggestion.Kind),
			Popularity: suggestion.Popularity,
		}
		if index, duplicate := seen[keyword]; duplicate {
			duplicates++
			if greaterIntPointer(candidate.Popularity, suggestions[index].Popularity) {
				suggestions[index] = candidate
			}
			continue
		}
		seen[keyword] = len(suggestions)
		suggestions = append(suggestions, candidate)
	}

	// The keyword and phrase endpoints are independently bounded, so merge
	// their deduplicated results by popularity before applying the report's
	// global limit.
	sort.SliceStable(suggestions, func(left, right int) bool {
		return pointerIntValue(suggestions[left].Popularity, -1) > pointerIntValue(suggestions[right].Popularity, -1)
	})

	summary := asc.KeywordDiscoverSummary{Available: len(suggestions)}
	truncated := input.Truncated
	if input.Limit > 0 && len(suggestions) > input.Limit {
		suggestions = suggestions[:input.Limit]
		truncated = true
	}

	scoreReady := make([]string, 0, len(suggestions))
	for _, suggestion := range suggestions {
		summary.Suggestions++
		switch suggestion.Source {
		case "phrase":
			summary.PhraseSource++
		default:
			summary.KeywordSource++
		}
		if len(scoreReady) < keywordMaxCount && isScoreReadyKeyword(suggestion.Keyword) {
			scoreReady = append(scoreReady, suggestion.Keyword)
		}
	}
	summary.Duplicates = duplicates
	summary.ScoreReady = len(scoreReady)

	return asc.KeywordDiscoverReport{
		SchemaVersion: keywordDiscoverSchemaVersion,
		GeneratedAt:   input.GeneratedAt,
		AppID:         input.AppID,
		Country:       input.Country,
		Genre:         input.Genre,
		Limit:         input.Limit,
		Truncated:     truncated,
		Sources:       input.Sources,
		Summary:       summary,
		ScoreKeywords: strings.Join(scoreReady, ","),
		Keywords:      suggestions,
	}
}

// isScoreReadyKeyword reports whether a suggestion satisfies the same keyword
// hygiene that `asc optimize keywords score` enforces, so scoreKeywords can be
// passed straight through without being rejected.
func isScoreReadyKeyword(keyword string) bool {
	if strings.ContainsRune(keyword, ',') {
		return false
	}
	_, err := normalizeKeywordList(keyword)
	return err == nil
}

func suggestionSources(sources []ads.SearchOptimizationSourceStatus) []asc.KeywordDiscoverSourceStatus {
	filtered := make([]asc.KeywordDiscoverSourceStatus, 0, 2)
	for _, source := range sources {
		if source.Name == keywordSuggestionSourceKeyword || source.Name == keywordSuggestionSourcePhrase {
			filtered = append(filtered, asc.KeywordDiscoverSourceStatus{
				Name:   source.Name,
				Status: source.Status,
				Count:  source.Count,
				Scope:  suggestionScope(source.Name),
				Error:  source.Error,
			})
		}
	}
	return filtered
}

func unavailableSuggestionCause(sources []asc.KeywordDiscoverSourceStatus) string {
	causes := make([]string, 0, len(sources))
	for _, source := range sources {
		if source.Status == keywordStatusUnavailable {
			causes = append(causes, source.Name+": "+source.Error)
		}
	}
	if len(causes) == 0 {
		return ""
	}
	return strings.Join(causes, "; ")
}

func suggestionScope(name string) string {
	if name == keywordSuggestionSourceKeyword {
		return "country"
	}
	if name == keywordSuggestionSourcePhrase {
		return "unscoped"
	}
	return ""
}

func keywordDiscoverUnavailableError(cause string) error {
	return fmt.Errorf(
		"optimize keywords discover: Apple Ads keyword suggestions are unavailable: %s. "+
			"Check Apple Ads authentication, ad-account access, and service/network availability, then retry. "+
			"Diagnose access with `asc ads auth status --validate`; select credentials with --ad-account or --ads-profile",
		cause,
	)
}
