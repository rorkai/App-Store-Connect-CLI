package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// FileLocation describes a file path and line number.
type FileLocation struct {
	Path       string `json:"path,omitempty"`
	LineNumber int    `json:"lineNumber,omitempty"`
}

// CiTestStatus represents the status of a test result.
type CiTestStatus string

const (
	CiTestStatusSuccess         CiTestStatus = "SUCCESS"
	CiTestStatusFailure         CiTestStatus = "FAILURE"
	CiTestStatusMixed           CiTestStatus = "MIXED"
	CiTestStatusSkipped         CiTestStatus = "SKIPPED"
	CiTestStatusExpectedFailure CiTestStatus = "EXPECTED_FAILURE"
)

// CiBuildActionAttributes describes a CI build action resource.
type CiBuildActionAttributes struct {
	Name              string                      `json:"name,omitempty"`
	ActionType        string                      `json:"actionType,omitempty"` // BUILD, ANALYZE, TEST, ARCHIVE
	ExecutionProgress CiBuildRunExecutionProgress `json:"executionProgress,omitempty"`
	CompletionStatus  CiBuildRunCompletionStatus  `json:"completionStatus,omitempty"`
	StartedDate       string                      `json:"startedDate,omitempty"`
	FinishedDate      string                      `json:"finishedDate,omitempty"`
	IssueCounts       *CiIssueCounts              `json:"issueCounts,omitempty"`
}

// CiBuildActionResource represents a CI build action resource.
type CiBuildActionResource struct {
	Type       ResourceType            `json:"type"`
	ID         string                  `json:"id"`
	Attributes CiBuildActionAttributes `json:"attributes"`
}

