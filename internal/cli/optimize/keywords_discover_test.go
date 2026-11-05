package optimize

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestKeywordsDiscoverCommandValidatesInputBeforeRequests(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing app",
			args: []string{"--country", "US"},
			want: "--app is required (or set ASC_APP_ID)",
		},
		{
			name: "non numeric app",
			args: []string{"--app", "com.example.app", "--country", "US"},
			want: "--app must be a numeric App Store app ID",
		},
		{
			name: "zero app",
			args: []string{"--app", "0", "--country", "US"},
			want: "--app must be a positive App Store app ID representable as a 64-bit integer",
		},
		{
			name: "overflow app",
			args: []string{"--app", "9223372036854775808", "--country", "US"},
			want: "--app must be a positive App Store app ID representable as a 64-bit integer",
		},
		{
			name: "country",
			args: []string{"--app", "1234567890", "--country", "usa"},
			want: `--country "usa" is not a supported App Store storefront`,
		},
		{
			name: "genre",
			args: []string{"--app", "1234567890", "--genre", "bad genre"},
			want: "--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES",
		},
		{
			name: "limit",
			args: []string{"--app", "1234567890", "--limit", "0"},
			want: "--limit must be at least 1",
		},
		{
			name: "positional",
			args: nil,
			want: "optimize keywords discover does not accept positional arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			failKeywordsAdsCollector(t)
			command := KeywordsDiscoverCommand()
			var err error
			if test.args == nil {
				err = command.Exec(context.Background(), []string{"extra"})
			} else {
				err = command.ParseAndRun(context.Background(), test.args)
			}
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestKeywordsDiscoverBoundsAdsCollectionWithCommandTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1s")
	t.Setenv("ASC_TIMEOUT_SECONDS", "")
	stubKeywordsAdsCollector(t, func(ctx context.Context, _, _ string, _ ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("Apple Ads collection context has no deadline")
		}
		if remaining := time.Until(deadline); remaining <= 0 || remaining > time.Second {
			t.Fatalf("Apple Ads collection deadline remaining = %v, want within 1s", remaining)
		}
		return ads.SearchOptimizationData{
			Sources:     []ads.SearchOptimizationSourceStatus{{Name: keywordSuggestionSourceKeyword, Status: keywordStatusAvailable, Count: 1}},
			Suggestions: []ads.SearchSuggestion{{Text: "focus timer", Kind: "keyword"}},
		}, nil
	})

	captureSearchPlanStdout(t, func() error {
		return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890", "--country", "US", "--output", "json",
		})
	})
}

func TestKeywordsDiscoverFlattensDedupesAndPreparesScoreInput(t *testing.T) {
	stubKeywordsAdsCollector(t, func(_ context.Context, profile, account string, request ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		if profile != "Ads" || account != "987654321" {
			t.Fatalf("ads credentials = (%q, %q)", profile, account)
		}
		if request.AppID != "1234567890" || request.Country != "US" || request.Limit != keywordDiscoverDefaultLimit {
			t.Fatalf("ads request = %+v", request)
		}
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "available", Count: 4},
				{Name: "phrase_suggestions", Status: "available", Count: 3},
			},
			Suggestions: []ads.SearchSuggestion{
				{Text: "Focus Timer", Popularity: intPtr(61), Kind: "keyword"},
				{Text: "habit   tracker", Popularity: intPtr(44), Kind: "keyword"},
				{Text: "  ", Kind: "keyword"},
				{Text: "a", Kind: "keyword"},
				{Text: "focus timer", Popularity: intPtr(60), Kind: "phrase"},
				{Text: "deep work sessions for focus", Kind: "phrase"},
				{Text: "pomodoro", Popularity: intPtr(20), Kind: "phrase"},
			},
		}, nil
	})

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890",
			"--country", "us",
			"--ad-account", "987654321",
			"--ads-profile", "Ads",
			"--output", "json",
		})
	})

	var report asc.KeywordDiscoverReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.SchemaVersion != "1" || report.AppID != "1234567890" || report.Country != "US" {
		t.Fatalf("unexpected report identity: %+v", report)
	}

	want := []asc.KeywordSuggestion{
		{Keyword: "focus timer", Source: "keyword", Popularity: intPtr(61)},
		{Keyword: "habit tracker", Source: "keyword", Popularity: intPtr(44)},
		{Keyword: "pomodoro", Source: "phrase", Popularity: intPtr(20)},
		{Keyword: "a", Source: "keyword"},
		{Keyword: "deep work sessions for focus", Source: "phrase"},
	}
	if len(report.Keywords) != len(want) {
		t.Fatalf("keywords = %+v, want %d entries", report.Keywords, len(want))
	}
	for index, expected := range want {
		got := report.Keywords[index]
		if got.Keyword != expected.Keyword || got.Source != expected.Source {
			t.Fatalf("keywords[%d] = %+v, want %+v", index, got, expected)
		}
		if (got.Popularity == nil) != (expected.Popularity == nil) {
			t.Fatalf("keywords[%d] popularity = %v, want %v", index, got.Popularity, expected.Popularity)
		}
		if got.Popularity != nil && *got.Popularity != *expected.Popularity {
			t.Fatalf("keywords[%d] popularity = %d, want %d", index, *got.Popularity, *expected.Popularity)
		}
	}

	if report.Summary.Duplicates != 1 {
		t.Fatalf("duplicates = %d, want 1", report.Summary.Duplicates)
	}
	if report.Summary.Suggestions != 5 || report.Summary.KeywordSource != 3 || report.Summary.PhraseSource != 2 {
		t.Fatalf("summary = %+v", report.Summary)
	}

	// Only suggestions that satisfy keyword hygiene are offered to `score`.
	if report.ScoreKeywords != "focus timer,habit tracker,pomodoro" {
		t.Fatalf("scoreKeywords = %q", report.ScoreKeywords)
	}
	if report.Summary.ScoreReady != 3 {
		t.Fatalf("scoreReady = %d, want 3", report.Summary.ScoreReady)
	}
}

