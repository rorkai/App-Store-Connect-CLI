package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CiBuildActionBuildRunLinkageResponse is the response for ciBuildAction buildRun relationship linkages (to-one).
type CiBuildActionBuildRunLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// CiProductAppLinkageResponse is the response for ciProduct app relationship linkages (to-one).
type CiProductAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// CiWorkflowRepositoryLinkageResponse is the response for ciWorkflow repository relationship linkages (to-one).
type CiWorkflowRepositoryLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetCiBuildActionArtifactsRelationships retrieves artifact linkages for a CI build action.
func (c *Client) GetCiBuildActionArtifactsRelationships(ctx context.Context, buildActionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiBuildActionLinkages(ctx, buildActionID, "artifacts", opts...)
}

// GetCiBuildActionBuildRunRelationship retrieves the build run linkage for a CI build action (to-one).
func (c *Client) GetCiBuildActionBuildRunRelationship(ctx context.Context, buildActionID string) (*CiBuildActionBuildRunLinkageResponse, error) {
	buildActionID = strings.TrimSpace(buildActionID)
	if buildActionID == "" {
		return nil, fmt.Errorf("buildActionID is required")
	}

	path := fmt.Sprintf("/v1/ciBuildActions/%s/relationships/buildRun", buildActionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiBuildActionBuildRunLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiBuildActionIssuesRelationships retrieves issue linkages for a CI build action.
func (c *Client) GetCiBuildActionIssuesRelationships(ctx context.Context, buildActionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiBuildActionLinkages(ctx, buildActionID, "issues", opts...)
}

// GetCiBuildActionTestResultsRelationships retrieves test result linkages for a CI build action.
func (c *Client) GetCiBuildActionTestResultsRelationships(ctx context.Context, buildActionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiBuildActionLinkages(ctx, buildActionID, "testResults", opts...)
}

func (c *Client) getCiBuildActionLinkages(ctx context.Context, buildActionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	buildActionID = strings.TrimSpace(buildActionID)
	if query.nextURL == "" && buildActionID == "" {
		return nil, fmt.Errorf("buildActionID is required")
	}

	path := fmt.Sprintf("/v1/ciBuildActions/%s/relationships/%s", buildActionID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciBuildActionRelationships: %w", err)
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

// GetCiBuildRunActionsRelationships retrieves build action linkages for a CI build run.
func (c *Client) GetCiBuildRunActionsRelationships(ctx context.Context, buildRunID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiBuildRunLinkages(ctx, buildRunID, "actions", opts...)
}

// GetCiBuildRunBuildsRelationships retrieves build linkages for a CI build run.
func (c *Client) GetCiBuildRunBuildsRelationships(ctx context.Context, buildRunID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiBuildRunLinkages(ctx, buildRunID, "builds", opts...)
}

func (c *Client) getCiBuildRunLinkages(ctx context.Context, buildRunID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	buildRunID = strings.TrimSpace(buildRunID)
	if query.nextURL == "" && buildRunID == "" {
		return nil, fmt.Errorf("buildRunID is required")
	}

	path := fmt.Sprintf("/v1/ciBuildRuns/%s/relationships/%s", buildRunID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciBuildRunRelationships: %w", err)
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

// GetCiMacOsVersionXcodeVersionsRelationships retrieves Xcode version linkages for a CI macOS version.
func (c *Client) GetCiMacOsVersionXcodeVersionsRelationships(ctx context.Context, macOsVersionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	macOsVersionID = strings.TrimSpace(macOsVersionID)
	if query.nextURL == "" && macOsVersionID == "" {
		return nil, fmt.Errorf("macOsVersionID is required")
	}

	path := fmt.Sprintf("/v1/ciMacOsVersions/%s/relationships/xcodeVersions", macOsVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciMacOsVersionXcodeVersionsRelationships: %w", err)
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

// GetCiProductAdditionalRepositoriesRelationships retrieves additional repository linkages for a CI product.
func (c *Client) GetCiProductAdditionalRepositoriesRelationships(ctx context.Context, productID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiProductLinkages(ctx, productID, "additionalRepositories", opts...)
}

// GetCiProductAppRelationship retrieves the app linkage for a CI product (to-one).
func (c *Client) GetCiProductAppRelationship(ctx context.Context, productID string) (*CiProductAppLinkageResponse, error) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return nil, fmt.Errorf("productID is required")
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/relationships/app", productID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiProductAppLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetCiProductBuildRunsRelationships retrieves build run linkages for a CI product.
func (c *Client) GetCiProductBuildRunsRelationships(ctx context.Context, productID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiProductLinkages(ctx, productID, "buildRuns", opts...)
}

// GetCiProductPrimaryRepositoriesRelationships retrieves primary repository linkages for a CI product.
func (c *Client) GetCiProductPrimaryRepositoriesRelationships(ctx context.Context, productID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiProductLinkages(ctx, productID, "primaryRepositories", opts...)
}

// GetCiProductWorkflowsRelationships retrieves workflow linkages for a CI product.
func (c *Client) GetCiProductWorkflowsRelationships(ctx context.Context, productID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiProductLinkages(ctx, productID, "workflows", opts...)
}

func (c *Client) getCiProductLinkages(ctx context.Context, productID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	productID = strings.TrimSpace(productID)
	if query.nextURL == "" && productID == "" {
		return nil, fmt.Errorf("productID is required")
	}

	path := fmt.Sprintf("/v1/ciProducts/%s/relationships/%s", productID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciProductRelationships: %w", err)
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

// GetCiWorkflowBuildRunsRelationships retrieves build run linkages for a CI workflow.
func (c *Client) GetCiWorkflowBuildRunsRelationships(ctx context.Context, workflowID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getCiWorkflowLinkages(ctx, workflowID, "buildRuns", opts...)
}

// GetCiWorkflowRepositoryRelationship retrieves the repository linkage for a CI workflow (to-one).
func (c *Client) GetCiWorkflowRepositoryRelationship(ctx context.Context, workflowID string) (*CiWorkflowRepositoryLinkageResponse, error) {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("workflowID is required")
	}

	path := fmt.Sprintf("/v1/ciWorkflows/%s/relationships/repository", workflowID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response CiWorkflowRepositoryLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

func (c *Client) getCiWorkflowLinkages(ctx context.Context, workflowID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	workflowID = strings.TrimSpace(workflowID)
	if query.nextURL == "" && workflowID == "" {
		return nil, fmt.Errorf("workflowID is required")
	}

	path := fmt.Sprintf("/v1/ciWorkflows/%s/relationships/%s", workflowID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciWorkflowRelationships: %w", err)
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

// GetCiXcodeVersionMacOsVersionsRelationships retrieves macOS version linkages for a CI Xcode version.
func (c *Client) GetCiXcodeVersionMacOsVersionsRelationships(ctx context.Context, xcodeVersionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	xcodeVersionID = strings.TrimSpace(xcodeVersionID)
	if query.nextURL == "" && xcodeVersionID == "" {
		return nil, fmt.Errorf("xcodeVersionID is required")
	}

	path := fmt.Sprintf("/v1/ciXcodeVersions/%s/relationships/macOsVersions", xcodeVersionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("ciXcodeVersionMacOsVersionsRelationships: %w", err)
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

// GetScmProviderRepositoriesRelationships retrieves repository linkages for an SCM provider.
func (c *Client) GetScmProviderRepositoriesRelationships(ctx context.Context, providerID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	providerID = strings.TrimSpace(providerID)
	if query.nextURL == "" && providerID == "" {
		return nil, fmt.Errorf("providerID is required")
	}

	path := fmt.Sprintf("/v1/scmProviders/%s/relationships/repositories", providerID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("scmProviderRepositoriesRelationships: %w", err)
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
