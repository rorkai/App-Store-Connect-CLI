package asc

// In-app purchase localization requests/responses.
type InAppPurchaseLocalizationCreateAttributes struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Description string `json:"description,omitempty"`
}

type InAppPurchaseLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type InAppPurchaseLocalizationCreateRelationships struct {
	InAppPurchaseV2 Relationship `json:"inAppPurchaseV2"`
}

type InAppPurchaseLocalizationCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    InAppPurchaseLocalizationCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseLocalizationCreateRelationships `json:"relationships"`
}

type InAppPurchaseLocalizationCreateRequest struct {
	Data InAppPurchaseLocalizationCreateData `json:"data"`
}

type InAppPurchaseLocalizationUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *InAppPurchaseLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseLocalizationUpdateRequest struct {
	Data InAppPurchaseLocalizationUpdateData `json:"data"`
}

type InAppPurchaseLocalizationResponse = SingleResponse[InAppPurchaseLocalizationAttributes]

// In-app purchase images.
type InAppPurchaseImageAttributes struct {
	FileSize           int64             `json:"fileSize,omitempty"`
	FileName           string            `json:"fileName,omitempty"`
	SourceFileChecksum string            `json:"sourceFileChecksum,omitempty"`
	AssetToken         string            `json:"assetToken,omitempty"`
	ImageAsset         *ImageAsset       `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation `json:"uploadOperations,omitempty"`
	State              string            `json:"state,omitempty"`
}

type (
	InAppPurchaseImagesResponse = Response[InAppPurchaseImageAttributes]
	InAppPurchaseImageResponse  = SingleResponse[InAppPurchaseImageAttributes]
)

type InAppPurchaseImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

type InAppPurchaseImageRelationships struct {
	InAppPurchase Relationship `json:"inAppPurchase"`
}

type InAppPurchaseImageCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    InAppPurchaseImageCreateAttributes `json:"attributes"`
	Relationships InAppPurchaseImageRelationships    `json:"relationships"`
}

type InAppPurchaseImageCreateRequest struct {
	Data InAppPurchaseImageCreateData `json:"data"`
}

type InAppPurchaseImageUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

type InAppPurchaseImageUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *InAppPurchaseImageUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseImageUpdateRequest struct {
	Data InAppPurchaseImageUpdateData `json:"data"`
}

// In-app purchase App Store review screenshots.
type InAppPurchaseAppStoreReviewScreenshotAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	AssetType          string              `json:"assetType,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

type InAppPurchaseAppStoreReviewScreenshotResponse = SingleResponse[InAppPurchaseAppStoreReviewScreenshotAttributes]

type InAppPurchaseAppStoreReviewScreenshotCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

type InAppPurchaseAppStoreReviewScreenshotRelationships struct {
	InAppPurchaseV2 Relationship `json:"inAppPurchaseV2"`
}

type InAppPurchaseAppStoreReviewScreenshotCreateData struct {
	Type          ResourceType                                          `json:"type"`
	Attributes    InAppPurchaseAppStoreReviewScreenshotCreateAttributes `json:"attributes"`
	Relationships InAppPurchaseAppStoreReviewScreenshotRelationships    `json:"relationships"`
}

type InAppPurchaseAppStoreReviewScreenshotCreateRequest struct {
	Data InAppPurchaseAppStoreReviewScreenshotCreateData `json:"data"`
}

type InAppPurchaseAppStoreReviewScreenshotUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

type InAppPurchaseAppStoreReviewScreenshotUpdateData struct {
	Type       ResourceType                                           `json:"type"`
	ID         string                                                 `json:"id"`
	Attributes *InAppPurchaseAppStoreReviewScreenshotUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseAppStoreReviewScreenshotUpdateRequest struct {
	Data InAppPurchaseAppStoreReviewScreenshotUpdateData `json:"data"`
}

// In-app purchase availability.
type InAppPurchaseAvailabilityAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

type InAppPurchaseAvailabilityResponse = SingleResponse[InAppPurchaseAvailabilityAttributes]

type InAppPurchaseAvailabilityCreateAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

type InAppPurchaseAvailabilityCreateRelationships struct {
	InAppPurchase        Relationship     `json:"inAppPurchase"`
	AvailableTerritories RelationshipList `json:"availableTerritories"`
}

type InAppPurchaseAvailabilityCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    InAppPurchaseAvailabilityCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseAvailabilityCreateRelationships `json:"relationships"`
}

type InAppPurchaseAvailabilityCreateRequest struct {
	Data InAppPurchaseAvailabilityCreateData `json:"data"`
}

// In-app purchase content.
type InAppPurchaseContentAttributes struct {
	FileName         string `json:"fileName,omitempty"`
	FileSize         int64  `json:"fileSize,omitempty"`
	URL              string `json:"url,omitempty"`
	LastModifiedDate string `json:"lastModifiedDate,omitempty"`
}

type InAppPurchaseContentResponse = SingleResponse[InAppPurchaseContentAttributes]

// In-app purchase price points and schedules.
type InAppPurchasePricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
}

