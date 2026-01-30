package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// AppClipAction represents App Clip experience actions.
type AppClipAction string

const (
	AppClipActionOpen AppClipAction = "OPEN"
	AppClipActionView AppClipAction = "VIEW"
	AppClipActionPlay AppClipAction = "PLAY"
)

// AppClipAdvancedExperienceBusinessCategory represents business category values.
type AppClipAdvancedExperienceBusinessCategory string

const (
	AppClipAdvancedExperienceBusinessCategoryAutomotive           AppClipAdvancedExperienceBusinessCategory = "AUTOMOTIVE"
	AppClipAdvancedExperienceBusinessCategoryBeauty               AppClipAdvancedExperienceBusinessCategory = "BEAUTY"
	AppClipAdvancedExperienceBusinessCategoryBikes                AppClipAdvancedExperienceBusinessCategory = "BIKES"
	AppClipAdvancedExperienceBusinessCategoryBooks                AppClipAdvancedExperienceBusinessCategory = "BOOKS"
	AppClipAdvancedExperienceBusinessCategoryCasino               AppClipAdvancedExperienceBusinessCategory = "CASINO"
	AppClipAdvancedExperienceBusinessCategoryEducation            AppClipAdvancedExperienceBusinessCategory = "EDUCATION"
	AppClipAdvancedExperienceBusinessCategoryEducationJapan       AppClipAdvancedExperienceBusinessCategory = "EDUCATION_JAPAN"
	AppClipAdvancedExperienceBusinessCategoryEntertainment        AppClipAdvancedExperienceBusinessCategory = "ENTERTAINMENT"
	AppClipAdvancedExperienceBusinessCategoryEVCharger            AppClipAdvancedExperienceBusinessCategory = "EV_CHARGER"
	AppClipAdvancedExperienceBusinessCategoryFinancialUSD         AppClipAdvancedExperienceBusinessCategory = "FINANCIAL_USD"
	AppClipAdvancedExperienceBusinessCategoryFinancialCNY         AppClipAdvancedExperienceBusinessCategory = "FINANCIAL_CNY"
	AppClipAdvancedExperienceBusinessCategoryFinancialGBP         AppClipAdvancedExperienceBusinessCategory = "FINANCIAL_GBP"
	AppClipAdvancedExperienceBusinessCategoryFinancialJPY         AppClipAdvancedExperienceBusinessCategory = "FINANCIAL_JPY"
	AppClipAdvancedExperienceBusinessCategoryFinancialEUR         AppClipAdvancedExperienceBusinessCategory = "FINANCIAL_EUR"
	AppClipAdvancedExperienceBusinessCategoryFitness              AppClipAdvancedExperienceBusinessCategory = "FITNESS"
	AppClipAdvancedExperienceBusinessCategoryFoodAndDrink         AppClipAdvancedExperienceBusinessCategory = "FOOD_AND_DRINK"
	AppClipAdvancedExperienceBusinessCategoryGas                  AppClipAdvancedExperienceBusinessCategory = "GAS"
	AppClipAdvancedExperienceBusinessCategoryGrocery              AppClipAdvancedExperienceBusinessCategory = "GROCERY"
	AppClipAdvancedExperienceBusinessCategoryHealthAndMedical     AppClipAdvancedExperienceBusinessCategory = "HEALTH_AND_MEDICAL"
	AppClipAdvancedExperienceBusinessCategoryHotelAndTravel       AppClipAdvancedExperienceBusinessCategory = "HOTEL_AND_TRAVEL"
	AppClipAdvancedExperienceBusinessCategoryMusic                AppClipAdvancedExperienceBusinessCategory = "MUSIC"
	AppClipAdvancedExperienceBusinessCategoryParking              AppClipAdvancedExperienceBusinessCategory = "PARKING"
	AppClipAdvancedExperienceBusinessCategoryPetServices          AppClipAdvancedExperienceBusinessCategory = "PET_SERVICES"
	AppClipAdvancedExperienceBusinessCategoryProfessionalServices AppClipAdvancedExperienceBusinessCategory = "PROFESSIONAL_SERVICES"
	AppClipAdvancedExperienceBusinessCategoryShopping             AppClipAdvancedExperienceBusinessCategory = "SHOPPING"
	AppClipAdvancedExperienceBusinessCategoryTicketing            AppClipAdvancedExperienceBusinessCategory = "TICKETING"
	AppClipAdvancedExperienceBusinessCategoryTransit              AppClipAdvancedExperienceBusinessCategory = "TRANSIT"
)

// AppClipAdvancedExperienceLanguage represents default language values.
type AppClipAdvancedExperienceLanguage string

const (
	AppClipAdvancedExperienceLanguageAR AppClipAdvancedExperienceLanguage = "AR"
	AppClipAdvancedExperienceLanguageCA AppClipAdvancedExperienceLanguage = "CA"
	AppClipAdvancedExperienceLanguageCS AppClipAdvancedExperienceLanguage = "CS"
	AppClipAdvancedExperienceLanguageDA AppClipAdvancedExperienceLanguage = "DA"
	AppClipAdvancedExperienceLanguageDE AppClipAdvancedExperienceLanguage = "DE"
	AppClipAdvancedExperienceLanguageEL AppClipAdvancedExperienceLanguage = "EL"
	AppClipAdvancedExperienceLanguageEN AppClipAdvancedExperienceLanguage = "EN"
	AppClipAdvancedExperienceLanguageES AppClipAdvancedExperienceLanguage = "ES"
	AppClipAdvancedExperienceLanguageFI AppClipAdvancedExperienceLanguage = "FI"
	AppClipAdvancedExperienceLanguageFR AppClipAdvancedExperienceLanguage = "FR"
	AppClipAdvancedExperienceLanguageHE AppClipAdvancedExperienceLanguage = "HE"
	AppClipAdvancedExperienceLanguageHI AppClipAdvancedExperienceLanguage = "HI"
	AppClipAdvancedExperienceLanguageHR AppClipAdvancedExperienceLanguage = "HR"
	AppClipAdvancedExperienceLanguageHU AppClipAdvancedExperienceLanguage = "HU"
	AppClipAdvancedExperienceLanguageID AppClipAdvancedExperienceLanguage = "ID"
	AppClipAdvancedExperienceLanguageIT AppClipAdvancedExperienceLanguage = "IT"
	AppClipAdvancedExperienceLanguageJA AppClipAdvancedExperienceLanguage = "JA"
	AppClipAdvancedExperienceLanguageKO AppClipAdvancedExperienceLanguage = "KO"
	AppClipAdvancedExperienceLanguageMS AppClipAdvancedExperienceLanguage = "MS"
	AppClipAdvancedExperienceLanguageNL AppClipAdvancedExperienceLanguage = "NL"
	AppClipAdvancedExperienceLanguageNO AppClipAdvancedExperienceLanguage = "NO"
	AppClipAdvancedExperienceLanguagePL AppClipAdvancedExperienceLanguage = "PL"
	AppClipAdvancedExperienceLanguagePT AppClipAdvancedExperienceLanguage = "PT"
	AppClipAdvancedExperienceLanguageRO AppClipAdvancedExperienceLanguage = "RO"
	AppClipAdvancedExperienceLanguageRU AppClipAdvancedExperienceLanguage = "RU"
	AppClipAdvancedExperienceLanguageSK AppClipAdvancedExperienceLanguage = "SK"
	AppClipAdvancedExperienceLanguageSV AppClipAdvancedExperienceLanguage = "SV"
	AppClipAdvancedExperienceLanguageTH AppClipAdvancedExperienceLanguage = "TH"
	AppClipAdvancedExperienceLanguageTR AppClipAdvancedExperienceLanguage = "TR"
	AppClipAdvancedExperienceLanguageUK AppClipAdvancedExperienceLanguage = "UK"
	AppClipAdvancedExperienceLanguageVI AppClipAdvancedExperienceLanguage = "VI"
	AppClipAdvancedExperienceLanguageZH AppClipAdvancedExperienceLanguage = "ZH"
)

