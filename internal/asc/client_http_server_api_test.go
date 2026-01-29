package asc

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"
)

func newServerAPITestClient(t *testing.T, check func(*http.Request), response *http.Response) *ServerAPIClient {
	t.Helper()

	key, err := generateTestPrivateKey(t)
	if err != nil {
		t.Fatalf("generate key error: %v", err)
	}

	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if check != nil {
			check(req)
		}
		return response, nil
	})

	return &ServerAPIClient{
		httpClient: &http.Client{Transport: transport},
		keyID:      "KEY123",
		issuerID:   "ISS456",
		bundleID:   "com.example.app",
		privateKey: key,
		baseURL:    "https://api.storekit.itunes.apple.com",
	}
}

func generateTestPrivateKey(t *testing.T) (*ecdsa.PrivateKey, error) {
	t.Helper()
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func TestGetAllSubscriptionStatuses_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"data":[]}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/subscriptions/tx-1" {
			t.Fatalf("expected path /inApps/v1/subscriptions/tx-1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("status") == "" {
			t.Fatalf("expected status query params")
		}
		assertAuthorized(t, req)
	}, response)

	_, err := client.GetAllSubscriptionStatuses(context.Background(), "tx-1", []ServerStatus{ServerStatusActive, ServerStatusExpired})
	if err != nil {
		t.Fatalf("GetAllSubscriptionStatuses() error: %v", err)
	}
}

func TestGetAllSubscriptionStatuses_EmptyTransactionID(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"data":[]}`))
	if _, err := client.GetAllSubscriptionStatuses(context.Background(), "", nil); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetTransactionHistory_SendsRequest(t *testing.T) {
	start := int64(1700000000000)
	end := int64(1700100000000)
	revoked := true
	ownership := ServerInAppOwnershipTypePurchased
	order := ServerOrderDescending

	request := TransactionHistoryRequest{
		StartDate:                    &start,
		EndDate:                      &end,
		ProductIDs:                   []string{"prod-1", "prod-2"},
		ProductTypes:                 []ServerProductType{ServerProductTypeAutoRenewable},
		SubscriptionGroupIdentifiers: []string{"group-1"},
		InAppOwnershipType:           &ownership,
		Revoked:                      &revoked,
		Sort:                         &order,
	}

	response := jsonResponse(http.StatusOK, `{"signedTransactions":[]}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v2/history/tx-1" {
			t.Fatalf("expected path /inApps/v2/history/tx-1, got %s", req.URL.Path)
		}
		values := req.URL.Query()
		if values.Get("revision") != "rev-1" {
			t.Fatalf("expected revision=rev-1, got %q", values.Get("revision"))
		}
		if values.Get("startDate") != "1700000000000" {
			t.Fatalf("expected startDate, got %q", values.Get("startDate"))
		}
		if values.Get("endDate") != "1700100000000" {
			t.Fatalf("expected endDate, got %q", values.Get("endDate"))
		}
		if values.Get("productId") == "" {
			t.Fatalf("expected productId params")
		}
		if values.Get("productType") != "AUTO_RENEWABLE" {
			t.Fatalf("expected productType AUTO_RENEWABLE, got %q", values.Get("productType"))
		}
		if values.Get("subscriptionGroupIdentifier") != "group-1" {
			t.Fatalf("expected subscriptionGroupIdentifier=group-1, got %q", values.Get("subscriptionGroupIdentifier"))
		}
		if values.Get("inAppOwnershipType") != "PURCHASED" {
			t.Fatalf("expected inAppOwnershipType=PURCHASED, got %q", values.Get("inAppOwnershipType"))
		}
		if values.Get("revoked") != "true" {
			t.Fatalf("expected revoked=true, got %q", values.Get("revoked"))
		}
		if values.Get("sort") != "DESCENDING" {
			t.Fatalf("expected sort=DESCENDING, got %q", values.Get("sort"))
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetTransactionHistory(context.Background(), "tx-1", request, "rev-1"); err != nil {
		t.Fatalf("GetTransactionHistory() error: %v", err)
	}
}

func TestGetTransactionHistory_EmptyTransactionID(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"signedTransactions":[]}`))
	if _, err := client.GetTransactionHistory(context.Background(), "", TransactionHistoryRequest{}, ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetRefundHistory_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"signedTransactions":[]}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v2/refund/lookup/tx-1" {
			t.Fatalf("expected path /inApps/v2/refund/lookup/tx-1, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("revision") != "rev-1" {
			t.Fatalf("expected revision=rev-1")
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetRefundHistory(context.Background(), "tx-1", "rev-1"); err != nil {
		t.Fatalf("GetRefundHistory() error: %v", err)
	}
}

func TestGetRefundHistory_EmptyTransactionID(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"signedTransactions":[]}`))
	if _, err := client.GetRefundHistory(context.Background(), "", ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetTransactionInfo_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"signedTransactionInfo":"signed"}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/transactions/tx-1" {
			t.Fatalf("expected path /inApps/v1/transactions/tx-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetTransactionInfo(context.Background(), "tx-1"); err != nil {
		t.Fatalf("GetTransactionInfo() error: %v", err)
	}
}

