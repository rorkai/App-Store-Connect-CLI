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

func buildScmPullRequestsQuery(query *scmPullRequestsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
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

// ResolveGitReferenceByName finds a git reference (branch or tag) by name.
// Returns an error if no reference or multiple references match the name.
func (c *Client) ResolveGitReferenceByName(ctx context.Context, repositoryID, refName string) (*ScmGitReferenceResource, error) {
	var allRefs []ScmGitReferenceResource
	var nextURL string

	for {
		var resp *ScmGitReferencesResponse
		var err error

		if nextURL != "" {
			resp, err = c.GetScmGitReferences(ctx, repositoryID, NextURL[ScmGitReferencesOption](nextURL))
		} else {
			resp, err = c.GetScmGitReferences(ctx, repositoryID, Limit[ScmGitReferencesOption](200))
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
		return nil, newXcodeCloudLookupError(ErrGitReferenceNotFound, "no git references found for repository %q", repositoryID)
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
		return nil, newXcodeCloudLookupError(ErrGitReferenceNotFound, "no git reference named %q found; use --git-reference-id to specify directly", refName)
	}

	if len(matches) > 1 {
		var ids []string
		for _, ref := range matches {
			ids = append(ids, fmt.Sprintf("%s (%s)", ref.ID, ref.Attributes.CanonicalName))
		}
		return nil, newXcodeCloudLookupError(ErrGitReferenceAmbiguous, "multiple git references match %q; use --git-reference-id with one of: %s", refName, strings.Join(ids, ", "))
	}

	return &matches[0], nil
}
