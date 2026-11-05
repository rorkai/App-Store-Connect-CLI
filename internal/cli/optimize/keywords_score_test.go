package optimize

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/ads"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/shared"
)

func TestKeywordsScoreCommandValidatesInputBeforeRequests(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing keywords",
			args: nil,
			want: "--keywords is required",
		},
		{
			name: "keyword budget",
			args: []string{"--keywords", strings.Join(manyKeywords(101), ",")},
			want: "--keywords accepts at most 100 keywords per invocation",
		},
		{
			name: "keyword word budget",
			args: []string{"--keywords", "one two three four five"},
			want: `--keywords entry "one two three four five" must contain at most 4 space-separated words`,
		},
		{
			name: "workers",
			args: []string{"--keywords", "focus timer", "--workers", "0"},
			want: "--workers must be at least 1",
		},
		{
			name: "country",
			args: []string{"--keywords", "focus timer", "--country", "usa"},
			want: `--country "usa" is not a supported App Store storefront`,
		},
		{
			name: "app id",
			args: []string{"--keywords", "focus timer", "--app", "com.example.app"},
			want: "--app must be a numeric App Store app ID",
		},
		{
			name: "genre",
			args: []string{"--keywords", "focus timer", "--genre", "bad genre"},
			want: "--genre must be an Apple Ads genre identifier such as PRODUCTIVITY_UTILITIES",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			failKeywordsClient(t)
			failKeywordsAdsCollector(t)
			err := KeywordsScoreCommand().ParseAndRun(context.Background(), test.args)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestKeywordScoreReportUsesRegisteredOutput(t *testing.T) {
	report := asc.KeywordScoreReport{
		Country: "US",
		Summary: asc.KeywordScoreSummary{Keywords: 1, Scored: 1},
		Rows: []asc.KeywordScoreRow{{
			Keyword: "focus timer",
			Status:  keywordStatusUnavailable,
			Error:   "keyword lookup returned status 429",
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
			for _, want := range []string{"focus timer", "Keyword", "Status", "429"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

func TestKeywordScoreReportRendersFallbackState(t *testing.T) {
	difficulty := 1.0
	minimum := 1.0
	report := asc.KeywordScoreReport{
		Country: "US",
		Summary: asc.KeywordScoreSummary{Keywords: 1, Scored: 1},
		Rows: []asc.KeywordScoreRow{{
			Keyword:            "thin result",
			Status:             keywordStatusAvailable,
			DifficultyScore:    &difficulty,
			MinDifficultyScore: &minimum,
			Fallback:           true,
		}},
	}

	for _, format := range []string{"json", "table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout := captureSearchPlanStdout(t, func() error {
				return shared.PrintOutput(&report, format, false)
			})
			if format == "json" {
				if !strings.Contains(stdout, `"fallback":true`) {
					t.Fatalf("JSON output missing fallback state:\n%s", stdout)
				}
				return
			}
			if strings.HasPrefix(strings.TrimSpace(stdout), "{") {
				t.Fatalf("%s output fell back to JSON:\n%s", format, stdout)
			}
			for _, want := range []string{"Fallback", "true", "thin result"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

func TestKeywordsScoreComposesCompetitionRankAndDegradesPopularity(t *testing.T) {
	var lookupIDs []string
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			if got := r.URL.Query().Get("country"); got != "us" {
				t.Errorf("country = %q, want us", got)
			}
			switch r.URL.Query().Get("term") {
			case "focus timer":
				writeScoreSearchResults(w, scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000},
					scoreSearchApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000},
					scoreSearchApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900},
					scoreSearchApp{ID: 444, Name: "Pomodoro Pal", Seller: "Delta Labs", Rating: 4.1, Count: 250},
					scoreSearchApp{ID: 1234567890, Name: "My Focus App", Seller: "Mine", Rating: 4.9, Count: 30},
					scoreSearchApp{ID: 555, Name: "Sixth App", Seller: "Zeta Labs", Rating: 3.0, Count: 10})
			case "tiny niche":
				writeScoreSearchResults(w, scoreSearchApp{ID: 777, Name: "Tiny", Seller: "Solo", Rating: 5, Count: 3})
			case "broken keyword":
				w.WriteHeader(http.StatusTooManyRequests)
			default:
				t.Errorf("unexpected term %q", r.URL.Query().Get("term"))
				w.WriteHeader(http.StatusInternalServerError)
			}
		case "/lookup":
			mu.Lock()
			lookupIDs = append(lookupIDs, r.URL.Query().Get("id"))
			mu.Unlock()
			if got := r.URL.Query().Get("entity"); got != "software" {
				t.Errorf("entity = %q, want software", got)
			}
			writeScoreLookupResults(w,
				scoreLookupApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000, Release: daysAgo(900), Updated: daysAgo(10)},
				scoreLookupApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000, Release: daysAgo(500), Updated: daysAgo(60)},
				scoreLookupApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900, Release: daysAgo(300), Updated: daysAgo(120)},
				scoreLookupApp{ID: 444, Name: "Pomodoro Pal", Seller: "Delta Labs", Rating: 4.1, Count: 250, Release: daysAgo(200), Updated: daysAgo(30)},
				scoreLookupApp{ID: 1234567890, Name: "My Focus App", Seller: "Mine", Rating: 4.9, Count: 30, Release: daysAgo(100), Updated: daysAgo(5)},
				scoreLookupApp{ID: 777, Name: "Tiny", Seller: "Solo", Rating: 5, Count: 3, Release: daysAgo(50), Updated: daysAgo(50)})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
			"--keywords", "focus timer,tiny niche,broken keyword",
			"--app", "1234567890",
			"--output", "json",
		})
	})

	var report asc.KeywordScoreReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.SchemaVersion != "1" || report.AppID != "1234567890" || report.Country != "US" {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if len(report.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(report.Rows))
	}

	scored := report.Rows[0]
	if scored.Status != keywordStatusAvailable || scored.DifficultyScore == nil || scored.MinDifficultyScore == nil {
		t.Fatalf("scored row = %+v", scored)
	}
	if scored.AppCount == nil || *scored.AppCount != 6 {
		t.Fatalf("scored row app count = %+v", scored.AppCount)
	}
	if scored.Rank == nil || *scored.Rank != 5 {
		t.Fatalf("scored row rank = %+v", scored.Rank)
	}
	if scored.KeywordMatch != keywordMatchTitleExactPhrase {
		t.Fatalf("scored row keyword match = %q", scored.KeywordMatch)
	}
	if len(scored.RawSignals) != keywordCompetitorSampleSize {
		t.Fatalf("raw signals = %d, want %d", len(scored.RawSignals), keywordCompetitorSampleSize)
	}
	if scored.RawSignals[0].ReleaseDate == "" || scored.RawSignals[0].CurrentVersionReleaseDate == "" {
		t.Fatalf("raw signals must carry hydrated release dates: %+v", scored.RawSignals[0])
	}
	if scored.RawSignals[0].AppScore <= 0 {
		t.Fatalf("raw signals must carry a per-app score: %+v", scored.RawSignals[0])
	}
	if scored.Popularity != nil {
		t.Fatalf("popularity must stay null without an Apple Ads source: %+v", scored.Popularity)
	}
	if scored.Fallback {
		t.Fatal("Fallback = true, want false for a full result window")
	}

	fallback := report.Rows[1]
	if !fallback.Fallback || fallback.DifficultyScore == nil || *fallback.DifficultyScore != 1 {
		t.Fatalf("fallback row = %+v", fallback)
	}
	if fallback.MinDifficultyScore == nil || *fallback.MinDifficultyScore != 1 {
		t.Fatalf("fallback row min difficulty = %+v", fallback.MinDifficultyScore)
	}

	failed := report.Rows[2]
	if failed.Status != keywordStatusUnavailable || failed.DifficultyScore != nil || failed.AppCount != nil || failed.IsBrandKeyword != nil {
		t.Fatalf("unavailable row must not invent scores: %+v", failed)
	}
	if !strings.Contains(failed.Error, "429") {
		t.Fatalf("unavailable row error = %q", failed.Error)
	}

	sources := map[string]asc.KeywordScoreSourceStatus{}
	for _, source := range report.Sources {
		sources[source.Name] = source
	}
	if got := sources[keywordSourcePopularity]; got.Status != keywordStatusUnavailable ||
		!strings.Contains(got.Error, "--genre") {
		t.Fatalf("popularity source = %+v, want an unavailable status naming the missing flag", got)
	}
	if got := sources[keywordSourceCompetition]; got.Status != keywordStatusAvailable || got.Count != 2 {
		t.Fatalf("competition source = %+v", got)
	}
	if got := sources[keywordSourceMetadata]; got.Status != keywordStatusAvailable {
		t.Fatalf("metadata source = %+v", got)
	}
	if got := sources[keywordSourceRank]; got.Status != keywordStatusAvailable || got.Count != 1 || !strings.Contains(got.Error, "429") {
		t.Fatalf("rank source = %+v", got)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(lookupIDs) != 1 {
		t.Fatalf("lookup requests = %v, want one deduplicated batch", lookupIDs)
	}
	if strings.Count(lookupIDs[0], "111") != 1 {
		t.Fatalf("lookup batch must deduplicate app IDs across keywords: %q", lookupIDs[0])
	}

	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal raw rows: %v", err)
	}
	for _, key := range []string{"popularity", "difficultyScore", "minDifficultyScore", "isBrandKeyword", "appCount"} {
		if string(raw.Rows[2][key]) != "null" {
			t.Fatalf("unavailable row %q = %s, want an explicit null", key, raw.Rows[2][key])
		}
	}
}

