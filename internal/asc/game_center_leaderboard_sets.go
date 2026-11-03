package asc

import (
	"net/url"
	"strings"
)

// GameCenterLeaderboardSetAttributes represents a Game Center leaderboard set resource.
type GameCenterLeaderboardSetAttributes struct {
	ReferenceName    string `json:"referenceName"`
	VendorIdentifier string `json:"vendorIdentifier"`
}

// GameCenterLeaderboardSetCreateAttributes describes attributes for creating a leaderboard set.
type GameCenterLeaderboardSetCreateAttributes struct {
	ReferenceName    string `json:"referenceName"`
	VendorIdentifier string `json:"vendorIdentifier"`
}

// GameCenterLeaderboardSetUpdateAttributes describes attributes for updating a leaderboard set.
type GameCenterLeaderboardSetUpdateAttributes struct {
	ReferenceName *string `json:"referenceName,omitempty"`
}

// GameCenterLeaderboardSetRelationships describes relationships for leaderboard sets.
type GameCenterLeaderboardSetRelationships struct {
	GameCenterDetail       *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterLeaderboards *RelationshipList `json:"gameCenterLeaderboards,omitempty"`
}

// GameCenterLeaderboardSetV2Relationships describes relationships for v2 leaderboard sets.
type GameCenterLeaderboardSetV2Relationships struct {
	GameCenterDetail       *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterGroup        *Relationship     `json:"gameCenterGroup,omitempty"`
	GameCenterLeaderboards *RelationshipList `json:"gameCenterLeaderboards,omitempty"`
	Versions               *RelationshipList `json:"versions"`
}

// GameCenterLeaderboardSetCreateData is the data portion of a leaderboard set create request.
type GameCenterLeaderboardSetCreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    GameCenterLeaderboardSetCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetRelationships   `json:"relationships,omitempty"`
}

// GameCenterLeaderboardSetCreateRequest is a request to create a leaderboard set.
type GameCenterLeaderboardSetCreateRequest struct {
	Data GameCenterLeaderboardSetCreateData `json:"data"`
}

