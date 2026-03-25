package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SearchResult contains public App Store search data for a single app result.
type SearchResult struct {
	AppID            int64   `json:"appId"`
	Name             string  `json:"name"`
	BundleID         string  `json:"bundleId"`
	SellerName       string  `json:"sellerName"`
	Country          string  `json:"country"`
	CountryName      string  `json:"countryName,omitempty"`
	URL              string  `json:"url"`
	ArtworkURL       string  `json:"artworkUrl"`
	PrimaryGenreName string  `json:"primaryGenreName"`
	FormattedPrice   string  `json:"formattedPrice"`
	Currency         string  `json:"currency"`
	AverageRating    float64 `json:"averageRating"`
	RatingCount      int64   `json:"ratingCount"`
}

type searchResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []lookupResult `json:"results"`
}

// SearchApps searches the public App Store in a single storefront.
func (c *Client) SearchApps(ctx context.Context, term, country string, limit int) ([]SearchResult, error) {
	query := url.Values{}
	query.Set("term", strings.TrimSpace(term))
	normalizedCountry, err := NormalizeCountryCode(country)
	if err != nil {
		return nil, err
	}
	if normalizedCountry != "" {
		query.Set("country", normalizedCountry)
	}
	query.Set("entity", "software")
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/search", query)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("search request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search request returned status %d", resp.StatusCode)
	}

	var payload searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to parse search response: %w", err)
	}

	results := make([]SearchResult, 0, len(payload.Results))
	for _, result := range payload.Results {
		app := buildApp(result, normalizedCountry)
		results = append(results, SearchResult{
			AppID:            app.AppID,
			Name:             app.Name,
			BundleID:         app.BundleID,
			SellerName:       app.SellerName,
			Country:          app.Country,
			CountryName:      app.CountryName,
			URL:              app.URL,
			ArtworkURL:       app.ArtworkURL,
			PrimaryGenreName: app.PrimaryGenreName,
			FormattedPrice:   app.FormattedPrice,
			Currency:         app.Currency,
			AverageRating:    app.AverageRating,
			RatingCount:      app.RatingCount,
		})
	}

	return results, nil
}
