package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// CiBuildRunExecutionProgress represents the execution progress of a build run.
type CiBuildRunExecutionProgress string

const (
	CiBuildRunExecutionProgressPending  CiBuildRunExecutionProgress = "PENDING"
	CiBuildRunExecutionProgressRunning  CiBuildRunExecutionProgress = "RUNNING"
	CiBuildRunExecutionProgressComplete CiBuildRunExecutionProgress = "COMPLETE"
)

// CiBuildRunCompletionStatus represents the completion status of a build run.
type CiBuildRunCompletionStatus string

const (
	CiBuildRunCompletionStatusSucceeded CiBuildRunCompletionStatus = "SUCCEEDED"
	CiBuildRunCompletionStatusFailed    CiBuildRunCompletionStatus = "FAILED"
	CiBuildRunCompletionStatusErrored   CiBuildRunCompletionStatus = "ERRORED"
	CiBuildRunCompletionStatusCanceled  CiBuildRunCompletionStatus = "CANCELED"
	CiBuildRunCompletionStatusSkipped   CiBuildRunCompletionStatus = "SKIPPED"
)

// CiBuildRunAttributes describes a CI build run resource.
type CiBuildRunAttributes struct {
	Number             int                         `json:"number,omitempty"`
	CreatedDate        string                      `json:"createdDate,omitempty"`
	StartedDate        string                      `json:"startedDate,omitempty"`
	FinishedDate       string                      `json:"finishedDate,omitempty"`
	SourceCommit       *CiGitRefInfo               `json:"sourceCommit,omitempty"`
	DestinationCommit  *CiGitRefInfo               `json:"destinationCommit,omitempty"`
	IsPullRequestBuild bool                        `json:"isPullRequestBuild,omitempty"`
	IssueCounts        *CiIssueCounts              `json:"issueCounts,omitempty"`
	ExecutionProgress  CiBuildRunExecutionProgress `json:"executionProgress,omitempty"`
	CompletionStatus   CiBuildRunCompletionStatus  `json:"completionStatus,omitempty"`
	StartReason        string                      `json:"startReason,omitempty"`
	CancelReason       string                      `json:"cancelReason,omitempty"`
}

// CiGitRefInfo describes git reference information.
type CiGitRefInfo struct {
	CommitSha string     `json:"commitSha,omitempty"`
	Author    *CiGitUser `json:"author,omitempty"`
	Committer *CiGitUser `json:"committer,omitempty"`
	Message   string     `json:"message,omitempty"`
	WebURL    string     `json:"webUrl,omitempty"`
}

// CiGitUser describes a git user.
type CiGitUser struct {
	DisplayName string `json:"displayName,omitempty"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

// CiIssueCounts describes issue counts.
type CiIssueCounts struct {
	AnalyzerWarnings int `json:"analyzerWarnings,omitempty"`
	Errors           int `json:"errors,omitempty"`
	TestFailures     int `json:"testFailures,omitempty"`
	Warnings         int `json:"warnings,omitempty"`
}

// CiBuildRunRelationships describes relationships for a CI build run.
type CiBuildRunRelationships struct {
	Builds            *RelationshipList `json:"builds,omitempty"`
	Workflow          *Relationship     `json:"workflow,omitempty"`
	Product           *Relationship     `json:"product,omitempty"`
	SourceBranchOrTag *Relationship     `json:"sourceBranchOrTag,omitempty"`
	DestinationBranch *Relationship     `json:"destinationBranch,omitempty"`
	PullRequest       *Relationship     `json:"pullRequest,omitempty"`
}

// CiBuildRunResource represents a CI build run resource.
type CiBuildRunResource struct {
	Type          ResourceType             `json:"type"`
	ID            string                   `json:"id"`
	Attributes    CiBuildRunAttributes     `json:"attributes"`
	Relationships *CiBuildRunRelationships `json:"relationships,omitempty"`
}

// CiBuildRunsResponse is the response from CI build runs endpoints.
type CiBuildRunsResponse struct {
	Data  []CiBuildRunResource `json:"data"`
	Links Links                `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiBuildRunsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiBuildRunsResponse) GetData() any {
	return r.Data
}