func TestGetTransactionInfo_EmptyTransactionID(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"signedTransactionInfo":"signed"}`))
	if _, err := client.GetTransactionInfo(context.Background(), ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLookUpOrderID_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"signedTransactions":[],"status":0}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/lookup/ORDER-1" {
			t.Fatalf("expected path /inApps/v1/lookup/ORDER-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.LookUpOrderID(context.Background(), "ORDER-1"); err != nil {
		t.Fatalf("LookUpOrderID() error: %v", err)
	}
}

func TestLookUpOrderID_EmptyOrderID(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"signedTransactions":[],"status":0}`))
	if _, err := client.LookUpOrderID(context.Background(), ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRequestTestNotification_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"testNotificationToken":"token-1"}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/notifications/test" {
			t.Fatalf("expected path /inApps/v1/notifications/test, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.RequestTestNotification(context.Background()); err != nil {
		t.Fatalf("RequestTestNotification() error: %v", err)
	}
}

func TestGetTestNotificationStatus_SendsRequest(t *testing.T) {
	response := jsonResponse(http.StatusOK, `{"signedPayload":"payload","sendAttempts":[]}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/notifications/test/token-1" {
			t.Fatalf("expected path /inApps/v1/notifications/test/token-1, got %s", req.URL.Path)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetTestNotificationStatus(context.Background(), "token-1"); err != nil {
		t.Fatalf("GetTestNotificationStatus() error: %v", err)
	}
}

func TestGetTestNotificationStatus_EmptyToken(t *testing.T) {
	client := newServerAPITestClient(t, nil, jsonResponse(http.StatusOK, `{"signedPayload":"payload","sendAttempts":[]}`))
	if _, err := client.GetTestNotificationStatus(context.Background(), ""); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetNotificationHistory_SendsRequest(t *testing.T) {
	start := int64(1700000000000)
	end := int64(1700100000000)
	notificationType := ServerNotificationTypeSubscribed
	request := NotificationHistoryRequest{
		StartDate:        &start,
		EndDate:          &end,
		NotificationType: &notificationType,
	}

	response := jsonResponse(http.StatusOK, `{"notificationHistory":[]}`)
	client := newServerAPITestClient(t, func(req *http.Request) {
		if req.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", req.Method)
		}
		if req.URL.Path != "/inApps/v1/notifications/history" {
			t.Fatalf("expected path /inApps/v1/notifications/history, got %s", req.URL.Path)
		}
		if req.URL.Query().Get("paginationToken") != "token-1" {
			t.Fatalf("expected paginationToken=token-1")
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body error: %v", err)
		}
		var payload NotificationHistoryRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode body error: %v", err)
		}
		if payload.StartDate == nil || *payload.StartDate != start {
			t.Fatalf("expected startDate %d", start)
		}
		if payload.EndDate == nil || *payload.EndDate != end {
			t.Fatalf("expected endDate %d", end)
		}
		assertAuthorized(t, req)
	}, response)

	if _, err := client.GetNotificationHistory(context.Background(), request, "token-1"); err != nil {
		t.Fatalf("GetNotificationHistory() error: %v", err)
	}
}

func TestServerAPIErrorResponses(t *testing.T) {
	tests := []struct {
		name string
		call func(*ServerAPIClient) error
	}{
		{
			name: "status",
			call: func(c *ServerAPIClient) error {
				_, err := c.GetAllSubscriptionStatuses(context.Background(), "tx-1", nil)
				return err
			},
		},
		{
			name: "history",
			call: func(c *ServerAPIClient) error {
				_, err := c.GetTransactionHistory(context.Background(), "tx-1", TransactionHistoryRequest{}, "")
				return err
			},
		},
		{
			name: "refunds",
			call: func(c *ServerAPIClient) error {
				_, err := c.GetRefundHistory(context.Background(), "tx-1", "")
				return err
			},
		},
		{
			name: "transaction",
			call: func(c *ServerAPIClient) error {
				_, err := c.GetTransactionInfo(context.Background(), "tx-1")
				return err
			},
		},
		{
			name: "order",
			call: func(c *ServerAPIClient) error {
				_, err := c.LookUpOrderID(context.Background(), "ORDER-1")
				return err
			},
		},
		{
			name: "request-test",
			call: func(c *ServerAPIClient) error {
				_, err := c.RequestTestNotification(context.Background())
				return err
			},
		},
		{
			name: "test-status",
			call: func(c *ServerAPIClient) error {
				_, err := c.GetTestNotificationStatus(context.Background(), "token-1")
				return err
			},
		},
		{
			name: "notifications-history",
			call: func(c *ServerAPIClient) error {
				start := int64(1700000000000)
				end := int64(1700100000000)
				notificationType := ServerNotificationTypeSubscribed
				request := NotificationHistoryRequest{
					StartDate:        &start,
					EndDate:          &end,
					NotificationType: &notificationType,
				}
				_, err := c.GetNotificationHistory(context.Background(), request, "")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := jsonResponse(http.StatusBadRequest, `{"errorCode":4000006,"errorMessage":"Invalid transaction ID"}`)
			client := newServerAPITestClient(t, nil, response)
			err := tt.call(client)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var apiErr *serverAPIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected serverAPIError, got %v", err)
			}
		})
	}
}
