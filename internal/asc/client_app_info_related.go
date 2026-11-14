package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetAppInfoPrimaryCategory retrieves the primary category for an app info.
func (c *Client) GetAppInfoPrimaryCategory(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/primaryCategory", appInfoID)})
}

// GetAppInfoPrimarySubcategoryOne retrieves the primary subcategory one for an app info.
func (c *Client) GetAppInfoPrimarySubcategoryOne(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/primarySubcategoryOne", appInfoID)})
}

// GetAppInfoPrimarySubcategoryTwo retrieves the primary subcategory two for an app info.
func (c *Client) GetAppInfoPrimarySubcategoryTwo(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/primarySubcategoryTwo", appInfoID)})
}

// GetAppInfoSecondaryCategory retrieves the secondary category for an app info.
func (c *Client) GetAppInfoSecondaryCategory(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/secondaryCategory", appInfoID)})
}

// GetAppInfoSecondarySubcategoryOne retrieves the secondary subcategory one for an app info.
func (c *Client) GetAppInfoSecondarySubcategoryOne(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/secondarySubcategoryOne", appInfoID)})
}

// GetAppInfoSecondarySubcategoryTwo retrieves the secondary subcategory two for an app info.
func (c *Client) GetAppInfoSecondarySubcategoryTwo(ctx context.Context, appInfoID string) (*AppCategoryResponse, error) {
	appInfoID = strings.TrimSpace(appInfoID)
	if appInfoID == "" {
		return nil, fmt.Errorf("appInfoID is required")
	}

	return getJSON[AppCategoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/appInfos/%s/secondarySubcategoryTwo", appInfoID)})
}