// AppClipAttributes describes an App Clip resource.
type AppClipAttributes struct {
	BundleID string `json:"bundleId,omitempty"`
}

// AppClipDefaultExperienceAttributes describes App Clip default experience attributes.
type AppClipDefaultExperienceAttributes struct {
	Action AppClipAction `json:"action,omitempty"`
}

// AppClipDefaultExperienceLocalizationAttributes describes default experience localization attributes.
type AppClipDefaultExperienceLocalizationAttributes struct {
	Locale   string `json:"locale,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
}

// AppClipAdvancedExperienceAttributes describes advanced experience attributes.
type AppClipAdvancedExperienceAttributes struct {
	Link             string                                    `json:"link,omitempty"`
	Version          string                                    `json:"version,omitempty"`
	Status           string                                    `json:"status,omitempty"`
	Action           AppClipAction                             `json:"action,omitempty"`
	IsPoweredBy      bool                                      `json:"isPoweredBy,omitempty"`
	Place            json.RawMessage                           `json:"place,omitempty"`
	PlaceStatus      string                                    `json:"placeStatus,omitempty"`
	BusinessCategory AppClipAdvancedExperienceBusinessCategory `json:"businessCategory,omitempty"`
	DefaultLanguage  AppClipAdvancedExperienceLanguage         `json:"defaultLanguage,omitempty"`
}

// AppClipAdvancedExperienceImageAttributes describes advanced experience image attributes.
type AppClipAdvancedExperienceImageAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// AppClipHeaderImageAttributes describes header image attributes.
type AppClipHeaderImageAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	SourceFileChecksum string              `json:"sourceFileChecksum,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// AppClipAppStoreReviewDetailAttributes describes review detail attributes.
type AppClipAppStoreReviewDetailAttributes struct {
	InvocationURLs []string `json:"invocationUrls,omitempty"`
}

// AppClipDefaultExperienceCreateAttributes describes default experience create attributes.
type AppClipDefaultExperienceCreateAttributes struct {
	Action *AppClipAction `json:"action,omitempty"`
}

// AppClipDefaultExperienceUpdateAttributes describes default experience update attributes.
type AppClipDefaultExperienceUpdateAttributes struct {
	Action *AppClipAction `json:"action,omitempty"`
}

// AppClipDefaultExperienceLocalizationCreateAttributes describes localization create attributes.
type AppClipDefaultExperienceLocalizationCreateAttributes struct {
	Locale   string  `json:"locale"`
	Subtitle *string `json:"subtitle,omitempty"`
}

// AppClipDefaultExperienceLocalizationUpdateAttributes describes localization update attributes.
type AppClipDefaultExperienceLocalizationUpdateAttributes struct {
	Subtitle *string `json:"subtitle,omitempty"`
}

// AppClipAdvancedExperienceCreateAttributes describes advanced experience create attributes.
type AppClipAdvancedExperienceCreateAttributes struct {
	Link             string                                     `json:"link"`
	DefaultLanguage  AppClipAdvancedExperienceLanguage          `json:"defaultLanguage"`
	IsPoweredBy      bool                                       `json:"isPoweredBy"`
	Action           *AppClipAction                             `json:"action,omitempty"`
	BusinessCategory *AppClipAdvancedExperienceBusinessCategory `json:"businessCategory,omitempty"`
	Place            json.RawMessage                            `json:"place,omitempty"`
}

// AppClipAdvancedExperienceUpdateAttributes describes advanced experience update attributes.
type AppClipAdvancedExperienceUpdateAttributes struct {
	Action           *AppClipAction                             `json:"action,omitempty"`
	IsPoweredBy      *bool                                      `json:"isPoweredBy,omitempty"`
	Place            json.RawMessage                            `json:"place,omitempty"`
	BusinessCategory *AppClipAdvancedExperienceBusinessCategory `json:"businessCategory,omitempty"`
	DefaultLanguage  *AppClipAdvancedExperienceLanguage         `json:"defaultLanguage,omitempty"`
	Removed          *bool                                      `json:"removed,omitempty"`
}

// AppClipAdvancedExperienceImageCreateAttributes describes image create attributes.
type AppClipAdvancedExperienceImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// AppClipAdvancedExperienceImageUpdateAttributes describes image update attributes.
type AppClipAdvancedExperienceImageUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// AppClipHeaderImageCreateAttributes describes header image create attributes.
type AppClipHeaderImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// AppClipHeaderImageUpdateAttributes describes header image update attributes.
type AppClipHeaderImageUpdateAttributes struct {
	SourceFileChecksum *string `json:"sourceFileChecksum,omitempty"`
	Uploaded           *bool   `json:"uploaded,omitempty"`
}

// AppClipAppStoreReviewDetailCreateAttributes describes review detail create attributes.
type AppClipAppStoreReviewDetailCreateAttributes struct {
	InvocationURLs []string `json:"invocationUrls,omitempty"`
}

// AppClipAppStoreReviewDetailUpdateAttributes describes review detail update attributes.
type AppClipAppStoreReviewDetailUpdateAttributes struct {
	InvocationURLs []string `json:"invocationUrls,omitempty"`
}

// AppClipDefaultExperienceCreateRelationships describes relationships for default experience create.
type AppClipDefaultExperienceCreateRelationships struct {
	AppClip                          *Relationship `json:"appClip"`
	ReleaseWithAppStoreVersion       *Relationship `json:"releaseWithAppStoreVersion,omitempty"`
	AppClipDefaultExperienceTemplate *Relationship `json:"appClipDefaultExperienceTemplate,omitempty"`
}

// AppClipDefaultExperienceUpdateRelationships describes relationships for default experience update.
type AppClipDefaultExperienceUpdateRelationships struct {
	ReleaseWithAppStoreVersion *Relationship `json:"releaseWithAppStoreVersion,omitempty"`
}

// AppClipDefaultExperienceLocalizationRelationships describes localization relationships.
type AppClipDefaultExperienceLocalizationRelationships struct {
	AppClipDefaultExperience *Relationship `json:"appClipDefaultExperience,omitempty"`
	AppClipHeaderImage       *Relationship `json:"appClipHeaderImage,omitempty"`
}

// AppClipAdvancedExperienceRelationships describes advanced experience relationships.
type AppClipAdvancedExperienceRelationships struct {
	AppClip       *Relationship     `json:"appClip,omitempty"`
	HeaderImage   *Relationship     `json:"headerImage,omitempty"`
	Localizations *RelationshipList `json:"localizations,omitempty"`
}

// AppClipHeaderImageRelationships describes header image relationships.
type AppClipHeaderImageRelationships struct {
	AppClipDefaultExperienceLocalization *Relationship `json:"appClipDefaultExperienceLocalization,omitempty"`
}

// AppClipAppStoreReviewDetailRelationships describes review detail relationships.
type AppClipAppStoreReviewDetailRelationships struct {
	AppClipDefaultExperience *Relationship `json:"appClipDefaultExperience,omitempty"`
}

// AppClipDefaultExperienceCreateData is the payload for creating a default experience.
type AppClipDefaultExperienceCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    *AppClipDefaultExperienceCreateAttributes    `json:"attributes,omitempty"`
	Relationships *AppClipDefaultExperienceCreateRelationships `json:"relationships"`
}

