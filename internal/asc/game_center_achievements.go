package asc

import (
	"net/url"
	"strings"
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

// GameCenterAchievementV2Relationships describes relationships for v2 achievements.
type GameCenterAchievementV2Relationships struct {
	GameCenterDetail *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterGroup  *Relationship     `json:"gameCenterGroup,omitempty"`
	Versions         *RelationshipList `json:"versions"`
}

// GameCenterAchievementCreateData is the data portion of an achievement create request.
type GameCenterAchievementCreateData struct {
	Type          ResourceType                          `json:"type"`
	Attributes    GameCenterAchievementCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementRelationships   `json:"relationships,omitempty"`
}

// GameCenterAchievementCreateRequest is a request to create an achievement.
type GameCenterAchievementCreateRequest struct {
	Data GameCenterAchievementCreateData `json:"data"`
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

// GameCenterAchievementUpdateData is the data portion of an achievement update request.
type GameCenterAchievementUpdateData struct {
	Type       ResourceType                           `json:"type"`
	ID         string                                 `json:"id"`
	Attributes *GameCenterAchievementUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterAchievementUpdateRequest is a request to update an achievement.
type GameCenterAchievementUpdateRequest struct {
	Data GameCenterAchievementUpdateData `json:"data"`
}

// GameCenterAchievementsResponse is the response from achievement list endpoints.
type GameCenterAchievementsResponse = Response[GameCenterAchievementAttributes]

// GameCenterAchievementResponse is the response from achievement detail endpoints.
type GameCenterAchievementResponse = SingleResponse[GameCenterAchievementAttributes]

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

// WithGCAchievementsLimit sets the max number of achievements to return.
func WithGCAchievementsLimit(limit int) GCAchievementsOption {
	return func(q *gcAchievementsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCAchievementsNextURL uses a next page URL directly.
func WithGCAchievementsNextURL(next string) GCAchievementsOption {
	return func(q *gcAchievementsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// WithGCAchievementVersionsLimit sets the max number of versions to return.
func WithGCAchievementVersionsLimit(limit int) GCAchievementVersionsOption {
	return func(q *gcAchievementVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCAchievementVersionsNextURL uses a next page URL directly.
func WithGCAchievementVersionsNextURL(next string) GCAchievementVersionsOption {
	return func(q *gcAchievementVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterAchievementLocalizationCreateAttributes describes attributes for creating a localization.
type GameCenterAchievementLocalizationCreateAttributes struct {
	Locale                  string `json:"locale"`
	Name                    string `json:"name"`
	BeforeEarnedDescription string `json:"beforeEarnedDescription"`
	AfterEarnedDescription  string `json:"afterEarnedDescription"`
}

// GameCenterAchievementLocalizationUpdateAttributes describes attributes for updating a localization.
type GameCenterAchievementLocalizationUpdateAttributes struct {
	Name                    *string `json:"name,omitempty"`
	BeforeEarnedDescription *string `json:"beforeEarnedDescription,omitempty"`
	AfterEarnedDescription  *string `json:"afterEarnedDescription,omitempty"`
}

// GameCenterAchievementLocalizationRelationships describes relationships for achievement localizations.
type GameCenterAchievementLocalizationRelationships struct {
	GameCenterAchievement *Relationship `json:"gameCenterAchievement"`
}

// GameCenterAchievementLocalizationCreateData is the data portion of a localization create request.
type GameCenterAchievementLocalizationCreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterAchievementLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementLocalizationRelationships   `json:"relationships,omitempty"`
}

// GameCenterAchievementLocalizationCreateRequest is a request to create a localization.
type GameCenterAchievementLocalizationCreateRequest struct {
	Data GameCenterAchievementLocalizationCreateData `json:"data"`
}

// GameCenterAchievementLocalizationV2Relationships describes relationships for v2 achievement localizations.
type GameCenterAchievementLocalizationV2Relationships struct {
	Version *Relationship `json:"version"`
}

// GameCenterAchievementLocalizationV2CreateData is the data portion of a v2 localization create request.
type GameCenterAchievementLocalizationV2CreateData struct {
	Type          ResourceType                                      `json:"type"`
	Attributes    GameCenterAchievementLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementLocalizationV2Relationships `json:"relationships,omitempty"`
}

// GameCenterAchievementLocalizationV2CreateRequest is a request to create a v2 localization.
type GameCenterAchievementLocalizationV2CreateRequest struct {
	Data GameCenterAchievementLocalizationV2CreateData `json:"data"`
}

// GameCenterAchievementLocalizationUpdateData is the data portion of a localization update request.
type GameCenterAchievementLocalizationUpdateData struct {
	Type       ResourceType                                       `json:"type"`
	ID         string                                             `json:"id"`
	Attributes *GameCenterAchievementLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterAchievementLocalizationUpdateRequest is a request to update a localization.
type GameCenterAchievementLocalizationUpdateRequest struct {
	Data GameCenterAchievementLocalizationUpdateData `json:"data"`
}

// GameCenterAchievementLocalizationsResponse is the response from achievement localization list endpoints.
type GameCenterAchievementLocalizationsResponse = Response[GameCenterAchievementLocalizationAttributes]

// GameCenterAchievementLocalizationResponse is the response from achievement localization detail endpoints.
type GameCenterAchievementLocalizationResponse = SingleResponse[GameCenterAchievementLocalizationAttributes]

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

// WithGCAchievementLocalizationsLimit sets the max number of localizations to return.
func WithGCAchievementLocalizationsLimit(limit int) GCAchievementLocalizationsOption {
	return func(q *gcAchievementLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCAchievementLocalizationsNextURL uses a next page URL directly.
func WithGCAchievementLocalizationsNextURL(next string) GCAchievementLocalizationsOption {
	return func(q *gcAchievementLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterAchievementReleaseRelationships describes relationships for achievement releases.
type GameCenterAchievementReleaseRelationships struct {
	GameCenterDetail      *Relationship `json:"gameCenterDetail"`
	GameCenterAchievement *Relationship `json:"gameCenterAchievement"`
}

// GameCenterAchievementReleaseCreateData is the data portion of an achievement release create request.
type GameCenterAchievementReleaseCreateData struct {
	Type          ResourceType                               `json:"type"`
	Relationships *GameCenterAchievementReleaseRelationships `json:"relationships"`
}

// GameCenterAchievementReleaseCreateRequest is a request to create an achievement release.
type GameCenterAchievementReleaseCreateRequest struct {
	Data GameCenterAchievementReleaseCreateData `json:"data"`
}

// GameCenterAchievementReleasesResponse is the response from achievement release list endpoints.
type GameCenterAchievementReleasesResponse = Response[GameCenterAchievementReleaseAttributes]

// GameCenterAchievementReleaseResponse is the response from achievement release detail endpoints.
type GameCenterAchievementReleaseResponse = SingleResponse[GameCenterAchievementReleaseAttributes]

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

// WithGCAchievementReleasesLimit sets the max number of achievement releases to return.
func WithGCAchievementReleasesLimit(limit int) GCAchievementReleasesOption {
	return func(q *gcAchievementReleasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCAchievementReleasesNextURL uses a next page URL directly.
func WithGCAchievementReleasesNextURL(next string) GCAchievementReleasesOption {
	return func(q *gcAchievementReleasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterAchievementImageUpdateAttributes describes attributes for committing an image upload.
type GameCenterAchievementImageUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// GameCenterAchievementImageRelationships describes relationships for achievement images.
type GameCenterAchievementImageRelationships struct {
	GameCenterAchievementLocalization *Relationship `json:"gameCenterAchievementLocalization"`
}

// GameCenterAchievementImageCreateData is the data portion of an image create (reserve) request.
type GameCenterAchievementImageCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterAchievementImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementImageRelationships   `json:"relationships"`
}

// GameCenterAchievementImageCreateRequest is a request to reserve an image upload.
type GameCenterAchievementImageCreateRequest struct {
	Data GameCenterAchievementImageCreateData `json:"data"`
}

// GameCenterAchievementImageV2Relationships describes relationships for v2 achievement images.
type GameCenterAchievementImageV2Relationships struct {
	Localization *Relationship `json:"localization"`
}

// GameCenterAchievementImageV2CreateData is the data portion of a v2 image create (reserve) request.
type GameCenterAchievementImageV2CreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    GameCenterAchievementImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterAchievementImageV2Relationships `json:"relationships"`
}

// GameCenterAchievementImageV2CreateRequest is a request to reserve a v2 image upload.
type GameCenterAchievementImageV2CreateRequest struct {
	Data GameCenterAchievementImageV2CreateData `json:"data"`
}

// GameCenterAchievementImageUpdateData is the data portion of an image update (commit) request.
type GameCenterAchievementImageUpdateData struct {
	Type       ResourceType                                `json:"type"`
	ID         string                                      `json:"id"`
	Attributes *GameCenterAchievementImageUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterAchievementImageUpdateRequest is a request to commit an image upload.
type GameCenterAchievementImageUpdateRequest struct {
	Data GameCenterAchievementImageUpdateData `json:"data"`
}

// GameCenterAchievementImagesResponse is the response from achievement image list endpoints.
type GameCenterAchievementImagesResponse = Response[GameCenterAchievementImageAttributes]

// GameCenterAchievementImageResponse is the response from achievement image detail endpoints.
type GameCenterAchievementImageResponse = SingleResponse[GameCenterAchievementImageAttributes]

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