func TestKeywordsScoreReportsIncompleteCompetitorMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			writeScoreSearchResults(
				w,
				scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000},
				scoreSearchApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000},
				scoreSearchApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900},
			)
		case "/lookup":
			writeScoreLookupResults(
				w,
				scoreLookupApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000, Release: daysAgo(900), Updated: daysAgo(10)},
				scoreLookupApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000, Updated: daysAgo(20)},
				scoreLookupApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900, Release: daysAgo(300), Updated: "not-a-date"},
				scoreLookupApp{ID: 999, Name: "Unrequested", Seller: "Extra Labs", Rating: 5, Count: 10000, Release: daysAgo(10), Updated: daysAgo(1)},
			)
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	stdout := captureSearchPlanStdout(t, func() error {
		command := keywordsScoreCommand(func() time.Time { return difficultyReferenceTime })
		return command.ParseAndRun(context.Background(), []string{
			"--keywords", "focus timer", "--output", "json",
		})
	})

	var report asc.KeywordScoreReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	sources := map[string]asc.KeywordScoreSourceStatus{}
	for _, source := range report.Sources {
		sources[source.Name] = source
	}
	metadata, ok := sources[keywordSourceMetadata]
	if !ok {
		t.Fatalf("report is missing the %s source: %+v", keywordSourceMetadata, report.Sources)
	}
	if metadata.Status != keywordStatusAvailable || metadata.Count != 1 ||
		!strings.Contains(metadata.Error, "lookup returned incomplete required release metadata") ||
		!strings.Contains(metadata.Error, "2 of 3 requested app IDs") {
		t.Fatalf("metadata source = %+v, want one complete row plus an incomplete-metadata diagnostic", metadata)
	}
	if len(report.Rows) != 1 || len(report.Rows[0].RawSignals) != 3 {
		t.Fatalf("raw signals = %+v, want all three competitors", report.Rows)
	}
	if report.Rows[0].RawSignals[1].ReleaseDate != "" || report.Rows[0].RawSignals[1].CurrentVersionReleaseDate == "" {
		t.Fatalf("partial metadata must preserve the valid update date: %+v", report.Rows[0].RawSignals[1])
	}
	if report.Rows[0].RawSignals[1].DaysSinceFirstRelease != keywordMissingDateDays || report.Rows[0].RawSignals[1].DaysSinceLastRelease != 20 {
		t.Fatalf("partial metadata must use the valid update date while degrading the missing release date: %+v", report.Rows[0].RawSignals[1])
	}
	if report.Rows[0].RawSignals[2].ReleaseDate == "" || report.Rows[0].RawSignals[2].CurrentVersionReleaseDate != "" {
		t.Fatalf("partial metadata must preserve the valid release date: %+v", report.Rows[0].RawSignals[2])
	}
	if report.Rows[0].RawSignals[2].DaysSinceFirstRelease != 300 || report.Rows[0].RawSignals[2].DaysSinceLastRelease != keywordMissingDateDays {
		t.Fatalf("partial metadata must use the valid release date while degrading the missing update date: %+v", report.Rows[0].RawSignals[2])
	}
}