// AppClipDefaultExperienceCreateRequest is the create request payload.
type AppClipDefaultExperienceCreateRequest struct {
	Data AppClipDefaultExperienceCreateData `json:"data"`
}

// AppClipDefaultExperienceUpdateData is the payload for updating a default experience.
type AppClipDefaultExperienceUpdateData struct {
	Type          ResourceType                                 `json:"type"`
	ID            string                                       `json:"id"`
	Attributes    *AppClipDefaultExperienceUpdateAttributes    `json:"attributes,omitempty"`
	Relationships *AppClipDefaultExperienceUpdateRelationships `json:"relationships,omitempty"`
}

// AppClipDefaultExperienceUpdateRequest is the update request payload.
type AppClipDefaultExperienceUpdateRequest struct {
	Data AppClipDefaultExperienceUpdateData `json:"data"`
}

// AppClipDefaultExperienceLocalizationCreateData is the payload for creating a localization.
type AppClipDefaultExperienceLocalizationCreateData struct {
	Type          ResourceType                                         `json:"type"`
	Attributes    AppClipDefaultExperienceLocalizationCreateAttributes `json:"attributes"`
	Relationships *AppClipDefaultExperienceLocalizationRelationships   `json:"relationships"`
}

// AppClipDefaultExperienceLocalizationCreateRequest is the create request payload.
type AppClipDefaultExperienceLocalizationCreateRequest struct {
	Data AppClipDefaultExperienceLocalizationCreateData `json:"data"`
}

// AppClipDefaultExperienceLocalizationUpdateData is the payload for updating a localization.
type AppClipDefaultExperienceLocalizationUpdateData struct {
	Type       ResourceType                                          `json:"type"`
	ID         string                                                `json:"id"`
	Attributes *AppClipDefaultExperienceLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// AppClipDefaultExperienceLocalizationUpdateRequest is the update request payload.
type AppClipDefaultExperienceLocalizationUpdateRequest struct {
	Data AppClipDefaultExperienceLocalizationUpdateData `json:"data"`
}

// AppClipAdvancedExperienceCreateData is the payload for creating an advanced experience.
type AppClipAdvancedExperienceCreateData struct {
	Type          ResourceType                              `json:"type"`
	Attributes    AppClipAdvancedExperienceCreateAttributes `json:"attributes"`
	Relationships *AppClipAdvancedExperienceRelationships   `json:"relationships"`
}

// AppClipAdvancedExperienceCreateRequest is the create request payload.
type AppClipAdvancedExperienceCreateRequest struct {
	Data AppClipAdvancedExperienceCreateData `json:"data"`
}

// AppClipAdvancedExperienceUpdateData is the payload for updating an advanced experience.
type AppClipAdvancedExperienceUpdateData struct {
	Type          ResourceType                               `json:"type"`
	ID            string                                     `json:"id"`
	Attributes    *AppClipAdvancedExperienceUpdateAttributes `json:"attributes,omitempty"`
	Relationships *AppClipAdvancedExperienceRelationships    `json:"relationships,omitempty"`
}

// AppClipAdvancedExperienceUpdateRequest is the update request payload.
type AppClipAdvancedExperienceUpdateRequest struct {
	Data AppClipAdvancedExperienceUpdateData `json:"data"`
}

// AppClipAdvancedExperienceImageCreateData is the payload for creating an image reservation.
type AppClipAdvancedExperienceImageCreateData struct {
	Type       ResourceType                                   `json:"type"`
	Attributes AppClipAdvancedExperienceImageCreateAttributes `json:"attributes"`
}

// AppClipAdvancedExperienceImageCreateRequest is the create request payload.
type AppClipAdvancedExperienceImageCreateRequest struct {
	Data AppClipAdvancedExperienceImageCreateData `json:"data"`
}

// AppClipAdvancedExperienceImageUpdateData is the payload for updating an image.
type AppClipAdvancedExperienceImageUpdateData struct {
	Type       ResourceType                                    `json:"type"`
	ID         string                                          `json:"id"`
	Attributes *AppClipAdvancedExperienceImageUpdateAttributes `json:"attributes,omitempty"`
}

// AppClipAdvancedExperienceImageUpdateRequest is the update request payload.
type AppClipAdvancedExperienceImageUpdateRequest struct {
	Data AppClipAdvancedExperienceImageUpdateData `json:"data"`
}

// AppClipHeaderImageCreateData is the payload for creating a header image.
type AppClipHeaderImageCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    AppClipHeaderImageCreateAttributes `json:"attributes"`
	Relationships *AppClipHeaderImageRelationships   `json:"relationships"`
}

// AppClipHeaderImageCreateRequest is the create request payload.
type AppClipHeaderImageCreateRequest struct {
	Data AppClipHeaderImageCreateData `json:"data"`
}

// AppClipHeaderImageUpdateData is the payload for updating a header image.
type AppClipHeaderImageUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *AppClipHeaderImageUpdateAttributes `json:"attributes,omitempty"`
}

// AppClipHeaderImageUpdateRequest is the update request payload.
type AppClipHeaderImageUpdateRequest struct {
	Data AppClipHeaderImageUpdateData `json:"data"`
}

// AppClipAppStoreReviewDetailCreateData is the payload for creating review details.
type AppClipAppStoreReviewDetailCreateData struct {
	Type          ResourceType                                 `json:"type"`
	Attributes    *AppClipAppStoreReviewDetailCreateAttributes `json:"attributes,omitempty"`
	Relationships *AppClipAppStoreReviewDetailRelationships    `json:"relationships"`
}

