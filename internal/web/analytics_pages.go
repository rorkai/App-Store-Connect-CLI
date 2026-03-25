package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const analyticsV2BaseURL = appStoreBaseURL + "/analytics/api/v2"

// AnalyticsTimeseriesGroup groups a timeseries by a dimension.
type AnalyticsTimeseriesGroup struct {
	Metric    string `json:"metric,omitempty"`
	Dimension string `json:"dimension,omitempty"`
	Rank      string `json:"rank,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// AnalyticsSettingMeasure is a minimal settings/all measure descriptor.
type AnalyticsSettingMeasure struct {
	Key         string `json:"key,omitempty"`
	TitleLocKey string `json:"titleLocKey,omitempty"`
}

// AnalyticsSettingDimension is a minimal settings/all dimension descriptor.
type AnalyticsSettingDimension struct {
	Key         string `json:"key,omitempty"`
	TitleLocKey string `json:"titleLocKey,omitempty"`
}

// AnalyticsSettingsConfiguration stores global analytics configuration dates.
type AnalyticsSettingsConfiguration struct {
	ItcBaseURL         string `json:"itcBaseUrl,omitempty"`
	DataStartDate      string `json:"dataStartDate,omitempty"`
	DataEndDate        string `json:"dataEndDate,omitempty"`
	BenchmarkStartDate string `json:"benchmarkStartDate,omitempty"`
	BenchmarkEndDate   string `json:"benchmarkEndDate,omitempty"`
	ImageServiceURL    string `json:"imageServiceUrl,omitempty"`
	GlobalOptInRate    int    `json:"globalOptInRate,omitempty"`
}

// AnalyticsSettingsResponse is the shared analytics settings payload.
type AnalyticsSettingsResponse struct {
	Measures        []AnalyticsSettingMeasure      `json:"measures,omitempty"`
	Dimensions      []AnalyticsSettingDimension    `json:"dimensions,omitempty"`
	EnabledFeatures []string                       `json:"enabledFeatures,omitempty"`
	Configuration   AnalyticsSettingsConfiguration `json:"configuration,omitempty"`
}

// AnalyticsAppFeature is a feature flag on analytics app metadata.
type AnalyticsAppFeature struct {
	ID    string `json:"id,omitempty"`
	Count int    `json:"count,omitempty"`
}

// AnalyticsAppAvailability captures analytics app availability dates.
type AnalyticsAppAvailability struct {
	OrderableAt    string `json:"orderableAt,omitempty"`
	DownloadableAt string `json:"downloadableAt,omitempty"`
}

// AnalyticsAppInfoResult is the app metadata returned by analytics app-info.
type AnalyticsAppInfoResult struct {
	Name         string                   `json:"name,omitempty"`
	AdamID       string                   `json:"adamId,omitempty"`
	IsEnabled    bool                     `json:"isEnabled,omitempty"`
	IsBundle     bool                     `json:"isBundle,omitempty"`
	IsArcade     bool                     `json:"isArcade,omitempty"`
	HasAppClips  bool                     `json:"hasAppClips,omitempty"`
	BundleID     string                   `json:"bundleId,omitempty"`
	Platforms    []string                 `json:"platforms,omitempty"`
	Devices      []string                 `json:"devices,omitempty"`
	Features     []AnalyticsAppFeature    `json:"features,omitempty"`
	Availability AnalyticsAppAvailability `json:"availability,omitempty"`
}

// AnalyticsAppInfoResponse wraps app-info results.
type AnalyticsAppInfoResponse struct {
	Size    int                      `json:"size,omitempty"`
	Results []AnalyticsAppInfoResult `json:"results,omitempty"`
}

// AnalyticsInAppEvent is a single analytics in-app event entry.
type AnalyticsInAppEvent struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Artwork    string `json:"artwork,omitempty"`
	Status     string `json:"status,omitempty"`
	Published  string `json:"published,omitempty"`
	Start      string `json:"start,omitempty"`
	End        string `json:"end,omitempty"`
	Archived   string `json:"archived,omitempty"`
	ValidEvent bool   `json:"validEvent,omitempty"`
}

// AnalyticsInAppEventsResponse wraps analytics in-app event entries.
type AnalyticsInAppEventsResponse struct {
	Results []AnalyticsInAppEvent `json:"results,omitempty"`
	Size    int                   `json:"size,omitempty"`
}

// AnalyticsSourcesListRequest queries the sources/list endpoint.
type AnalyticsSourcesListRequest struct {
	AppID     string
	StartDate string
	EndDate   string
	StartTime string
	EndTime   string
	Measures  []string
	Dimension string
	Frequency string
	Limit     int
}

// AnalyticsSourcesListItem is a single sources/list row.
type AnalyticsSourcesListItem struct {
	SourceID    string             `json:"sourceId,omitempty"`
	SourceTitle string             `json:"sourceTitle,omitempty"`
	Title       string             `json:"title,omitempty"`
	Measures    map[string]float64 `json:"measures,omitempty"`
}

// AnalyticsSourcesListResponse wraps sources/list results.
type AnalyticsSourcesListResponse struct {
	Size           int                        `json:"size,omitempty"`
	Results        []AnalyticsSourcesListItem `json:"results,omitempty"`
	MeetsThreshold bool                       `json:"meetsThreshold,omitempty"`
}

// AnalyticsSourcesPage reproduces the Acquisition > Sources page default view.
type AnalyticsSourcesPage struct {
	AppID          string                       `json:"appId"`
	StartDate      string                       `json:"startDate"`
	EndDate        string                       `json:"endDate"`
	Measure        string                       `json:"measure"`
	GroupDimension string                       `json:"groupDimension"`
	Result         *AnalyticsTimeseriesResponse `json:"result,omitempty"`
}

// AnalyticsInAppEventsPage reproduces the Acquisition > In-App Events page.
type AnalyticsInAppEventsPage struct {
	AppID              string                     `json:"appId"`
	RequestedStartDate string                     `json:"requestedStartDate,omitempty"`
	RequestedEndDate   string                     `json:"requestedEndDate,omitempty"`
	EffectiveStartTime string                     `json:"effectiveStartTime,omitempty"`
	EffectiveEndTime   string                     `json:"effectiveEndTime,omitempty"`
	SelectedEventID    string                     `json:"selectedEventId,omitempty"`
	Events             []AnalyticsInAppEvent      `json:"events,omitempty"`
	SelectedMetrics    *AnalyticsMeasuresResponse `json:"selectedMetrics,omitempty"`
}

// AnalyticsCampaignsPage reproduces the Acquisition > Campaigns page.
type AnalyticsCampaignsPage struct {
	AppID     string                        `json:"appId"`
	StartDate string                        `json:"startDate"`
	EndDate   string                        `json:"endDate"`
	Result    *AnalyticsSourcesListResponse `json:"result,omitempty"`
}

// AnalyticsSalesSummary reproduces the Monetization > Sales page.
type AnalyticsSalesSummary struct {
	AppID               string                    `json:"appId"`
	StartDate           string                    `json:"startDate"`
	EndDate             string                    `json:"endDate"`
	Summary             []AnalyticsMeasureResult  `json:"summary,omitempty"`
	DownloadToPaid      *AnalyticsCohortsResponse `json:"downloadToPaid,omitempty"`
	ProceedsPerDownload *AnalyticsCohortsResponse `json:"proceedsPerDownload,omitempty"`
	RevenueByPurchase   *AnalyticsBreakdown       `json:"revenueByPurchase,omitempty"`
	RevenueByTerritory  *AnalyticsBreakdown       `json:"revenueByTerritory,omitempty"`
}

// AnalyticsBenchmarkPeerGroupWindow is a benchmark peer-group date window.
type AnalyticsBenchmarkPeerGroupWindow struct {
	Start string `json:"start,omitempty"`
	End   string `json:"end,omitempty"`
}

// AnalyticsBenchmarkPeerGroup describes a benchmark peer group.
type AnalyticsBenchmarkPeerGroup struct {
	ID           string                              `json:"id,omitempty"`
	Title        string                              `json:"title,omitempty"`
	Monetization string                              `json:"monetization,omitempty"`
	Category     string                              `json:"category,omitempty"`
	Size         string                              `json:"size,omitempty"`
	Windows      []AnalyticsBenchmarkPeerGroupWindow `json:"windows,omitempty"`
	Availability map[string]bool                     `json:"availability,omitempty"`
	MemberOf     bool                                `json:"memberOf,omitempty"`
	Primary      bool                                `json:"primary,omitempty"`
}

// AnalyticsV2DimensionValuesRequest queries analytics v2 dimension values.
type AnalyticsV2DimensionValuesRequest struct {
	AppID            string
	StartTime        string
	EndTime          string
	Measures         []string
	Frequency        string
	Dimensions       []string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsV2DimensionValuesResult is a v2 dimension-values result row.
type AnalyticsV2DimensionValuesResult struct {
	AdamID     string                        `json:"adamId,omitempty"`
	Dimension  string                        `json:"dimension,omitempty"`
	ActualSize int                           `json:"actualSize,omitempty"`
	Values     []AnalyticsBenchmarkPeerGroup `json:"values,omitempty"`
}

// AnalyticsV2DimensionValuesResponse wraps v2 dimension-values results.
type AnalyticsV2DimensionValuesResponse struct {
	Size    int                                `json:"size,omitempty"`
	Results []AnalyticsV2DimensionValuesResult `json:"results,omitempty"`
}

// AnalyticsV2TimeSeriesRequest queries analytics v2 time-series data.
type AnalyticsV2TimeSeriesRequest struct {
	AppID            string
	StartTime        string
	EndTime          string
	Measures         []string
	Frequency        string
	DimensionFilters []AnalyticsDimensionFilter
}

// AnalyticsV2TimeSeriesResult is a v2 time-series result row.
type AnalyticsV2TimeSeriesResult struct {
	AdamID         string           `json:"adamId,omitempty"`
	Group          any              `json:"group,omitempty"`
	Data           []map[string]any `json:"data,omitempty"`
	Totals         any              `json:"totals,omitempty"`
	MeetsThreshold map[string]bool  `json:"meetsThreshold,omitempty"`
}

// AnalyticsV2TimeSeriesResponse wraps v2 time-series results.
type AnalyticsV2TimeSeriesResponse struct {
	Size    int                           `json:"size,omitempty"`
	Results []AnalyticsV2TimeSeriesResult `json:"results,omitempty"`
}

// AnalyticsBenchmarkMetric is a merged app-vs-percentiles benchmark card.
type AnalyticsBenchmarkMetric struct {
	Key      string   `json:"key,omitempty"`
	Label    string   `json:"label,omitempty"`
	AppValue *float64 `json:"appValue,omitempty"`
	P25      *float64 `json:"p25,omitempty"`
	P50      *float64 `json:"p50,omitempty"`
	P75      *float64 `json:"p75,omitempty"`
}

// AnalyticsBenchmarksSummary reproduces the Benchmarks dashboard summary cards.
type AnalyticsBenchmarksSummary struct {
	AppID          string                        `json:"appId"`
	Category       string                        `json:"category,omitempty"`
	WeekStart      string                        `json:"weekStart,omitempty"`
	WeekEnd        string                        `json:"weekEnd,omitempty"`
	PeerGroupIDs   []string                      `json:"peerGroupIds,omitempty"`
	SelectedGroups []AnalyticsBenchmarkPeerGroup `json:"selectedGroups,omitempty"`
	Metrics        []AnalyticsBenchmarkMetric    `json:"metrics,omitempty"`
}

func (c *Client) doAnalyticsGetRequest(ctx context.Context, path, referer string) ([]byte, error) {
	return c.doRequestBase(ctx, c.analyticsBaseURL(), http.MethodGet, path, nil, analyticsHeaders(referer))
}

func (c *Client) analyticsV2BaseURL() string {
	baseURL := strings.TrimSpace(c.baseURL)
	if baseURL == "" {
		return analyticsV2BaseURL
	}
	if strings.Contains(baseURL, "/analytics/api/v1") {
		return strings.Replace(baseURL, "/analytics/api/v1", "/analytics/api/v2", 1)
	}
	return baseURL
}

func (c *Client) doAnalyticsV2Request(ctx context.Context, path string, body any, referer string) ([]byte, error) {
	return c.doRequestBase(ctx, c.analyticsV2BaseURL(), http.MethodPost, path, body, analyticsHeaders(referer))
}

// GetAnalyticsSettings loads the shared analytics settings payload.
func (c *Client) GetAnalyticsSettings(ctx context.Context) (*AnalyticsSettingsResponse, error) {
	body, err := c.doAnalyticsGetRequest(ctx, "/settings/all", appStoreBaseURL+"/apps")
	if err != nil {
		return nil, err
	}
	var result AnalyticsSettingsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics settings response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsAppInfo loads app metadata used by several analytics tabs.
func (c *Client) GetAnalyticsAppInfo(ctx context.Context, appID string) (*AnalyticsAppInfoResult, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	body, err := c.doAnalyticsGetRequest(ctx, "/app-info/"+appID, analyticsAppReferer(appID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsAppInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics app-info response: %w", err)
	}
	if len(result.Results) == 0 {
		return nil, fmt.Errorf("analytics app-info returned no results for app %s", appID)
	}
	return &result.Results[0], nil
}

// GetAnalyticsInAppEvents loads the In-App Events list for an app.
func (c *Client) GetAnalyticsInAppEvents(ctx context.Context, appID string) (*AnalyticsInAppEventsResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("app id is required")
	}
	body, err := c.doAnalyticsGetRequest(ctx, "/app-info/"+appID+"/in-app-events", analyticsAppReferer(appID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsInAppEventsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics in-app-events response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsSourcesList loads ranked analytics sources or campaigns.
func (c *Client) GetAnalyticsSourcesList(ctx context.Context, req AnalyticsSourcesListRequest) (*AnalyticsSourcesListResponse, error) {
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
	dimension := strings.TrimSpace(req.Dimension)
	if dimension == "" {
		return nil, fmt.Errorf("dimension is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	payload := map[string]any{
		"adamId":    []string{strings.TrimSpace(req.AppID)},
		"startTime": startTime,
		"endTime":   endTime,
		"measures":  measures,
		"dimension": dimension,
		"frequency": frequency,
		"limit":     limit,
	}
	body, err := c.doAnalyticsRequest(ctx, "/data/sources/list", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsSourcesListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics sources list response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsV2DimensionValues loads benchmark peer groups from the v2 API.
func (c *Client) GetAnalyticsV2DimensionValues(ctx context.Context, req AnalyticsV2DimensionValuesRequest) (*AnalyticsV2DimensionValuesResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	measures, err := normalizeStringList(req.Measures, "measures")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}
	startTime := strings.TrimSpace(req.StartTime)
	endTime := strings.TrimSpace(req.EndTime)
	if startTime == "" || endTime == "" {
		return nil, fmt.Errorf("startTime and endTime are required")
	}
	payload := map[string]any{
		"adamId":     []string{strings.TrimSpace(req.AppID)},
		"startTime":  startTime,
		"endTime":    endTime,
		"measures":   measures,
		"frequency":  frequency,
		"dimensions": req.Dimensions,
	}
	if req.DimensionFilters != nil {
		payload["dimensionFilters"] = req.DimensionFilters
	}
	body, err := c.doAnalyticsV2Request(ctx, "/data/dimension-values", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsV2DimensionValuesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics v2 dimension-values response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsV2TimeSeries loads benchmark week values from the v2 API.
func (c *Client) GetAnalyticsV2TimeSeries(ctx context.Context, req AnalyticsV2TimeSeriesRequest) (*AnalyticsV2TimeSeriesResponse, error) {
	if strings.TrimSpace(req.AppID) == "" {
		return nil, fmt.Errorf("app id is required")
	}
	measures, err := normalizeStringList(req.Measures, "measures")
	if err != nil {
		return nil, err
	}
	frequency, err := NormalizeAnalyticsFrequency(req.Frequency)
	if err != nil {
		return nil, err
	}
	startTime := strings.TrimSpace(req.StartTime)
	endTime := strings.TrimSpace(req.EndTime)
	if startTime == "" || endTime == "" {
		return nil, fmt.Errorf("startTime and endTime are required")
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
	body, err := c.doAnalyticsV2Request(ctx, "/data/time-series", payload, analyticsAppReferer(req.AppID))
	if err != nil {
		return nil, err
	}
	var result AnalyticsV2TimeSeriesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analytics v2 time-series response: %w", err)
	}
	return &result, nil
}

// GetAnalyticsSourcesPage reproduces the Acquisition > Sources page default view.
func (c *Client) GetAnalyticsSourcesPage(ctx context.Context, appID, startDate, endDate string) (*AnalyticsSourcesPage, error) {
	result, err := c.GetAnalyticsTimeseries(ctx, AnalyticsTimeseriesRequest{
		AppID:     appID,
		StartDate: startDate,
		EndDate:   endDate,
		Measures:  []string{"pageViewUnique"},
		Frequency: "day",
		Group: &AnalyticsTimeseriesGroup{
			Metric:    "pageViewUnique",
			Dimension: "source",
			Rank:      "DESCENDING",
			Limit:     10,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("sources page: %w", err)
	}
	return &AnalyticsSourcesPage{
		AppID:          strings.TrimSpace(appID),
		StartDate:      strings.TrimSpace(startDate),
		EndDate:        strings.TrimSpace(endDate),
		Measure:        "pageViewUnique",
		GroupDimension: "source",
		Result:         result,
	}, nil
}

// GetAnalyticsInAppEventsPage reproduces the Acquisition > In-App Events page.
func (c *Client) GetAnalyticsInAppEventsPage(ctx context.Context, appID, startDate, endDate string) (*AnalyticsInAppEventsPage, error) {
	settings, err := c.GetAnalyticsSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("in-app-events settings: %w", err)
	}
	events, err := c.GetAnalyticsInAppEvents(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("in-app-events list: %w", err)
	}
	page := &AnalyticsInAppEventsPage{
		AppID:              strings.TrimSpace(appID),
		RequestedStartDate: strings.TrimSpace(startDate),
		RequestedEndDate:   strings.TrimSpace(endDate),
		Events:             events.Results,
	}
	page.EffectiveStartTime, page.EffectiveEndTime, err = analyticsClampedTimeRange(
		startDate,
		endDate,
		settings.Configuration.DataStartDate,
		settings.Configuration.DataEndDate,
	)
	if err != nil {
		return nil, fmt.Errorf("in-app-events range: %w", err)
	}
	if len(events.Results) == 0 {
		return page, nil
	}
	selected := events.Results[0]
	page.SelectedEventID = selected.ID
	metrics, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:     appID,
		StartTime: page.EffectiveStartTime,
		EndTime:   page.EffectiveEndTime,
		Measures:  []string{"eventImpressions", "totalDownloads", "eventOpens"},
		Frequency: "day",
		DimensionFilters: []AnalyticsDimensionFilter{
			{
				"dimensionKey": "inAppEvent",
				"optionKeys":   []string{selected.ID},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("in-app-events metrics: %w", err)
	}
	page.SelectedMetrics = metrics
	return page, nil
}

func analyticsClampedTimeRange(startDate, endDate, minTime, maxTime string) (string, string, error) {
	startTime, endTime, err := analyticsTimeRange(startDate, endDate, "", "")
	if err != nil {
		return "", "", err
	}
	start, _ := time.Parse(time.RFC3339, startTime)
	end, _ := time.Parse(time.RFC3339, endTime)
	if parsedMin, ok, err := analyticsOptionalRFC3339(minTime); err != nil {
		return "", "", fmt.Errorf("invalid minimum time: %w", err)
	} else if ok && start.Before(parsedMin) {
		start = parsedMin
	}
	if parsedMax, ok, err := analyticsOptionalRFC3339(maxTime); err != nil {
		return "", "", fmt.Errorf("invalid maximum time: %w", err)
	} else if ok && end.After(parsedMax) {
		end = parsedMax
	}
	if end.Before(start) {
		return "", "", fmt.Errorf("requested range is outside the available analytics window")
	}
	return start.Format(time.RFC3339), end.Format(time.RFC3339), nil
}

func analyticsOptionalRFC3339(value string) (time.Time, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, err
	}
	return parsed, true, nil
}

// GetAnalyticsCampaignsPage reproduces the Acquisition > Campaigns page.
func (c *Client) GetAnalyticsCampaignsPage(ctx context.Context, appID, startDate, endDate string) (*AnalyticsCampaignsPage, error) {
	result, err := c.GetAnalyticsSourcesList(ctx, AnalyticsSourcesListRequest{
		AppID:     appID,
		StartDate: startDate,
		EndDate:   endDate,
		Measures:  []string{"impressionsTotal", "totalDownloads", "proceeds", "sessions"},
		Dimension: "campaignId",
		Frequency: "day",
	})
	if err != nil {
		return nil, fmt.Errorf("campaigns page: %w", err)
	}
	return &AnalyticsCampaignsPage{
		AppID:     strings.TrimSpace(appID),
		StartDate: strings.TrimSpace(startDate),
		EndDate:   strings.TrimSpace(endDate),
		Result:    result,
	}, nil
}

// GetAnalyticsSalesSummary reproduces the Monetization > Sales summary page.
func (c *Client) GetAnalyticsSalesSummary(ctx context.Context, appID, startDate, endDate string) (*AnalyticsSalesSummary, error) {
	summary, err := c.GetAnalyticsMeasures(ctx, AnalyticsMeasuresRequest{
		AppID:     appID,
		StartDate: startDate,
		EndDate:   endDate,
		Measures:  []string{"proceeds", "payingUsers", "iap"},
		Frequency: "day",
	})
	if err != nil {
		return nil, fmt.Errorf("sales summary cards: %w", err)
	}
	downloadToPaid, err := c.GetAnalyticsCohorts(ctx, AnalyticsCohortsRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"cohort-download-to-paid-rate"},
		Periods:          []string{"d1", "d7", "d14", "d35", "d60"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("sales download to paid: %w", err)
	}
	proceedsPerDownload, err := c.GetAnalyticsCohorts(ctx, AnalyticsCohortsRequest{
		AppID:            appID,
		StartDate:        startDate,
		EndDate:          endDate,
		Measures:         []string{"cohort-download-proceeds-per-download-average"},
		Periods:          []string{"d1", "d7", "d14", "d35", "d60"},
		Frequency:        "day",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("sales proceeds per download: %w", err)
	}
	revenueByPurchase, err := c.getAnalyticsBreakdown(ctx, appID, "Proceeds by In-App Purchases", "proceeds", "inAppPurchase", startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("sales revenue by purchase: %w", err)
	}
	revenueByTerritory, err := c.getAnalyticsBreakdown(ctx, appID, "Proceeds by Territory", "proceeds", "storefront", startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("sales revenue by territory: %w", err)
	}
	return &AnalyticsSalesSummary{
		AppID:               strings.TrimSpace(appID),
		StartDate:           strings.TrimSpace(startDate),
		EndDate:             strings.TrimSpace(endDate),
		Summary:             summary.Results,
		DownloadToPaid:      downloadToPaid,
		ProceedsPerDownload: proceedsPerDownload,
		RevenueByPurchase:   revenueByPurchase,
		RevenueByTerritory:  revenueByTerritory,
	}, nil
}

var analyticsBenchmarkMappings = []struct {
	AppKey   string
	BenchKey string
	Label    string
}{
	{AppKey: "conversionRate", BenchKey: "benchConversionRate", Label: "Conversion Rate"},
	{AppKey: "arppu", BenchKey: "benchArppu", Label: "Proceeds per Paying User"},
	{AppKey: "crashRate", BenchKey: "benchCrashRate", Label: "Crash Rate"},
	{AppKey: "retentionD1", BenchKey: "benchRetentionD1", Label: "Day 1 Retention"},
	{AppKey: "retentionD7", BenchKey: "benchRetentionD7", Label: "Day 7 Retention"},
	{AppKey: "retentionD28", BenchKey: "benchRetentionD28", Label: "Day 28 Retention"},
	{AppKey: "proceeds-per-download-average-d35", BenchKey: "proceeds-per-download-average-d35-benchmark", Label: "D35 Proceeds per Download"},
	{AppKey: "download-to-paid-rate-d35", BenchKey: "download-to-paid-rate-d35-benchmark", Label: "D35 Download to Paid Conversion"},
}

func analyticsMustRFC3339(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("time is required")
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}

type analyticsBenchmarkWeekWindow struct {
	StartTime string
	StartDate string
	EndDate   string
}

func analyticsBenchmarkWeekWindowForDisplay(settings *AnalyticsSettingsResponse) (analyticsBenchmarkWeekWindow, error) {
	if settings == nil {
		return analyticsBenchmarkWeekWindow{}, fmt.Errorf("settings are required")
	}
	endRaw, err := analyticsMustRFC3339(settings.Configuration.BenchmarkEndDate)
	if err != nil {
		return analyticsBenchmarkWeekWindow{}, fmt.Errorf("invalid benchmark end date: %w", err)
	}
	endExclusive, _ := time.Parse(time.RFC3339, endRaw)
	start := endExclusive.AddDate(0, 0, -7)
	endInclusive := endExclusive.AddDate(0, 0, -1)
	return analyticsBenchmarkWeekWindow{
		StartTime: start.Format(time.RFC3339),
		StartDate: start.Format("2006-01-02"),
		EndDate:   endInclusive.Format("2006-01-02"),
	}, nil
}

func analyticsFirstPrimaryMemberCategory(groups []AnalyticsBenchmarkPeerGroup) string {
	for _, group := range groups {
		if group.MemberOf && group.Primary && strings.TrimSpace(group.Category) != "" && group.Category != "GENRE_ALL_ITEMS" {
			return group.Category
		}
	}
	for _, group := range groups {
		if group.MemberOf && strings.TrimSpace(group.Category) != "" && group.Category != "GENRE_ALL_ITEMS" {
			return group.Category
		}
	}
	return ""
}

func analyticsBenchmarkPeerGroups(result *AnalyticsV2DimensionValuesResponse) []AnalyticsBenchmarkPeerGroup {
	if result == nil || len(result.Results) == 0 {
		return nil
	}
	return result.Results[0].Values
}

func analyticsSelectedBenchmarkGroups(groups []AnalyticsBenchmarkPeerGroup, category string) []AnalyticsBenchmarkPeerGroup {
	selected := make([]AnalyticsBenchmarkPeerGroup, 0)
	for _, group := range groups {
		if !group.MemberOf || !group.Primary || strings.TrimSpace(group.Size) != "ALL" {
			continue
		}
		if category != "" && strings.TrimSpace(group.Category) != category {
			continue
		}
		selected = append(selected, group)
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].ID < selected[j].ID
	})
	return selected
}

func analyticsFloatPointerFromAny(value any) *float64 {
	switch v := value.(type) {
	case nil:
		return nil
	case float64:
		out := v
		return &out
	case int:
		out := float64(v)
		return &out
	case map[string]any:
		return nil
	default:
		return nil
	}
}

func analyticsPercentilesFromAny(value any) (*float64, *float64, *float64) {
	payload, ok := value.(map[string]any)
	if !ok {
		return nil, nil, nil
	}
	return analyticsFloatPointerFromAny(payload["p25"]), analyticsFloatPointerFromAny(payload["p50"]), analyticsFloatPointerFromAny(payload["p75"])
}

// GetAnalyticsBenchmarks reproduces the Benchmarks summary cards.
func (c *Client) GetAnalyticsBenchmarks(ctx context.Context, appID string) (*AnalyticsBenchmarksSummary, error) {
	settings, err := c.GetAnalyticsSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("benchmarks settings: %w", err)
	}
	appInfo, err := c.GetAnalyticsAppInfo(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("benchmarks app-info: %w", err)
	}
	startTime := strings.TrimSpace(settings.Configuration.DataStartDate)
	endTime := strings.TrimSpace(settings.Configuration.DataEndDate)
	if startTime == "" || endTime == "" {
		return nil, fmt.Errorf("benchmarks settings did not include analytics data range")
	}
	peerGroupResponse, err := c.GetAnalyticsV2DimensionValues(ctx, AnalyticsV2DimensionValuesRequest{
		AppID:     appID,
		StartTime: startTime,
		EndTime:   endTime,
		Measures: []string{
			"benchCrashRate",
			"benchRetentionD1",
			"benchRetentionD7",
			"benchRetentionD28",
			"benchConversionRate",
			"benchArppu",
			"proceeds-per-download-average-d35-benchmark",
			"download-to-paid-rate-d35-benchmark",
		},
		Frequency:        "week",
		Dimensions:       []string{},
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("benchmarks peer groups: %w", err)
	}
	peerGroups := analyticsBenchmarkPeerGroups(peerGroupResponse)
	category := analyticsFirstPrimaryMemberCategory(peerGroups)
	selectedGroups := analyticsSelectedBenchmarkGroups(peerGroups, category)
	if len(selectedGroups) == 0 {
		selectedGroups = analyticsSelectedBenchmarkGroups(peerGroups, "")
	}
	peerGroupIDs := make([]string, 0, len(selectedGroups))
	for _, group := range selectedGroups {
		peerGroupIDs = append(peerGroupIDs, group.ID)
	}

	weekWindow, err := analyticsBenchmarkWeekWindowForDisplay(settings)
	if err != nil {
		return nil, fmt.Errorf("benchmarks week range: %w", err)
	}
	appSeries, err := c.GetAnalyticsV2TimeSeries(ctx, AnalyticsV2TimeSeriesRequest{
		AppID:     appID,
		StartTime: weekWindow.StartTime,
		EndTime:   weekWindow.StartTime,
		Measures: []string{
			"crashRate",
			"retentionD1",
			"retentionD7",
			"retentionD28",
			"conversionRate",
			"arppu",
			"proceeds-per-download-average-d35",
			"download-to-paid-rate-d35",
		},
		Frequency:        "week",
		DimensionFilters: []AnalyticsDimensionFilter{},
	})
	if err != nil {
		return nil, fmt.Errorf("benchmarks app metrics: %w", err)
	}
	benchmarkFilters := []AnalyticsDimensionFilter{}
	if len(peerGroupIDs) > 0 {
		benchmarkFilters = append(benchmarkFilters, AnalyticsDimensionFilter{
			"dimensionKey": "peerGroupId",
			"optionKeys":   peerGroupIDs,
		})
	}
	benchmarkSeries, err := c.GetAnalyticsV2TimeSeries(ctx, AnalyticsV2TimeSeriesRequest{
		AppID:     appID,
		StartTime: weekWindow.StartTime,
		EndTime:   weekWindow.StartTime,
		Measures: []string{
			"benchCrashRate",
			"benchRetentionD1",
			"benchRetentionD7",
			"benchRetentionD28",
			"benchConversionRate",
			"benchArppu",
			"proceeds-per-download-average-d35-benchmark",
			"download-to-paid-rate-d35-benchmark",
		},
		Frequency:        "week",
		DimensionFilters: benchmarkFilters,
	})
	if err != nil {
		return nil, fmt.Errorf("benchmarks peer metrics: %w", err)
	}

	var appPoint map[string]any
	if appSeries != nil && len(appSeries.Results) > 0 && len(appSeries.Results[0].Data) > 0 {
		appPoint = appSeries.Results[0].Data[0]
	}
	var benchmarkPoint map[string]any
	if benchmarkSeries != nil && len(benchmarkSeries.Results) > 0 && len(benchmarkSeries.Results[0].Data) > 0 {
		benchmarkPoint = benchmarkSeries.Results[0].Data[0]
	}

	metrics := make([]AnalyticsBenchmarkMetric, 0, len(analyticsBenchmarkMappings))
	for _, mapping := range analyticsBenchmarkMappings {
		p25, p50, p75 := analyticsPercentilesFromAny(benchmarkPoint[mapping.BenchKey])
		metrics = append(metrics, AnalyticsBenchmarkMetric{
			Key:      mapping.AppKey,
			Label:    mapping.Label,
			AppValue: analyticsFloatPointerFromAny(appPoint[mapping.AppKey]),
			P25:      p25,
			P50:      p50,
			P75:      p75,
		})
	}

	return &AnalyticsBenchmarksSummary{
		AppID:          strings.TrimSpace(appInfo.AdamID),
		Category:       category,
		WeekStart:      weekWindow.StartDate,
		WeekEnd:        weekWindow.EndDate,
		PeerGroupIDs:   peerGroupIDs,
		SelectedGroups: selectedGroups,
		Metrics:        metrics,
	}, nil
}
