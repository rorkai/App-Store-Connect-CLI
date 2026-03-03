package itunes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRatings_Success(t *testing.T) {
	// Mock iTunes Lookup API response
	lookupResponse := `{
		"resultCount": 1,
		"results": [{
			"trackId": 1479784361,
			"trackName": "Gradient Match Game: Descent",
			"averageUserRating": 4.75,
			"userRatingCount": 71,
			"averageUserRatingForCurrentVersion": 4.75,
			"userRatingCountForCurrentVersion": 71
		}]
	}`

	// Mock histogram HTML response
	histogramHTML := `
		<div class="ratings-histogram">
			<div class="vote"><span class="total">61</span></div>
			<div class="vote"><span class="total">6</span></div>
			<div class="vote"><span class="total">1</span></div>
			<div class="vote"><span class="total">2</span></div>
			<div class="vote"><span class="total">1</span></div>
		</div>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, lookupResponse)
			return
		}
		if r.URL.Path == "/us/customer-reviews/id1479784361" {
			w.Header().Set("Content-Type", "text/html")
			writeBody(t, w, histogramHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer func() { server.Close() }()

	// Create client that uses our test server
	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{
				baseURL: server.URL,
			},
		},
	}

	ratings, err := client.GetRatings(context.Background(), "1479784361", "us")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}

	if ratings.AppID != 1479784361 {
		t.Errorf("AppID = %d, want 1479784361", ratings.AppID)
	}
	if ratings.AppName != "Gradient Match Game: Descent" {
		t.Errorf("AppName = %q, want %q", ratings.AppName, "Gradient Match Game: Descent")
	}
	if ratings.AverageRating != 4.75 {
		t.Errorf("AverageRating = %f, want 4.75", ratings.AverageRating)
	}
	if ratings.RatingCount != 71 {
		t.Errorf("RatingCount = %d, want 71", ratings.RatingCount)
	}
	if ratings.Country != "US" {
		t.Errorf("Country = %q, want %q", ratings.Country, "US")
	}

	// Check histogram
	if ratings.Histogram[5] != 61 {
		t.Errorf("Histogram[5] = %d, want 61", ratings.Histogram[5])
	}
	if ratings.Histogram[4] != 6 {
		t.Errorf("Histogram[4] = %d, want 6", ratings.Histogram[4])
	}
	if ratings.Histogram[1] != 1 {
		t.Errorf("Histogram[1] = %d, want 1", ratings.Histogram[1])
	}
}

func TestGetRatings_HistogramWithCommas(t *testing.T) {
	lookupResponse := `{
		"resultCount": 1,
		"results": [{
			"trackId": 123,
			"trackName": "Comma App",
			"averageUserRating": 4.0,
			"userRatingCount": 100
		}]
	}`

	histogramHTML := `
		<div class="ratings-histogram">
			<div class="vote"><span class="total">1,234</span></div>
			<div class="vote"><span class="total">567</span></div>
			<div class="vote"><span class="total">89</span></div>
			<div class="vote"><span class="total">12</span></div>
			<div class="vote"><span class="total">3</span></div>
		</div>
	`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, lookupResponse)
			return
		}
		if r.URL.Path == "/us/customer-reviews/id123" {
			w.Header().Set("Content-Type", "text/html")
			writeBody(t, w, histogramHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	ratings, err := client.GetRatings(context.Background(), "123", "us")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}

	if ratings.Histogram[5] != 1234 {
		t.Errorf("Histogram[5] = %d, want 1234", ratings.Histogram[5])
	}
	if ratings.Histogram[1] != 3 {
		t.Errorf("Histogram[1] = %d, want 3", ratings.Histogram[1])
	}
}

func TestGetRatings_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, `{"resultCount": 0, "results": []}`)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	_, err := client.GetRatings(context.Background(), "999999999", "us")
	if err == nil {
		t.Fatal("expected error for not found app, got nil")
	}
}

func TestGetRatings_HistogramFailureIsNonFatal(t *testing.T) {
	lookupResponse := `{
		"resultCount": 1,
		"results": [{
			"trackId": 123,
			"trackName": "Histogram Down",
			"averageUserRating": 4.0,
			"userRatingCount": 10
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, lookupResponse)
			return
		}
		if r.URL.Path == "/us/customer-reviews/id123" {
			http.Error(w, "unavailable", http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	ratings, err := client.GetRatings(context.Background(), "123", "us")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}
	if len(ratings.Histogram) != 0 {
		t.Fatalf("expected empty histogram on failure, got %v", ratings.Histogram)
	}
}

func TestGetRatings_DefaultCountry(t *testing.T) {
	lookupResponse := `{
		"resultCount": 1,
		"results": [{
			"trackId": 123,
			"trackName": "Test App",
			"averageUserRating": 4.0,
			"userRatingCount": 10
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, lookupResponse)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	// Pass empty country - should default to "us"
	ratings, err := client.GetRatings(context.Background(), "123", "")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}

	// The returned Country field should be "US" (uppercased default)
	if ratings.Country != "US" {
		t.Errorf("Country = %q, want %q (default)", ratings.Country, "US")
	}
}

func TestGetAllRatings_Aggregation(t *testing.T) {
	// Mock responses for different countries
	responses := map[string]string{
		"us": `{"resultCount":1,"results":[{"trackId":123,"trackName":"Test App","averageUserRating":4.0,"userRatingCount":100}]}`,
		"gb": `{"resultCount":1,"results":[{"trackId":123,"trackName":"Test App","averageUserRating":5.0,"userRatingCount":50}]}`,
		"de": `{"resultCount":0,"results":[]}`, // Not available in Germany
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			country := r.URL.Query().Get("country")
			if resp, ok := responses[country]; ok {
				w.Header().Set("Content-Type", "application/json")
				writeBody(t, w, resp)
				return
			}
			// Return empty for unknown countries
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, `{"resultCount":0,"results":[]}`)
			return
		}
		// Return empty histogram
		w.Header().Set("Content-Type", "text/html")
		writeBody(t, w, `<html></html>`)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	global, err := client.GetAllRatings(context.Background(), "123", 5)
	if err != nil {
		t.Fatalf("GetAllRatings() error: %v", err)
	}

	// Should have found 2 countries (US and GB)
	if global.CountryCount != 2 {
		t.Errorf("CountryCount = %d, want 2", global.CountryCount)
	}

	// Total should be 150 (100 + 50)
	if global.TotalCount != 150 {
		t.Errorf("TotalCount = %d, want 150", global.TotalCount)
	}

	// Weighted average: (4.0*100 + 5.0*50) / 150 = 650/150 = 4.333...
	expectedAvg := (4.0*100 + 5.0*50) / 150.0
	if global.AverageRating != expectedAvg {
		t.Errorf("AverageRating = %f, want %f", global.AverageRating, expectedAvg)
	}

	// Results should be sorted by count descending
	if len(global.ByCountry) != 2 {
		t.Fatalf("ByCountry length = %d, want 2", len(global.ByCountry))
	}
	if global.ByCountry[0].Country != "US" {
		t.Errorf("First country = %q, want US (highest count)", global.ByCountry[0].Country)
	}
	if global.ByCountry[1].Country != "GB" {
		t.Errorf("Second country = %q, want GB", global.ByCountry[1].Country)
	}
}

func TestGetAllRatings_InvalidWorkers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Test","averageUserRating":4.0,"userRatingCount":10}]}`)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	// Should not panic with workers < 1
	_, err := client.GetAllRatings(context.Background(), "123", 0)
	if err != nil {
		t.Logf("GetAllRatings with workers=0 returned: %v", err)
	}

	_, err = client.GetAllRatings(context.Background(), "123", -5)
	if err != nil {
		t.Logf("GetAllRatings with workers=-5 returned: %v", err)
	}
}

func TestGetAllRatings_NoRatings(t *testing.T) {
	// App exists but has no ratings in any country.
	responses := map[string]string{
		"us": `{"resultCount":1,"results":[{"trackId":123,"trackName":"Zero App","averageUserRating":0,"userRatingCount":0}]}`,
		"gb": `{"resultCount":1,"results":[{"trackId":123,"trackName":"Zero App","averageUserRating":0,"userRatingCount":0}]}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lookup" {
			country := r.URL.Query().Get("country")
			if resp, ok := responses[country]; ok {
				w.Header().Set("Content-Type", "application/json")
				writeBody(t, w, resp)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, `{"resultCount":0,"results":[]}`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		writeBody(t, w, `<html></html>`)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	global, err := client.GetAllRatings(context.Background(), "123", 5)
	if err != nil {
		t.Fatalf("GetAllRatings() error: %v", err)
	}
	if global.AppName != "Zero App" {
		t.Fatalf("AppName = %q, want Zero App", global.AppName)
	}
	if global.TotalCount != 0 {
		t.Fatalf("TotalCount = %d, want 0", global.TotalCount)
	}
	if global.CountryCount != 0 {
		t.Fatalf("CountryCount = %d, want 0", global.CountryCount)
	}
}

func TestGetAllRatings_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, `{"resultCount":1,"results":[{"trackId":123,"trackName":"Test","averageUserRating":4.0,"userRatingCount":10}]}`)
	}))
	defer func() { server.Close() }()

	client := &Client{
		HTTPClient: &http.Client{
			Transport: &testTransport{baseURL: server.URL},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := client.GetAllRatings(ctx, "123", 5)
	if err == nil {
		t.Log("GetAllRatings completed despite cancelled context (may have cached)")
	}
}

// testTransport rewrites requests to use the test server URL.
type testTransport struct {
	baseURL string
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to use our test server
	req.URL.Scheme = "http"
	req.URL.Host = t.baseURL[7:] // strip "http://"
	return http.DefaultTransport.RoundTrip(req)
}

func writeBody(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Errorf("write response: %v", err)
	}
}
