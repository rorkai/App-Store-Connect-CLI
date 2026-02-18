package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// AppCategoryAttributes describes app category metadata.
type AppCategoryAttributes struct {
	Platforms []Platform `json:"platforms,omitempty"`
}

// AppCategory represents an app category resource.
type AppCategory struct {
	Type       ResourceType          `json:"type"`
	ID         string                `json:"id"`
	Attributes AppCategoryAttributes `json:"attributes"`
}

// AppCategoriesResponse is the response from app categories endpoint.
type AppCategoriesResponse struct {
	Data  []AppCategory `json:"data"`
	Links Links         `json:"links"`
}

// GetLinks returns the links field for pagination.
func (r *AppCategoriesResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for aggregation.
func (r *AppCategoriesResponse) GetData() any {
	return r.Data
}

// AppCategoryResponse is the response from app category detail endpoints.
type AppCategoryResponse struct {
	Data  AppCategory `json:"data"`
	Links Links       `json:"links"`
}

// AppCategoryParentLinkageResponse is the response for app category parent relationship.
type AppCategoryParentLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// GetAppCategories retrieves all app categories.
func (c *Client) GetAppCategories(ctx context.Context, opts ...AppCategoriesOption) (*AppCategoriesResponse, error) {
	query := &appCategoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/appCategories"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCategories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCategoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCategoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// appCategoriesQuery holds query parameters for app categories.
type appCategoriesQuery struct {
	listQuery
}

// AppCategoriesOption configures app categories queries.
type AppCategoriesOption func(*appCategoriesQuery)

// WithAppCategoriesLimit sets the limit for app categories queries.
func WithAppCategoriesLimit(limit int) AppCategoriesOption {
	return func(q *appCategoriesQuery) {
		q.limit = limit
	}
}

// WithAppCategoriesNextURL uses a next page URL directly.
func WithAppCategoriesNextURL(next string) AppCategoriesOption {
	return func(q *appCategoriesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildAppCategoriesQuery(query *appCategoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GetAppCategory retrieves a single app category by ID.
func (c *Client) GetAppCategory(ctx context.Context, categoryID string) (*AppCategoryResponse, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	path := fmt.Sprintf("/v1/appCategories/%s", categoryID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCategoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCategoryParent retrieves the parent category for a category.
func (c *Client) GetAppCategoryParent(ctx context.Context, categoryID string) (*AppCategoryResponse, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	path := fmt.Sprintf("/v1/appCategories/%s/parent", categoryID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCategoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCategoryParentRelationship retrieves the parent category linkage for a category.
func (c *Client) GetAppCategoryParentRelationship(ctx context.Context, categoryID string) (*AppCategoryParentLinkageResponse, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	path := fmt.Sprintf("/v1/appCategories/%s/relationships/parent", categoryID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCategoryParentLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCategorySubcategories retrieves subcategories for a category.
func (c *Client) GetAppCategorySubcategories(ctx context.Context, categoryID string, opts ...AppCategoriesOption) (*AppCategoriesResponse, error) {
	query := &appCategoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	categoryID = strings.TrimSpace(categoryID)
	if query.nextURL == "" && categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	path := fmt.Sprintf("/v1/appCategories/%s/subcategories", categoryID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCategorySubcategories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppCategoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppCategoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppCategorySubcategoriesRelationships retrieves subcategory linkages for a category.
func (c *Client) GetAppCategorySubcategoriesRelationships(ctx context.Context, categoryID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	categoryID = strings.TrimSpace(categoryID)
	if query.nextURL == "" && categoryID == "" {
		return nil, fmt.Errorf("categoryID is required")
	}

	path := fmt.Sprintf("/v1/appCategories/%s/relationships/subcategories", categoryID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appCategorySubcategoriesRelationships: %w", err)
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

// AppInfoUpdateCategoriesRelationships describes relationships for updating categories.
type AppInfoUpdateCategoriesRelationships struct {
	PrimaryCategory         *Relationship `json:"primaryCategory,omitempty"`
	SecondaryCategory       *Relationship `json:"secondaryCategory,omitempty"`
	PrimarySubcategoryOne   *Relationship `json:"primarySubcategoryOne,omitempty"`
	PrimarySubcategoryTwo   *Relationship `json:"primarySubcategoryTwo,omitempty"`
	SecondarySubcategoryOne *Relationship `json:"secondarySubcategoryOne,omitempty"`
	SecondarySubcategoryTwo *Relationship `json:"secondarySubcategoryTwo,omitempty"`
}

// AppInfoUpdateCategoriesData is the data for updating app info categories.
type AppInfoUpdateCategoriesData struct {
	Type          ResourceType                          `json:"type"`
	ID            string                                `json:"id"`
	Relationships *AppInfoUpdateCategoriesRelationships `json:"relationships,omitempty"`
}

// AppInfoUpdateCategoriesRequest is a request to update app info categories.
type AppInfoUpdateCategoriesRequest struct {
	Data AppInfoUpdateCategoriesData `json:"data"`
}

// UpdateAppInfoCategories updates the categories for an app info resource.
func (c *Client) UpdateAppInfoCategories(ctx context.Context, appInfoID string, primaryCategoryID, secondaryCategoryID string) (*AppInfoResponse, error) {
	relationships := &AppInfoUpdateCategoriesRelationships{}

	if primaryCategoryID != "" {
		relationships.PrimaryCategory = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   primaryCategoryID,
			},
		}
	}

	if secondaryCategoryID != "" {
		relationships.SecondaryCategory = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   secondaryCategoryID,
			},
		}
	}

	request := AppInfoUpdateCategoriesRequest{
		Data: AppInfoUpdateCategoriesData{
			Type:          ResourceTypeAppInfos,
			ID:            appInfoID,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(request)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appInfos/%s", appInfoID), body)
	if err != nil {
		return nil, err
	}

	var response AppInfoResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