type InAppPurchasePricePointsResponse = Response[InAppPurchasePricePointAttributes]

type InAppPurchasePriceScheduleAttributes struct{}

type InAppPurchasePriceScheduleResponse = SingleResponse[InAppPurchasePriceScheduleAttributes]

type InAppPurchasePriceAttributes struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
	Manual    bool   `json:"manual,omitempty"`
}

type InAppPurchasePricesResponse = Response[InAppPurchasePriceAttributes]

type InAppPurchasePriceScheduleCreateAttributes struct {
	BaseTerritoryID string
	Prices          []InAppPurchasePriceSchedulePrice
}

type InAppPurchasePriceSchedulePrice struct {
	PricePointID string
	StartDate    string
	EndDate      string
}

type InAppPurchasePriceScheduleCreateRequest struct {
	Data     InAppPurchasePriceScheduleCreateData     `json:"data"`
	Included []InAppPurchasePriceInlineCreateResource `json:"included,omitempty"`
}

type InAppPurchasePriceScheduleCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Relationships InAppPurchasePriceScheduleCreateRelationships `json:"relationships"`
}

type InAppPurchasePriceScheduleCreateRelationships struct {
	InAppPurchase Relationship     `json:"inAppPurchase"`
	BaseTerritory Relationship     `json:"baseTerritory"`
	ManualPrices  RelationshipList `json:"manualPrices"`
}

type InAppPurchasePriceInlineCreateResource struct {
	Type          ResourceType                          `json:"type"`
	ID            string                                `json:"id,omitempty"`
	Attributes    InAppPurchasePriceInlineAttributes    `json:"attributes"`
	Relationships InAppPurchasePriceInlineRelationships `json:"relationships"`
}

type InAppPurchasePriceInlineAttributes struct {
	StartDate string `json:"startDate,omitempty"`
	EndDate   string `json:"endDate,omitempty"`
}

type InAppPurchasePriceInlineRelationships struct {
	InAppPurchaseV2         Relationship `json:"inAppPurchaseV2"`
	InAppPurchasePricePoint Relationship `json:"inAppPurchasePricePoint"`
}

// In-app purchase offer codes.
type InAppPurchaseOfferCodeAttributes struct {
	Name                  string   `json:"name,omitempty"`
	CustomerEligibilities []string `json:"customerEligibilities,omitempty"`
	ProductionCodeCount   int      `json:"productionCodeCount,omitempty"`
	SandboxCodeCount      int      `json:"sandboxCodeCount,omitempty"`
	Active                bool     `json:"active,omitempty"`
}

type (
	InAppPurchaseOfferCodesResponse = Response[InAppPurchaseOfferCodeAttributes]
	InAppPurchaseOfferCodeResponse  = SingleResponse[InAppPurchaseOfferCodeAttributes]
)

