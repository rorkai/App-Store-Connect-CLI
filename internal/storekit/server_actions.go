package storekit

import (
	"context"
	"encoding/json"
	"net/http"
)

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
	_, err = c.sendJSON(ctx, http.MethodPut, path+"/appAccountToken", struct {
		AppAccountToken string `json:"appAccountToken"`
	}{appAccountToken})
	return err
}

// sendJSON sends value as a JSON body. No-content responses remain nil.
func (c *Client) sendJSON(ctx context.Context, method, path string, value any) (json.RawMessage, error) {
	body, err := jsonBody(value)
	if err != nil {
		return nil, err
	}
	var response json.RawMessage
	if err := c.request(ctx, method, path, "application/json", body, &response); err != nil {
		return nil, err
	}
	return response, nil
}