// AppClipAppStoreReviewDetailCreateRequest is the create request payload.
type AppClipAppStoreReviewDetailCreateRequest struct {
	Data AppClipAppStoreReviewDetailCreateData `json:"data"`
}

// AppClipAppStoreReviewDetailUpdateData is the payload for updating review details.
type AppClipAppStoreReviewDetailUpdateData struct {
	Type       ResourceType                                 `json:"type"`
	ID         string                                       `json:"id"`
	Attributes *AppClipAppStoreReviewDetailUpdateAttributes `json:"attributes,omitempty"`
}

// AppClipAppStoreReviewDetailUpdateRequest is the update request payload.
type AppClipAppStoreReviewDetailUpdateRequest struct {
	Data AppClipAppStoreReviewDetailUpdateData `json:"data"`
}

// BetaAppClipInvocationCreateAttributes describes invocation create attributes.
type BetaAppClipInvocationCreateAttributes struct {
	URL string `json:"url"`
}

// BetaAppClipInvocationUpdateAttributes describes invocation update attributes.
type BetaAppClipInvocationUpdateAttributes struct {
	URL *string `json:"url,omitempty"`
}

// BetaAppClipInvocationLocalizationAttributes describes localization attributes.
type BetaAppClipInvocationLocalizationAttributes struct {
	Title  string `json:"title,omitempty"`
	Locale string `json:"locale,omitempty"`
}

// BetaAppClipInvocationLocalizationCreateAttributes describes localization create attributes.
type BetaAppClipInvocationLocalizationCreateAttributes struct {
	Title  string `json:"title"`
	Locale string `json:"locale"`
}

// BetaAppClipInvocationLocalizationUpdateAttributes describes localization update attributes.
type BetaAppClipInvocationLocalizationUpdateAttributes struct {
	Title *string `json:"title,omitempty"`
}

// BetaAppClipInvocationRelationships describes invocation relationships.
type BetaAppClipInvocationRelationships struct {
	BuildBundle                        *Relationship     `json:"buildBundle,omitempty"`
	BetaAppClipInvocationLocalizations *RelationshipList `json:"betaAppClipInvocationLocalizations,omitempty"`
}

// BetaAppClipInvocationLocalizationRelationships describes localization relationships.
type BetaAppClipInvocationLocalizationRelationships struct {
	BetaAppClipInvocation *Relationship `json:"betaAppClipInvocation,omitempty"`
}

// BetaAppClipInvocationCreateData is the payload for creating an invocation.
type BetaAppClipInvocationCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    BetaAppClipInvocationCreateAttributes `json:"attributes"`
	Relationships *BetaAppClipInvocationRelationships   `json:"relationships,omitempty"`
}

// BetaAppClipInvocationCreateRequest is the create request payload.
type BetaAppClipInvocationCreateRequest struct {
	Data BetaAppClipInvocationCreateData `json:"data"`
}

// BetaAppClipInvocationUpdateData is the payload for updating an invocation.
type BetaAppClipInvocationUpdateData struct {
	Type       ResourceType                           `json:"type"`
	ID         string                                 `json:"id"`
	Attributes *BetaAppClipInvocationUpdateAttributes `json:"attributes,omitempty"`
}

// BetaAppClipInvocationUpdateRequest is the update request payload.
type BetaAppClipInvocationUpdateRequest struct {
	Data BetaAppClipInvocationUpdateData `json:"data"`
}

// BetaAppClipInvocationLocalizationCreateData is the payload for creating a localization.
type BetaAppClipInvocationLocalizationCreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    BetaAppClipInvocationLocalizationCreateAttributes `json:"attributes"`
	Relationships *BetaAppClipInvocationLocalizationRelationships   `json:"relationships"`
}

// BetaAppClipInvocationLocalizationCreateRequest is the create request payload.
type BetaAppClipInvocationLocalizationCreateRequest struct {
	Data BetaAppClipInvocationLocalizationCreateData `json:"data"`
}

// BetaAppClipInvocationLocalizationUpdateData is the payload for updating a localization.
type BetaAppClipInvocationLocalizationUpdateData struct {
	Type       ResourceType                                       `json:"type"`
	ID         string                                             `json:"id"`
	Attributes *BetaAppClipInvocationLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// BetaAppClipInvocationLocalizationUpdateRequest is the update request payload.
type BetaAppClipInvocationLocalizationUpdateRequest struct {
	Data BetaAppClipInvocationLocalizationUpdateData `json:"data"`
}

// Response aliases.
type (
	AppClipsResponse                              = Response[AppClipAttributes]
	AppClipResponse                               = SingleResponse[AppClipAttributes]
	AppClipDefaultExperiencesResponse             = Response[AppClipDefaultExperienceAttributes]
	AppClipDefaultExperienceResponse              = SingleResponse[AppClipDefaultExperienceAttributes]
	AppClipDefaultExperienceLocalizationsResponse = Response[AppClipDefaultExperienceLocalizationAttributes]
	AppClipDefaultExperienceLocalizationResponse  = SingleResponse[AppClipDefaultExperienceLocalizationAttributes]
	AppClipDefaultExperiencesLinkagesResponse     = LinkagesResponse
	AppClipAdvancedExperiencesResponse            = Response[AppClipAdvancedExperienceAttributes]
	AppClipAdvancedExperienceResponse             = SingleResponse[AppClipAdvancedExperienceAttributes]
	AppClipAdvancedExperiencesLinkagesResponse    = LinkagesResponse
	AppClipAdvancedExperienceImageResponse        = SingleResponse[AppClipAdvancedExperienceImageAttributes]
	AppClipHeaderImageResponse                    = SingleResponse[AppClipHeaderImageAttributes]
	AppClipAppStoreReviewDetailResponse           = SingleResponse[AppClipAppStoreReviewDetailAttributes]
	BetaAppClipInvocationResponse                 = SingleResponse[BetaAppClipInvocationAttributes]
	BetaAppClipInvocationLocalizationsResponse    = Response[BetaAppClipInvocationLocalizationAttributes]
	BetaAppClipInvocationLocalizationResponse     = SingleResponse[BetaAppClipInvocationLocalizationAttributes]
)

// AppClipDefaultExperienceReviewDetailLinkageResponse is the response for review detail relationships.
type AppClipDefaultExperienceReviewDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse is the response for release relationship.
type AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse is the response for header image relationships.
type AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links,omitempty"`
}

