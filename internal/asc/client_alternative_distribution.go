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
