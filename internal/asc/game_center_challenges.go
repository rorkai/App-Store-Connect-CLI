package asc

import (
	"encoding/json"
	"net/url"
)

// GameCenterChallengeAttributes represents a Game Center challenge resource.
type GameCenterChallengeAttributes struct {
	ReferenceName    string `json:"referenceName"`
	VendorIdentifier string `json:"vendorIdentifier"`
	Archived         bool   `json:"archived,omitempty"`
	ChallengeType    string `json:"challengeType,omitempty"`
	Repeatable       bool   `json:"repeatable,omitempty"`
}

// GameCenterChallengeRelationships describes relationships for challenges.
type GameCenterChallengeRelationships struct {
	GameCenterDetail *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterGroup  *Relationship     `json:"gameCenterGroup,omitempty"`
	Leaderboard      *Relationship     `json:"leaderboard,omitempty"`
	LeaderboardV2    *Relationship     `json:"leaderboardV2,omitempty"`
	Versions         *RelationshipList `json:"versions,omitempty"`
}

// GameCenterChallengeUpdateRelationships describes relationships for challenge updates.
type GameCenterChallengeUpdateRelationships struct {
	Leaderboard   *Relationship `json:"leaderboard,omitempty"`
	LeaderboardV2 *Relationship `json:"leaderboardV2,omitempty"`
}

// GameCenterChallengeCreateData is the data portion of a challenge create request.
type GameCenterChallengeCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    GameCenterChallengeCreateAttributes `json:"attributes"`
	Relationships *GameCenterChallengeRelationships   `json:"relationships,omitempty"`
}

// GameCenterChallengeCreateRequest is a request to create a challenge.
type GameCenterChallengeCreateRequest struct {
	Data     GameCenterChallengeCreateData            `json:"data"`
	Included []GameCenterChallengeVersionInlineCreate `json:"included,omitempty"`
}

// GameCenterChallengeVersionInlineCreate is an inline resource for creating an
// initial challenge version alongside the parent challenge.
type GameCenterChallengeVersionInlineCreate struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id,omitempty"`
}

// GameCenterChallengeUpdateData is the data portion of a challenge update request.
type GameCenterChallengeUpdateData struct {
	Type          ResourceType                            `json:"type"`
	ID            string                                  `json:"id"`
	Attributes    *GameCenterChallengeUpdateAttributes    `json:"attributes,omitempty"`
	Relationships *GameCenterChallengeUpdateRelationships `json:"relationships,omitempty"`
}

// GameCenterChallengesResponse is the response from challenge list endpoints.
type GameCenterChallengesResponse = Response[GameCenterChallengeAttributes]

// GameCenterChallengeDeleteResult represents CLI output for challenge deletions.
type GameCenterChallengeDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCChallengesOption is a functional option for GetGameCenterChallenges.
type GCChallengesOption func(*gcChallengesQuery)

type gcChallengesQuery struct {
	listQuery
}

