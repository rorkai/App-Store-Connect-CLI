package asc

import (
	"net/url"
)

// AppEventBadge represents the app event badge/event type.
type AppEventBadge string

// Supported app event badge values.
const (
	AppEventBadgeLiveEvent    AppEventBadge = "LIVE_EVENT"
	AppEventBadgePremiere     AppEventBadge = "PREMIERE"
	AppEventBadgeChallenge    AppEventBadge = "CHALLENGE"
	AppEventBadgeCompetition  AppEventBadge = "COMPETITION"
	AppEventBadgeNewSeason    AppEventBadge = "NEW_SEASON"
	AppEventBadgeMajorUpdate  AppEventBadge = "MAJOR_UPDATE"
	AppEventBadgeSpecialEvent AppEventBadge = "SPECIAL_EVENT"
)

// ValidAppEventBadges lists supported app event badge values.
var ValidAppEventBadges = []string{
	string(AppEventBadgeLiveEvent),
	string(AppEventBadgePremiere),
	string(AppEventBadgeChallenge),
	string(AppEventBadgeCompetition),
	string(AppEventBadgeNewSeason),
	string(AppEventBadgeMajorUpdate),
	string(AppEventBadgeSpecialEvent),
}

// AppEventPriority represents the app event priority.
type AppEventPriority string

const (
	AppEventPriorityHigh   AppEventPriority = "HIGH"
	AppEventPriorityNormal AppEventPriority = "NORMAL"
)

// ValidAppEventPriorities lists supported app event priorities.
var ValidAppEventPriorities = []string{
	string(AppEventPriorityHigh),
	string(AppEventPriorityNormal),
}

// AppEventPurpose represents the app event purpose.
type AppEventPurpose string

const (
	AppEventPurposeAllUsers        AppEventPurpose = "APPROPRIATE_FOR_ALL_USERS"
	AppEventPurposeAttractNewUsers AppEventPurpose = "ATTRACT_NEW_USERS"
	AppEventPurposeKeepInformed    AppEventPurpose = "KEEP_ACTIVE_USERS_INFORMED"
	AppEventPurposeBringBackUsers  AppEventPurpose = "BRING_BACK_LAPSED_USERS"
)

// ValidAppEventPurposes lists supported app event purposes.
var ValidAppEventPurposes = []string{
	string(AppEventPurposeAllUsers),
	string(AppEventPurposeAttractNewUsers),
	string(AppEventPurposeKeepInformed),
	string(AppEventPurposeBringBackUsers),
}

// AppEventPurchaseRequirement represents app event purchase requirement.
type AppEventPurchaseRequirement string

const (
	AppEventPurchaseRequirementNoCostAssociated AppEventPurchaseRequirement = "NO_COST_ASSOCIATED"
	AppEventPurchaseRequirementNoIAPRequired    AppEventPurchaseRequirement = "NO_IAP_REQUIRED"
	AppEventPurchaseRequirementIAPRequired      AppEventPurchaseRequirement = "IAP_REQUIRED"
)

// ValidAppEventPurchaseRequirements lists supported purchase requirement values.
var ValidAppEventPurchaseRequirements = []string{
	string(AppEventPurchaseRequirementNoCostAssociated),
	string(AppEventPurchaseRequirementNoIAPRequired),
	string(AppEventPurchaseRequirementIAPRequired),
}

// AppEventAssetType represents app event asset types.
type AppEventAssetType string

const (
	AppEventAssetTypeEventCard        AppEventAssetType = "EVENT_CARD"
	AppEventAssetTypeEventDetailsPage AppEventAssetType = "EVENT_DETAILS_PAGE"
)

// ValidAppEventAssetTypes lists supported app event asset types.
var ValidAppEventAssetTypes = []string{
	string(AppEventAssetTypeEventCard),
	string(AppEventAssetTypeEventDetailsPage),
}

// AppEventTerritorySchedule represents a schedule for app events in territories.
type AppEventTerritorySchedule struct {
	Territories  []string `json:"territories,omitempty"`
	PublishStart string   `json:"publishStart,omitempty"`
	EventStart   string   `json:"eventStart,omitempty"`
	EventEnd     string   `json:"eventEnd,omitempty"`
}

// AppEventAttributes represents app event attributes.
type AppEventAttributes struct {
	ReferenceName              string                      `json:"referenceName,omitempty"`
	Badge                      string                      `json:"badge,omitempty"`
	EventState                 string                      `json:"eventState,omitempty"`
	DeepLink                   string                      `json:"deepLink,omitempty"`
	PurchaseRequirement        string                      `json:"purchaseRequirement,omitempty"`
	PrimaryLocale              string                      `json:"primaryLocale,omitempty"`
	Priority                   string                      `json:"priority,omitempty"`
	Purpose                    string                      `json:"purpose,omitempty"`
	TerritorySchedules         []AppEventTerritorySchedule `json:"territorySchedules,omitempty"`
	ArchivedTerritorySchedules []AppEventTerritorySchedule `json:"archivedTerritorySchedules,omitempty"`
}

