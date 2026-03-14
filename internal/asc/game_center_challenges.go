package asc

import (
	"net/url"
	"strings"
)

// GameCenterChallengeAttributes represents a Game Center challenge resource.
type GameCenterChallengeAttributes struct {
	ReferenceName    string `json:"referenceName"`
	VendorIdentifier string `json:"vendorIdentifier"`
	Archived         bool   `json:"archived,omitempty"`
	ChallengeType    string `json:"challengeType,omitempty"`
	Repeatable       bool   `json:"repeatable,omitempty"`
}

// GameCenterChallengeCreateAttributes describes attributes for creating a challenge.
type GameCenterChallengeCreateAttributes struct {
	ReferenceName    string `json:"referenceName"`
	VendorIdentifier string `json:"vendorIdentifier"`
	ChallengeType    string `json:"challengeType"`
	Repeatable       *bool  `json:"repeatable,omitempty"`
}

// GameCenterChallengeUpdateAttributes describes attributes for updating a challenge.
type GameCenterChallengeUpdateAttributes struct {
	ReferenceName *string `json:"referenceName,omitempty"`
	Archived      *bool   `json:"archived,omitempty"`
	Repeatable    *bool   `json:"repeatable,omitempty"`
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

// GameCenterChallengeUpdateRequest is a request to update a challenge.
type GameCenterChallengeUpdateRequest struct {
	Data GameCenterChallengeUpdateData `json:"data"`
}

// GameCenterChallengesResponse is the response from challenge list endpoints.
type GameCenterChallengesResponse = Response[GameCenterChallengeAttributes]

// GameCenterChallengeResponse is the response from challenge detail endpoints.
type GameCenterChallengeResponse = SingleResponse[GameCenterChallengeAttributes]

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

// WithGCChallengesLimit sets the max number of challenges to return.
func WithGCChallengesLimit(limit int) GCChallengesOption {
	return func(q *gcChallengesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCChallengesNextURL uses a next page URL directly.
func WithGCChallengesNextURL(next string) GCChallengesOption {
	return func(q *gcChallengesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterChallengeVersionCreateRequest is a request to create a challenge version.
type GameCenterChallengeVersionCreateRequest struct {
	Data GameCenterChallengeVersionCreateData `json:"data"`
}

// GameCenterChallengeVersionsResponse is the response from version list endpoints.
type GameCenterChallengeVersionsResponse = Response[GameCenterChallengeVersionAttributes]

// GameCenterChallengeVersionResponse is the response from version detail endpoints.
type GameCenterChallengeVersionResponse = SingleResponse[GameCenterChallengeVersionAttributes]

// GCChallengeVersionsOption is a functional option for GetGameCenterChallengeVersions.
type GCChallengeVersionsOption func(*gcChallengeVersionsQuery)

type gcChallengeVersionsQuery struct {
	listQuery
}

// WithGCChallengeVersionsLimit sets the max number of versions to return.
func WithGCChallengeVersionsLimit(limit int) GCChallengeVersionsOption {
	return func(q *gcChallengeVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCChallengeVersionsNextURL uses a next page URL directly.
func WithGCChallengeVersionsNextURL(next string) GCChallengeVersionsOption {
	return func(q *gcChallengeVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterChallengeLocalizationUpdateAttributes describes attributes for updating a localization.
type GameCenterChallengeLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
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

// GameCenterChallengeLocalizationCreateRequest is a request to create a localization.
type GameCenterChallengeLocalizationCreateRequest struct {
	Data GameCenterChallengeLocalizationCreateData `json:"data"`
}

// GameCenterChallengeLocalizationUpdateData is the data portion of a localization update request.
type GameCenterChallengeLocalizationUpdateData struct {
	Type       ResourceType                                     `json:"type"`
	ID         string                                           `json:"id"`
	Attributes *GameCenterChallengeLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterChallengeLocalizationUpdateRequest is a request to update a localization.
type GameCenterChallengeLocalizationUpdateRequest struct {
	Data GameCenterChallengeLocalizationUpdateData `json:"data"`
}

// GameCenterChallengeLocalizationsResponse is the response from localization list endpoints.
type GameCenterChallengeLocalizationsResponse = Response[GameCenterChallengeLocalizationAttributes]

// GameCenterChallengeLocalizationResponse is the response from localization detail endpoints.
type GameCenterChallengeLocalizationResponse = SingleResponse[GameCenterChallengeLocalizationAttributes]

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

// WithGCChallengeLocalizationsLimit sets the max number of localizations to return.
func WithGCChallengeLocalizationsLimit(limit int) GCChallengeLocalizationsOption {
	return func(q *gcChallengeLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCChallengeLocalizationsNextURL uses a next page URL directly.
func WithGCChallengeLocalizationsNextURL(next string) GCChallengeLocalizationsOption {
	return func(q *gcChallengeLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
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

// GameCenterChallengeImageUpdateAttributes describes attributes for committing an image upload.
type GameCenterChallengeImageUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// GameCenterChallengeImageRelationships describes relationships for challenge images.
type GameCenterChallengeImageRelationships struct {
	Localization *Relationship `json:"localization"`
}

// GameCenterChallengeImageCreateData is the data portion of an image create request.
type GameCenterChallengeImageCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    GameCenterChallengeImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterChallengeImageRelationships   `json:"relationships"`
}

// GameCenterChallengeImageCreateRequest is a request to reserve an image upload.
type GameCenterChallengeImageCreateRequest struct {
	Data GameCenterChallengeImageCreateData `json:"data"`
}

// GameCenterChallengeImageUpdateData is the data portion of an image update (commit) request.
type GameCenterChallengeImageUpdateData struct {
	Type       ResourceType                              `json:"type"`
	ID         string                                    `json:"id"`
	Attributes *GameCenterChallengeImageUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterChallengeImageUpdateRequest is a request to commit an image upload.
type GameCenterChallengeImageUpdateRequest struct {
	Data GameCenterChallengeImageUpdateData `json:"data"`
}

// GameCenterChallengeImagesResponse is the response from challenge image list endpoints.
type GameCenterChallengeImagesResponse = Response[GameCenterChallengeImageAttributes]

// GameCenterChallengeImageResponse is the response from challenge image detail endpoints.
type GameCenterChallengeImageResponse = SingleResponse[GameCenterChallengeImageAttributes]

// GameCenterChallengeImageDeleteResult represents CLI output for image deletions.
type GameCenterChallengeImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterChallengeImageUploadResult represents CLI output for image uploads.
type GameCenterChallengeImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
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

// GameCenterChallengeVersionReleaseCreateRequest is a request to create a release.
type GameCenterChallengeVersionReleaseCreateRequest struct {
	Data GameCenterChallengeVersionReleaseCreateData `json:"data"`
}

// GameCenterChallengeVersionReleasesResponse is the response from release list endpoints.
type GameCenterChallengeVersionReleasesResponse = Response[GameCenterChallengeVersionReleaseAttributes]

// GameCenterChallengeVersionReleaseResponse is the response from release detail endpoints.
type GameCenterChallengeVersionReleaseResponse = SingleResponse[GameCenterChallengeVersionReleaseAttributes]

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

// WithGCChallengeVersionReleasesLimit sets the max number of releases to return.
func WithGCChallengeVersionReleasesLimit(limit int) GCChallengeVersionReleasesOption {
	return func(q *gcChallengeVersionReleasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCChallengeVersionReleasesNextURL uses a next page URL directly.
func WithGCChallengeVersionReleasesNextURL(next string) GCChallengeVersionReleasesOption {
	return func(q *gcChallengeVersionReleasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCChallengeVersionReleasesQuery(query *gcChallengeVersionReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
