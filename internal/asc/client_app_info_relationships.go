package asc

import (
	"context"
	"fmt"
	"strings"
)

// AppInfoAgeRatingDeclarationLinkageResponse is the response for age rating relationships.
type AppInfoAgeRatingDeclarationLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoPrimaryCategoryLinkageResponse is the response for primary category relationships.
type AppInfoPrimaryCategoryLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoPrimarySubcategoryOneLinkageResponse is the response for primary subcategory one relationships.
type AppInfoPrimarySubcategoryOneLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoPrimarySubcategoryTwoLinkageResponse is the response for primary subcategory two relationships.
type AppInfoPrimarySubcategoryTwoLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoSecondaryCategoryLinkageResponse is the response for secondary category relationships.
type AppInfoSecondaryCategoryLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoSecondarySubcategoryOneLinkageResponse is the response for secondary subcategory one relationships.
type AppInfoSecondarySubcategoryOneLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// AppInfoSecondarySubcategoryTwoLinkageResponse is the response for secondary subcategory two relationships.
type AppInfoSecondarySubcategoryTwoLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetAppInfoAgeRatingDeclarationRelationship retrieves the age rating linkage for an app info.
func (c *Client) GetAppInfoAgeRatingDeclarationRelationship(ctx context.Context, appInfoID string) (*AppInfoAgeRatingDeclarationLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoAgeRatingDeclarationLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/ageRatingDeclaration", appInfoID),
		ParseErr: "failed to parse ageRatingDeclaration relationship response",
	})
}

// GetAppInfoPrimaryCategoryRelationship retrieves the primary category linkage for an app info.
func (c *Client) GetAppInfoPrimaryCategoryRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimaryCategoryLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoPrimaryCategoryLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/primaryCategory", appInfoID),
		ParseErr: "failed to parse primaryCategory relationship response",
	})
}

// GetAppInfoPrimarySubcategoryOneRelationship retrieves the primary subcategory one linkage.
func (c *Client) GetAppInfoPrimarySubcategoryOneRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimarySubcategoryOneLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoPrimarySubcategoryOneLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/primarySubcategoryOne", appInfoID),
		ParseErr: "failed to parse primarySubcategoryOne relationship response",
	})
}

// GetAppInfoPrimarySubcategoryTwoRelationship retrieves the primary subcategory two linkage.
func (c *Client) GetAppInfoPrimarySubcategoryTwoRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimarySubcategoryTwoLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoPrimarySubcategoryTwoLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/primarySubcategoryTwo", appInfoID),
		ParseErr: "failed to parse primarySubcategoryTwo relationship response",
	})
}

// GetAppInfoSecondaryCategoryRelationship retrieves the secondary category linkage.
func (c *Client) GetAppInfoSecondaryCategoryRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondaryCategoryLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoSecondaryCategoryLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/secondaryCategory", appInfoID),
		ParseErr: "failed to parse secondaryCategory relationship response",
	})
}

// GetAppInfoSecondarySubcategoryOneRelationship retrieves the secondary subcategory one linkage.
func (c *Client) GetAppInfoSecondarySubcategoryOneRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondarySubcategoryOneLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoSecondarySubcategoryOneLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/secondarySubcategoryOne", appInfoID),
		ParseErr: "failed to parse secondarySubcategoryOne relationship response",
	})
}

// GetAppInfoSecondarySubcategoryTwoRelationship retrieves the secondary subcategory two linkage.
func (c *Client) GetAppInfoSecondarySubcategoryTwoRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondarySubcategoryTwoLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppInfoSecondarySubcategoryTwoLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appInfos/%s/relationships/secondarySubcategoryTwo", appInfoID),
		ParseErr: "failed to parse secondarySubcategoryTwo relationship response",
	})
}
