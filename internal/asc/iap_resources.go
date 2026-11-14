package asc

// In-app purchase localization requests/responses.
type InAppPurchaseLocalizationCreateAttributes struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Description string `json:"description,omitempty"`
}

type InAppPurchaseLocalizationUpdateAttributes struct {
	Name        *NullableString `json:"name,omitempty"`
	Description *NullableString `json:"description,omitempty"`
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

	api apiAttributes
}

type InAppPurchaseAppStoreReviewScreenshotCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// In-app purchase availability.
type InAppPurchaseAvailabilityAttributes struct {
	AvailableInNewTerritories bool `json:"availableInNewTerritories"`
}

// In-app purchase content.
type InAppPurchaseContentAttributes struct {
	FileName         string `json:"fileName,omitempty"`
	FileSize         int64  `json:"fileSize,omitempty"`
	URL              string `json:"url,omitempty"`
	LastModifiedDate string `json:"lastModifiedDate,omitempty"`
}

// In-app purchase price points and schedules.
type InAppPurchasePricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
}

type InAppPurchasePricePointsResponse = Response[InAppPurchasePricePointAttributes]

type InAppPurchasePriceScheduleAttributes struct{}

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

type InAppPurchaseOfferPriceAttributes struct{}

type InAppPurchaseOfferPricesResponse = Response[InAppPurchaseOfferPriceAttributes]

type InAppPurchaseOfferPriceInlineCreateResource struct {
	Type          ResourceType                               `json:"type"`
	ID            string                                     `json:"id,omitempty"`
	Relationships InAppPurchaseOfferPriceInlineRelationships `json:"relationships"`
}

type InAppPurchaseOfferPriceInlineRelationships struct {
	Territory  Relationship  `json:"territory"`
	PricePoint *Relationship `json:"pricePoint,omitempty"`
}

type (
	InAppPurchaseOfferCodeCustomCodesResponse = Response[InAppPurchaseOfferCodeCustomCodeAttributes]
	InAppPurchaseOfferCodeCustomCodeResponse  = SingleResponse[InAppPurchaseOfferCodeCustomCodeAttributes]
)

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
	Environment    string `json:"environment,omitempty"`
}
