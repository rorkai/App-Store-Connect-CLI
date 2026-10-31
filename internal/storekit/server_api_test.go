package storekit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestServerAPIEndpoints(t *testing.T) {
	revoked := false
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		method   string
		path     string
		query    url.Values
		body     string
		response string
		call     func(context.Context, *Client) (json.RawMessage, error)
	}{
		{
			name: "transaction info", method: http.MethodGet, path: "/inApps/v1/transactions/2000000000000001",
			response: `{"signedTransactionInfo":"a.b.c"}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetTransactionInfo(ctx, "2000000000000001")
			},
		},
		{
			name: "transaction history first page", method: http.MethodGet, path: "/inApps/v2/history/2000000000000001",
			response: `{"revision":"rev-1","hasMore":false,"signedTransactions":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetTransactionHistory(ctx, "2000000000000001", TransactionHistoryQuery{}, "")
			},
		},
		{
			name: "transaction history filters", method: http.MethodGet, path: "/inApps/v2/history/2000000000000001",
			query: url.Values{
				"revision":           {"rev-1"},
				"startDate":          {"1788220800000"},
				"endDate":            {"1788307200000"},
				"productId":          {"com.example.monthly", "com.example.annual"},
				"productType":        {"AUTO_RENEWABLE", "CONSUMABLE"},
				"inAppOwnershipType": {"ASSIGNED"},
				"sort":               {"DESCENDING"},
				"revoked":            {"false"},
			},
			response: `{"revision":"rev-2","hasMore":false,"signedTransactions":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetTransactionHistory(ctx, "2000000000000001", TransactionHistoryQuery{
					StartDate:     start,
					EndDate:       end,
					ProductIDs:    []string{"com.example.monthly", "com.example.annual"},
					ProductTypes:  []string{"AUTO_RENEWABLE", "CONSUMABLE"},
					OwnershipType: "ASSIGNED",
					Sort:          "DESCENDING",
					Revoked:       &revoked,
				}, "rev-1")
			},
		},
		{
			name: "subscription statuses", method: http.MethodGet, path: "/inApps/v1/subscriptions/2000000000000001",
			query:    url.Values{"status": {"1", "4"}},
			response: `{"environment":"Sandbox","data":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetAllSubscriptionStatuses(ctx, "2000000000000001", []int{1, 4})
			},
		},
		{
			name: "refund history", method: http.MethodGet, path: "/inApps/v2/refund/lookup/2000000000000001",
			query:    url.Values{"revision": {"rev-1"}},
			response: `{"revision":"rev-2","hasMore":false,"signedTransactions":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetRefundHistory(ctx, "2000000000000001", "rev-1")
			},
		},
		{
			name: "order lookup", method: http.MethodGet, path: "/inApps/v1/lookup/MQXYZ12345",
			response: `{"status":0,"signedTransactions":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.LookUpOrderID(ctx, "MQXYZ12345")
			},
		},
		{
			name: "notification history", method: http.MethodPost, path: "/inApps/v1/notifications/history",
			query:    url.Values{"paginationToken": {"token-1"}},
			body:     `{"startDate":1788220800000,"endDate":1788307200000,"notificationType":"SUBSCRIBED","notificationSubtype":"INITIAL_BUY","onlyFailures":true}`,
			response: `{"notificationHistory":[],"hasMore":false}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetNotificationHistory(ctx, NotificationHistoryRequest{
					StartDate:           start.UnixMilli(),
					EndDate:             end.UnixMilli(),
					NotificationType:    "SUBSCRIBED",
					NotificationSubtype: "INITIAL_BUY",
					OnlyFailures:        true,
				}, "token-1")
			},
		},
		{
			name: "notification history by transaction", method: http.MethodPost, path: "/inApps/v1/notifications/history",
			body:     `{"startDate":1788220800000,"endDate":1788307200000,"transactionId":"2000000000000001"}`,
			response: `{"notificationHistory":[],"hasMore":false}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetNotificationHistory(ctx, NotificationHistoryRequest{
					StartDate:     start.UnixMilli(),
					EndDate:       end.UnixMilli(),
					TransactionID: "2000000000000001",
				}, "")
			},
		},
		{
			name: "request test notification", method: http.MethodPost, path: "/inApps/v1/notifications/test",
			response: `{"testNotificationToken":"token-1"}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.RequestTestNotification(ctx)
			},
		},
		{
			name: "test notification status", method: http.MethodGet, path: "/inApps/v1/notifications/test/token-1",
			response: `{"signedPayload":"a.b.c","sendAttempts":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetTestNotificationStatus(ctx, "token-1")
			},
		},
		{
			name: "customer groups", method: http.MethodGet, path: "/groups/v1/currentGroups/2000000000000001",
			response: `{"groups":[]}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetCustomerGroups(ctx, "2000000000000001")
			},
		},
		{
			name: "group members", method: http.MethodGet, path: "/groups/v1/group/900000000000000000",
			query:    url.Values{"limit": {"50"}, "paginationToken": {"token-1"}},
			response: `{"members":[],"hasMore":false}`,
			call: func(ctx context.Context, c *Client) (json.RawMessage, error) {
				return c.GetGroupMembers(ctx, "900000000000000000", 50, "token-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("method = %s, want %s", r.Method, tt.method)
				}
				if r.URL.Path != tt.path {
					t.Errorf("path = %s, want %s", r.URL.Path, tt.path)
				}
				wantQuery := tt.query
				if wantQuery == nil {
					wantQuery = url.Values{}
				}
				if got := r.URL.Query(); !reflect.DeepEqual(got, wantQuery) {
					t.Errorf("query = %v, want %v", got, wantQuery)
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("read body: %v", err)
				}
				if tt.body == "" {
					if len(body) != 0 {
						t.Errorf("body = %q, want empty", body)
					}
				} else {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
					}
					assertJSONEqual(t, string(body), tt.body)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()

			client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			got, err := tt.call(context.Background(), client)
			if err != nil {
				t.Fatalf("call error = %v", err)
			}
			assertJSONEqual(t, string(got), tt.response)
		})
	}
}

func TestReadOnlyModeAllowsNotificationHistoryButRefusesTestNotification(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")
	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"notificationHistory":[],"hasMore":false}`)
	}))
	defer server.Close()
	client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if _, err := client.RequestTestNotification(context.Background()); !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("RequestTestNotification() error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("server received %d requests, want 0", got)
	}
	if _, err := client.GetNotificationHistory(context.Background(), NotificationHistoryRequest{StartDate: 1, EndDate: 2}, ""); err != nil {
		t.Fatalf("GetNotificationHistory() error = %v", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests, want 1", got)
	}
}

