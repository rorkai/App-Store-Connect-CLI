package asc

import (
	"net/url"
	"strings"
)

// GameCenterActivityAttributes represents a Game Center activity resource.
type GameCenterActivityAttributes struct {
	ReferenceName       string            `json:"referenceName"`
	VendorIdentifier    string            `json:"vendorIdentifier"`
	PlayStyle           string            `json:"playStyle,omitempty"`
	MinimumPlayersCount int               `json:"minimumPlayersCount,omitempty"`
	MaximumPlayersCount int               `json:"maximumPlayersCount,omitempty"`
	SupportsPartyCode   bool              `json:"supportsPartyCode,omitempty"`
	Archived            bool              `json:"archived,omitempty"`
	Properties          map[string]string `json:"properties,omitempty"`
}

// GameCenterActivityCreateAttributes describes attributes for creating an activity.
type GameCenterActivityCreateAttributes struct {
	ReferenceName       string            `json:"referenceName"`
	VendorIdentifier    string            `json:"vendorIdentifier"`
	PlayStyle           *string           `json:"playStyle,omitempty"`
	MinimumPlayersCount *int              `json:"minimumPlayersCount,omitempty"`
	MaximumPlayersCount *int              `json:"maximumPlayersCount,omitempty"`
	SupportsPartyCode   *bool             `json:"supportsPartyCode,omitempty"`
	Properties          map[string]string `json:"properties,omitempty"`
}

// GameCenterActivityUpdateAttributes describes attributes for updating an activity.
type GameCenterActivityUpdateAttributes struct {
	ReferenceName       *string           `json:"referenceName,omitempty"`
	PlayStyle           *string           `json:"playStyle,omitempty"`
	MinimumPlayersCount *int              `json:"minimumPlayersCount,omitempty"`
	MaximumPlayersCount *int              `json:"maximumPlayersCount,omitempty"`
	SupportsPartyCode   *bool             `json:"supportsPartyCode,omitempty"`
	Archived            *bool             `json:"archived,omitempty"`
	Properties          map[string]string `json:"properties,omitempty"`
}

// GameCenterActivityRelationships describes relationships for activities.
type GameCenterActivityRelationships struct {
	GameCenterDetail *Relationship     `json:"gameCenterDetail,omitempty"`
	GameCenterGroup  *Relationship     `json:"gameCenterGroup,omitempty"`
	Versions         *RelationshipList `json:"versions,omitempty"`
}

// GameCenterActivityCreateData is the data portion of an activity create request.
type GameCenterActivityCreateData struct {
	Type          ResourceType                       `json:"type"`
	Attributes    GameCenterActivityCreateAttributes `json:"attributes"`
	Relationships *GameCenterActivityRelationships   `json:"relationships,omitempty"`
}

// GameCenterActivityCreateRequest is a request to create an activity.
type GameCenterActivityCreateRequest struct {
	Data     GameCenterActivityCreateData            `json:"data"`
	Included []GameCenterActivityVersionInlineCreate `json:"included,omitempty"`
}

// GameCenterActivityVersionInlineCreate is an inline resource for creating an
// initial activity version alongside the parent activity.
type GameCenterActivityVersionInlineCreate struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id,omitempty"`
	Attributes *GameCenterActivityVersionCreateAttributes `json:"attributes,omitempty"`
}

