package asc

import (
	"encoding/json"
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

// GameCenterChallengesResponse is the response from challenge list endpoints.
type GameCenterChallengesResponse = Response[GameCenterChallengeAttributes]

// GameCenterChallengeVersionRelationships describes relationships for challenge versions.
type GameCenterChallengeVersionRelationships struct {
	Challenge *Relationship `json:"challenge,omitempty"`
}

// GameCenterChallengeVersionsResponse is the response from version list endpoints.
type GameCenterChallengeVersionsResponse = Response[GameCenterChallengeVersionAttributes]

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

// GameCenterChallengeLocalizationsResponse is the response from localization list endpoints.
type GameCenterChallengeLocalizationsResponse = Response[GameCenterChallengeLocalizationAttributes]

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

// GameCenterChallengeImagesResponse is the response from challenge image list endpoints.
type GameCenterChallengeImagesResponse = Response[GameCenterChallengeImageAttributes]

// GameCenterChallengeVersionDefaultImageResponse preserves a missing default image as null.
type GameCenterChallengeVersionDefaultImageResponse struct {
	Data     *Resource[GameCenterChallengeImageAttributes] `json:"data"`
	Links    Links                                         `json:"links"`
	Included json.RawMessage                               `json:"included,omitempty"`
	Meta     json.RawMessage                               `json:"meta,omitempty"`
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

// GameCenterChallengeVersionReleasesResponse is the response from release list endpoints.
type GameCenterChallengeVersionReleasesResponse = Response[GameCenterChallengeVersionReleaseAttributes]