func TestKeywordDiscoverScoreInputExcludesCommaDelimitedSuggestions(t *testing.T) {
	report := buildKeywordDiscoverReport(keywordDiscoverBuildInput{
		Limit: 10,
		Suggestions: []ads.SearchSuggestion{
			{Text: "photo editor, filters", Kind: "keyword"},
			{Text: "focus timer", Kind: "keyword"},
		},
	})

	if report.ScoreKeywords != "focus timer" {
		t.Fatalf("scoreKeywords = %q, want only the delimiter-safe suggestion", report.ScoreKeywords)
	}
	if report.Summary.ScoreReady != 1 {
		t.Fatalf("scoreReady = %d, want 1", report.Summary.ScoreReady)
	}
	if len(report.Keywords) != 2 {
		t.Fatalf("keywords = %+v, want both suggestions preserved for inspection", report.Keywords)
	}
}

func TestKeywordDiscoverReportUsesRegisteredOutput(t *testing.T) {
	report := asc.KeywordDiscoverReport{
		AppID:   "1234567890",
		Country: "US",
		Summary: asc.KeywordDiscoverSummary{Suggestions: 1, Available: 1, ScoreReady: 1},
		Keywords: []asc.KeywordSuggestion{{
			Keyword: "focus timer",
			Source:  "keyword",
		}},
	}

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout := captureSearchPlanStdout(t, func() error {
				return shared.PrintOutput(&report, format, false)
			})
			if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
				t.Fatalf("%s output fell back to JSON:\n%s", format, stdout)
			}
			for _, want := range []string{"focus timer", "Keyword", "Source"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

func TestKeywordDiscoverJSONKeepsEmptyScoreKeywords(t *testing.T) {
	report := asc.KeywordDiscoverReport{
		AppID:   "1234567890",
		Country: "US",
		Summary: asc.KeywordDiscoverSummary{Available: 1, Suggestions: 1},
		Keywords: []asc.KeywordSuggestion{{
			Keyword: "a",
			Source:  "keyword",
		}},
	}

	payload, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	value, present := decoded["scoreKeywords"]
	if !present {
		t.Fatalf("JSON = %s, want an explicit scoreKeywords handoff field", payload)
	}
	if value != "" {
		t.Fatalf("scoreKeywords = %#v, want an empty string", value)
	}
}

func TestKeywordsDiscoverAppliesLimitAndReportsTruncation(t *testing.T) {
	suggestions := make([]ads.SearchSuggestion, 0, 6)
	for index := range 6 {
		suggestions = append(suggestions, ads.SearchSuggestion{Text: fmt.Sprintf("keyword%03d", index), Kind: "keyword"})
	}
	stubKeywordsAdsCollector(t, func(_ context.Context, _, _ string, request ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		if request.Limit != 2 {
			t.Fatalf("ads request limit = %d, want 2", request.Limit)
		}
		return ads.SearchOptimizationData{Suggestions: suggestions}, nil
	})

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890", "--limit", "2", "--output", "json",
		})
	})

	var report asc.KeywordDiscoverReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if len(report.Keywords) != 2 || report.Limit != 2 || !report.Truncated {
		t.Fatalf("report = %+v", report)
	}
	if report.Summary.Suggestions != 2 {
		t.Fatalf("summary must count what was returned: %+v", report.Summary)
	}
	if report.ScoreKeywords != "keyword000,keyword001" {
		t.Fatalf("scoreKeywords = %q", report.ScoreKeywords)
	}
}

func TestKeywordDiscoverSortsBothSuggestionStreamsBeforeLimit(t *testing.T) {
	report := buildKeywordDiscoverReport(keywordDiscoverBuildInput{
		Limit: 2,
		Suggestions: []ads.SearchSuggestion{
			{Text: "keyword lower", Popularity: intPtr(50), Kind: "keyword"},
			{Text: "phrase higher", Popularity: intPtr(90), Kind: "phrase"},
			{Text: "keyword higher", Popularity: intPtr(80), Kind: "keyword"},
			{Text: "phrase lower", Popularity: intPtr(40), Kind: "phrase"},
		},
	})

	if !report.Truncated || report.Summary.Available != 4 || report.Summary.Suggestions != 2 {
		t.Fatalf("summary = %+v, truncated = %t", report.Summary, report.Truncated)
	}
	if len(report.Keywords) != 2 || report.Keywords[0].Keyword != "phrase higher" || report.Keywords[1].Keyword != "keyword higher" {
		t.Fatalf("keywords = %+v, want the highest-popularity terms across both endpoints", report.Keywords)
	}
}