func TestKeywordsScoreReturnsParentCancellationAfterPartialSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		writeScoreSearchResults(w)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	previousCollector := collectSearchPopularityForKeywords
	t.Cleanup(func() { collectSearchPopularityForKeywords = previousCollector })
	collectSearchPopularityForKeywords = func(context.Context, string, string, ads.SearchOptimizationRequest) ([]ads.SearchPopularity, error) {
		cancel()
		return nil, context.Canceled
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previousStdout := os.Stdout
	os.Stdout = writer
	runErr := KeywordsScoreCommand().ParseAndRun(ctx, []string{
		"--keywords", "focus timer",
		"--genre", "PRODUCTIVITY_UTILITIES",
		"--output", "json",
	})
	_ = writer.Close()
	os.Stdout = previousStdout
	stdout, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", runErr)
	}
	if len(stdout) != 0 {
		t.Fatalf("stdout = %q, want no partial report", stdout)
	}
}

func TestKeywordsScoreReportsEffectiveWorkerCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			writeScoreSearchResults(w, scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000})
		case "/lookup":
			writeScoreLookupResults(w, scoreLookupApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000, Release: daysAgo(900), Updated: daysAgo(10)})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
			"--keywords", "focus timer",
			"--workers", "10",
			"--output", "json",
		})
	})

	var report asc.KeywordScoreReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Workers != 1 {
		t.Fatalf("report.Workers = %d, want effective keyword count 1", report.Workers)
	}
}

