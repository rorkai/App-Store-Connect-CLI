package asc

import (
	"net/url"
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

// GameCenterLeaderboardsResponse is the response from leaderboard list endpoints.
type GameCenterLeaderboardsResponse = Response[GameCenterLeaderboardAttributes]

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

// GameCenterLeaderboardLocalizationV2CreateData is the data portion of a v2 localization create request.
type GameCenterLeaderboardLocalizationV2CreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterLeaderboardLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardLocalizationV2Relationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardLocalizationsResponse is the response from leaderboard localization list endpoints.
type GameCenterLeaderboardLocalizationsResponse = Response[GameCenterLeaderboardLocalizationAttributes]

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

func buildGCLeaderboardLocalizationsQuery(query *gcLeaderboardLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardReleaseAttributes represents a Game Center leaderboard release resource.
type GameCenterLeaderboardReleaseAttributes struct {
	Live bool `json:"live"`
}

// GameCenterLeaderboardReleasesResponse is the response from leaderboard release list endpoints.
type GameCenterLeaderboardReleasesResponse = Response[GameCenterLeaderboardReleaseAttributes]

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

// GameCenterLeaderboardImageV2CreateData is the data portion of a v2 image create (reserve) request.
type GameCenterLeaderboardImageV2CreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterLeaderboardImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardImageV2Relationships `json:"relationships"`
}

// GameCenterLeaderboardImagesResponse is the response from leaderboard image list endpoints.
type GameCenterLeaderboardImagesResponse = Response[GameCenterLeaderboardImageAttributes]

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