// CiBuildActionsResponse is the response from CI build actions endpoints.
type CiBuildActionsResponse struct {
	Data  []CiBuildActionResource `json:"data"`
	Links Links                   `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiBuildActionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiBuildActionsResponse) GetData() any {
	return r.Data
}

// CiBuildActionResponse is the response from CI build action detail endpoints.
type CiBuildActionResponse struct {
	Data  CiBuildActionResource `json:"data"`
	Links Links                 `json:"links"`
}

// CiArtifactAttributes describes a CI artifact resource.
type CiArtifactAttributes struct {
	FileType    string `json:"fileType,omitempty"`
	FileName    string `json:"fileName,omitempty"`
	FileSize    int    `json:"fileSize,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// CiArtifactResource represents a CI artifact resource.
type CiArtifactResource struct {
	Type       ResourceType         `json:"type"`
	ID         string               `json:"id"`
	Attributes CiArtifactAttributes `json:"attributes"`
}

// CiArtifactsResponse is the response from CI artifacts endpoints.
type CiArtifactsResponse struct {
	Data  []CiArtifactResource `json:"data"`
	Links Links                `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiArtifactsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiArtifactsResponse) GetData() any {
	return r.Data
}

// CiArtifactResponse is the response from CI artifact detail endpoints.
type CiArtifactResponse struct {
	Data  CiArtifactResource `json:"data"`
	Links Links              `json:"links"`
}

// CiTestDestinationResult describes a destination-specific test result.
type CiTestDestinationResult struct {
	UUID       string       `json:"uuid,omitempty"`
	DeviceName string       `json:"deviceName,omitempty"`
	OSVersion  string       `json:"osVersion,omitempty"`
	Status     CiTestStatus `json:"status,omitempty"`
	Duration   float64      `json:"duration,omitempty"`
}

// CiTestResultAttributes describes a CI test result resource.
type CiTestResultAttributes struct {
	ClassName              string                    `json:"className,omitempty"`
	Name                   string                    `json:"name,omitempty"`
	Status                 CiTestStatus              `json:"status,omitempty"`
	FileSource             *FileLocation             `json:"fileSource,omitempty"`
	Message                string                    `json:"message,omitempty"`
	DestinationTestResults []CiTestDestinationResult `json:"destinationTestResults,omitempty"`
}

// CiTestResultResource represents a CI test result resource.
type CiTestResultResource struct {
	Type       ResourceType           `json:"type"`
	ID         string                 `json:"id"`
	Attributes CiTestResultAttributes `json:"attributes"`
}

// CiTestResultsResponse is the response from CI test results endpoints.
type CiTestResultsResponse struct {
	Data  []CiTestResultResource `json:"data"`
	Links Links                  `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiTestResultsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiTestResultsResponse) GetData() any {
	return r.Data
}

// CiTestResultResponse is the response from CI test result detail endpoints.
type CiTestResultResponse struct {
	Data  CiTestResultResource `json:"data"`
	Links Links                `json:"links"`
}

// CiIssueAttributes describes a CI issue resource.
type CiIssueAttributes struct {
	IssueType  string        `json:"issueType,omitempty"`
	Message    string        `json:"message,omitempty"`
	FileSource *FileLocation `json:"fileSource,omitempty"`
	Category   string        `json:"category,omitempty"`
}

// CiIssueResource represents a CI issue resource.
type CiIssueResource struct {
	Type       ResourceType      `json:"type"`
	ID         string            `json:"id"`
	Attributes CiIssueAttributes `json:"attributes"`
}

// CiIssuesResponse is the response from CI issues endpoints.
type CiIssuesResponse struct {
	Data  []CiIssueResource `json:"data"`
	Links Links             `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiIssuesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiIssuesResponse) GetData() any {
	return r.Data
}

// CiIssueResponse is the response from CI issue detail endpoints.
type CiIssueResponse struct {
	Data  CiIssueResource `json:"data"`
	Links Links           `json:"links"`
}
type ciBuildActionsQuery struct {
	listQuery
}

// CiBuildActionsOption is a functional option for GetCiBuildActions.
type CiBuildActionsOption func(*ciBuildActionsQuery)

// WithCiBuildActionsLimit sets the max number of build actions to return.
func WithCiBuildActionsLimit(limit int) CiBuildActionsOption {
	return func(q *ciBuildActionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiBuildActionsNextURL uses a next page URL directly.
func WithCiBuildActionsNextURL(next string) CiBuildActionsOption {
	return func(q *ciBuildActionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiBuildActionsQuery(query *ciBuildActionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciArtifactsQuery struct {
	listQuery
}

// CiArtifactsOption is a functional option for GetCiBuildActionArtifacts.
type CiArtifactsOption func(*ciArtifactsQuery)

// WithCiArtifactsLimit sets the max number of artifacts to return.
func WithCiArtifactsLimit(limit int) CiArtifactsOption {
	return func(q *ciArtifactsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiArtifactsNextURL uses a next page URL directly.
func WithCiArtifactsNextURL(next string) CiArtifactsOption {
	return func(q *ciArtifactsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiArtifactsQuery(query *ciArtifactsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciTestResultsQuery struct {
	listQuery
}

// CiTestResultsOption is a functional option for GetCiBuildActionTestResults.
type CiTestResultsOption func(*ciTestResultsQuery)

// WithCiTestResultsLimit sets the max number of test results to return.
func WithCiTestResultsLimit(limit int) CiTestResultsOption {
	return func(q *ciTestResultsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiTestResultsNextURL uses a next page URL directly.
func WithCiTestResultsNextURL(next string) CiTestResultsOption {
	return func(q *ciTestResultsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiTestResultsQuery(query *ciTestResultsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciIssuesQuery struct {
	listQuery
}

// CiIssuesOption is a functional option for GetCiBuildActionIssues.
type CiIssuesOption func(*ciIssuesQuery)

// WithCiIssuesLimit sets the max number of issues to return.
func WithCiIssuesLimit(limit int) CiIssuesOption {
	return func(q *ciIssuesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiIssuesNextURL uses a next page URL directly.
func WithCiIssuesNextURL(next string) CiIssuesOption {
	return func(q *ciIssuesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiIssuesQuery(query *ciIssuesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetCiBuildActions retrieves build actions for a build run.
func (c *Client) GetCiBuildActions(ctx context.Context, buildRunID string, opts ...CiBuildActionsOption) (*CiBuildActionsResponse, error) {
	query := &ciBuildActionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildRuns/%s/actions", buildRunID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciBuildActions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiBuildActionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiBuildActionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildAction retrieves a build action by ID.
func (c *Client) GetCiBuildAction(ctx context.Context, buildActionID string) (*CiBuildActionResponse, error) {
	path := fmt.Sprintf("/v1/ciBuildActions/%s", buildActionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiBuildActionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildActionBuildRun retrieves the build run for a build action.
func (c *Client) GetCiBuildActionBuildRun(ctx context.Context, buildActionID string) (*CiBuildRunResponse, error) {
	path := fmt.Sprintf("/v1/ciBuildActions/%s/buildRun", buildActionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiBuildRunResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildActionArtifacts retrieves artifacts for a build action.
func (c *Client) GetCiBuildActionArtifacts(ctx context.Context, buildActionID string, opts ...CiArtifactsOption) (*CiArtifactsResponse, error) {
	query := &ciArtifactsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildActions/%s/artifacts", buildActionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciArtifacts: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiArtifactsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiArtifactsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiArtifact retrieves a single artifact by ID.
func (c *Client) GetCiArtifact(ctx context.Context, artifactID string) (*CiArtifactResponse, error) {
	path := fmt.Sprintf("/v1/ciArtifacts/%s", artifactID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiArtifactResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildActionTestResults retrieves test results for a build action.
func (c *Client) GetCiBuildActionTestResults(ctx context.Context, buildActionID string, opts ...CiTestResultsOption) (*CiTestResultsResponse, error) {
	query := &ciTestResultsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildActions/%s/testResults", buildActionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciTestResults: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiTestResultsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiTestResultsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiTestResult retrieves a single test result by ID.
func (c *Client) GetCiTestResult(ctx context.Context, testResultID string) (*CiTestResultResponse, error) {
	path := fmt.Sprintf("/v1/ciTestResults/%s", testResultID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiTestResultResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildActionIssues retrieves issues for a build action.
func (c *Client) GetCiBuildActionIssues(ctx context.Context, buildActionID string, opts ...CiIssuesOption) (*CiIssuesResponse, error) {
	query := &ciIssuesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildActions/%s/issues", buildActionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciIssues: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiIssuesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiIssuesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiIssue retrieves a single issue by ID.
func (c *Client) GetCiIssue(ctx context.Context, issueID string) (*CiIssueResponse, error) {
	path := fmt.Sprintf("/v1/ciIssues/%s", issueID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiIssueResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DownloadCiArtifact downloads an artifact from its download URL.
func (c *Client) DownloadCiArtifact(ctx context.Context, downloadURL string) (*ReportDownload, error) {
	if err := validateCiArtifactDownloadURL(downloadURL); err != nil {
		return nil, fmt.Errorf("ci artifact download: %w", err)
	}

	resp, err := c.doStreamNoAuth(ctx, "GET", downloadURL, "application/octet-stream")
	if err != nil {
		return nil, err
	}

	return &ReportDownload{Body: resp.Body, ContentLength: resp.ContentLength}, nil
}

func validateCiArtifactDownloadURL(downloadURL string) error {
	if strings.TrimSpace(downloadURL) == "" {
		return fmt.Errorf("empty download URL")
	}
	parsedURL, err := url.Parse(downloadURL)
	if err != nil {
		return fmt.Errorf("invalid download URL: %w", err)
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("rejected download URL with insecure scheme %q (expected https)", parsedURL.Scheme)
	}
	host := strings.ToLower(parsedURL.Hostname())
	if isAllowedCiArtifactHost(host) {
		return nil
	}
	if isAllowedAnalyticsCDNHost(host) {
		if !hasSignedAnalyticsQuery(parsedURL.Query()) {
			return fmt.Errorf("rejected ci artifact download URL from CDN host %q without signed query", parsedURL.Host)
		}
		return nil
	}
	if host == "" {
		return fmt.Errorf("rejected ci artifact download URL with empty host")
	}
	return fmt.Errorf("rejected ci artifact download URL from untrusted host %q", parsedURL.Host)
}

func isAllowedCiArtifactHost(host string) bool {
	if isAllowedAnalyticsHost(host) {
		return true
	}
	return host == "icloud-content.com" || strings.HasSuffix(host, ".icloud-content.com")
}
