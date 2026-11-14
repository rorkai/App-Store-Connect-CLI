package asc

import (
	"context"
	"encoding/csv"
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
	Environment    string `json:"environment,omitempty"`
}

// SubscriptionOfferCodeOneTimeUseCodesResponse is the response from list endpoints.
type SubscriptionOfferCodeOneTimeUseCodesResponse = Response[SubscriptionOfferCodeOneTimeUseCodeAttributes]

// OfferCodeValuesResult represents parsed offer code values output.
type OfferCodeValuesResult struct {
	Codes []string `json:"codes"`
}

// SubscriptionOfferCodeOneTimeUseCodeCreateAttributes describes attributes for generating offer codes.
type SubscriptionOfferCodeOneTimeUseCodeCreateAttributes struct {
	NumberOfCodes  int    `json:"numberOfCodes"`
	ExpirationDate string `json:"expirationDate"`
	Environment    string `json:"environment,omitempty"`
}

// CreateSubscriptionOfferCodeOneTimeUseCode generates a new one-time use offer code batch.
func (c *Client) CreateSubscriptionOfferCodeOneTimeUseCode(ctx context.Context, req SubscriptionOfferCodeOneTimeUseCodeCreateRequest) (*SubscriptionOfferCodeOneTimeUseCodeResponse, error) {
	return sendJSON[SubscriptionOfferCodeOneTimeUseCodeResponse](ctx, c, writeRequest{Method: "POST", Path: "/v1/subscriptionOfferCodeOneTimeUseCodes", Payload: req})
}

// GetSubscriptionOfferCodeOneTimeUseCodeValues retrieves offer code values as a list of codes.
func (c *Client) GetSubscriptionOfferCodeOneTimeUseCodeValues(ctx context.Context, oneTimeUseCodeID string) ([]string, error) {
	oneTimeUseCodeID = strings.TrimSpace(oneTimeUseCodeID)
	path := fmt.Sprintf("/v1/subscriptionOfferCodeOneTimeUseCodes/%s/values", oneTimeUseCodeID)

	resp, err := c.doStream(ctx, path, "text/csv")
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

	resp, err := c.doStream(ctx, path, "text/csv")
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
