package asc

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
)

// SalesReportType represents the type of sales report.
type SalesReportType string

const (
	SalesReportTypeSales                           SalesReportType = "SALES"
	SalesReportTypePreOrder                        SalesReportType = "PRE_ORDER"
	SalesReportTypeNewsstand                       SalesReportType = "NEWSSTAND"
	SalesReportTypeSubscription                    SalesReportType = "SUBSCRIPTION"
	SalesReportTypeSubscriptionEvent               SalesReportType = "SUBSCRIPTION_EVENT"
	SalesReportTypeSubscriber                      SalesReportType = "SUBSCRIBER"
	SalesReportTypeSubscriptionOfferCodeRedemption SalesReportType = "SUBSCRIPTION_OFFER_CODE_REDEMPTION"
	SalesReportTypeInstalls                        SalesReportType = "INSTALLS"
	SalesReportTypeFirstAnnual                     SalesReportType = "FIRST_ANNUAL"
	SalesReportTypeWinBackEligibility              SalesReportType = "WIN_BACK_ELIGIBILITY"
)

// SalesReportSubType represents the report detail level.
type SalesReportSubType string

const (
	SalesReportSubTypeSummary            SalesReportSubType = "SUMMARY"
	SalesReportSubTypeDetailed           SalesReportSubType = "DETAILED"
	SalesReportSubTypeSummaryInstallType SalesReportSubType = "SUMMARY_INSTALL_TYPE"
	SalesReportSubTypeSummaryTerritory   SalesReportSubType = "SUMMARY_TERRITORY"
	SalesReportSubTypeSummaryChannel     SalesReportSubType = "SUMMARY_CHANNEL"
)

// SalesReportFrequency represents the reporting frequency.
type SalesReportFrequency string

const (
	SalesReportFrequencyDaily   SalesReportFrequency = "DAILY"
	SalesReportFrequencyWeekly  SalesReportFrequency = "WEEKLY"
	SalesReportFrequencyMonthly SalesReportFrequency = "MONTHLY"
	SalesReportFrequencyYearly  SalesReportFrequency = "YEARLY"
)

// SalesReportVersion represents the report format version.
type SalesReportVersion string

const (
	SalesReportVersion1_0 SalesReportVersion = "1_0"
	SalesReportVersion1_1 SalesReportVersion = "1_1"
	SalesReportVersion1_2 SalesReportVersion = "1_2"
	SalesReportVersion1_3 SalesReportVersion = "1_3"
	SalesReportVersion1_4 SalesReportVersion = "1_4"
)

// AnalyticsAccessType represents analytics report access types.
type AnalyticsAccessType string

const (
	AnalyticsAccessTypeOngoing         AnalyticsAccessType = "ONGOING"
	AnalyticsAccessTypeOneTimeSnapshot AnalyticsAccessType = "ONE_TIME_SNAPSHOT"
)

// SalesReportParams describes sales report query parameters.
type SalesReportParams struct {
	VendorNumber  string
	ReportType    SalesReportType
	ReportSubType SalesReportSubType
	Frequency     SalesReportFrequency
	ReportDate    string
	Version       SalesReportVersion
}

// ReportDownload represents a streaming download response.
type ReportDownload struct {
	Body          io.ReadCloser
	ContentLength int64
}

// AnalyticsReportRequestAttributes describes analytics report request data.
type AnalyticsReportRequestAttributes struct {
	AccessType             AnalyticsAccessType `json:"accessType,omitempty"`
	StoppedDueToInactivity *bool               `json:"stoppedDueToInactivity,omitempty"`
}

// AnalyticsReportRequestRelationships describes request relationships.
type AnalyticsReportRequestRelationships struct {
	App     *Relationship     `json:"app,omitempty"`
	Reports *RelationshipList `json:"reports,omitempty"`
}

// AnalyticsReportRequestResource represents an analytics report request resource.
type AnalyticsReportRequestResource struct {
	Type          ResourceType                         `json:"type"`
	ID            string                               `json:"id"`
	Attributes    AnalyticsReportRequestAttributes     `json:"attributes"`
	Relationships *AnalyticsReportRequestRelationships `json:"relationships,omitempty"`
}

// AnalyticsReportRequestResponse is the response for a single analytics report request.
type AnalyticsReportRequestResponse struct {
	Data  AnalyticsReportRequestResource `json:"data"`
	Links Links                          `json:"links"`
}

// AnalyticsReportRequestsResponse is the response for analytics report requests.
type AnalyticsReportRequestsResponse struct {
	Data  []AnalyticsReportRequestResource `json:"data"`
	Links Links                            `json:"links"`
}

