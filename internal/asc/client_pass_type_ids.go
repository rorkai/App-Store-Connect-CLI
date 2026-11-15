package asc

import (
	"context"
)

// GetPassTypeIDCertificatesRelationships retrieves certificate linkages for a pass type ID.
func (c *Client) GetPassTypeIDCertificatesRelationships(ctx context.Context, passTypeID string, opts ...LinkagesOption) (*PassTypeIDCertificatesLinkagesResponse, error) {
	return getTypedResourceLinkages[PassTypeIDCertificatesLinkagesResponse](
		c,
		ctx,
		passTypeID,
		"certificates",
		"pass type ID",
		"/v1/passTypeIds/%s/relationships/%s",
		"passTypeIdCertificatesRelationships",
		opts...,
	)
}