// AppClipAdvancedExperienceImageUploadResult represents upload results.
type AppClipAdvancedExperienceImageUploadResult struct {
	ID                 string `json:"id"`
	ExperienceID       string `json:"experienceId,omitempty"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}

// AppClipHeaderImageUploadResult represents header image upload results.
type AppClipHeaderImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}

// AppClipDefaultExperienceDeleteResult represents default experience deletion.
type AppClipDefaultExperienceDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppClipDefaultExperienceLocalizationDeleteResult represents localization deletion.
type AppClipDefaultExperienceLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppClipAdvancedExperienceDeleteResult represents advanced experience deletion.
type AppClipAdvancedExperienceDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppClipAdvancedExperienceImageDeleteResult represents advanced image deletion.
type AppClipAdvancedExperienceImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// AppClipHeaderImageDeleteResult represents header image deletion.
type AppClipHeaderImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BetaAppClipInvocationDeleteResult represents invocation deletion.
type BetaAppClipInvocationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// BetaAppClipInvocationLocalizationDeleteResult represents localization deletion.
type BetaAppClipInvocationLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GetAppClips retrieves App Clips for an app.
func (c *Client) GetAppClips(ctx context.Context, appID string, opts ...AppClipsOption) (*AppClipsResponse, error) {
	query := &appClipsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/appClips", appID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClips: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppClipsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClip retrieves an App Clip by ID.
func (c *Client) GetAppClip(ctx context.Context, appClipID string) (*AppClipResponse, error) {
	appClipID = strings.TrimSpace(appClipID)
	if appClipID == "" {
		return nil, fmt.Errorf("appClipID is required")
	}

	path := fmt.Sprintf("/v1/appClips/%s", appClipID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperiences retrieves default experiences for an App Clip.
func (c *Client) GetAppClipDefaultExperiences(ctx context.Context, appClipID string, opts ...AppClipDefaultExperiencesOption) (*AppClipDefaultExperiencesResponse, error) {
	query := &appClipDefaultExperiencesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appClipID = strings.TrimSpace(appClipID)
	if appClipID == "" {
		return nil, fmt.Errorf("appClipID is required")
	}

	path := fmt.Sprintf("/v1/appClips/%s/appClipDefaultExperiences", appClipID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClipDefaultExperiences: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppClipDefaultExperiencesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperiencesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperiencesRelationships retrieves default experience linkages for an App Clip.
func (c *Client) GetAppClipDefaultExperiencesRelationships(ctx context.Context, appClipID string, opts ...LinkagesOption) (*AppClipDefaultExperiencesLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appClipID = strings.TrimSpace(appClipID)
	path := fmt.Sprintf("/v1/appClips/%s/relationships/appClipDefaultExperiences", appClipID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClipDefaultExperiencesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperiencesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipAdvancedExperiencesRelationships retrieves advanced experience linkages for an App Clip.
func (c *Client) GetAppClipAdvancedExperiencesRelationships(ctx context.Context, appClipID string, opts ...LinkagesOption) (*AppClipAdvancedExperiencesLinkagesResponse, error) {
	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appClipID = strings.TrimSpace(appClipID)
	path := fmt.Sprintf("/v1/appClips/%s/relationships/appClipAdvancedExperiences", appClipID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClipAdvancedExperiencesRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperiencesLinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperience retrieves a default experience by ID.
func (c *Client) GetAppClipDefaultExperience(ctx context.Context, experienceID string, opts ...AppClipDefaultExperienceOption) (*AppClipDefaultExperienceResponse, error) {
	query := &appClipDefaultExperienceQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s", experienceID)
	if queryString := buildAppClipDefaultExperienceQuery(query); queryString != "" {
		path += "?" + queryString
	}
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperienceReviewDetailRelationship retrieves the review detail linkage for a default experience.
func (c *Client) GetAppClipDefaultExperienceReviewDetailRelationship(ctx context.Context, experienceID string) (*AppClipDefaultExperienceReviewDetailLinkageResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s/relationships/appClipAppStoreReviewDetail", experienceID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceReviewDetailLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship retrieves releaseWithAppStoreVersion linkage.
func (c *Client) GetAppClipDefaultExperienceReleaseWithAppStoreVersionRelationship(ctx context.Context, experienceID string) (*AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s/relationships/releaseWithAppStoreVersion", experienceID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceReleaseWithAppStoreVersionLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipDefaultExperience creates a default experience for an App Clip.
func (c *Client) CreateAppClipDefaultExperience(ctx context.Context, appClipID string, attrs *AppClipDefaultExperienceCreateAttributes, releaseVersionID string, templateID string) (*AppClipDefaultExperienceResponse, error) {
	appClipID = strings.TrimSpace(appClipID)
	if appClipID == "" {
		return nil, fmt.Errorf("appClipID is required")
	}

	relationships := &AppClipDefaultExperienceCreateRelationships{
		AppClip: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClips,
				ID:   appClipID,
			},
		},
	}
	releaseVersionID = strings.TrimSpace(releaseVersionID)
	if releaseVersionID != "" {
		relationships.ReleaseWithAppStoreVersion = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppStoreVersions,
				ID:   releaseVersionID,
			},
		}
	}
	templateID = strings.TrimSpace(templateID)
	if templateID != "" {
		relationships.AppClipDefaultExperienceTemplate = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClipDefaultExperiences,
				ID:   templateID,
			},
		}
	}

	payload := AppClipDefaultExperienceCreateRequest{
		Data: AppClipDefaultExperienceCreateData{
			Type:          ResourceTypeAppClipDefaultExperiences,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipDefaultExperiences", body)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipDefaultExperience updates a default experience.
func (c *Client) UpdateAppClipDefaultExperience(ctx context.Context, experienceID string, attrs *AppClipDefaultExperienceUpdateAttributes, releaseVersionID string) (*AppClipDefaultExperienceResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	var relationships *AppClipDefaultExperienceUpdateRelationships
	releaseVersionID = strings.TrimSpace(releaseVersionID)
	if releaseVersionID != "" {
		relationships = &AppClipDefaultExperienceUpdateRelationships{
			ReleaseWithAppStoreVersion: &Relationship{
				Data: ResourceData{
					Type: ResourceTypeAppStoreVersions,
					ID:   releaseVersionID,
				},
			},
		}
	}

	payload := AppClipDefaultExperienceUpdateRequest{
		Data: AppClipDefaultExperienceUpdateData{
			Type:          ResourceTypeAppClipDefaultExperiences,
			ID:            experienceID,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s", experienceID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppClipDefaultExperience deletes a default experience by ID.
func (c *Client) DeleteAppClipDefaultExperience(ctx context.Context, experienceID string) error {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return fmt.Errorf("experienceID is required")
	}
	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s", experienceID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppClipDefaultExperienceLocalizations retrieves localizations for a default experience.
func (c *Client) GetAppClipDefaultExperienceLocalizations(ctx context.Context, experienceID string, opts ...AppClipDefaultExperienceLocalizationsOption) (*AppClipDefaultExperienceLocalizationsResponse, error) {
	query := &appClipDefaultExperienceLocalizationsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	path := fmt.Sprintf("/v1/appClipDefaultExperiences/%s/appClipDefaultExperienceLocalizations", experienceID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClipDefaultExperienceLocalizations: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppClipDefaultExperienceLocalizationsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceLocalizationsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperienceLocalization retrieves a localization by ID.
func (c *Client) GetAppClipDefaultExperienceLocalization(ctx context.Context, localizationID string) (*AppClipDefaultExperienceLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	path := fmt.Sprintf("/v1/appClipDefaultExperienceLocalizations/%s", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipDefaultExperienceLocalizationHeaderImageRelationship retrieves header image linkage for a localization.
func (c *Client) GetAppClipDefaultExperienceLocalizationHeaderImageRelationship(ctx context.Context, localizationID string) (*AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	path := fmt.Sprintf("/v1/appClipDefaultExperienceLocalizations/%s/relationships/appClipHeaderImage", localizationID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceLocalizationHeaderImageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipDefaultExperienceLocalization creates a localization.
func (c *Client) CreateAppClipDefaultExperienceLocalization(ctx context.Context, experienceID string, attrs AppClipDefaultExperienceLocalizationCreateAttributes) (*AppClipDefaultExperienceLocalizationResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	payload := AppClipDefaultExperienceLocalizationCreateRequest{
		Data: AppClipDefaultExperienceLocalizationCreateData{
			Type:       ResourceTypeAppClipDefaultExperienceLocalizations,
			Attributes: attrs,
			Relationships: &AppClipDefaultExperienceLocalizationRelationships{
				AppClipDefaultExperience: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppClipDefaultExperiences,
						ID:   experienceID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipDefaultExperienceLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipDefaultExperienceLocalization updates a localization.
func (c *Client) UpdateAppClipDefaultExperienceLocalization(ctx context.Context, localizationID string, attrs *AppClipDefaultExperienceLocalizationUpdateAttributes) (*AppClipDefaultExperienceLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := AppClipDefaultExperienceLocalizationUpdateRequest{
		Data: AppClipDefaultExperienceLocalizationUpdateData{
			Type:       ResourceTypeAppClipDefaultExperienceLocalizations,
			ID:         localizationID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipDefaultExperienceLocalizations/%s", localizationID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipDefaultExperienceLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppClipDefaultExperienceLocalization deletes a localization by ID.
func (c *Client) DeleteAppClipDefaultExperienceLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/appClipDefaultExperienceLocalizations/%s", localizationID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppClipAdvancedExperiences retrieves advanced experiences for an App Clip.
func (c *Client) GetAppClipAdvancedExperiences(ctx context.Context, appClipID string, opts ...AppClipAdvancedExperiencesOption) (*AppClipAdvancedExperiencesResponse, error) {
	query := &appClipAdvancedExperiencesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	appClipID = strings.TrimSpace(appClipID)
	if appClipID == "" {
		return nil, fmt.Errorf("appClipID is required")
	}

	path := fmt.Sprintf("/v1/appClips/%s/appClipAdvancedExperiences", appClipID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("appClipAdvancedExperiences: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAppClipAdvancedExperiencesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperiencesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetAppClipAdvancedExperience retrieves an advanced experience by ID.
func (c *Client) GetAppClipAdvancedExperience(ctx context.Context, experienceID string) (*AppClipAdvancedExperienceResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	path := fmt.Sprintf("/v1/appClipAdvancedExperiences/%s", experienceID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipAdvancedExperience creates an advanced experience.
func (c *Client) CreateAppClipAdvancedExperience(ctx context.Context, appClipID string, attrs AppClipAdvancedExperienceCreateAttributes, headerImageID string, localizationIDs []string) (*AppClipAdvancedExperienceResponse, error) {
	appClipID = strings.TrimSpace(appClipID)
	if appClipID == "" {
		return nil, fmt.Errorf("appClipID is required")
	}

	relationships := &AppClipAdvancedExperienceRelationships{
		AppClip: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClips,
				ID:   appClipID,
			},
		},
	}

	headerImageID = strings.TrimSpace(headerImageID)
	if headerImageID != "" {
		relationships.HeaderImage = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClipAdvancedExperienceImages,
				ID:   headerImageID,
			},
		}
	}

	if len(localizationIDs) > 0 {
		list := make([]ResourceData, 0, len(localizationIDs))
		for _, id := range localizationIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			list = append(list, ResourceData{
				Type: ResourceTypeAppClipAdvancedExperienceLocalizations,
				ID:   id,
			})
		}
		if len(list) > 0 {
			relationships.Localizations = &RelationshipList{Data: list}
		}
	}

	payload := AppClipAdvancedExperienceCreateRequest{
		Data: AppClipAdvancedExperienceCreateData{
			Type:          ResourceTypeAppClipAdvancedExperiences,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipAdvancedExperiences", body)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipAdvancedExperience updates an advanced experience.
func (c *Client) UpdateAppClipAdvancedExperience(ctx context.Context, experienceID string, attrs *AppClipAdvancedExperienceUpdateAttributes, appClipID string, headerImageID string, localizationIDs []string) (*AppClipAdvancedExperienceResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	var relationships *AppClipAdvancedExperienceRelationships
	appClipID = strings.TrimSpace(appClipID)
	if appClipID != "" {
		if relationships == nil {
			relationships = &AppClipAdvancedExperienceRelationships{}
		}
		relationships.AppClip = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClips,
				ID:   appClipID,
			},
		}
	}
	headerImageID = strings.TrimSpace(headerImageID)
	if headerImageID != "" {
		if relationships == nil {
			relationships = &AppClipAdvancedExperienceRelationships{}
		}
		relationships.HeaderImage = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeAppClipAdvancedExperienceImages,
				ID:   headerImageID,
			},
		}
	}
	if len(localizationIDs) > 0 {
		list := make([]ResourceData, 0, len(localizationIDs))
		for _, id := range localizationIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			list = append(list, ResourceData{
				Type: ResourceTypeAppClipAdvancedExperienceLocalizations,
				ID:   id,
			})
		}
		if len(list) > 0 {
			if relationships == nil {
				relationships = &AppClipAdvancedExperienceRelationships{}
			}
			relationships.Localizations = &RelationshipList{Data: list}
		}
	}

	payload := AppClipAdvancedExperienceUpdateRequest{
		Data: AppClipAdvancedExperienceUpdateData{
			Type:          ResourceTypeAppClipAdvancedExperiences,
			ID:            experienceID,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipAdvancedExperiences/%s", experienceID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppClipAdvancedExperience deletes an advanced experience by ID.
func (c *Client) DeleteAppClipAdvancedExperience(ctx context.Context, experienceID string) error {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return fmt.Errorf("experienceID is required")
	}
	path := fmt.Sprintf("/v1/appClipAdvancedExperiences/%s", experienceID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetAppClipAdvancedExperienceImage retrieves an advanced experience image by ID.
func (c *Client) GetAppClipAdvancedExperienceImage(ctx context.Context, imageID string) (*AppClipAdvancedExperienceImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	path := fmt.Sprintf("/v1/appClipAdvancedExperienceImages/%s", imageID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipAdvancedExperienceImage reserves an upload slot.
func (c *Client) CreateAppClipAdvancedExperienceImage(ctx context.Context, fileName string, fileSize int64) (*AppClipAdvancedExperienceImageResponse, error) {
	payload := AppClipAdvancedExperienceImageCreateRequest{
		Data: AppClipAdvancedExperienceImageCreateData{
			Type:       ResourceTypeAppClipAdvancedExperienceImages,
			Attributes: AppClipAdvancedExperienceImageCreateAttributes{FileName: fileName, FileSize: fileSize},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipAdvancedExperienceImages", body)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipAdvancedExperienceImage commits an upload.
func (c *Client) UpdateAppClipAdvancedExperienceImage(ctx context.Context, imageID string, uploaded bool) (*AppClipAdvancedExperienceImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	payload := AppClipAdvancedExperienceImageUpdateRequest{
		Data: AppClipAdvancedExperienceImageUpdateData{
			Type: ResourceTypeAppClipAdvancedExperienceImages,
			ID:   imageID,
			Attributes: &AppClipAdvancedExperienceImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipAdvancedExperienceImages/%s", imageID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipAdvancedExperienceImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppClipAdvancedExperienceImage deletes an image by ID.
func (c *Client) DeleteAppClipAdvancedExperienceImage(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("imageID is required")
	}
	path := fmt.Sprintf("/v1/appClipAdvancedExperienceImages/%s", imageID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadAppClipAdvancedExperienceImage performs the full upload flow for an image.
func (c *Client) UploadAppClipAdvancedExperienceImage(ctx context.Context, filePath string) (*AppClipAdvancedExperienceImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateAppClipAdvancedExperienceImage(ctx, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations
	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	committed, err := c.UpdateAppClipAdvancedExperienceImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &AppClipAdvancedExperienceImageUploadResult{
		ID:                 committed.Data.ID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetAppClipHeaderImage retrieves a header image by ID.
func (c *Client) GetAppClipHeaderImage(ctx context.Context, imageID string) (*AppClipHeaderImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	path := fmt.Sprintf("/v1/appClipHeaderImages/%s", imageID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipHeaderImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipHeaderImage reserves a header image upload.
func (c *Client) CreateAppClipHeaderImage(ctx context.Context, localizationID string, fileName string, fileSize int64) (*AppClipHeaderImageResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := AppClipHeaderImageCreateRequest{
		Data: AppClipHeaderImageCreateData{
			Type:       ResourceTypeAppClipHeaderImages,
			Attributes: AppClipHeaderImageCreateAttributes{FileName: fileName, FileSize: fileSize},
			Relationships: &AppClipHeaderImageRelationships{
				AppClipDefaultExperienceLocalization: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppClipDefaultExperienceLocalizations,
						ID:   localizationID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipHeaderImages", body)
	if err != nil {
		return nil, err
	}

	var response AppClipHeaderImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipHeaderImage commits a header image upload.
func (c *Client) UpdateAppClipHeaderImage(ctx context.Context, imageID string, uploaded bool) (*AppClipHeaderImageResponse, error) {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return nil, fmt.Errorf("imageID is required")
	}

	payload := AppClipHeaderImageUpdateRequest{
		Data: AppClipHeaderImageUpdateData{
			Type: ResourceTypeAppClipHeaderImages,
			ID:   imageID,
			Attributes: &AppClipHeaderImageUpdateAttributes{
				Uploaded: &uploaded,
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipHeaderImages/%s", imageID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipHeaderImageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteAppClipHeaderImage deletes a header image by ID.
func (c *Client) DeleteAppClipHeaderImage(ctx context.Context, imageID string) error {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return fmt.Errorf("imageID is required")
	}
	path := fmt.Sprintf("/v1/appClipHeaderImages/%s", imageID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// UploadAppClipHeaderImage performs the full upload flow for a header image.
func (c *Client) UploadAppClipHeaderImage(ctx context.Context, localizationID string, filePath string) (*AppClipHeaderImageUploadResult, error) {
	if err := ValidateImageFile(filePath); err != nil {
		return nil, fmt.Errorf("invalid image file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat file: %w", err)
	}
	fileName := info.Name()
	fileSize := info.Size()

	reservation, err := c.CreateAppClipHeaderImage(ctx, localizationID, fileName, fileSize)
	if err != nil {
		return nil, fmt.Errorf("failed to reserve image upload: %w", err)
	}

	imageID := reservation.Data.ID
	operations := reservation.Data.Attributes.UploadOperations
	if len(operations) == 0 {
		return nil, fmt.Errorf("no upload operations returned from API")
	}

	if err := UploadAsset(ctx, filePath, operations); err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	committed, err := c.UpdateAppClipHeaderImage(ctx, imageID, true)
	if err != nil {
		return nil, fmt.Errorf("failed to commit image upload: %w", err)
	}

	state := ""
	if committed.Data.Attributes.AssetDeliveryState != nil {
		state = committed.Data.Attributes.AssetDeliveryState.State
	}

	return &AppClipHeaderImageUploadResult{
		ID:                 committed.Data.ID,
		LocalizationID:     localizationID,
		FileName:           fileName,
		FileSize:           fileSize,
		AssetDeliveryState: state,
		Uploaded:           true,
	}, nil
}

// GetAppClipAppStoreReviewDetail retrieves review detail by ID.
func (c *Client) GetAppClipAppStoreReviewDetail(ctx context.Context, detailID string) (*AppClipAppStoreReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	path := fmt.Sprintf("/v1/appClipAppStoreReviewDetails/%s", detailID)
	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response AppClipAppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateAppClipAppStoreReviewDetail creates review detail for a default experience.
func (c *Client) CreateAppClipAppStoreReviewDetail(ctx context.Context, experienceID string, attrs *AppClipAppStoreReviewDetailCreateAttributes) (*AppClipAppStoreReviewDetailResponse, error) {
	experienceID = strings.TrimSpace(experienceID)
	if experienceID == "" {
		return nil, fmt.Errorf("experienceID is required")
	}

	payload := AppClipAppStoreReviewDetailCreateRequest{
		Data: AppClipAppStoreReviewDetailCreateData{
			Type:       ResourceTypeAppClipAppStoreReviewDetails,
			Attributes: attrs,
			Relationships: &AppClipAppStoreReviewDetailRelationships{
				AppClipDefaultExperience: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppClipDefaultExperiences,
						ID:   experienceID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/appClipAppStoreReviewDetails", body)
	if err != nil {
		return nil, err
	}

	var response AppClipAppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateAppClipAppStoreReviewDetail updates review detail by ID.
func (c *Client) UpdateAppClipAppStoreReviewDetail(ctx context.Context, detailID string, attrs *AppClipAppStoreReviewDetailUpdateAttributes) (*AppClipAppStoreReviewDetailResponse, error) {
	detailID = strings.TrimSpace(detailID)
	if detailID == "" {
		return nil, fmt.Errorf("detailID is required")
	}

	payload := AppClipAppStoreReviewDetailUpdateRequest{
		Data: AppClipAppStoreReviewDetailUpdateData{
			Type:       ResourceTypeAppClipAppStoreReviewDetails,
			ID:         detailID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/appClipAppStoreReviewDetails/%s", detailID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response AppClipAppStoreReviewDetailResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaAppClipInvocation retrieves a beta App Clip invocation by ID.
func (c *Client) GetBetaAppClipInvocation(ctx context.Context, invocationID string, opts ...BetaAppClipInvocationOption) (*BetaAppClipInvocationResponse, error) {
	query := &betaAppClipInvocationQuery{}
	for _, opt := range opts {
		opt(query)
	}

	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return nil, fmt.Errorf("invocationID is required")
	}

	path := fmt.Sprintf("/v1/betaAppClipInvocations/%s", invocationID)
	if queryString := buildBetaAppClipInvocationQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaAppClipInvocationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateBetaAppClipInvocation creates a beta App Clip invocation.
func (c *Client) CreateBetaAppClipInvocation(ctx context.Context, buildBundleID string, attrs BetaAppClipInvocationCreateAttributes, localizationIDs []string) (*BetaAppClipInvocationResponse, error) {
	buildBundleID = strings.TrimSpace(buildBundleID)
	if buildBundleID == "" {
		return nil, fmt.Errorf("buildBundleID is required")
	}

	relationships := &BetaAppClipInvocationRelationships{
		BuildBundle: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeBuildBundles,
				ID:   buildBundleID,
			},
		},
	}
	if len(localizationIDs) > 0 {
		list := make([]ResourceData, 0, len(localizationIDs))
		for _, id := range localizationIDs {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			list = append(list, ResourceData{
				Type: ResourceTypeBetaAppClipInvocationLocalizations,
				ID:   id,
			})
		}
		if len(list) > 0 {
			relationships.BetaAppClipInvocationLocalizations = &RelationshipList{Data: list}
		}
	}

	payload := BetaAppClipInvocationCreateRequest{
		Data: BetaAppClipInvocationCreateData{
			Type:          ResourceTypeBetaAppClipInvocations,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/betaAppClipInvocations", body)
	if err != nil {
		return nil, err
	}

	var response BetaAppClipInvocationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaAppClipInvocation updates a beta App Clip invocation.
func (c *Client) UpdateBetaAppClipInvocation(ctx context.Context, invocationID string, attrs *BetaAppClipInvocationUpdateAttributes) (*BetaAppClipInvocationResponse, error) {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return nil, fmt.Errorf("invocationID is required")
	}

	payload := BetaAppClipInvocationUpdateRequest{
		Data: BetaAppClipInvocationUpdateData{
			Type:       ResourceTypeBetaAppClipInvocations,
			ID:         invocationID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/betaAppClipInvocations/%s", invocationID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response BetaAppClipInvocationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaAppClipInvocation deletes a beta App Clip invocation.
func (c *Client) DeleteBetaAppClipInvocation(ctx context.Context, invocationID string) error {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return fmt.Errorf("invocationID is required")
	}
	path := fmt.Sprintf("/v1/betaAppClipInvocations/%s", invocationID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}

// GetBetaAppClipInvocationLocalizations retrieves localizations via the invocation include.
func (c *Client) GetBetaAppClipInvocationLocalizations(ctx context.Context, invocationID string, limit int) (*BetaAppClipInvocationLocalizationsResponse, error) {
	opts := []BetaAppClipInvocationOption{
		WithBetaAppClipInvocationInclude([]string{"betaAppClipInvocationLocalizations"}),
	}
	if limit > 0 {
		opts = append(opts, WithBetaAppClipInvocationLocalizationsLimit(limit))
	}

	invocation, err := c.GetBetaAppClipInvocation(ctx, invocationID, opts...)
	if err != nil {
		return nil, err
	}

	resp := &BetaAppClipInvocationLocalizationsResponse{
		Links: Links{},
		Data:  []Resource[BetaAppClipInvocationLocalizationAttributes]{},
	}
	if len(invocation.Included) == 0 {
		return resp, nil
	}

	var included []Resource[BetaAppClipInvocationLocalizationAttributes]
	if err := json.Unmarshal(invocation.Included, &included); err != nil {
		return nil, fmt.Errorf("failed to parse included localizations: %w", err)
	}
	resp.Data = included

	return resp, nil
}

// CreateBetaAppClipInvocationLocalization creates a localization.
func (c *Client) CreateBetaAppClipInvocationLocalization(ctx context.Context, invocationID string, attrs BetaAppClipInvocationLocalizationCreateAttributes) (*BetaAppClipInvocationLocalizationResponse, error) {
	invocationID = strings.TrimSpace(invocationID)
	if invocationID == "" {
		return nil, fmt.Errorf("invocationID is required")
	}

	payload := BetaAppClipInvocationLocalizationCreateRequest{
		Data: BetaAppClipInvocationLocalizationCreateData{
			Type:       ResourceTypeBetaAppClipInvocationLocalizations,
			Attributes: attrs,
			Relationships: &BetaAppClipInvocationLocalizationRelationships{
				BetaAppClipInvocation: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeBetaAppClipInvocations,
						ID:   invocationID,
					},
				},
			},
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, http.MethodPost, "/v1/betaAppClipInvocationLocalizations", body)
	if err != nil {
		return nil, err
	}

	var response BetaAppClipInvocationLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// UpdateBetaAppClipInvocationLocalization updates a localization.
func (c *Client) UpdateBetaAppClipInvocationLocalization(ctx context.Context, localizationID string, attrs *BetaAppClipInvocationLocalizationUpdateAttributes) (*BetaAppClipInvocationLocalizationResponse, error) {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return nil, fmt.Errorf("localizationID is required")
	}

	payload := BetaAppClipInvocationLocalizationUpdateRequest{
		Data: BetaAppClipInvocationLocalizationUpdateData{
			Type:       ResourceTypeBetaAppClipInvocationLocalizations,
			ID:         localizationID,
			Attributes: attrs,
		},
	}

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/betaAppClipInvocationLocalizations/%s", localizationID)
	data, err := c.do(ctx, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}

	var response BetaAppClipInvocationLocalizationResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// DeleteBetaAppClipInvocationLocalization deletes a localization.
func (c *Client) DeleteBetaAppClipInvocationLocalization(ctx context.Context, localizationID string) error {
	localizationID = strings.TrimSpace(localizationID)
	if localizationID == "" {
		return fmt.Errorf("localizationID is required")
	}
	path := fmt.Sprintf("/v1/betaAppClipInvocationLocalizations/%s", localizationID)
	_, err := c.do(ctx, http.MethodDelete, path, nil)
	return err
}
