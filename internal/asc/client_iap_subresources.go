package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// CreateInAppPurchaseLocalization creates a localization for an in-app purchase.
//
// Deprecated: Use CreateInAppPurchaseLocalizationV2 with an in-app purchase version ID.
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

	return sendJSON[InAppPurchaseLocalizationResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseLocalizations", Payload: payload})
}

// GetInAppPurchaseAppStoreReviewScreenshotForIAP retrieves the review screenshot for an IAP.
func (c *Client) GetInAppPurchaseAppStoreReviewScreenshotForIAP(ctx context.Context, iapID string, opts ...IAPReviewScreenshotOption) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	query := applyOptions(opts)
	return getJSON[InAppPurchaseAppStoreReviewScreenshotResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s/appStoreReviewScreenshot", iapID), Query: buildIAPReviewScreenshotQuery(query)})
}

// GetInAppPurchaseAppStoreReviewScreenshot retrieves a review screenshot by ID.
func (c *Client) GetInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string, opts ...IAPReviewScreenshotOption) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}

	query := applyOptions(opts)
	return getJSON[InAppPurchaseAppStoreReviewScreenshotResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID), Query: buildIAPReviewScreenshotQuery(query)})
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

	return sendJSON[InAppPurchaseAppStoreReviewScreenshotResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseAppStoreReviewScreenshots", Payload: payload})
}

// UpdateInAppPurchaseAppStoreReviewScreenshot updates a review screenshot.
func (c *Client) UpdateInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string, attrs InAppPurchaseAppStoreReviewScreenshotUpdateAttributes) (*InAppPurchaseAppStoreReviewScreenshotResponse, error) {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return nil, fmt.Errorf("screenshotID is required")
	}
	if attrs.SourceFileChecksum == nil && attrs.Uploaded == nil {
		return nil, fmt.Errorf("at least one attribute is required")
	}
	if attrs.SourceFileChecksum != nil {
		checksum := strings.TrimSpace(*attrs.SourceFileChecksum)
		if checksum == "" {
			return nil, fmt.Errorf("sourceFileChecksum must not be empty")
		}
		attrs.SourceFileChecksum = &checksum
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

	return sendJSON[InAppPurchaseAppStoreReviewScreenshotResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID), Payload: payload})
}

// DeleteInAppPurchaseAppStoreReviewScreenshot deletes a review screenshot by ID.
func (c *Client) DeleteInAppPurchaseAppStoreReviewScreenshot(ctx context.Context, screenshotID string) error {
	screenshotID = strings.TrimSpace(screenshotID)
	if screenshotID == "" {
		return fmt.Errorf("screenshotID is required")
	}
	return sendNoContent(ctx, c, http.MethodDelete, fmt.Sprintf("/v1/inAppPurchaseAppStoreReviewScreenshots/%s", screenshotID), nil)
}

