package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// GetAlternativeDistributionDomains retrieves alternative distribution domains.
func (c *Client) GetAlternativeDistributionDomains(ctx context.Context, opts ...AlternativeDistributionDomainsOption) (*AlternativeDistributionDomainsResponse, error) {
	query := &alternativeDistributionDomainsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/alternativeDistributionDomains"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionDomains: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAlternativeDistributionDomainsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionDomainsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution domains response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionDomain retrieves a single alternative distribution domain by ID.
func (c *Client) GetAlternativeDistributionDomain(ctx context.Context, domainID string) (*AlternativeDistributionDomainResponse, error) {
	domainID = strings.TrimSpace(domainID)
	if domainID == "" {
		return nil, fmt.Errorf("domainID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionDomains/%s", domainID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionDomainResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution domain response: %w", err)
	}

	return &response, nil
}

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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/alternativeDistributionDomains", body)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionDomainResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution domain response: %w", err)
	}

	return &response, nil
}

// DeleteAlternativeDistributionDomain deletes an alternative distribution domain by ID.
func (c *Client) DeleteAlternativeDistributionDomain(ctx context.Context, domainID string) error {
	domainID = strings.TrimSpace(domainID)
	if domainID == "" {
		return fmt.Errorf("domainID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionDomains/%s", domainID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetAlternativeDistributionKeys retrieves alternative distribution keys.
func (c *Client) GetAlternativeDistributionKeys(ctx context.Context, opts ...AlternativeDistributionKeysOption) (*AlternativeDistributionKeysResponse, error) {
	query := &alternativeDistributionKeysQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/alternativeDistributionKeys"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionKeys: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAlternativeDistributionKeysQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionKeysResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution keys response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionKey retrieves an alternative distribution key by ID.
func (c *Client) GetAlternativeDistributionKey(ctx context.Context, keyID string) (*AlternativeDistributionKeyResponse, error) {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return nil, fmt.Errorf("keyID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionKeys/%s", keyID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionKeyResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution key response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/alternativeDistributionKeys", body)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionKeyResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution key response: %w", err)
	}

	return &response, nil
}

// DeleteAlternativeDistributionKey deletes an alternative distribution key by ID.
func (c *Client) DeleteAlternativeDistributionKey(ctx context.Context, keyID string) error {
	keyID = strings.TrimSpace(keyID)
	if keyID == "" {
		return fmt.Errorf("keyID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionKeys/%s", keyID)
	_, err := c.do(ctx, "DELETE", path, nil)
	return err
}

// GetAlternativeDistributionPackage retrieves an alternative distribution package by ID.
func (c *Client) GetAlternativeDistributionPackage(ctx context.Context, packageID string) (*AlternativeDistributionPackageResponse, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, fmt.Errorf("packageID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackages/%s", packageID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageForVersion retrieves a package for an app store version.
func (c *Client) GetAlternativeDistributionPackageForVersion(ctx context.Context, versionID string) (*AlternativeDistributionPackageResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/alternativeDistributionPackage", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/alternativeDistributionPackages", body)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersions retrieves package versions for a package.
func (c *Client) GetAlternativeDistributionPackageVersions(ctx context.Context, packageID string, opts ...AlternativeDistributionPackageVersionsOption) (*AlternativeDistributionPackageVersionsResponse, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, fmt.Errorf("packageID is required")
	}

	query := &alternativeDistributionPackageVersionsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackages/%s/versions", packageID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionPackageVersions: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAlternativeDistributionPackageVersionsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageVersionsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package versions response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersionsRelationships retrieves version linkages for a package.
func (c *Client) GetAlternativeDistributionPackageVersionsRelationships(ctx context.Context, packageID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	packageID = strings.TrimSpace(packageID)
	if packageID == "" {
		return nil, fmt.Errorf("packageID is required")
	}

	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackages/%s/relationships/versions", packageID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionPackageVersionsRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package version relationships response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersion retrieves a package version by ID.
func (c *Client) GetAlternativeDistributionPackageVersion(ctx context.Context, versionID string) (*AlternativeDistributionPackageVersionResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s", versionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageVersionResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package version response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersionVariants retrieves variants for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionVariants(ctx context.Context, versionID string, opts ...AlternativeDistributionPackageVariantsOption) (*AlternativeDistributionPackageVariantsResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	query := &alternativeDistributionPackageVariantsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s/variants", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionPackageVariants: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAlternativeDistributionPackageVariantsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageVariantsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package variants response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersionVariantsRelationships retrieves variant linkages for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionVariantsRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAlternativeDistributionPackageVersionLinkages(ctx, versionID, "variants", opts...)
}

// GetAlternativeDistributionPackageVersionDeltas retrieves deltas for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionDeltas(ctx context.Context, versionID string, opts ...AlternativeDistributionPackageDeltasOption) (*AlternativeDistributionPackageDeltasResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	query := &alternativeDistributionPackageDeltasQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s/deltas", versionID)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionPackageDeltas: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildAlternativeDistributionPackageDeltasQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageDeltasResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package deltas response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVersionDeltasRelationships retrieves delta linkages for a package version.
func (c *Client) GetAlternativeDistributionPackageVersionDeltasRelationships(ctx context.Context, versionID string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	return c.getAlternativeDistributionPackageVersionLinkages(ctx, versionID, "deltas", opts...)
}

func (c *Client) getAlternativeDistributionPackageVersionLinkages(ctx context.Context, versionID, relationship string, opts ...LinkagesOption) (*LinkagesResponse, error) {
	versionID = strings.TrimSpace(versionID)
	if versionID == "" {
		return nil, fmt.Errorf("versionID is required")
	}

	query := &linkagesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageVersions/%s/relationships/%s", versionID, relationship)
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("alternativeDistributionPackageVersionRelationships: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildLinkagesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response LinkagesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package version relationships response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageVariant retrieves a package variant by ID.
func (c *Client) GetAlternativeDistributionPackageVariant(ctx context.Context, variantID string) (*AlternativeDistributionPackageVariantResponse, error) {
	variantID = strings.TrimSpace(variantID)
	if variantID == "" {
		return nil, fmt.Errorf("variantID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageVariants/%s", variantID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageVariantResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package variant response: %w", err)
	}

	return &response, nil
}

// GetAlternativeDistributionPackageDelta retrieves a package delta by ID.
func (c *Client) GetAlternativeDistributionPackageDelta(ctx context.Context, deltaID string) (*AlternativeDistributionPackageDeltaResponse, error) {
	deltaID = strings.TrimSpace(deltaID)
	if deltaID == "" {
		return nil, fmt.Errorf("deltaID is required")
	}

	path := fmt.Sprintf("/v1/alternativeDistributionPackageDeltas/%s", deltaID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageDeltaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package delta response: %w", err)
	}

	return &response, nil
}

// GetAppAlternativeDistributionKey retrieves an app's alternative distribution key.
func (c *Client) GetAppAlternativeDistributionKey(ctx context.Context, appID string) (*AlternativeDistributionKeyResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/alternativeDistributionKey", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionKeyResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution key response: %w", err)
	}

	return &response, nil
}

// GetAppAlternativeDistributionKeyRelationship retrieves an app's alternative distribution key relationship.
func (c *Client) GetAppAlternativeDistributionKeyRelationship(ctx context.Context, appID string) (*AppAlternativeDistributionKeyLinkageResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	path := fmt.Sprintf("/v1/apps/%s/relationships/alternativeDistributionKey", appID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppAlternativeDistributionKeyLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution key relationship response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionAlternativeDistributionPackage retrieves the alternative distribution package for an app store version.
func (c *Client) GetAppStoreVersionAlternativeDistributionPackage(ctx context.Context, appStoreVersionID string) (*AlternativeDistributionPackageResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/alternativeDistributionPackage", appStoreVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AlternativeDistributionPackageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package response: %w", err)
	}

	return &response, nil
}

// GetAppStoreVersionAlternativeDistributionPackageRelationship retrieves the package relationship for an app store version.
func (c *Client) GetAppStoreVersionAlternativeDistributionPackageRelationship(ctx context.Context, appStoreVersionID string) (*AppStoreVersionAlternativeDistributionPackageLinkageResponse, error) {
	appStoreVersionID = strings.TrimSpace(appStoreVersionID)
	if appStoreVersionID == "" {
		return nil, fmt.Errorf("appStoreVersionID is required")
	}

	path := fmt.Sprintf("/v1/appStoreVersions/%s/relationships/alternativeDistributionPackage", appStoreVersionID)
	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppStoreVersionAlternativeDistributionPackageLinkageResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse alternative distribution package relationship response: %w", err)
	}

	return &response, nil
}
