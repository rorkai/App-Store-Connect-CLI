package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CreateSubscriptionVersion creates a version for a subscription.
func (c *Client) CreateSubscriptionVersion(ctx context.Context, subscriptionID string) (*SubscriptionVersionResponse, error) {
	subscriptionID = strings.TrimSpace(subscriptionID)
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	payload := SubscriptionVersionCreateRequest{Data: SubscriptionVersionCreateData{
		Type: ResourceTypeSubscriptionVersions,
		Relationships: &SubscriptionVersionRelationships{Subscription: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptions,
			ID:   subscriptionID,
		}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v1/subscriptionVersions", body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersion retrieves a subscription version by ID.
func (c *Client) GetSubscriptionVersion(ctx context.Context, versionID string, opts ...SubscriptionVersionOption) (*SubscriptionVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	query := &subscriptionVersionQuery{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v1/subscriptionVersions/%s", versionID)
	if queryString := buildSubscriptionVersionQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersions retrieves versions for a subscription.
func (c *Client) GetSubscriptionVersions(ctx context.Context, subscriptionID string, opts ...SubscriptionVersionsOption) (*SubscriptionVersionsResponse, error) {
	query := &subscriptionVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	subscriptionID = strings.TrimSpace(subscriptionID)
	if query.nextURL == "" && subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	path := fmt.Sprintf("/v1/subscriptions/%s/versions", subscriptionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersionsRelationships retrieves version linkages for a subscription.
func (c *Client) GetSubscriptionVersionsRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, subscriptionID, "/v1/subscriptions/%s/relationships/versions", "subscriptionVersionsRelationships", opts...)
}

// GetSubscriptionVersionLocalizations retrieves localizations for a version.
func (c *Client) GetSubscriptionVersionLocalizations(ctx context.Context, versionID string, opts ...SubscriptionVersionLocalizationsOption) (*SubscriptionLocalizationsV2Response, error) {
	query := &subscriptionVersionLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	path := fmt.Sprintf("/v1/subscriptionVersions/%s/localizations", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionVersionLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionVersionLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionLocalizationsV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersionLocalizationsRelationships retrieves localization linkages for a version.
func (c *Client) GetSubscriptionVersionLocalizationsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, versionID, "/v1/subscriptionVersions/%s/relationships/localizations", "subscriptionVersionLocalizationsRelationships", opts...)
}

// GetSubscriptionVersionImage retrieves the singular image for a version.
func (c *Client) GetSubscriptionVersionImage(ctx context.Context, versionID string, opts ...SubscriptionImageV2Option) (*SubscriptionImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	query := &subscriptionImageV2Query{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v1/subscriptionVersions/%s/image", versionID)
	if queryString := buildSubscriptionImageV2Query(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersionImageRelationship retrieves the singular image linkage.
func (c *Client) GetSubscriptionVersionImageRelationship(ctx context.Context, versionID string) (*SubscriptionVersionImageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	data, err := c.do(ctx, http.MethodGet, fmt.Sprintf("/v1/subscriptionVersions/%s/relationships/image", versionID), nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionVersionImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersionImages retrieves all images for a version.
func (c *Client) GetSubscriptionVersionImages(ctx context.Context, versionID string, opts ...SubscriptionVersionImagesOption) (*SubscriptionImagesV2Response, error) {
	query := &subscriptionVersionImagesQuery{}
	for _, opt := range opts {
		opt(query)
	}
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	path := fmt.Sprintf("/v1/subscriptionVersions/%s/images", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("subscriptionVersionImages: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionVersionImagesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionImagesV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionVersionImagesRelationships retrieves image linkages for a version.
func (c *Client) GetSubscriptionVersionImagesRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, versionID, "/v1/subscriptionVersions/%s/relationships/images", "subscriptionVersionImagesRelationships", opts...)
}

func (c *Client) getSubscriptionVersionLinkages(ctx context.Context, ownerID, pathFormat, label string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}
	ownerID = strings.TrimSpace(ownerID)
	if query.nextURL == "" && ownerID == "" {
		return nil, fmt.Errorf("owner ID is required")
	}
	path := fmt.Sprintf(pathFormat, ownerID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// CreateSubscriptionLocalizationV2 creates a version-scoped localization.
func (c *Client) CreateSubscriptionLocalizationV2(ctx context.Context, versionID string, attrs SubscriptionLocalizationV2CreateAttributes) (*SubscriptionLocalizationV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if strings.TrimSpace(attrs.Name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(attrs.Locale) == "" {
		return nil, fmt.Errorf("locale is required")
	}
	payload := SubscriptionLocalizationV2CreateRequest{Data: SubscriptionLocalizationV2CreateData{
		Type:       ResourceTypeSubscriptionLocalizations,
		Attributes: attrs,
		Relationships: &SubscriptionVersionRelationship{Version: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionVersions,
			ID:   versionID,
		}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v2/subscriptionLocalizations", body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionLocalizationV2 retrieves a version-scoped localization.
func (c *Client) GetSubscriptionLocalizationV2(ctx context.Context, localizationID string, opts ...SubscriptionLocalizationV2Option) (*SubscriptionLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localization ID is required")
	}
	query := &subscriptionLocalizationV2Query{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID)
	if queryString := buildSubscriptionLocalizationV2Query(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionLocalizationV2 updates a version-scoped localization.
func (c *Client) UpdateSubscriptionLocalizationV2(ctx context.Context, localizationID string, attrs SubscriptionLocalizationV2UpdateAttributes) (*SubscriptionLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localization ID is required")
	}
	payload := SubscriptionLocalizationV2UpdateRequest{Data: SubscriptionLocalizationV2UpdateData{
		Type:       ResourceTypeSubscriptionLocalizations,
		ID:         localizationID,
		Attributes: attrs,
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID), body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionLocalizationV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionLocalizationV2 deletes a version-scoped localization.
func (c *Client) DeleteSubscriptionLocalizationV2(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localization ID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID), nil)
	return err
}

// CreateSubscriptionImageV2 reserves a version-scoped image upload.
func (c *Client) CreateSubscriptionImageV2(ctx context.Context, versionID, fileName string, fileSize int64) (*SubscriptionImageV2Response, error) {
	versionID = strings.TrimSpace(versionID)
	fileName = strings.TrimSpace(fileName)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("file name is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("file size must be greater than zero")
	}
	payload := SubscriptionImageV2CreateRequest{Data: SubscriptionImageV2CreateData{
		Type: ResourceTypeSubscriptionImages,
		Attributes: SubscriptionImageV2CreateAttributes{
			FileName: fileName,
			FileSize: fileSize,
		},
		Relationships: &SubscriptionVersionRelationship{Version: &Relationship{Data: ResourceData{
			Type: ResourceTypeSubscriptionVersions,
			ID:   versionID,
		}}},
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/v2/subscriptionImages", body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// GetSubscriptionImageV2 retrieves a version-scoped image.
func (c *Client) GetSubscriptionImageV2(ctx context.Context, imageID string, opts ...SubscriptionImageV2Option) (*SubscriptionImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("image ID is required")
	}
	query := &subscriptionImageV2Query{}
	for _, opt := range opts {
		opt(query)
	}
	path := fmt.Sprintf("/v2/subscriptionImages/%s", imageID)
	if queryString := buildSubscriptionImageV2Query(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var response SubscriptionImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// UpdateSubscriptionImageV2 commits or updates a version-scoped image.
func (c *Client) UpdateSubscriptionImageV2(ctx context.Context, imageID string, attrs SubscriptionImageV2UpdateAttributes) (*SubscriptionImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("image ID is required")
	}
	payload := SubscriptionImageV2UpdateRequest{Data: SubscriptionImageV2UpdateData{
		Type:       ResourceTypeSubscriptionImages,
		ID:         imageID,
		Attributes: attrs,
	}}
	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v2/subscriptionImages/%s", imageID), body)
	if err != nil {
		return nil, err
	}
	var response SubscriptionImageV2Response
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return &response, nil
}

// DeleteSubscriptionImageV2 deletes a version-scoped image.
func (c *Client) DeleteSubscriptionImageV2(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("image ID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v2/subscriptionImages/%s", imageID), nil)
	return err
}
