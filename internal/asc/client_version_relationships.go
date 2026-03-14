package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// AppStoreVersionAgeRatingDeclarationLinkageResponse is the response for age rating relationships.
type AppStoreVersionAgeRatingDeclarationLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionReviewDetailLinkageResponse is the response for review detail relationships.
type AppStoreVersionReviewDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionAppClipDefaultExperienceLinkageResponse is the response for app clip default experience relationships.
type AppStoreVersionAppClipDefaultExperienceLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionPhasedReleaseLinkageResponse is the response for phased release relationships.
type AppStoreVersionPhasedReleaseLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionBuildLinkageResponse is the response for build relationships.
type AppStoreVersionBuildLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionSubmissionLinkageResponse is the response for submission relationships.
type AppStoreVersionSubmissionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionRoutingAppCoverageLinkageResponse is the response for routing coverage relationships.
type AppStoreVersionRoutingAppCoverageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppStoreVersionGameCenterAppVersionLinkageResponse is the response for Game Center app version relationships.
type AppStoreVersionGameCenterAppVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetAppStoreVersionAgeRatingDeclarationRelationship retrieves the age rating linkage for a version.
func (c *Client) GetAppStoreVersionAgeRatingDeclarationRelationship(ctx context.Context, versionID string) (*AppStoreVersionAgeRatingDeclarationLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	appInfoID, err := c.ResolveAppInfoIDForAppStoreVersion(ctx, versionID)
	if err != nil {
		return nil, err
	}

	resp, err := c.GetAppInfoAgeRatingDeclarationRelationship(ctx, appInfoID)
	if err != nil {
		return nil, err
	}

	return &AppStoreVersionAgeRatingDeclarationLinkageResponse{
		Data:  resp.Data,
		Links: resp.Links,
	}, nil
}

// GetAppStoreVersionReviewDetailRelationship retrieves the review detail linkage for a version.
func (c *Client) GetAppStoreVersionReviewDetailRelationship(ctx context.Context, versionID string) (*AppStoreVersionReviewDetailLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/appStoreReviewDetail", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionReviewDetailLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse appStoreReviewDetail relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionAppClipDefaultExperienceRelationship retrieves the app clip default experience linkage.
func (c *Client) GetAppStoreVersionAppClipDefaultExperienceRelationship(ctx context.Context, versionID string) (*AppStoreVersionAppClipDefaultExperienceLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/appClipDefaultExperience", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionAppClipDefaultExperienceLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse appClipDefaultExperience relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionLocalizationsRelationships retrieves localization linkages for a version.
func (c *Client) GetAppStoreVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		"appStoreVersionLocalizations",
		"versionID",
		"/v1/appStoreVersions/%s/relationships/%s",
		"appStoreVersionLocalizationsRelationships",
		opts...,
	)
}

// GetAppStoreVersionPhasedReleaseRelationship retrieves the phased release linkage for a version.
func (c *Client) GetAppStoreVersionPhasedReleaseRelationship(ctx context.Context, versionID string) (*AppStoreVersionPhasedReleaseLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/appStoreVersionPhasedRelease", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionPhasedReleaseLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse appStoreVersionPhasedRelease relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionBuildRelationship retrieves the build linkage for a version.
func (c *Client) GetAppStoreVersionBuildRelationship(ctx context.Context, versionID string) (*AppStoreVersionBuildLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/build", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionBuildLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse build relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionExperimentsRelationships retrieves experiment linkages for a version (v1).
func (c *Client) GetAppStoreVersionExperimentsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		"appStoreVersionExperiments",
		"versionID",
		"/v1/appStoreVersions/%s/relationships/%s",
		"appStoreVersionExperimentsRelationships",
		opts...,
	)
}

// GetAppStoreVersionExperimentsV2Relationships retrieves experiment linkages for a version (v2).
func (c *Client) GetAppStoreVersionExperimentsV2Relationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		"appStoreVersionExperimentsV2",
		"versionID",
		"/v1/appStoreVersions/%s/relationships/%s",
		"appStoreVersionExperimentsV2Relationships",
		opts...,
	)
}

// GetAppStoreVersionSubmissionRelationship retrieves the submission linkage for a version.
func (c *Client) GetAppStoreVersionSubmissionRelationship(ctx context.Context, versionID string) (*AppStoreVersionSubmissionLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/appStoreVersionSubmission", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionSubmissionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse appStoreVersionSubmission relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionCustomerReviewsRelationships retrieves customer review linkages for a version.
func (c *Client) GetAppStoreVersionCustomerReviewsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		versionID,
		"customerReviews",
		"versionID",
		"/v1/appStoreVersions/%s/relationships/%s",
		"customerReviewsRelationships",
		opts...,
	)
}

// GetAppStoreVersionRoutingAppCoverageRelationship retrieves routing coverage linkage for a version.
func (c *Client) GetAppStoreVersionRoutingAppCoverageRelationship(ctx context.Context, versionID string) (*AppStoreVersionRoutingAppCoverageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/routingAppCoverage", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionRoutingAppCoverageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse routingAppCoverage relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionGameCenterAppVersionRelationship retrieves Game Center app version linkage.
func (c *Client) GetAppStoreVersionGameCenterAppVersionRelationship(ctx context.Context, versionID string) (*AppStoreVersionGameCenterAppVersionLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/gameCenterAppVersion", versionID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionGameCenterAppVersionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse gameCenterAppVersion relationship response: %w", err)
	}

	return &response, nil
}
