package asc

import (
	"net/url"
)

// GameCenterAchievementAttributes represents a Game Center achievement resource.
type GameCenterAchievementAttributes struct {
	ReferenceName      string            `json:"referenceName"`
	VendorIdentifier   string            `json:"vendorIdentifier"`
	Points             int               `json:"points"`
	ShowBeforeEarned   bool              `json:"showBeforeEarned"`
	Repeatable         bool              `json:"repeatable"`
	Archived           bool              `json:"archived,omitempty"`
	ActivityProperties map[string]string `json:"activityProperties,omitempty"`
}

// GameCenterAchievementCreateAttributes describes attributes for creating an achievement.
type GameCenterAchievementCreateAttributes struct {
	ReferenceName      string            `json:"referenceName"`
	VendorIdentifier   string            `json:"vendorIdentifier"`
	Points             int               `json:"points"`
	ShowBeforeEarned   bool              `json:"showBeforeEarned"`
	Repeatable         bool              `json:"repeatable"`
	ActivityProperties map[string]string `json:"activityProperties,omitzero"`
}

// GameCenterAchievementUpdateAttributes describes attributes for updating an achievement.
type GameCenterAchievementUpdateAttributes struct {
	ReferenceName      *string           `json:"referenceName,omitempty"`
	Points             *int              `json:"points,omitempty"`
	ShowBeforeEarned   *bool             `json:"showBeforeEarned,omitempty"`
	Repeatable         *bool             `json:"repeatable,omitempty"`
	Archived           *bool             `json:"archived,omitempty"`
	ActivityProperties map[string]string `json:"activityProperties,omitzero"`
}

// GameCenterAchievementRelationships describes relationships for achievements.
type GameCenterAchievementRelationships struct {
	GameCenterDetail *Relationship `json:"gameCenterDetail"`
}

// GameCenterAchievementV2CreateData is the data portion of a v2 achievement create request.
type GameCenterAchievementV2CreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    GameCenterAchievementCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementV2Relationships `json:"relationships,omitempty"`
}

// GameCenterAchievementV2CreateRequest is a request to create a v2 achievement.
type GameCenterAchievementV2CreateRequest struct {
	Data     GameCenterAchievementV2CreateData          `json:"data"`
	Included []GameCenterAchievementVersionInlineCreate `json:"included"`
}

// GameCenterAchievementVersionInlineCreate is an inline achievement version.
type GameCenterAchievementVersionInlineCreate struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// GameCenterAchievementsResponse is the response from achievement list endpoints.
type GameCenterAchievementsResponse = Response[GameCenterAchievementAttributes]

// GameCenterAchievementDeleteResult represents CLI output for achievement deletions.
type GameCenterAchievementDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCAchievementsOption is a functional option for GetGameCenterAchievements.
type GCAchievementsOption func(*gcAchievementsQuery)

type gcAchievementsQuery struct {
	listQuery
}

func buildGCAchievementsQuery(query *gcAchievementsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAchievementVersionAttributes represents a v2 achievement version resource.
type GameCenterAchievementVersionAttributes struct {
	Version int                    `json:"version,omitempty"`
	State   GameCenterVersionState `json:"state,omitempty"`
}

// GameCenterAchievementVersionRelationships describes relationships for achievement versions.
type GameCenterAchievementVersionRelationships struct {
	Achievement *Relationship `json:"achievement,omitempty"`
}

// GameCenterAchievementVersionCreateData is the data portion of a version create request.
type GameCenterAchievementVersionCreateData struct {
	Type          ResourceType                               `json:"type"`
	Relationships *GameCenterAchievementVersionRelationships `json:"relationships,omitempty"`
}

// GameCenterAchievementVersionCreateRequest is a request to create an achievement version.
type GameCenterAchievementVersionCreateRequest struct {
	Data GameCenterAchievementVersionCreateData `json:"data"`
}

// GameCenterAchievementVersionsResponse is the response from achievement version list endpoints.
type GameCenterAchievementVersionsResponse = Response[GameCenterAchievementVersionAttributes]

// GameCenterAchievementVersionResponse is the response from achievement version detail endpoints.
type GameCenterAchievementVersionResponse = SingleResponse[GameCenterAchievementVersionAttributes]

// GCAchievementVersionsOption is a functional option for GetGameCenterAchievementVersions.
type GCAchievementVersionsOption func(*gcAchievementVersionsQuery)

type gcAchievementVersionsQuery struct {
	listQuery
}

func buildGCAchievementVersionsQuery(query *gcAchievementVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAchievementLocalizationAttributes represents a Game Center achievement localization.
type GameCenterAchievementLocalizationAttributes struct {
	Locale                  string `json:"locale"`
	Name                    string `json:"name,omitempty"`
	BeforeEarnedDescription string `json:"beforeEarnedDescription,omitempty"`
	AfterEarnedDescription  string `json:"afterEarnedDescription,omitempty"`
}

// GameCenterAchievementLocalizationV2CreateData is the data portion of a v2 localization create request.
type GameCenterAchievementLocalizationV2CreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterAchievementLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementLocalizationV2Relationships `json:"relationships,omitempty"`
}

// GameCenterAchievementLocalizationsResponse is the response from achievement localization list endpoints.
type GameCenterAchievementLocalizationsResponse = Response[GameCenterAchievementLocalizationAttributes]

// GameCenterAchievementLocalizationDeleteResult represents CLI output for localization deletions.
type GameCenterAchievementLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCAchievementLocalizationsOption is a functional option for GetGameCenterAchievementLocalizations.
type GCAchievementLocalizationsOption func(*gcAchievementLocalizationsQuery)

type gcAchievementLocalizationsQuery struct {
	listQuery
}

func buildGCAchievementLocalizationsQuery(query *gcAchievementLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAchievementReleaseAttributes represents a Game Center achievement release resource.
type GameCenterAchievementReleaseAttributes struct {
	Live bool `json:"live"`
}

// GameCenterAchievementReleasesResponse is the response from achievement release list endpoints.
type GameCenterAchievementReleasesResponse = Response[GameCenterAchievementReleaseAttributes]

// GameCenterAchievementReleaseDeleteResult represents CLI output for achievement release deletions.
type GameCenterAchievementReleaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCAchievementReleasesOption is a functional option for GetGameCenterAchievementReleases.
type GCAchievementReleasesOption func(*gcAchievementReleasesQuery)

type gcAchievementReleasesQuery struct {
	listQuery
}

func buildGCAchievementReleasesQuery(query *gcAchievementReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterAchievementImageAttributes represents a Game Center achievement image resource.
type GameCenterAchievementImageAttributes struct {
	FileSize           int64               `json:"fileSize"`
	FileName           string              `json:"fileName"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// GameCenterAchievementImageCreateAttributes describes attributes for reserving an image upload.
type GameCenterAchievementImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GameCenterAchievementImageV2CreateData is the data portion of a v2 image create (reserve) request.
type GameCenterAchievementImageV2CreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterAchievementImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementImageV2Relationships `json:"relationships"`
}

// GameCenterAchievementImagesResponse is the response from achievement image list endpoints.
type GameCenterAchievementImagesResponse = Response[GameCenterAchievementImageAttributes]

// GameCenterAchievementImageDeleteResult represents CLI output for image deletions.
type GameCenterAchievementImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterAchievementImageUploadResult represents CLI output for image uploads.
type GameCenterAchievementImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}
