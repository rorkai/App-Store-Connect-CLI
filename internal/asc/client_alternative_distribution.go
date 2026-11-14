package asc

import (
	"context"
	"fmt"
	"strings"
)

// CreateAlternativeDistributionDomain creates an alternative distribution domain.
func (c *Client) CreateAlternativeDistributionDomain(ctx context.Context, domain, referenceName string) (*AlternativeDistributionDomainResponse, error) {
	domain = strings.TrimSpace(domain)
	referenceName = strings.TrimSpace(referenceName)
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if referenceName == "" {
		return nil, fmt.Errorf("referenceName is required")
	}

	payload := AlternativeDistributionDomainCreateRequest{
		Data: AlternativeDistributionDomainCreateData{
			Type: ResourceTypeAlternativeDistributionDomains,
			Attributes: AlternativeDistributionDomainCreateAttributes{
				Domain:        domain,
				ReferenceName: referenceName,
			},
		},
	}

	return sendJSON[AlternativeDistributionDomainResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/alternativeDistributionDomains",
		Payload:  payload,
		ParseErr: "failed to parse alternative distribution domain response",
	})
}

// GetAlternativeDistributionKeys retrieves alternative distribution keys.
func (c *Client) GetAlternativeDistributionKeys(ctx context.Context, opts ...AlternativeDistributionKeysOption) (*AlternativeDistributionKeysResponse, error) {
	query := applyOptions(opts)
	return getJSON[AlternativeDistributionKeysResponse](ctx, c, readRequest{
		Path:        "/v1/alternativeDistributionKeys",
		Query:       buildAlternativeDistributionKeysQuery(query),
		Next:        query.nextURL,
		NextContext: "alternativeDistributionKeys",
		ParseErr:    "failed to parse alternative distribution keys response",
	})
}

// GetAlternativeDistributionKey retrieves an alternative distribution key by ID.
func (c *Client) GetAlternativeDistributionKey(ctx context.Context, keyID string) (*AlternativeDistributionKeyResponse, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("keyID is required")
	}

	return getJSON[AlternativeDistributionKeyResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/alternativeDistributionKeys/%s", keyID),
		ParseErr: "failed to parse alternative distribution key response",
	})
}

// CreateAlternativeDistributionKey creates an alternative distribution key.
func (c *Client) CreateAlternativeDistributionKey(ctx context.Context, appID, publicKey string) (*AlternativeDistributionKeyResponse, error) {
	appID = strings.TrimSpace(appID)
	publicKey = strings.TrimSpace(publicKey)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}
	if publicKey == "" {
		return nil, fmt.Errorf("publicKey is required")
	}

	payload := AlternativeDistributionKeyCreateRequest{
		Data: AlternativeDistributionKeyCreateData{
			Type:       ResourceTypeAlternativeDistributionKeys,
			Attributes: AlternativeDistributionKeyCreateAttributes{PublicKey: publicKey},
			Relationships: &AlternativeDistributionKeyCreateRelationships{
				App: &Relationship{
					Data: ResourceData{
						Type: ResourceTypeApps,
						ID:   appID,
					},
				},
			},
		},
	}

	return sendJSON[AlternativeDistributionKeyResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/alternativeDistributionKeys",
		Payload:  payload,
		ParseErr: "failed to parse alternative distribution key response",
	})
}

// DeleteAlternativeDistributionKey deletes an alternative distribution key by ID.
func (c *Client) DeleteAlternativeDistributionKey(ctx context.Context, keyID string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("keyID is required")
	}

	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/alternativeDistributionKeys/%s", keyID), nil)
}

// GetAlternativeDistributionPackage retrieves an alternative distribution package by ID.
func (c *Client) GetAlternativeDistributionPackage(ctx context.Context, packageID string) (*AlternativeDistributionPackageResponse, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, fmt.Errorf("packageID is required")
	}

	return getJSON[AlternativeDistributionPackageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/alternativeDistributionPackages/%s", packageID),
		ParseErr: "failed to parse alternative distribution package response",
	})
}

// GetAlternativeDistributionPackageForVersion retrieves a package for an app store version.
func (c *Client) GetAlternativeDistributionPackageForVersion(ctx context.Context, versionID string) (*AlternativeDistributionPackageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[AlternativeDistributionPackageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appStoreVersions/%s/alternativeDistributionPackage", versionID),
		ParseErr: "failed to parse alternative distribution package response",
	})
}

// CreateAlternativeDistributionPackage creates an alternative distribution package.
func (c *Client) CreateAlternativeDistributionPackage(ctx context.Context, appStoreVersionID string) (*AlternativeDistributionPackageResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	payload := AlternativeDistributionPackageCreateRequest{
		Data: AlternativeDistributionPackageCreateData{
			Type: ResourceTypeAlternativeDistributionPackages,
			Relationships: AlternativeDistributionPackageCreateRelationships{
				AppStoreVersion: Relationship{
					Data: ResourceData{
						Type: ResourceTypeAppStoreVersions,
						ID:   appStoreVersionID,
					},
				},
			},
		},
	}

	return sendJSON[AlternativeDistributionPackageResponse](ctx, c, writeRequest{
		Method:   "POST",
		Path:     "/v1/alternativeDistributionPackages",
		Payload:  payload,
		ParseErr: "failed to parse alternative distribution package response",
	})
}

