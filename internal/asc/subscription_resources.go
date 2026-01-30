package asc

// SubscriptionLocalizationAttributes describes a subscription localization resource.
type SubscriptionLocalizationAttributes struct {
	Name        string `json:"name,omitempty"`
	Locale      string `json:"locale,omitempty"`
	Description string `json:"description,omitempty"`
	State       string `json:"state,omitempty"`
}

// SubscriptionLocalizationCreateAttributes describes attributes for creating a localization.
type SubscriptionLocalizationCreateAttributes struct {
	Name        string `json:"name"`
	Locale      string `json:"locale"`
	Description string `json:"description,omitempty"`
}

// SubscriptionLocalizationUpdateAttributes describes attributes for updating a localization.
type SubscriptionLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SubscriptionLocalizationRelationships describes relationships for localizations.
type SubscriptionLocalizationRelationships struct {
	Subscription *Relationship `json:"subscription"`
}

// SubscriptionLocalizationCreateData is the data portion of a localization create request.
type SubscriptionLocalizationCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    SubscriptionLocalizationCreateAttributes `json:"attributes"`
	Relationships *SubscriptionLocalizationRelationships   `json:"relationships,omitempty"`
}

// SubscriptionLocalizationCreateRequest is a request to create a localization.
type SubscriptionLocalizationCreateRequest struct {
	Data SubscriptionLocalizationCreateData `json:"data"`
}

// SubscriptionLocalizationUpdateData is the data portion of a localization update request.
type SubscriptionLocalizationUpdateData struct {
	Type       ResourceType                             `json:"type"`
	ID         string                                   `json:"id"`
	Attributes SubscriptionLocalizationUpdateAttributes `json:"attributes"`
}

// SubscriptionLocalizationUpdateRequest is a request to update a localization.
type SubscriptionLocalizationUpdateRequest struct {
	Data SubscriptionLocalizationUpdateData `json:"data"`
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

// SubscriptionImageCreateAttributes describes attributes for creating a subscription image.
type SubscriptionImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// SubscriptionImageUpdateAttributes describes attributes for updating a subscription image.
type SubscriptionImageUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// SubscriptionImageRelationships describes relationships for subscription images.
type SubscriptionImageRelationships struct {
	Subscription *Relationship `json:"subscription"`
}

// SubscriptionImageCreateData is the data portion of a subscription image create request.
type SubscriptionImageCreateData struct {
	Type          ResourceType                      `json:"type"`
	Attributes    SubscriptionImageCreateAttributes `json:"attributes"`
	Relationships *SubscriptionImageRelationships   `json:"relationships,omitempty"`
}

// SubscriptionImageCreateRequest is a request to create a subscription image.
type SubscriptionImageCreateRequest struct {
	Data SubscriptionImageCreateData `json:"data"`
}

// SubscriptionImageUpdateData is the data portion of a subscription image update request.
type SubscriptionImageUpdateData struct {
	Type       ResourceType                      `json:"type"`
	ID         string                            `json:"id"`
	Attributes SubscriptionImageUpdateAttributes `json:"attributes"`
}

// SubscriptionImageUpdateRequest is a request to update a subscription image.
type SubscriptionImageUpdateRequest struct {
	Data SubscriptionImageUpdateData `json:"data"`
}

// SubscriptionIntroductoryOfferAttributes describes a subscription introductory offer.
type SubscriptionIntroductoryOfferAttributes struct {
	StartDate       string                    `json:"startDate,omitempty"`
	EndDate         string                    `json:"endDate,omitempty"`
	Duration        SubscriptionOfferDuration `json:"duration,omitempty"`
	OfferMode       SubscriptionOfferMode     `json:"offerMode,omitempty"`
	NumberOfPeriods int                       `json:"numberOfPeriods,omitempty"`
}

// SubscriptionIntroductoryOfferCreateAttributes describes attributes for creating an introductory offer.
type SubscriptionIntroductoryOfferCreateAttributes struct {
	StartDate       string                    `json:"startDate,omitempty"`
	EndDate         string                    `json:"endDate,omitempty"`
	Duration        SubscriptionOfferDuration `json:"duration"`
	OfferMode       SubscriptionOfferMode     `json:"offerMode"`
	NumberOfPeriods int                       `json:"numberOfPeriods"`
}

// SubscriptionIntroductoryOfferUpdateAttributes describes attributes for updating an introductory offer.
type SubscriptionIntroductoryOfferUpdateAttributes struct {
	EndDate *string `json:"endDate,omitempty"`
}

// SubscriptionIntroductoryOfferRelationships describes relationships for introductory offers.
type SubscriptionIntroductoryOfferRelationships struct {
	Subscription           *Relationship `json:"subscription"`
	Territory              *Relationship `json:"territory,omitempty"`
	SubscriptionPricePoint *Relationship `json:"subscriptionPricePoint,omitempty"`
}

// SubscriptionIntroductoryOfferCreateData is the data portion of an introductory offer create request.
type SubscriptionIntroductoryOfferCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    SubscriptionIntroductoryOfferCreateAttributes `json:"attributes"`
	Relationships *SubscriptionIntroductoryOfferRelationships   `json:"relationships,omitempty"`
}

