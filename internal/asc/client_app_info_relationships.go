package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/ageRatingDeclaration", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoAgeRatingDeclarationLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse ageRatingDeclaration relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoPrimaryCategoryRelationship retrieves the primary category linkage for an app info.
func (c *Client) GetAppInfoPrimaryCategoryRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimaryCategoryLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/primaryCategory", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoPrimaryCategoryLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse primaryCategory relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoPrimarySubcategoryOneRelationship retrieves the primary subcategory one linkage.
func (c *Client) GetAppInfoPrimarySubcategoryOneRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimarySubcategoryOneLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/primarySubcategoryOne", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoPrimarySubcategoryOneLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse primarySubcategoryOne relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoPrimarySubcategoryTwoRelationship retrieves the primary subcategory two linkage.
func (c *Client) GetAppInfoPrimarySubcategoryTwoRelationship(ctx context.Context, appInfoID string) (*AppInfoPrimarySubcategoryTwoLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/primarySubcategoryTwo", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoPrimarySubcategoryTwoLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse primarySubcategoryTwo relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoSecondaryCategoryRelationship retrieves the secondary category linkage.
func (c *Client) GetAppInfoSecondaryCategoryRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondaryCategoryLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/secondaryCategory", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoSecondaryCategoryLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse secondaryCategory relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoSecondarySubcategoryOneRelationship retrieves the secondary subcategory one linkage.
func (c *Client) GetAppInfoSecondarySubcategoryOneRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondarySubcategoryOneLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/secondarySubcategoryOne", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoSecondarySubcategoryOneLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse secondarySubcategoryOne relationship response: %w", err)
	}

	return &response, nil
}

// GetAppInfoSecondarySubcategoryTwoRelationship retrieves the secondary subcategory two linkage.
func (c *Client) GetAppInfoSecondarySubcategoryTwoRelationship(ctx context.Context, appInfoID string) (*AppInfoSecondarySubcategoryTwoLinkageResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	path := fmt.Sprintf("/v1/appInfos/%s/relationships/secondarySubcategoryTwo", appInfoID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppInfoSecondarySubcategoryTwoLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse secondarySubcategoryTwo relationship response: %w", err)
	}

	return &response, nil
}
