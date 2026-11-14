package asc

import (
	"context"
	"fmt"
	"strings"
)

// GetPassTypeIDs retrieves the list of pass type IDs.
func (c *Client) GetPassTypeIDs(ctx context.Context, opts ...PassTypeIDsOption) (*PassTypeIDsResponse, error) {
	query := applyOptions(opts)
	return getJSON[PassTypeIDsResponse](ctx, c, readRequest{
		Path:        "/v1/passTypeIds",
		Query:       buildPassTypeIDsQuery(query),
		Next:        query.nextURL,
		NextContext: "passTypeIds",
	})
}

// GetPassTypeID retrieves a single pass type ID by ID.
func (c *Client) GetPassTypeID(ctx context.Context, id string, opts ...PassTypeIDOption) (*PassTypeIDResponse, error) {
	id = strings.TrimSpace(id)
	query := applyOptions(opts)
	return getJSON[PassTypeIDResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/passTypeIds/%s", id), Query: buildPassTypeIDQuery(query)})
}

// CreatePassTypeID creates a new pass type ID.
func (c *Client) CreatePassTypeID(ctx context.Context, attrs PassTypeIDCreateAttributes) (*PassTypeIDResponse, error) {
	request := PassTypeIDCreateRequest{
		Data: PassTypeIDCreateData{
			Type:       ResourceTypePassTypeIds,
			Attributes: attrs,
		},
	}
	return sendJSON[PassTypeIDResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/passTypeIds", Payload: request})
}

// UpdatePassTypeID updates an existing pass type ID.
func (c *Client) UpdatePassTypeID(ctx context.Context, id string, attrs PassTypeIDUpdateAttributes) (*PassTypeIDResponse, error) {
	id = strings.TrimSpace(id)
	request := PassTypeIDUpdateRequest{
		Data: PassTypeIDUpdateData{
			Type:       ResourceTypePassTypeIds,
			ID:         id,
			Attributes: &attrs,
		},
	}
	return sendJSON[PassTypeIDResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/passTypeIds/%s", id), Payload: request})
}

// DeletePassTypeID deletes a pass type ID by ID.
func (c *Client) DeletePassTypeID(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	return sendNoContent(ctx, c, "DELETE", fmt.Sprintf("/v1/passTypeIds/%s", id), nil)
}

// GetPassTypeIDCertificates retrieves certificates for a pass type ID.
func (c *Client) GetPassTypeIDCertificates(ctx context.Context, passTypeID string, opts ...PassTypeIDCertificatesOption) (*CertificatesResponse, error) {
	passTypeID = strings.TrimSpace(passTypeID)
	query := applyOptions(opts)
	return getJSON[CertificatesResponse](ctx, c, readRequest{
		Path:        fmt.Sprintf("/v1/passTypeIds/%s/certificates", passTypeID),
		Query:       buildPassTypeIDCertificatesQuery(query),
		Next:        query.nextURL,
		NextContext: "passTypeIdCertificates",
	})
}

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

// GetCertificatePassTypeID retrieves the pass type ID for a certificate.
func (c *Client) GetCertificatePassTypeID(ctx context.Context, certificateID string) (*PassTypeIDResponse, error) {
	certificateID = strings.TrimSpace(certificateID)
	return getJSON[PassTypeIDResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/certificates/%s/passTypeId", certificateID)})
}

// GetCertificatePassTypeIDRelationship retrieves the pass type ID linkage for a certificate.
func (c *Client) GetCertificatePassTypeIDRelationship(ctx context.Context, certificateID string) (*CertificatePassTypeIDLinkageResponse, error) {
	certificateID = strings.TrimSpace(certificateID)
	return getJSON[CertificatePassTypeIDLinkageResponse](ctx, c, readRequest{Path: fmt.Sprintf("/v1/certificates/%s/relationships/passTypeId", certificateID)})
}
