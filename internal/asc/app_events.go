package asc

import (
	"net/url"
	"strconv"
	"strings"
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

// AppEventLocalizationAttributes represents app event localization attributes.
type AppEventLocalizationAttributes struct {
	Locale           string `json:"locale,omitempty"`
	Name             string `json:"name,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	LongDescription  string `json:"longDescription,omitempty"`
}

// AppEventLocalizationCreateAttributes describes attributes for creating localizations.
type AppEventLocalizationCreateAttributes struct {
	Locale           string `json:"locale"`
	Name             string `json:"name,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	LongDescription  string `json:"longDescription,omitempty"`
}

// AppEventLocalizationUpdateAttributes describes attributes for updating localizations.
type AppEventLocalizationUpdateAttributes struct {
	Name             *string `json:"name,omitempty"`
	ShortDescription *string `json:"shortDescription,omitempty"`
	LongDescription  *string `json:"longDescription,omitempty"`
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

// AppEventsOption is a functional option for app event list endpoints.
type AppEventsOption func(*appEventsQuery)

// AppEventLocalizationsOption is a functional option for app event localizations list.
type AppEventLocalizationsOption func(*appEventLocalizationsQuery)

// AppEventScreenshotsOption is a functional option for app event screenshots list.
type AppEventScreenshotsOption func(*appEventScreenshotsQuery)

// AppEventVideoClipsOption is a functional option for app event video clips list.
type AppEventVideoClipsOption func(*appEventVideoClipsQuery)

type appEventsQuery struct {
	listQuery
	states             []string
	ids                []string
	fields             []string
	localizationFields []string
	include            []string
	localizationsLimit int
}

type appEventLocalizationsQuery struct {
	listQuery
	localizationRelatedQuery
}

type appEventScreenshotsQuery struct {
	listQuery
}

type appEventVideoClipsQuery struct {
	listQuery
}

// WithAppEventsLimit sets the max number of app events to return.
func WithAppEventsLimit(limit int) AppEventsOption {
	return func(q *appEventsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppEventsNextURL uses a next page URL directly.
func WithAppEventsNextURL(next string) AppEventsOption {
	return func(q *appEventsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppEventsStates filters app events by lifecycle state(s).
func WithAppEventsStates(states []string) AppEventsOption {
	return func(q *appEventsQuery) {
		q.states = normalizeUpperList(states)
	}
}

// WithAppEventsIDs filters app events by ID(s).
func WithAppEventsIDs(ids []string) AppEventsOption {
	return func(q *appEventsQuery) {
		q.ids = normalizeList(ids)
	}
}

// WithAppEventsFields sets fields[appEvents] for app event responses.
func WithAppEventsFields(fields []string) AppEventsOption {
	return func(q *appEventsQuery) {
		q.fields = normalizeList(fields)
	}
}

// WithAppEventsLocalizationFields sets fields[appEventLocalizations] for included localizations.
func WithAppEventsLocalizationFields(fields []string) AppEventsOption {
	return func(q *appEventsQuery) {
		q.localizationFields = normalizeList(fields)
	}
}

// WithAppEventsInclude includes related resources in app event responses.
func WithAppEventsInclude(include []string) AppEventsOption {
	return func(q *appEventsQuery) {
		q.include = normalizeList(include)
	}
}

// WithAppEventsLocalizationsLimit sets the maximum number of included localizations.
func WithAppEventsLocalizationsLimit(limit int) AppEventsOption {
	return func(q *appEventsQuery) {
		if limit > 0 {
			q.localizationsLimit = limit
		}
	}
}

// WithAppEventLocalizationsLimit sets the max number of localizations to return.
func WithAppEventLocalizationsLimit(limit int) AppEventLocalizationsOption {
	return func(q *appEventLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
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

// WithAppEventLocalizationsNextURL uses a next page URL directly.
func WithAppEventLocalizationsNextURL(next string) AppEventLocalizationsOption {
	return func(q *appEventLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppEventScreenshotsLimit sets the max number of screenshots to return.
func WithAppEventScreenshotsLimit(limit int) AppEventScreenshotsOption {
	return func(q *appEventScreenshotsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppEventScreenshotsNextURL uses a next page URL directly.
func WithAppEventScreenshotsNextURL(next string) AppEventScreenshotsOption {
	return func(q *appEventScreenshotsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

// WithAppEventVideoClipsLimit sets the max number of video clips to return.
func WithAppEventVideoClipsLimit(limit int) AppEventVideoClipsOption {
	return func(q *appEventVideoClipsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithAppEventVideoClipsNextURL uses a next page URL directly.
func WithAppEventVideoClipsNextURL(next string) AppEventVideoClipsOption {
	return func(q *appEventVideoClipsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildAppEventsQuery(query *appEventsQuery) string {
	values := url.Values{}
	addCSV(values, "filter[eventState]", query.states)
	addCSV(values, "filter[id]", query.ids)
	addCSV(values, "fields[appEvents]", query.fields)
	addCSV(values, "fields[appEventLocalizations]", query.localizationFields)
	addCSV(values, "include", query.include)
	addLimit(values, query.limit)
	if query.localizationsLimit > 0 {
		values.Set("limit[localizations]", strconv.Itoa(query.localizationsLimit))
	}
	return values.Encode()
}

func buildAppEventLocalizationsQuery(query *appEventLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	query.addTo(values)
	return values.Encode()
}

func buildAppEventScreenshotsQuery(query *appEventScreenshotsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

func buildAppEventVideoClipsQuery(query *appEventVideoClipsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