type InAppPurchaseOfferCodeCreateAttributes struct {
	Name                  string
	CustomerEligibilities []string
	Prices                []InAppPurchaseOfferCodePrice
}

type InAppPurchaseOfferCodePrice struct {
	TerritoryID  string
	PricePointID string
}

type InAppPurchaseOfferCodeCreateRequest struct {
	Data     InAppPurchaseOfferCodeCreateData              `json:"data"`
	Included []InAppPurchaseOfferPriceInlineCreateResource `json:"included,omitempty"`
}

type InAppPurchaseOfferCodeCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    InAppPurchaseOfferCodeCreateRequestAttributes `json:"attributes"`
	Relationships InAppPurchaseOfferCodeCreateRelationships     `json:"relationships"`
}

type InAppPurchaseOfferCodeCreateRequestAttributes struct {
	Name                  string   `json:"name"`
	CustomerEligibilities []string `json:"customerEligibilities"`
}

type InAppPurchaseOfferCodeCreateRelationships struct {
	InAppPurchase Relationship     `json:"inAppPurchase"`
	Prices        RelationshipList `json:"prices"`
}

type InAppPurchaseOfferCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

type InAppPurchaseOfferCodeUpdateRequest struct {
	Data InAppPurchaseOfferCodeUpdateData `json:"data"`
}

type InAppPurchaseOfferCodeUpdateData struct {
	Type       ResourceType                            `json:"type"`
	ID         string                                  `json:"id"`
	Attributes *InAppPurchaseOfferCodeUpdateAttributes `json:"attributes,omitempty"`
}

type InAppPurchaseOfferPriceAttributes struct{}

type InAppPurchaseOfferPricesResponse = Response[InAppPurchaseOfferPriceAttributes]

type InAppPurchaseOfferPriceInlineCreateResource struct {
	Type          ResourceType                               `json:"type"`
	ID            string                                     `json:"id,omitempty"`
	Relationships InAppPurchaseOfferPriceInlineRelationships `json:"relationships"`
}

type InAppPurchaseOfferPriceInlineRelationships struct {
	Territory  Relationship `json:"territory"`
	PricePoint Relationship `json:"pricePoint"`
}

// Offer code custom codes and one-time use codes.
type InAppPurchaseOfferCodeCustomCodeAttributes struct {
	CustomCode     string `json:"customCode,omitempty"`
	NumberOfCodes  int    `json:"numberOfCodes,omitempty"`
	CreatedDate    string `json:"createdDate,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	Active         bool   `json:"active,omitempty"`
}

type (
	InAppPurchaseOfferCodeCustomCodesResponse = Response[InAppPurchaseOfferCodeCustomCodeAttributes]
	InAppPurchaseOfferCodeCustomCodeResponse  = SingleResponse[InAppPurchaseOfferCodeCustomCodeAttributes]
)

// InAppPurchaseOfferCodeCustomCodeCreateAttributes describes attributes for creating custom codes.
type InAppPurchaseOfferCodeCustomCodeCreateAttributes struct {
	CustomCode     string  `json:"customCode"`
	NumberOfCodes  int     `json:"numberOfCodes"`
	ExpirationDate *string `json:"expirationDate,omitempty"`
}

// InAppPurchaseOfferCodeCustomCodeCreateRelationships describes relationships for creating custom codes.
type InAppPurchaseOfferCodeCustomCodeCreateRelationships struct {
	OfferCode Relationship `json:"offerCode"`
}

// InAppPurchaseOfferCodeCustomCodeCreateData is the data portion of a create request.
type InAppPurchaseOfferCodeCustomCodeCreateData struct {
	Type          ResourceType                                        `json:"type"`
	Attributes    InAppPurchaseOfferCodeCustomCodeCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseOfferCodeCustomCodeCreateRelationships `json:"relationships"`
}

// InAppPurchaseOfferCodeCustomCodeCreateRequest is a request to create custom codes.
type InAppPurchaseOfferCodeCustomCodeCreateRequest struct {
	Data InAppPurchaseOfferCodeCustomCodeCreateData `json:"data"`
}

// InAppPurchaseOfferCodeCustomCodeUpdateAttributes describes attributes for updating custom codes.
type InAppPurchaseOfferCodeCustomCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// InAppPurchaseOfferCodeCustomCodeUpdateData is the data portion of an update request.
type InAppPurchaseOfferCodeCustomCodeUpdateData struct {
	Type       ResourceType                                      `json:"type"`
	ID         string                                            `json:"id"`
	Attributes *InAppPurchaseOfferCodeCustomCodeUpdateAttributes `json:"attributes,omitempty"`
}

// InAppPurchaseOfferCodeCustomCodeUpdateRequest is a request to update custom codes.
type InAppPurchaseOfferCodeCustomCodeUpdateRequest struct {
	Data InAppPurchaseOfferCodeCustomCodeUpdateData `json:"data"`
}

type InAppPurchaseOfferCodeOneTimeUseCodeAttributes struct {
	NumberOfCodes  int    `json:"numberOfCodes,omitempty"`
	CreatedDate    string `json:"createdDate,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	Active         bool   `json:"active,omitempty"`
	Environment    string `json:"environment,omitempty"`
}

