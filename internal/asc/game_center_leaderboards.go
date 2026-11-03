package asc

import (
	"net/url"
	"strings"
)

// GameCenterLeaderboardAttributes represents a Game Center leaderboard resource.
type GameCenterLeaderboardAttributes struct {
	ReferenceName       string            `json:"referenceName"`
	VendorIdentifier    string            `json:"vendorIdentifier"`
	DefaultFormatter    string            `json:"defaultFormatter"`
	ScoreSortType       string            `json:"scoreSortType"`
	ScoreRangeStart     string            `json:"scoreRangeStart,omitempty"`
	ScoreRangeEnd       string            `json:"scoreRangeEnd,omitempty"`
	RecurrenceStartDate string            `json:"recurrenceStartDate,omitempty"`
	RecurrenceDuration  string            `json:"recurrenceDuration,omitempty"`
	RecurrenceRule      string            `json:"recurrenceRule,omitempty"`
	SubmissionType      string            `json:"submissionType"`
	Archived            bool              `json:"archived,omitempty"`
	ActivityProperties  map[string]string `json:"activityProperties,omitempty"`
	Visibility          string            `json:"visibility,omitempty"`
}

// GameCenterLeaderboardCreateAttributes describes attributes for creating a leaderboard.
type GameCenterLeaderboardCreateAttributes struct {
	ReferenceName       string            `json:"referenceName"`
	VendorIdentifier    string            `json:"vendorIdentifier"`
	DefaultFormatter    string            `json:"defaultFormatter"`
	ScoreSortType       string            `json:"scoreSortType"`
	ScoreRangeStart     string            `json:"scoreRangeStart,omitempty"`
	ScoreRangeEnd       string            `json:"scoreRangeEnd,omitempty"`
	RecurrenceStartDate string            `json:"recurrenceStartDate,omitempty"`
	RecurrenceDuration  string            `json:"recurrenceDuration,omitempty"`
	RecurrenceRule      string            `json:"recurrenceRule,omitempty"`
	SubmissionType      string            `json:"submissionType"`
	ActivityProperties  map[string]string `json:"activityProperties,omitzero"`
	Visibility          string            `json:"visibility,omitempty"`
}

// GameCenterLeaderboardUpdateAttributes describes attributes for updating a leaderboard.
type GameCenterLeaderboardUpdateAttributes struct {
	ReferenceName       *string           `json:"referenceName,omitempty"`
	DefaultFormatter    *string           `json:"defaultFormatter,omitempty"`
	ScoreSortType       *string           `json:"scoreSortType,omitempty"`
	ScoreRangeStart     *string           `json:"scoreRangeStart,omitempty"`
	ScoreRangeEnd       *string           `json:"scoreRangeEnd,omitempty"`
	RecurrenceStartDate *string           `json:"recurrenceStartDate,omitempty"`
	RecurrenceDuration  *string           `json:"recurrenceDuration,omitempty"`
	RecurrenceRule      *string           `json:"recurrenceRule,omitempty"`
	SubmissionType      *string           `json:"submissionType,omitempty"`
	Archived            *bool             `json:"archived,omitempty"`
	ActivityProperties  map[string]string `json:"activityProperties,omitzero"`
	Visibility          *string           `json:"visibility,omitempty"`
}

// GameCenterLeaderboardRelationships describes relationships for leaderboards.
type GameCenterLeaderboardRelationships struct {
	GameCenterDetail          *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterLeaderboardSets *RelationshipList `json:"gameCenterLeaderboardSets,omitempty"`
}

// GameCenterLeaderboardV2Relationships describes relationships for v2 leaderboards.
type GameCenterLeaderboardV2Relationships struct {
	GameCenterDetail          *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterGroup           *Relationship     `json:"gameCenterGroup,omitempty"`
	GameCenterLeaderboardSets *RelationshipList `json:"gameCenterLeaderboardSets,omitempty"`
	Versions                  *RelationshipList `json:"versions"`
}

// GameCenterLeaderboardCreateData is the data portion of a leaderboard create request.
type GameCenterLeaderboardCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    GameCenterLeaderboardCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardRelationships   `json:"relationships,omitempty"`
}

// GameCenterLeaderboardCreateRequest is a request to create a leaderboard.
type GameCenterLeaderboardCreateRequest struct {
	Data GameCenterLeaderboardCreateData `json:"data"`
}

// GameCenterLeaderboardV2CreateData is the data portion of a v2 leaderboard create request.
type GameCenterLeaderboardV2CreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    GameCenterLeaderboardCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardV2Relationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardV2CreateRequest is a request to create a v2 leaderboard.
type GameCenterLeaderboardV2CreateRequest struct {
	Data     GameCenterLeaderboardV2CreateData          `json:"data"`
	Included []GameCenterLeaderboardVersionInlineCreate `json:"included"`
}