// CiBuildRunResponse is the response from CI build run detail/create endpoints.
type CiBuildRunResponse struct {
	Data  CiBuildRunResource `json:"data"`
	Links Links              `json:"links"`
}

// CiBuildRunCreateRequest is a request to create a CI build run.
type CiBuildRunCreateRequest struct {
	Data CiBuildRunCreateData `json:"data"`
}

// CiBuildRunCreateAttributes are optional attributes for creating a CI build run.
type CiBuildRunCreateAttributes struct {
	Clean *bool `json:"clean,omitempty"`
}

// CiBuildRunCreateData is the data portion of a CI build run create request.
type CiBuildRunCreateData struct {
	Type          ResourceType                   `json:"type"`
	Attributes    *CiBuildRunCreateAttributes    `json:"attributes,omitempty"`
	Relationships *CiBuildRunCreateRelationships `json:"relationships,omitempty"`
}

// CiBuildRunCreateRelationships describes relationships for creating a CI build run.
type CiBuildRunCreateRelationships struct {
	BuildRun          *Relationship `json:"buildRun,omitempty"`
	Workflow          *Relationship `json:"workflow,omitempty"`
	SourceBranchOrTag *Relationship `json:"sourceBranchOrTag,omitempty"`
	PullRequest       *Relationship `json:"pullRequest,omitempty"`
}

// Query types for Xcode Cloud endpoints

type ciBuildRunsQuery struct {
	listQuery
	sort string
}

// CiBuildRunsOption is a functional option for GetCiBuildRuns.
type CiBuildRunsOption func(*ciBuildRunsQuery)

type ciBuildRunGetQuery struct {
	include           []string
	fieldsCiBuildRuns []string
}

// CiBuildRunGetOption is a functional option for GetCiBuildRun.
type CiBuildRunGetOption func(*ciBuildRunGetQuery)

// WithCiBuildRunInclude includes related resources in the build run detail response.
func WithCiBuildRunInclude(relationships ...string) CiBuildRunGetOption {
	return func(q *ciBuildRunGetQuery) {
		q.include = normalizeUniqueList(append(q.include, relationships...))
	}
}

// WithCiBuildRunFields limits returned fields for ciBuildRuns resources.
func WithCiBuildRunFields(fields ...string) CiBuildRunGetOption {
	return func(q *ciBuildRunGetQuery) {
		q.fieldsCiBuildRuns = normalizeUniqueList(append(q.fieldsCiBuildRuns, fields...))
	}
}

