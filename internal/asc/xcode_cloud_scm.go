package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

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
	path := fmt.Sprintf("/v1/scmRepositories/%s", strings.TrimSpace(repositoryID))
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

	path := fmt.Sprintf("/v1/scmRepositories/%s/gitReferences", strings.TrimSpace(repositoryID))
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
