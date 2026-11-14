package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// BetaLicenseAgreementRelationships describes beta license agreement relationships.
type BetaLicenseAgreementRelationships struct {
	App *Relationship `json:"app,omitempty"`
}

// BetaLicenseAgreementResource represents a beta license agreement resource.
type BetaLicenseAgreementResource struct {
	Type          ResourceType                       `json:"type"`
	ID            string                             `json:"id"`
	Attributes    BetaLicenseAgreementAttributes     `json:"attributes"`
	Relationships *BetaLicenseAgreementRelationships `json:"relationships,omitempty"`
}

// BetaLicenseAgreementsResponse is the response from beta license agreement endpoints (list).
type BetaLicenseAgreementsResponse struct {
	Data     []BetaLicenseAgreementResource `json:"data"`
	Links    Links                          `json:"links"`
	Included json.RawMessage                `json:"included,omitempty"`
	Meta     json.RawMessage                `json:"meta,omitempty"`
}

// BetaLicenseAgreementResponse is the response from beta license agreement endpoints (single).
type BetaLicenseAgreementResponse struct {
	Data     BetaLicenseAgreementResource `json:"data"`
	Links    Links                        `json:"links"`
	Included json.RawMessage              `json:"included,omitempty"`
}

// GetBetaLicenseAgreementForApp retrieves a beta license agreement for an app.
func (c *Client) GetBetaLicenseAgreementForApp(ctx context.Context, appID string, fields []string) (*BetaLicenseAgreementResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	values := url.Values{}
	addCSV(values, "fields[betaLicenseAgreements]", normalizeList(fields))
	return getJSON[BetaLicenseAgreementResponse](ctx, c, readRequest{
		Path:  fmt.Sprintf("/v1/apps/%s/betaLicenseAgreement", appID),
		Query: values.Encode(),
	})
}

// UpdateBetaLicenseAgreement updates a beta license agreement by ID.
func (c *Client) UpdateBetaLicenseAgreement(ctx context.Context, agreementID string, agreementText *string) (*BetaLicenseAgreementResponse, error) {
	agreementID = strings.TrimSpace(agreementID)
	if agreementID == "" {
		return nil, fmt.Errorf("agreementID is required")
	}
	if agreementText == nil {
		return nil, fmt.Errorf("agreementText is required")
	}
	trimmed := strings.TrimSpace(*agreementText)
	if trimmed == "" {
		return nil, fmt.Errorf("agreementText is required")
	}

	payload := BetaLicenseAgreementUpdateRequest{
		Data: BetaLicenseAgreementUpdateData{
			Type: ResourceTypeBetaLicenseAgreements,
			ID:   agreementID,
			Attributes: &BetaLicenseAgreementUpdateAttributes{
				AgreementText: &trimmed,
			},
		},
	}

	return sendJSON[BetaLicenseAgreementResponse](ctx, c, writeRequest{Method: "PATCH", Path: fmt.Sprintf("/v1/betaLicenseAgreements/%s", agreementID), Payload: payload})
}

// GetLinks returns the pagination links for beta license agreements.
func (r *BetaLicenseAgreementsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for pagination aggregation.
func (r *BetaLicenseAgreementsResponse) GetData() any {
	return r.Data
}
