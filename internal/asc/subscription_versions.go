package asc

import (
	"net/url"
	"strconv"
	"strings"
)

// SubscriptionVersionState is the App Store review state of a subscription version.
type SubscriptionVersionState string

const (
	SubscriptionVersionStatePrepareForSubmission   SubscriptionVersionState = "PREPARE_FOR_SUBMISSION"
	SubscriptionVersionStateReadyForReview         SubscriptionVersionState = "READY_FOR_REVIEW"
	SubscriptionVersionStateWaitingForReview       SubscriptionVersionState = "WAITING_FOR_REVIEW"
	SubscriptionVersionStateInReview               SubscriptionVersionState = "IN_REVIEW"
	SubscriptionVersionStateAccepted               SubscriptionVersionState = "ACCEPTED"
	SubscriptionVersionStateApproved               SubscriptionVersionState = "APPROVED"
	SubscriptionVersionStateReplacedWithNewVersion SubscriptionVersionState = "REPLACED_WITH_NEW_VERSION"
	SubscriptionVersionStateRejected               SubscriptionVersionState = "REJECTED"
	SubscriptionVersionStateDeveloperRejected      SubscriptionVersionState = "DEVELOPER_REJECTED"
)

// SubscriptionVersionAttributes describes a subscription version resource.
type SubscriptionVersionAttributes struct {
	Version int                      `json:"version,omitempty"`
	State   SubscriptionVersionState `json:"state,omitempty"`
}

// SubscriptionVersionRelationships describes relationships used to create a version.
type SubscriptionVersionRelationships struct {
	Subscription *Relationship `json:"subscription"`
}

// SubscriptionVersionCreateData is the data portion of a version create request.
type SubscriptionVersionCreateData struct {
	Type          ResourceType                      `json:"type"`
	Relationships *SubscriptionVersionRelationships `json:"relationships"`
}

// SubscriptionVersionCreateRequest creates a version for a subscription.
type SubscriptionVersionCreateRequest struct {
	Data SubscriptionVersionCreateData `json:"data"`
}

// SubscriptionLocalizationV2Attributes describes version-scoped localization metadata.
type SubscriptionLocalizationV2Attributes struct {
	Name        string `json:"name,omitempty"`
	Locale      string `json:"locale,omitempty"`
	Description string `json:"description,omitempty"`
}

// SubscriptionLocalizationV2CreateAttributes describes localization create attributes.
type SubscriptionLocalizationV2CreateAttributes struct {
	Name        string  `json:"name"`
	Locale      string  `json:"locale"`
	Description *string `json:"description,omitempty"`
}

// SubscriptionLocalizationV2UpdateAttributes describes localization update attributes.
type SubscriptionLocalizationV2UpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// SubscriptionVersionRelationship identifies the owning version.
type SubscriptionVersionRelationship struct {
	Version *Relationship `json:"version"`
}

// SubscriptionLocalizationV2CreateData is the data portion of a localization create request.
type SubscriptionLocalizationV2CreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    SubscriptionLocalizationV2CreateAttributes `json:"attributes"`
	Relationships *SubscriptionVersionRelationship           `json:"relationships"`
}

// SubscriptionLocalizationV2CreateRequest creates a version-scoped localization.
type SubscriptionLocalizationV2CreateRequest struct {
	Data SubscriptionLocalizationV2CreateData `json:"data"`
}

// SubscriptionLocalizationV2UpdateData is the data portion of a localization update request.
type SubscriptionLocalizationV2UpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes SubscriptionLocalizationV2UpdateAttributes `json:"attributes,omitempty"`
}

// SubscriptionLocalizationV2UpdateRequest updates a version-scoped localization.
type SubscriptionLocalizationV2UpdateRequest struct {
	Data SubscriptionLocalizationV2UpdateData `json:"data"`
}