type (
	InAppPurchaseOfferCodeOneTimeUseCodesResponse = Response[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]
	InAppPurchaseOfferCodeOneTimeUseCodeResponse  = SingleResponse[InAppPurchaseOfferCodeOneTimeUseCodeAttributes]
)

// InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes describes attributes for generating one-time use codes.
type InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes struct {
	NumberOfCodes  int    `json:"numberOfCodes"`
	ExpirationDate string `json:"expirationDate"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships describes relationships for creating one-time use codes.
type InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships struct {
	OfferCode Relationship `json:"offerCode"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeCreateData is the data portion of a create request.
type InAppPurchaseOfferCodeOneTimeUseCodeCreateData struct {
	Type          ResourceType                                            `json:"type"`
	Attributes    InAppPurchaseOfferCodeOneTimeUseCodeCreateAttributes    `json:"attributes"`
	Relationships InAppPurchaseOfferCodeOneTimeUseCodeCreateRelationships `json:"relationships"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest is a request to generate one-time use codes.
type InAppPurchaseOfferCodeOneTimeUseCodeCreateRequest struct {
	Data InAppPurchaseOfferCodeOneTimeUseCodeCreateData `json:"data"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes describes attributes for updating one-time use codes.
type InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeUpdateData is the data portion of an update request.
type InAppPurchaseOfferCodeOneTimeUseCodeUpdateData struct {
	Type       ResourceType                                          `json:"type"`
	ID         string                                                `json:"id"`
	Attributes *InAppPurchaseOfferCodeOneTimeUseCodeUpdateAttributes `json:"attributes,omitempty"`
}

// InAppPurchaseOfferCodeOneTimeUseCodeUpdateRequest is a request to update one-time use codes.
type InAppPurchaseOfferCodeOneTimeUseCodeUpdateRequest struct {
	Data InAppPurchaseOfferCodeOneTimeUseCodeUpdateData `json:"data"`
}

// In-app purchase submissions.
type InAppPurchaseSubmissionResponse = SingleResponse[InAppPurchaseSubmissionAttributes]

type InAppPurchaseSubmissionAttributes struct{}

type InAppPurchaseSubmissionCreateRequest struct {
	Data InAppPurchaseSubmissionCreateData `json:"data"`
}

type InAppPurchaseSubmissionCreateData struct {
	Type          ResourceType                         `json:"type"`
	Relationships InAppPurchaseSubmissionRelationships `json:"relationships"`
}

type InAppPurchaseSubmissionRelationships struct {
	InAppPurchaseV2 Relationship `json:"inAppPurchaseV2"`
}
