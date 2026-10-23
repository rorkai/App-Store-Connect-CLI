package storekit

import (
	"context"
	"encoding/json"
	"net/http"
)

// DeliveryStatus describes fulfillment of an In-App Purchase.
type DeliveryStatus string

const (
	DeliveryDelivered               DeliveryStatus = "DELIVERED"
	DeliveryUndeliveredQualityIssue DeliveryStatus = "UNDELIVERED_QUALITY_ISSUE"
	DeliveryUndeliveredWrongItem    DeliveryStatus = "UNDELIVERED_WRONG_ITEM"
	DeliveryUndeliveredServerOutage DeliveryStatus = "UNDELIVERED_SERVER_OUTAGE"
	DeliveryUndeliveredOther        DeliveryStatus = "UNDELIVERED_OTHER"
)

// RefundPreference supplies a preference; Apple decides the refund outcome.
type RefundPreference string

const (
	RefundDecline       RefundPreference = "DECLINE"
	RefundGrantFull     RefundPreference = "GRANT_FULL"
	RefundGrantProrated RefundPreference = "GRANT_PRORATED"
)

// ConsumptionRequest is Apple's current v2 consumption request. A nil
// ConsumptionPercentage omits it, as required for auto-renewable subscriptions.
type ConsumptionRequest struct {
	CustomerConsented     bool             `json:"customerConsented"`
	DeliveryStatus        DeliveryStatus   `json:"deliveryStatus"`
	SampleContentProvided bool             `json:"sampleContentProvided"`
	ConsumptionPercentage *int             `json:"consumptionPercentage,omitempty"`
	RefundPreference      RefundPreference `json:"refundPreference,omitempty"`
}

// ExtendRenewalDateRequest describes one renewal extension. Reuse the same
// RequestIdentifier when retrying a request with an uncertain outcome.
type ExtendRenewalDateRequest struct {
	ExtendByDays      int    `json:"extendByDays"`
	ExtendReasonCode  int    `json:"extendReasonCode"`
	RequestIdentifier string `json:"requestIdentifier"`
}

// MassExtendRenewalDateRequest extends a product's eligible active subscribers.
// Omitting StorefrontCountryCodes targets all eligible storefronts.
type MassExtendRenewalDateRequest struct {
	ExtendRenewalDateRequest
	ProductID              string   `json:"productId"`
	StorefrontCountryCodes []string `json:"storefrontCountryCodes,omitempty"`
}

// GetAppTransactionInfo returns Apple's signed app-purchase information.
func (c *Client) GetAppTransactionInfo(ctx context.Context, transactionID string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/transactions/appTransactions/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path, nil)
}

// SetAppAccountToken replaces the account token on the original transaction.
func (c *Client) SetAppAccountToken(ctx context.Context, originalTransactionID, appAccountToken string) error {
	path, err := pathWithID("inApps/v1/transactions/", "original transaction ID", originalTransactionID)
	if err != nil {
		return err
	}
	return c.sendJSON(ctx, http.MethodPut, path+"/appAccountToken", struct {
		AppAccountToken string `json:"appAccountToken"`
	}{appAccountToken}, nil)
}

// SendConsumptionInformation sends v2 consumption information after a
// CONSUMPTION_REQUEST notification. Callers must have the customer's consent.
func (c *Client) SendConsumptionInformation(ctx context.Context, transactionID string, request ConsumptionRequest) error {
	path, err := pathWithID("inApps/v2/transactions/consumption/", "transaction ID", transactionID)
	if err != nil {
		return err
	}
	return c.sendJSON(ctx, http.MethodPut, path, request, nil)
}

// FinishTransaction acknowledges delivery. It does not grant an entitlement.
func (c *Client) FinishTransaction(ctx context.Context, transactionID string) error {
	path, err := pathWithID("inApps/v1/transactions/", "transaction ID", transactionID)
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodPost, path+"/finish", "", nil, nil)
}

// ExtendSubscriptionRenewalDate returns Apple's success flag and new date.
func (c *Client) ExtendSubscriptionRenewalDate(ctx context.Context, originalTransactionID string, request ExtendRenewalDateRequest) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/subscriptions/extend/", "original transaction ID", originalTransactionID)
	if err != nil {
		return nil, err
	}
	var response json.RawMessage
	err = c.sendJSON(ctx, http.MethodPut, path, request, &response)
	return response, err
}

// ExtendRenewalDateForAllActiveSubscribers submits an asynchronous operation.
func (c *Client) ExtendRenewalDateForAllActiveSubscribers(ctx context.Context, request MassExtendRenewalDateRequest) (json.RawMessage, error) {
	var response json.RawMessage
	err := c.sendJSON(ctx, http.MethodPost, "inApps/v1/subscriptions/extend/mass", request, &response)
	return response, err
}

// GetStatusOfSubscriptionRenewalDateExtensions returns completion and final
// counts. Detailed outcomes arrive through RENEWAL_EXTENSION notifications.
func (c *Client) GetStatusOfSubscriptionRenewalDateExtensions(ctx context.Context, productID, requestID string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/subscriptions/extend/mass/", "product ID", productID)
	if err != nil {
		return nil, err
	}
	suffix, err := pathWithID("", "request ID", requestID)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path+"/"+suffix, nil)
}

// sendJSON sends value as a JSON body. A nil response discards the response
// body, so callers that need no body do not fail on an empty or non-JSON one.
func (c *Client) sendJSON(ctx context.Context, method, path string, value, response any) error {
	body, err := jsonBody(value)
	if err != nil {
		return err
	}
	return c.request(ctx, method, path, "application/json", body, response)
}