// GameCenterLeaderboardSetV2CreateData is the data portion of a v2 leaderboard set create request.
type GameCenterLeaderboardSetV2CreateData struct {
	Type          ResourceType                             `json:"type"`
	Attributes    GameCenterLeaderboardSetCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetV2Relationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardSetV2CreateRequest is a request to create a v2 leaderboard set.
type GameCenterLeaderboardSetV2CreateRequest struct {
	Data     GameCenterLeaderboardSetV2CreateData          `json:"data"`
	Included []GameCenterLeaderboardSetVersionInlineCreate `json:"included"`
}

// GameCenterLeaderboardSetVersionInlineCreate is an inline leaderboard set version.
type GameCenterLeaderboardSetVersionInlineCreate struct {
	Type ResourceType `json:"type"`
	ID   string       `json:"id"`
}

// GameCenterLeaderboardSetUpdateData is the data portion of a leaderboard set update request.
type GameCenterLeaderboardSetUpdateData struct {
	Type       ResourceType                              `json:"type"`
	ID         string                                    `json:"id"`
	Attributes *GameCenterLeaderboardSetUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardSetUpdateRequest is a request to update a leaderboard set.
type GameCenterLeaderboardSetUpdateRequest struct {
	Data GameCenterLeaderboardSetUpdateData `json:"data"`
}

// GameCenterLeaderboardSetsResponse is the response from leaderboard set list endpoints.
type GameCenterLeaderboardSetsResponse = Response[GameCenterLeaderboardSetAttributes]

// GameCenterLeaderboardSetResponse is the response from leaderboard set detail endpoints.
type GameCenterLeaderboardSetResponse = SingleResponse[GameCenterLeaderboardSetAttributes]

// GameCenterLeaderboardSetDeleteResult represents CLI output for leaderboard set deletions.
type GameCenterLeaderboardSetDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardSetsOption is a functional option for GetGameCenterLeaderboardSets.
type GCLeaderboardSetsOption func(*gcLeaderboardSetsQuery)

type gcLeaderboardSetsQuery struct {
	listQuery
}

// WithGCLeaderboardSetsLimit sets the max number of leaderboard sets to return.
func WithGCLeaderboardSetsLimit(limit int) GCLeaderboardSetsOption {
	return func(q *gcLeaderboardSetsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetsNextURL uses a next page URL directly.
func WithGCLeaderboardSetsNextURL(next string) GCLeaderboardSetsOption {
	return func(q *gcLeaderboardSetsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardSetsQuery(query *gcLeaderboardSetsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetVersionAttributes represents a v2 leaderboard set version resource.
type GameCenterLeaderboardSetVersionAttributes struct {
	Version int                    `json:"version,omitempty"`
	State   GameCenterVersionState `json:"state,omitempty"`
}

// GameCenterLeaderboardSetVersionRelationships describes relationships for leaderboard set versions.
type GameCenterLeaderboardSetVersionRelationships struct {
	LeaderboardSet *Relationship `json:"leaderboardSet,omitempty"`
}

// GameCenterLeaderboardSetVersionCreateData is the data portion of a version create request.
type GameCenterLeaderboardSetVersionCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Relationships *GameCenterLeaderboardSetVersionRelationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardSetVersionCreateRequest is a request to create a leaderboard set version.
type GameCenterLeaderboardSetVersionCreateRequest struct {
	Data GameCenterLeaderboardSetVersionCreateData `json:"data"`
}

// GameCenterLeaderboardSetVersionsResponse is the response from leaderboard set version list endpoints.
type GameCenterLeaderboardSetVersionsResponse = Response[GameCenterLeaderboardSetVersionAttributes]

// GameCenterLeaderboardSetVersionResponse is the response from leaderboard set version detail endpoints.
type GameCenterLeaderboardSetVersionResponse = SingleResponse[GameCenterLeaderboardSetVersionAttributes]

// GCLeaderboardSetVersionsOption is a functional option for GetGameCenterLeaderboardSetVersions.
type GCLeaderboardSetVersionsOption func(*gcLeaderboardSetVersionsQuery)

type gcLeaderboardSetVersionsQuery struct {
	listQuery
}

// WithGCLeaderboardSetVersionsLimit sets the max number of versions to return.
func WithGCLeaderboardSetVersionsLimit(limit int) GCLeaderboardSetVersionsOption {
	return func(q *gcLeaderboardSetVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetVersionsNextURL uses a next page URL directly.
func WithGCLeaderboardSetVersionsNextURL(next string) GCLeaderboardSetVersionsOption {
	return func(q *gcLeaderboardSetVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardSetVersionsQuery(query *gcLeaderboardSetVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetLocalizationAttributes represents a Game Center leaderboard set localization resource.
type GameCenterLeaderboardSetLocalizationAttributes struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
}

// GameCenterLeaderboardSetLocalizationCreateAttributes describes attributes for creating a leaderboard set localization.
type GameCenterLeaderboardSetLocalizationCreateAttributes struct {
	Locale string `json:"locale"`
	Name   string `json:"name"`
}

// GameCenterLeaderboardSetLocalizationUpdateAttributes describes attributes for updating a leaderboard set localization.
type GameCenterLeaderboardSetLocalizationUpdateAttributes struct {
	Name *string `json:"name,omitempty"`
}

// GameCenterLeaderboardSetLocalizationRelationships describes relationships for leaderboard set localizations.
type GameCenterLeaderboardSetLocalizationRelationships struct {
	GameCenterLeaderboardSet *Relationship `json:"gameCenterLeaderboardSet"`
}

// GameCenterLeaderboardSetLocalizationCreateData is the data portion of a leaderboard set localization create request.
type GameCenterLeaderboardSetLocalizationCreateData struct {
	Type          ResourceType                                         `json:"type"`
	Attributes    GameCenterLeaderboardSetLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetLocalizationRelationships   `json:"relationships,omitempty"`
}

// GameCenterLeaderboardSetLocalizationCreateRequest is a request to create a leaderboard set localization.
type GameCenterLeaderboardSetLocalizationCreateRequest struct {
	Data GameCenterLeaderboardSetLocalizationCreateData `json:"data"`
}

// GameCenterLeaderboardSetLocalizationV2Relationships describes relationships for v2 leaderboard set localizations.
type GameCenterLeaderboardSetLocalizationV2Relationships struct {
	Version *Relationship `json:"version"`
}

// GameCenterLeaderboardSetLocalizationV2CreateData is the data portion of a v2 localization create request.
type GameCenterLeaderboardSetLocalizationV2CreateData struct {
	Type          ResourceType                                         `json:"type"`
	Attributes    GameCenterLeaderboardSetLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetLocalizationV2Relationships `json:"relationships,omitempty"`
}

// GameCenterLeaderboardSetLocalizationV2CreateRequest is a request to create a v2 localization.
type GameCenterLeaderboardSetLocalizationV2CreateRequest struct {
	Data GameCenterLeaderboardSetLocalizationV2CreateData `json:"data"`
}

// GameCenterLeaderboardSetLocalizationUpdateData is the data portion of a leaderboard set localization update request.
type GameCenterLeaderboardSetLocalizationUpdateData struct {
	Type       ResourceType                                          `json:"type"`
	ID         string                                                `json:"id"`
	Attributes *GameCenterLeaderboardSetLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardSetLocalizationUpdateRequest is a request to update a leaderboard set localization.
type GameCenterLeaderboardSetLocalizationUpdateRequest struct {
	Data GameCenterLeaderboardSetLocalizationUpdateData `json:"data"`
}

// GameCenterLeaderboardSetLocalizationsResponse is the response from leaderboard set localization list endpoints.
type GameCenterLeaderboardSetLocalizationsResponse = Response[GameCenterLeaderboardSetLocalizationAttributes]

// GameCenterLeaderboardSetLocalizationResponse is the response from leaderboard set localization detail endpoints.
type GameCenterLeaderboardSetLocalizationResponse = SingleResponse[GameCenterLeaderboardSetLocalizationAttributes]

// GameCenterLeaderboardSetLocalizationDeleteResult represents CLI output for leaderboard set localization deletions.
type GameCenterLeaderboardSetLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardSetLocalizationsOption is a functional option for GetGameCenterLeaderboardSetLocalizations.
type GCLeaderboardSetLocalizationsOption func(*gcLeaderboardSetLocalizationsQuery)

type gcLeaderboardSetLocalizationsQuery struct {
	listQuery
}

// WithGCLeaderboardSetLocalizationsLimit sets the max number of leaderboard set localizations to return.
func WithGCLeaderboardSetLocalizationsLimit(limit int) GCLeaderboardSetLocalizationsOption {
	return func(q *gcLeaderboardSetLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetLocalizationsNextURL uses a next page URL directly.
func WithGCLeaderboardSetLocalizationsNextURL(next string) GCLeaderboardSetLocalizationsOption {
	return func(q *gcLeaderboardSetLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardSetLocalizationsQuery(query *gcLeaderboardSetLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetReleaseAttributes represents a Game Center leaderboard set release resource.
type GameCenterLeaderboardSetReleaseAttributes struct {
	Live bool `json:"live"`
}

// GameCenterLeaderboardSetReleaseRelationships describes relationships for leaderboard set releases.
type GameCenterLeaderboardSetReleaseRelationships struct {
	GameCenterDetail         *Relationship `json:"gameCenterDetail"`
	GameCenterLeaderboardSet *Relationship `json:"gameCenterLeaderboardSet"`
}

// GameCenterLeaderboardSetReleaseCreateData is the data portion of a leaderboard set release create request.
type GameCenterLeaderboardSetReleaseCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Relationships *GameCenterLeaderboardSetReleaseRelationships `json:"relationships"`
}

// GameCenterLeaderboardSetReleaseCreateRequest is a request to create a leaderboard set release.
type GameCenterLeaderboardSetReleaseCreateRequest struct {
	Data GameCenterLeaderboardSetReleaseCreateData `json:"data"`
}

// GameCenterLeaderboardSetReleasesResponse is the response from leaderboard set release list endpoints.
type GameCenterLeaderboardSetReleasesResponse = Response[GameCenterLeaderboardSetReleaseAttributes]

// GameCenterLeaderboardSetReleaseResponse is the response from leaderboard set release detail endpoints.
type GameCenterLeaderboardSetReleaseResponse = SingleResponse[GameCenterLeaderboardSetReleaseAttributes]

// GameCenterLeaderboardSetReleaseDeleteResult represents CLI output for leaderboard set release deletions.
type GameCenterLeaderboardSetReleaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCLeaderboardSetReleasesOption is a functional option for GetGameCenterLeaderboardSetReleases.
type GCLeaderboardSetReleasesOption func(*gcLeaderboardSetReleasesQuery)

type gcLeaderboardSetReleasesQuery struct {
	listQuery
}

// WithGCLeaderboardSetReleasesLimit sets the max number of leaderboard set releases to return.
func WithGCLeaderboardSetReleasesLimit(limit int) GCLeaderboardSetReleasesOption {
	return func(q *gcLeaderboardSetReleasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetReleasesNextURL uses a next page URL directly.
func WithGCLeaderboardSetReleasesNextURL(next string) GCLeaderboardSetReleasesOption {
	return func(q *gcLeaderboardSetReleasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardSetReleasesQuery(query *gcLeaderboardSetReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetMembersUpdateRequest is a request to replace leaderboard set members.
type GameCenterLeaderboardSetMembersUpdateRequest struct {
	Data []RelationshipData `json:"data"`
}

// GameCenterLeaderboardSetMembersUpdateResult represents CLI output for member updates.
type GameCenterLeaderboardSetMembersUpdateResult struct {
	SetID       string   `json:"setId"`
	MemberCount int      `json:"memberCount"`
	MemberIDs   []string `json:"memberIds"`
	Updated     bool     `json:"updated"`
}

// GCLeaderboardSetMembersOption is a functional option for GetGameCenterLeaderboardSetMembers.
type GCLeaderboardSetMembersOption func(*gcLeaderboardSetMembersQuery)

type gcLeaderboardSetMembersQuery struct {
	listQuery
}

// WithGCLeaderboardSetMembersLimit sets the max number of members to return.
func WithGCLeaderboardSetMembersLimit(limit int) GCLeaderboardSetMembersOption {
	return func(q *gcLeaderboardSetMembersQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCLeaderboardSetMembersNextURL uses a next page URL directly.
func WithGCLeaderboardSetMembersNextURL(next string) GCLeaderboardSetMembersOption {
	return func(q *gcLeaderboardSetMembersQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCLeaderboardSetMembersQuery(query *gcLeaderboardSetMembersQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterLeaderboardSetImageAttributes represents a Game Center leaderboard set image resource.
type GameCenterLeaderboardSetImageAttributes struct {
	FileSize           int64               `json:"fileSize"`
	FileName           string              `json:"fileName"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// GameCenterLeaderboardSetImageCreateAttributes describes attributes for reserving an image upload.
type GameCenterLeaderboardSetImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GameCenterLeaderboardSetImageUpdateAttributes describes attributes for committing an image upload.
type GameCenterLeaderboardSetImageUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// GameCenterLeaderboardSetImageRelationships describes relationships for leaderboard set images.
type GameCenterLeaderboardSetImageRelationships struct {
	GameCenterLeaderboardSetLocalization *Relationship `json:"gameCenterLeaderboardSetLocalization"`
}

// GameCenterLeaderboardSetImageCreateData is the data portion of an image create (reserve) request.
type GameCenterLeaderboardSetImageCreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    GameCenterLeaderboardSetImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetImageRelationships   `json:"relationships"`
}

// GameCenterLeaderboardSetImageCreateRequest is a request to reserve an image upload.
type GameCenterLeaderboardSetImageCreateRequest struct {
	Data GameCenterLeaderboardSetImageCreateData `json:"data"`
}

// GameCenterLeaderboardSetImageV2Relationships describes relationships for v2 leaderboard set images.
type GameCenterLeaderboardSetImageV2Relationships struct {
	Localization *Relationship `json:"localization"`
}

// GameCenterLeaderboardSetImageV2CreateData is the data portion of a v2 image create (reserve) request.
type GameCenterLeaderboardSetImageV2CreateData struct {
	Type          ResourceType                                  `json:"type"`
	Attributes    GameCenterLeaderboardSetImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterLeaderboardSetImageV2Relationships `json:"relationships"`
}

// GameCenterLeaderboardSetImageV2CreateRequest is a request to reserve a v2 image upload.
type GameCenterLeaderboardSetImageV2CreateRequest struct {
	Data GameCenterLeaderboardSetImageV2CreateData `json:"data"`
}

// GameCenterLeaderboardSetImageUpdateData is the data portion of an image update (commit) request.
type GameCenterLeaderboardSetImageUpdateData struct {
	Type       ResourceType                                   `json:"type"`
	ID         string                                         `json:"id"`
	Attributes *GameCenterLeaderboardSetImageUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterLeaderboardSetImageUpdateRequest is a request to update a leaderboard set image.
type GameCenterLeaderboardSetImageUpdateRequest struct {
	Data GameCenterLeaderboardSetImageUpdateData `json:"data"`
}

// GameCenterLeaderboardSetImageResponse is the response from leaderboard set image detail endpoints.
type GameCenterLeaderboardSetImageResponse = SingleResponse[GameCenterLeaderboardSetImageAttributes]

// GameCenterLeaderboardSetImageDeleteResult represents CLI output for leaderboard set image deletions.
type GameCenterLeaderboardSetImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterLeaderboardSetImageUploadResult represents CLI output for leaderboard set image uploads.
type GameCenterLeaderboardSetImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}