// GetInAppPurchaseAvailability retrieves availability for an in-app purchase.
func (c *Client) GetInAppPurchaseAvailability(ctx context.Context, iapID string) (*InAppPurchaseAvailabilityResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	return getJSON[InAppPurchaseAvailabilityResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s/inAppPurchaseAvailability", iapID)})
}

// GetInAppPurchaseAvailabilityByID retrieves an in-app purchase availability by ID.
func (c *Client) GetInAppPurchaseAvailabilityByID(ctx context.Context, availabilityID string) (*InAppPurchaseAvailabilityResponse, error) {
	availabilityID = strings.TrimSpace(availabilityID)
	if availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}
	return getJSON[InAppPurchaseAvailabilityResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseAvailabilities/%s", availabilityID)})
}

// GetInAppPurchaseAvailabilityAvailableTerritories lists available territories for an availability.
func (c *Client) GetInAppPurchaseAvailabilityAvailableTerritories(ctx context.Context, availabilityID string, opts ...IAPAvailabilityTerritoriesOption) (*TerritoriesResponse, error) {
	query := applyOptions(opts)

	availabilityID = strings.TrimSpace(availabilityID)
	if query.nextURL == "" && availabilityID == "" {
		return nil, fmt.Errorf("availabilityID is required")
	}
	return getJSON[TerritoriesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchaseAvailabilities/%s/availableTerritories", availabilityID),
		Query:       buildIAPAvailabilityTerritoriesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-availability-territories",
	})
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

	return sendJSON[InAppPurchaseAvailabilityResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseAvailabilities", Payload: payload})
}

// GetInAppPurchaseContent retrieves the content resource for an IAP.
func (c *Client) GetInAppPurchaseContent(ctx context.Context, iapID string, opts ...IAPContentOption) (*InAppPurchaseContentResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	query := applyOptions(opts)
	return getJSON[InAppPurchaseContentResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s/content", iapID), Query: buildIAPContentQuery(query)})
}

// GetInAppPurchaseContentByID retrieves an in-app purchase content resource by ID.
func (c *Client) GetInAppPurchaseContentByID(ctx context.Context, contentID string, opts ...IAPContentOption) (*InAppPurchaseContentResponse, error) {
	contentID = strings.TrimSpace(contentID)
	if contentID == "" {
		return nil, fmt.Errorf("contentID is required")
	}

	query := applyOptions(opts)
	return getJSON[InAppPurchaseContentResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseContents/%s", contentID), Query: buildIAPContentQuery(query)})
}

// GetInAppPurchasePricePoints retrieves price points for an in-app purchase.
func (c *Client) GetInAppPurchasePricePoints(ctx context.Context, iapID string, opts ...IAPPricePointsOption) (*InAppPurchasePricePointsResponse, error) {
	query := applyOptions(opts)

	iapID = strings.TrimSpace(iapID)
	if query.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	return getJSON[InAppPurchasePricePointsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/inAppPurchases/%s/pricePoints", iapID),
		Query:       buildIAPPricePointsQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-price-points",
	})
}

// GetInAppPurchasePricePointEqualizations retrieves equalized price points for a price point.
func (c *Client) GetInAppPurchasePricePointEqualizations(ctx context.Context, pricePointID string, opts ...IAPPricePointsOption) (*InAppPurchasePricePointsResponse, error) {
	query := applyOptions(opts)

	pricePointID = strings.TrimSpace(pricePointID)
	if query.nextURL == "" && pricePointID == "" {
		return nil, fmt.Errorf("pricePointID is required")
	}
	return getJSON[InAppPurchasePricePointsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchasePricePoints/%s/equalizations", pricePointID),
		Query:       buildIAPPricePointsQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-price-point-equalizations",
	})
}

// GetInAppPurchasePriceSchedule retrieves the price schedule for an IAP.
func (c *Client) GetInAppPurchasePriceSchedule(ctx context.Context, iapID string, opts ...IAPPriceScheduleOption) (*InAppPurchasePriceScheduleResponse, error) {
	query := applyOptions(opts)

	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	return getJSON[InAppPurchasePriceScheduleResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s/iapPriceSchedule", iapID), Query: buildIAPPriceScheduleQuery(query)})
}

// GetInAppPurchasePriceScheduleByID retrieves an in-app purchase price schedule by ID.
func (c *Client) GetInAppPurchasePriceScheduleByID(ctx context.Context, scheduleID string, opts ...IAPPriceScheduleOption) (*InAppPurchasePriceScheduleResponse, error) {
	query := applyOptions(opts)

	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}
	return getJSON[InAppPurchasePriceScheduleResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s", scheduleID), Query: buildIAPPriceScheduleQuery(query)})
}

// GetInAppPurchasePromotedPurchase retrieves the promoted purchase for an in-app purchase.
func (c *Client) GetInAppPurchasePromotedPurchase(ctx context.Context, iapID string, opts ...PromotedPurchaseGetOption) (*PromotedPurchaseResponse, error) {
	iapID = strings.TrimSpace(iapID)
	if iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}

	query := applyOptions(opts)
	return getJSON[PromotedPurchaseResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v2/inAppPurchases/%s/promotedPurchase", iapID), Query: buildPromotedPurchaseGetQuery(query)})
}

// GetInAppPurchasePriceScheduleBaseTerritory retrieves base territory for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleBaseTerritory(ctx context.Context, scheduleID string) (*TerritoryResponse, error) {
	scheduleID = strings.TrimSpace(scheduleID)
	if scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}
	return getJSON[TerritoryResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/baseTerritory", scheduleID)})
}

// GetInAppPurchasePriceScheduleManualPrices retrieves manual prices for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleManualPrices(ctx context.Context, scheduleID string, opts ...IAPPriceSchedulePricesOption) (*InAppPurchasePricesResponse, error) {
	query := applyOptions(opts)

	scheduleID = strings.TrimSpace(scheduleID)
	if query.nextURL == "" && scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}
	return getJSON[InAppPurchasePricesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/manualPrices", scheduleID),
		Query:       buildIAPPriceSchedulePricesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-manual-prices",
	})
}

// GetInAppPurchasePriceScheduleAutomaticPrices retrieves automatic prices for a schedule.
func (c *Client) GetInAppPurchasePriceScheduleAutomaticPrices(ctx context.Context, scheduleID string, opts ...IAPPriceSchedulePricesOption) (*InAppPurchasePricesResponse, error) {
	query := applyOptions(opts)

	scheduleID = strings.TrimSpace(scheduleID)
	if query.nextURL == "" && scheduleID == "" {
		return nil, fmt.Errorf("scheduleID is required")
	}
	return getJSON[InAppPurchasePricesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchasePriceSchedules/%s/automaticPrices", scheduleID),
		Query:       buildIAPPriceSchedulePricesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-automatic-prices",
	})
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

	return sendJSON[InAppPurchasePriceScheduleResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchasePriceSchedules", Payload: payload})
}

