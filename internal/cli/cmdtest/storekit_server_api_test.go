package cmdtest

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestStoreKitServerAPICommandTree(t *testing.T) {
	root := RootCommand("1.2.3")
	paths := [][]string{
		{"storekit", "transactions", "view"},
		{"storekit", "transactions", "history"},
		{"storekit", "subscriptions", "status"},
		{"storekit", "refunds", "history"},
		{"storekit", "orders", "lookup"},
		{"storekit", "notifications", "history"},
		{"storekit", "notifications", "test"},
		{"storekit", "notifications", "test-status"},
		{"storekit", "groups", "list"},
		{"storekit", "groups", "members"},
	}
	for _, path := range paths {
		if findSubcommand(root, path...) == nil {
			t.Errorf("missing command path %s", strings.Join(path, " "))
		}
	}
}

func TestStoreKitGroupsListProduction(t *testing.T) {
	setupStoreKitAuth(t)
	stubStoreKitTransport(t, func(req *http.Request) string {
		if req.Method != http.MethodGet || req.URL.Host != "api.storekit.apple.com" || req.URL.Path != "/groups/v1/currentGroups/2000000000000001" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return `{"groups":[{"groupId":"900000000000000000","groupType":"ORGANIZATION","roles":[]}]}`
	})

	stdout, stderr := runStoreKitCommand(t,
		"storekit", "groups", "list", "--transaction-id", "2000000000000001", "--environment", "production", "--output", "json")
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stdout, `"groupType":"ORGANIZATION"`) {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestStoreKitTransactionsHistoryPaginatesProduction(t *testing.T) {
	setupStoreKitAuth(t)
	var queries []string
	stubStoreKitTransport(t, func(req *http.Request) string {
		if req.Method != http.MethodGet || req.URL.Host != "api.storekit.apple.com" || req.URL.Path != "/inApps/v2/history/2000000000000001" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		queries = append(queries, req.URL.RawQuery)
		if req.URL.Query().Get("revision") == "" {
			return `{"bundleId":"com.example.app","environment":"Production","revision":"rev-1","hasMore":true,"signedTransactions":["a.b.c"]}`
		}
		return `{"bundleId":"com.example.app","environment":"Production","revision":"rev-2","hasMore":false,"signedTransactions":["d.e.f"]}`
	})

	stdout, stderr := runStoreKitCommand(t,
		"storekit", "transactions", "history", "--transaction-id", "2000000000000001",
		"--product-type", "auto_renewable", "--ownership-type", "assigned", "--sort", "descending", "--environment", "production", "--paginate", "--output", "json")
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	wantQueries := []string{
		"inAppOwnershipType=ASSIGNED&productType=AUTO_RENEWABLE&sort=DESCENDING",
		"inAppOwnershipType=ASSIGNED&productType=AUTO_RENEWABLE&revision=rev-1&sort=DESCENDING",
	}
	if !reflect.DeepEqual(queries, wantQueries) {
		t.Fatalf("queries = %q, want %q", queries, wantQueries)
	}
	var response struct {
		Revision           string   `json:"revision"`
		HasMore            bool     `json:"hasMore"`
		SignedTransactions []string `json:"signedTransactions"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	if response.Revision != "rev-2" || response.HasMore || !reflect.DeepEqual(response.SignedTransactions, []string{"a.b.c", "d.e.f"}) {
		t.Fatalf("response = %#v", response)
	}
}

func TestStoreKitTransactionsViewDecodeAddsUnverifiedPayload(t *testing.T) {
	setupStoreKitAuth(t)
	token := storeKitTestJWS(`{"transactionId":"2000000000000001","productId":"com.example.monthly","purchaseDate":1788220800000,"inAppOwnershipType":"ASSIGNED","revocationType":"ASSIGNMENT_REVOKE"}`)
	stubStoreKitTransport(t, func(req *http.Request) string {
		if req.Method != http.MethodGet || req.URL.Host != "api.storekit-sandbox.apple.com" || req.URL.Path != "/inApps/v1/transactions/2000000000000001" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		return `{"signedTransactionInfo":"` + token + `"}`
	})

	stdout, stderr := runStoreKitCommand(t,
		"storekit", "transactions", "view", "--transaction-id", "2000000000000001", "--environment", "sandbox", "--decode", "--output", "json")
	if !strings.Contains(stderr, "not signature-verified") {
		t.Fatalf("stderr = %q, want unverified warning", stderr)
	}
	var response struct {
		SignedTransactionInfo        string `json:"signedTransactionInfo"`
		SignedTransactionInfoDecoded struct {
			ProductID          string `json:"productId"`
			InAppOwnershipType string `json:"inAppOwnershipType"`
			RevocationType     string `json:"revocationType"`
		} `json:"signedTransactionInfoDecoded"`
	}
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("invalid JSON output %q: %v", stdout, err)
	}
	decoded := response.SignedTransactionInfoDecoded
	if response.SignedTransactionInfo != token || decoded.ProductID != "com.example.monthly" || decoded.InAppOwnershipType != "ASSIGNED" || decoded.RevocationType != "ASSIGNMENT_REVOKE" {
		t.Fatalf("response = %#v", response)
	}

	stdout, stderr = runStoreKitCommand(t,
		"storekit", "transactions", "view", "--transaction-id", "2000000000000001", "--environment", "sandbox", "--output", "table")
	if !strings.Contains(stderr, "not signature-verified") || !strings.Contains(stdout, "Product ID") || !strings.Contains(stdout, "com.example.monthly") || !strings.Contains(stdout, "2026-09-01T00:00:00Z") ||
		!strings.Contains(stdout, "Ownership") || !strings.Contains(stdout, "ASSIGNED") || !strings.Contains(stdout, "ASSIGNMENT_REVOKE") {
		t.Fatalf("table stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestStoreKitNotificationsHistoryPostsRequestInReadOnlyMode(t *testing.T) {
	setupStoreKitAuth(t)
	t.Setenv("ASC_READ_ONLY", "1")
	stubStoreKitTransport(t, func(req *http.Request) string {
		if req.Method != http.MethodPost || req.URL.Path != "/inApps/v1/notifications/history" || req.URL.RawQuery != "" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		want := map[string]any{"startDate": float64(1788220800000), "endDate": float64(1788825600000), "notificationType": "SUBSCRIBED", "onlyFailures": true}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("body = %#v, want %#v", body, want)
		}
		return `{"notificationHistory":[],"hasMore":true,"paginationToken":"next-1"}`
	})

	stdout, stderr := runStoreKitCommand(t,
		"storekit", "notifications", "history", "--start", "2026-09-01", "--end", "2026-09-08",
		"--type", "subscribed", "--only-failures", "--environment", "sandbox", "--output", "json")
	if !strings.Contains(stdout, `"paginationToken":"next-1"`) {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "--pagination-token next-1") {
		t.Fatalf("stderr = %q, want more-pages hint", stderr)
	}
}

func TestStoreKitServerAPIUsageErrors(t *testing.T) {
	setupStoreKitAuth(t)
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"missing transaction", []string{"storekit", "transactions", "view", "--environment", "sandbox"}, "--transaction-id is required"},
		{"decode table", []string{"storekit", "transactions", "view", "--transaction-id", "1", "--decode", "--output", "table", "--environment", "sandbox"}, "--decode requires --output json"},
		{"product type", []string{"storekit", "transactions", "history", "--transaction-id", "1", "--product-type", "BUNDLE", "--environment", "sandbox"}, "--product-type must be one of"},
		{"ownership type", []string{"storekit", "transactions", "history", "--transaction-id", "1", "--ownership-type", "GIFTED", "--environment", "sandbox"}, "--ownership-type must be one of: FAMILY_SHARED, PURCHASED, ASSIGNED"},
		{"ownership type list", []string{"storekit", "transactions", "history", "--transaction-id", "1", "--ownership-type", "PURCHASED,ASSIGNED", "--environment", "sandbox"}, "--ownership-type must be one of: FAMILY_SHARED, PURCHASED, ASSIGNED"},
		{"history dates", []string{"storekit", "transactions", "history", "--transaction-id", "1", "--start", "2026-09-02", "--end", "2026-09-01", "--environment", "sandbox"}, "--start must be before --end"},
		{"status", []string{"storekit", "subscriptions", "status", "--transaction-id", "1", "--status", "6", "--environment", "sandbox"}, "--status values must be integers from 1 to 5"},
		{"order sandbox", []string{"storekit", "orders", "lookup", "--order-id", "MQXYZ12345", "--environment", "sandbox"}, "orders lookup requires --environment production"},
		{"history missing end", []string{"storekit", "notifications", "history", "--start", "2026-09-01", "--environment", "sandbox"}, "--end is required"},
		{"history bad date", []string{"storekit", "notifications", "history", "--start", "yesterday", "--end", "2026-09-01", "--environment", "sandbox"}, "--start must be YYYY-MM-DD or RFC3339"},
		{"subtype without type", []string{"storekit", "notifications", "history", "--start", "2026-09-01", "--end", "2026-09-02", "--subtype", "INITIAL_BUY", "--environment", "sandbox"}, "--subtype requires --type"},
		{"type and transaction", []string{"storekit", "notifications", "history", "--start", "2026-09-01", "--end", "2026-09-02", "--type", "REFUND", "--transaction-id", "1", "--environment", "sandbox"}, "--type and --transaction-id are mutually exclusive"},
		{"test confirm", []string{"storekit", "notifications", "test", "--environment", "sandbox"}, "--confirm is required"},
		{"members limit", []string{"storekit", "groups", "members", "--group-id", "900000000000000000", "--limit", "101", "--environment", "sandbox"}, "--limit must be between 1 and 100"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubStoreKitTransport(t, func(req *http.Request) string {
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.String())
				return ""
			})
			stdout, stderr, err := runRootCommand(t, tt.args)
			if !errors.Is(err, flag.ErrHelp) {
				t.Fatalf("run error = %v, want flag.ErrHelp", err)
			}
			if stdout != "" || !strings.Contains(stderr, tt.wantErr) {
				t.Fatalf("stdout=%q stderr=%q, want %q", stdout, stderr, tt.wantErr)
			}
		})
	}
}

func stubStoreKitTransport(t *testing.T, respond func(*http.Request) string) {
	t.Helper()
	originalTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = originalTransport })
	http.DefaultTransport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") {
			t.Fatalf("missing bearer token: %q", req.Header.Get("Authorization"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(respond(req))),
		}, nil
	})
}

func runStoreKitCommand(t *testing.T, args ...string) (string, string) {
	t.Helper()
	stdout, stderr, err := runRootCommand(t, args)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	return stdout, stderr
}

func storeKitTestJWS(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".c2ln"
}