// GetAlternativeDistributionPackageVersions retrieves package versions for a package.
func (c *Client) GetAlternativeDistributionPackageVersions(ctx context.Context, packageID string, opts ...AlternativeDistributionPackageVersionsOption) (*AlternativeDistributionPackageVersionsResponse, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, fmt.Errorf("packageID is required")
	}

	query := applyOptions(opts)
	return getJSON[AlternativeDistributionPackageVersionsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/alternativeDistributionPackages/%s/versions", packageID),
		Query:       buildAlternativeDistributionPackageVersionsQuery(query),
		Next:        query.nextURL,
		NextContext: "alternativeDistributionPackageVersions",
		ParseErr:    "failed to parse alternative distribution package versions response",
	})
}

// GetAlternativeDistributionPackageVersion retrieves a package version by ID.
func (c *Client) GetAlternativeDistributionPackageVersion(ctx context.Context, versionID string) (*AlternativeDistributionPackageVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	return getJSON[AlternativeDistributionPackageVersionResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s", versionID),
		ParseErr: "failed to parse alternative distribution package version response",
	})
}

// GetAlternativeDistributionPackageVersionVariants retrieves variants for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionVariants(ctx context.Context, versionID string, opts ...AlternativeDistributionPackageVariantsOption) (*AlternativeDistributionPackageVariantsResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	query := applyOptions(opts)
	return getJSON[AlternativeDistributionPackageVariantsResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s/variants", versionID),
		Query:       buildAlternativeDistributionPackageVariantsQuery(query),
		Next:        query.nextURL,
		NextContext: "alternativeDistributionPackageVariants",
		ParseErr:    "failed to parse alternative distribution package variants response",
	})
}

// GetAlternativeDistributionPackageVersionDeltas retrieves deltas for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionDeltas(ctx context.Context, versionID string, opts ...AlternativeDistributionPackageDeltasOption) (*AlternativeDistributionPackageDeltasResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	query := applyOptions(opts)
	return getJSON[AlternativeDistributionPackageDeltasResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s/deltas", versionID),
		Query:       buildAlternativeDistributionPackageDeltasQuery(query),
		Next:        query.nextURL,
		NextContext: "alternativeDistributionPackageDeltas",
		ParseErr:    "failed to parse alternative distribution package deltas response",
	})
}

// GetAlternativeDistributionPackageVariant retrieves a package variant by ID.
func (c *Client) GetAlternativeDistributionPackageVariant(ctx context.Context, variantID string) (*AlternativeDistributionPackageVariantResponse, error) {
	variantID = strings.TrimSpace(variantID)
	if variantID == "" {
		return nil, fmt.Errorf("variantID is required")
	}

	return getJSON[AlternativeDistributionPackageVariantResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/alternativeDistributionPackageVariants/%s", variantID),
		ParseErr: "failed to parse alternative distribution package variant response",
	})
}

// GetAlternativeDistributionPackageDelta retrieves a package delta by ID.
func (c *Client) GetAlternativeDistributionPackageDelta(ctx context.Context, deltaID string) (*AlternativeDistributionPackageDeltaResponse, error) {
	deltaID = strings.TrimSpace(deltaID)
	if deltaID == "" {
		return nil, fmt.Errorf("deltaID is required")
	}

	return getJSON[AlternativeDistributionPackageDeltaResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/alternativeDistributionPackageDeltas/%s", deltaID),
		ParseErr: "failed to parse alternative distribution package delta response",
	})
}

// GetAppStoreVersionAlternativeDistributionPackage retrieves the alternative distribution package for an app store version.
func (c *Client) GetAppStoreVersionAlternativeDistributionPackage(ctx context.Context, appStoreVersionID string) (*AlternativeDistributionPackageResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	return getJSON[AlternativeDistributionPackageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appStoreVersions/%s/alternativeDistributionPackage", appStoreVersionID),
		ParseErr: "failed to parse alternative distribution package response",
	})
}

// GetAppStoreVersionAlternativeDistributionPackageRelationship retrieves the package relationship for an app store version.
func (c *Client) GetAppStoreVersionAlternativeDistributionPackageRelationship(ctx context.Context, appStoreVersionID string) (*AppStoreVersionAlternativeDistributionPackageLinkageResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	return getJSON[AppStoreVersionAlternativeDistributionPackageLinkageResponse](ctx, c, readRequest{
		Path:     fmt.Sprintf("/v1/appStoreVersions/%s/relationships/alternativeDistributionPackage", appStoreVersionID),
		ParseErr: "failed to parse alternative distribution package relationship response",
	})
}
