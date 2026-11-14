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
