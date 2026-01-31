package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CreateInAppPurchaseLocalization creates a localization for an in-app purchase.
func (c *Client) CreateInAppPurchaseLocalization(ctx context.Context, iapID string, attrs InAppPurchaseLocalizationCreateAttributes) (*InAppPurchaseLocalizationResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	name := strings.TrimSpace(attrs.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	locale := strings.TrimSpace(attrs.Locale)
	if locale == "" {
		return nil, fmt.Errorf("locale is required")
	}
	description := strings.TrimSpace(attrs.Description)

	payload := InAppPurchaseLocalizationCreateRequest{
		Data: InAppPurchaseLocalizationCreateData{
			Type: ResourceTypeInAppPurchaseLocalizations,
			Attributes: InAppPurchaseLocalizationCreateAttributes{
				Name:        name,
				Locale:      locale,
				Description: description,
			},
			Relationships: InAppPurchaseLocalizationCreateRelationships{
				InAppPurchaseV2: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateInAppPurchaseLocalization updates an IAP localization by ID.
func (c *Client) UpdateInAppPurchaseLocalization(ctx context.Context, localizationID string, attrs InAppPurchaseLocalizationUpdateAttributes) (*InAppPurchaseLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := InAppPurchaseLocalizationUpdateRequest{
		Data: InAppPurchaseLocalizationUpdateData{
			Type: ResourceTypeInAppPurchaseLocalizations,
			ID:   localizationID,
		},
	}
	if attrs.Name != nil || attrs.Description != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/inAppPurchaseLocalizations/%s", localizationID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteInAppPurchaseLocalization deletes an IAP localization by ID.
func (c *Client) DeleteInAppPurchaseLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v1/inAppPurchaseLocalizations/%s", localizationID), nil)
	return err
}

// GetInAppPurchaseImages retrieves images for an in-app purchase.
func (c *Client) GetInAppPurchaseImages(ctx context.Context, iapID string, opts ...IAPImagesOption) (*InAppPurchaseImagesResponse, error) {
	query := &iapImagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	iapID = strings.TrimSpace(iapID)
	if query.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/images", iapID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-images: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPImagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseImagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseImage retrieves an in-app purchase image by ID.
func (c *Client) GetInAppPurchaseImage(ctx context.Context, imageID string) (*InAppPurchaseImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseImages/%s", imageID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseImage creates an image upload reservation.
func (c *Client) CreateInAppPurchaseImage(ctx context.Context, iapID, fileName string, fileSize int64) (*InAppPurchaseImageResponse, error) {
	iapID = strings.TrimSpace(iapID)
	fileName = strings.TrimSpace(fileName)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}

	payload := InAppPurchaseImageCreateRequest{
		Data: InAppPurchaseImageCreateData{
			Type: ResourceTypeInAppPurchaseImages,
			Attributes: InAppPurchaseImageCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: InAppPurchaseImageRelationships{
				InAppPurchase: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseImages", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateInAppPurchaseImage updates an in-app purchase image.
func (c *Client) UpdateInAppPurchaseImage(ctx context.Context, imageID string, attrs InAppPurchaseImageUpdateAttributes) (*InAppPurchaseImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	payload := InAppPurchaseImageUpdateRequest{
		Data: InAppPurchaseImageUpdateData{
			Type: ResourceTypeInAppPurchaseImages,
			ID:   imageID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/inAppPurchaseImages/%s", imageID), body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteInAppPurchaseImage deletes an in-app purchase image by ID.
func (c *Client) DeleteInAppPurchaseImage(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("imageID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v1/inAppPurchaseImages/%s", imageID), nil)
	return err
}

// GetInAppPurchaseAppStoreReviewScreenshotForIAP retrieves the review screenshot for an IAP.
func (c *Client) GetInAppPurchaseAppStoreReviewScreenshotForIAP(ctx context.Context, iapID string) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/appStoreReviewScreenshot", iapID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseAppStoreReviewScreenshot retrieves a review screenshot by ID.
func (c *Client) GetInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseAppStoreReviewScreenshot creates a review screenshot upload reservation.
func (c *Client) CreateInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, iapID, fileName string, fileSize int64) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	iapID = strings.TrimSpace(iapID)
	fileName = strings.TrimSpace(fileName)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	if fileName == "" {
		return nil, fmt.Errorf("fileName is required")
	}
	if fileSize <= 0 {
		return nil, fmt.Errorf("fileSize is required")
	}

	payload := InAppPurchaseAppStoreReviewScreenshotCreateRequest{
		Data: InAppPurchaseAppStoreReviewScreenshotCreateData{
			Type: ResourceTypeInAppPurchaseAppStoreReviewScreenshots,
			Attributes: InAppPurchaseAppStoreReviewScreenshotCreateAttributes{
				FileName: fileName,
				FileSize: fileSize,
			},
			Relationships: InAppPurchaseAppStoreReviewScreenshotRelationships{
				InAppPurchaseV2: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseAppStoreReviewScreenshots", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateInAppPurchaseAppStoreReviewScreenshot updates a review screenshot.
func (c *Client) UpdateInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string, attrs InAppPurchaseAppStoreReviewScreenshotUpdateAttributes) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}

	payload := InAppPurchaseAppStoreReviewScreenshotUpdateRequest{
		Data: InAppPurchaseAppStoreReviewScreenshotUpdateData{
			Type: ResourceTypeInAppPurchaseAppStoreReviewScreenshots,
			ID:   screenshotID,
		},
	}
	if attrs.SourceFileChecksum != nil || attrs.Uploaded != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID), body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAppStoreReviewScreenshotResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteInAppPurchaseAppStoreReviewScreenshot deletes a review screenshot by ID.
func (c *Client) DeleteInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string) error {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return fmt.Errorf("screenshotID is required")
	}
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID), nil)
	return err
}

// GetInAppPurchaseAvailability retrieves availability for an in-app purchase.
func (c *Client) GetInAppPurchaseAvailability(ctx context.Context, iapID string) (*InAppPurchaseAvailabilityResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/inAppPurchaseAvailability", iapID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseAvailabilityByID retrieves an in-app purchase availability by ID.
func (c *Client) GetInAppPurchaseAvailabilityByID(ctx context.Context, availabilityID string) (*InAppPurchaseAvailabilityResponse, error) {
	availabilityID = strings.TrimSpace(availabilityID)
	if availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseAvailabilities/%s", availabilityID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseAvailabilityAvailableTerritories lists available territories for an availability.
func (c *Client) GetInAppPurchaseAvailabilityAvailableTerritories(ctx context.Context, availabilityID string, opts ...IAPAvailabilityTerritoriesOption) (*TerritoriesResponse, error) {
	query := &iapAvailabilityTerritoriesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	availabilityID = strings.TrimSpace(availabilityID)
	if query.nextURL == "" && availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseAvailabilities/%s/availableTerritories", availabilityID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-availability-territories: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPAvailabilityTerritoriesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoriesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseAvailability creates or updates availability for an in-app purchase.
func (c *Client) CreateInAppPurchaseAvailability(ctx context.Context, iapID string, availableInNewTerritories bool, territories []string) (*InAppPurchaseAvailabilityResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	if len(territories) == 0 {
		return nil, fmt.Errorf("at least one territory is required")
	}

	relationshipData := make([]ResourceData, 0, len(territories))
	for _, territoryID := range territories {
		territoryID = strings.ToUpper(strings.TrimSpace(territoryID))
		if territoryID == "" {
			return nil, fmt.Errorf("territory ID is required")
		}
		relationshipData = append(relationshipData, ResourceData{
			Type: ResourceTypeTerritories,
			ID:   territoryID,
		})
	}

	payload := InAppPurchaseAvailabilityCreateRequest{
		Data: InAppPurchaseAvailabilityCreateData{
			Type:       ResourceTypeInAppPurchaseAvailabilities,
			Attributes: InAppPurchaseAvailabilityCreateAttributes{AvailableInNewTerritories: availableInNewTerritories},
			Relationships: InAppPurchaseAvailabilityCreateRelationships{
				InAppPurchase: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
				AvailableTerritories: RelationshipList{Data: relationshipData},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseAvailabilities", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseAvailabilityResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseContent retrieves the content resource for an IAP.
func (c *Client) GetInAppPurchaseContent(ctx context.Context, iapID string) (*InAppPurchaseContentResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/content", iapID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseContentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseContentByID retrieves an in-app purchase content resource by ID.
func (c *Client) GetInAppPurchaseContentByID(ctx context.Context, contentID string) (*InAppPurchaseContentResponse, error) {
	contentID = strings.TrimSpace(contentID)
	if contentID == "" {
		return nil, fmt.Errorf("contentID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseContents/%s", contentID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseContentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePricePoints retrieves price points for an in-app purchase.
func (c *Client) GetInAppPurchasePricePoints(ctx context.Context, iapID string, opts ...IAPPricePointsOption) (*InAppPurchasePricePointsResponse, error) {
	query := &iapPricePointsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	iapID = strings.TrimSpace(iapID)
	if query.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/pricePoints", iapID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-price-points: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPPricePointsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePricePointsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePricePointEqualizations retrieves equalized price points for a price point.
func (c *Client) GetInAppPurchasePricePointEqualizations(ctx context.Context, pricePointID string) (*InAppPurchasePricePointsResponse, error) {
	pricePointID = strings.TrimSpace(pricePointID)
	if pricePointID == "" {
		return nil, fmt.Errorf("pricePointID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchasePricePoints/%s/equalizations", pricePointID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePricePointsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePriceSchedule retrieves the price schedule for an IAP.
func (c *Client) GetInAppPurchasePriceSchedule(ctx context.Context, iapID string) (*InAppPurchasePriceScheduleResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/iapPriceSchedule", iapID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePriceScheduleByID retrieves an in-app purchase price schedule by ID.
func (c *Client) GetInAppPurchasePriceScheduleByID(ctx context.Context, scheduleID string) (*InAppPurchasePriceScheduleResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s", scheduleID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePromotedPurchase retrieves the promoted purchase for an in-app purchase.
func (c *Client) GetInAppPurchasePromotedPurchase(ctx context.Context, iapID string) (*PromotedPurchaseResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/promotedPurchase", iapID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response PromotedPurchaseResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePriceScheduleBaseTerritory retrieves base territory for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleBaseTerritory(ctx context.Context, scheduleID string) (*TerritoryResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/baseTerritory", scheduleID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response TerritoryResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePriceScheduleManualPrices retrieves manual prices for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleManualPrices(ctx context.Context, scheduleID string, opts ...IAPPriceSchedulePricesOption) (*InAppPurchasePricesResponse, error) {
	query := &iapPriceSchedulePricesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	scheduleID = strings.TrimSpace(scheduleID)
	if query.nextURL == "" && scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/manualPrices", scheduleID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-manual-prices: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPPriceSchedulePricesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchasePriceScheduleAutomaticPrices retrieves automatic prices for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleAutomaticPrices(ctx context.Context, scheduleID string, opts ...IAPPriceSchedulePricesOption) (*InAppPurchasePricesResponse, error) {
	query := &iapPriceSchedulePricesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	scheduleID = strings.TrimSpace(scheduleID)
	if query.nextURL == "" && scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/automaticPrices", scheduleID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-automatic-prices: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPPriceSchedulePricesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchasePriceSchedule creates an IAP price schedule.
func (c *Client) CreateInAppPurchasePriceSchedule(ctx context.Context, iapID string, attrs InAppPurchasePriceScheduleCreateAttributes) (*InAppPurchasePriceScheduleResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	baseTerritoryID := strings.ToUpper(strings.TrimSpace(attrs.BaseTerritoryID))
	if baseTerritoryID == "" {
		return nil, fmt.Errorf("base territory ID is required")
	}
	if len(attrs.Prices) == 0 {
		return nil, fmt.Errorf("at least one price is required")
	}

	included := make([]InAppPurchasePriceInlineCreateResource, 0, len(attrs.Prices))
	relationshipData := make([]ResourceData, 0, len(attrs.Prices))
	for idx, price := range attrs.Prices {
		pricePointID := strings.TrimSpace(price.PricePointID)
		if pricePointID == "" {
			return nil, fmt.Errorf("price point ID is required")
		}
		resourceID := fmt.Sprintf("${local-manual-price-%d}", idx+1)
		relationshipData = append(relationshipData, ResourceData{
			Type: ResourceTypeInAppPurchasePrices,
			ID:   resourceID,
		})
		included = append(included, InAppPurchasePriceInlineCreateResource{
			Type: ResourceTypeInAppPurchasePrices,
			ID:   resourceID,
			Attributes: InAppPurchasePriceInlineAttributes{
				StartDate: strings.TrimSpace(price.StartDate),
				EndDate:   strings.TrimSpace(price.EndDate),
			},
			Relationships: InAppPurchasePriceInlineRelationships{
				InAppPurchaseV2: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
				InAppPurchasePricePoint: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchasePricePoints,
						ID:   pricePointID,
					},
				},
			},
		})
	}

	payload := InAppPurchasePriceScheduleCreateRequest{
		Data: InAppPurchasePriceScheduleCreateData{
			Type: ResourceTypeInAppPurchasePriceSchedules,
			Relationships: InAppPurchasePriceScheduleCreateRelationships{
				InAppPurchase: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
				BaseTerritory: Relationship{
					Data: ResourceData{
						Type: ResourceTypeTerritories,
						ID:   baseTerritoryID,
					},
				},
				ManualPrices: RelationshipList{Data: relationshipData},
			},
		},
		Included: included,
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchasePriceSchedules", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchasePriceScheduleResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodes retrieves offer codes for an in-app purchase.
func (c *Client) GetInAppPurchaseOfferCodes(ctx context.Context, iapID string, opts ...IAPOfferCodesOption) (*InAppPurchaseOfferCodesResponse, error) {
	query := &iapOfferCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	iapID = strings.TrimSpace(iapID)
	if query.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	path := fmt.Sprintf("/v2/inAppPurchases/%s/offerCodes", iapID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-offer-codes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPOfferCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodeCustomCodes retrieves custom codes for an offer code.
func (c *Client) GetInAppPurchaseOfferCodeCustomCodes(ctx context.Context, offerCodeID string, opts ...IAPOfferCodeCustomCodesOption) (*InAppPurchaseOfferCodeCustomCodesResponse, error) {
	query := &iapOfferCodeCustomCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/customCodes", offerCodeID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-offer-code-custom-codes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPOfferCodeCustomCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeCustomCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodeCustomCode retrieves a custom code by ID.
func (c *Client) GetInAppPurchaseOfferCodeCustomCode(ctx context.Context, customCodeID string) (*InAppPurchaseOfferCodeCustomCodeResponse, error) {
	customCodeID = strings.TrimSpace(customCodeID)
	if customCodeID == "" {
		return nil, fmt.Errorf("customCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodeCustomCodes/%s", customCodeID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeCustomCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodeOneTimeUseCodes retrieves one-time use codes for an offer code.
func (c *Client) GetInAppPurchaseOfferCodeOneTimeUseCodes(ctx context.Context, offerCodeID string, opts ...IAPOfferCodeOneTimeUseCodesOption) (*InAppPurchaseOfferCodeOneTimeUseCodesResponse, error) {
	query := &iapOfferCodeOneTimeUseCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/oneTimeUseCodes", offerCodeID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-offer-code-one-time-use-codes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPOfferCodeOneTimeUseCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeOneTimeUseCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodeOneTimeUseCode retrieves a one-time use code batch by ID.
func (c *Client) GetInAppPurchaseOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string) (*InAppPurchaseOfferCodeOneTimeUseCodeResponse, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	if oneTimeUseCodeID == "" {
		return nil, fmt.Errorf("oneTimeUseCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodeOneTimeUseCodes/%s", oneTimeUseCodeID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeOneTimeUseCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCodePrices retrieves prices for an offer code.
func (c *Client) GetInAppPurchaseOfferCodePrices(ctx context.Context, offerCodeID string, opts ...IAPOfferCodePricesOption) (*InAppPurchaseOfferPricesResponse, error) {
	query := &iapOfferCodePricesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/prices", offerCodeID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("in-app-purchase-offer-code-prices: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildIAPOfferCodePricesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferPricesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetInAppPurchaseOfferCode retrieves an offer code by ID.
func (c *Client) GetInAppPurchaseOfferCode(ctx context.Context, offerCodeID string) (*InAppPurchaseOfferCodeResponse, error) {
	offerCodeID = strings.TrimSpace(offerCodeID)
	if offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}

	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s", offerCodeID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseOfferCode creates an offer code.
func (c *Client) CreateInAppPurchaseOfferCode(ctx context.Context, iapID string, attrs InAppPurchaseOfferCodeCreateAttributes) (*InAppPurchaseOfferCodeResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	name := strings.TrimSpace(attrs.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(attrs.CustomerEligibilities) == 0 {
		return nil, fmt.Errorf("customer eligibilities are required")
	}
	if len(attrs.Prices) == 0 {
		return nil, fmt.Errorf("at least one price is required")
	}

	included := make([]InAppPurchaseOfferPriceInlineCreateResource, 0, len(attrs.Prices))
	relationshipData := make([]ResourceData, 0, len(attrs.Prices))
	for idx, price := range attrs.Prices {
		territoryID := strings.ToUpper(strings.TrimSpace(price.TerritoryID))
		pricePointID := strings.TrimSpace(price.PricePointID)
		if territoryID == "" {
			return nil, fmt.Errorf("territory ID is required")
		}
		if pricePointID == "" {
			return nil, fmt.Errorf("price point ID is required")
		}
		resourceID := fmt.Sprintf("${local-price-%d}", idx+1)
		relationshipData = append(relationshipData, ResourceData{
			Type: ResourceTypeInAppPurchaseOfferPrices,
			ID:   resourceID,
		})
		included = append(included, InAppPurchaseOfferPriceInlineCreateResource{
			Type: ResourceTypeInAppPurchaseOfferPrices,
			ID:   resourceID,
			Relationships: InAppPurchaseOfferPriceInlineRelationships{
				Territory: Relationship{
					Data: ResourceData{
						Type: ResourceTypeTerritories,
						ID:   territoryID,
					},
				},
				PricePoint: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchasePricePoints,
						ID:   pricePointID,
					},
				},
			},
		})
	}

	payload := InAppPurchaseOfferCodeCreateRequest{
		Data: InAppPurchaseOfferCodeCreateData{
			Type: ResourceTypeInAppPurchaseOfferCodes,
			Attributes: InAppPurchaseOfferCodeCreateRequestAttributes{
				Name:                  name,
				CustomerEligibilities: attrs.CustomerEligibilities,
			},
			Relationships: InAppPurchaseOfferCodeCreateRelationships{
				InAppPurchase: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
				Prices: RelationshipList{
					Data: relationshipData,
				},
			},
		},
		Included: included,
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseOfferCodes", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateInAppPurchaseOfferCode updates an offer code by ID.
func (c *Client) UpdateInAppPurchaseOfferCode(ctx context.Context, offerCodeID string, attrs InAppPurchaseOfferCodeUpdateAttributes) (*InAppPurchaseOfferCodeResponse, error) {
	offerCodeID = strings.TrimSpace(offerCodeID)
	if offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}

	payload := InAppPurchaseOfferCodeUpdateRequest{
		Data: InAppPurchaseOfferCodeUpdateData{
			Type: ResourceTypeInAppPurchaseOfferCodes,
			ID:   offerCodeID,
		},
	}
	if attrs.Active != nil {
		payload.Data.Attributes = &attrs
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s", offerCodeID), body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseOfferCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateInAppPurchaseSubmission submits an IAP for review.
func (c *Client) CreateInAppPurchaseSubmission(ctx context.Context, iapID string) (*InAppPurchaseSubmissionResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	payload := InAppPurchaseSubmissionCreateRequest{
		Data: InAppPurchaseSubmissionCreateData{
			Type: ResourceTypeInAppPurchaseSubmissions,
			Relationships: InAppPurchaseSubmissionRelationships{
				InAppPurchaseV2: Relationship{
					Data: ResourceData{
						Type: ResourceTypeInAppPurchases,
						ID:   iapID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/inAppPurchaseSubmissions", body)
	if err != nil {
		return nil, err
	}

	var response InAppPurchaseSubmissionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}
