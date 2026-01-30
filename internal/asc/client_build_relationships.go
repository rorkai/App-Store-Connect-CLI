package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// BuildAppLinkageResponse is the response for build app relationships.
type BuildAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// BuildAppStoreVersionLinkageResponse is the response for build app store version relationships.
type BuildAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// BuildBuildBetaDetailLinkageResponse is the response for build beta detail relationships.
type BuildBuildBetaDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// BuildPreReleaseVersionLinkageResponse is the response for build pre-release version relationships.
type BuildPreReleaseVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// GetBuildAppRelationship retrieves the app linkage for a build.
func (c *Client) GetBuildAppRelationship(ctx context.Context, buildID string) (*BuildAppLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/relationships/app", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildAppLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildAppStoreVersionRelationship retrieves the app store version linkage for a build.
func (c *Client) GetBuildAppStoreVersionRelationship(ctx context.Context, buildID string) (*BuildAppStoreVersionLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/relationships/appStoreVersion", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildAppStoreVersionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildBetaBuildLocalizationsRelationships retrieves beta build localization linkages for a build.
func (c *Client) GetBuildBetaBuildLocalizationsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "betaBuildLocalizations", opts...)
}

// GetBuildBuildBetaDetailRelationship retrieves build beta detail linkage for a build.
func (c *Client) GetBuildBuildBetaDetailRelationship(ctx context.Context, buildID string) (*BuildBuildBetaDetailLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/relationships/buildBetaDetail", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildBuildBetaDetailLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildDiagnosticSignaturesRelationships retrieves diagnostic signature linkages for a build.
func (c *Client) GetBuildDiagnosticSignaturesRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "diagnosticSignatures", opts...)
}

// GetBuildIndividualTestersRelationships retrieves individual tester linkages for a build.
func (c *Client) GetBuildIndividualTestersRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "individualTesters", opts...)
}

// GetBuildPreReleaseVersionRelationship retrieves pre-release version linkage for a build.
func (c *Client) GetBuildPreReleaseVersionRelationship(ctx context.Context, buildID string) (*BuildPreReleaseVersionLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/relationships/preReleaseVersion", buildID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BuildPreReleaseVersionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBuildIconsRelationships retrieves build icon linkages for a build.
func (c *Client) GetBuildIconsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "icons", opts...)
}

func (c *Client) getBuildLinkages(ctx context.Context, buildID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	buildID = strings.TrimSpace(buildID)
	if query.nextURL == "" && buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	path := fmt.Sprintf("/v1/builds/%s/relationships/%s", buildID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("buildRelationships: %w", err)
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