// SubscriptionIntroductoryOfferCreateRequest is a request to create an introductory offer.
type SubscriptionIntroductoryOfferCreateRequest struct {
	Data SubscriptionIntroductoryOfferCreateData `json:"data"`
}

// SubscriptionIntroductoryOfferUpdateData is the data portion of an introductory offer update request.
type SubscriptionIntroductoryOfferUpdateData struct {
	Type       ResourceType                                  `json:"type"`
	ID         string                                        `json:"id"`
	Attributes SubscriptionIntroductoryOfferUpdateAttributes `json:"attributes"`
}

// SubscriptionIntroductoryOfferUpdateRequest is a request to update an introductory offer.
type SubscriptionIntroductoryOfferUpdateRequest struct {
	Data SubscriptionIntroductoryOfferUpdateData `json:"data"`
}

// SubscriptionPromotionalOfferAttributes describes a subscription promotional offer.
type SubscriptionPromotionalOfferAttributes struct {
	Duration        SubscriptionOfferDuration `json:"duration,omitempty"`
	Name            string                    `json:"name,omitempty"`
	NumberOfPeriods int                       `json:"numberOfPeriods,omitempty"`
	OfferCode       string                    `json:"offerCode,omitempty"`
	OfferMode       SubscriptionOfferMode     `json:"offerMode,omitempty"`
}

// SubscriptionPromotionalOfferCreateAttributes describes attributes for creating a promotional offer.
type SubscriptionPromotionalOfferCreateAttributes struct {
	Duration        SubscriptionOfferDuration `json:"duration"`
	Name            string                    `json:"name"`
	NumberOfPeriods int                       `json:"numberOfPeriods"`
	OfferCode       string                    `json:"offerCode"`
	OfferMode       SubscriptionOfferMode     `json:"offerMode"`
}

// SubscriptionPromotionalOfferRelationships describes relationships for promotional offers.
type SubscriptionPromotionalOfferRelationships struct {
	Subscription Relationship     `json:"subscription"`
	Prices       RelationshipList `json:"prices"`
}

// SubscriptionPromotionalOfferCreateData is the data portion of a promotional offer create request.
type SubscriptionPromotionalOfferCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    SubscriptionPromotionalOfferCreateAttributes `json:"attributes"`
	Relationships SubscriptionPromotionalOfferRelationships    `json:"relationships"`
}

// SubscriptionPromotionalOfferCreateRequest is a request to create a promotional offer.
type SubscriptionPromotionalOfferCreateRequest struct {
	Data SubscriptionPromotionalOfferCreateData `json:"data"`
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

// SubscriptionPromotionalOfferUpdateRequest is a request to update a promotional offer.
type SubscriptionPromotionalOfferUpdateRequest struct {
	Data SubscriptionPromotionalOfferUpdateData `json:"data"`
}

// SubscriptionPromotionalOfferPriceAttributes describes promotional offer price resources.
type SubscriptionPromotionalOfferPriceAttributes struct{}

// SubscriptionOfferCodeAttributes describes a subscription offer code.
type SubscriptionOfferCodeAttributes struct {
	Name                  string                            `json:"name,omitempty"`
	OfferEligibility      SubscriptionOfferEligibility      `json:"offerEligibility,omitempty"`
	CustomerEligibilities []SubscriptionCustomerEligibility `json:"customerEligibilities,omitempty"`
	Duration              SubscriptionOfferDuration         `json:"duration,omitempty"`
	OfferMode             SubscriptionOfferMode             `json:"offerMode,omitempty"`
	NumberOfPeriods       int                               `json:"numberOfPeriods,omitempty"`
	AutoRenewEnabled      *bool                             `json:"autoRenewEnabled,omitempty"`
	Active                bool                              `json:"active,omitempty"`
}

// SubscriptionOfferCodeCreateAttributes describes attributes for creating an offer code.
type SubscriptionOfferCodeCreateAttributes struct {
	Name                  string                            `json:"name"`
	OfferEligibility      SubscriptionOfferEligibility      `json:"offerEligibility"`
	CustomerEligibilities []SubscriptionCustomerEligibility `json:"customerEligibilities"`
	Duration              SubscriptionOfferDuration         `json:"duration"`
	OfferMode             SubscriptionOfferMode             `json:"offerMode"`
	NumberOfPeriods       int                               `json:"numberOfPeriods"`
	AutoRenewEnabled      *bool                             `json:"autoRenewEnabled,omitempty"`
}

// SubscriptionOfferCodeUpdateAttributes describes attributes for updating an offer code.
type SubscriptionOfferCodeUpdateAttributes struct {
	Active *bool `json:"active,omitempty"`
}

// SubscriptionOfferCodeRelationships describes relationships for offer codes.
type SubscriptionOfferCodeRelationships struct {
	Subscription Relationship     `json:"subscription"`
	Prices       RelationshipList `json:"prices"`
}

// SubscriptionOfferCodeCreateData is the data portion of an offer code create request.
type SubscriptionOfferCodeCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    SubscriptionOfferCodeCreateAttributes `json:"attributes"`
	Relationships SubscriptionOfferCodeRelationships    `json:"relationships"`
}