func TestCollectPagesMergesItemsAndKeepsFinalCursor(t *testing.T) {
	first := json.RawMessage(`{"bundleId":"com.example.app","revision":"rev-1","hasMore":true,"signedTransactions":["a"]}`)
	pages := map[string]string{
		"rev-1": `{"bundleId":"com.example.app","revision":"rev-2","hasMore":true,"signedTransactions":["b","c"]}`,
		"rev-2": `{"bundleId":"com.example.app","revision":"rev-3","hasMore":false,"signedTransactions":[]}`,
	}
	var requested []string
	merged, err := CollectPages(first, SignedTransactionPages, func(cursor string) (json.RawMessage, error) {
		requested = append(requested, cursor)
		return json.RawMessage(pages[cursor]), nil
	})
	if err != nil {
		t.Fatalf("CollectPages() error = %v", err)
	}
	if !reflect.DeepEqual(requested, []string{"rev-1", "rev-2"}) {
		t.Fatalf("requested cursors = %v", requested)
	}
	assertJSONEqual(t, string(merged), `{"bundleId":"com.example.app","revision":"rev-3","hasMore":false,"signedTransactions":["a","b","c"]}`)
}

func TestCollectPagesRejectsRepeatedOrMissingCursor(t *testing.T) {
	repeat := json.RawMessage(`{"paginationToken":"t-1","hasMore":true,"members":[]}`)
	_, err := CollectPages(repeat, GroupMemberPages, func(string) (json.RawMessage, error) { return repeat, nil })
	if err == nil || !strings.Contains(err.Error(), `repeated paginationToken "t-1"`) {
		t.Fatalf("repeated cursor error = %v", err)
	}
	missing := json.RawMessage(`{"hasMore":true,"notificationHistory":[]}`)
	_, err = CollectPages(missing, NotificationHistoryPages, func(string) (json.RawMessage, error) { return missing, nil })
	if err == nil || !strings.Contains(err.Error(), "hasMore is true but paginationToken is empty") {
		t.Fatalf("missing cursor error = %v", err)
	}
}

func TestAddDecodedJWSPayloadsAddsSiblingsWithoutChangingSignedFields(t *testing.T) {
	transaction := testJWS(t, `{"transactionId":"2000000000000001","productId":"com.example.monthly","signedDate":1788220800000}`)
	renewal := testJWS(t, `{"autoRenewStatus":1}`)
	notification := testJWS(t, `{"notificationType":"TEST","data":{"signedTransactionInfo":"`+transaction+`"}}`)
	raw := json.RawMessage(`{"signedTransactions":["` + transaction + `"],"data":[{"lastTransactions":[{"status":1,"signedTransactionInfo":"` + transaction + `","signedRenewalInfo":"` + renewal + `"}]}],"signedPayload":"` + notification + `"}`)

	got, err := AddDecodedJWSPayloads(raw)
	if err != nil {
		t.Fatalf("AddDecodedJWSPayloads() error = %v", err)
	}
	transactionJSON := `{"transactionId":"2000000000000001","productId":"com.example.monthly","signedDate":1788220800000}`
	assertJSONEqual(t, string(got), `{
		"signedTransactions":["`+transaction+`"],
		"signedTransactionsDecoded":[`+transactionJSON+`],
		"data":[{"lastTransactions":[{
			"status":1,
			"signedTransactionInfo":"`+transaction+`",
			"signedTransactionInfoDecoded":`+transactionJSON+`,
			"signedRenewalInfo":"`+renewal+`",
			"signedRenewalInfoDecoded":{"autoRenewStatus":1}
		}]}],
		"signedPayload":"`+notification+`",
		"signedPayloadDecoded":{"notificationType":"TEST","data":{
			"signedTransactionInfo":"`+transaction+`",
			"signedTransactionInfoDecoded":`+transactionJSON+`
		}}
	}`)
}

func TestDecodeJWSPayloadUnverifiedRejectsMalformedTokens(t *testing.T) {
	for _, token := range []string{"", "a.b", "a.!!!.c", "a." + base64.RawURLEncoding.EncodeToString([]byte("not json")) + ".c"} {
		if _, err := DecodeJWSPayloadUnverified(token); err == nil {
			t.Errorf("DecodeJWSPayloadUnverified(%q) error = nil", token)
		}
	}
}

func testJWS(t *testing.T, payload string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256","x5c":[]}`))
	return header + "." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".c2lnbmF0dXJl"
}

func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("invalid expected JSON %q: %v", want, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON = %s\nwant %s", got, want)
	}
}
