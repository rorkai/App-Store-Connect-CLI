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
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

func TestKeywordsRankCommandValidatesInputBeforeRequests(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing app",
			args: []string{"--keywords", "focus timer"},
			want: "--app is required (or set ASC_APP_ID)",
		},
		{
			name: "non numeric app",
			args: []string{"--app", "com.example.app", "--keywords", "focus timer"},
			want: "--app must be a numeric App Store app ID",
		},
		{
			name: "zero app",
			args: []string{"--app", "0", "--keywords", "focus timer"},
			want: "--app must be a positive App Store app ID representable as a 64-bit integer",
		},
		{
			name: "overflowing app",
			args: []string{"--app", "9223372036854775808", "--keywords", "focus timer"},
			want: "--app must be a positive App Store app ID representable as a 64-bit integer",
		},
		{
			name: "missing keywords",
			args: []string{"--app", "1234567890"},
			want: "--keywords is required",
		},
		{
			name: "keyword too long",
			args: []string{"--app", "1234567890", "--keywords", strings.Repeat("a", 61)},
			want: fmt.Sprintf("--keywords entry %q must be between 2 and 60 characters", strings.Repeat("a", 61)),
		},
		{
			name: "keyword word count",
			args: []string{"--app", "1234567890", "--keywords", "one two three four five"},
			want: `--keywords entry "one two three four five" must contain at most 4 space-separated words`,
		},
		{
			name: "keyword budget",
			args: []string{"--app", "1234567890", "--keywords", strings.Join(manyKeywords(101), ",")},
			want: "--keywords accepts at most 100 keywords per invocation",
		},
		{
			name: "workers",
			args: []string{"--app", "1234567890", "--keywords", "focus timer", "--workers", "0"},
			want: "--workers must be at least 1",
		},
		{
			name: "country",
			args: []string{"--app", "1234567890", "--keywords", "focus timer", "--country", "usa"},
			want: `--country "usa" is not a supported App Store storefront`,
		},
		{
			name: "platform",
			args: []string{"--app", "1234567890", "--keywords", "focus timer", "--platform", "MAC_OS"},
			want: "--platform must be one of: IOS, TV_OS",
		},
		{
			name: "tv storefront",
			args: []string{"--app", "1234567890", "--keywords", "focus timer", "--platform", "TV_OS", "--country", "kz"},
			want: "TV_OS ranking is unavailable for storefront KZ",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("ASC_APP_ID", "")
			failKeywordsClient(t)
			err := KeywordsRankCommand().ParseAndRun(context.Background(), test.args)
			if err == nil || err.Error() != test.want {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("error = %v, want usage error", err)
			}
		})
	}
}

func TestKeywordsRankReportsRanksAbsencesAndPerKeywordFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Errorf("path = %q, want /search", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "200" {
			t.Errorf("limit = %q, want 200", got)
		}
		if got := r.URL.Query().Get("country"); got != "us" {
			t.Errorf("country = %q, want us", got)
		}
		switch r.URL.Query().Get("term") {
		case "focus timer":
			writeSearchResults(w, 111, 1234567890, 222)
		case "habit tracker":
			writeSearchResults(w, 111, 222)
		case "broken keyword":
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			t.Errorf("unexpected term %q", r.URL.Query().Get("term"))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsRankCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890",
			"--keywords", "Focus Timer,habit tracker,broken keyword",
			"--country", "US",
			"--output", "json",
		})
	})

	var report asc.KeywordRankReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v\n%s", err, stdout)
	}
	if report.AppID != "1234567890" || report.Country != "US" || report.Platform != "IOS" {
		t.Fatalf("unexpected report identity: %+v", report)
	}
	if len(report.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(report.Rows))
	}

	ranked := report.Rows[0]
	if ranked.Keyword != "focus timer" || ranked.Status != "available" || ranked.Rank == nil || *ranked.Rank != 2 {
		t.Fatalf("ranked row = %+v", ranked)
	}
	if ranked.TotalResults == nil || *ranked.TotalResults != 3 {
		t.Fatalf("ranked row total results = %+v", ranked)
	}

	absent := report.Rows[1]
	if absent.Keyword != "habit tracker" || absent.Status != "empty" || absent.Rank != nil {
		t.Fatalf("absent row = %+v", absent)
	}
	if absent.TotalResults == nil || *absent.TotalResults != 2 {
		t.Fatalf("absent row total results = %+v", absent)
	}

	failed := report.Rows[2]
	if failed.Keyword != "broken keyword" || failed.Status != "unavailable" || failed.Rank != nil {
		t.Fatalf("failed row = %+v", failed)
	}
	if failed.TotalResults != nil {
		t.Fatalf("failed row must not invent a result count: %+v", failed)
	}
	if !strings.Contains(failed.Error, "429") {
		t.Fatalf("failed row error = %q, want the HTTP status", failed.Error)
	}

	if report.Summary.Keywords != 3 || report.Summary.Ranked != 1 || report.Summary.Absent != 1 || report.Summary.Unavailable != 1 {
		t.Fatalf("summary = %+v", report.Summary)
	}

	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatalf("unmarshal raw rows: %v", err)
	}
	if string(raw.Rows[1]["rank"]) != "null" {
		t.Fatalf("absent row must serialize an explicit null rank: %s", stdout)
	}
}