// SubscriptionOfferCodeCreateRequest is a request to create an offer code.
type SubscriptionOfferCodeCreateRequest struct {
	Data SubscriptionOfferCodeCreateData `json:"data"`
}

// SubscriptionOfferCodeUpdateData is the data portion of an offer code update request.
type SubscriptionOfferCodeUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes SubscriptionOfferCodeUpdateAttributes `json:"attributes"`
}

// SubscriptionOfferCodeUpdateRequest is a request to update an offer code.
type SubscriptionOfferCodeUpdateRequest struct {
	Data SubscriptionOfferCodeUpdateData `json:"data"`
}

// SubscriptionOfferCodeCustomCodeAttributes describes custom offer codes.
type SubscriptionOfferCodeCustomCodeAttributes struct {
	CustomCode     string `json:"customCode,omitempty"`
	NumberOfCodes  int    `json:"numberOfCodes,omitempty"`
	CreatedDate    string `json:"createdDate,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	Active         bool   `json:"active,omitempty"`
}

// SubscriptionOfferCodePriceAttributes describes offer code price resources.
type SubscriptionOfferCodePriceAttributes struct{}

// SubscriptionPricePointAttributes describes subscription price points.
type SubscriptionPricePointAttributes struct {
	CustomerPrice string `json:"customerPrice,omitempty"`
	Proceeds      string `json:"proceeds,omitempty"`
	ProceedsYear2 string `json:"proceedsYear2,omitempty"`
}

// SubscriptionSubmissionAttributes describes a subscription submission resource.
type SubscriptionSubmissionAttributes struct{}

// SubscriptionSubmissionRelationships describes submission relationships.
type SubscriptionSubmissionRelationships struct {
	Subscription *Relationship `json:"subscription"`
}

// SubscriptionSubmissionCreateData is the data portion of a submission create request.
type SubscriptionSubmissionCreateData struct {
	Type          ResourceType                         `json:"type"`
	Relationships *SubscriptionSubmissionRelationships `json:"relationships"`
}

// SubscriptionSubmissionCreateRequest is a request to create a submission.
type SubscriptionSubmissionCreateRequest struct {
	Data SubscriptionSubmissionCreateData `json:"data"`
}

// SubscriptionGroupSubmissionAttributes describes a subscription group submission resource.
type SubscriptionGroupSubmissionAttributes struct{}

// SubscriptionGroupSubmissionRelationships describes group submission relationships.
type SubscriptionGroupSubmissionRelationships struct {
	SubscriptionGroup *Relationship `json:"subscriptionGroup"`
}

// SubscriptionGroupSubmissionCreateData is the data portion of a group submission create request.
type SubscriptionGroupSubmissionCreateData struct {
	Type          ResourceType                              `json:"type"`
	Relationships *SubscriptionGroupSubmissionRelationships `json:"relationships"`
}

// SubscriptionGroupSubmissionCreateRequest is a request to create a group submission.
type SubscriptionGroupSubmissionCreateRequest struct {
	Data SubscriptionGroupSubmissionCreateData `json:"data"`
}

// SubscriptionAppStoreReviewScreenshotAttributes describes a subscription review screenshot.
type SubscriptionAppStoreReviewScreenshotAttributes struct {
	FileSize           int64             `json:"fileSize,omitempty"`
	FileName           string            `json:"fileName,omitempty"`
	SourceFileChecksum string            `json:"sourceFileChecksum,omitempty"`
	AssetToken         string            `json:"assetToken,omitempty"`
	ImageAsset         *ImageAsset       `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation `json:"uploadOperations,omitempty"`
	State              string            `json:"state,omitempty"`
}

// SubscriptionAppStoreReviewScreenshotCreateAttributes describes attributes for creating review screenshots.
type SubscriptionAppStoreReviewScreenshotCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// SubscriptionAppStoreReviewScreenshotUpdateAttributes describes attributes for updating review screenshots.
type SubscriptionAppStoreReviewScreenshotUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// SubscriptionAppStoreReviewScreenshotRelationships describes relationships for review screenshots.
type SubscriptionAppStoreReviewScreenshotRelationships struct {
	Subscription *Relationship `json:"subscription"`
}

