package asc

import (
	"context"
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
	return sendJSON[SubscriptionVersionResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/subscriptionVersions", Payload: payload})
}

// GetSubscriptionVersion retrieves a subscription version by ID.
func (c *Client) GetSubscriptionVersion(ctx context.Context, versionID string, opts ...SubscriptionVersionOption) (*SubscriptionVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	query := applyOptions(opts)
	return getJSON[SubscriptionVersionResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionVersions/%s", versionID),
		Query: buildSubscriptionVersionQuery(query),
	})
}

// GetSubscriptionVersions retrieves versions for a subscription.
func (c *Client) GetSubscriptionVersions(ctx context.Context, subscriptionID string, opts ...SubscriptionVersionsOption) (*SubscriptionVersionsResponse, error) {
	query := applyOptions(opts)
	subscriptionID = strings.TrimSpace(subscriptionID)
	if query.nextURL == "" && subscriptionID == "" {
		return nil, fmt.Errorf("subscription ID is required")
	}
	return getJSON[SubscriptionVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptions/%s/versions", subscriptionID),
		Query:       buildSubscriptionVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "subscriptionVersions",
	})
}

// GetSubscriptionVersionsRelationships retrieves version linkages for a subscription.
func (c *Client) GetSubscriptionVersionsRelationships(ctx context.Context, subscriptionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, subscriptionID, "/v1/subscriptions/%s/relationships/versions", "subscriptionVersionsRelationships", opts...)
}

// GetSubscriptionVersionLocalizations retrieves localizations for a version.
func (c *Client) GetSubscriptionVersionLocalizations(ctx context.Context, versionID string, opts ...SubscriptionVersionLocalizationsOption) (*SubscriptionLocalizationsV2Response, error) {
	query := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	return getJSON[SubscriptionLocalizationsV2Response](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionVersions/%s/localizations", versionID),
		Query:       buildSubscriptionVersionLocalizationsQuery(query),
		Next:        query.nextURL,
		NextContext: "subscriptionVersionLocalizations",
	})
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
	query := applyOptions(opts)
	return getJSON[SubscriptionImageV2Response](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/subscriptionVersions/%s/image", versionID),
		Query: buildSubscriptionImageV2Query(query),
	})
}

// GetSubscriptionVersionImageRelationship retrieves the singular image linkage.
func (c *Client) GetSubscriptionVersionImageRelationship(ctx context.Context, versionID string) (*SubscriptionVersionImageLinkageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	return getJSON[SubscriptionVersionImageLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/subscriptionVersions/%s/relationships/image", versionID)})
}

// GetSubscriptionVersionImages retrieves all images for a version.
func (c *Client) GetSubscriptionVersionImages(ctx context.Context, versionID string, opts ...SubscriptionVersionImagesOption) (*SubscriptionImagesV2Response, error) {
	query := applyOptions(opts)
	versionID = strings.TrimSpace(versionID)
	if query.nextURL == "" && versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}
	return getJSON[SubscriptionImagesV2Response](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/subscriptionVersions/%s/images", versionID),
		Query:       buildSubscriptionVersionImagesQuery(query),
		Next:        query.nextURL,
		NextContext: "subscriptionVersionImages",
	})
}

// GetSubscriptionVersionImagesRelationships retrieves image linkages for a version.
func (c *Client) GetSubscriptionVersionImagesRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getSubscriptionVersionLinkages(ctx, versionID, "/v1/subscriptionVersions/%s/relationships/images", "subscriptionVersionImagesRelationships", opts...)
}

func (c *Client) getSubscriptionVersionLinkages(ctx context.Context, ownerID, pathFormat, label string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	query := applyOptions(opts)
	ownerID = strings.TrimSpace(ownerID)
	if query.nextURL == "" && ownerID == "" {
		return nil, fmt.Errorf("owner ID is required")
	}
	return getJSON[LinkagesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf(pathFormat, ownerID),
		Query:       buildLinkagesQuery(query),
		Next:        query.nextURL,
		NextContext: label,
	})
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
	return sendJSON[SubscriptionLocalizationV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/subscriptionLocalizations", Payload: payload})
}

// GetSubscriptionLocalizationV2 retrieves a version-scoped localization.
func (c *Client) GetSubscriptionLocalizationV2(ctx context.Context, localizationID string, opts ...SubscriptionLocalizationV2Option) (*SubscriptionLocalizationV2Response, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localization ID is required")
	}
	query := applyOptions(opts)
	return getJSON[SubscriptionLocalizationV2Response](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID),
		Query: buildSubscriptionLocalizationV2Query(query),
	})
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
	return sendJSON[SubscriptionLocalizationV2Response](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID), Payload: payload})
}

// DeleteSubscriptionLocalizationV2 deletes a version-scoped localization.
func (c *Client) DeleteSubscriptionLocalizationV2(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localization ID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/subscriptionLocalizations/%s", localizationID), nil)
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
	return sendJSON[SubscriptionImageV2Response](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v2/subscriptionImages", Payload: payload})
}

// GetSubscriptionImageV2 retrieves a version-scoped image.
func (c *Client) GetSubscriptionImageV2(ctx context.Context, imageID string, opts ...SubscriptionImageV2Option) (*SubscriptionImageV2Response, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("image ID is required")
	}
	query := applyOptions(opts)
	return getJSON[SubscriptionImageV2Response](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v2/subscriptionImages/%s", imageID),
		Query: buildSubscriptionImageV2Query(query),
	})
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
	return sendJSON[SubscriptionImageV2Response](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v2/subscriptionImages/%s", imageID), Payload: payload})
}

// DeleteSubscriptionImageV2 deletes a version-scoped image.
func (c *Client) DeleteSubscriptionImageV2(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("image ID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v2/subscriptionImages/%s", imageID), nil)
}