// GameCenterLeaderboardVersionInlineCreate is an inline leaderboard version.
type GameCenterLeaderboardVersionInlineCreate struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// GameCenterLeaderboardUpdateData is the data portion of a leaderboard update request.
type GameCenterLeaderboardUpdateData struct {
	Type       ResourceType                           `json:"type"`
	ID         string                                 `json:"id"`
	Attributes *GameCenterLeaderboardUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardUpdateRequest is a request to update a leaderboard.
type GameCenterLeaderboardUpdateRequest struct {
	Data GameCenterLeaderboardUpdateData `json:"data"`
}

// GameCenterLeaderboardsResponse is the response from leaderboard list endpoints.
type GameCenterLeaderboardsResponse = Response[GameCenterLeaderboardAttributes]

// GameCenterLeaderboardResponse is the response from leaderboard detail endpoints.
type GameCenterLeaderboardResponse = SingleResponse[GameCenterLeaderboardAttributes]

// GameCenterLeaderboardDeleteResult represents CLI output for leaderboard deletions.
type GameCenterLeaderboardDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardsOption is a functional option for GetGameCenterLeaderboards.
type GCLeaderboardsOption func(*gcLeaderboardsQuery)

type gcLeaderboardsQuery struct {
	listQuery
}

// WithGCLeaderboardsLimit sets the max number of leaderboards to return.
func WithGCLeaderboardsLimit(limit int) GCLeaderboardsOption {
	return func(q *gcLeaderboardsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardsNextURL uses a next page URL directly.
func WithGCLeaderboardsNextURL(next string) GCLeaderboardsOption {
	return func(q *gcLeaderboardsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardsQuery(query *gcLeaderboardsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardVersionAttributes represents a v2 leaderboard version resource.
type GameCenterLeaderboardVersionAttributes struct {
	Version int                    `json:"version,omitempty"`
	State   GameCenterVersionState `json:"state,omitempty"`
}

// GameCenterLeaderboardVersionRelationships describes relationships for leaderboard versions.
type GameCenterLeaderboardVersionRelationships struct {
	Leaderboard *Relationship `json:"leaderboard,omitempty"`
}

// GameCenterLeaderboardVersionCreateData is the data portion of a version create request.
type GameCenterLeaderboardVersionCreateData struct {
	Type          ResourceType                               `json:"type"`
	Relationships *GameCenterLeaderboardVersionRelationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardVersionCreateRequest is a request to create a leaderboard version.
type GameCenterLeaderboardVersionCreateRequest struct {
	Data GameCenterLeaderboardVersionCreateData `json:"data"`
}

// GameCenterLeaderboardVersionsResponse is the response from leaderboard version list endpoints.
type GameCenterLeaderboardVersionsResponse = Response[GameCenterLeaderboardVersionAttributes]

// GameCenterLeaderboardVersionResponse is the response from leaderboard version detail endpoints.
type GameCenterLeaderboardVersionResponse = SingleResponse[GameCenterLeaderboardVersionAttributes]

// GCLeaderboardVersionsOption is a functional option for GetGameCenterLeaderboardVersions.
type GCLeaderboardVersionsOption func(*gcLeaderboardVersionsQuery)

type gcLeaderboardVersionsQuery struct {
	listQuery
}

// WithGCLeaderboardVersionsLimit sets the max number of versions to return.
func WithGCLeaderboardVersionsLimit(limit int) GCLeaderboardVersionsOption {
	return func(q *gcLeaderboardVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardVersionsNextURL uses a next page URL directly.
func WithGCLeaderboardVersionsNextURL(next string) GCLeaderboardVersionsOption {
	return func(q *gcLeaderboardVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardVersionsQuery(query *gcLeaderboardVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardLocalizationAttributes represents a Game Center leaderboard localization resource.
type GameCenterLeaderboardLocalizationAttributes struct {
	Locale                  string  `json:"locale"`
	Name                    string  `json:"name"`
	FormatterOverride       *string `json:"formatterOverride,omitempty"`
	FormatterSuffix         *string `json:"formatterSuffix,omitempty"`
	FormatterSuffixSingular *string `json:"formatterSuffixSingular,omitempty"`
	Description             *string `json:"description,omitempty"`
}

// GameCenterLeaderboardLocalizationCreateAttributes describes attributes for creating a localization.
type GameCenterLeaderboardLocalizationCreateAttributes struct {
	Locale                  string  `json:"locale"`
	Name                    string  `json:"name"`
	FormatterOverride       *string `json:"formatterOverride,omitempty"`
	FormatterSuffix         *string `json:"formatterSuffix,omitempty"`
	FormatterSuffixSingular *string `json:"formatterSuffixSingular,omitempty"`
	Description             *string `json:"description,omitempty"`
}

// GameCenterLeaderboardLocalizationUpdateAttributes describes attributes for updating a localization.
type GameCenterLeaderboardLocalizationUpdateAttributes struct {
	Name                    *string `json:"name,omitempty"`
	FormatterOverride       *string `json:"formatterOverride,omitempty"`
	FormatterSuffix         *string `json:"formatterSuffix,omitempty"`
	FormatterSuffixSingular *string `json:"formatterSuffixSingular,omitempty"`
	Description             *string `json:"description,omitempty"`
}

// GameCenterLeaderboardLocalizationRelationships describes relationships for leaderboard localizations.
type GameCenterLeaderboardLocalizationRelationships struct {
	GameCenterLeaderboard *Relationship `json:"gameCenterLeaderboard"`
}

// GameCenterLeaderboardLocalizationCreateData is the data portion of a localization create request.
type GameCenterLeaderboardLocalizationCreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterLeaderboardLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardLocalizationRelationships   `json:"relationships,omitempty"`
}

// GameCenterLeaderboardLocalizationCreateRequest is a request to create a localization.
type GameCenterLeaderboardLocalizationCreateRequest struct {
	Data GameCenterLeaderboardLocalizationCreateData `json:"data"`
}

// GameCenterLeaderboardLocalizationV2Relationships describes relationships for v2 leaderboard localizations.
type GameCenterLeaderboardLocalizationV2Relationships struct {
	Version *Relationship `json:"version"`
}

// GameCenterLeaderboardLocalizationV2CreateData is the data portion of a v2 localization create request.
type GameCenterLeaderboardLocalizationV2CreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterLeaderboardLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardLocalizationV2Relationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardLocalizationV2CreateRequest is a request to create a v2 localization.
type GameCenterLeaderboardLocalizationV2CreateRequest struct {
	Data GameCenterLeaderboardLocalizationV2CreateData `json:"data"`
}

// GameCenterLeaderboardLocalizationUpdateData is the data portion of a localization update request.
type GameCenterLeaderboardLocalizationUpdateData struct {
	Type       ResourceType                                       `json:"type"`
	ID         string                                             `json:"id"`
	Attributes *GameCenterLeaderboardLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardLocalizationUpdateRequest is a request to update a localization.
type GameCenterLeaderboardLocalizationUpdateRequest struct {
	Data GameCenterLeaderboardLocalizationUpdateData `json:"data"`
}

// GameCenterLeaderboardLocalizationsResponse is the response from leaderboard localization list endpoints.
type GameCenterLeaderboardLocalizationsResponse = Response[GameCenterLeaderboardLocalizationAttributes]

// GameCenterLeaderboardLocalizationResponse is the response from leaderboard localization detail endpoints.
type GameCenterLeaderboardLocalizationResponse = SingleResponse[GameCenterLeaderboardLocalizationAttributes]

// GameCenterLeaderboardLocalizationDeleteResult represents CLI output for localization deletions.
type GameCenterLeaderboardLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardLocalizationsOption is a functional option for GetGameCenterLeaderboardLocalizations.
type GCLeaderboardLocalizationsOption func(*gcLeaderboardLocalizationsQuery)

type gcLeaderboardLocalizationsQuery struct {
	listQuery
}

// WithGCLeaderboardLocalizationsLimit sets the max number of localizations to return.
func WithGCLeaderboardLocalizationsLimit(limit int) GCLeaderboardLocalizationsOption {
	return func(q *gcLeaderboardLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardLocalizationsNextURL uses a next page URL directly.
func WithGCLeaderboardLocalizationsNextURL(next string) GCLeaderboardLocalizationsOption {
	return func(q *gcLeaderboardLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardLocalizationsQuery(query *gcLeaderboardLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardReleaseAttributes represents a Game Center leaderboard release resource.
type GameCenterLeaderboardReleaseAttributes struct {
	Live bool `json:"live"`
}

// GameCenterLeaderboardReleaseRelationships describes relationships for leaderboard releases.
type GameCenterLeaderboardReleaseRelationships struct {
	GameCenterDetail      *Relationship `json:"gameCenterDetail"`
	GameCenterLeaderboard *Relationship `json:"gameCenterLeaderboard"`
}

// GameCenterLeaderboardReleaseCreateData is the data portion of a leaderboard release create request.
type GameCenterLeaderboardReleaseCreateData struct {
	Type          ResourceType                               `json:"type"`
	Relationships *GameCenterLeaderboardReleaseRelationships `json:"relationships"`
}

// GameCenterLeaderboardReleaseCreateRequest is a request to create a leaderboard release.
type GameCenterLeaderboardReleaseCreateRequest struct {
	Data GameCenterLeaderboardReleaseCreateData `json:"data"`
}

// GameCenterLeaderboardReleasesResponse is the response from leaderboard release list endpoints.
type GameCenterLeaderboardReleasesResponse = Response[GameCenterLeaderboardReleaseAttributes]

// GameCenterLeaderboardReleaseResponse is the response from leaderboard release detail endpoints.
type GameCenterLeaderboardReleaseResponse = SingleResponse[GameCenterLeaderboardReleaseAttributes]

// GameCenterLeaderboardReleaseDeleteResult represents CLI output for leaderboard release deletions.
type GameCenterLeaderboardReleaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardReleasesOption is a functional option for GetGameCenterLeaderboardReleases.
type GCLeaderboardReleasesOption func(*gcLeaderboardReleasesQuery)

type gcLeaderboardReleasesQuery struct {
	listQuery
}

// WithGCLeaderboardReleasesLimit sets the max number of leaderboard releases to return.
func WithGCLeaderboardReleasesLimit(limit int) GCLeaderboardReleasesOption {
	return func(q *gcLeaderboardReleasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardReleasesNextURL uses a next page URL directly.
func WithGCLeaderboardReleasesNextURL(next string) GCLeaderboardReleasesOption {
	return func(q *gcLeaderboardReleasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardReleasesQuery(query *gcLeaderboardReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardImageAttributes represents a Game Center leaderboard image resource.
type GameCenterLeaderboardImageAttributes struct {
	FileSize           int64               `json:"fileSize"`
	FileName           string              `json:"fileName"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// GameCenterLeaderboardImageCreateAttributes describes attributes for reserving an image upload.
type GameCenterLeaderboardImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GameCenterLeaderboardImageUpdateAttributes describes attributes for committing an image upload.
type GameCenterLeaderboardImageUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// GameCenterLeaderboardImageRelationships describes relationships for leaderboard images.
type GameCenterLeaderboardImageRelationships struct {
	GameCenterLeaderboardLocalization *Relationship `json:"gameCenterLeaderboardLocalization"`
}

// GameCenterLeaderboardImageCreateData is the data portion of an image create (reserve) request.
type GameCenterLeaderboardImageCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterLeaderboardImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardImageRelationships   `json:"relationships"`
}

// GameCenterLeaderboardImageCreateRequest is a request to reserve an image upload.
type GameCenterLeaderboardImageCreateRequest struct {
	Data GameCenterLeaderboardImageCreateData `json:"data"`
}

// GameCenterLeaderboardImageV2Relationships describes relationships for v2 leaderboard images.
type GameCenterLeaderboardImageV2Relationships struct {
	Localization *Relationship `json:"localization"`
}

// GameCenterLeaderboardImageV2CreateData is the data portion of a v2 image create (reserve) request.
type GameCenterLeaderboardImageV2CreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterLeaderboardImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardImageV2Relationships `json:"relationships"`
}

// GameCenterLeaderboardImageV2CreateRequest is a request to reserve a v2 image upload.
type GameCenterLeaderboardImageV2CreateRequest struct {
	Data GameCenterLeaderboardImageV2CreateData `json:"data"`
}

// GameCenterLeaderboardImageUpdateData is the data portion of an image update (commit) request.
type GameCenterLeaderboardImageUpdateData struct {
	Type       ResourceType                                `json:"type"`
	ID         string                                      `json:"id"`
	Attributes *GameCenterLeaderboardImageUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardImageUpdateRequest is a request to commit an image upload.
type GameCenterLeaderboardImageUpdateRequest struct {
	Data GameCenterLeaderboardImageUpdateData `json:"data"`
}

// GameCenterLeaderboardImagesResponse is the response from leaderboard image list endpoints.
type GameCenterLeaderboardImagesResponse = Response[GameCenterLeaderboardImageAttributes]

// GameCenterLeaderboardImageResponse is the response from leaderboard image detail endpoints.
type GameCenterLeaderboardImageResponse = SingleResponse[GameCenterLeaderboardImageAttributes]

// GameCenterLeaderboardImageDeleteResult represents CLI output for image deletions.
type GameCenterLeaderboardImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterLeaderboardImageUploadResult represents CLI output for image uploads.
type GameCenterLeaderboardImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}
