package itunes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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

type allRatingsLookupError struct {
	appID string
	cause error
}

func (e *allRatingsLookupError) Error() string {
	return fmt.Sprintf("app not found in any country: %s", e.appID)
}

func (e *allRatingsLookupError) Unwrap() error {
	return e.cause
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
	if err := c.fetchHistogram(ctx, appID, normalizedCountry, ratings); err != nil {
		// A best-effort auxiliary request may fail for storefront reasons, but
		// it must not turn an explicit caller cancellation into a successful
		// ratings response.
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	return ratings, nil
}

func (c *Client) fetchHistogram(ctx context.Context, appID, country string, ratings *AppRatings) error {
	storefront, ok := Storefronts[country]
	if !ok {
		return fmt.Errorf("unknown country code: %s", country)
	}

	req, err := c.newRequest(ctx, fmt.Sprintf("/%s/customer-reviews/id%s", country, appID), nil)
	if err != nil {
		return fmt.Errorf("failed to create histogram request: %w", err)
	}
	q := req.URL.Query()
	q.Set("displayable-kind", "11")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-Apple-Store-Front", storefront+",12")
	req.Header.Set("Accept", "text/html")

	return c.do(ctx, "histogram", req, func(resp *http.Response) error {
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
	})
}

// GetAllRatings fetches rating statistics for an app across all supported countries.
func (c *Client) GetAllRatings(
	ctx context.Context,
	appID string,
	workers int,
	newCountryContext func(context.Context) (context.Context, context.CancelFunc),
) (*GlobalRatings, error) {
	if workers < 1 {
		workers = 10
	}
	if newCountryContext == nil {
		return nil, fmt.Errorf("country context factory is required")
	}

	workCtx, cancelWork := context.WithCancel(ctx)
	defer cancelWork()

	countries := AllCountries()

	var (
		mu                 sync.Mutex
		wg                 sync.WaitGroup
		deadlineOnce       sync.Once
		countryDeadlineErr error
		retryableStop      bool
		retryDelayStopErr  error
		httpFailureCount   int
		httpFailures       = make(map[int]error)
		results            []*AppRatings
		appName            string
		appIDInt           int64
		total              int64
		weighted           float64
		found              bool
		histogram          = make(map[int]int64)
	)

	sem := make(chan struct{}, workers)

	for _, country := range countries {
		wg.Add(1)
		go func(country string) {
			defer wg.Done()

			select {
			case <-workCtx.Done():
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}
			if workCtx.Err() != nil {
				return
			}

			countryCtx, countryCancel := newCountryContext(workCtx)
			ratings, err := c.GetRatings(countryCtx, appID, country)
			countryErr := countryCtx.Err()
			countryCancel()
			if err != nil {
				// A preserved retryable storefront status may arrive after the child
				// deadline; keep it instead of misclassifying it as a deadline failure.
				var statusError interface{ HTTPStatusCode() int }
				if errors.As(err, &statusError) && isRetryablePublicStatus(statusError.HTTPStatusCode()) {
					isRetryDelayStop := asc.IsRetryDelayExceeded(err)
					stopQueuedWork := isRetryDelayStop || errors.Is(err, context.DeadlineExceeded) || errors.Is(countryErr, context.DeadlineExceeded)
					mu.Lock()
					httpFailureCount++
					status := statusError.HTTPStatusCode()
					if _, exists := httpFailures[status]; !exists {
						httpFailures[status] = err
					}
					if stopQueuedWork {
						retryableStop = true
					}
					if isRetryDelayStop && retryDelayStopErr == nil {
						retryDelayStopErr = err
					}
					mu.Unlock()
					if stopQueuedWork {
						cancelWork()
					}
					return
				}
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(countryErr, context.DeadlineExceeded) {
					deadlineOnce.Do(func() {
						countryDeadlineErr = context.DeadlineExceeded
						cancelWork()
					})
					return
				}
				if errors.As(err, &statusError) {
					mu.Lock()
					httpFailureCount++
					status := statusError.HTTPStatusCode()
					if _, exists := httpFailures[status]; !exists {
						httpFailures[status] = err
					}
					mu.Unlock()
				}
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
	if retryableStop {
		return nil, &allRatingsLookupError{appID: appID, cause: ratingsRetryableStopCause(httpFailures, retryDelayStopErr)}
	}
	if countryDeadlineErr != nil {
		return nil, countryDeadlineErr
	}
	if !found {
		if httpFailureCount == len(countries) {
			return nil, &allRatingsLookupError{appID: appID, cause: preferredRatingsHTTPError(httpFailures)}
		}
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

func ratingsRetryableStopCause(failures map[int]error, retryDelayStopErr error) error {
	preferred := preferredRatingsHTTPError(failures)
	if retryDelayStopErr == nil {
		return preferred
	}
	if preferred == nil {
		return retryDelayStopErr
	}
	return errors.Join(preferred, retryDelayStopErr)
}

func preferredRatingsHTTPError(failures map[int]error) error {
	selectedStatus := 0
	var selected error
	for status, err := range failures {
		// Server failures take precedence over client failures because they best
		// represent a full-storefront outage. Within a class, use the lowest
		// status so the result is stable regardless of goroutine completion order.
		if selected == nil || (status >= 500 && selectedStatus < 500) ||
			(status/100 == selectedStatus/100 && status < selectedStatus) {
			selectedStatus = status
			selected = err
		}
	}
	return selected
}
