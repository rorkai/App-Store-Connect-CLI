package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const analyticsAPIBaseURL = appStoreBaseURL + "/analytics/api/v1"

// NewAnalyticsClient creates an analytics client reusing an authenticated web session.
func NewAnalyticsClient(session *AuthSession) *Client {
	return &Client{
		httpClient:         session.Client,
		baseURL:            analyticsAPIBaseURL,
		minRequestInterval: resolveWebMinRequestInterval(),
	}
}

// AnalyticsDimensionFilter preserves private analytics filter payloads.
type AnalyticsDimensionFilter map[string]any

// AnalyticsDimensionSort describes a sorted dimension lookup request.
type AnalyticsDimensionSort struct {
	Rank      string `json:"rank,omitempty"`
	Dimension string `json:"dimension,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// AnalyticsMeasuresRequest describes a private measures query.
type AnalyticsMeasuresRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Measures         []string
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsTimeseriesRequest describes a private timeseries query.
type AnalyticsTimeseriesRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Measures         []string
	Frequency        string
	Group            *AnalyticsTimeseriesGroup
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsDimensionsRequest describes a private grouped-dimensions query.
type AnalyticsDimensionsRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Measure          string
	Dimensions       []string
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
	Limit            int
	HideEmptyValues  bool
}

// AnalyticsDimensionValuesRequest describes a private dimension lookup query.
type AnalyticsDimensionValuesRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Measure          string
	Dimensions       []AnalyticsDimensionSort
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsRetentionRequest describes a private retention query.
type AnalyticsRetentionRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsCohortsRequest describes a private cohorts query.
type AnalyticsCohortsRequest struct {
	AppID            string
	StartDate        string
	EndDate          string
	StartTime        string
	EndTime          string
	Measures         []string
	Periods          []string
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsMeasurePoint is a single date/value point in a measure series.
type AnalyticsMeasurePoint struct {
	Date  string   `json:"date,omitempty"`
	Value *float64 `json:"value,omitempty"`
}

// AnalyticsMeasureResult is a single measure series from the private analytics API.
type AnalyticsMeasureResult struct {
	AdamID         string                  `json:"adamId,omitempty"`
	Measure        string                  `json:"measure,omitempty"`
	Total          *float64                `json:"total,omitempty"`
	Type           string                  `json:"type,omitempty"`
	PreviousTotal  *float64                `json:"previousTotal,omitempty"`
	PercentChange  *float64                `json:"percentChange,omitempty"`
	MeetsThreshold bool                    `json:"meetsThreshold,omitempty"`
	Data           []AnalyticsMeasurePoint `json:"data,omitempty"`
}

// AnalyticsMeasuresResponse wraps multiple measure series.
type AnalyticsMeasuresResponse struct {
	Size    int                      `json:"size,omitempty"`
	Results []AnalyticsMeasureResult `json:"results,omitempty"`
}

// AnalyticsTimeseriesResult is a dynamic private timeseries response row set.
type AnalyticsTimeseriesResult struct {
	AdamID string           `json:"adamId,omitempty"`
	Group  any              `json:"group,omitempty"`
	Data   []map[string]any `json:"data,omitempty"`
	Totals map[string]any   `json:"totals,omitempty"`
}

// AnalyticsTimeseriesResponse wraps private timeseries responses.
type AnalyticsTimeseriesResponse struct {
	Size    int                         `json:"size,omitempty"`
	Results []AnalyticsTimeseriesResult `json:"results,omitempty"`
}

// AnalyticsDimensionDataPoint is a grouped metric row.
type AnalyticsDimensionDataPoint struct {
	Key            string  `json:"key,omitempty"`
	Value          float64 `json:"value,omitempty"`
	MeetsThreshold bool    `json:"meetsThreshold,omitempty"`
}

// AnalyticsDimensionsResult is a grouped metric response.
type AnalyticsDimensionsResult struct {
	AdamID    string                        `json:"adamId,omitempty"`
	Measure   string                        `json:"measure,omitempty"`
	Dimension string                        `json:"dimension,omitempty"`
	Frequency string                        `json:"frequency,omitempty"`
	Type      string                        `json:"type,omitempty"`
	Total     *float64                      `json:"total,omitempty"`
	Data      []AnalyticsDimensionDataPoint `json:"data,omitempty"`
}

// AnalyticsDimensionsResponse wraps grouped metric responses.
type AnalyticsDimensionsResponse struct {
	Size    int                         `json:"size,omitempty"`
	Results []AnalyticsDimensionsResult `json:"results,omitempty"`
}

// AnalyticsDimensionValuesResult holds lookup values for a dimension.
type AnalyticsDimensionValuesResult struct {
	AdamID     string           `json:"adamId,omitempty"`
	Dimension  string           `json:"dimension,omitempty"`
	ActualSize int              `json:"actualSize,omitempty"`
	Values     []map[string]any `json:"values,omitempty"`
}

// AnalyticsDimensionValuesResponse wraps dimension lookup responses.
type AnalyticsDimensionValuesResponse struct {
	Size    int                              `json:"size,omitempty"`
	Results []AnalyticsDimensionValuesResult `json:"results,omitempty"`
}

// AnalyticsRetentionPoint is a retention value for a specific date.
type AnalyticsRetentionPoint struct {
	Date                string   `json:"date,omitempty"`
	RetentionPercentage *float64 `json:"retentionPercentage,omitempty"`
	Value               *float64 `json:"value,omitempty"`
}

// AnalyticsRetentionCohort is one retention cohort in the private API response.
type AnalyticsRetentionCohort struct {
	AppPurchase    string                    `json:"appPurchase,omitempty"`
	MeetsThreshold bool                      `json:"meetsThreshold,omitempty"`
	Data           []AnalyticsRetentionPoint `json:"data,omitempty"`
}

// AnalyticsRetentionResponse wraps retention results.
type AnalyticsRetentionResponse struct {
	AdamID  string                     `json:"adamId,omitempty"`
	Results []AnalyticsRetentionCohort `json:"results,omitempty"`
}

// AnalyticsCohortsResponse preserves the wide private cohort response shape.
type AnalyticsCohortsResponse struct {
	Results map[string][]any `json:"results,omitempty"`
}

// AnalyticsBreakdownItem is a label-resolved breakdown row.
type AnalyticsBreakdownItem struct {
	Key            string  `json:"key,omitempty"`
	Label          string  `json:"label,omitempty"`
	Value          float64 `json:"value,omitempty"`
	MeetsThreshold bool    `json:"meetsThreshold,omitempty"`
}

// AnalyticsBreakdown describes a grouped dimension result with resolved labels.
type AnalyticsBreakdown struct {
	Name      string                   `json:"name,omitempty"`
	Measure   string                   `json:"measure,omitempty"`
	Dimension string                   `json:"dimension,omitempty"`
	Frequency string                   `json:"frequency,omitempty"`
	Total     *float64                 `json:"total,omitempty"`
	Items     []AnalyticsBreakdownItem `json:"items,omitempty"`
}

// AnalyticsOverview bundles the private overview page data families.
type AnalyticsOverview struct {
	AppID              string                      `json:"appId"`
	StartDate          string                      `json:"startDate"`
	EndDate            string                      `json:"endDate"`
	Acquisition        []AnalyticsMeasureResult    `json:"acquisition,omitempty"`
	Sales              []AnalyticsMeasureResult    `json:"sales,omitempty"`
	Subscriptions      []AnalyticsMeasureResult    `json:"subscriptions,omitempty"`
	PlanTimeline       []AnalyticsTimeseriesResult `json:"planTimeline,omitempty"`
	DownloadToPaid     *AnalyticsCohortsResponse   `json:"downloadToPaid,omitempty"`
	Retention          *AnalyticsRetentionResponse `json:"retention,omitempty"`
	FeatureBreakdowns  []AnalyticsBreakdown        `json:"featureBreakdowns,omitempty"`
	AppUsageBreakdowns []AnalyticsBreakdown        `json:"appUsageBreakdowns,omitempty"`
}

// AnalyticsSubscriptionsSummary bundles the private subscriptions summary page.
type AnalyticsSubscriptionsSummary struct {
	AppID                     string                      `json:"appId"`
	StartDate                 string                      `json:"startDate"`
	EndDate                   string                      `json:"endDate"`
	Summary                   []AnalyticsMeasureResult    `json:"summary,omitempty"`
	PlanTimeline              []AnalyticsTimeseriesResult `json:"planTimeline,omitempty"`
	ActivePlansBySubscription *AnalyticsBreakdown         `json:"activePlansBySubscription,omitempty"`
	SubscriptionRetention     *AnalyticsCohortsResponse   `json:"subscriptionRetention,omitempty"`
}

func analyticsHeaders(referer string) http.Header {
	headers := make(http.Header)
	headers.Set("Accept", "application/json")
	headers.Set("Content-Type", "application/json")
	headers.Set("X-Requested-By", "appstoreconnect.apple.com")
	headers.Set("Origin", appStoreBaseURL)
	if referer != "" {
		headers.Set("Referer", referer)
	}
	return headers
}

func analyticsAppReferer(appID string) string {
	return appStoreBaseURL + "/apps/" + url.PathEscape(strings.TrimSpace(appID)) + "/analytics"
}

func (c *Client) analyticsBaseURL() string {
	baseURL := strings.TrimSpace(c.baseURL)
	if baseURL == "" {
		return analyticsAPIBaseURL
	}
	return baseURL
}

func (c *Client) doAnalyticsRequest(ctx context.Context, path string, body any, referer string) ([]byte, error) {
	return c.doRequestBase(ctx, c.analyticsBaseURL(), http.MethodPost, path, body, analyticsHeaders(referer))
}

// NormalizeAnalyticsFrequency validates analytics frequency values shared by the
// CLI layer and the private web client.
func NormalizeAnalyticsFrequency(raw string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		value = "day"
	}
	switch value {
	case "day", "week", "month":
		return value, nil
	default:
		return "", fmt.Errorf("frequency must be one of day, week, month")
	}
}

func analyticsTimeRange(startDate, endDate, startTime, endTime string) (string, string, error) {
	startTime = strings.TrimSpace(startTime)
	endTime = strings.TrimSpace(endTime)
	if startTime != "" || endTime != "" {
		if startTime == "" || endTime == "" {
			return "", "", fmt.Errorf("startTime and endTime must be provided together")
		}
		return startTime, endTime, nil
	}
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	if startDate == "" || endDate == "" {
		return "", "", fmt.Errorf("start and end dates are required")
	}
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return "", "", fmt.Errorf("invalid start date %q", startDate)
	}
	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", fmt.Errorf("invalid end date %q", endDate)
	}
	if end.Before(start) {
		return "", "", fmt.Errorf("end date must be on or after start date")
	}
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

func normalizeStringList(values []string, field string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("%s must include at least one value", field)
	}
	return result, nil
}

func monthlyRecurringRevenueRange(startDate, endDate string) (string, string, bool, error) {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(startDate))
	if err != nil {
		return "", "", false, fmt.Errorf("invalid start date %q", startDate)
	}
	end, err := time.Parse("2006-01-02", strings.TrimSpace(endDate))
	if err != nil {
		return "", "", false, fmt.Errorf("invalid end date %q", endDate)
	}
	if end.Before(start) {
		return "", "", false, fmt.Errorf("end date must be on or after start date")
	}
	startMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	if start.Day() != 1 {
		startMonth = startMonth.AddDate(0, 1, 0)
	}
	endMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastDayOfMonth := endMonth.AddDate(0, 1, -1)
	if end.Day() != lastDayOfMonth.Day() {
		endMonth = endMonth.AddDate(0, -1, 0)
		lastDayOfMonth = endMonth.AddDate(0, 1, -1)
	}
	if lastDayOfMonth.Before(startMonth) {
		return "", "", false, nil
	}
	return startMonth.Format("2006-01-02"), lastDayOfMonth.Format("2006-01-02"), true, nil
}

func overviewRetentionRange(endDate string) (string, string, error) {
	end, err := time.Parse("2006-01-02", strings.TrimSpace(endDate))
	if err != nil {
		return "", "", fmt.Errorf("invalid end date %q", endDate)
	}
	start := end.AddDate(0, 0, -29)
	return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
}

func subscriptionRetentionWindow(endDate string) (string, string, error) {
	end, err := time.Parse("2006-01-02", strings.TrimSpace(endDate))
	if err != nil {
		return "", "", fmt.Errorf("invalid end date %q", endDate)
	}
	completedMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastDay := completedMonth.AddDate(0, 1, -1)
	if end.Day() != lastDay.Day() {
		completedMonth = completedMonth.AddDate(0, -1, 0)
		lastDay = completedMonth.AddDate(0, 1, -1)
	}
	start := completedMonth.AddDate(0, -11, 0)
	return start.Format(time.RFC3339), time.Date(lastDay.Year(), lastDay.Month(), lastDay.Day(), 23, 59, 59, 0, time.UTC).Format(time.RFC3339), nil
}

// GetAnalyticsMeasures queries private analytics measure series.
func (c *Client) GetAnalyticsMeasures(ctx context.Context, req AnalyticsMeasuresRequest) (*AnalyticsMeasuresResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	measures, err := normalizeStringList(req.Measures, "measures")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"adamId":    []string{strings.TrimSpace(req.AppID)},
		"startTime": startTime,
		"endTime":   endTime,
		"measures":  measures,
		"frequency": frequency,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/app/detail/measures", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsMeasuresResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics measures response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsTimeseries queries private analytics timeseries rows.
func (c *Client) GetAnalyticsTimeseries(ctx context.Context, req AnalyticsTimeseriesRequest) (*AnalyticsTimeseriesResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	measures, err := normalizeStringList(req.Measures, "measures")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"adamId":    []string{strings.TrimSpace(req.AppID)},
		"startTime": startTime,
		"endTime":   endTime,
		"measures":  measures,
		"frequency": frequency,
	}
	if req.Group != nil {
		payload["group"] = req.Group
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/timeseries", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsTimeseriesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics timeseries response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsDimensions queries grouped dimension rows.
func (c *Client) GetAnalyticsDimensions(ctx context.Context, req AnalyticsDimensionsRequest) (*AnalyticsDimensionsResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Measure) == "" {
		return nil, fmt.Errorf("measure is required")
	}
	dimensions, err := normalizeStringList(req.Dimensions, "dimensions")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 4
	}

	payload := map[string]any{
		"adamId":          []string{strings.TrimSpace(req.AppID)},
		"startTime":       startTime,
		"endTime":         endTime,
		"measure":         strings.TrimSpace(req.Measure),
		"dimensions":      dimensions,
		"frequency":       frequency,
		"limit":           limit,
		"hideEmptyValues": req.HideEmptyValues,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/app/detail/dimensions", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsDimensionsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics dimensions response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsDimensionValues queries available dimension values for a measure.
func (c *Client) GetAnalyticsDimensionValues(ctx context.Context, req AnalyticsDimensionValuesRequest) (*AnalyticsDimensionValuesResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Measure) == "" {
		return nil, fmt.Errorf("measure is required")
	}
	if len(req.Dimensions) == 0 {
		return nil, fmt.Errorf("dimensions are required")
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"adamId":     []string{strings.TrimSpace(req.AppID)},
		"startTime":  startTime,
		"endTime":    endTime,
		"measure":    strings.TrimSpace(req.Measure),
		"dimensions": req.Dimensions,
		"frequency":  frequency,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/dimension-values", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsDimensionValuesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics dimension values response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsRetention queries private retention data.
func (c *Client) GetAnalyticsRetention(ctx context.Context, req AnalyticsRetentionRequest) (*AnalyticsRetentionResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"adamId":    []string{strings.TrimSpace(req.AppID)},
		"startTime": startTime,
		"endTime":   endTime,
		"frequency": frequency,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/retention", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsRetentionResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics retention response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsCohorts queries private cohort data.
func (c *Client) GetAnalyticsCohorts(ctx context.Context, req AnalyticsCohortsRequest) (*AnalyticsCohortsResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	startTime, endTime, err := analyticsTimeRange(req.StartDate, req.EndDate, req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	measures, err := normalizeStringList(req.Measures, "measures")
	if err != nil {
		return nil, err
	}
	periods, err := normalizeStringList(req.Periods, "periods")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{
		"adamId":    strings.TrimSpace(req.AppID),
		"startTime": startTime,
		"endTime":   endTime,
		"measures":  measures,
		"periods":   periods,
		"frequency": frequency,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}

	body, err := c.doAnalyticsRequest(ctx, "/data/cohorts", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsCohortsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics cohorts response: %w", err)
	}
	return &result, nil
}

func buildAnalyticsBreakdown(name string, grouped *AnalyticsDimensionsResponse, values *AnalyticsDimensionValuesResponse) *AnalyticsBreakdown {
	if grouped == nil || len(grouped.Results) == 0 {
		return nil
	}
	result := grouped.Results[0]
	labels := map[string]string{}
	if values != nil {
		for _, valueResult := range values.Results {
			for _, item := range valueResult.Values {
				id := strings.TrimSpace(fmt.Sprint(item["id"]))
				if id == "" || id == "<nil>" {
					continue
				}
				label := strings.TrimSpace(fmt.Sprint(item["title"]))
				if label == "" || label == "<nil>" {
					label = strings.TrimSpace(fmt.Sprint(item["name"]))
				}
				if label != "" && label != "<nil>" {
					labels[id] = label
				}
			}
		}
	}

	items := make([]AnalyticsBreakdownItem, 0, len(result.Data))
	for _, item := range result.Data {
		label := strings.TrimSpace(labels[item.Key])
		if label == "" {
			label = item.Key
		}
		items = append(items, AnalyticsBreakdownItem{
			Key:            item.Key,
			Label:          label,
			Value:          item.Value,
			MeetsThreshold: item.MeetsThreshold,
		})
	}

	return &AnalyticsBreakdown{
		Name:      name,
		Measure:   result.Measure,
		Dimension: result.Dimension,
		Frequency: result.Frequency,
		Total:     result.Total,
		Items:     items,
	}
}

func (c *Client) getAnalyticsBreakdown(
	ctx context.Context,
	appID, name, measure, dimension, startDate, endDate string,
) (*AnalyticsBreakdown, error) {
	grouped, err := c.GetAnalyticsDimensions(ctx, AnalyticsDimensionsRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measure:          measure,
		Dimensions:       []string{dimension},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
		Limit:            4,
		HideEmptyValues:  true,
	})
	if err != nil {
		return nil, err
	}
	values, err := c.GetAnalyticsDimensionValues(ctx, AnalyticsDimensionValuesRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measure:          measure,
		Dimensions:       []AnalyticsDimensionSort{{Rank: "DESCENDING", Dimension: dimension, Limit: 200}},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, err
	}
	return buildAnalyticsBreakdown(name, grouped, values), nil
}

// GetAnalyticsOverview recreates the overview dashboard from private web endpoints.
func (c *Client) GetAnalyticsOverview(ctx context.Context, appID, startDate, endDate string) (*AnalyticsOverview, error) {
	acquisition, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:     appID,
		StartDate: startDate,
		EndDate:   endDate,
		Measures: []string{
			"units",
			"redownloads",
			"conversionRate",
			"impressionsTotal",
			"pageViewCount",
			"updates",
		},
		Frequency: "day",
	})
	if err != nil {
		return nil, fmt.Errorf("overview acquisition: %w", err)
	}
	sales, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:     appID,
		StartDate: startDate,
		EndDate:   endDate,
		Measures:  []string{"proceeds", "payingUsers", "iap"},
		Frequency: "day",
	})
	if err != nil {
		return nil, fmt.Errorf("overview sales: %w", err)
	}
	subscriptions, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"subscription-state-plans-active", "subscription-state-paid"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("overview subscriptions: %w", err)
	}
	if mrrStart, mrrEnd, ok, err := monthlyRecurringRevenueRange(startDate, endDate); err != nil {
		return nil, fmt.Errorf("overview monthly recurring revenue range: %w", err)
	} else if ok {
		mrr, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
			AppID:            appID,
			StartDate:        mrrStart,
			EndDate:          mrrEnd,
			Measures:         []string{"revenue-recurring"},
			Frequency:        "month",
			DimensionFilters: []AnalyticsDimensionFilter{},
		})
		if err != nil {
			return nil, fmt.Errorf("overview monthly recurring revenue: %w", err)
		}
		subscriptions.Results = append(subscriptions.Results, mrr.Results...)
	}
	timeline, err := c.GetAnalyticsTimeseries(ctx, AnalyticsTimeseriesRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"summary-plans-paid-net", "summary-plans-paid-starts", "summary-plans-paid-churned"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("overview plan timeline: %w", err)
	}
	downloadToPaid, err := c.GetAnalyticsCohorts(ctx, AnalyticsCohortsRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"cohort-download-to-paid-rate"},
		Periods:          []string{"d1", "d7", "d35"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("overview download to paid: %w", err)
	}
	retentionStart, retentionEnd, err := overviewRetentionRange(endDate)
	if err != nil {
		return nil, fmt.Errorf("overview retention range: %w", err)
	}
	retention, err := c.GetAnalyticsRetention(ctx, AnalyticsRetentionRequest{
		AppID:            appID,
		StartDate:        retentionStart,
		EndDate:          retentionEnd,
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("overview retention: %w", err)
	}

	eventBreakdown, err := c.getAnalyticsBreakdown(ctx, appID, "App Opens by In-App Event", "eventOpens", "inAppEvent", startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("overview app opens breakdown: %w", err)
	}
	campaignBreakdown, err := c.getAnalyticsBreakdown(ctx, appID, "Total Downloads by Campaign", "totalDownloads", "campaignId", startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("overview campaign breakdown: %w", err)
	}
	crashBreakdown, err := c.getAnalyticsBreakdown(ctx, appID, "Crashes by App Version", "crashes", "appVersion", startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("overview crash breakdown: %w", err)
	}

	featureBreakdowns := make([]AnalyticsBreakdown, 0, 2)
	if eventBreakdown != nil {
		featureBreakdowns = append(featureBreakdowns, *eventBreakdown)
	}
	if campaignBreakdown != nil {
		featureBreakdowns = append(featureBreakdowns, *campaignBreakdown)
	}
	appUsageBreakdowns := make([]AnalyticsBreakdown, 0, 1)
	if crashBreakdown != nil {
		appUsageBreakdowns = append(appUsageBreakdowns, *crashBreakdown)
	}

	return &AnalyticsOverview{
		AppID:              strings.TrimSpace(appID),
		StartDate:          strings.TrimSpace(startDate),
		EndDate:            strings.TrimSpace(endDate),
		Acquisition:        acquisition.Results,
		Sales:              sales.Results,
		Subscriptions:      subscriptions.Results,
		PlanTimeline:       timeline.Results,
		DownloadToPaid:     downloadToPaid,
		Retention:          retention,
		FeatureBreakdowns:  featureBreakdowns,
		AppUsageBreakdowns: appUsageBreakdowns,
	}, nil
}

// GetAnalyticsSubscriptionsSummary recreates the subscriptions summary page.
func (c *Client) GetAnalyticsSubscriptionsSummary(ctx context.Context, appID, startDate, endDate string) (*AnalyticsSubscriptionsSummary, error) {
	summary, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"subscription-state-plans-active", "subscription-state-paid"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("subscriptions summary cards: %w", err)
	}
	if mrrStart, mrrEnd, ok, err := monthlyRecurringRevenueRange(startDate, endDate); err != nil {
		return nil, fmt.Errorf("subscriptions monthly recurring revenue range: %w", err)
	} else if ok {
		mrr, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
			AppID:            appID,
			StartDate:        mrrStart,
			EndDate:          mrrEnd,
			Measures:         []string{"revenue-recurring"},
			Frequency:        "month",
			DimensionFilters: []AnalyticsDimensionFilter{},
		})
		if err != nil {
			return nil, fmt.Errorf("subscriptions monthly recurring revenue: %w", err)
		}
		summary.Results = append(summary.Results, mrr.Results...)
	}
	timeline, err := c.GetAnalyticsTimeseries(ctx, AnalyticsTimeseriesRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"summary-plans-paid-net", "summary-plans-paid-starts", "summary-plans-paid-churned"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("subscriptions plan timeline: %w", err)
	}
	activePlansBreakdown, err := c.getAnalyticsBreakdown(ctx, appID, "Active Plans by Subscription", "subscription-state-plans-active", "subscriptionPlanId", endDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("subscriptions active plans breakdown: %w", err)
	}
	retentionStart, retentionEnd, err := subscriptionRetentionWindow(endDate)
	if err != nil {
		return nil, fmt.Errorf("subscriptions retention window: %w", err)
	}
	retention, err := c.GetAnalyticsCohorts(ctx, AnalyticsCohortsRequest{
		AppID:            appID,
		StartTime:        retentionStart,
		EndTime:          retentionEnd,
		Measures:         []string{"cohort-subscription-retention-rate"},
		Periods:          []string{"m1", "m3", "m6", "m12"},
		Frequency:        "month",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("subscriptions retention: %w", err)
	}

	return &AnalyticsSubscriptionsSummary{
		AppID:                     strings.TrimSpace(appID),
		StartDate:                 strings.TrimSpace(startDate),
		EndDate:                   strings.TrimSpace(endDate),
		Summary:                   summary.Results,
		PlanTimeline:              timeline.Results,
		ActivePlansBySubscription: activePlansBreakdown,
		SubscriptionRetention:     retention,
	}, nil
}