// GameCenterActivityUpdateData is the data portion of an activity update request.
type GameCenterActivityUpdateData struct {
	Type       ResourceType                        `json:"type"`
	ID         string                              `json:"id"`
	Attributes *GameCenterActivityUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterActivityUpdateRequest is a request to update an activity.
type GameCenterActivityUpdateRequest struct {
	Data GameCenterActivityUpdateData `json:"data"`
}

// GameCenterActivitiesResponse is the response from activity list endpoints.
type GameCenterActivitiesResponse = Response[GameCenterActivityAttributes]

// GameCenterActivityResponse is the response from activity detail endpoints.
type GameCenterActivityResponse = SingleResponse[GameCenterActivityAttributes]

// GameCenterActivityDeleteResult represents CLI output for activity deletions.
type GameCenterActivityDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCActivitiesOption is a functional option for GetGameCenterActivities.
type GCActivitiesOption func(*gcActivitiesQuery)

type gcActivitiesQuery struct {
	listQuery
}

// WithGCActivitiesLimit sets the max number of activities to return.
func WithGCActivitiesLimit(limit int) GCActivitiesOption {
	return func(q *gcActivitiesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCActivitiesNextURL uses a next page URL directly.
func WithGCActivitiesNextURL(next string) GCActivitiesOption {
	return func(q *gcActivitiesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCActivitiesQuery(query *gcActivitiesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterActivityVersionAttributes represents a Game Center activity version resource.
type GameCenterActivityVersionAttributes struct {
	Version     int                    `json:"version,omitempty"`
	State       GameCenterVersionState `json:"state,omitempty"`
	FallbackURL string                 `json:"fallbackUrl,omitempty"`
}

// GameCenterActivityVersionRelationships describes relationships for activity versions.
type GameCenterActivityVersionRelationships struct {
	Activity *Relationship `json:"activity,omitempty"`
}

// GameCenterActivityVersionCreateAttributes describes attributes for creating a version.
type GameCenterActivityVersionCreateAttributes struct {
	FallbackURL *string `json:"fallbackUrl,omitempty"`
}

// GameCenterActivityVersionCreateData is the data portion of a version create request.
type GameCenterActivityVersionCreateData struct {
	Type          ResourceType                               `json:"type"`
	Attributes    *GameCenterActivityVersionCreateAttributes `json:"attributes,omitempty"`
	Relationships *GameCenterActivityVersionRelationships    `json:"relationships,omitempty"`
}

// GameCenterActivityVersionCreateRequest is a request to create an activity version.
type GameCenterActivityVersionCreateRequest struct {
	Data GameCenterActivityVersionCreateData `json:"data"`
}

// GameCenterActivityVersionUpdateAttributes describes attributes for updating a version.
type GameCenterActivityVersionUpdateAttributes struct {
	FallbackURL *string `json:"fallbackUrl,omitempty"`
}

// GameCenterActivityVersionUpdateData is the data portion of a version update request.
type GameCenterActivityVersionUpdateData struct {
	Type       ResourceType                               `json:"type"`
	ID         string                                     `json:"id"`
	Attributes *GameCenterActivityVersionUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterActivityVersionUpdateRequest is a request to update an activity version.
type GameCenterActivityVersionUpdateRequest struct {
	Data GameCenterActivityVersionUpdateData `json:"data"`
}

// GameCenterActivityVersionsResponse is the response from version list endpoints.
type GameCenterActivityVersionsResponse = Response[GameCenterActivityVersionAttributes]

// GameCenterActivityVersionResponse is the response from version detail endpoints.
type GameCenterActivityVersionResponse = SingleResponse[GameCenterActivityVersionAttributes]

// GCActivityVersionsOption is a functional option for GetGameCenterActivityVersions.
type GCActivityVersionsOption func(*gcActivityVersionsQuery)

type gcActivityVersionsQuery struct {
	listQuery
}

// WithGCActivityVersionsLimit sets the max number of versions to return.
func WithGCActivityVersionsLimit(limit int) GCActivityVersionsOption {
	return func(q *gcActivityVersionsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCActivityVersionsNextURL uses a next page URL directly.
func WithGCActivityVersionsNextURL(next string) GCActivityVersionsOption {
	return func(q *gcActivityVersionsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCActivityVersionsQuery(query *gcActivityVersionsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterActivityLocalizationAttributes represents an activity localization.
type GameCenterActivityLocalizationAttributes struct {
	Locale      string `json:"locale"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// GameCenterActivityLocalizationCreateAttributes describes attributes for creating a localization.
type GameCenterActivityLocalizationCreateAttributes struct {
	Locale      string `json:"locale"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// GameCenterActivityLocalizationUpdateAttributes describes attributes for updating a localization.
type GameCenterActivityLocalizationUpdateAttributes struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

// GameCenterActivityLocalizationRelationships describes relationships for activity localizations.
type GameCenterActivityLocalizationRelationships struct {
	Version *Relationship `json:"version"`
}

// GameCenterActivityLocalizationCreateData is the data portion of a localization create request.
type GameCenterActivityLocalizationCreateData struct {
	Type          ResourceType                                   `json:"type"`
	Attributes    GameCenterActivityLocalizationCreateAttributes `json:"attributes"`
	Relationships *GameCenterActivityLocalizationRelationships   `json:"relationships,omitempty"`
}

// GameCenterActivityLocalizationCreateRequest is a request to create a localization.
type GameCenterActivityLocalizationCreateRequest struct {
	Data GameCenterActivityLocalizationCreateData `json:"data"`
}

// GameCenterActivityLocalizationUpdateData is the data portion of a localization update request.
type GameCenterActivityLocalizationUpdateData struct {
	Type       ResourceType                                    `json:"type"`
	ID         string                                          `json:"id"`
	Attributes *GameCenterActivityLocalizationUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterActivityLocalizationUpdateRequest is a request to update a localization.
type GameCenterActivityLocalizationUpdateRequest struct {
	Data GameCenterActivityLocalizationUpdateData `json:"data"`
}

// GameCenterActivityLocalizationsResponse is the response from localization list endpoints.
type GameCenterActivityLocalizationsResponse = Response[GameCenterActivityLocalizationAttributes]

// GameCenterActivityLocalizationResponse is the response from localization detail endpoints.
type GameCenterActivityLocalizationResponse = SingleResponse[GameCenterActivityLocalizationAttributes]

// GameCenterActivityLocalizationDeleteResult represents CLI output for localization deletions.
type GameCenterActivityLocalizationDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCActivityLocalizationsOption is a functional option for GetGameCenterActivityLocalizations.
type GCActivityLocalizationsOption func(*gcActivityLocalizationsQuery)

type gcActivityLocalizationsQuery struct {
	listQuery
}

// WithGCActivityLocalizationsLimit sets the max number of localizations to return.
func WithGCActivityLocalizationsLimit(limit int) GCActivityLocalizationsOption {
	return func(q *gcActivityLocalizationsQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCActivityLocalizationsNextURL uses a next page URL directly.
func WithGCActivityLocalizationsNextURL(next string) GCActivityLocalizationsOption {
	return func(q *gcActivityLocalizationsQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCActivityLocalizationsQuery(query *gcActivityLocalizationsQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}

// GameCenterActivityImageAttributes represents an activity image resource.
type GameCenterActivityImageAttributes struct {
	FileSize           int64               `json:"fileSize"`
	FileName           string              `json:"fileName"`
	ImageAsset         *ImageAsset         `json:"imageAsset,omitempty"`
	UploadOperations   []UploadOperation   `json:"uploadOperations,omitempty"`
	AssetDeliveryState *AssetDeliveryState `json:"assetDeliveryState,omitempty"`
}

// GameCenterActivityImageCreateAttributes describes attributes for reserving an image upload.
type GameCenterActivityImageCreateAttributes struct {
	FileSize int64  `json:"fileSize"`
	FileName string `json:"fileName"`
}

// GameCenterActivityImageUpdateAttributes describes attributes for committing an image upload.
type GameCenterActivityImageUpdateAttributes struct {
	Uploaded *bool `json:"uploaded,omitempty"`
}

// GameCenterActivityImageRelationships describes relationships for activity images.
type GameCenterActivityImageRelationships struct {
	Localization *Relationship `json:"localization"`
}

// GameCenterActivityImageCreateData is the data portion of an image create request.
type GameCenterActivityImageCreateData struct {
	Type          ResourceType                            `json:"type"`
	Attributes    GameCenterActivityImageCreateAttributes `json:"attributes"`
	Relationships *GameCenterActivityImageRelationships   `json:"relationships"`
}

// GameCenterActivityImageCreateRequest is a request to reserve an image upload.
type GameCenterActivityImageCreateRequest struct {
	Data GameCenterActivityImageCreateData `json:"data"`
}

// GameCenterActivityImageUpdateData is the data portion of an image update (commit) request.
type GameCenterActivityImageUpdateData struct {
	Type       ResourceType                             `json:"type"`
	ID         string                                   `json:"id"`
	Attributes *GameCenterActivityImageUpdateAttributes `json:"attributes,omitempty"`
}

// GameCenterActivityImageUpdateRequest is a request to commit an image upload.
type GameCenterActivityImageUpdateRequest struct {
	Data GameCenterActivityImageUpdateData `json:"data"`
}

// GameCenterActivityImagesResponse is the response from activity image list endpoints.
type GameCenterActivityImagesResponse = Response[GameCenterActivityImageAttributes]

// GameCenterActivityImageResponse is the response from activity image detail endpoints.
type GameCenterActivityImageResponse = SingleResponse[GameCenterActivityImageAttributes]

// GameCenterActivityImageDeleteResult represents CLI output for image deletions.
type GameCenterActivityImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterActivityImageUploadResult represents CLI output for image uploads.
type GameCenterActivityImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId"`
	FileName           string `json:"fileName"`
	FileSize           int64  `json:"fileSize"`
	AssetDeliveryState string `json:"assetDeliveryState,omitempty"`
	Uploaded           bool   `json:"uploaded"`
}

// GameCenterActivityVersionReleaseAttributes represents an activity version release.
type GameCenterActivityVersionReleaseAttributes struct{}

// GameCenterActivityVersionReleaseRelationships describes relationships for activity version releases.
type GameCenterActivityVersionReleaseRelationships struct {
	Version *Relationship `json:"version,omitempty"`
}

// GameCenterActivityVersionReleaseCreateData is the data portion of a release create request.
type GameCenterActivityVersionReleaseCreateData struct {
	Type          ResourceType                                   `json:"type"`
	Relationships *GameCenterActivityVersionReleaseRelationships `json:"relationships,omitempty"`
}

// GameCenterActivityVersionReleaseCreateRequest is a request to create a release.
type GameCenterActivityVersionReleaseCreateRequest struct {
	Data GameCenterActivityVersionReleaseCreateData `json:"data"`
}

// GameCenterActivityVersionReleasesResponse is the response from release list endpoints.
type GameCenterActivityVersionReleasesResponse = Response[GameCenterActivityVersionReleaseAttributes]

// GameCenterActivityVersionReleaseResponse is the response from release detail endpoints.
type GameCenterActivityVersionReleaseResponse = SingleResponse[GameCenterActivityVersionReleaseAttributes]

// GameCenterActivityVersionReleaseDeleteResult represents CLI output for release deletions.
type GameCenterActivityVersionReleaseDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GCActivityVersionReleasesOption is a functional option for GetGameCenterActivityVersionReleases.
type GCActivityVersionReleasesOption func(*gcActivityVersionReleasesQuery)

type gcActivityVersionReleasesQuery struct {
	listQuery
}

// WithGCActivityVersionReleasesLimit sets the max number of releases to return.
func WithGCActivityVersionReleasesLimit(limit int) GCActivityVersionReleasesOption {
	return func(q *gcActivityVersionReleasesQuery) {
		if limit > 0 {
			q.limit = limit
		}
	}
}

// WithGCActivityVersionReleasesNextURL uses a next page URL directly.
func WithGCActivityVersionReleasesNextURL(next string) GCActivityVersionReleasesOption {
	return func(q *gcActivityVersionReleasesQuery) {
		if strings.TrimSpace(next) != "" {
			q.nextURL = strings.TrimSpace(next)
		}
	}
}

func buildGCActivityVersionReleasesQuery(query *gcActivityVersionReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
