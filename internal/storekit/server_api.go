package storekit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

// TransactionHistoryQuery holds the optional Get Transaction History v2
// filters. Zero values are omitted from the request.
type TransactionHistoryQuery struct {
	StartDate     time.Time
	EndDate       time.Time
	ProductIDs    []string
	ProductTypes  []string
	OwnershipType string
	Sort          string
	Revoked       *bool
}

func (q TransactionHistoryQuery) values() url.Values {
	values := url.Values{}
	if !q.StartDate.IsZero() {
		values.Set("startDate", strconv.FormatInt(q.StartDate.UnixMilli(), 10))
	}
	if !q.EndDate.IsZero() {
		values.Set("endDate", strconv.FormatInt(q.EndDate.UnixMilli(), 10))
	}
	for _, id := range q.ProductIDs {
		values.Add("productId", id)
	}
	for _, productType := range q.ProductTypes {
		values.Add("productType", productType)
	}
	if q.OwnershipType != "" {
		values.Set("inAppOwnershipType", q.OwnershipType)
	}
	if q.Sort != "" {
		values.Set("sort", q.Sort)
	}
	if q.Revoked != nil {
		values.Set("revoked", strconv.FormatBool(*q.Revoked))
	}
	return values
}

// NotificationHistoryRequest is Apple's NotificationHistoryRequest body.
// Dates are UNIX time in milliseconds.
type NotificationHistoryRequest struct {
	StartDate           int64  `json:"startDate"`
	EndDate             int64  `json:"endDate"`
	NotificationType    string `json:"notificationType,omitempty"`
	NotificationSubtype string `json:"notificationSubtype,omitempty"`
	TransactionID       string `json:"transactionId,omitempty"`
	OnlyFailures        bool   `json:"onlyFailures,omitempty"`
}

// GetTransactionInfo calls Get Transaction Info.
func (c *Client) GetTransactionInfo(ctx context.Context, transactionID string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/transactions/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path, nil)
}

// GetTransactionHistory calls Get Transaction History (v2). revision is empty
// for the first page.
func (c *Client) GetTransactionHistory(ctx context.Context, transactionID string, query TransactionHistoryQuery, revision string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v2/history/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	values := query.values()
	if revision = strings.TrimSpace(revision); revision != "" {
		values.Set("revision", revision)
	}
	return c.getJSON(ctx, path, values)
}

// GetAllSubscriptionStatuses calls Get All Subscription Statuses.
func (c *Client) GetAllSubscriptionStatuses(ctx context.Context, transactionID string, statuses []int) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/subscriptions/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	for _, status := range statuses {
		values.Add("status", strconv.Itoa(status))
	}
	return c.getJSON(ctx, path, values)
}

// GetRefundHistory calls Get Refund History (v2). revision is empty for the
// first page.
func (c *Client) GetRefundHistory(ctx context.Context, transactionID, revision string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v2/refund/lookup/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	if revision = strings.TrimSpace(revision); revision != "" {
		values.Set("revision", revision)
	}
	return c.getJSON(ctx, path, values)
}

// LookUpOrderID calls Look Up Order ID.
func (c *Client) LookUpOrderID(ctx context.Context, orderID string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/lookup/", "order ID", orderID)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path, nil)
}

