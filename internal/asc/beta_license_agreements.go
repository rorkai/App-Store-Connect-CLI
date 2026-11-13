package asc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// BetaLicenseAgreementAttributes describes a beta license agreement.
type BetaLicenseAgreementAttributes struct {
	AgreementText string `json:"agreementText,omitempty"`
}

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

// BetaLicenseAgreementUpdateAttributes describes fields for updating a beta license agreement.
type BetaLicenseAgreementUpdateAttributes struct {
	AgreementText *string `json:"agreementText,omitempty"`
}

// BetaLicenseAgreementUpdateData is the data portion of a beta license agreement update request.
type BetaLicenseAgreementUpdateData struct {
	Type       ResourceType                          `json:"type"`
	ID         string                                `json:"id"`
	Attributes *BetaLicenseAgreementUpdateAttributes `json:"attributes,omitempty"`
}

// BetaLicenseAgreementUpdateRequest is a request to update a beta license agreement.
type BetaLicenseAgreementUpdateRequest struct {
	Data BetaLicenseAgreementUpdateData `json:"data"`
}

// GetBetaLicenseAgreements retrieves beta license agreements with optional filters.
func (c *Client) GetBetaLicenseAgreements(ctx context.Context, opts ...BetaLicenseAgreementsOption) (*BetaLicenseAgreementsResponse, error) {
	query := &betaLicenseAgreementsQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := "/v1/betaLicenseAgreements"
	if query.nextURL != "" {
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("betaLicenseAgreements: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildBetaLicenseAgreementsQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaLicenseAgreementsResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaLicenseAgreement retrieves a beta license agreement by ID.
func (c *Client) GetBetaLicenseAgreement(ctx context.Context, agreementID string, opts ...BetaLicenseAgreementOption) (*BetaLicenseAgreementResponse, error) {
	agreementID = strings.TrimSpace(agreementID)
	if agreementID == "" {
		return nil, fmt.Errorf("agreementID is required")
	}

	query := &betaLicenseAgreementQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/betaLicenseAgreements/%s", agreementID)
	if queryString := buildBetaLicenseAgreementQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaLicenseAgreementForApp retrieves a beta license agreement for an app.
func (c *Client) GetBetaLicenseAgreementForApp(ctx context.Context, appID string, fields []string) (*BetaLicenseAgreementResponse, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	values := url.Values{}
	addCSV(values, "fields[betaLicenseAgreements]", normalizeList(fields))

	path := fmt.Sprintf("/v1/apps/%s/betaLicenseAgreement", appID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response BetaLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetBetaLicenseAgreementApp retrieves the app for a beta license agreement.
func (c *Client) GetBetaLicenseAgreementApp(ctx context.Context, agreementID string, fields []string) (*AppResponse, error) {
	agreementID = strings.TrimSpace(agreementID)
	if agreementID == "" {
		return nil, fmt.Errorf("agreementID is required")
	}

	values := url.Values{}
	addCSV(values, "fields[apps]", normalizeList(fields))

	path := fmt.Sprintf("/v1/betaLicenseAgreements/%s/app", agreementID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response AppResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
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

	body, err := BuildRequestBody(payload)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/v1/betaLicenseAgreements/%s", agreementID)
	data, err := c.do(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}

	var response BetaLicenseAgreementResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetLinks returns the pagination links for beta license agreements.
func (r *BetaLicenseAgreementsResponse) GetLinks() *Links {
	return &r.Links
}

// GetData returns the data field for pagination aggregation.
func (r *BetaLicenseAgreementsResponse) GetData() any {
	return r.Data
}