// AppEventCreateAttributes describes attributes for creating an app event.
type AppEventCreateAttributes struct {
	ReferenceName       string                      `json:"referenceName"`
	Badge               string                      `json:"badge,omitempty"`
	DeepLink            string                      `json:"deepLink,omitempty"`
	PurchaseRequirement string                      `json:"purchaseRequirement,omitempty"`
	PrimaryLocale       string                      `json:"primaryLocale,omitempty"`
	Priority            string                      `json:"priority,omitempty"`
	Purpose             string                      `json:"purpose,omitempty"`
	TerritorySchedules  []AppEventTerritorySchedule `json:"territorySchedules,omitempty"`
}

// AppEventUpdateAttributes describes attributes for updating an app event.
type AppEventUpdateAttributes struct {
	ReferenceName       *string                     `json:"referenceName,omitempty"`
	Badge               *string                     `json:"badge,omitempty"`
	DeepLink            *string                     `json:"deepLink,omitempty"`
	PurchaseRequirement *string                     `json:"purchaseRequirement,omitempty"`
	PrimaryLocale       *string                     `json:"primaryLocale,omitempty"`
	Priority            *string                     `json:"priority,omitempty"`
	Purpose             *string                     `json:"purpose,omitempty"`
	TerritorySchedules  []AppEventTerritorySchedule `json:"territorySchedules,omitempty"`
}

// AppEventLocalizationCreateAttributes describes attributes for creating localizations.
type AppEventLocalizationCreateAttributes struct {
	Locale           string `json:"locale"`
	Name             string `json:"name,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	LongDescription  string `json:"longDescription,omitempty"`
}

// AppEventScreenshotAttributes represents app event screenshot attributes.
type AppEventScreenshotAttributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
	AppEventAssetType  string              `json:"appEventAssetType,omitempty"`
}

// AppMediaVideoState represents the state of a video asset.
type AppMediaVideoState struct {
	State    *string       `json:"state,omitempty"`
	Errors   []StateDetail `json:"errors,omitempty"`
	Warnings []StateDetail `json:"warnings,omitempty"`
}

// AppMediaPreviewFrameImageState represents the state of a preview frame image.
type AppMediaPreviewFrameImageState struct {
	State    string        `json:"state,omitempty"`
	Errors   []StateDetail `json:"errors,omitempty"`
	Warnings []StateDetail `json:"warnings,omitempty"`
}

// PreviewFrameImage represents a preview frame image.
type PreviewFrameImage struct {
	Image *ImageAsset                     `json:"image,omitempty"`
	State *AppMediaPreviewFrameImageState `json:"state,omitempty"`
}

// AppEventVideoClipAttributes represents app event video clip attributes.
type AppEventVideoClipAttributes struct {
	FileSize             int64               `json:"fileSize,omitempty"`
	FileName             string              `json:"fileName,omitempty"`
	PreviewFrameTimeCode string              `json:"previewFrameTimeCode,omitempty"`
	VideoURL             string              `json:"videoUrl,omitempty"`
	PreviewFrameImage    *PreviewFrameImage  `json:"previewFrameImage,omitempty"`
	PreviewImage         *ImageAsset         `json:"previewImage,omitempty"`
	UploadOperations     []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState   *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
	VideoDeliveryState   *AppMediaVideoState `json:"videoDeliveryState,omitempty"`
	AppEventAssetType    string              `json:"appEventAssetType,omitempty"`
}

// Response types.
type (
	AppEventsResponse                               = Response[AppEventAttributes]
	AppEventResponse                                = SingleResponse[AppEventAttributes]
	AppEventLocalizationsResponse                   = Response[AppEventLocalizationAttributes]
	AppEventLocalizationResponse                    = SingleResponse[AppEventLocalizationAttributes]
	AppEventScreenshotsResponse                     = Response[AppEventScreenshotAttributes]
	AppEventScreenshotResponse                      = SingleResponse[AppEventScreenshotAttributes]
	AppEventVideoClipsResponse                      = Response[AppEventVideoClipAttributes]
	AppEventVideoClipResponse                       = SingleResponse[AppEventVideoClipAttributes]
	AppEventLocalizationsLinkagesResponse           = LinkagesResponse
	AppEventLocalizationScreenshotsLinkagesResponse = LinkagesResponse
	AppEventLocalizationVideoClipsLinkagesResponse  = LinkagesResponse
)

// AppEventLocalizationsOption is a functional option for app event localizations list.
type AppEventLocalizationsOption func(*appEventLocalizationsQuery)

type appEventLocalizationsQuery struct {
	listQuery
	localizationRelatedQuery
}

// WithAppEventLocalizationsInclude includes related resources.
func WithAppEventLocalizationsInclude(include []string) AppEventLocalizationsOption {
	return func(q *appEventLocalizationsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppEventLocalizationsPlacementsLimit sets limit[placements].
func WithAppEventLocalizationsPlacementsLimit(limit int) AppEventLocalizationsOption {
	return func(q *appEventLocalizationsQuery) {
		q.placementsLimit = limit
	}
}

func buildAppEventLocalizationsQuery(query *appEventLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	query.addTo(values)
	return values.Encode()
}
