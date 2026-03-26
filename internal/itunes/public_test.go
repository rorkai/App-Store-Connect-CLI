package itunes

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestNormalizeCountryCode(t *testing.T) {
	country, err := NormalizeCountryCode(" US ")
	if err != nil {
		t.Fatalf("NormalizeCountryCode() error: %v", err)
	}
	if country != "us" {
		t.Fatalf("NormalizeCountryCode() = %q, want us", country)
	}

	country, err = NormalizeCountryCode(" kz ")
	if err != nil {
		t.Fatalf("NormalizeCountryCode() error: %v", err)
	}
	if country != "kz" {
		t.Fatalf("NormalizeCountryCode() = %q, want kz", country)
	}

	if _, err := NormalizeCountryCode("usa"); err == nil {
		t.Fatal("expected invalid country code error")
	}
	if _, err := NormalizeCountryCode("zz"); err == nil {
		t.Fatal("expected unsupported public country code error")
	}
}

func TestLookupAppOmitsCountryWhenUnspecified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "123" {
			t.Fatalf("expected id=123, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "" {
			t.Fatalf("expected no country query parameter, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "" {
			t.Fatalf("expected no entity query parameter, got %q", got)
		}

		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha",
				"bundleId": "com.example.alpha",
				"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123",
				"artworkUrl512": "https://example.com/icon.png",
				"sellerName": "Alpha Inc",
				"primaryGenreName": "Games",
				"genres": ["Games"],
				"version": "1.0.0",
				"description": "Alpha description",
				"price": 0,
				"formattedPrice": "Free",
				"currency": "USD",
				"averageUserRating": 4.5,
				"userRatingCount": 12,
				"averageUserRatingForCurrentVersion": 4.5,
				"userRatingCountForCurrentVersion": 12
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	app, err := client.LookupApp(context.Background(), "123", LookupOptions{})
	if err != nil {
		t.Fatalf("LookupApp() error: %v", err)
	}
	if app.Country != "" {
		t.Fatalf("expected empty country, got %q", app.Country)
	}
	if app.CountryName != "" {
		t.Fatalf("expected empty country name, got %q", app.CountryName)
	}
	if app.AppID != 123 {
		t.Fatalf("AppID = %d, want 123", app.AppID)
	}
	if app.Name != "Alpha" {
		t.Fatalf("Name = %q, want Alpha", app.Name)
	}
}

func TestLookupAppIncludesCountryWhenProvided(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("country"); got != "us" {
			t.Fatalf("expected country=us, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha",
				"bundleId": "com.example.alpha",
				"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123",
				"artworkUrl512": "https://example.com/icon.png",
				"sellerName": "Alpha Inc",
				"primaryGenreName": "Games",
				"genres": ["Games"],
				"version": "1.0.0",
				"description": "Alpha description",
				"price": 0,
				"formattedPrice": "Free",
				"currency": "USD",
				"averageUserRating": 4.5,
				"userRatingCount": 12,
				"averageUserRatingForCurrentVersion": 4.5,
				"userRatingCountForCurrentVersion": 12
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	app, err := client.LookupApp(context.Background(), "123", LookupOptions{Country: "US", IncludeSoftwareEntity: true})
	if err != nil {
		t.Fatalf("LookupApp() error: %v", err)
	}
	if app.Country != "US" {
		t.Fatalf("Country = %q, want US", app.Country)
	}
	if app.CountryName != "United States" {
		t.Fatalf("CountryName = %q, want United States", app.CountryName)
	}
}

func TestLookupAppMatchesZeroPaddedRequestedID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("id"); got != "123" {
			t.Fatalf("expected canonical id=123, got %q", got)
		}
		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha",
				"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123"
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	app, err := client.LookupApp(context.Background(), "00123", LookupOptions{})
	if err != nil {
		t.Fatalf("LookupApp() error: %v", err)
	}
	if app.AppID != 123 {
		t.Fatalf("AppID = %d, want 123", app.AppID)
	}
}

func TestSearchAppsRequestShape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("term"); got != "focus" {
			t.Fatalf("expected term=focus, got %q", got)
		}
		if got := r.URL.Query().Get("country"); got != "us" {
			t.Fatalf("expected country=us, got %q", got)
		}
		if got := r.URL.Query().Get("entity"); got != "software" {
			t.Fatalf("expected entity=software, got %q", got)
		}
		if got := r.URL.Query().Get("limit"); got != "25" {
			t.Fatalf("expected limit=25, got %q", got)
		}

		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha Search",
				"bundleId": "com.example.alpha",
				"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123",
				"artworkUrl512": "https://example.com/icon.png",
				"sellerName": "Alpha Inc",
				"primaryGenreName": "Games",
				"formattedPrice": "Free",
				"currency": "USD",
				"averageUserRating": 4.5,
				"userRatingCount": 12
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	results, err := client.SearchApps(context.Background(), "focus", "US", 25)
	if err != nil {
		t.Fatalf("SearchApps() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Country != "US" {
		t.Fatalf("Country = %q, want US", results[0].Country)
	}
	if results[0].CountryName != "United States" {
		t.Fatalf("CountryName = %q, want United States", results[0].CountryName)
	}
}

func TestLookupAppOmitsSoftwareEntityWhenNotRequested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("entity"); got != "" {
			t.Fatalf("expected no entity query parameter, got %q", got)
		}
		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha",
				"trackViewUrl": "https://apps.apple.com/us/app/alpha/id123"
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	app, err := client.LookupApp(context.Background(), "123", LookupOptions{})
	if err != nil {
		t.Fatalf("LookupApp() error: %v", err)
	}
	if app.AppID != 123 {
		t.Fatalf("AppID = %d, want 123", app.AppID)
	}
}

func TestListStorefrontsDeterministicOrder(t *testing.T) {
	storefronts := ListStorefronts()
	if len(storefronts) == 0 {
		t.Fatal("expected storefronts")
	}

	countries := make([]string, 0, len(storefronts))
	for _, storefront := range storefronts {
		countries = append(countries, storefront.Country)
	}

	sorted := append([]string(nil), countries...)
	sort.Strings(sorted)
	if fmt.Sprint(sorted) != fmt.Sprint(countries) {
		t.Fatalf("storefront order is not deterministic: got %v, want %v", countries, sorted)
	}
	if storefronts[0].Country != "AE" {
		t.Fatalf("expected first storefront AE, got %q", storefronts[0].Country)
	}
}

func TestListStorefrontsIncludesPublicCountryWithoutHistogramStorefrontID(t *testing.T) {
	storefronts := ListStorefronts()

	for _, storefront := range storefronts {
		if storefront.Country != "KZ" {
			continue
		}
		if storefront.CountryName != "Kazakhstan" {
			t.Fatalf("CountryName = %q, want Kazakhstan", storefront.CountryName)
		}
		if storefront.StorefrontID != "" {
			t.Fatalf("StorefrontID = %q, want empty string", storefront.StorefrontID)
		}
		return
	}

	t.Fatal("expected KZ storefront in public storefront list")
}

func TestGetRatingsHistogramRequestHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lookup":
			if got := r.URL.Query().Get("country"); got != "us" {
				t.Fatalf("expected lookup country=us, got %q", got)
			}
			if got := r.URL.Query().Get("entity"); got != "software" {
				t.Fatalf("expected lookup entity=software, got %q", got)
			}
			writeBody(t, w, `{
				"resultCount": 1,
				"results": [{
					"trackId": 123,
					"trackName": "Alpha",
					"averageUserRating": 4.5,
					"userRatingCount": 12,
					"averageUserRatingForCurrentVersion": 4.5,
					"userRatingCountForCurrentVersion": 12
				}]
			}`)
		case "/us/customer-reviews/id123":
			if got := r.Header.Get("X-Apple-Store-Front"); got != Storefronts["us"]+",12" {
				t.Fatalf("expected storefront header %q, got %q", Storefronts["us"]+",12", got)
			}
			if got := r.URL.Query().Get("displayable-kind"); got != "11" {
				t.Fatalf("expected displayable-kind=11, got %q", got)
			}
			writeBody(t, w, `<span class="total">10</span><span class="total">1</span><span class="total">0</span><span class="total">0</span><span class="total">0</span>`)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	ratings, err := client.GetRatings(context.Background(), "123", "us")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}
	if ratings.Country != "US" {
		t.Fatalf("Country = %q, want US", ratings.Country)
	}
	if ratings.Histogram[5] != 10 {
		t.Fatalf("Histogram[5] = %d, want 10", ratings.Histogram[5])
	}
}

func TestGetRatingsAllowsCountryOutsideHistogramStorefrontMap(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/lookup" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("country"); got != "kz" {
			t.Fatalf("expected lookup country=kz, got %q", got)
		}
		writeBody(t, w, `{
			"resultCount": 1,
			"results": [{
				"trackId": 123,
				"trackName": "Alpha",
				"averageUserRating": 4.5,
				"userRatingCount": 12
			}]
		}`)
	}))
	defer server.Close()

	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}
	ratings, err := client.GetRatings(context.Background(), "123", "kz")
	if err != nil {
		t.Fatalf("GetRatings() error: %v", err)
	}
	if ratings.Country != "KZ" {
		t.Fatalf("Country = %q, want KZ", ratings.Country)
	}
	if len(ratings.Histogram) != 0 {
		t.Fatalf("expected empty histogram, got %+v", ratings.Histogram)
	}
}