func TestKeywordsRankFailsWhenEveryKeywordFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") == "alpha" {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	err := KeywordsRankCommand().ParseAndRun(context.Background(), []string{
		"--app", "1234567890",
		"--keywords", "alpha,bravo",
		"--output", "json",
	})
	if err == nil {
		t.Fatal("expected an error when every keyword lookup fails")
	}
	if !strings.Contains(err.Error(), "optimize keywords rank") || !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %v, want the representative server failure", err)
	}
}

func TestKeywordsRankReturnsParentCancellationAfterPartialSuccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		mu       sync.Mutex
		requests int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		requestNumber := requests
		mu.Unlock()
		if requestNumber == 1 {
			writeSearchResults(w, 1234567890)
			return
		}
		cancel()
		<-r.Context().Done()
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	runErr := KeywordsRankCommand().ParseAndRun(ctx, []string{
		"--app", "1234567890",
		"--keywords", "focus timer,habit tracker",
		"--workers", "1",
		"--output", "json",
	})
	_ = writer.Close()
	os.Stdout = previous
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

func TestKeywordsRankBoundsConcurrentRequestsWithWorkers(t *testing.T) {
	var (
		mu        sync.Mutex
		active    int
		maxActive int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		time.Sleep(20 * time.Millisecond)
		mu.Lock()
		active--
		mu.Unlock()
		writeSearchResults(w, 1234567890)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	keywords := manyKeywords(8)
	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsRankCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890",
			"--keywords", strings.Join(keywords, ","),
			"--workers", "2",
			"--output", "json",
		})
	})

	var report asc.KeywordRankReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if len(report.Rows) != len(keywords) || report.Workers != 2 {
		t.Fatalf("report = %+v", report.Summary)
	}
	mu.Lock()
	observed := maxActive
	mu.Unlock()
	if observed > 2 {
		t.Fatalf("max concurrent requests = %d, want at most 2", observed)
	}
	if observed < 2 {
		t.Fatalf("max concurrent requests = %d, want parallel execution", observed)
	}
}

func TestKeywordsRankReportsEffectiveWorkerCount(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeSearchResults(w, 1234567890)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	stdout := captureSearchPlanStdout(t, func() error {
		return KeywordsRankCommand().ParseAndRun(context.Background(), []string{
			"--app", "1234567890",
			"--keywords", "focus timer,habit tracker",
			"--workers", "10",
			"--output", "json",
		})
	})

	var report asc.KeywordRankReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("unmarshal report: %v", err)
	}
	if report.Workers != 2 {
		t.Fatalf("report.Workers = %d, want effective keyword count 2", report.Workers)
	}
}

func TestKeywordsRankRendersTableAndMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("term") == "focus timer" {
			writeSearchResults(w, 1234567890)
			return
		}
		writeSearchResults(w, 111)
	}))
	defer server.Close()
	stubKeywordsClient(t, server.URL)

	for _, format := range []string{"table", "markdown"} {
		t.Run(format, func(t *testing.T) {
			stdout := captureSearchPlanStdout(t, func() error {
				return KeywordsRankCommand().ParseAndRun(context.Background(), []string{
					"--app", "1234567890",
					"--keywords", "focus timer,habit tracker",
					"--output", format,
				})
			})
			for _, want := range []string{"Keyword", "Rank", "Status", "focus timer", "habit tracker", "available", "empty"} {
				if !strings.Contains(stdout, want) {
					t.Fatalf("%s output missing %q:\n%s", format, want, stdout)
				}
			}
		})
	}
}

func writeSearchResults(w http.ResponseWriter, appIDs ...int64) {
	results := make([]string, 0, len(appIDs))
	for index, appID := range appIDs {
		results = append(results, fmt.Sprintf(`{"trackId":%d,"trackName":"App %d"}`, appID, index))
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"resultCount":%d,"results":[%s]}`, len(results), strings.Join(results, ","))
}

func stubKeywordsClient(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("ASC_BASE_DELAY", "1ms")
	previous := newKeywordsItunesClient
	t.Cleanup(func() { newKeywordsItunesClient = previous })
	newKeywordsItunesClient = func() *itunes.Client {
		return &itunes.Client{
			HTTPClient:              http.DefaultClient,
			BaseURL:                 baseURL,
			StorefrontSearchBaseURL: baseURL,
		}
	}
}

func failKeywordsClient(t *testing.T) {
	t.Helper()
	previous := newKeywordsItunesClient
	t.Cleanup(func() { newKeywordsItunesClient = previous })
	newKeywordsItunesClient = func() *itunes.Client {
		t.Fatal("iTunes client built before input validation")
		return nil
	}
}