// SubscriptionAppStoreReviewScreenshotCreateData is the data portion of a create request.
type SubscriptionAppStoreReviewScreenshotCreateData struct {
	Type          ResourceType                                         `json:"type"`
	Attributes    SubscriptionAppStoreReviewScreenshotCreateAttributes `json:"attributes"`
	Relationships *SubscriptionAppStoreReviewScreenshotRelationships   `json:"relationships,omitempty"`
}

// SubscriptionAppStoreReviewScreenshotCreateRequest is a request to create review screenshots.
type SubscriptionAppStoreReviewScreenshotCreateRequest struct {
	Data SubscriptionAppStoreReviewScreenshotCreateData `json:"data"`
}

// SubscriptionAppStoreReviewScreenshotUpdateData is the data portion of an update request.
type SubscriptionAppStoreReviewScreenshotUpdateData struct {
	Type       ResourceType                                         `json:"type"`
	ID         string                                               `json:"id"`
	Attributes SubscriptionAppStoreReviewScreenshotUpdateAttributes `json:"attributes"`
}

// SubscriptionAppStoreReviewScreenshotUpdateRequest is a request to update review screenshots.
type SubscriptionAppStoreReviewScreenshotUpdateRequest struct {
	Data SubscriptionAppStoreReviewScreenshotUpdateData `json:"data"`
}

// SubscriptionGroupLocalizationAttributes describes a subscription group localization.
type SubscriptionGroupLocalizationAttributes struct {
	Name          string `json:"name,omitempty"`
	CustomAppName string `json:"customAppName,omitempty"`
	Locale        string `json:"locale,omitempty"`
	State         string `json:"state,omitempty"`
}

// SubscriptionGroupLocalizationCreateAttributes describes attributes for creating group localizations.
type SubscriptionGroupLocalizationCreateAttributes struct {
	Name          string `json:"name"`
	CustomAppName string `json:"customAppName,omitempty"`
	Locale        string `json:"locale"`
}

// SubscriptionGroupLocalizationUpdateAttributes describes attributes for updating group localizations.
type SubscriptionGroupLocalizationUpdateAttributes struct {
	Name          *string `json:"name,omitempty"`
	CustomAppName *string `json:"customAppName,omitempty"`
}

// SubscriptionGroupLocalizationRelationships describes group localization relationships.
type SubscriptionGroupLocalizationRelationships struct {
	SubscriptionGroup *Relationship `json:"subscriptionGroup"`
}

// SubscriptionGroupLocalizationCreateData is the data portion of a group localization create request.
type SubscriptionGroupLocalizationCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    SubscriptionGroupLocalizationCreateAttributes `json:"attributes"`
	Relationships *SubscriptionGroupLocalizationRelationships   `json:"relationships,omitempty"`
}

// SubscriptionGroupLocalizationCreateRequest is a request to create group localizations.
type SubscriptionGroupLocalizationCreateRequest struct {
	Data SubscriptionGroupLocalizationCreateData `json:"data"`
}

// SubscriptionGroupLocalizationUpdateData is the data portion of a group localization update request.
type SubscriptionGroupLocalizationUpdateData struct {
	Type       ResourceType                                  `json:"type"`
	ID         string                                        `json:"id"`
	Attributes SubscriptionGroupLocalizationUpdateAttributes `json:"attributes"`
}

// SubscriptionGroupLocalizationUpdateRequest is a request to update group localizations.
type SubscriptionGroupLocalizationUpdateRequest struct {
	Data SubscriptionGroupLocalizationUpdateData `json:"data"`
}

// Response types for subscription resources.
type (
	SubscriptionLocalizationsResponse            = Response[SubscriptionLocalizationAttributes]
	SubscriptionLocalizationResponse             = SingleResponse[SubscriptionLocalizationAttributes]
	SubscriptionImagesResponse                   = Response[SubscriptionImageAttributes]
	SubscriptionImageResponse                    = SingleResponse[SubscriptionImageAttributes]
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
	SubscriptionSubmissionResponse               = SingleResponse[SubscriptionSubmissionAttributes]
	SubscriptionGroupSubmissionResponse          = SingleResponse[SubscriptionGroupSubmissionAttributes]
	SubscriptionAppStoreReviewScreenshotResponse = SingleResponse[SubscriptionAppStoreReviewScreenshotAttributes]
	SubscriptionGroupLocalizationsResponse       = Response[SubscriptionGroupLocalizationAttributes]
	SubscriptionGroupLocalizationResponse        = SingleResponse[SubscriptionGroupLocalizationAttributes]
)
