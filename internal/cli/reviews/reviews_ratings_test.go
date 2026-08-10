package reviews

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/itunes"
)

func TestRatingsAllRenewsCountryOperationDeadlines(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", time.Hour.String())

	type observedRequest struct {
		kind     string
		country  string
		deadline time.Time
	}

	var (
		mu                 sync.Mutex
		observed           []observedRequest
		countryLookupCount int
	)
	record := func(req *http.Request, kind, country string) int {
		t.Helper()
		deadline, ok := req.Context().Deadline()
		if !ok {
			t.Errorf("%s request for %q has no deadline", kind, country)
		}

		mu.Lock()
		defer mu.Unlock()
		observed = append(observed, observedRequest{kind: kind, country: country, deadline: deadline})
		if kind == "country lookup" {
			countryLookupCount++
			return countryLookupCount
		}
		return 0
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/lookup" && req.URL.Query().Get("bundleId") != "":
			time.Sleep(5 * time.Millisecond)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Deadline App","bundleId":"com.example.deadlines"}]}`)
		case req.URL.Path == "/lookup":
			mu.Lock()
			lookupNumber := countryLookupCount
			mu.Unlock()
			if lookupNumber == 1 {
				time.Sleep(5 * time.Millisecond)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Deadline App","averageUserRating":4.5,"userRatingCount":10}]}`)
		case strings.Contains(req.URL.Path, "/customer-reviews/id123"):
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, `<span class="total">10</span>`)
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()

	serverClient := server.Client()
	baseTransport := serverClient.Transport
	serverClient.Transport = reviewRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case req.URL.Path == "/lookup" && req.URL.Query().Get("bundleId") != "":
			record(req, "app resolution", req.URL.Query().Get("country"))
		case req.URL.Path == "/lookup":
			record(req, "country lookup", req.URL.Query().Get("country"))
		case strings.Contains(req.URL.Path, "/customer-reviews/id123"):
			parts := strings.Split(strings.Trim(req.URL.Path, "/"), "/")
			country := ""
			if len(parts) > 0 {
				country = parts[0]
			}
			record(req, "histogram", country)
		}
		return baseTransport.RoundTrip(req)
	})
	client := &itunes.Client{BaseURL: server.URL, HTTPClient: serverClient}
	stdout, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open stdout sink: %v", err)
	}
	defer stdout.Close()
	originalStdout := os.Stdout
	os.Stdout = stdout
	defer func() { os.Stdout = originalStdout }()

	if err := executeRatingsWithClient(context.Background(), client, "com.example.deadlines", "us", true, 1, "json", false); err != nil {
		t.Fatalf("executeRatingsWithClient() error: %v", err)
	}

	mu.Lock()
	requests := append([]observedRequest(nil), observed...)
	mu.Unlock()

	var resolution observedRequest
	var countryLookups []observedRequest
	var firstCountryHistogram observedRequest
	for _, request := range requests {
		switch request.kind {
		case "app resolution":
			resolution = request
		case "country lookup":
			countryLookups = append(countryLookups, request)
		case "histogram":
			if len(countryLookups) > 0 && request.country == countryLookups[0].country && firstCountryHistogram.deadline.IsZero() {
				firstCountryHistogram = request
			}
		}
	}
	if resolution.deadline.IsZero() || len(countryLookups) < 2 || firstCountryHistogram.deadline.IsZero() {
		t.Fatalf("missing deadline observations: resolution=%v lookups=%d histogram=%v", resolution, len(countryLookups), firstCountryHistogram)
	}
	if !countryLookups[0].deadline.After(resolution.deadline) {
		t.Fatalf("first country deadline = %s, want later than app resolution deadline %s", countryLookups[0].deadline, resolution.deadline)
	}
	if !countryLookups[1].deadline.After(countryLookups[0].deadline) {
		t.Fatalf("second country deadline = %s, want later than first country deadline %s", countryLookups[1].deadline, countryLookups[0].deadline)
	}
	if !firstCountryHistogram.deadline.Equal(countryLookups[0].deadline) {
		t.Fatalf("first country histogram deadline = %s, want country operation deadline %s", firstCountryHistogram.deadline, countryLookups[0].deadline)
	}
}

func TestRatingsAllDeadlineDoesNotPrintSuccessOutput(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "1ns")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Deadline App","averageUserRating":4.5,"userRatingCount":10}]}`)
	}))
	defer server.Close()

	client := &itunes.Client{BaseURL: server.URL, HTTPClient: server.Client()}
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = stdoutWriter
	commandErr := executeRatingsWithClient(context.Background(), client, "123", "us", true, 1, "json", false)
	os.Stdout = originalStdout
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if err := stdoutReader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	if !errors.Is(commandErr, context.DeadlineExceeded) {
		t.Fatalf("executeRatingsWithClient() error = %v, want context.DeadlineExceeded", commandErr)
	}
	if len(output) != 0 {
		t.Fatalf("stdout = %q, want no success output", output)
	}
}

