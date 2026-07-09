package asc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Xcode Cloud Resource Types
const (
	ResourceTypeCiProducts       ResourceType = "ciProducts"
	ResourceTypeCiWorkflows      ResourceType = "ciWorkflows"
	ResourceTypeCiBuildRuns      ResourceType = "ciBuildRuns"
	ResourceTypeCiBuildActions   ResourceType = "ciBuildActions"
	ResourceTypeCiArtifacts      ResourceType = "ciArtifacts"
	ResourceTypeCiTestResults    ResourceType = "ciTestResults"
	ResourceTypeCiIssues         ResourceType = "ciIssues"
	ResourceTypeCiMacOsVersions  ResourceType = "ciMacOsVersions"
	ResourceTypeCiXcodeVersions  ResourceType = "ciXcodeVersions"
	ResourceTypeScmProviders     ResourceType = "scmProviders"
	ResourceTypeScmRepositories  ResourceType = "scmRepositories"
	ResourceTypeScmGitReferences ResourceType = "scmGitReferences"
	ResourceTypeScmPullRequests  ResourceType = "scmPullRequests"
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

// CiProductAttributes describes a CI product resource.
type CiProductAttributes struct {
	Name        string `json:"name,omitempty"`
	CreatedDate string `json:"createdDate,omitempty"`
	ProductType string `json:"productType,omitempty"`
	BundleID    string `json:"bundleId,omitempty"`
}

// CiProductRelationships describes relationships for a CI product.
type CiProductRelationships struct {
	App                 *Relationship     `json:"app,omitempty"`
	PrimaryRepositories *RelationshipList `json:"primaryRepositories,omitempty"`
}

// CiProductResource represents a CI product resource.
type CiProductResource struct {
	Type          ResourceType            `json:"type"`
	ID            string                  `json:"id"`
	Attributes    CiProductAttributes     `json:"attributes"`
	Relationships *CiProductRelationships `json:"relationships,omitempty"`
}

// CiProductsResponse is the response from CI products endpoints.
type CiProductsResponse struct {
	Data  []CiProductResource `json:"data"`
	Links Links               `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiProductsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiProductsResponse) GetData() any {
	return r.Data
}

// CiProductResponse is the response from CI product detail endpoints.
type CiProductResponse struct {
	Data  CiProductResource `json:"data"`
	Links Links             `json:"links"`
}

// CiWorkflowAttributes describes a CI workflow resource.
type CiWorkflowAttributes struct {
	Name                            string                       `json:"name,omitempty"`
	Description                     string                       `json:"description,omitempty"`
	BranchStartCondition            *CiBranchStartCondition      `json:"branchStartCondition,omitempty"`
	TagStartCondition               *CiTagStartCondition         `json:"tagStartCondition,omitempty"`
	PullRequestStartCondition       *CiPullRequestStartCondition `json:"pullRequestStartCondition,omitempty"`
	ScheduledStartCondition         *CiScheduledStartCondition   `json:"scheduledStartCondition,omitempty"`
	ManualBranchStartCondition      *CiManualStartCondition      `json:"manualBranchStartCondition,omitempty"`
	ManualTagStartCondition         *CiManualStartCondition      `json:"manualTagStartCondition,omitempty"`
	ManualPullRequestStartCondition *CiManualStartCondition      `json:"manualPullRequestStartCondition,omitempty"`
	Actions                         []CiAction                   `json:"actions,omitempty"`
	IsEnabled                       bool                         `json:"isEnabled,omitempty"`
	IsLockedForEditing              bool                         `json:"isLockedForEditing,omitempty"`
	Clean                           bool                         `json:"clean,omitempty"`
	ContainerFilePath               string                       `json:"containerFilePath,omitempty"`
	LastModifiedDate                string                       `json:"lastModifiedDate,omitempty"`
}

// CiAction describes a build, analyze, test, or archive action in a CI workflow.
type CiAction struct {
	Name                      string          `json:"name,omitempty"`
	ActionType                string          `json:"actionType,omitempty"`
	Destination               string          `json:"destination,omitempty"`
	BuildDistributionAudience string          `json:"buildDistributionAudience,omitempty"`
	TestConfiguration         json.RawMessage `json:"testConfiguration,omitempty"`
	Scheme                    string          `json:"scheme,omitempty"`
	Platform                  string          `json:"platform,omitempty"`
	IsRequiredToPass          bool            `json:"isRequiredToPass"`
}

// CiBranchStartCondition describes branch start conditions.
type CiBranchStartCondition struct {
	Source              *CiBranchPatterns      `json:"source,omitempty"`
	FilesAndFoldersRule *CiFilesAndFoldersRule `json:"filesAndFoldersRule,omitempty"`
	AutoCancel          bool                   `json:"autoCancel,omitempty"`
}

// CiTagStartCondition describes tag start conditions.
type CiTagStartCondition struct {
	Source              *CiTagPatterns         `json:"source,omitempty"`
	FilesAndFoldersRule *CiFilesAndFoldersRule `json:"filesAndFoldersRule,omitempty"`
	AutoCancel          bool                   `json:"autoCancel,omitempty"`
}

// CiPullRequestStartCondition describes pull request start conditions.
type CiPullRequestStartCondition struct {
	Source              *CiBranchPatterns      `json:"source,omitempty"`
	Destination         *CiBranchPatterns      `json:"destination,omitempty"`
	FilesAndFoldersRule *CiFilesAndFoldersRule `json:"filesAndFoldersRule,omitempty"`
	AutoCancel          bool                   `json:"autoCancel,omitempty"`
}

// CiScheduledStartCondition describes scheduled start conditions.
type CiScheduledStartCondition struct {
	Source   *CiBranchPatterns `json:"source,omitempty"`
	Schedule *CiSchedule       `json:"schedule,omitempty"`
}

// CiManualStartCondition describes manual start conditions.
type CiManualStartCondition struct {
	Source *CiBranchPatterns `json:"source,omitempty"`
}

// CiBranchPatterns describes branch patterns.
type CiBranchPatterns struct {
	Patterns   []CiStartConditionPattern `json:"patterns,omitempty"`
	IsAllMatch bool                      `json:"isAllMatch,omitempty"`
}

// CiTagPatterns describes tag patterns.
type CiTagPatterns struct {
	Patterns   []CiStartConditionPattern `json:"patterns,omitempty"`
	IsAllMatch bool                      `json:"isAllMatch,omitempty"`
}

// CiStartConditionPattern describes a start condition pattern.
type CiStartConditionPattern struct {
	Pattern  string `json:"pattern,omitempty"`
	IsPrefix bool   `json:"isPrefix,omitempty"`
}

// CiFilesAndFoldersRule describes files and folders rules.
type CiFilesAndFoldersRule struct {
	Mode  string   `json:"mode,omitempty"`
	Paths []string `json:"paths,omitempty"`
}

// CiSchedule describes a CI schedule.
type CiSchedule struct {
	Frequency string   `json:"frequency,omitempty"`
	Days      []string `json:"days,omitempty"`
	Hour      int      `json:"hour,omitempty"`
	Minute    int      `json:"minute,omitempty"`
	Timezone  string   `json:"timezone,omitempty"`
}

// CiWorkflowRelationships describes relationships for a CI workflow.
type CiWorkflowRelationships struct {
	Product      *Relationship                     `json:"product,omitempty"`
	Repository   *CiWorkflowRepositoryRelationship `json:"repository,omitempty"`
	XcodeVersion *Relationship                     `json:"xcodeVersion,omitempty"`
	MacOsVersion *Relationship                     `json:"macOsVersion,omitempty"`
	BuildRuns    *CiWorkflowBuildRunsRelationship  `json:"buildRuns,omitempty"`
}

// CiRelationshipLinks contains JSON:API links for an Xcode Cloud relationship.
type CiRelationshipLinks struct {
	Self    string `json:"self,omitempty"`
	Related string `json:"related,omitempty"`
}

// CiWorkflowRepositoryRelationship describes a workflow's repository relationship.
// Apple may return only links when the repository isn't included in the response.
type CiWorkflowRepositoryRelationship struct {
	Links CiRelationshipLinks `json:"links"`
	Data  *ResourceData       `json:"data,omitempty"`
}

// CiWorkflowBuildRunsRelationship describes a workflow's build-runs relationship.
type CiWorkflowBuildRunsRelationship struct {
	Links CiRelationshipLinks `json:"links"`
}

// CiWorkflowResource represents a CI workflow resource.
type CiWorkflowResource struct {
	Type          ResourceType             `json:"type"`
	ID            string                   `json:"id"`
	Attributes    CiWorkflowAttributes     `json:"attributes"`
	Relationships *CiWorkflowRelationships `json:"relationships,omitempty"`
}

// CiWorkflowsResponse is the response from CI workflows endpoints.
type CiWorkflowsResponse struct {
	Data  []CiWorkflowResource `json:"data"`
	Links Links                `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiWorkflowsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiWorkflowsResponse) GetData() any {
	return r.Data
}

// CiWorkflowResponse is the response from CI workflow detail endpoints.
type CiWorkflowResponse struct {
	Data  CiWorkflowResource `json:"data"`
	Links Links              `json:"links"`
}

// ScmProviderAttributes describes an SCM provider resource.
type ScmProviderAttributes struct {
	ScmProviderType *ScmProviderType `json:"scmProviderType,omitempty"`
	URL             string           `json:"url,omitempty"`
}

// ScmProviderType describes the SCM provider type metadata.
type ScmProviderType struct {
	Kind        string `json:"kind,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	IsOnPremise bool   `json:"isOnPremise,omitempty"`
}

// ScmProviderResource represents an SCM provider resource.
type ScmProviderResource struct {
	Type       ResourceType          `json:"type"`
	ID         string                `json:"id"`
	Attributes ScmProviderAttributes `json:"attributes"`
}

// ScmProvidersResponse is the response from SCM providers endpoints.
type ScmProvidersResponse struct {
	Data  []ScmProviderResource `json:"data"`
	Links Links                 `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *ScmProvidersResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ScmProvidersResponse) GetData() any {
	return r.Data
}

// ScmProviderResponse is the response from SCM provider detail endpoints.
type ScmProviderResponse struct {
	Data  ScmProviderResource `json:"data"`
	Links Links               `json:"links"`
}

// ScmRepositoryAttributes describes an SCM repository resource.
type ScmRepositoryAttributes struct {
	HTTPCloneURL     string `json:"httpCloneUrl,omitempty"`
	SSHCloneURL      string `json:"sshCloneUrl,omitempty"`
	OwnerName        string `json:"ownerName,omitempty"`
	RepositoryName   string `json:"repositoryName,omitempty"`
	LastAccessedDate string `json:"lastAccessedDate,omitempty"`
}

// ScmRepositoryRelationships describes relationships for an SCM repository.
type ScmRepositoryRelationships struct {
	ScmProvider   *Relationship `json:"scmProvider,omitempty"`
	DefaultBranch *Relationship `json:"defaultBranch,omitempty"`
}

// ScmRepositoryResource represents an SCM repository resource.
type ScmRepositoryResource struct {
	Type          ResourceType                `json:"type"`
	ID            string                      `json:"id"`
	Attributes    ScmRepositoryAttributes     `json:"attributes"`
	Relationships *ScmRepositoryRelationships `json:"relationships,omitempty"`
}

// ScmRepositoriesResponse is the response from SCM repositories endpoints.
type ScmRepositoriesResponse struct {
	Data  []ScmRepositoryResource `json:"data"`
	Links Links                   `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *ScmRepositoriesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ScmRepositoriesResponse) GetData() any {
	return r.Data
}

// ScmGitReferenceAttributes describes an SCM git reference resource.
type ScmGitReferenceAttributes struct {
	Name          string `json:"name,omitempty"`
	CanonicalName string `json:"canonicalName,omitempty"`
	IsDeleted     bool   `json:"isDeleted,omitempty"`
	Kind          string `json:"kind,omitempty"` // BRANCH or TAG
}

// ScmGitReferenceRelationships describes relationships for an SCM git reference.
type ScmGitReferenceRelationships struct {
	Repository *Relationship `json:"repository,omitempty"`
}

// ScmGitReferenceResource represents an SCM git reference resource.
type ScmGitReferenceResource struct {
	Type          ResourceType                  `json:"type"`
	ID            string                        `json:"id"`
	Attributes    ScmGitReferenceAttributes     `json:"attributes"`
	Relationships *ScmGitReferenceRelationships `json:"relationships,omitempty"`
}

// ScmGitReferenceResponse is the response from SCM git reference detail endpoints.
type ScmGitReferenceResponse struct {
	Data  ScmGitReferenceResource `json:"data"`
	Links Links                   `json:"links"`
}

// ScmGitReferencesResponse is the response from SCM git references endpoints.
type ScmGitReferencesResponse struct {
	Data  []ScmGitReferenceResource `json:"data"`
	Links Links                     `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *ScmGitReferencesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ScmGitReferencesResponse) GetData() any {
	return r.Data
}

// ScmPullRequestAttributes describes an SCM pull request resource.
type ScmPullRequestAttributes struct {
	Title                      string `json:"title,omitempty"`
	Number                     int    `json:"number,omitempty"`
	WebURL                     string `json:"webUrl,omitempty"`
	SourceRepositoryOwner      string `json:"sourceRepositoryOwner,omitempty"`
	SourceRepositoryName       string `json:"sourceRepositoryName,omitempty"`
	SourceBranchName           string `json:"sourceBranchName,omitempty"`
	DestinationRepositoryOwner string `json:"destinationRepositoryOwner,omitempty"`
	DestinationRepositoryName  string `json:"destinationRepositoryName,omitempty"`
	DestinationBranchName      string `json:"destinationBranchName,omitempty"`
	IsClosed                   bool   `json:"isClosed,omitempty"`
	IsCrossRepository          bool   `json:"isCrossRepository,omitempty"`
}

// ScmPullRequestRelationships describes relationships for an SCM pull request.
type ScmPullRequestRelationships struct {
	Repository *Relationship `json:"repository,omitempty"`
}

// ScmPullRequestResource represents an SCM pull request resource.
type ScmPullRequestResource struct {
	Type          ResourceType                 `json:"type"`
	ID            string                       `json:"id"`
	Attributes    ScmPullRequestAttributes     `json:"attributes"`
	Relationships *ScmPullRequestRelationships `json:"relationships,omitempty"`
}

// ScmPullRequestsResponse is the response from SCM pull request endpoints.
type ScmPullRequestsResponse struct {
	Data  []ScmPullRequestResource `json:"data"`
	Links Links                    `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *ScmPullRequestsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *ScmPullRequestsResponse) GetData() any {
	return r.Data
}

// ScmPullRequestResponse is the response from SCM pull request detail endpoints.
type ScmPullRequestResponse struct {
	Data  ScmPullRequestResource `json:"data"`
	Links Links                  `json:"links"`
}

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

// CiTestDestinationKind represents the kind of test destination.
type CiTestDestinationKind string

const (
	CiTestDestinationKindSimulator CiTestDestinationKind = "SIMULATOR"
	CiTestDestinationKindMac       CiTestDestinationKind = "MAC"
)

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

type ciProductsQuery struct {
	listQuery
	appID string
}

// CiProductsOption is a functional option for GetCiProducts.
type CiProductsOption func(*ciProductsQuery)

// WithCiProductsLimit sets the max number of CI products to return.
func WithCiProductsLimit(limit int) CiProductsOption {
	return func(q *ciProductsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiProductsNextURL uses a next page URL directly.
func WithCiProductsNextURL(next string) CiProductsOption {
	return func(q *ciProductsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithCiProductsAppID filters CI products by app ID.
func WithCiProductsAppID(appID string) CiProductsOption {
	return func(q *ciProductsQuery) {
		if strings.TrimSpace(appID) != "" {
			q.appID = strings.TrimSpace(appID)
		}
	}
}

func buildCiProductsQuery(query *ciProductsQuery) string {
	values := url.Values{}
	if query.appID != "" {
		values.Set("filter[app]", query.appID)
	}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciWorkflowsQuery struct {
	listQuery
}

// CiWorkflowsOption is a functional option for GetCiWorkflows.
type CiWorkflowsOption func(*ciWorkflowsQuery)

// WithCiWorkflowsLimit sets the max number of CI workflows to return.
func WithCiWorkflowsLimit(limit int) CiWorkflowsOption {
	return func(q *ciWorkflowsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiWorkflowsNextURL uses a next page URL directly.
func WithCiWorkflowsNextURL(next string) CiWorkflowsOption {
	return func(q *ciWorkflowsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiWorkflowsQuery(query *ciWorkflowsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type scmProvidersQuery struct {
	listQuery
}

// ScmProvidersOption is a functional option for GetScmProviders.
type ScmProvidersOption func(*scmProvidersQuery)

// WithScmProvidersLimit sets the max number of SCM providers to return.
func WithScmProvidersLimit(limit int) ScmProvidersOption {
	return func(q *scmProvidersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithScmProvidersNextURL uses a next page URL directly.
func WithScmProvidersNextURL(next string) ScmProvidersOption {
	return func(q *scmProvidersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildScmProvidersQuery(query *scmProvidersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type scmRepositoriesQuery struct {
	listQuery
}

// ScmRepositoriesOption is a functional option for GetScmRepositories.
type ScmRepositoriesOption func(*scmRepositoriesQuery)

// WithScmRepositoriesLimit sets the max number of SCM repositories to return.
func WithScmRepositoriesLimit(limit int) ScmRepositoriesOption {
	return func(q *scmRepositoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithScmRepositoriesNextURL uses a next page URL directly.
func WithScmRepositoriesNextURL(next string) ScmRepositoriesOption {
	return func(q *scmRepositoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildScmRepositoriesQuery(query *scmRepositoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type scmGitReferencesQuery struct {
	listQuery
}

// ScmGitReferencesOption is a functional option for GetScmGitReferences.
type ScmGitReferencesOption func(*scmGitReferencesQuery)

// WithScmGitReferencesLimit sets the max number of git references to return.
func WithScmGitReferencesLimit(limit int) ScmGitReferencesOption {
	return func(q *scmGitReferencesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithScmGitReferencesNextURL uses a next page URL directly.
func WithScmGitReferencesNextURL(next string) ScmGitReferencesOption {
	return func(q *scmGitReferencesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildScmGitReferencesQuery(query *scmGitReferencesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type scmPullRequestsQuery struct {
	listQuery
}

// ScmPullRequestsOption is a functional option for GetScmRepositoryPullRequests.
type ScmPullRequestsOption func(*scmPullRequestsQuery)

// WithScmPullRequestsLimit sets the max number of pull requests to return.
func WithScmPullRequestsLimit(limit int) ScmPullRequestsOption {
	return func(q *scmPullRequestsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithScmPullRequestsNextURL uses a next page URL directly.
func WithScmPullRequestsNextURL(next string) ScmPullRequestsOption {
	return func(q *scmPullRequestsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildScmPullRequestsQuery(query *scmPullRequestsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

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

// GetCiProducts retrieves CI products, optionally filtered by app ID.
func (c *Client) GetCiProducts(ctx context.Context, opts ...CiProductsOption) (*CiProductsResponse, error) {
	query := &ciProductsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/ciProducts"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciProducts: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiProductsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiProductsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiProduct retrieves a CI product by ID.
func (c *Client) GetCiProduct(ctx context.Context, productID string) (*CiProductResponse, error) {
	path := fmt.Sprintf("/v1/ciProducts/%s", productID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiProductResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiProductApp retrieves the app for a CI product.
func (c *Client) GetCiProductApp(ctx context.Context, productID string) (*AppResponse, error) {
	path := fmt.Sprintf("/v1/ciProducts/%s/app", productID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiProductBuildRuns retrieves build runs for a CI product.
func (c *Client) GetCiProductBuildRuns(ctx context.Context, productID string, opts ...CiBuildRunsOption) (*CiBuildRunsResponse, error) {
	query := &ciBuildRunsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/buildRuns", productID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciProductBuildRuns: %w", err)
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

// GetCiProductPrimaryRepositories retrieves primary repositories for a CI product.
func (c *Client) GetCiProductPrimaryRepositories(ctx context.Context, productID string, opts ...CiProductRepositoriesOption) (*ScmRepositoriesResponse, error) {
	query := &ciProductRepositoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/primaryRepositories", productID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciProductPrimaryRepositories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiProductRepositoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmRepositoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiProductAdditionalRepositories retrieves additional repositories for a CI product.
func (c *Client) GetCiProductAdditionalRepositories(ctx context.Context, productID string, opts ...CiProductRepositoriesOption) (*ScmRepositoriesResponse, error) {
	query := &ciProductRepositoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/additionalRepositories", productID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciProductAdditionalRepositories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiProductRepositoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmRepositoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteCiProduct deletes a CI product by ID.
func (c *Client) DeleteCiProduct(ctx context.Context, productID string) error {
	productID = strings.TrimSpace(productID)
	path := fmt.Sprintf("/v1/ciProducts/%s", productID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetCiWorkflows retrieves CI workflows for a product.
func (c *Client) GetCiWorkflows(ctx context.Context, productID string, opts ...CiWorkflowsOption) (*CiWorkflowsResponse, error) {
	query := &ciWorkflowsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/workflows", productID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciWorkflows: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiWorkflowsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiWorkflowsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiWorkflow retrieves a CI workflow by ID.
func (c *Client) GetCiWorkflow(ctx context.Context, workflowID string) (*CiWorkflowResponse, error) {
	path := fmt.Sprintf("/v1/ciWorkflows/%s", workflowID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiWorkflowResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateCiWorkflow creates a CI workflow from a JSON payload.
func (c *Client) CreateCiWorkflow(ctx context.Context, payload json.RawMessage) (*CiWorkflowResponse, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, fmt.Errorf("empty workflow payload")
	}
	data, err := c.do(ctx, "POST", "/v1/ciWorkflows", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var response CiWorkflowResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateCiWorkflow updates a CI workflow using a JSON payload.
func (c *Client) UpdateCiWorkflow(ctx context.Context, workflowID string, payload json.RawMessage) (*CiWorkflowResponse, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, fmt.Errorf("empty workflow payload")
	}
	path := fmt.Sprintf("/v1/ciWorkflows/%s", workflowID)
	data, err := c.do(ctx, "PATCH", path, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var response CiWorkflowResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteCiWorkflow deletes a CI workflow by ID.
func (c *Client) DeleteCiWorkflow(ctx context.Context, workflowID string) error {
	workflowID = strings.TrimSpace(workflowID)
	path := fmt.Sprintf("/v1/ciWorkflows/%s", workflowID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetCiWorkflowRepository retrieves the repository for a CI workflow.
func (c *Client) GetCiWorkflowRepository(ctx context.Context, workflowID string) (*ScmRepositoryResource, error) {
	path := fmt.Sprintf("/v1/ciWorkflows/%s/repository", workflowID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data ScmRepositoryResource `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.Data, nil
}

// GetScmProviders retrieves SCM providers.
func (c *Client) GetScmProviders(ctx context.Context, opts ...ScmProvidersOption) (*ScmProvidersResponse, error) {
	query := &scmProvidersQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/scmProviders"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmProviders: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildScmProvidersQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmProvidersResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmProvider retrieves an SCM provider by ID.
func (c *Client) GetScmProvider(ctx context.Context, providerID string) (*ScmProviderResponse, error) {
	path := fmt.Sprintf("/v1/scmProviders/%s", strings.TrimSpace(providerID))
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmProviderResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmProviderRepositories retrieves repositories for an SCM provider.
func (c *Client) GetScmProviderRepositories(ctx context.Context, providerID string, opts ...ScmRepositoriesOption) (*ScmRepositoriesResponse, error) {
	query := &scmRepositoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/scmProviders/%s/repositories", strings.TrimSpace(providerID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmProviderRepositories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildScmRepositoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmRepositoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmRepositories retrieves SCM repositories.
func (c *Client) GetScmRepositories(ctx context.Context, opts ...ScmRepositoriesOption) (*ScmRepositoriesResponse, error) {
	query := &scmRepositoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/scmRepositories"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmRepositories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildScmRepositoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmRepositoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmRepository retrieves an SCM repository by ID.
func (c *Client) GetScmRepository(ctx context.Context, repositoryID string) (*ScmRepositoryResource, error) {
	path := fmt.Sprintf("/v1/scmRepositories/%s", repositoryID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data ScmRepositoryResource `json:"data"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.Data, nil
}

// GetScmGitReference retrieves a git reference by ID.
func (c *Client) GetScmGitReference(ctx context.Context, referenceID string) (*ScmGitReferenceResponse, error) {
	path := fmt.Sprintf("/v1/scmGitReferences/%s", strings.TrimSpace(referenceID))
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmGitReferenceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmGitReferences retrieves git references for a repository.
func (c *Client) GetScmGitReferences(ctx context.Context, repositoryID string, opts ...ScmGitReferencesOption) (*ScmGitReferencesResponse, error) {
	query := &scmGitReferencesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/scmRepositories/%s/gitReferences", repositoryID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmGitReferences: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildScmGitReferencesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmGitReferencesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmRepositoryGitReferencesRelationships retrieves git reference linkages for a repository.
func (c *Client) GetScmRepositoryGitReferencesRelationships(ctx context.Context, repositoryID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	repositoryID = strings.TrimSpace(repositoryID)
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/scmRepositories/%s/relationships/gitReferences", repositoryID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmRepositoryGitReferencesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmRepositoryPullRequests retrieves pull requests for a repository.
func (c *Client) GetScmRepositoryPullRequests(ctx context.Context, repositoryID string, opts ...ScmPullRequestsOption) (*ScmPullRequestsResponse, error) {
	query := &scmPullRequestsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/scmRepositories/%s/pullRequests", strings.TrimSpace(repositoryID))
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmRepositoryPullRequests: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildScmPullRequestsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmPullRequestsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmRepositoryPullRequestsRelationships retrieves pull request linkages for a repository.
func (c *Client) GetScmRepositoryPullRequestsRelationships(ctx context.Context, repositoryID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	repositoryID = strings.TrimSpace(repositoryID)
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/scmRepositories/%s/relationships/pullRequests", repositoryID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmRepositoryPullRequestsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetScmPullRequest retrieves a pull request by ID.
func (c *Client) GetScmPullRequest(ctx context.Context, pullRequestID string) (*ScmPullRequestResponse, error) {
	path := fmt.Sprintf("/v1/scmPullRequests/%s", strings.TrimSpace(pullRequestID))
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response ScmPullRequestResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiMacOsVersions retrieves the list of CI macOS versions.
func (c *Client) GetCiMacOsVersions(ctx context.Context, opts ...CiMacOsVersionsOption) (*CiMacOsVersionsResponse, error) {
	query := &ciMacOsVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/ciMacOsVersions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciMacOsVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiMacOsVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiMacOsVersion retrieves a CI macOS version by ID.
func (c *Client) GetCiMacOsVersion(ctx context.Context, macOsVersionID string) (*CiMacOsVersionResponse, error) {
	path := fmt.Sprintf("/v1/ciMacOsVersions/%s", macOsVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiMacOsVersionXcodeVersions retrieves Xcode versions for a macOS version.
func (c *Client) GetCiMacOsVersionXcodeVersions(ctx context.Context, macOsVersionID string, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := &ciXcodeVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciMacOsVersions/%s/xcodeVersions", macOsVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciMacOsVersionXcodeVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiXcodeVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersions retrieves the list of CI Xcode versions.
func (c *Client) GetCiXcodeVersions(ctx context.Context, opts ...CiXcodeVersionsOption) (*CiXcodeVersionsResponse, error) {
	query := &ciXcodeVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/ciXcodeVersions"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciXcodeVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiXcodeVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersion retrieves a CI Xcode version by ID.
func (c *Client) GetCiXcodeVersion(ctx context.Context, xcodeVersionID string) (*CiXcodeVersionResponse, error) {
	path := fmt.Sprintf("/v1/ciXcodeVersions/%s", xcodeVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiXcodeVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiXcodeVersionMacOsVersions retrieves macOS versions for an Xcode version.
func (c *Client) GetCiXcodeVersionMacOsVersions(ctx context.Context, xcodeVersionID string, opts ...CiMacOsVersionsOption) (*CiMacOsVersionsResponse, error) {
	query := &ciMacOsVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/ciXcodeVersions/%s/macOsVersions", xcodeVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciXcodeVersionMacOsVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildCiMacOsVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiMacOsVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

// ResolveCiProductForApp finds the CI product for a given app ID.
// Returns an error if no product or multiple products are found.
func (c *Client) ResolveCiProductForApp(ctx context.Context, appID string) (*CiProductResource, error) {
	resp, err := c.GetCiProducts(ctx, WithCiProductsAppID(appID), WithCiProductsLimit(200))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch CI products: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("no Xcode Cloud product found for app %q (ensure Xcode Cloud is enabled)", appID)
	}

	if len(resp.Data) > 1 {
		return nil, fmt.Errorf("multiple Xcode Cloud products found for app %q; this is unexpected", appID)
	}

	return &resp.Data[0], nil
}

// ResolveCiWorkflowByName finds a workflow by name for a given product.
// Returns an error if no workflow or multiple workflows match the name.
func (c *Client) ResolveCiWorkflowByName(ctx context.Context, productID, workflowName string) (*CiWorkflowResource, error) {
	var allWorkflows []CiWorkflowResource
	var nextURL string

	for {
		var resp *CiWorkflowsResponse
		var err error

		if nextURL != "" {
			resp, err = c.GetCiWorkflows(ctx, productID, WithCiWorkflowsNextURL(nextURL))
		} else {
			resp, err = c.GetCiWorkflows(ctx, productID, WithCiWorkflowsLimit(200))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch CI workflows: %w", err)
		}

		allWorkflows = append(allWorkflows, resp.Data...)

		if resp.Links.Next == "" {
			break
		}
		nextURL = resp.Links.Next
	}

	if len(allWorkflows) == 0 {
		return nil, fmt.Errorf("no Xcode Cloud workflows found for product %q", productID)
	}

	// Find matching workflows by name (case-insensitive)
	var matches []CiWorkflowResource
	normalizedName := strings.ToLower(strings.TrimSpace(workflowName))
	for _, wf := range allWorkflows {
		if strings.ToLower(wf.Attributes.Name) == normalizedName {
			matches = append(matches, wf)
		}
	}

	if len(matches) == 0 {
		// List available workflows in error message
		var names []string
		for _, wf := range allWorkflows {
			names = append(names, wf.Attributes.Name)
		}
		return nil, fmt.Errorf("no workflow named %q found; available: %s", workflowName, strings.Join(names, ", "))
	}

	if len(matches) > 1 {
		var ids []string
		for _, wf := range matches {
			ids = append(ids, wf.ID)
		}
		return nil, fmt.Errorf("multiple workflows named %q found; use --workflow-id with one of: %s", workflowName, strings.Join(ids, ", "))
	}

	return &matches[0], nil
}

// ResolveGitReferenceByName finds a git reference (branch or tag) by name.
// Returns an error if no reference or multiple references match the name.
func (c *Client) ResolveGitReferenceByName(ctx context.Context, repositoryID, refName string) (*ScmGitReferenceResource, error) {
	var allRefs []ScmGitReferenceResource
	var nextURL string

	for {
		var resp *ScmGitReferencesResponse
		var err error

		if nextURL != "" {
			resp, err = c.GetScmGitReferences(ctx, repositoryID, WithScmGitReferencesNextURL(nextURL))
		} else {
			resp, err = c.GetScmGitReferences(ctx, repositoryID, WithScmGitReferencesLimit(200))
		}
		if err != nil {
			return nil, fmt.Errorf("failed to fetch git references: %w", err)
		}

		allRefs = append(allRefs, resp.Data...)

		if resp.Links.Next == "" {
			break
		}
		nextURL = resp.Links.Next
	}

	if len(allRefs) == 0 {
		return nil, fmt.Errorf("no git references found for repository %q", repositoryID)
	}

	// Find matching references by name
	normalizedName := strings.TrimSpace(refName)
	headsName := "refs/heads/" + normalizedName
	tagsName := "refs/tags/" + normalizedName
	var matches []ScmGitReferenceResource
	for _, ref := range allRefs {
		// Match by exact name or canonical ref (e.g., "main" or "refs/heads/main").
		canonical := ref.Attributes.CanonicalName
		if ref.Attributes.Name == normalizedName ||
			canonical == normalizedName ||
			canonical == headsName ||
			canonical == tagsName {
			if !ref.Attributes.IsDeleted {
				matches = append(matches, ref)
			}
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no git reference named %q found; use --git-reference-id to specify directly", refName)
	}

	if len(matches) > 1 {
		var ids []string
		for _, ref := range matches {
			ids = append(ids, fmt.Sprintf("%s (%s)", ref.ID, ref.Attributes.CanonicalName))
		}
		return nil, fmt.Errorf("multiple git references match %q; use --git-reference-id with one of: %s", refName, strings.Join(ids, ", "))
	}

	return &matches[0], nil
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

// CiMacOsVersionAttributes describes a CI macOS version resource.
type CiMacOsVersionAttributes struct {
	Version string `json:"version,omitempty"`
	Name    string `json:"name,omitempty"`
}

// CiMacOsVersionRelationships describes relationships for a CI macOS version.
type CiMacOsVersionRelationships struct {
	XcodeVersions *RelationshipList `json:"xcodeVersions,omitempty"`
}

// CiMacOsVersionResource represents a CI macOS version resource.
type CiMacOsVersionResource struct {
	Type          ResourceType                 `json:"type"`
	ID            string                       `json:"id"`
	Attributes    CiMacOsVersionAttributes     `json:"attributes"`
	Relationships *CiMacOsVersionRelationships `json:"relationships,omitempty"`
}

// CiMacOsVersionsResponse is the response from CI macOS versions endpoints.
type CiMacOsVersionsResponse struct {
	Data     []CiMacOsVersionResource `json:"data"`
	Included []CiXcodeVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiMacOsVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiMacOsVersionsResponse) GetData() any {
	return r.Data
}

// CiMacOsVersionResponse is the response from CI macOS version detail endpoints.
type CiMacOsVersionResponse struct {
	Data     CiMacOsVersionResource   `json:"data"`
	Included []CiXcodeVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// CiTestDestinationRuntime describes an available runtime for a test destination.
type CiTestDestinationRuntime struct {
	RuntimeName       string `json:"runtimeName,omitempty"`
	RuntimeIdentifier string `json:"runtimeIdentifier,omitempty"`
}

// CiTestDestination describes a test destination for an Xcode version.
type CiTestDestination struct {
	DeviceTypeName       string                     `json:"deviceTypeName,omitempty"`
	DeviceTypeIdentifier string                     `json:"deviceTypeIdentifier,omitempty"`
	AvailableRuntimes    []CiTestDestinationRuntime `json:"availableRuntimes,omitempty"`
	Kind                 CiTestDestinationKind      `json:"kind,omitempty"`
}

// CiXcodeVersionAttributes describes a CI Xcode version resource.
type CiXcodeVersionAttributes struct {
	Version          string              `json:"version,omitempty"`
	Name             string              `json:"name,omitempty"`
	TestDestinations []CiTestDestination `json:"testDestinations,omitempty"`
}

// CiXcodeVersionRelationships describes relationships for a CI Xcode version.
type CiXcodeVersionRelationships struct {
	MacOsVersions *RelationshipList `json:"macOsVersions,omitempty"`
}

// CiXcodeVersionResource represents a CI Xcode version resource.
type CiXcodeVersionResource struct {
	Type          ResourceType                 `json:"type"`
	ID            string                       `json:"id"`
	Attributes    CiXcodeVersionAttributes     `json:"attributes"`
	Relationships *CiXcodeVersionRelationships `json:"relationships,omitempty"`
}

// CiXcodeVersionsResponse is the response from CI Xcode versions endpoints.
type CiXcodeVersionsResponse struct {
	Data     []CiXcodeVersionResource `json:"data"`
	Included []CiMacOsVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *CiXcodeVersionsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *CiXcodeVersionsResponse) GetData() any {
	return r.Data
}

// CiXcodeVersionResponse is the response from CI Xcode version detail endpoints.
type CiXcodeVersionResponse struct {
	Data     CiXcodeVersionResource   `json:"data"`
	Included []CiMacOsVersionResource `json:"included,omitempty"`
	Links    Links                    `json:"links"`
}

type ciMacOsVersionsQuery struct {
	listQuery
}

// CiMacOsVersionsOption is a functional option for GetCiMacOsVersions.
type CiMacOsVersionsOption func(*ciMacOsVersionsQuery)

// WithCiMacOsVersionsLimit sets the max number of macOS versions to return.
func WithCiMacOsVersionsLimit(limit int) CiMacOsVersionsOption {
	return func(q *ciMacOsVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiMacOsVersionsNextURL uses a next page URL directly.
func WithCiMacOsVersionsNextURL(next string) CiMacOsVersionsOption {
	return func(q *ciMacOsVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiMacOsVersionsQuery(query *ciMacOsVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciXcodeVersionsQuery struct {
	listQuery
}

// CiXcodeVersionsOption is a functional option for GetCiXcodeVersions.
type CiXcodeVersionsOption func(*ciXcodeVersionsQuery)

// WithCiXcodeVersionsLimit sets the max number of Xcode versions to return.
func WithCiXcodeVersionsLimit(limit int) CiXcodeVersionsOption {
	return func(q *ciXcodeVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiXcodeVersionsNextURL uses a next page URL directly.
func WithCiXcodeVersionsNextURL(next string) CiXcodeVersionsOption {
	return func(q *ciXcodeVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiXcodeVersionsQuery(query *ciXcodeVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

type ciProductRepositoriesQuery struct {
	listQuery
}

// CiProductRepositoriesOption is a functional option for CI product repository lists.
type CiProductRepositoriesOption func(*ciProductRepositoriesQuery)

// WithCiProductRepositoriesLimit sets the max number of repositories to return.
func WithCiProductRepositoriesLimit(limit int) CiProductRepositoriesOption {
	return func(q *ciProductRepositoriesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithCiProductRepositoriesNextURL uses a next page URL directly.
func WithCiProductRepositoriesNextURL(next string) CiProductRepositoriesOption {
	return func(q *ciProductRepositoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildCiProductRepositoriesQuery(query *ciProductRepositoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
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