func TestKeywordsScoreFlattensApplePopularityWhenAdsIsAvailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			writeScoreSearchResults(w,
				scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000},
				scoreSearchApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000},
				scoreSearchApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900},
				scoreSearchApp{ID: 444, Name: "Pomodoro", Seller: "Delta Labs", Rating: 4.1, Count: 250},
				scoreSearchApp{ID: 555, Name: "Fifth", Seller: "Zeta Labs", Rating: 4.0, Count: 100})
		case "/lookup":
			writeScoreLookupResults(w, scoreLookupApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000, Release: daysAgo(900), Updated: daysAgo(10)})
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	previous := collectSearchPopularityForKeywords
	t.Cleanup(func() { collectSearchPopularityForKeywords = previous })
	collectSearchPopularityForKeywords = func(_ context.Context, profile, account string, request ads.SearchOptimizationRequest) ([]ads.SearchPopularity, error) {
		if profile != "Ads" || account != "987654321" {
			t.Fatalf("ads credentials = (%q, %q)", profile, account)
		}
		if request.AppID != "" || request.Country != "US" || request.Genre != "PRODUCTIVITY_UTILITIES" {
			t.Fatalf("ads request = %+v", request)
		}
		if request.PopularityStart == "" || request.PopularityEnd == "" {
			t.Fatalf("ads request is missing a popularity window: %+v", request)
		}
		return []ads.SearchPopularity{
			{Term: "Focus Timer", Country: "US", Genre: "PRODUCTIVITY_UTILITIES", Week: "2026-08-01", Popularity5: intPtr(3), Popularity100: intPtr(52), RankInGenre: intPtr(40)},
			{Term: "focus timer", Country: "US", Genre: "PRODUCTIVITY_UTILITIES", Week: "2026-08-08", Popularity5: intPtr(4), Popularity100: intPtr(61), RankInGenre: intPtr(12)},
			{Term: "unrelated term", Country: "US", Popularity5: intPtr(5)},
		}, nil
	}

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
			"--keywords", "focus timer",
			"--app", "1234567890",
			"--genre", "productivity_utilities",
			"--ad-account", "987654321",
			"--ads-profile", "Ads",
			"--country", "US",
			"--output", "json",
		})
	})

	var report asc.KeywordScoreReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.Genre != "PRODUCTIVITY_UTILITIES" {
		t.Fatalf("genre = %q", report.Genre)
	}
	popularity := report.Rows[0].Popularity
	if popularity == nil {
		t.Fatal("popularity must be present when Apple Ads returns a row")
	}
	if popularity.Week != "2026-08-08" || popularity.Popularity5 == nil || *popularity.Popularity5 != 4 ||
		popularity.Popularity100 == nil || *popularity.Popularity100 != 61 {
		t.Fatalf("popularity must flatten the most recent week: %+v", popularity)
	}
	sources := map[string]asc.KeywordScoreSourceStatus{}
	for _, source := range report.Sources {
		sources[source.Name] = source
	}
	popularitySource, ok := sources[keywordSourcePopularity]
	if !ok {
		t.Fatalf("report is missing the %s source: %+v", keywordSourcePopularity, report.Sources)
	}
	if popularitySource.Status != keywordStatusAvailable {
		t.Fatalf("popularity source = %+v", popularitySource)
	}
}

