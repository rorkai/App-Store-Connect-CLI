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
