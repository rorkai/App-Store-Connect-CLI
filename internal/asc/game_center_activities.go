package asc

import (
	"encoding/json"
	"net/url"
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
	Properties          map[string]string `json:"properties,omitzero"`
}

// GameCenterActivityUpdateAttributes describes attributes for updating an activity.
type GameCenterActivityUpdateAttributes struct {
	ReferenceName       *string           `json:"referenceName,omitempty"`
	PlayStyle           *string           `json:"playStyle,omitempty"`
	MinimumPlayersCount *int              `json:"minimumPlayersCount,omitempty"`
	MaximumPlayersCount *int              `json:"maximumPlayersCount,omitempty"`
	SupportsPartyCode   *bool             `json:"supportsPartyCode,omitempty"`
	Archived            *bool             `json:"archived,omitempty"`
	Properties          map[string]string `json:"properties,omitzero"`
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

// GameCenterActivitiesResponse is the response from activity list endpoints.
type GameCenterActivitiesResponse = Response[GameCenterActivityAttributes]

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

// GameCenterActivityVersionsResponse is the response from version list endpoints.
type GameCenterActivityVersionsResponse = Response[GameCenterActivityVersionAttributes]

// GCActivityVersionsOption is a functional option for GetGameCenterActivityVersions.
type GCActivityVersionsOption func(*gcActivityVersionsQuery)

type gcActivityVersionsQuery struct {
	listQuery
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

// GameCenterActivityLocalizationsResponse is the response from localization list endpoints.
type GameCenterActivityLocalizationsResponse = Response[GameCenterActivityLocalizationAttributes]

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

// GameCenterActivityImagesResponse is the response from activity image list endpoints.
type GameCenterActivityImagesResponse = Response[GameCenterActivityImageAttributes]

// GameCenterActivityVersionDefaultImageResponse preserves a missing default image as null.
type GameCenterActivityVersionDefaultImageResponse struct {
	Data     *Resource[GameCenterActivityImageAttributes] `json:"data"`
	Links    Links                                        `json:"links"`
	Included json.RawMessage                              `json:"included,omitempty"`
	Meta     json.RawMessage                              `json:"meta,omitempty"`
}

// GameCenterActivityImageDeleteResult represents CLI output for image deletions.
type GameCenterActivityImageDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// GameCenterActivityImageUploadResult represents CLI output for image uploads.
type GameCenterActivityImageUploadResult struct {
	ID                 string `json:"id"`
	LocalizationID     string `json:"localizationId,omitempty"`
	VersionID          string `json:"versionId,omitempty"`
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

// GameCenterActivityVersionReleasesResponse is the response from release list endpoints.
type GameCenterActivityVersionReleasesResponse = Response[GameCenterActivityVersionReleaseAttributes]

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

func buildGCActivityVersionReleasesQuery(query *gcActivityVersionReleasesQuery) string {
	values := url.Values{}
	addLimit(values, query.limit)
	return values.Encode()
}
