package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
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
	if response.AccountID != "" && response.AccountID != accountID {
		return nil, errors.New("DSA status response identifies another account")
	}
	return &asc.WebDSAStatus{TraderDeclaration: *response.TraderDeclaration, SellerInfo: response.SellerInfo}, nil
}

func isDSASellerInfoRequestURL(value *url.URL) bool {
	return value != nil && strings.HasPrefix(value.Path, "/v1/accounts/") && strings.HasSuffix(value.Path, "/sellerInfo")
}
