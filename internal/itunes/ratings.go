package itunes

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

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

// GetRatings fetches rating statistics for an app in a specific country.
func (c *Client) GetRatings(ctx context.Context, appID, country string) (*AppRatings, error) {
	normalizedCountry := strings.ToLower(strings.TrimSpace(country))
	if normalizedCountry == "" {
		normalizedCountry = "us"
	}

	app, err := c.LookupApp(ctx, appID, LookupOptions{
		Country:               normalizedCountry,
		IncludeSoftwareEntity: true,
	})
	if err != nil {
		return nil, err
	}

	ratings := &AppRatings{
		AppID:                app.AppID,
		AppName:              app.Name,
		Country:              app.Country,
		CountryName:          app.CountryName,
		AverageRating:        app.AverageRating,
		RatingCount:          app.RatingCount,
		CurrentVersionRating: app.CurrentVersionRating,
		CurrentVersionCount:  app.CurrentVersionCount,
		Histogram:            make(map[int]int64),
	}

	// Histogram scraping is best-effort and must remain non-fatal.
	_ = c.fetchHistogram(ctx, appID, normalizedCountry, ratings)
	return ratings, nil
}

func (c *Client) fetchHistogram(ctx context.Context, appID, country string, ratings *AppRatings) error {
	storefront, ok := Storefronts[country]
	if !ok {
		return fmt.Errorf("unknown country code: %s", country)
	}

	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("/%s/customer-reviews/id%s", country, appID), nil)
	if err != nil {
		return fmt.Errorf("failed to create histogram request: %w", err)
	}
	q := req.URL.Query()
	q.Set("displayable-kind", "11")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Apple-Store-Front", storefront+",12")
	req.Header.Set("Accept", "text/html")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("histogram request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("histogram request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read histogram response: %w", err)
	}

	re := regexp.MustCompile(`<span class="total">([0-9,]+)</span>`)
	matches := re.FindAllStringSubmatch(string(body), 5)
	stars := []int{5, 4, 3, 2, 1}
	for i, match := range matches {
		if i >= len(stars) || len(match) < 2 {
			continue
		}
		raw := strings.ReplaceAll(match[1], ",", "")
		count, _ := strconv.ParseInt(raw, 10, 64)
		ratings.Histogram[stars[i]] = count
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

	sem := make(chan struct{}, workers)

	for _, country := range countries {
		wg.Add(1)
		go func(country string) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
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
			for star, count := range ratings.Histogram {
				histogram[star] += count
			}
			mu.Unlock()
		}(country)
	}

	wg.Wait()

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

	sort.Slice(results, func(i, j int) bool {
		return results[i].RatingCount > results[j].RatingCount
	})

	globalAvg := float64(0)
	if total > 0 {
		globalAvg = weighted / float64(total)
	}

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