func TestKeywordsDiscoverFailsActionablyWhenAppleAdsIsUnavailable(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{}, errors.New("no Apple Ads credentials found")
	})

	err := KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
		"--app", "1234567890", "--output", "json",
	})
	if err == nil {
		t.Fatal("expected an error when the only source is unavailable")
	}
	for _, want := range []string{
		"optimize keywords discover",
		"no Apple Ads credentials found",
		"--ad-account",
		"--ads-profile",
		"asc ads auth status --validate",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want it to mention %q", err, want)
		}
	}
}

func TestKeywordsDiscoverFailsWhenAppleReturnsNoSuggestions(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "unavailable", Error: "forbidden"},
				{Name: "phrase_suggestions", Status: "unavailable", Error: "forbidden"},
			},
		}, nil
	})

	err := KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
		"--app", "1234567890", "--output", "json",
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("error = %v, want the reported suggestion failure", err)
	}
	for _, want := range []string{"service/network availability", "then retry", "asc ads auth status --validate"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %v, want neutral recovery guidance containing %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "auth login") {
		t.Fatalf("error = %v, must not prescribe reauthentication for service failures", err)
	}
}

func TestKeywordsDiscoverFailsWhenOneSuggestionSourceIsUnavailable(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "unavailable", Error: "forbidden"},
				{Name: "phrase_suggestions", Status: "empty"},
			},
		}, nil
	})

	err := KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
		"--app", "1234567890", "--output", "json",
	})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("error = %v, want the unavailable source diagnostic", err)
	}
}

func TestKeywordsDiscoverReportsSourceScopes(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "available", Count: 1},
				{Name: "phrase_suggestions", Status: "available", Count: 1},
			},
			Suggestions: []ads.SearchSuggestion{
				{Text: "keyword one", Kind: "keyword"},
				{Text: "phrase one", Kind: "phrase"},
			},
		}, nil
	})

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890", "--country", "GB", "--output", "json",
		})
	})
	var report asc.KeywordDiscoverReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if len(report.Sources) != 2 {
		t.Fatalf("sources = %+v", report.Sources)
	}
	if report.Sources[0].Scope != "country" || report.Sources[1].Scope != "unscoped" {
		t.Fatalf("source scopes = %+v, want country and unscoped", report.Sources)
	}
}

func TestKeywordsDiscoverReportsEmptySuggestionsWithoutFailing(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{
				{Name: "keyword_suggestions", Status: "empty"},
				{Name: "phrase_suggestions", Status: "empty"},
			},
		}, nil
	})

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890", "--output", "json",
		})
	})

	var report asc.KeywordDiscoverReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if len(report.Keywords) != 0 || report.ScoreKeywords != "" {
		t.Fatalf("report = %+v", report)
	}
	wantSources := map[string]string{
		"keyword_suggestions": keywordStatusEmpty,
		"phrase_suggestions":  keywordStatusEmpty,
	}
	if len(report.Sources) != len(wantSources) {
		t.Fatalf("sources = %+v, want %d entries", report.Sources, len(wantSources))
	}
	for _, source := range report.Sources {
		wantStatus, ok := wantSources[source.Name]
		if !ok || source.Status != wantStatus {
			t.Fatalf("source = %+v, want an expected empty source", source)
		}
		delete(wantSources, source.Name)
	}
	if len(wantSources) != 0 {
		t.Fatalf("sources = %+v, missing expected entries", report.Sources)
	}
}

func TestKeywordsDiscoverRendersTableAndMarkdown(t *testing.T) {
	stubKeywordsAdsCollector(t, func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error) {
		return ads.SearchOptimizationData{
			Sources: []ads.SearchOptimizationSourceStatus{{Name: "keyword_suggestions", Status: "available", Count: 1}},
			Suggestions: []ads.SearchSuggestion{
				{Text: "focus timer", Popularity: intPtr(61), Kind: "keyword"},
				{Text: "deep work", Kind: "phrase"},
			},
		}, nil
	})

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout := captureSearchPlanStdout(t, func() error {
				return KeywordsDiscoverCommand().ParseAndRun(context.Background(), []string{
					"--app", "1234567890", "--output", format,
				})
			})
			normalized := strings.ToLower(stdout)
			for _, want := range []string{"keyword", "source", "popularity", "focus timer", "deep work", "phrase", "sources"} {
				if !strings.Contains(normalized, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

func stubKeywordsAdsCollector(
	t *testing.T,
	collect func(context.Context, string, string, ads.SearchOptimizationRequest) (ads.SearchOptimizationData, error),
) {
	t.Helper()
	previous := collectSearchDataForDiscover
	t.Cleanup(func() { collectSearchDataForDiscover = previous })
	collectSearchDataForDiscover = collect
}
