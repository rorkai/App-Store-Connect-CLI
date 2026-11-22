package itunes

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRankAppIOSUsesOrderedSearchResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/search" {
			t.Fatalf("request = %s %s, want GET /search", r.Method, r.URL.Path)
		}

		query := r.URL.Query()
		if got := query.Get("term"); got != "focus timer" {
			t.Fatalf("term = %q, want focus timer", got)
		}
		if got := query.Get("country"); got != "us" {
			t.Fatalf("country = %q, want us", got)
		}
		if got := query.Get("entity"); got != "software" {
			t.Fatalf("entity = %q, want software", got)
		}
		if got := query.Get("limit"); got != "200" {
			t.Fatalf("limit = %q, want 200", got)
		}

		writeBody(t, w, `{
			"resultCount": 3,
			"results": [
				{"trackId": 111, "trackName": "One"},
				{"trackId": 1234567890, "trackName": "Focus Timer"},
				{"trackId": 333, "trackName": "Three"}
			]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.RankApp(
		context.Background(),
		"1234567890",
		"focus timer",
		"us",
		PublicSearchPlatformIOS,
	)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if !result.Found {
		t.Fatal("RankApp() Found = false, want true")
	}
	if result.Rank == nil || *result.Rank != 2 {
		t.Fatalf("RankApp() Rank = %v, want 2", result.Rank)
	}
	if result.ResultCount != 3 {
		t.Fatalf("RankApp() ResultCount = %d, want 3", result.ResultCount)
	}
}

func TestRankAppIOSReportsAbsentApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"resultCount": 2,
			"results": [
				{"trackId": 111, "trackName": "One"},
				{"trackId": 222, "trackName": "Two"}
			]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformIOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if result.Found {
		t.Fatal("RankApp() Found = true, want false")
	}
	if result.Rank != nil {
		t.Fatalf("RankApp() Rank = %v, want nil", result.Rank)
	}
	if result.ResultCount != 2 {
		t.Fatalf("RankApp() ResultCount = %d, want 2", result.ResultCount)
	}
}

func TestRankAppIOSMatchesZeroPaddedAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{"trackId": 123, "trackName": "Alpha"}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.RankApp(context.Background(), "00123", "alpha", "us", PublicSearchPlatformIOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if !result.Found || result.Rank == nil || *result.Rank != 1 {
		t.Fatalf("RankApp() = %+v, want found rank 1", result)
	}
}

func TestRankAppIOSHandlesEmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{"resultCount":0,"results":[]}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	result, err := client.RankApp(context.Background(), "1234567890", "missing", "us", PublicSearchPlatformIOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if result.Found || result.Rank != nil || result.ResultCount != 0 {
		t.Fatalf("RankApp() = %+v, want empty not-found result", result)
	}
}

func TestRankAppRejectsUnsupportedPlatform(t *testing.T) {
	client := &Client{}
	_, err := client.RankApp(context.Background(), "123", "alpha", "us", PublicSearchPlatform("MAC_OS"))
	if err == nil {
		t.Fatal("RankApp() error = nil, want unsupported platform error")
	}
}

func TestRankAppTVOSUsesStorefrontSearchOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/WebObjects/MZStore.woa/wa/search" {
			t.Fatalf("request = %s %s, want GET /WebObjects/MZStore.woa/wa/search", r.Method, r.URL.Path)
		}

		query := r.URL.Query()
		if got := query.Get("clientApplication"); got != "Software" {
			t.Fatalf("clientApplication = %q, want Software", got)
		}
		if got := query.Get("src"); got != "hint" {
			t.Fatalf("src = %q, want hint", got)
		}
		if got := query.Get("submit"); got != "edit" {
			t.Fatalf("submit = %q, want edit", got)
		}
		if got := query.Get("term"); got != "focus timer" {
			t.Fatalf("term = %q, want focus timer", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("X-Apple-Store-Front"); got != Storefronts["us"]+",33" {
			t.Fatalf("X-Apple-Store-Front = %q, want %q", got, Storefronts["us"]+",33")
		}

		// The target app carries the middle score so neither ascending nor
		// descending score order reproduces the response order. Ranking by
		// score instead of response position lands it at rank 2, not 3.
		writeBody(t, w, `{
			"pageData": {
				"bubbles": [{
					"results": [
						{"id":"111","entity":"tvSoftware","score":9.9},
						{"id":"222","entity":"tvSoftware","score":1.0},
						{"id":"1234567890","entity":"tvSoftware","score":5.0}
					]
				}]
			},
			"storePlatformData": {
				"lockup": {"results": {"111":{},"222":{}}}
			}
		}`)
	}))
	defer server.Close()

	client := &Client{
		HTTPClient:              server.Client(),
		StorefrontSearchBaseURL: server.URL,
	}
	result, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if !result.Found || result.Rank == nil || *result.Rank != 3 || result.ResultCount != 3 {
		t.Fatalf("RankApp() = %+v, want found rank 3 of 3", result)
	}
}

func TestRankAppTVOSDeduplicatesFirstOccurrence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"pageData": {"bubbles": [{"results": [
				{"id":"111","entity":"tvSoftware"},
				{"id":"1234567890","entity":"tvSoftware"},
				{"id":"1234567890","entity":"tvSoftware"},
				{"id":"333","entity":"tvSoftware"}
			]}]}
		}`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	result, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if !result.Found || result.Rank == nil || *result.Rank != 2 || result.ResultCount != 3 {
		t.Fatalf("RankApp() = %+v, want first occurrence at rank 2 of 3 unique results", result)
	}
}