// SubscriptionImageV2Attributes describes a version-scoped subscription image.
type SubscriptionImageV2Attributes struct {
	FileSize           int64               `json:"fileSize,omitempty"`
	FileName           string              `json:"fileName,omitempty"`
	AssetToken         string              `json:"assetToken,omitempty"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AppMediaAssetState `json:"assetDeliveryState,omitempty"`
}

// SubscriptionImageV2CreateAttributes describes image reservation attributes.
type SubscriptionImageV2CreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// SubscriptionImageV2UpdateAttributes describes the upload commit state.
type SubscriptionImageV2UpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// SubscriptionImageV2CreateData is the data portion of an image create request.
type SubscriptionImageV2CreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    SubscriptionImageV2CreateAttributes `json:"attributes"`
	Relationships *SubscriptionVersionRelationship    `json:"relationships"`
}

// SubscriptionImageV2CreateRequest reserves a version-scoped image upload.
type SubscriptionImageV2CreateRequest struct {
	Data SubscriptionImageV2CreateData `json:"data"`
}

// SubscriptionImageV2UpdateData is the data portion of an image update request.
type SubscriptionImageV2UpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes SubscriptionImageV2UpdateAttributes `json:"attributes,omitempty"`
}

// SubscriptionImageV2UpdateRequest commits a version-scoped image upload.
type SubscriptionImageV2UpdateRequest struct {
	Data SubscriptionImageV2UpdateData `json:"data"`
}

type (
	SubscriptionVersionsResponse        = Response[SubscriptionVersionAttributes]
	SubscriptionVersionResponse         = SingleResponse[SubscriptionVersionAttributes]
	SubscriptionLocalizationsV2Response = Response[SubscriptionLocalizationV2Attributes]
	SubscriptionLocalizationV2Response  = SingleResponse[SubscriptionLocalizationV2Attributes]
	SubscriptionImagesV2Response        = Response[SubscriptionImageV2Attributes]
	SubscriptionImageV2Response         = SingleResponse[SubscriptionImageV2Attributes]
)

// SubscriptionVersionImageLinkageResponse is a singular image relationship response.
type SubscriptionVersionImageLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// SubscriptionVersionsOption configures a subscription's version list.
type SubscriptionVersionsOption func(*subscriptionVersionsQuery)

// SubscriptionVersionOption configures a subscription version detail request.
type SubscriptionVersionOption func(*subscriptionVersionQuery)

// SubscriptionVersionLocalizationsOption configures version localization reads.
type SubscriptionVersionLocalizationsOption func(*subscriptionVersionLocalizationsQuery)

// SubscriptionVersionImagesOption configures version image reads.
type SubscriptionVersionImagesOption func(*subscriptionVersionImagesQuery)

// SubscriptionLocalizationV2Option configures a v2 localization detail request.
type SubscriptionLocalizationV2Option func(*subscriptionLocalizationV2Query)

// SubscriptionImageV2Option configures a v2 image detail request.
type SubscriptionImageV2Option func(*subscriptionImageV2Query)

type subscriptionVersionsQuery struct {
	listQuery
	states             []SubscriptionVersionState
	fields             []string
	subscriptionFields []string
	imageFields        []string
	localizationFields []string
	include            []string
	imageLimit         int
	localizationLimit  int
}

type subscriptionVersionQuery struct {
	fields             []string
	subscriptionFields []string
	imageFields        []string
	localizationFields []string
	include            []string
	imageLimit         int
	localizationLimit  int
}

type subscriptionVersionLocalizationsQuery struct {
	listQuery
	fields        []string
	versionFields []string
	include       []string
}

type subscriptionVersionImagesQuery struct {
	listQuery
	fields []string
}

type subscriptionLocalizationV2Query struct {
	fields        []string
	versionFields []string
	include       []string
}

type subscriptionImageV2Query struct {
	fields []string
}

func WithSubscriptionVersionsLimit(limit int) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithSubscriptionVersionsNextURL(next string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithSubscriptionVersionsStates(states []SubscriptionVersionState) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.states = states }
}

func WithSubscriptionVersionsFields(fields []string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.fields = normalizeList(fields) }
}

func WithSubscriptionVersionsSubscriptionFields(fields []string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.subscriptionFields = normalizeList(fields) }
}

func WithSubscriptionVersionsImageFields(fields []string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.imageFields = normalizeList(fields) }
}

func WithSubscriptionVersionsLocalizationFields(fields []string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.localizationFields = normalizeList(fields) }
}

func WithSubscriptionVersionsInclude(include []string) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) { q.include = normalizeList(include) }
}

func WithSubscriptionVersionsImageLimit(limit int) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) {
		if limit > 0 {
			q.imageLimit = limit
		}
	}
}

func WithSubscriptionVersionsLocalizationLimit(limit int) SubscriptionVersionsOption {
	return func(q *subscriptionVersionsQuery) {
		if limit > 0 {
			q.localizationLimit = limit
		}
	}
}

func WithSubscriptionVersionFields(fields []string) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) { q.fields = normalizeList(fields) }
}

func WithSubscriptionVersionSubscriptionFields(fields []string) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) { q.subscriptionFields = normalizeList(fields) }
}

func WithSubscriptionVersionImageFields(fields []string) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) { q.imageFields = normalizeList(fields) }
}

func WithSubscriptionVersionLocalizationFields(fields []string) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) { q.localizationFields = normalizeList(fields) }
}

func WithSubscriptionVersionInclude(include []string) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) { q.include = normalizeList(include) }
}

func WithSubscriptionVersionImageLimit(limit int) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) {
		if limit > 0 {
			q.imageLimit = limit
		}
	}
}

func WithSubscriptionVersionLocalizationLimit(limit int) SubscriptionVersionOption {
	return func(q *subscriptionVersionQuery) {
		if limit > 0 {
			q.localizationLimit = limit
		}
	}
}

func WithSubscriptionVersionLocalizationsLimit(limit int) SubscriptionVersionLocalizationsOption {
	return func(q *subscriptionVersionLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithSubscriptionVersionLocalizationsNextURL(next string) SubscriptionVersionLocalizationsOption {
	return func(q *subscriptionVersionLocalizationsQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithSubscriptionVersionLocalizationsFields(fields []string) SubscriptionVersionLocalizationsOption {
	return func(q *subscriptionVersionLocalizationsQuery) { q.fields = normalizeList(fields) }
}

func WithSubscriptionVersionLocalizationsVersionFields(fields []string) SubscriptionVersionLocalizationsOption {
	return func(q *subscriptionVersionLocalizationsQuery) { q.versionFields = normalizeList(fields) }
}

func WithSubscriptionVersionLocalizationsInclude(include []string) SubscriptionVersionLocalizationsOption {
	return func(q *subscriptionVersionLocalizationsQuery) { q.include = normalizeList(include) }
}

func WithSubscriptionVersionImagesLimit(limit int) SubscriptionVersionImagesOption {
	return func(q *subscriptionVersionImagesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

func WithSubscriptionVersionImagesNextURL(next string) SubscriptionVersionImagesOption {
	return func(q *subscriptionVersionImagesQuery) { q.nextURL = strings.TrimSpace(next) }
}

func WithSubscriptionVersionImagesFields(fields []string) SubscriptionVersionImagesOption {
	return func(q *subscriptionVersionImagesQuery) { q.fields = normalizeList(fields) }
}

func WithSubscriptionLocalizationV2Fields(fields []string) SubscriptionLocalizationV2Option {
	return func(q *subscriptionLocalizationV2Query) { q.fields = normalizeList(fields) }
}

func WithSubscriptionLocalizationV2VersionFields(fields []string) SubscriptionLocalizationV2Option {
	return func(q *subscriptionLocalizationV2Query) { q.versionFields = normalizeList(fields) }
}

func WithSubscriptionLocalizationV2Include(include []string) SubscriptionLocalizationV2Option {
	return func(q *subscriptionLocalizationV2Query) { q.include = normalizeList(include) }
}

func WithSubscriptionImageV2Fields(fields []string) SubscriptionImageV2Option {
	return func(q *subscriptionImageV2Query) { q.fields = normalizeList(fields) }
}

func buildSubscriptionVersionsQuery(q *subscriptionVersionsQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	states := make([]string, 0, len(q.states))
	for _, state := range q.states {
		states = append(states, string(state))
	}
	addCSV(values, "filter[state]", states)
	addCSV(values, "fields[subscriptionVersions]", q.fields)
	addCSV(values, "fields[subscriptions]", q.subscriptionFields)
	addCSV(values, "fields[subscriptionImages]", q.imageFields)
	addCSV(values, "fields[subscriptionLocalizations]", q.localizationFields)
	addCSV(values, "include", q.include)
	addNamedLimit(values, "limit[images]", q.imageLimit)
	addNamedLimit(values, "limit[localizations]", q.localizationLimit)
	return values.Encode()
}

func buildSubscriptionVersionQuery(q *subscriptionVersionQuery) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionVersions]", q.fields)
	addCSV(values, "fields[subscriptions]", q.subscriptionFields)
	addCSV(values, "fields[subscriptionImages]", q.imageFields)
	addCSV(values, "fields[subscriptionLocalizations]", q.localizationFields)
	addCSV(values, "include", q.include)
	addNamedLimit(values, "limit[images]", q.imageLimit)
	addNamedLimit(values, "limit[localizations]", q.localizationLimit)
	return values.Encode()
}

func buildSubscriptionVersionLocalizationsQuery(q *subscriptionVersionLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "fields[subscriptionLocalizations]", q.fields)
	addCSV(values, "fields[subscriptionVersions]", q.versionFields)
	addCSV(values, "include", q.include)
	return values.Encode()
}

func buildSubscriptionVersionImagesQuery(q *subscriptionVersionImagesQuery) string {
	values := url.Values{}
	addLimit(values, q.limit)
	addCSV(values, "fields[subscriptionImages]", q.fields)
	return values.Encode()
}

func buildSubscriptionLocalizationV2Query(q *subscriptionLocalizationV2Query) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionLocalizations]", q.fields)
	addCSV(values, "fields[subscriptionVersions]", q.versionFields)
	addCSV(values, "include", q.include)
	return values.Encode()
}

func buildSubscriptionImageV2Query(q *subscriptionImageV2Query) string {
	values := url.Values{}
	addCSV(values, "fields[subscriptionImages]", q.fields)
	return values.Encode()
}

func addNamedLimit(values url.Values, key string, limit int) {
	if limit > 0 {
		values.Set(key, strconv.Itoa(limit))
	}
}
