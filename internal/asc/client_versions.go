package asc

import (
	"context"
	"fmt"
	"strings"
)

// AppStoreVersionAttributes describes app store version metadata.
type AppStoreVersionAttributes struct {
	Platform            Platform `json:"platform,omitempty"`
	VersionString       string   `json:"versionString,omitempty"`
	AppStoreState       string   `json:"appStoreState,omitempty"`
	AppVersionState     string   `json:"appVersionState,omitempty"`
	Copyright           string   `json:"copyright,omitempty"`
	CreatedDate         string   `json:"createdDate,omitempty"`
	ReleaseType         string   `json:"releaseType,omitempty"`
	EarliestReleaseDate string   `json:"earliestReleaseDate,omitempty"`
	// Downloadable is nullable in the App Store Connect API. A pointer keeps
	// false distinct from an omitted or null attribute in sparse responses.
	Downloadable *bool `json:"downloadable,omitempty"`

	api apiAttributes
}

// AppStoreVersionCreateAttributes describes app store version create payload attributes.
type AppStoreVersionCreateAttributes struct {
	Platform            Platform `json:"platform"`
	VersionString       string   `json:"versionString"`
	Copyright           string   `json:"copyright,omitempty"`
	ReleaseType         string   `json:"releaseType,omitempty"`
	EarliestReleaseDate string   `json:"earliestReleaseDate,omitempty"`
	ReviewType          string   `json:"reviewType,omitempty"`
}

// AppStoreVersionUpdateAttributes describes app store version update payload attributes.
type AppStoreVersionUpdateAttributes struct {
	Copyright           *string `json:"copyright,omitempty"`
	ReleaseType         *string `json:"releaseType,omitempty"`
	ReviewType          *string `json:"reviewType,omitempty"`
	EarliestReleaseDate *string `json:"earliestReleaseDate,omitempty"`
	VersionString       *string `json:"versionString,omitempty"`
	// Downloadable is nullable on AppStoreVersionUpdateRequest. A pointer keeps
	// an unset flag out of the request body instead of sending false.
	Downloadable *bool `json:"downloadable,omitempty"`
}

// AppStoreVersionUpdateRelationships describes relationships for app store version update requests.
type AppStoreVersionUpdateRelationships struct {
	Build *Relationship `json:"build,omitempty"`
}

// AppStoreVersionUpdateData is the data portion of an app store version update request.
type AppStoreVersionUpdateData struct {
	Type          ResourceType                        `json:"type"`
	ID            string                              `json:"id"`
	Attributes    AppStoreVersionUpdateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionUpdateRelationships `json:"relationships,omitempty"`
}

// AppStoreVersionCreateRelationships describes relationships for app store version create requests.
type AppStoreVersionCreateRelationships struct {
	App   *Relationship `json:"app"`
	Build *Relationship `json:"build,omitempty"`
}

// AppStoreVersionsResponse is the response from app store versions endpoints.
type AppStoreVersionsResponse = Response[AppStoreVersionAttributes]

// PreReleaseVersion represents a pre-release version resource.
type PreReleaseVersion = Resource[PrereleaseVersionAttributes]

// PreReleaseVersionResponse is the response from pre-release version detail.
type PreReleaseVersionResponse = SingleResponse[PrereleaseVersionAttributes]

// AppStoreVersionSubmissionResource represents a submission with relationships.
type AppStoreVersionSubmissionResource struct {
	Type       ResourceType `json:"type"`
	ID         string       `json:"id"`
	Attributes struct {
		CreatedDate *string `json:"createdDate,omitempty"`
	} `json:"attributes"`
	Relationships struct {
		AppStoreVersion *Relationship `json:"appStoreVersion,omitempty"`
	} `json:"relationships"`
}

// AppStoreVersionSubmissionResourceResponse is a response containing a submission resource.
type AppStoreVersionSubmissionResourceResponse struct {
	Data AppStoreVersionSubmissionResource `json:"data"`
}

// AppStoreVersionBuildRelationshipUpdateRequest is a request to attach a build to a version.
type AppStoreVersionBuildRelationshipUpdateRequest struct {
	Data ResourceData `json:"data"`
}

// GetPreReleaseVersions retrieves TestFlight pre-release versions for an app.
func (c *Client) GetPreReleaseVersions(ctx context.Context, appID string, opts ...PreReleaseVersionsOption) (*PreReleaseVersionsResponse, error) {
	query := applyOptions(opts)
	appID = strings.TrimSpace(appID)
	return getJSON[PreReleaseVersionsResponse](ctx, c, readRequest{
		Path:        "/v1/preReleaseVersions",
		Query:       buildPreReleaseVersionsQuery(appID, query),
		Next:        query.nextURL,
		NextContext: "preReleaseVersions",
	})
}

// CreateAppStoreVersion creates a new app store version for an app.
func (c *Client) CreateAppStoreVersion(ctx context.Context, appID string, attrs AppStoreVersionCreateAttributes) (*AppStoreVersionResponse, error) {
	return c.CreateAppStoreVersionWithBuild(ctx, appID, attrs, "")
}

// UpdateAppStoreVersion updates an existing app store version.
func (c *Client) UpdateAppStoreVersion(ctx context.Context, versionID string, attrs AppStoreVersionUpdateAttributes) (*AppStoreVersionResponse, error) {
	return c.UpdateAppStoreVersionWithBuild(ctx, versionID, attrs, "")
}

// AttachBuildToVersion attaches a build to an app store version.
func (c *Client) AttachBuildToVersion(ctx context.Context, versionID, buildID string) error {
	request := AppStoreVersionBuildRelationshipUpdateRequest{
		Data: ResourceData{
			Type: ResourceTypeBuilds,
			ID:   buildID,
		},
	}
	return sendNoContent(ctx, c, "PATCH", fmt.Sprintf("/v1/appStoreVersions/%s/relationships/build", versionID), request)
}
