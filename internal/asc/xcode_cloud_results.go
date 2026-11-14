package asc

import (
	"context"
	"errors"
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
	IsRequiredToPass  *bool                       `json:"isRequiredToPass,omitempty"`
}

// CiBuildActionRelationships describes relationships for a CI build action.
type CiBuildActionRelationships struct {
	BuildRun    *CiResourceRelationship  `json:"buildRun,omitempty"`
	Artifacts   *CiRelationshipLinksOnly `json:"artifacts,omitempty"`
	Issues      *CiRelationshipLinksOnly `json:"issues,omitempty"`
	TestResults *CiRelationshipLinksOnly `json:"testResults,omitempty"`
}

// CiBuildActionResource represents a CI build action resource.
type CiBuildActionResource struct {
	Type          ResourceType                `json:"type"`
	ID            string                      `json:"id"`
	Attributes    CiBuildActionAttributes     `json:"attributes"`
	Relationships *CiBuildActionRelationships `json:"relationships,omitempty"`
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

func buildCiIssuesQuery(query *ciIssuesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetCiBuildActions retrieves build actions for a build run.
func (c *Client) GetCiBuildActions(ctx context.Context, buildRunID string, opts ...CiBuildActionsOption) (*CiBuildActionsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiBuildActionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciBuildRuns/%s/actions", buildRunID),
		Query:       buildCiBuildActionsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciBuildActions",
	})
}

// GetCiBuildAction retrieves a build action by ID.
func (c *Client) GetCiBuildAction(ctx context.Context, buildActionID string) (*CiBuildActionResponse, error) {
	return getJSON[CiBuildActionResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciBuildActions/%s", buildActionID)})
}

// GetCiBuildActionBuildRun retrieves the build run for a build action.
func (c *Client) GetCiBuildActionBuildRun(ctx context.Context, buildActionID string) (*CiBuildRunResponse, error) {
	return getJSON[CiBuildRunResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciBuildActions/%s/buildRun", buildActionID)})
}

// GetCiBuildActionArtifacts retrieves artifacts for a build action.
func (c *Client) GetCiBuildActionArtifacts(ctx context.Context, buildActionID string, opts ...CiArtifactsOption) (*CiArtifactsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiArtifactsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciBuildActions/%s/artifacts", buildActionID),
		Query:       buildCiArtifactsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciArtifacts",
	})
}

// GetCiArtifact retrieves a single artifact by ID.
func (c *Client) GetCiArtifact(ctx context.Context, artifactID string) (*CiArtifactResponse, error) {
	return getJSON[CiArtifactResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciArtifacts/%s", artifactID)})
}

// GetCiBuildActionTestResults retrieves test results for a build action.
func (c *Client) GetCiBuildActionTestResults(ctx context.Context, buildActionID string, opts ...CiTestResultsOption) (*CiTestResultsResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiTestResultsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciBuildActions/%s/testResults", buildActionID),
		Query:       buildCiTestResultsQuery(query),
		Next:        query.nextURL,
		NextContext: "ciTestResults",
	})
}

// GetCiTestResult retrieves a single test result by ID.
func (c *Client) GetCiTestResult(ctx context.Context, testResultID string) (*CiTestResultResponse, error) {
	return getJSON[CiTestResultResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciTestResults/%s", testResultID)})
}

// GetCiBuildActionIssues retrieves issues for a build action.
func (c *Client) GetCiBuildActionIssues(ctx context.Context, buildActionID string, opts ...CiIssuesOption) (*CiIssuesResponse, error) {
	query := applyOptions(opts)

	return getJSON[CiIssuesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/ciBuildActions/%s/issues", buildActionID),
		Query:       buildCiIssuesQuery(query),
		Next:        query.nextURL,
		NextContext: "ciIssues",
	})
}

// GetCiIssue retrieves a single issue by ID.
func (c *Client) GetCiIssue(ctx context.Context, issueID string) (*CiIssueResponse, error) {
	return getJSON[CiIssueResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/ciIssues/%s", issueID)})
}

// DownloadCiArtifact downloads an artifact from its download URL.
func (c *Client) DownloadCiArtifact(ctx context.Context, downloadURL string) (*ReportDownload, error) {
	if err := validateCiArtifactDownloadURL(downloadURL); err != nil {
		return nil, fmt.Errorf("ci artifact download: %w", err)
	}

	resp, err := c.doStreamNoAuth(ctx, downloadURL, "application/octet-stream")
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
		// url.Error repeats the whole URL, so the cause is dropped.
		return errors.New("invalid download URL")
	}
	if parsedURL.Scheme != "https" {
		return fmt.Errorf("rejected download URL with insecure scheme %q (expected https)", parsedURL.Scheme)
	}
	host := strings.ToLower(parsedURL.Hostname())
	if isAllowedCiArtifactHost(host) {
		return nil
	}
	if isAllowedAnalyticsCDNHost(host) {
		if !hasSignedQuery(parsedURL.Query()) {
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