func TestNormalizeRatingsOutput(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		pretty     bool
		wantFormat string
		wantErr    string
	}{
		{name: "default json", input: "", pretty: false, wantFormat: "json"},
		{name: "markdown alias md", input: "md", pretty: false, wantFormat: "markdown"},
		{name: "trim and lowercase", input: "  TABLE  ", pretty: false, wantFormat: "table"},
		{name: "pretty table rejected", input: "table", pretty: true, wantErr: "--pretty is only valid with JSON output"},
		{name: "unsupported format rejected", input: "yaml", pretty: false, wantErr: "unsupported format: yaml"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeRatingsOutput(tc.input, tc.pretty)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantFormat {
				t.Fatalf("expected format %q, got %q", tc.wantFormat, got)
			}
		})
	}
}

func TestUniqueExactRatingsAppMatch(t *testing.T) {
	results := []itunes.SearchResult{
		{AppID: 123, Name: "Alpha", BundleID: "com.example.alpha"},
		{AppID: 456, Name: "Alpha", BundleID: "com.example.alpha.pro"},
	}

	if got, ok := uniqueExactRatingsAppMatch(results, "com.example.alpha", func(result itunes.SearchResult) string {
		return result.BundleID
	}); !ok || got != "123" {
		t.Fatalf("bundle match = %q, %v; want 123, true", got, ok)
	}

	if got, ok := uniqueExactRatingsAppMatch(results, "Alpha", func(result itunes.SearchResult) string {
		return result.Name
	}); ok || got != "" {
		t.Fatalf("ambiguous name match = %q, %v; want empty, false", got, ok)
	}
}

func TestRatingsAppLookupCountryUsesDefaultForAllStorefronts(t *testing.T) {
	if got := ratingsAppLookupCountry("kz", true); got != "us" {
		t.Fatalf("ratingsAppLookupCountry(all) = %q, want us", got)
	}
	if got := ratingsAppLookupCountry("kz", false); got != "kz" {
		t.Fatalf("ratingsAppLookupCountry(single) = %q, want kz", got)
	}
}

func TestResolveRatingsAppIDLooksUpTrimmedBundleWithHttptest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("bundleId"); got != "com.example.alpha" {
			t.Fatalf("expected trimmed lookup bundleId com.example.alpha, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "us" {
			t.Fatalf("expected lookup country=us, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Alpha","bundleId":"com.example.alpha"}]}`)
	}))
	defer server.Close()

	client := &itunes.Client{BaseURL: server.URL, HTTPClient: server.Client()}
	got, err := resolveRatingsAppID(context.Background(), client, "  com.example.alpha  ", "us")
	if err != nil {
		t.Fatalf("resolveRatingsAppID() error: %v", err)
	}
	if got != "123" {
		t.Fatalf("resolveRatingsAppID() = %q, want 123", got)
	}
}

func TestResolveRatingsAppIDHidesBundleLookupFailureDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		http.Error(w, "backend leaked com.example.secret", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &itunes.Client{BaseURL: server.URL, HTTPClient: server.Client()}
	_, err := resolveRatingsAppID(context.Background(), client, "com.example.secret", "us")
	if err == nil {
		t.Fatal("expected lookup failure")
	}
	if strings.Contains(err.Error(), "com.example.secret") || strings.Contains(err.Error(), "backend leaked") {
		t.Fatalf("expected generic lookup error, got %q", err.Error())
	}
}

func TestResolveRatingsAppIDSearchesBeyondFirstTenForExactName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("term"); got != "Alpha" {
			t.Fatalf("expected search term Alpha, got %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != strconv.Itoa(ratingsAppSearchLimit) {
			t.Fatalf("expected search limit=%d, got %q", ratingsAppSearchLimit, got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"resultCount":11,"results":[
			{"trackId":1,"trackName":"Alpha Notes 1","bundleId":"com.example.alpha1"},
			{"trackId":2,"trackName":"Alpha Notes 2","bundleId":"com.example.alpha2"},
			{"trackId":3,"trackName":"Alpha Notes 3","bundleId":"com.example.alpha3"},
			{"trackId":4,"trackName":"Alpha Notes 4","bundleId":"com.example.alpha4"},
			{"trackId":5,"trackName":"Alpha Notes 5","bundleId":"com.example.alpha5"},
			{"trackId":6,"trackName":"Alpha Notes 6","bundleId":"com.example.alpha6"},
			{"trackId":7,"trackName":"Alpha Notes 7","bundleId":"com.example.alpha7"},
			{"trackId":8,"trackName":"Alpha Notes 8","bundleId":"com.example.alpha8"},
			{"trackId":9,"trackName":"Alpha Notes 9","bundleId":"com.example.alpha9"},
			{"trackId":10,"trackName":"Alpha Notes 10","bundleId":"com.example.alpha10"},
			{"trackId":123,"trackName":"Alpha","bundleId":"com.example.alpha"}
		]}`)
	}))
	defer server.Close()

	client := &itunes.Client{BaseURL: server.URL, HTTPClient: server.Client()}
	got, err := resolveRatingsAppID(context.Background(), client, "Alpha", "us")
	if err != nil {
		t.Fatalf("resolveRatingsAppID() error: %v", err)
	}
	if got != "123" {
		t.Fatalf("resolveRatingsAppID() = %q, want 123", got)
	}
}
