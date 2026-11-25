package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/urlsanitize"
)

// GetDSAStatus reads only the selected account's trader and verification states.
func (c *Client) GetDSAStatus(ctx context.Context, accountID string) (*asc.WebDSAStatus, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, errors.New("DSA status requires a selected account")
	}
	path := "/v1/accounts/" + url.PathEscape(accountID) + "/sellerInfo"
	body, err := c.doRequestBase(ctx, appStoreBaseURL, http.MethodGet, path, nil, irisRequestHeaders())
	if err != nil {
		return nil, err
	}
	var response struct {
		AccountID         string                   `json:"accountId"`
		TraderDeclaration *string                  `json:"traderDeclaration"`
		SellerInfo        []asc.WebDSASellerStatus `json:"sellerInfo"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.New("DSA status returned a malformed response")
	}
	if response.TraderDeclaration == nil {
		return nil, errors.New("DSA status response is missing traderDeclaration")
	}
	if response.AccountID != "" && !strings.EqualFold(response.AccountID, accountID) {
		return nil, errors.New("DSA status response identifies another account")
	}
	return &asc.WebDSAStatus{TraderDeclaration: *response.TraderDeclaration, SellerInfo: response.SellerInfo}, nil
}

func isDSASellerInfoRequestURL(value *url.URL) bool {
	return value != nil && strings.HasPrefix(value.Path, "/v1/accounts/") && strings.HasSuffix(value.Path, "/sellerInfo")
}

// DSA transport errors can contain the account URL or private broker details.
// Keep only the classifications used by the CLI and its callers.
func redactedDSAError(ctx context.Context, message string, err error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if broker, ok := errors.AsType[*BrokerError](err); ok {
		return fmt.Errorf("%s: %w", message, &BrokerError{Status: broker.Status})
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
		return &urlsanitize.TransportError{Message: message + " (timeout)", Err: os.ErrDeadlineExceeded}
	}
	return &urlsanitize.TransportError{Message: message}
}