// GetNotificationHistory calls Get Notification History. Apple transports
// this read as a POST, so it is marked as a read for read-only mode.
func (c *Client) GetNotificationHistory(ctx context.Context, request NotificationHistoryRequest, paginationToken string) (json.RawMessage, error) {
	body, err := jsonBody(request)
	if err != nil {
		return nil, err
	}
	path := "inApps/v1/notifications/history"
	if paginationToken = strings.TrimSpace(paginationToken); paginationToken != "" {
		path += "?" + url.Values{"paginationToken": {paginationToken}}.Encode()
	}
	var response json.RawMessage
	if err := c.request(readonly.WithReadIntent(ctx), http.MethodPost, path, "application/json", body, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// RequestTestNotification calls Request a Test Notification, which asks the
// App Store to send a TEST notification to the configured server URL.
func (c *Client) RequestTestNotification(ctx context.Context) (json.RawMessage, error) {
	var response json.RawMessage
	if err := c.request(ctx, http.MethodPost, "inApps/v1/notifications/test", "", nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// GetTestNotificationStatus calls Get Test Notification Status.
func (c *Client) GetTestNotificationStatus(ctx context.Context, testNotificationToken string) (json.RawMessage, error) {
	path, err := pathWithID("inApps/v1/notifications/test/", "test notification token", testNotificationToken)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path, nil)
}

// GetCustomerGroups calls Get Customer Groups.
func (c *Client) GetCustomerGroups(ctx context.Context, transactionID string) (json.RawMessage, error) {
	path, err := pathWithID("groups/v1/currentGroups/", "transaction ID", transactionID)
	if err != nil {
		return nil, err
	}
	return c.getJSON(ctx, path, nil)
}

// GetGroupMembers calls Get Group Members. A zero limit uses Apple's default.
func (c *Client) GetGroupMembers(ctx context.Context, groupID string, limit int, paginationToken string) (json.RawMessage, error) {
	path, err := pathWithID("groups/v1/group/", "group ID", groupID)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	if limit > 0 {
		values.Set("limit", strconv.Itoa(limit))
	}
	if paginationToken = strings.TrimSpace(paginationToken); paginationToken != "" {
		values.Set("paginationToken", paginationToken)
	}
	return c.getJSON(ctx, path, values)
}

func pathWithID(prefix, label, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return prefix + url.PathEscape(id), nil
}

// getJSON returns Apple's response unmodified as raw JSON so new or
// undocumented fields are never dropped.
func (c *Client) getJSON(ctx context.Context, path string, values url.Values) (json.RawMessage, error) {
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	var response json.RawMessage
	if err := c.request(ctx, http.MethodGet, path, "", nil, &response); err != nil {
		return nil, err
	}
	return response, nil
}

// PageCursor names the item array and continuation token of a paginated App
// Store Server API response that reports hasMore.
type PageCursor struct {
	ItemsKey string
	TokenKey string
}

var (
	// SignedTransactionPages covers Get Transaction History and Get Refund History.
	SignedTransactionPages   = PageCursor{ItemsKey: "signedTransactions", TokenKey: "revision"}
	NotificationHistoryPages = PageCursor{ItemsKey: "notificationHistory", TokenKey: "paginationToken"}
	GroupMemberPages         = PageCursor{ItemsKey: "members", TokenKey: "paginationToken"}
)

// HasMore reports whether a page says more results exist, and its token.
func (p PageCursor) HasMore(page json.RawMessage) (bool, string, error) {
	fields, err := decodeObject(page)
	if err != nil {
		return false, "", err
	}
	return pageState(fields, p.TokenKey)
}

// CollectPages follows hasMore and the continuation token from first, and
// returns the final page's envelope with every page's items concatenated.
// The final page keeps Apple's last token, which callers can store to resume.
func CollectPages(first json.RawMessage, cursor PageCursor, next func(token string) (json.RawMessage, error)) (json.RawMessage, error) {
	page, err := decodeObject(first)
	if err != nil {
		return nil, err
	}
	items, err := pageItems(page, cursor.ItemsKey)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for {
		hasMore, token, err := pageState(page, cursor.TokenKey)
		if err != nil {
			return nil, err
		}
		if !hasMore {
			break
		}
		if token == "" {
			return nil, fmt.Errorf("StoreKit response hasMore is true but %s is empty", cursor.TokenKey)
		}
		if seen[token] {
			return nil, fmt.Errorf("StoreKit pagination returned repeated %s %q", cursor.TokenKey, token)
		}
		seen[token] = true
		raw, err := next(token)
		if err != nil {
			return nil, err
		}
		if page, err = decodeObject(raw); err != nil {
			return nil, err
		}
		pageItemsValue, err := pageItems(page, cursor.ItemsKey)
		if err != nil {
			return nil, err
		}
		items = append(items, pageItemsValue...)
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	encodedItems, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode StoreKit pages: %w", err)
	}
	page[cursor.ItemsKey] = encodedItems
	merged, err := json.Marshal(page)
	if err != nil {
		return nil, fmt.Errorf("encode StoreKit pages: %w", err)
	}
	return merged, nil
}

func decodeObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("decode StoreKit page: expected a JSON object")
	}
	return fields, nil
}

func pageItems(page map[string]json.RawMessage, key string) ([]json.RawMessage, error) {
	raw, ok := page[key]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode StoreKit page %s: %w", key, err)
	}
	return items, nil
}

func pageState(page map[string]json.RawMessage, tokenKey string) (bool, string, error) {
	var hasMore bool
	if raw, ok := page["hasMore"]; ok {
		if err := json.Unmarshal(raw, &hasMore); err != nil {
			return false, "", fmt.Errorf("decode StoreKit page hasMore: %w", err)
		}
	}
	var token string
	if raw, ok := page[tokenKey]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &token); err != nil {
			return false, "", fmt.Errorf("decode StoreKit page %s: %w", tokenKey, err)
		}
	}
	return hasMore, strings.TrimSpace(token), nil
}
