package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AppStoreReviewDetailAttributes describes App Store review details.
type AppStoreReviewDetailAttributes struct {
	ContactFirstName    string `json:"contactFirstName,omitempty"`
	ContactLastName     string `json:"contactLastName,omitempty"`
	ContactPhone        string `json:"contactPhone,omitempty"`
	ContactEmail        string `json:"contactEmail,omitempty"`
	DemoAccountName     string `json:"demoAccountName,omitempty"`
	DemoAccountPassword string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired bool   `json:"demoAccountRequired,omitempty"`
	Notes               string `json:"notes,omitempty"`

	api apiAttributes
}

// AppStoreReviewDetailResponse is the response from review detail endpoints.
type AppStoreReviewDetailResponse = SingleResponse[AppStoreReviewDetailAttributes]

// AppStoreReviewDetailCreateAttributes describes create attributes.
type AppStoreReviewDetailCreateAttributes struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

// AppStoreReviewDetailCreateRelationships describes relationships for create requests.
type AppStoreReviewDetailCreateRelationships struct {
	AppStoreVersion *Relationship `json:"appStoreVersion"`
}

// AppStoreReviewDetailCreateData is the data portion of a create request.
type AppStoreReviewDetailCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    *AppStoreReviewDetailCreateAttributes    `json:"attributes,omitempty"`
	Relationships *AppStoreReviewDetailCreateRelationships `json:"relationships"`
}

// AppStoreReviewDetailCreateRequest is a request to create review details.
type AppStoreReviewDetailCreateRequest struct {
	Data AppStoreReviewDetailCreateData `json:"data"`
}

// AppStoreReviewDetailUpdateAttributes describes update attributes.
type AppStoreReviewDetailUpdateAttributes struct {
	ContactFirstName    *string `json:"contactFirstName,omitempty"`
	ContactLastName     *string `json:"contactLastName,omitempty"`
	ContactPhone        *string `json:"contactPhone,omitempty"`
	ContactEmail        *string `json:"contactEmail,omitempty"`
	DemoAccountName     *string `json:"demoAccountName,omitempty"`
	DemoAccountPassword *string `json:"demoAccountPassword,omitempty"`
	DemoAccountRequired *bool   `json:"demoAccountRequired,omitempty"`
	Notes               *string `json:"notes,omitempty"`
}

// AppStoreReviewDetailUpdateData is the data portion of an update request.
type AppStoreReviewDetailUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *AppStoreReviewDetailUpdateAttributes `json:"attributes,omitempty"`
}

// AppStoreReviewDetailUpdateRequest is a request to update review details.
type AppStoreReviewDetailUpdateRequest struct {
	Data AppStoreReviewDetailUpdateData `json:"data"`
}

// GetAppStoreReviewDetail retrieves a review detail by ID.
func (c *Client) GetAppStoreReviewDetail(ctx context.Context, detailID string) (*AppStoreReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/appStoreReviewDetails/%s", detailID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppStoreReviewDetailForVersion retrieves the review detail for an app store version.
func (c *Client) GetAppStoreReviewDetailForVersion(ctx context.Context, versionID string) (*AppStoreReviewDetailResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/appStoreReviewDetail", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppStoreReviewDetail creates review details for a version.
func (c *Client) CreateAppStoreReviewDetail(ctx context.Context, versionID string, attrs *AppStoreReviewDetailCreateAttributes) (*AppStoreReviewDetailResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}
	if attrs != nil {
		if err := validateSecretMutationValue(attrs.DemoAccountPassword); err != nil {
			return nil, err
		}
	}

	payload := AppStoreReviewDetailCreateRequest{
		Data: AppStoreReviewDetailCreateData{
			Type:       ResourceTypeAppStoreReviewDetails,
			Attributes: attrs,
			Relationships: &AppStoreReviewDetailCreateRelationships{
				AppStoreVersion: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   versionID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/appStoreReviewDetails", body)
	if err != nil {
		var password *string
		if attrs != nil {
			password = attrs.DemoAccountPassword
		}
		return nil, redactSubmittedSecretFromError(err, password)
	}

	var response AppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppStoreReviewDetail updates review details by ID.
func (c *Client) UpdateAppStoreReviewDetail(ctx context.Context, detailID string, attrs AppStoreReviewDetailUpdateAttributes) (*AppStoreReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}
	if err := validateSecretMutationValue(attrs.DemoAccountPassword); err != nil {
		return nil, err
	}

	payload := AppStoreReviewDetailUpdateRequest{
		Data: AppStoreReviewDetailUpdateData{
			Type:       ResourceTypeAppStoreReviewDetails,
			ID:         detailID,
			Attributes: &attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "PATCH", fmt.Sprintf("/v1/appStoreReviewDetails/%s", detailID), body)
	if err != nil {
		return nil, redactSubmittedSecretFromError(err, attrs.DemoAccountPassword)
	}

	var response AppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
