package asc

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// SubscriptionOfferCodeOneTimeUseCodeAttributes describes a one-time use offer code batch.
type SubscriptionOfferCodeOneTimeUseCodeAttributes struct {
	NumberOfCodes  int    `json:"numberOfCodes,omitempty"`
	CreatedDate    string `json:"createdDate,omitempty"`
	ExpirationDate string `json:"expirationDate,omitempty"`
	Active         bool   `json:"active,omitempty"`
}

// SubscriptionOfferCodeOneTimeUseCodesResponse is the response from list endpoints.
type SubscriptionOfferCodeOneTimeUseCodesResponse = Response[SubscriptionOfferCodeOneTimeUseCodeAttributes]

// SubscriptionOfferCodeOneTimeUseCodeResponse is the response from detail endpoints.
type SubscriptionOfferCodeOneTimeUseCodeResponse = SingleResponse[SubscriptionOfferCodeOneTimeUseCodeAttributes]

// OfferCodeValuesResult represents parsed offer code values output.
type OfferCodeValuesResult struct {
	Codes []string `json:"codes"`
}

// SubscriptionOfferCodeOneTimeUseCodeCreateAttributes describes attributes for generating offer codes.
type SubscriptionOfferCodeOneTimeUseCodeCreateAttributes struct {
	NumberOfCodes  int    `json:"numberOfCodes"`
	ExpirationDate string `json:"expirationDate"`
}

// SubscriptionOfferCodeOneTimeUseCodeCreateRelationships describes relationships for offer code creation.
type SubscriptionOfferCodeOneTimeUseCodeCreateRelationships struct {
	OfferCode Relationship `json:"offerCode"`
}

// SubscriptionOfferCodeOneTimeUseCodeCreateData is the data portion of a create request.
type SubscriptionOfferCodeOneTimeUseCodeCreateData struct {
	Type          ResourceType                                           `json:"type"`
	Attributes    SubscriptionOfferCodeOneTimeUseCodeCreateAttributes    `json:"attributes"`
	Relationships SubscriptionOfferCodeOneTimeUseCodeCreateRelationships `json:"relationships"`
}

// SubscriptionOfferCodeOneTimeUseCodeCreateRequest is a request to generate offer codes.
type SubscriptionOfferCodeOneTimeUseCodeCreateRequest struct {
	Data SubscriptionOfferCodeOneTimeUseCodeCreateData `json:"data"`
}

// GetSubscriptionOfferCodeOneTimeUseCodes retrieves one-time use offer code batches for an offer code.
func (c *Client) GetSubscriptionOfferCodeOneTimeUseCodes(ctx context.Context, offerCodeID string, opts ...SubscriptionOfferCodeOneTimeUseCodesOption) (*SubscriptionOfferCodeOneTimeUseCodesResponse, error) {
	offerCodeID = strings.TrimSpace(offerCodeID)
	query := &subscriptionOfferCodeOneTimeUseCodesQuery{}
	for _, opt := range opts {
		opt(query)
	}

	path := fmt.Sprintf("/v1/subscriptionOfferCodes/%s/oneTimeUseCodes", offerCodeID)
	if query.nextURL != "" {
		// Validate nextURL to prevent credential exfiltration
		if err := validateNextURL(query.nextURL); err != nil {
			return nil, fmt.Errorf("offer codes: %w", err)
		}
		path = query.nextURL
	} else if queryString := buildSubscriptionOfferCodeOneTimeUseCodesQuery(query); queryString != "" {
		path += "?" + queryString
	}

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeOneTimeUseCodesResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionOfferCodeOneTimeUseCode retrieves a one-time use offer code batch by ID.
func (c *Client) GetSubscriptionOfferCodeOneTimeUseCode(ctx context.Context, oneTimeUseCodeID string) (*SubscriptionOfferCodeOneTimeUseCodeResponse, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	path := fmt.Sprintf("/v1/subscriptionOfferCodeOneTimeUseCodes/%s", oneTimeUseCodeID)

	data, err := c.do(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeOneTimeUseCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// CreateSubscriptionOfferCodeOneTimeUseCode generates a new one-time use offer code batch.
func (c *Client) CreateSubscriptionOfferCodeOneTimeUseCode(ctx context.Context, req SubscriptionOfferCodeOneTimeUseCodeCreateRequest) (*SubscriptionOfferCodeOneTimeUseCodeResponse, error) {
	body, err := BuildRequestBody(req)
	if err != nil {
		return nil, err
	}

	data, err := c.do(ctx, "POST", "/v1/subscriptionOfferCodeOneTimeUseCodes", body)
	if err != nil {
		return nil, err
	}

	var response SubscriptionOfferCodeOneTimeUseCodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response, nil
}

// GetSubscriptionOfferCodeOneTimeUseCodeValues retrieves offer code values as a list of codes.
func (c *Client) GetSubscriptionOfferCodeOneTimeUseCodeValues(ctx context.Context, oneTimeUseCodeID string) ([]string, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	path := fmt.Sprintf("/v1/subscriptionOfferCodeOneTimeUseCodes/%s/values", oneTimeUseCodeID)

	resp, err := c.doStream(ctx, "GET", path, nil, "text/csv")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseSubscriptionOfferCodeOneTimeUseCodeValues(resp.Body)
}

// GetInAppPurchaseOfferCodeOneTimeUseCodeValues retrieves offer code values as a list of codes.
func (c *Client) GetInAppPurchaseOfferCodeOneTimeUseCodeValues(ctx context.Context, oneTimeUseCodeID string) ([]string, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	path := fmt.Sprintf("/v1/inAppPurchaseOfferCodeOneTimeUseCodes/%s/values", oneTimeUseCodeID)

	resp, err := c.doStream(ctx, "GET", path, nil, "text/csv")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return parseSubscriptionOfferCodeOneTimeUseCodeValues(resp.Body)
}

func parseSubscriptionOfferCodeOneTimeUseCodeValues(reader io.Reader) ([]string, error) {
	csvReader := csv.NewReader(reader)
	csvReader.FieldsPerRecord = -1
	csvReader.LazyQuotes = true

	var codes []string
	headerChecked := false
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) == 0 {
			continue
		}
		if !headerChecked {
			headerChecked = true
			if strings.EqualFold(strings.TrimSpace(record[0]), "code") {
				continue
			}
		}
		code := strings.TrimSpace(record[0])
		if code == "" {
			continue
		}
		codes = append(codes, code)
	}

	return codes, nil
}
