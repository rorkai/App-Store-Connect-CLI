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

// AppStoreVersionCreateData is the data portion of an app store version create request.
type AppStoreVersionCreateData struct {
	Type          ResourceType                        `json:"type"`
	Attributes    AppStoreVersionCreateAttributes     `json:"attributes"`
	Relationships *AppStoreVersionCreateRelationships `json:"relationships"`
}

// AppStoreVersionsResponse is the response from app store versions endpoints.
type AppStoreVersionsResponse = Response[AppStoreVersionAttributes]

// PreReleaseVersionAttributes describes TestFlight pre-release version metadata.
type PreReleaseVersionAttributes struct {
	Version  string   `json:"version,omitempty"`
	Platform Platform `json:"platform,omitempty"`
}

// PreReleaseVersion represents a pre-release version resource.
type PreReleaseVersion = Resource[PreReleaseVersionAttributes]

// PreReleaseVersionsResponse is the response from pre-release versions endpoints.
type PreReleaseVersionsResponse = Response[PreReleaseVersionAttributes]

// PreReleaseVersionResponse is the response from pre-release version detail.
type PreReleaseVersionResponse = SingleResponse[PreReleaseVersionAttributes]

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

// GetAppStoreVersion retrieves an app store version by ID.
func (c *Client) GetAppStoreVersion(ctx context.Context, versionID string, opts ...AppStoreVersionOption) (*AppStoreVersionResponse, error) {
	query := applyOptions(opts)
	return getJSON[AppStoreVersionResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/appStoreVersions/%s", versionID),
		Query: buildAppStoreVersionQuery(query),
	})
}

// CreateAppStoreVersion creates a new app store version for an app.
func (c *Client) CreateAppStoreVersion(ctx context.Context, appID string, attrs AppStoreVersionCreateAttributes) (*AppStoreVersionResponse, error) {
	return c.CreateAppStoreVersionWithBuild(ctx, appID, "", attrs)
}

// CreateAppStoreVersionWithBuild creates a new app store version and, when
// buildID is set, attaches that build in the same request.
func (c *Client) CreateAppStoreVersionWithBuild(ctx context.Context, appID, buildID string, attrs AppStoreVersionCreateAttributes) (*AppStoreVersionResponse, error) {
	relationships := &AppStoreVersionCreateRelationships{
		App: &Relationship{
			Data: ResourceData{
				Type: ResourceTypeApps,
				ID:   strings.TrimSpace(appID),
			},
		},
	}
	if buildID = strings.TrimSpace(buildID); buildID != "" {
		relationships.Build = &Relationship{
			Data: ResourceData{
				Type: ResourceTypeBuilds,
				ID:   buildID,
			},
		}
	}
	payload := AppStoreVersionCreateRequest{
		Data: AppStoreVersionCreateData{
			Type:          ResourceTypeAppStoreVersions,
			Attributes:    attrs,
			Relationships: relationships,
		},
	}
	return sendJSON[AppStoreVersionResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/appStoreVersions", Payload: payload})
}

// UpdateAppStoreVersion updates an existing app store version.
func (c *Client) UpdateAppStoreVersion(ctx context.Context, versionID string, attrs AppStoreVersionUpdateAttributes) (*AppStoreVersionResponse, error) {
	return c.UpdateAppStoreVersionWithBuild(ctx, versionID, "", attrs)
}

// UpdateAppStoreVersionWithBuild updates version attributes and optionally
// attaches a build in the same request.
func (c *Client) UpdateAppStoreVersionWithBuild(ctx context.Context, versionID, buildID string, attrs AppStoreVersionUpdateAttributes) (*AppStoreVersionResponse, error) {
	payload := AppStoreVersionUpdateRequest{
		Data: AppStoreVersionUpdateData{
			Type:       ResourceTypeAppStoreVersions,
			ID:         strings.TrimSpace(versionID),
			Attributes: attrs,
		},
	}
	if buildID = strings.TrimSpace(buildID); buildID != "" {
		payload.Data.Relationships = &AppStoreVersionUpdateRelationships{
			Build: &Relationship{Data: ResourceData{Type: ResourceTypeBuilds, ID: buildID}},
		}
	}
	return sendJSON[AppStoreVersionResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/appStoreVersions/%s", strings.TrimSpace(versionID)), Payload: payload})
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
