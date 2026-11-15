package asc

import (
	"context"
)

// GetMerchantIDCertificatesRelationships retrieves certificate linkages for a merchant ID.
func (c *Client) GetMerchantIDCertificatesRelationships(ctx context.Context, merchantID string, opts ...LinkagesOption) (*MerchantIDCertificatesLinkagesResponse, error) {
	return getTypedResourceLinkages[MerchantIDCertificatesLinkagesResponse](
		c,
		ctx,
		merchantID,
		"certificates",
		"merchant ID",
		"/v1/merchantIds/%s/relationships/%s",
		"merchantIdCertificatesRelationships",
		opts...,
	)
}