func TestRankAppTVOSHandlesEmptyResultWindows(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty bubbles", body: `{"pageData":{"bubbles":[]}}`},
		{name: "empty results", body: `{"pageData":{"bubbles":[{"results":[]}]}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeBody(t, w, test.body)
			}))
			defer server.Close()

			client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
			result, err := client.RankApp(context.Background(), "1234567890", "missing", "us", PublicSearchPlatformTVOS)
			if err != nil {
				t.Fatalf("RankApp() error: %v", err)
			}
			if result.Found || result.Rank != nil || result.ResultCount != 0 {
				t.Fatalf("RankApp() = %+v, want empty not-found result", result)
			}
		})
	}
}

func TestRankAppTVOSErrorsOnMissingResultWindow(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing page data", body: `{}`},
		{name: "missing bubbles", body: `{"pageData":{}}`},
		{name: "null bubbles", body: `{"pageData":{"bubbles":null}}`},
		{name: "missing bubble results", body: `{"pageData":{"bubbles":[{}]}}`},
		{name: "null bubble results", body: `{"pageData":{"bubbles":[{"results":null}]}}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeBody(t, w, test.body)
			}))
			defer server.Close()

			client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
			_, err := client.RankApp(context.Background(), "1234567890", "missing", "us", PublicSearchPlatformTVOS)
			if err == nil {
				t.Fatal("RankApp() error = nil, want missing result window error")
			}
		})
	}
}

func TestRankAppTVOSReportsAbsentApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"pageData": {"bubbles": [{"results": [
				{"id":"111","entity":"tvSoftware"},
				{"id":"222","entity":"tvSoftware"}
			]}]}
		}`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	result, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err != nil {
		t.Fatalf("RankApp() error: %v", err)
	}
	if result.Found || result.Rank != nil || result.ResultCount != 2 {
		t.Fatalf("RankApp() = %+v, want not found in 2 results", result)
	}
}

func TestRankAppTVOSErrorsOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{"pageData":`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	_, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err == nil {
		t.Fatal("RankApp() error = nil, want malformed JSON error")
	}
}

func TestRankAppTVOSErrorsOnMissingTVAppID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"pageData": {"bubbles": [{"results": [
				{"entity":"tvSoftware","score":2.0}
			]}]}
		}`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	_, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err == nil {
		t.Fatal("RankApp() error = nil, want missing TV app ID error")
	}
}

func TestRankAppTVOSErrorsOnNonTVResultSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeBody(t, w, `{
			"pageData": {"bubbles": [{"results": [
				{"id":"111","entity":"software","score":2.0}
			]}]}
		}`)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	_, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err == nil {
		t.Fatal("RankApp() error = nil, want unexpected entity error")
	}
}

func TestRankAppTVOSRetainsHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	client := &Client{HTTPClient: server.Client(), StorefrontSearchBaseURL: server.URL}
	_, err := client.RankApp(context.Background(), "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if err == nil {
		t.Fatal("RankApp() error = nil, want HTTP status error")
	}

	var statusError interface{ HTTPStatusCode() int }
	if !errors.As(err, &statusError) {
		t.Fatalf("error %T does not expose HTTP status", err)
	}
	if got := statusError.HTTPStatusCode(); got != http.StatusTooManyRequests {
		t.Fatalf("HTTPStatusCode() = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestRankAppTVOSRespectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &Client{HTTPClient: http.DefaultClient}
	_, err := client.RankApp(ctx, "1234567890", "focus timer", "us", PublicSearchPlatformTVOS)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RankApp() error = %v, want context.Canceled", err)
	}
}