// GetInAppPurchaseOfferCodes retrieves offer codes for an in-app purchase.
func (c *Client) GetInAppPurchaseOfferCodes(ctx context.Context, iapID string, opts ...IAPOfferCodesOption) (*InAppPurchaseOfferCodesResponse, error) {
	query := applyOptions(opts)

	iapID = strings.TrimSpace(iapID)
	if query.nextURL == "" && iapID == "" {
		return nil, fmt.Errorf("iapID is required")
	}
	return getJSON[InAppPurchaseOfferCodesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v2/inAppPurchases/%s/offerCodes", iapID),
		Query:       buildIAPOfferCodesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-offer-codes",
	})
}

// GetInAppPurchaseOfferCodeCustomCodes retrieves custom codes for an offer code.
func (c *Client) GetInAppPurchaseOfferCodeCustomCodes(ctx context.Context, offerCodeID string, opts ...IAPOfferCodeCustomCodesOption) (*InAppPurchaseOfferCodeCustomCodesResponse, error) {
	query := applyOptions(opts)

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	return getJSON[InAppPurchaseOfferCodeCustomCodesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/customCodes", offerCodeID),
		Query:       buildIAPOfferCodeCustomCodesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-offer-code-custom-codes",
	})
}

// GetInAppPurchaseOfferCodeCustomCode retrieves a custom code by ID.
func (c *Client) GetInAppPurchaseOfferCodeCustomCode(ctx context.Context, customCodeID string) (*InAppPurchaseOfferCodeCustomCodeResponse, error) {
	customCodeID = strings.TrimSpace(customCodeID)
	if customCodeID == "" {
		return nil, fmt.Errorf("customCodeID is required")
	}
	return getJSON[InAppPurchaseOfferCodeCustomCodeResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodeCustomCodes/%s", customCodeID)})
}

// CreateInAppPurchaseOfferCodeCustomCode creates a custom code for an offer code.
func (c *Client) CreateInAppPurchaseOfferCodeCustomCode(ctx context.Context, req InAppPurchaseOfferCodeCustomCodeCreateRequest) (*InAppPurchaseOfferCodeCustomCodeResponse, error) {
	offerCodeID := strings.TrimSpace(req.Data.Relationships.OfferCode.Data.ID)
	if offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	customCode := strings.TrimSpace(req.Data.Attributes.CustomCode)
	if customCode == "" {
		return nil, fmt.Errorf("customCode is required")
	}
	if req.Data.Attributes.NumberOfCodes <= 0 {
		return nil, fmt.Errorf("numberOfCodes must be greater than 0")
	}
	req.Data.Relationships.OfferCode.Data.ID = offerCodeID
	req.Data.Attributes.CustomCode = customCode

	return sendJSON[InAppPurchaseOfferCodeCustomCodeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseOfferCodeCustomCodes", Payload: req})
}

// UpdateInAppPurchaseOfferCodeCustomCode updates a custom code by ID.
func (c *Client) UpdateInAppPurchaseOfferCodeCustomCode(ctx context.Context, customCodeID string, attrs InAppPurchaseOfferCodeCustomCodeUpdateAttributes) (*InAppPurchaseOfferCodeCustomCodeResponse, error) {
	customCodeID = strings.TrimSpace(customCodeID)
	if customCodeID == "" {
		return nil, fmt.Errorf("customCodeID is required")
	}

	payload := InAppPurchaseOfferCodeCustomCodeUpdateRequest{
		Data: InAppPurchaseOfferCodeCustomCodeUpdateData{
			Type: ResourceTypeInAppPurchaseOfferCodeCustomCodes,
			ID:   customCodeID,
		},
	}
	if attrs.Active != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[InAppPurchaseOfferCodeCustomCodeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodeCustomCodes/%s", customCodeID), Payload: payload})
}

// GetInAppPurchaseOfferCodeOneTimeUseCodes retrieves one-time use codes for an offer code.
func (c *Client) GetInAppPurchaseOfferCodeOneTimeUseCodes(ctx context.Context, offerCodeID string, opts ...IAPOfferCodeOneTimeUseCodesOption) (*InAppPurchaseOfferCodeOneTimeUseCodesResponse, error) {
	query := applyOptions(opts)

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	return getJSON[InAppPurchaseOfferCodeOneTimeUseCodesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/oneTimeUseCodes", offerCodeID),
		Query:       buildIAPOfferCodeOneTimeUseCodesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-offer-code-one-time-use-codes",
	})
}

// GetInAppPurchaseOfferCodeOneTimeUseCode retrieves a one-time use code batch by ID.
func (c *Client) GetInAppPurchaseOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string) (*InAppPurchaseOfferCodeOneTimeUseCodeResponse, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	if oneTimeUseCodeID == "" {
		return nil, fmt.Errorf("oneTimeUseCodeID is required")
	}
	return getJSON[InAppPurchaseOfferCodeOneTimeUseCodeResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodeOneTimeUseCodes/%s", oneTimeUseCodeID)})
}

// CreateInAppPurchaseOfferCodeOneTimeUseCode generates a new one-time use code batch.
func (c *Client) CreateInAppPurchaseOfferCodeOneTimeUseCode(ctx context.Context, req InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest) (*InAppPurchaseOfferCodeOneTimeUseCodeResponse, error) {
	offerCodeID := strings.TrimSpace(req.Data.Relationships.OfferCode.Data.ID)
	if offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	if req.Data.Attributes.NumberOfCodes <= 0 {
		return nil, fmt.Errorf("numberOfCodes must be greater than 0")
	}
	expirationDate := strings.TrimSpace(req.Data.Attributes.ExpirationDate)
	environment := strings.TrimSpace(strings.ToUpper(req.Data.Attributes.Environment))
	if expirationDate == "" {
		return nil, fmt.Errorf("expirationDate is required")
	}
	req.Data.Relationships.OfferCode.Data.ID = offerCodeID
	req.Data.Attributes.ExpirationDate = expirationDate
	req.Data.Attributes.Environment = environment

	return sendJSON[InAppPurchaseOfferCodeOneTimeUseCodeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseOfferCodeOneTimeUseCodes", Payload: req})
}

// UpdateInAppPurchaseOfferCodeOneTimeUseCode updates a one-time use code batch by ID.
func (c *Client) UpdateInAppPurchaseOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string, attrs InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes) (*InAppPurchaseOfferCodeOneTimeUseCodeResponse, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	if oneTimeUseCodeID == "" {
		return nil, fmt.Errorf("oneTimeUseCodeID is required")
	}

	payload := InAppPurchaseOfferCodeOneTimeUseCodeUpdateRequest{
		Data: InAppPurchaseOfferCodeOneTimeUseCodeUpdateData{
			Type: ResourceTypeInAppPurchaseOfferCodeOneTimeUseCodes,
			ID:   oneTimeUseCodeID,
		},
	}
	if attrs.Active != nil {
		payload.Data.Attributes = &attrs
	}

	return sendJSON[InAppPurchaseOfferCodeOneTimeUseCodeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodeOneTimeUseCodes/%s", oneTimeUseCodeID), Payload: payload})
}

// GetInAppPurchaseOfferCodePrices retrieves prices for an offer code.
func (c *Client) GetInAppPurchaseOfferCodePrices(ctx context.Context, offerCodeID string, opts ...IAPOfferCodePricesOption) (*InAppPurchaseOfferPricesResponse, error) {
	query := applyOptions(opts)

	offerCodeID = strings.TrimSpace(offerCodeID)
	if query.nextURL == "" && offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	return getJSON[InAppPurchaseOfferPricesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s/prices", offerCodeID),
		Query:       buildIAPOfferCodePricesQuery(query),
		Next:        query.nextURL,
		NextContext: "in-app-purchase-offer-code-prices",
	})
}

// GetInAppPurchaseOfferCode retrieves an offer code by ID.
func (c *Client) GetInAppPurchaseOfferCode(ctx context.Context, offerCodeID string) (*InAppPurchaseOfferCodeResponse, error) {
	offerCodeID = strings.TrimSpace(offerCodeID)
	if offerCodeID == "" {
		return nil, fmt.Errorf("offerCodeID is required")
	}
	return getJSON[InAppPurchaseOfferCodeResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s", offerCodeID)})
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
		relationships := InAppPurchaseOfferPriceInlineRelationships{
			Territory: Relationship{
				Data: ResourceData{
					Type: ResourceTypeTerritories,
					ID:   territoryID,
				},
			},
		}
		if !strings.EqualFold(pricePointID, "FREE") {
			relationships.PricePoint = &Relationship{
				Data: ResourceData{
					Type: ResourceTypeInAppPurchasePricePoints,
					ID:   pricePointID,
				},
			}
		}
		included = append(included, InAppPurchaseOfferPriceInlineCreateResource{
			Type:          ResourceTypeInAppPurchaseOfferPrices,
			ID:            resourceID,
			Relationships: relationships,
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

	return sendJSON[InAppPurchaseOfferCodeResponse](ctx, c, writeRequest{Method: http.MethodPost, Path: "/v1/inAppPurchaseOfferCodes", Payload: payload})
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

	return sendJSON[InAppPurchaseOfferCodeResponse](ctx, c, writeRequest{Method: http.MethodPatch, Path: fmt.Sprintf("/v1/inAppPurchaseOfferCodes/%s", offerCodeID), Payload: payload})
}
