package asc

import (
	"context"
	"fmt"
	"net/url"
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

func buildAppCategoriesQuery(query *appCategoriesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
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
func (c *Client) UpdateAppInfoCategories(ctx context.Context, appInfoID string, primaryCategoryID, secondaryCategoryID, primarySubcategoryOneID, primarySubcategoryTwoID, secondarySubcategoryOneID, secondarySubcategoryTwoID string) (*AppInfoResponse, error) {
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

	if primarySubcategoryOneID != "" {
		relationships.PrimarySubcategoryOne = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   primarySubcategoryOneID,
			},
		}
	}

	if primarySubcategoryTwoID != "" {
		relationships.PrimarySubcategoryTwo = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   primarySubcategoryTwoID,
			},
		}
	}

	if secondarySubcategoryOneID != "" {
		relationships.SecondarySubcategoryOne = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   secondarySubcategoryOneID,
			},
		}
	}

	if secondarySubcategoryTwoID != "" {
		relationships.SecondarySubcategoryTwo = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppCategories,
				ID:   secondarySubcategoryTwoID,
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

	return sendJSON[AppInfoResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appInfos/%s", appInfoID), Payload: request})
}