// WithCiBuildRunsLimit sets the max number of build runs to return.
func WithCiBuildRunsLimit(limit int) CiBuildRunsOption {
	return func(q *ciBuildRunsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiBuildRunsNextURL uses a next page URL directly.
func WithCiBuildRunsNextURL(next string) CiBuildRunsOption {
	return func(q *ciBuildRunsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCiBuildRunsSort sets the sort order for build runs.
func WithCiBuildRunsSort(sort string) CiBuildRunsOption {
	return func(q *ciBuildRunsQuery) {
		if strings.TrimSpace(sort) != "" {
			q.sort = strings.TrimSpace(sort)
		}
	}
}

func buildCiBuildRunsQuery(query *ciBuildRunsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	if query.sort != "" {
		values.Set("sort", query.sort)
	}
	return values.Encode()
}

func buildCiBuildRunGetQuery(query *ciBuildRunGetQuery) string {
	values := url.Values{}
	addCSV(values, "include", query.include)
	addCSV(values, "fields[ciBuildRuns]", query.fieldsCiBuildRuns)
	return values.Encode()
}

type ciBuildRunBuildsQuery struct {
	listQuery
}

// CiBuildRunBuildsOption is a functional option for GetCiBuildRunBuilds.
type CiBuildRunBuildsOption func(*ciBuildRunBuildsQuery)

// WithCiBuildRunBuildsLimit sets the max number of builds to return.
func WithCiBuildRunBuildsLimit(limit int) CiBuildRunBuildsOption {
	return func(q *ciBuildRunBuildsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiBuildRunBuildsNextURL uses a next page URL directly.
func WithCiBuildRunBuildsNextURL(next string) CiBuildRunBuildsOption {
	return func(q *ciBuildRunBuildsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiBuildRunBuildsQuery(query *ciBuildRunBuildsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetCiBuildRuns retrieves build runs for a workflow.
func (c *Client) GetCiBuildRuns(ctx context.Context, workflowID string, opts ...CiBuildRunsOption) (*CiBuildRunsResponse, error) {
	query := &ciBuildRunsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciWorkflows/%s/buildRuns", workflowID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciBuildRuns: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiBuildRunsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiBuildRunsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildRun retrieves a CI build run by ID.
func (c *Client) GetCiBuildRun(ctx context.Context, buildRunID string, opts ...CiBuildRunGetOption) (*CiBuildRunResponse, error) {
	query := &ciBuildRunGetQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildRuns/%s", buildRunID)
	if queryString := buildCiBuildRunGetQuery(query); queryString != "" {
		path += "?" + queryString
	}

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

// GetCiBuildRunBuilds retrieves builds for a CI build run.
func (c *Client) GetCiBuildRunBuilds(ctx context.Context, buildRunID string, opts ...CiBuildRunBuildsOption) (*BuildsResponse, error) {
	query := &ciBuildRunBuildsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciBuildRuns/%s/builds", buildRunID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciBuildRunBuilds: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiBuildRunBuildsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateCiBuildRun creates a new CI build run (triggers a workflow).
func (c *Client) CreateCiBuildRun(ctx context.Context, req CiBuildRunCreateRequest) (*CiBuildRunResponse, error) {
	body, err := BuildRequestBody(req)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/ciBuildRuns", body)
	if err != nil {
		return nil, err
	}

	var response CiBuildRunResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// XcodeCloudRunResult represents the result of triggering a build run.
type XcodeCloudRunResult struct {
	BuildRunID        string `json:"buildRunId"`
	BuildNumber       int    `json:"buildNumber,omitempty"`
	WorkflowID        string `json:"workflowId"`
	WorkflowName      string `json:"workflowName,omitempty"`
	TriggerSource     string `json:"triggerSource,omitempty"`
	GitReferenceID    string `json:"gitReferenceId,omitempty"`
	GitReferenceName  string `json:"gitReferenceName,omitempty"`
	PullRequestID     string `json:"pullRequestId,omitempty"`
	SourceRunID       string `json:"sourceRunId,omitempty"`
	Clean             bool   `json:"clean,omitempty"`
	ExecutionProgress string `json:"executionProgress,omitempty"`
	CompletionStatus  string `json:"completionStatus,omitempty"`
	StartReason       string `json:"startReason,omitempty"`
	CreatedDate       string `json:"createdDate,omitempty"`
	StartedDate       string `json:"startedDate,omitempty"`
	FinishedDate      string `json:"finishedDate,omitempty"`
}

// XcodeCloudStatusResult represents the status of a build run.
type XcodeCloudStatusResult struct {
	BuildRunID        string         `json:"buildRunId"`
	BuildNumber       int            `json:"buildNumber,omitempty"`
	WorkflowID        string         `json:"workflowId,omitempty"`
	ExecutionProgress string         `json:"executionProgress"`
	CompletionStatus  string         `json:"completionStatus,omitempty"`
	StartReason       string         `json:"startReason,omitempty"`
	CancelReason      string         `json:"cancelReason,omitempty"`
	CreatedDate       string         `json:"createdDate,omitempty"`
	StartedDate       string         `json:"startedDate,omitempty"`
	FinishedDate      string         `json:"finishedDate,omitempty"`
	SourceCommit      *CiGitRefInfo  `json:"sourceCommit,omitempty"`
	IssueCounts       *CiIssueCounts `json:"issueCounts,omitempty"`
}

// IsBuildRunComplete returns true if the build run has finished.
func IsBuildRunComplete(progress CiBuildRunExecutionProgress) bool {
	return progress == CiBuildRunExecutionProgressComplete
}

// IsBuildRunSuccessful returns true if the build run completed successfully.
func IsBuildRunSuccessful(status CiBuildRunCompletionStatus) bool {
	return status == CiBuildRunCompletionStatusSucceeded
}