func TestCollectKeywordPopularityDoesNotRequireApp(t *testing.T) {
	previous := collectSearchPopularityForKeywords
	t.Cleanup(func() { collectSearchPopularityForKeywords = previous })
	called := false
	collectSearchPopularityForKeywords = func(_ context.Context, profile, account string, request ads.SearchOptimizationRequest) ([]ads.SearchPopularity, error) {
		called = true
		if profile != "Ads" || account != "987654321" {
			t.Fatalf("ads credentials = (%q, %q)", profile, account)
		}
		if request.AppID != "" || request.Country != "US" || request.Genre != "PRODUCTIVITY_UTILITIES" {
			t.Fatalf("ads request = %+v", request)
		}
		return []ads.SearchPopularity{{
			Term: "focus timer", Country: "US", Genre: "PRODUCTIVITY_UTILITIES", Week: "2026-08-08", Popularity5: intPtr(4), Popularity100: intPtr(61),
		}}, nil
	}

	popularity, status := collectKeywordPopularity(context.Background(), keywordPopularityRequest{
		Keywords:   []string{"focus timer"},
		Country:    "US",
		Genre:      "PRODUCTIVITY_UTILITIES",
		AdsProfile: "Ads",
		AdAccount:  "987654321",
	})
	if !called {
		t.Fatal("popularity collector was not called without an app ID")
	}
	if status.Status != keywordStatusAvailable || len(popularity) != 1 {
		t.Fatalf("popularity = %+v, status = %+v", popularity, status)
	}
}

func TestCollectKeywordPopularityNormalizesEquivalentTerms(t *testing.T) {
	previous := collectSearchPopularityForKeywords
	t.Cleanup(func() { collectSearchPopularityForKeywords = previous })
	collectSearchPopularityForKeywords = func(context.Context, string, string, ads.SearchOptimizationRequest) ([]ads.SearchPopularity, error) {
		return []ads.SearchPopularity{
			{Term: "focus timer", Week: "2026-08-08", Popularity5: intPtr(4)},
			{Term: "ＨＡＢＩＴ　ＴＲＡＣＫＥＲ", Week: "2026-08-08", Popularity5: intPtr(3)},
		}, nil
	}

	popularity, status := collectKeywordPopularity(context.Background(), keywordPopularityRequest{
		Keywords: []string{"focus-timer", "habit tracker"},
		Country:  "US",
		Genre:    "PRODUCTIVITY_UTILITIES",
	})
	if status.Status != keywordStatusAvailable {
		t.Fatalf("status = %+v", status)
	}
	for _, keyword := range []string{"focus-timer", "habit tracker"} {
		if popularity[keyword].Popularity5 == nil {
			t.Fatalf("popularity[%q] = %+v, want a normalized match", keyword, popularity[keyword])
		}
	}
}

func TestKeywordsScoreDegradesWhenCompetitorMetadataIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeScoreSearchResults(w,
			scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000},
			scoreSearchApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000},
			scoreSearchApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900},
			scoreSearchApp{ID: 444, Name: "Pomodoro", Seller: "Delta Labs", Rating: 4.1, Count: 250},
			scoreSearchApp{ID: 555, Name: "Fifth", Seller: "Zeta Labs", Rating: 4.0, Count: 100})
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
			"--keywords", "focus timer", "--output", "json",
		})
	})

	var report asc.KeywordScoreReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	row := report.Rows[0]
	if row.Status != keywordStatusAvailable || row.DifficultyScore == nil {
		t.Fatalf("competition must still score without metadata: %+v", row)
	}
	if row.RawSignals[0].ReleaseDate != "" || row.RawSignals[0].DaysSinceFirstRelease != keywordMissingDateDays {
		t.Fatalf("missing metadata must degrade to the documented window: %+v", row.RawSignals[0])
	}
	sources := map[string]asc.KeywordScoreSourceStatus{}
	for _, source := range report.Sources {
		sources[source.Name] = source
	}
	metadataSource, ok := sources[keywordSourceMetadata]
	if !ok {
		t.Fatalf("report is missing the %s source: %+v", keywordSourceMetadata, report.Sources)
	}
	if metadataSource.Status != keywordStatusUnavailable || !strings.Contains(metadataSource.Error, "503") {
		t.Fatalf("metadata source = %+v", metadataSource)
	}
}