// GetLinks returns the links field for pagination
func (r *AnalyticsReportRequestsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation
func (r *AnalyticsReportRequestsResponse) GetData() any {
	return r.Data
}

// AnalyticsReportRequestCreateAttributes describes attributes for creation.
type AnalyticsReportRequestCreateAttributes struct {
	AccessType AnalyticsAccessType `json:"accessType"`
}

// AnalyticsReportAttributes describes analytics report metadata.
type AnalyticsReportAttributes struct {
	Name        string `json:"name,omitempty"`
	ReportType  string `json:"reportType,omitempty"`
	Category    string `json:"category,omitempty"`
	SubCategory string `json:"subCategory,omitempty"`
	Granularity string `json:"granularity,omitempty"`
}

// AnalyticsReportsResponse is the response from analytics reports endpoint.
type AnalyticsReportsResponse = Response[AnalyticsReportAttributes]

// AnalyticsReportInstanceAttributes describes analytics report instance metadata.
type AnalyticsReportInstanceAttributes struct {
	ReportDate     string `json:"reportDate,omitempty"`
	ProcessingDate string `json:"processingDate,omitempty"`
	Granularity    string `json:"granularity,omitempty"`
	Version        string `json:"version,omitempty"`
}

// AnalyticsReportInstancesResponse is the response from analytics report instances endpoint.
type AnalyticsReportInstancesResponse = Response[AnalyticsReportInstanceAttributes]

// AnalyticsReportSegmentAttributes describes analytics report segment metadata.
type AnalyticsReportSegmentAttributes struct {
	URL               string `json:"url,omitempty"`
	Checksum          string `json:"checksum,omitempty"`
	SizeInBytes       int64  `json:"sizeInBytes,omitempty"`
	URLExpirationDate string `json:"urlExpirationDate,omitempty"`
}

// AnalyticsReportSegmentsResponse is the response from analytics report segments endpoint.
type AnalyticsReportSegmentsResponse = Response[AnalyticsReportSegmentAttributes]

// AnalyticsReportInstancesLinkagesResponse is the response from analytics report instances linkages endpoint.
type AnalyticsReportInstancesLinkagesResponse = LinkagesResponse

// AnalyticsReportInstanceSegmentsLinkagesResponse is the response from analytics report instance segments linkages endpoint.
type AnalyticsReportInstanceSegmentsLinkagesResponse = LinkagesResponse

type analyticsReportRequestsQuery struct {
	listQuery
	accessType AnalyticsAccessType
}

type analyticsReportsQuery struct {
	listQuery
	category string
}

type analyticsReportInstancesQuery struct {
	listQuery
	granularities   []string
	processingDates []string
}

type analyticsReportSegmentsQuery struct {
	listQuery
}

// AnalyticsReportRequestsOption is a functional option for request listings.
type AnalyticsReportRequestsOption func(*analyticsReportRequestsQuery)

// AnalyticsReportsOption is a functional option for report listings.
type AnalyticsReportsOption func(*analyticsReportsQuery)

// AnalyticsReportInstancesOption is a functional option for instance listings.
type AnalyticsReportInstancesOption func(*analyticsReportInstancesQuery)

// AnalyticsReportSegmentsOption is a functional option for segment listings.
type AnalyticsReportSegmentsOption func(*analyticsReportSegmentsQuery)

// WithAnalyticsReportRequestsAccessType filters requests by access type.
func WithAnalyticsReportRequestsAccessType(accessType AnalyticsAccessType) AnalyticsReportRequestsOption {
	return func(q *analyticsReportRequestsQuery) {
		q.accessType = AnalyticsAccessType(strings.TrimSpace(string(accessType)))
	}
}

// WithAnalyticsReportInstancesGranularities filters instances by one or more granularities.
func WithAnalyticsReportInstancesGranularities(granularities []string) AnalyticsReportInstancesOption {
	return func(q *analyticsReportInstancesQuery) {
		q.granularities = normalizeUniqueList(normalizeUpperList(granularities))
	}
}

// WithAnalyticsReportInstancesProcessingDates filters instances by one or more processing dates.
func WithAnalyticsReportInstancesProcessingDates(dates []string) AnalyticsReportInstancesOption {
	return func(q *analyticsReportInstancesQuery) {
		q.processingDates = normalizeUniqueList(dates)
	}
}

func buildSalesReportQuery(params SalesReportParams) string {
	values := url.Values{}
	if strings.TrimSpace(params.VendorNumber) != "" {
		values.Set("filter[vendorNumber]", strings.TrimSpace(params.VendorNumber))
	}
	if params.ReportType != "" {
		values.Set("filter[reportType]", string(params.ReportType))
	}
	if params.ReportSubType != "" {
		values.Set("filter[reportSubType]", string(params.ReportSubType))
	}
	if params.Frequency != "" {
		values.Set("filter[frequency]", string(params.Frequency))
	}
	if strings.TrimSpace(params.ReportDate) != "" {
		values.Set("filter[reportDate]", strings.TrimSpace(params.ReportDate))
	}
	if params.Version != "" {
		values.Set("filter[version]", string(params.Version))
	}
	return values.Encode()
}

func buildAnalyticsReportRequestsQuery(query *analyticsReportRequestsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(string(query.accessType)) != "" {
		values.Set("filter[accessType]", strings.TrimSpace(string(query.accessType)))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAnalyticsReportsQuery(query *analyticsReportsQuery) string {
	values := url.Values{}
	if strings.TrimSpace(query.category) != "" {
		values.Set("filter[category]", strings.TrimSpace(query.category))
	}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAnalyticsReportInstancesQuery(query *analyticsReportInstancesQuery) string {
	values := url.Values{}
	addCSV(values, "filter[granularity]", query.granularities)
	addCSV(values, "filter[processingDate]", query.processingDates)
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAnalyticsReportSegmentsQuery(query *analyticsReportSegmentsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetSalesReport retrieves a sales report as a gzip stream.
func (c *Client) GetSalesReport(ctx context.Context, params SalesReportParams) (*ReportDownload, error) {
	path := "/v1/salesReports"
	if queryString := buildSalesReportQuery(params); queryString != "" {
		path += "?" + queryString
	}

	resp, err := c.doStream(ctx, path, "application/a-gzip")
	if err != nil {
		return nil, err
	}
	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}

// CreateAnalyticsReportRequest creates a new analytics report request.
func (c *Client) CreateAnalyticsReportRequest(ctx context.Context, appID string, accessType AnalyticsAccessType) (*AnalyticsReportRequestResponse, error) {
	payload := AnalyticsReportRequestCreateRequest{
		Data: AnalyticsReportRequestCreateData{
			Type: ResourceTypeAnalyticsReportRequests,
			Attributes: AnalyticsReportRequestCreateAttributes{
				AccessType: accessType,
			},
			Relationships: AnalyticsReportRequestCreateRelationships{
				App: Relationship{
					Data: ResourceData{Type: ResourceTypeApps, ID: appID},
				},
			},
		},
	}

	return sendJSON[AnalyticsReportRequestResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/analyticsReportRequests", Payload: payload})
}

// GetAnalyticsReportRequests retrieves analytics report requests for an app.
func (c *Client) GetAnalyticsReportRequests(ctx context.Context, appID string, opts ...AnalyticsReportRequestsOption) (*AnalyticsReportRequestsResponse, error) {
	query := applyOptions(opts)
	path := "/v1/analyticsReportRequests"
	if strings.TrimSpace(appID) != "" {
		path = fmt.Sprintf("/v1/apps/%s/analyticsReportRequests", strings.TrimSpace(appID))
	}
	return getJSON[AnalyticsReportRequestsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildAnalyticsReportRequestsQuery(query),
		Next:        query.nextURL,
		NextContext: "analyticsReportRequests",
	})
}

// GetAnalyticsReportRequest retrieves a specific analytics report request by ID.
func (c *Client) GetAnalyticsReportRequest(ctx context.Context, requestID string) (*AnalyticsReportRequestResponse, error) {
	path, err := resourcePath("/v1/analyticsReportRequests/%s", requestID)
	if err != nil {
		return nil, fmt.Errorf("analytics report request: %w", err)
	}
	return getJSON[AnalyticsReportRequestResponse](ctx, c, readRequest{Path: path})
}

// DeleteAnalyticsReportRequest deletes an analytics report request by ID.
func (c *Client) DeleteAnalyticsReportRequest(ctx context.Context, requestID string) error {
	path, err := resourcePath("/v1/analyticsReportRequests/%s", requestID)
	if err != nil {
		return fmt.Errorf("analytics report request: %w", err)
	}
	return sendNoContent(ctx, c, "DELETE", path, nil)
}

// GetAnalyticsReport retrieves a specific analytics report by ID.
func (c *Client) GetAnalyticsReport(ctx context.Context, reportID string) (*AnalyticsReportResponse, error) {
	path, err := resourcePath("/v1/analyticsReports/%s", reportID)
	if err != nil {
		return nil, fmt.Errorf("analytics report: %w", err)
	}
	return getJSON[AnalyticsReportResponse](ctx, c, readRequest{Path: path})
}

// GetAnalyticsReports retrieves analytics reports for a request.
func (c *Client) GetAnalyticsReports(ctx context.Context, requestID string, opts ...AnalyticsReportsOption) (*AnalyticsReportsResponse, error) {
	query := applyOptions(opts)

	var path string
	if query.nextURL == "" {
		var err error
		path, err = resourcePath("/v1/analyticsReportRequests/%s/reports", requestID)
		if err != nil {
			return nil, fmt.Errorf("analytics reports: %w", err)
		}
	}
	return getJSON[AnalyticsReportsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildAnalyticsReportsQuery(query),
		Next:        query.nextURL,
		NextContext: "analyticsReports",
	})
}

// GetAnalyticsReportInstancesRelationships retrieves report instance linkages for a report.
func (c *Client) GetAnalyticsReportInstancesRelationships(ctx context.Context, reportID string, opts ...LinkagesOption) (*AnalyticsReportInstancesLinkagesResponse, error) {
	return getTypedResourceLinkages[AnalyticsReportInstancesLinkagesResponse](
		c,
		ctx,
		reportID,
		"instances",
		"report ID",
		"/v1/analyticsReports/%s/relationships/%s",
		"analyticsReportInstancesRelationships",
		opts...,
	)
}

// GetAnalyticsReportInstances retrieves report instances for a report.
func (c *Client) GetAnalyticsReportInstances(ctx context.Context, reportID string, opts ...AnalyticsReportInstancesOption) (*AnalyticsReportInstancesResponse, error) {
	query := applyOptions(opts)

	var path string
	if query.nextURL == "" {
		var err error
		path, err = resourcePath("/v1/analyticsReports/%s/instances", reportID)
		if err != nil {
			return nil, fmt.Errorf("analytics report instances: %w", err)
		}
	}
	return getJSON[AnalyticsReportInstancesResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildAnalyticsReportInstancesQuery(query),
		Next:        query.nextURL,
		NextContext: "analyticsReportInstances",
	})
}

// GetAnalyticsReportInstance retrieves a specific report instance by ID.
func (c *Client) GetAnalyticsReportInstance(ctx context.Context, instanceID string) (*AnalyticsReportInstanceResponse, error) {
	path, err := resourcePath("/v1/analyticsReportInstances/%s", instanceID)
	if err != nil {
		return nil, fmt.Errorf("analytics report instance: %w", err)
	}
	return getJSON[AnalyticsReportInstanceResponse](ctx, c, readRequest{Path: path})
}

// GetAnalyticsReportInstanceSegmentsRelationships retrieves segment linkages for a report instance.
func (c *Client) GetAnalyticsReportInstanceSegmentsRelationships(ctx context.Context, instanceID string, opts ...LinkagesOption) (*AnalyticsReportInstanceSegmentsLinkagesResponse, error) {
	if strings.TrimSpace(instanceID) != "" {
		var err error
		instanceID, err = ValidateResourcePathSegment(instanceID)
		if err != nil {
			return nil, fmt.Errorf("analytics report instance segments relationship: %w", err)
		}
	}
	return getTypedResourceLinkages[AnalyticsReportInstanceSegmentsLinkagesResponse](
		c,
		ctx,
		instanceID,
		"segments",
		"instance ID",
		"/v1/analyticsReportInstances/%s/relationships/%s",
		"analyticsReportInstanceSegmentsRelationships",
		opts...,
	)
}

// GetAnalyticsReportSegments retrieves report segments for an instance.
func (c *Client) GetAnalyticsReportSegments(ctx context.Context, instanceID string, opts ...AnalyticsReportSegmentsOption) (*AnalyticsReportSegmentsResponse, error) {
	query := applyOptions(opts)

	var path string
	if query.nextURL == "" {
		var err error
		path, err = resourcePath("/v1/analyticsReportInstances/%s/segments", instanceID)
		if err != nil {
			return nil, fmt.Errorf("analytics report segments: %w", err)
		}
	}
	return getJSON[AnalyticsReportSegmentsResponse](ctx, c, readRequest{
		Path:        path,
		Query:       buildAnalyticsReportSegmentsQuery(query),
		Next:        query.nextURL,
		NextContext: "analyticsReportSegments",
	})
}

// GetAnalyticsReportSegment retrieves a specific report segment by ID.
func (c *Client) GetAnalyticsReportSegment(ctx context.Context, segmentID string) (*AnalyticsReportSegmentResponse, error) {
	path, err := resourcePath("/v1/analyticsReportSegments/%s", segmentID)
	if err != nil {
		return nil, fmt.Errorf("analytics report segment: %w", err)
	}
	return getJSON[AnalyticsReportSegmentResponse](ctx, c, readRequest{Path: path})
}

// DownloadAnalyticsReport downloads an analytics report from a presigned URL.
func (c *Client) DownloadAnalyticsReport(ctx context.Context, downloadURL string) (*ReportDownload, error) {
	// Validate the download URL to prevent SSRF attacks
	if err := validateAnalyticsDownloadURL(downloadURL); err != nil {
		return nil, fmt.Errorf("analytics download: %w", err)
	}

	resp, err := c.doStreamNoAuth(ctx, downloadURL, "application/a-gzip")
	if err != nil {
		return nil, err
	}
	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}