func buildGCChallengesQuery(query *gcChallengesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterChallengeVersionAttributes represents a Game Center challenge version resource.
type GameCenterChallengeVersionAttributes struct {
	Version int                    `json:"version,omitempty"`
	State   GameCenterVersionState `json:"state,omitempty"`
}

// GameCenterChallengeVersionRelationships describes relationships for challenge versions.
type GameCenterChallengeVersionRelationships struct {
	Challenge *Relationship `json:"challenge,omitempty"`
}

// GameCenterChallengeVersionCreateData is the data portion of a version create request.
type GameCenterChallengeVersionCreateData struct {
	Type          ResourceType                             `json:"type"`
	Relationships *GameCenterChallengeVersionRelationships `json:"relationships,omitempty"`
}

// GameCenterChallengeVersionsResponse is the response from version list endpoints.
type GameCenterChallengeVersionsResponse = Response[GameCenterChallengeVersionAttributes]

// GCChallengeVersionsOption is a functional option for GetGameCenterChallengeVersions.
type GCChallengeVersionsOption func(*gcChallengeVersionsQuery)

type gcChallengeVersionsQuery struct {
	listQuery
}

func buildGCChallengeVersionsQuery(query *gcChallengeVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterChallengeLocalizationAttributes represents a challenge localization.
type GameCenterChallengeLocalizationAttributes struct {
	Locale      string `json:"locale"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// GameCenterChallengeLocalizationCreateAttributes describes attributes for creating a localization.
type GameCenterChallengeLocalizationCreateAttributes struct {
	Locale      string `json:"locale"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GameCenterChallengeLocalizationRelationships describes relationships for challenge localizations.
type GameCenterChallengeLocalizationRelationships struct {
	Version *Relationship `json:"version"`
}

// GameCenterChallengeLocalizationCreateData is the data portion of a localization create request.
type GameCenterChallengeLocalizationCreateData struct {
	Type          ResourceType                                    `json:"type"`
	Attributes    GameCenterChallengeLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterChallengeLocalizationRelationships   `json:"relationships,omitempty"`
}

// GameCenterChallengeLocalizationsResponse is the response from localization list endpoints.
type GameCenterChallengeLocalizationsResponse = Response[GameCenterChallengeLocalizationAttributes]

// GameCenterChallengeLocalizationDeleteResult represents CLI output for localization deletions.
type GameCenterChallengeLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCChallengeLocalizationsOption is a functional option for GetGameCenterChallengeLocalizations.
type GCChallengeLocalizationsOption func(*gcChallengeLocalizationsQuery)

type gcChallengeLocalizationsQuery struct {
	listQuery
}

func buildGCChallengeLocalizationsQuery(query *gcChallengeLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterChallengeImageAttributes represents a challenge image resource.
type GameCenterChallengeImageAttributes struct {
	FileSize           int64               `json:"fileSize"`
	FileName           string              `json:"fileName"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// GameCenterChallengeImageCreateAttributes describes attributes for reserving an image upload.
type GameCenterChallengeImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GameCenterChallengeImageRelationships describes relationships for challenge images.
type GameCenterChallengeImageRelationships struct {
	Localization *Relationship `json:"localization,omitempty"`
	Version      *Relationship `json:"version,omitempty"`
}

// GameCenterChallengeImageCreateData is the data portion of an image create request.
type GameCenterChallengeImageCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    GameCenterChallengeImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterChallengeImageRelationships   `json:"relationships"`
}

// GameCenterChallengeImagesResponse is the response from challenge image list endpoints.
type GameCenterChallengeImagesResponse = Response[GameCenterChallengeImageAttributes]

// GameCenterChallengeVersionDefaultImageResponse preserves a missing default image as null.
type GameCenterChallengeVersionDefaultImageResponse struct {
	Data     *Resource[GameCenterChallengeImageAttributes] `json:"data"`
	Links    Links                                         `json:"links"`
	Included json.RawMessage                               `json:"included,omitempty"`
	Meta     json.RawMessage                               `json:"meta,omitempty"`
}

// GameCenterChallengeImageDeleteResult represents CLI output for image deletions.
type GameCenterChallengeImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterChallengeImageUploadResult represents CLI output for image uploads.
type GameCenterChallengeImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId,omitempty"`
	VersionID          string `json:"versionId,omitempty"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}

// GameCenterChallengeVersionReleaseAttributes represents a challenge version release.
type GameCenterChallengeVersionReleaseAttributes struct{}

// GameCenterChallengeVersionReleaseRelationships describes relationships for challenge version releases.
type GameCenterChallengeVersionReleaseRelationships struct {
	Version *Relationship `json:"version,omitempty"`
}

// GameCenterChallengeVersionReleaseCreateData is the data portion of a release create request.
type GameCenterChallengeVersionReleaseCreateData struct {
	Type          ResourceType                                    `json:"type"`
	Relationships *GameCenterChallengeVersionReleaseRelationships `json:"relationships,omitempty"`
}

// GameCenterChallengeVersionReleasesResponse is the response from release list endpoints.
type GameCenterChallengeVersionReleasesResponse = Response[GameCenterChallengeVersionReleaseAttributes]

// GameCenterChallengeVersionReleaseDeleteResult represents CLI output for release deletions.
type GameCenterChallengeVersionReleaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCChallengeVersionReleasesOption is a functional option for GetGameCenterChallengeVersionReleases.
type GCChallengeVersionReleasesOption func(*gcChallengeVersionReleasesQuery)

type gcChallengeVersionReleasesQuery struct {
	listQuery
}

func buildGCChallengeVersionReleasesQuery(query *gcChallengeVersionReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
