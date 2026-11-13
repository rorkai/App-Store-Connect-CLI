package asc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// BuildAppLinkageResponse is the response for build app relationships.
type BuildAppLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildAppStoreVersionLinkageResponse is the response for build app store version relationships.
type BuildAppStoreVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildBuildBetaDetailLinkageResponse is the response for build beta detail relationships.
type BuildBuildBetaDetailLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildPreReleaseVersionLinkageResponse is the response for build pre-release version relationships.
type BuildPreReleaseVersionLinkageResponse struct {
	Data  ResourceData `json:"data"`
	Links Links        `json:"links"`
}

// BuildAppEncryptionDeclarationRelationshipUpdateRequest is a request to update the appEncryptionDeclaration relationship on a build.
type BuildAppEncryptionDeclarationRelationshipUpdateRequest struct {
	Data ResourceData `json:"data"`
}

// GetBuildAppRelationship retrieves the app linkage for a build.
func (c *Client) GetBuildAppRelationship(ctx context.Context, buildID string) (*BuildAppLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	return getJSON[BuildAppLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/builds/%s/relationships/app", buildID), ParseErr: "failed to parse app relationship response"})
}

// UpdateBuildAppEncryptionDeclarationRelationship updates the app encryption declaration relationship on a build.
func (c *Client) UpdateBuildAppEncryptionDeclarationRelationship(ctx context.Context, buildID, declarationID string) error {
	buildID = strings.TrimSpace(buildID)
	declarationID = strings.TrimSpace(declarationID)
	if buildID == "" {
		return fmt.Errorf("buildID is required")
	}
	if declarationID == "" {
		return fmt.Errorf("declarationID is required")
	}

	request := BuildAppEncryptionDeclarationRelationshipUpdateRequest{
		Data: ResourceData{
			Type: ResourceTypeAppEncryptionDeclarations,
			ID:   declarationID,
		},
	}
	return sendNoContent(ctx, c, http.MethodPatch, fmt.Sprintf("/v1/builds/%s/relationships/appEncryptionDeclaration", buildID), request)
}

// GetBuildAppStoreVersionRelationship retrieves the app store version linkage for a build.
func (c *Client) GetBuildAppStoreVersionRelationship(ctx context.Context, buildID string) (*BuildAppStoreVersionLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	return getJSON[BuildAppStoreVersionLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/builds/%s/relationships/appStoreVersion", buildID), ParseErr: "failed to parse appStoreVersion relationship response"})
}

// GetBuildBetaBuildLocalizationsRelationships retrieves beta build localization linkages for a build.
func (c *Client) GetBuildBetaBuildLocalizationsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "betaBuildLocalizations", opts...)
}

// GetBuildBuildBetaDetailRelationship retrieves build beta detail linkage for a build.
func (c *Client) GetBuildBuildBetaDetailRelationship(ctx context.Context, buildID string) (*BuildBuildBetaDetailLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	return getJSON[BuildBuildBetaDetailLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/builds/%s/relationships/buildBetaDetail", buildID), ParseErr: "failed to parse buildBetaDetail relationship response"})
}

// GetBuildDiagnosticSignaturesRelationships retrieves diagnostic signature linkages for a build.
func (c *Client) GetBuildDiagnosticSignaturesRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "diagnosticSignatures", opts...)
}

// GetBuildIndividualTestersRelationships retrieves individual tester linkages for a build.
func (c *Client) GetBuildIndividualTestersRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "individualTesters", opts...)
}

// GetBuildPreReleaseVersionRelationship retrieves pre-release version linkage for a build.
func (c *Client) GetBuildPreReleaseVersionRelationship(ctx context.Context, buildID string) (*BuildPreReleaseVersionLinkageResponse, error) {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return nil, fmt.Errorf("buildID is required")
	}

	return getJSON[BuildPreReleaseVersionLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/builds/%s/relationships/preReleaseVersion", buildID), ParseErr: "failed to parse preReleaseVersion relationship response"})
}

// GetBuildIconsRelationships retrieves build icon linkages for a build.
func (c *Client) GetBuildIconsRelationships(ctx context.Context, buildID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getBuildLinkages(ctx, buildID, "icons", opts...)
}

func (c *Client) getBuildLinkages(ctx context.Context, buildID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getResourceLinkages(
		ctx,
		buildID,
		relationship,
		"buildID",
		"/v1/builds/%s/relationships/%s",
		"buildRelationships",
		opts...,
	)
}