func TestKeywordsScoreFailsWhenEveryKeywordFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	err := KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
		"--keywords", "focus timer,habit tracker", "--output", "json",
	})
	if err == nil || !strings.Contains(err.Error(), "optimize keywords score") || !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v, want the representative failure", err)
	}
}

func TestKeywordsScoreRendersTableAndMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			writeScoreLookupResults(w, scoreLookupApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000, Release: daysAgo(900), Updated: daysAgo(10)})
			return
		}
		writeScoreSearchResults(w,
			scoreSearchApp{ID: 111, Name: "Focus Timer", Seller: "Alpha Labs", Rating: 4.6, Count: 8000},
			scoreSearchApp{ID: 222, Name: "Deep Work", Seller: "Beta Labs", Rating: 4.2, Count: 4000},
			scoreSearchApp{ID: 333, Name: "Timer Buddy", Seller: "Gamma Labs", Rating: 3.9, Count: 900},
			scoreSearchApp{ID: 444, Name: "Pomodoro", Seller: "Delta Labs", Rating: 4.1, Count: 250},
			scoreSearchApp{ID: 555, Name: "Fifth", Seller: "Zeta Labs", Rating: 4.0, Count: 100})
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)
	failKeywordsAdsCollector(t)

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout := captureSearchPlanStdout(t, func() error {
				return KeywordsScoreCommand().ParseAndRun(context.Background(), []string{
					"--keywords", "focus timer", "--output", format,
				})
			})
			normalized := strings.ToLower(stdout)
			for _, want := range []string{"keyword", "difficulty", "min difficulty", "popularity", "brand", "focus timer", "title exact phrase", "sources", "unavailable", "search term popularity"} {
				if !strings.Contains(normalized, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

type scoreSearchApp struct {
	ID     int64
	Name   string
	Seller string
	Rating float64
	Count  int64
}

type scoreLookupApp struct {
	ID      int64
	Name    string
	Seller  string
	Rating  float64
	Count   int64
	Release string
	Updated string
}

func writeScoreSearchResults(w http.ResponseWriter, apps ...scoreSearchApp) {
	results := make([]string, 0, len(apps))
	for _, app := range apps {
		results = append(results, fmt.Sprintf(
			`{"trackId":%d,"trackName":%q,"sellerName":%q,"averageUserRating":%v,"userRatingCount":%d}`,
			app.ID, app.Name, app.Seller, app.Rating, app.Count,
		))
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"resultCount":%d,"results":[%s]}`, len(results), strings.Join(results, ","))
}

func writeScoreLookupResults(w http.ResponseWriter, apps ...scoreLookupApp) {
	results := make([]string, 0, len(apps))
	for _, app := range apps {
		results = append(results, fmt.Sprintf(
			`{"trackId":%d,"trackName":%q,"sellerName":%q,"averageUserRating":%v,"userRatingCount":%d,"releaseDate":%q,"currentVersionReleaseDate":%q}`,
			app.ID, app.Name, app.Seller, app.Rating, app.Count, app.Release, app.Updated,
		))
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"resultCount":%d,"results":[%s]}`, len(results), strings.Join(results, ","))
}

func failKeywordsAdsCollector(t *testing.T) {
	t.Helper()
	previous := collectSearchPopularityForKeywords
	t.Cleanup(func() { collectSearchPopularityForKeywords = previous })
	collectSearchPopularityForKeywords = func(context.Context, string, string, ads.SearchOptimizationRequest) ([]ads.SearchPopularity, error) {
		t.Fatal("Apple Ads collector ran without complete popularity inputs")
		return nil, nil
	}
}
