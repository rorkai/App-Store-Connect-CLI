package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Client is an iTunes Lookup API client.
type Client struct {
	HTTPClient *http.Client
}

// NewClient creates a new iTunes API client.
func NewClient() *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
	}
}

// AppRatings contains rating statistics for an app in a single country.
type AppRatings struct {
	AppID                int64         `json:"appId"`
	AppName              string        `json:"appName"`
	Country              string        `json:"country"`
	CountryName          string        `json:"countryName,omitempty"`
	AverageRating        float64       `json:"averageRating"`
	RatingCount          int64         `json:"ratingCount"`
	CurrentVersionRating float64       `json:"currentVersionRating,omitempty"`
	CurrentVersionCount  int64         `json:"currentVersionCount,omitempty"`
	Histogram            map[int]int64 `json:"histogram,omitempty"`
}

// GlobalRatings contains aggregated rating statistics across all countries.
type GlobalRatings struct {
	AppID         int64         `json:"appId"`
	AppName       string        `json:"appName"`
	AverageRating float64       `json:"averageRating"`
	TotalCount    int64         `json:"totalCount"`
	CountryCount  int           `json:"countryCount"`
	Histogram     map[int]int64 `json:"histogram,omitempty"`
	ByCountry     []AppRatings  `json:"byCountry"`
}

// lookupResponse is the response from iTunes Lookup API.
type lookupResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []lookupResult `json:"results"`
}

// lookupResult is a single app result from iTunes Lookup API.
type lookupResult struct {
	TrackID                            int64   `json:"trackId"`
	TrackName                          string  `json:"trackName"`
	AverageUserRating                  float64 `json:"averageUserRating"`
	UserRatingCount                    int64   `json:"userRatingCount"`
	AverageUserRatingForCurrentVersion float64 `json:"averageUserRatingForCurrentVersion"`
	UserRatingCountForCurrentVersion   int64   `json:"userRatingCountForCurrentVersion"`
}

// GetRatings fetches rating statistics for an app in a specific country.
func (c *Client) GetRatings(ctx context.Context, appID, country string) (*AppRatings, error) {
	country = strings.ToLower(strings.TrimSpace(country))
	if country == "" {
		country = "us"
	}

	// Fetch basic info from iTunes Lookup API
	lookupURL := fmt.Sprintf("https://itunes.apple.com/lookup?id=%s&country=%s&entity=software", appID, country)

	req, err := http.NewRequestWithContext(ctx, "GET", lookupURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lookup request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lookup request returned status %d", resp.StatusCode)
	}

	var lookup lookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&lookup); err != nil {
		return nil, fmt.Errorf("failed to parse lookup response: %w", err)
	}

	if lookup.ResultCount == 0 {
		return nil, fmt.Errorf("app not found: %s", appID)
	}

	app := lookup.Results[0]

	ratings := &AppRatings{
		AppID:                app.TrackID,
		AppName:              app.TrackName,
		Country:              strings.ToUpper(country),
		CountryName:          CountryNames[country],
		AverageRating:        app.AverageUserRating,
		RatingCount:          app.UserRatingCount,
		CurrentVersionRating: app.AverageUserRatingForCurrentVersion,
		CurrentVersionCount:  app.UserRatingCountForCurrentVersion,
		Histogram:            make(map[int]int64),
	}

	// Fetch histogram from HTML endpoint. Non-fatal: histogram is optional enhancement.
	_ = c.fetchHistogram(ctx, appID, country, ratings)

	return ratings, nil
}

// fetchHistogram scrapes the ratings histogram from the iTunes customer reviews page.
func (c *Client) fetchHistogram(ctx context.Context, appID, country string, ratings *AppRatings) error {
	storefront, ok := Storefronts[country]
	if !ok {
		return fmt.Errorf("unknown country code: %s", country)
	}

	histogramURL := fmt.Sprintf("https://itunes.apple.com/%s/customer-reviews/id%s?displayable-kind=11", country, appID)

	req, err := http.NewRequestWithContext(ctx, "GET", histogramURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create histogram request: %w", err)
	}
	req.Header.Set("X-Apple-Store-Front", storefront+",12")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("histogram request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("histogram request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read histogram response: %w", err)
	}

	// Extract values from <span class="total">NUMBER</span>
	re := regexp.MustCompile(`<span class="total">([0-9,]+)</span>`)
	matches := re.FindAllStringSubmatch(string(body), 5)

	stars := []int{5, 4, 3, 2, 1}
	for i, match := range matches {
		if i < len(stars) && len(match) > 1 {
			raw := strings.ReplaceAll(match[1], ",", "")
			count, _ := strconv.ParseInt(raw, 10, 64)
			ratings.Histogram[stars[i]] = count
		}
	}

	return nil
}

// GetAllRatings fetches rating statistics for an app across all supported countries.
func (c *Client) GetAllRatings(ctx context.Context, appID string, workers int) (*GlobalRatings, error) {
	if workers < 1 {
		workers = 10
	}

	countries := AllCountries()

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		results   []*AppRatings
		appName   string
		appIDInt  int64
		total     int64
		weighted  float64
		found     bool
		histogram = make(map[int]int64)
	)

	// Semaphore for limiting concurrency
	sem := make(chan struct{}, workers)

	for _, country := range countries {
		wg.Add(1)
		go func(country string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}: // acquire
				defer func() { <-sem }() // release
			}

			ratings, err := c.GetRatings(ctx, appID, country)
			if err != nil {
				return
			}

			mu.Lock()
			if !found {
				found = true
				appName = ratings.AppName
				appIDInt = ratings.AppID
			}
			if ratings.RatingCount == 0 {
				mu.Unlock()
				return
			}

			results = append(results, ratings)
			total += ratings.RatingCount
			weighted += ratings.AverageRating * float64(ratings.RatingCount)

			// Aggregate histogram
			for star, count := range ratings.Histogram {
				histogram[star] += count
			}
			mu.Unlock()
		}(country)
	}

	wg.Wait()

	// Check for context cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if !found {
		return nil, fmt.Errorf("app not found in any country: %s", appID)
	}
	if len(results) == 0 {
		return &GlobalRatings{
			AppID:         appIDInt,
			AppName:       appName,
			AverageRating: 0,
			TotalCount:    0,
			CountryCount:  0,
			Histogram:     histogram,
			ByCountry:     nil,
		}, nil
	}

	// Sort by rating count descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].RatingCount > results[j].RatingCount
	})

	// Calculate global average
	globalAvg := float64(0)
	if total > 0 {
		globalAvg = weighted / float64(total)
	}

	// Convert to value slice for JSON serialization
	byCountry := make([]AppRatings, len(results))
	for i, r := range results {
		byCountry[i] = *r
	}

	return &GlobalRatings{
		AppID:         appIDInt,
		AppName:       appName,
		AverageRating: globalAvg,
		TotalCount:    total,
		CountryCount:  len(results),
		Histogram:     histogram,
		ByCountry:     byCountry,
	}, nil
}
