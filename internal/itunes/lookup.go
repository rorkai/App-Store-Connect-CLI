package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// App contains public App Store storefront metadata for a single app.
type App struct {
	AppID                int64    `json:"appId"`
	Name                 string   `json:"name"`
	BundleID             string   `json:"bundleId"`
	Country              string   `json:"country,omitempty"`
	CountryName          string   `json:"countryName,omitempty"`
	URL                  string   `json:"url"`
	ArtworkURL           string   `json:"artworkUrl"`
	SellerName           string   `json:"sellerName"`
	PrimaryGenreName     string   `json:"primaryGenreName"`
	Genres               []string `json:"genres,omitempty"`
	Version              string   `json:"version"`
	Description          string   `json:"description"`
	Price                float64  `json:"price"`
	FormattedPrice       string   `json:"formattedPrice"`
	Currency             string   `json:"currency"`
	AverageRating        float64  `json:"averageRating"`
	RatingCount          int64    `json:"ratingCount"`
	CurrentVersionRating float64  `json:"currentVersionRating"`
	CurrentVersionCount  int64    `json:"currentVersionCount"`
}

// LookupOptions controls /lookup behavior.
type LookupOptions struct {
	Country               string
	IncludeSoftwareEntity bool
}

type lookupResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []lookupResult `json:"results"`
}

type lookupResult struct {
	TrackID                            int64    `json:"trackId"`
	TrackName                          string   `json:"trackName"`
	BundleID                           string   `json:"bundleId"`
	TrackViewURL                       string   `json:"trackViewUrl"`
	ArtworkURL512                      string   `json:"artworkUrl512"`
	ArtworkURL100                      string   `json:"artworkUrl100"`
	SellerName                         string   `json:"sellerName"`
	PrimaryGenreName                   string   `json:"primaryGenreName"`
	Genres                             []string `json:"genres"`
	Version                            string   `json:"version"`
	Description                        string   `json:"description"`
	Price                              float64  `json:"price"`
	FormattedPrice                     string   `json:"formattedPrice"`
	Currency                           string   `json:"currency"`
	AverageUserRating                  float64  `json:"averageUserRating"`
	UserRatingCount                    int64    `json:"userRatingCount"`
	AverageUserRatingForCurrentVersion float64  `json:"averageUserRatingForCurrentVersion"`
	UserRatingCountForCurrentVersion   int64    `json:"userRatingCountForCurrentVersion"`
}

// NormalizeCountryCode validates and normalizes a storefront country code.
func NormalizeCountryCode(country string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(country))
	if normalized == "" {
		return "", nil
	}
	if len(normalized) != 2 {
		return "", fmt.Errorf("unsupported country code: %s", strings.TrimSpace(country))
	}
	for _, r := range normalized {
		if r < 'a' || r > 'z' {
			return "", fmt.Errorf("unsupported country code: %s", strings.TrimSpace(country))
		}
	}
	if _, ok := publicCountrySet[normalized]; !ok {
		return "", fmt.Errorf("unsupported country code: %s", strings.TrimSpace(country))
	}
	return normalized, nil
}

func canonicalizeLookupID(id string) string {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ""
	}
	parsed, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return trimmed
	}
	return strconv.FormatInt(parsed, 10)
}

// LookupApps fetches public App Store metadata for one or more app IDs.
func (c *Client) LookupApps(ctx context.Context, ids []string, opts LookupOptions) (map[string]App, error) {
	requestedIDs := make([]string, 0, len(ids))
	queryIDs := make([]string, 0, len(ids))
	seenQueryIDs := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		requestedIDs = append(requestedIDs, trimmed)
		canonicalID := canonicalizeLookupID(trimmed)
		if _, ok := seenQueryIDs[canonicalID]; ok {
			continue
		}
		seenQueryIDs[canonicalID] = struct{}{}
		queryIDs = append(queryIDs, canonicalID)
	}
	if len(queryIDs) == 0 {
		return nil, fmt.Errorf("app ID is required")
	}

	query := url.Values{}
	query.Set("id", strings.Join(queryIDs, ","))
	country, err := NormalizeCountryCode(opts.Country)
	if err != nil {
		return nil, err
	}
	if country != "" {
		query.Set("country", country)
	}
	if opts.IncludeSoftwareEntity {
		query.Set("entity", "software")
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/lookup", query)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("lookup request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lookup request returned status %d", resp.StatusCode)
	}

	var lookup lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&lookup); err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	appsByCanonicalID := make(map[string]App, len(lookup.Results))
	for _, result := range lookup.Results {
		if result.TrackID == 0 {
			continue
		}
		appsByCanonicalID[strconv.FormatInt(result.TrackID, 10)] = buildApp(result, country)
	}

	apps := make(map[string]App, len(appsByCanonicalID)+len(requestedIDs))
	for canonicalID, app := range appsByCanonicalID {
		apps[canonicalID] = app
	}
	for _, requestedID := range requestedIDs {
		app, ok := appsByCanonicalID[canonicalizeLookupID(requestedID)]
		if !ok {
			continue
		}
		apps[requestedID] = app
	}
	return apps, nil
}

// LookupApp fetches public App Store metadata for a single app ID.
func (c *Client) LookupApp(ctx context.Context, appID string, opts LookupOptions) (*App, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app ID is required")
	}
	canonicalAppID := canonicalizeLookupID(appID)

	apps, err := c.LookupApps(ctx, []string{appID}, opts)
	if err != nil {
		return nil, err
	}

	app, ok := apps[appID]
	if !ok && canonicalAppID != appID {
		app, ok = apps[canonicalAppID]
	}
	if !ok {
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	return &app, nil
}

func buildApp(result lookupResult, country string) App {
	artworkURL := strings.TrimSpace(result.ArtworkURL512)
	if artworkURL == "" {
		artworkURL = strings.TrimSpace(result.ArtworkURL100)
	}

	app := App{
		AppID:                result.TrackID,
		Name:                 strings.TrimSpace(result.TrackName),
		BundleID:             strings.TrimSpace(result.BundleID),
		URL:                  strings.TrimSpace(result.TrackViewURL),
		ArtworkURL:           artworkURL,
		SellerName:           strings.TrimSpace(result.SellerName),
		PrimaryGenreName:     strings.TrimSpace(result.PrimaryGenreName),
		Genres:               append([]string(nil), result.Genres...),
		Version:              strings.TrimSpace(result.Version),
		Description:          strings.TrimSpace(result.Description),
		Price:                result.Price,
		FormattedPrice:       strings.TrimSpace(result.FormattedPrice),
		Currency:             strings.TrimSpace(result.Currency),
		AverageRating:        result.AverageUserRating,
		RatingCount:          result.UserRatingCount,
		CurrentVersionRating: result.AverageUserRatingForCurrentVersion,
		CurrentVersionCount:  result.UserRatingCountForCurrentVersion,
	}
	if country != "" {
		app.Country = strings.ToUpper(country)
		app.CountryName = publicCountryName(country)
	}
	return app
}
