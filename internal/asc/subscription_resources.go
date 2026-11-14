package asc

// SubscriptionLocalizationCreateAttributes describes attributes for creating a localization.
type SubscriptionLocalizationCreateAttributes struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Description string `json:"description,omitempty"`
}

// SubscriptionImageAttributes describes a subscription image resource.
type SubscriptionImageAttributes struct {
	FileSize           int64             `json:"fileSize,omitempty"`
	FileName           string            `json:"fileName,omitempty"`
	SourceFileChecksum string            `json:"sourceFileChecksum,omitempty"`
	AssetToken         string            `json:"assetToken,omitempty"`
	ImageAsset         *ImageAsset       `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation `json:"uploadOperations,omitempty"`
	State              string            `json:"state,omitempty"`
}

// SubscriptionIntroductoryOfferAttributes describes a subscription introductory offer.
type SubscriptionIntroductoryOfferAttributes struct {
	StartDate                  string                    `json:"startDate,omitempty"`
	EndDate                    string                    `json:"endDate,omitempty"`
	Duration                   SubscriptionOfferDuration `json:"duration,omitempty"`
	OfferMode                  SubscriptionOfferMode     `json:"offerMode,omitempty"`
	NumberOfPeriods            int                       `json:"numberOfPeriods,omitempty"`
	TargetSubscriptionPlanType SubscriptionPlanType      `json:"targetSubscriptionPlanType,omitempty"`
}

// SubscriptionIntroductoryOfferCreateAttributes describes attributes for creating an introductory offer.
type SubscriptionIntroductoryOfferCreateAttributes struct {
	StartDate                  string                    `json:"startDate,omitempty"`
	EndDate                    string                    `json:"endDate,omitempty"`
	Duration                   SubscriptionOfferDuration `json:"duration"`
	OfferMode                  SubscriptionOfferMode     `json:"offerMode"`
	NumberOfPeriods            int                       `json:"numberOfPeriods"`
	TargetSubscriptionPlanType SubscriptionPlanType      `json:"targetSubscriptionPlanType,omitempty"`
}

// SubscriptionIntroductoryOfferUpdateData is the data portion of an introductory offer update request.
type SubscriptionIntroductoryOfferUpdateData struct {
	Type       ResourceType                                  `json:"type"`
	ID         string                                        `json:"id"`
	Attributes SubscriptionIntroductoryOfferUpdateAttributes `json:"attributes"`
}

// SubscriptionPromotionalOfferAttributes describes a subscription promotional offer.
type SubscriptionPromotionalOfferAttributes struct {
	Duration                   SubscriptionOfferDuration `json:"duration,omitempty"`
	Name                       string                    `json:"name,omitempty"`
	NumberOfPeriods            int                       `json:"numberOfPeriods,omitempty"`
	OfferCode                  string                    `json:"offerCode,omitempty"`
	OfferMode                  SubscriptionOfferMode     `json:"offerMode,omitempty"`
	TargetSubscriptionPlanType SubscriptionPlanType      `json:"targetSubscriptionPlanType,omitempty"`
}

// SubscriptionPromotionalOfferCreateAttributes describes attributes for creating a promotional offer.
type SubscriptionPromotionalOfferCreateAttributes struct {
	Duration                   SubscriptionOfferDuration `json:"duration"`
	Name                       string                    `json:"name"`
	NumberOfPeriods            int                       `json:"numberOfPeriods"`
	OfferCode                  string                    `json:"offerCode"`
	OfferMode                  SubscriptionOfferMode     `json:"offerMode"`
	TargetSubscriptionPlanType SubscriptionPlanType      `json:"targetSubscriptionPlanType,omitempty"`
}

// SubscriptionPromotionalOfferCreateRequest is a request to create a promotional offer.
type SubscriptionPromotionalOfferCreateRequest struct {
	Data     SubscriptionPromotionalOfferCreateData          `json:"data"`
	Included []SubscriptionPromotionalOfferPriceInlineCreate `json:"included,omitempty"`
}

// SubscriptionPromotionalOfferPrice describes a promotional offer price input.
type SubscriptionPromotionalOfferPrice struct {
	ID           string
	TerritoryID  string
	PricePointID string
}

// SubscriptionPromotionalOfferPriceRelationships describes inline promotional offer price relationships.
type SubscriptionPromotionalOfferPriceRelationships struct {
	Territory              *Relationship `json:"territory,omitempty"`
	SubscriptionPricePoint *Relationship `json:"subscriptionPricePoint,omitempty"`
}

// SubscriptionPromotionalOfferPriceInlineCreate describes an inline promotional offer price resource.
type SubscriptionPromotionalOfferPriceInlineCreate struct {
	Type          ResourceType                                   `json:"type"`
	ID            string                                         `json:"id"`
	Relationships SubscriptionPromotionalOfferPriceRelationships `json:"relationships"`
}

// SubscriptionPromotionalOfferUpdateRelationships describes relationships for promotional offer updates.
type SubscriptionPromotionalOfferUpdateRelationships struct {
	Prices RelationshipList `json:"prices"`
}

// SubscriptionPromotionalOfferUpdateData is the data portion of a promotional offer update request.
type SubscriptionPromotionalOfferUpdateData struct {
	Type          ResourceType                                     `json:"type"`
	ID            string                                           `json:"id"`
	Relationships *SubscriptionPromotionalOfferUpdateRelationships `json:"relationships,omitempty"`
}

// SubscriptionPromotionalOfferPriceAttributes describes promotional offer price resources.
type SubscriptionPromotionalOfferPriceAttributes struct{}

// SubscriptionOfferCodeAttributes describes a subscription offer code.
type SubscriptionOfferCodeAttributes struct {
	Name                       string                            `json:"name,omitempty"`
	OfferEligibility           SubscriptionOfferEligibility      `json:"offerEligibility,omitempty"`
	CustomerEligibilities      []SubscriptionCustomerEligibility `json:"customerEligibilities,omitempty"`
	Duration                   SubscriptionOfferDuration         `json:"duration,omitempty"`
	OfferMode                  SubscriptionOfferMode             `json:"offerMode,omitempty"`
	NumberOfPeriods            int                               `json:"numberOfPeriods,omitempty"`
	AutoRenewEnabled           *bool                             `json:"autoRenewEnabled,omitempty"`
	Active                     bool                              `json:"active,omitempty"`
	TotalNumberOfCodes         int                               `json:"totalNumberOfCodes,omitempty"`
	ProductionCodeCount        int                               `json:"productionCodeCount,omitempty"`
	SandboxCodeCount           int                               `json:"sandboxCodeCount,omitempty"`
	TargetSubscriptionPlanType SubscriptionPlanType              `json:"targetSubscriptionPlanType,omitempty"`
}

// SubscriptionOfferCodeCreateAttributes describes attributes for creating an offer code.
type SubscriptionOfferCodeCreateAttributes struct {
	Name                       string                            `json:"name"`
	OfferEligibility           SubscriptionOfferEligibility      `json:"offerEligibility"`
	CustomerEligibilities      []SubscriptionCustomerEligibility `json:"customerEligibilities"`
	Duration                   SubscriptionOfferDuration         `json:"duration"`
	OfferMode                  SubscriptionOfferMode             `json:"offerMode"`
	NumberOfPeriods            int                               `json:"numberOfPeriods"`
	AutoRenewEnabled           *bool                             `json:"autoRenewEnabled,omitempty"`
	TargetSubscriptionPlanType SubscriptionPlanType              `json:"targetSubscriptionPlanType,omitempty"`
}

type SubscriptionOfferCodePrice struct {
	TerritoryID  string
	PricePointID string
}

// SubscriptionOfferCodeCreateRequest is a request to create an offer code.
type SubscriptionOfferCodeCreateRequest struct {
	Data     SubscriptionOfferCodeCreateData          `json:"data"`
	Included []SubscriptionOfferCodePriceInlineCreate `json:"included,omitempty"`
}

// SubscriptionOfferCodeUpdateData is the data portion of an offer code update request.
type SubscriptionOfferCodeUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes SubscriptionOfferCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodePriceAttributes describes offer code price resources.
type SubscriptionOfferCodePriceAttributes struct{}

// SubscriptionAppStoreReviewScreenshotAttributes describes a subscription review screenshot.
type SubscriptionAppStoreReviewScreenshotAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	AssetType          string              `json:"assetType,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// SubscriptionAppStoreReviewScreenshotCreateAttributes describes attributes for creating review screenshots.
type SubscriptionAppStoreReviewScreenshotCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// SubscriptionAppStoreReviewScreenshotUpdateData is the data portion of an update request.
type SubscriptionAppStoreReviewScreenshotUpdateData struct {
	Type       ResourceType                                         `json:"type"`
	ID         string                                               `json:"id"`
	Attributes SubscriptionAppStoreReviewScreenshotUpdateAttributes `json:"attributes"`
}

// SubscriptionGroupLocalizationCreateAttributes describes attributes for creating group localizations.
type SubscriptionGroupLocalizationCreateAttributes struct {
	Name          string `json:"name"`
	CustomAppName string `json:"customAppName,omitempty"`
	Locale        string `json:"locale"`
}

// Response types for subscription resources.
type (
	SubscriptionLocalizationsResponse            = Response[SubscriptionLocalizationAttributes]
	SubscriptionLocalizationResponse             = SingleResponse[SubscriptionLocalizationAttributes]
	SubscriptionImagesResponse                   = Response[SubscriptionImageAttributes]
	SubscriptionIntroductoryOffersResponse       = Response[SubscriptionIntroductoryOfferAttributes]
	SubscriptionIntroductoryOfferResponse        = SingleResponse[SubscriptionIntroductoryOfferAttributes]
	SubscriptionPromotionalOffersResponse        = Response[SubscriptionPromotionalOfferAttributes]
	SubscriptionPromotionalOfferResponse         = SingleResponse[SubscriptionPromotionalOfferAttributes]
	SubscriptionPromotionalOfferPricesResponse   = Response[SubscriptionPromotionalOfferPriceAttributes]
	SubscriptionOfferCodesResponse               = Response[SubscriptionOfferCodeAttributes]
	SubscriptionOfferCodeResponse                = SingleResponse[SubscriptionOfferCodeAttributes]
	SubscriptionOfferCodeCustomCodesResponse     = Response[SubscriptionOfferCodeCustomCodeAttributes]
	SubscriptionOfferCodePricesResponse          = Response[SubscriptionOfferCodePriceAttributes]
	SubscriptionPricePointsResponse              = Response[SubscriptionPricePointAttributes]
	SubscriptionPricePointResponse               = SingleResponse[SubscriptionPricePointAttributes]
	SubscriptionAppStoreReviewScreenshotResponse = SingleResponse[SubscriptionAppStoreReviewScreenshotAttributes]
	SubscriptionGroupLocalizationsResponse       = Response[SubscriptionGroupLocalizationAttributes]
	SubscriptionGroupLocalizationResponse        = SingleResponse[SubscriptionGroupLocalizationAttributes]
)
