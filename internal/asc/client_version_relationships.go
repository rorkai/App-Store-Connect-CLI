package asc

import (
	"context"
	"fmt"
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
