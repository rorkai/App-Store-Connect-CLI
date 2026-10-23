package cmdtest

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	rootcmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

// Regression: each action must wire the caller's flags to the exact Apple
// endpoint, preserve false/zero/omitted values, and print receipts only on success.
func TestStoreKitServerActionsRequestContracts(t *testing.T) {
	const requestID = "11111111-2222-4333-8444-555555555555"
	tests := []struct {
		name, method, path, body, response string
		status                             int
		production                         bool
		wantErr                            string
		args                               []string
		want                               map[string]any
	}{
		{
			name: "app transaction", method: "GET", path: "/inApps/v1/transactions/appTransactions/123", response: `{"signedAppTransactionInfo":"a.b.c","futureField":true}`, status: 200,
			args: []string{"app-transactions", "view", "--transaction-id", "123"}, want: map[string]any{"signedAppTransactionInfo": "a.b.c", "futureField": true},
		},
		{
			name: "account token", method: "PUT", path: "/inApps/v1/transactions/123/appAccountToken", body: `{"appAccountToken":"` + requestID + `"}`, status: 200,
			args: []string{"transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", requestID, "--confirm"}, want: map[string]any{"originalTransactionId": "123", "appAccountToken": requestID, "updated": true},
		},
		{
			name: "account token non-JSON success", method: "PUT", path: "/inApps/v1/transactions/123/appAccountToken", body: `{"appAccountToken":"` + requestID + `"}`, response: "OK", status: 200,
			args: []string{"transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", requestID, "--confirm"}, want: map[string]any{"originalTransactionId": "123", "appAccountToken": requestID, "updated": true},
		},
		{
			name: "consumption omitted percentage", method: "PUT", path: "/inApps/v2/transactions/consumption/123", body: `{"customerConsented":true,"deliveryStatus":"DELIVERED","sampleContentProvided":false,"refundPreference":"GRANT_PRORATED"}`, status: 202,
			args: []string{"transactions", "send-consumption", "--transaction-id", "123", "--customer-consented", "true", "--delivery-status", "delivered", "--sample-content-provided", "false", "--refund-preference", "grant_prorated", "--confirm"}, want: map[string]any{"transactionId": "123", "submitted": true},
		},
		{
			name: "consumption undelivered subscription", method: "PUT", path: "/inApps/v2/transactions/consumption/123", body: `{"customerConsented":true,"deliveryStatus":"UNDELIVERED_OTHER","sampleContentProvided":false}`, status: 202,
			args: []string{"transactions", "send-consumption", "--transaction-id", "123", "--customer-consented", "true", "--delivery-status", "UNDELIVERED_OTHER", "--sample-content-provided", "false", "--confirm"}, want: map[string]any{"transactionId": "123", "submitted": true},
		},
		{
			name: "consumption explicit zero", method: "PUT", path: "/inApps/v2/transactions/consumption/123", body: `{"customerConsented":true,"deliveryStatus":"UNDELIVERED_OTHER","sampleContentProvided":true,"consumptionPercentage":0}`, status: 202,
			args: []string{"transactions", "send-consumption", "--transaction-id", "123", "--customer-consented", "true", "--delivery-status", "UNDELIVERED_OTHER", "--sample-content-provided", "true", "--consumption-percentage", "0", "--confirm"}, want: map[string]any{"transactionId": "123", "submitted": true},
		},
		{
			name: "finish", method: "POST", path: "/inApps/v1/transactions/123/finish", status: 200,
			args: []string{"transactions", "finish", "--transaction-id", "123", "--confirm"}, want: map[string]any{"transactionId": "123", "finished": true},
		},
		{
			name: "individual extension unsuccessful", method: "PUT", path: "/inApps/v1/subscriptions/extend/123", body: `{"extendByDays":7,"extendReasonCode":0,"requestIdentifier":"incident-123"}`, response: `{"originalTransactionId":"123","success":false}`, status: 200, wantErr: "Apple did not extend the renewal date",
			args: []string{"subscriptions", "extend", "--original-transaction-id", "123", "--days", "7", "--reason", "0", "--request-id", "incident-123", "--confirm"}, want: map[string]any{"originalTransactionId": "123", "success": false},
		},
		{
			name: "bulk extension", method: "POST", path: "/inApps/v1/subscriptions/extend/mass", body: `{"extendByDays":90,"extendReasonCode":3,"requestIdentifier":"` + requestID + `","productId":"com.example.monthly","storefrontCountryCodes":["USA","GBR"]}`, response: `{"requestIdentifier":"` + requestID + `"}`, status: 200, production: true,
			args: []string{"subscriptions", "extend-all", "--product-id", "com.example.monthly", "--days", "90", "--reason", "3", "--request-id", requestID, "--storefront-country-codes", "usa,GBR", "--confirm"}, want: map[string]any{"requestIdentifier": requestID},
		},
		{
			name: "bulk all storefronts", method: "POST", path: "/inApps/v1/subscriptions/extend/mass", body: `{"extendByDays":1,"extendReasonCode":1,"requestIdentifier":"` + requestID + `","productId":"com.example.monthly"}`, response: `{"requestIdentifier":"` + requestID + `"}`, status: 200,
			args: []string{"subscriptions", "extend-all", "--product-id", "com.example.monthly", "--days", "1", "--reason", "1", "--request-id", requestID, "--confirm"}, want: map[string]any{"requestIdentifier": requestID},
		},
		{
			name: "bulk braced request id", method: "POST", path: "/inApps/v1/subscriptions/extend/mass", body: `{"extendByDays":1,"extendReasonCode":1,"requestIdentifier":"` + requestID + `","productId":"com.example.monthly"}`, response: `{"requestIdentifier":"` + requestID + `"}`, status: 200,
			args: []string{"subscriptions", "extend-all", "--product-id", "com.example.monthly", "--days", "1", "--reason", "1", "--request-id", "{" + strings.ToUpper(requestID) + "}", "--confirm"}, want: map[string]any{"requestIdentifier": requestID},
		},
		{
			name: "bulk status urn request id", method: "GET", path: "/inApps/v1/subscriptions/extend/mass/com.example.monthly/" + requestID, response: `{"requestIdentifier":"` + requestID + `","complete":false}`, status: 200,
			args: []string{"subscriptions", "extension-status", "--product-id", "com.example.monthly", "--request-id", "urn:uuid:" + requestID}, want: map[string]any{"requestIdentifier": requestID, "complete": false},
		},
		{
			name: "bulk incomplete status", method: "GET", path: "/inApps/v1/subscriptions/extend/mass/com.example.monthly/" + requestID, response: `{"requestIdentifier":"` + requestID + `","complete":false}`, status: 200,
			args: []string{"subscriptions", "extension-status", "--product-id", "com.example.monthly", "--request-id", requestID}, want: map[string]any{"requestIdentifier": requestID, "complete": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupStoreKitAuth(t)
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != tt.method || r.URL.Path != tt.path || r.URL.RawQuery != "" {
					t.Errorf("request=%s %s, want %s %s", r.Method, r.URL, tt.method, tt.path)
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Error("missing bearer token")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if tt.body == "" {
					if len(body) != 0 {
						t.Errorf("unexpected body %q", body)
					}
				} else {
					var got, want any
					if err := json.Unmarshal(body, &got); err != nil {
						t.Errorf("request JSON: %v", err)
					}
					if err := json.Unmarshal([]byte(tt.body), &want); err != nil {
						t.Error(err)
					}
					if !reflect.DeepEqual(got, want) {
						t.Errorf("body=%s want %s", body, tt.body)
					}
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("Content-Type=%q", r.Header.Get("Content-Type"))
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				wantHost := "api.storekit-sandbox.apple.com"
				if tt.production {
					wantHost = "api.storekit.apple.com"
				}
				if r.URL.Host != wantHost {
					t.Errorf("host=%s want %s", r.URL.Host, wantHost)
				}
				cloned := r.Clone(r.Context())
				u := *r.URL
				cloned.URL = &u
				cloned.URL.Scheme = "http"
				cloned.URL.Host = strings.TrimPrefix(server.URL, "http://")
				return original.RoundTrip(cloned)
			})
			env := "sandbox"
			if tt.production {
				env = "production"
			}
			args := append([]string{"storekit"}, tt.args...)
			args = append(args, "--environment", env, "--output", "json")
			stdout, stderr, err := runRootCommand(t, args)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("run error: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr) || rootcmd.ExitCodeFromError(err) != rootcmd.ExitError) {
				t.Fatalf("err=%v exit=%d want %q", err, rootcmd.ExitCodeFromError(err), tt.wantErr)
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("JSON %q: %v", stdout, err)
			}
			if !reflect.DeepEqual(got, tt.want) || stderr != "" || calls != 1 {
				t.Fatalf("output=%#v stderr=%q calls=%d want=%#v", got, stderr, calls, tt.want)
			}
		})
	}
}

// Regression: invalid or unconfirmed writes must fail before credentials or HTTP.
func TestStoreKitServerActionsUsageErrors(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"app transaction required", []string{"app-transactions", "view"}, "--transaction-id is required"},
		{"token original required", []string{"transactions", "set-account-token", "--app-account-token", id, "--confirm"}, "--original-transaction-id is required"},
		{"token uuid", []string{"transactions", "set-account-token", "--original-transaction-id", "1", "--app-account-token", "bad", "--confirm"}, "--app-account-token must be a UUID"},
		{"token confirm", []string{"transactions", "set-account-token", "--original-transaction-id", "1", "--app-account-token", id}, "--confirm is required"},
		{"finish confirm", []string{"transactions", "finish", "--transaction-id", "1"}, "--confirm is required"},
		{"consent required", []string{"transactions", "send-consumption", "--transaction-id", "1", "--delivery-status", "DELIVERED", "--sample-content-provided", "false", "--confirm"}, "--customer-consented must be explicitly true"},
		{"consent false", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "false", "--delivery-status", "DELIVERED", "--sample-content-provided", "false", "--confirm"}, "--customer-consented must be explicitly true"},
		{"sample required", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--delivery-status", "DELIVERED", "--confirm"}, "--sample-content-provided is required"},
		{"delivery required", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--confirm"}, "--delivery-status is required"},
		{"delivery enum", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "OTHER", "--confirm"}, "--delivery-status must be one of"},
		{"percentage range", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "DELIVERED", "--consumption-percentage", "100001", "--confirm"}, "--consumption-percentage must be between 0 and 100000"},
		{"undelivered percentage", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "UNDELIVERED_OTHER", "--consumption-percentage", "1", "--confirm"}, "--consumption-percentage must be 0 when delivery status is not DELIVERED"},
		{"prorated zero", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "DELIVERED", "--refund-preference", "GRANT_PRORATED", "--consumption-percentage", "0", "--confirm"}, "--consumption-percentage must be between 1 and 99999 for GRANT_PRORATED"},
		{"prorated full before confirm", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "DELIVERED", "--refund-preference", "GRANT_PRORATED", "--consumption-percentage", "100000"}, "--consumption-percentage must be between 1 and 99999 for GRANT_PRORATED"},
		{"refund enum", []string{"transactions", "send-consumption", "--transaction-id", "1", "--customer-consented", "true", "--sample-content-provided", "false", "--delivery-status", "DELIVERED", "--refund-preference", "YES", "--confirm"}, "--refund-preference must be one of"},
		{"days range", []string{"subscriptions", "extend", "--original-transaction-id", "1", "--days", "91", "--reason", "3", "--request-id", "incident", "--confirm"}, "--days must be between 1 and 90"},
		{"reason required", []string{"subscriptions", "extend", "--original-transaction-id", "1", "--days", "7", "--request-id", "incident", "--confirm"}, "--reason is required"},
		{"reason range", []string{"subscriptions", "extend", "--original-transaction-id", "1", "--days", "7", "--reason", "4", "--request-id", "incident", "--confirm"}, "--reason must be between 0 and 3"},
		{"request required", []string{"subscriptions", "extend", "--original-transaction-id", "1", "--days", "7", "--reason", "3", "--confirm"}, "--request-id is required"},
		{"request length", []string{"subscriptions", "extend", "--original-transaction-id", "1", "--days", "7", "--reason", "3", "--request-id", strings.Repeat("a", 129), "--confirm"}, "--request-id must be at most 128 characters"},
		{"bulk uuid", []string{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", "incident", "--confirm"}, "--request-id must be a UUID"},
		{"bulk country", []string{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", id, "--storefront-country-codes", "US", "--confirm"}, "--storefront-country-codes must contain three-letter country codes"},
		{"bulk country before confirm", []string{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", id, "--storefront-country-codes", "US"}, "--storefront-country-codes must contain three-letter country codes"},
		{"bulk empty countries", []string{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", id, "--storefront-country-codes", "", "--confirm"}, "--storefront-country-codes must not be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupStoreKitAuth(t)
			stubStoreKitTransport(t, func(r *http.Request) string { t.Errorf("unexpected HTTP %s", r.URL); return `{}` })
			args := append([]string{"storekit"}, tt.args...)
			args = append(args, "--environment", "sandbox")
			stdout, stderr, err := runRootCommand(t, args)
			if !errors.Is(err, flag.ErrHelp) || stdout != "" || !strings.Contains(stderr, tt.want) {
				t.Fatalf("stdout=%q stderr=%q err=%v want=%q", stdout, stderr, err, tt.want)
			}
		})
	}
}

// Regression: Apple responses and accepted-write receipts must render as
// table and markdown rows.
func TestStoreKitServerActionsHumanOutput(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	tests := []struct {
		name, format, response string
		args, want             []string
		warning                bool
	}{
		{"app transaction", "table", `{"signedAppTransactionInfo":"` + storeKitTestJWS(`{"appTransactionId":"123","bundleId":"com.example.app","originalApplicationVersion":"1.0","originalPurchaseDate":1788220800000,"receiptType":"Sandbox"}`) + `"}`, []string{"app-transactions", "view", "--transaction-id", "123"}, []string{"App Transaction ID", "Original App Version", "1.0", "com.example.app", "2026-09-01T00:00:00Z"}, true},
		{"account token", "markdown", ``, []string{"transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", id, "--confirm"}, []string{"|123|" + id + "|true|"}, false},
		{"consumption", "markdown", ``, []string{"transactions", "send-consumption", "--transaction-id", "123", "--customer-consented", "true", "--delivery-status", "DELIVERED", "--sample-content-provided", "false", "--confirm"}, []string{"|123|true|"}, false},
		{"finish", "markdown", ``, []string{"transactions", "finish", "--transaction-id", "123", "--confirm"}, []string{"|123|true|"}, false},
		{"single success", "table", `{"success":true,"originalTransactionId":"123","effectiveDate":1788220800000,"webOrderLineItemId":"456"}`, []string{"subscriptions", "extend", "--original-transaction-id", "123", "--days", "7", "--reason", "3", "--request-id", "incident", "--confirm"}, []string{"123", "true", "2026-09-01T00:00:00Z", "456"}, false},
		{"submission", "markdown", `{"requestIdentifier":"` + id + `"}`, []string{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", id, "--confirm"}, []string{"|" + id + "|"}, false},
		{"completed status", "markdown", `{"requestIdentifier":"` + id + `","complete":true,"completeDate":1788220800000,"succeededCount":12,"failedCount":0}`, []string{"subscriptions", "extension-status", "--product-id", "p", "--request-id", id}, []string{"| " + id + " | true | 2026-09-01T00:00:00Z | 12 | 0 |"}, false},
		{"incomplete status", "markdown", `{"requestIdentifier":"` + id + `","complete":false}`, []string{"subscriptions", "extension-status", "--product-id", "p", "--request-id", id}, []string{"| " + id + " | false |  |  |  |"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setupStoreKitAuth(t)
			stubStoreKitTransport(t, func(*http.Request) string { return tt.response })
			args := append([]string{"storekit"}, tt.args...)
			args = append(args, "--environment", "sandbox", "--output", tt.format)
			stdout, stderr := runStoreKitCommand(t, args...)
			for _, want := range tt.want {
				actual := stdout
				if tt.format == "markdown" {
					actual = normalizeStoreKitMarkdown(actual)
					want = normalizeStoreKitMarkdown(want)
				}
				if !strings.Contains(actual, want) {
					t.Errorf("output=%q missing %q", stdout, want)
				}
			}
			if tt.warning {
				if !strings.Contains(stderr, "not signature-verified") {
					t.Errorf("stderr=%q", stderr)
				}
			} else if stderr != "" {
				t.Errorf("stderr=%q", stderr)
			}
		})
	}
}

// Regression: every new write must be refused locally in read-only mode and
// must never print a successful receipt after refusal or an Apple error.
func TestStoreKitServerActionsRefuseWritesAndPreserveAPIErrors(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	writes := [][]string{
		{"transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", id},
		{"transactions", "finish", "--transaction-id", "123"},
		{"transactions", "send-consumption", "--transaction-id", "123", "--customer-consented", "true", "--delivery-status", "DELIVERED", "--sample-content-provided", "false"},
		{"subscriptions", "extend", "--original-transaction-id", "123", "--days", "7", "--reason", "3", "--request-id", "incident"},
		{"subscriptions", "extend-all", "--product-id", "p", "--days", "7", "--reason", "3", "--request-id", id},
	}
	for _, args := range writes {
		t.Run(args[1], func(t *testing.T) {
			setupStoreKitAuth(t)
			t.Setenv("ASC_READ_ONLY", "1")
			calls := 0
			stubStoreKitTransport(t, func(*http.Request) string { calls++; return `{}` })
			command := append([]string{"storekit"}, args...)
			command = append(command, "--environment", "sandbox", "--confirm", "--output", "json")
			stdout, _, err := runRootCommand(t, command)
			if !errors.Is(err, readonly.ErrRefused) || stdout != "" || calls != 0 {
				t.Fatalf("stdout=%q err=%v calls=%d", stdout, err, calls)
			}
		})
	}
	t.Run("API failure", func(t *testing.T) {
		setupStoreKitAuth(t)
		t.Setenv("ASC_READ_ONLY", "")
		original := http.DefaultTransport
		t.Cleanup(func() { http.DefaultTransport = original })
		http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 429, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"errorCode":4290000,"errorMessage":"Rate limit exceeded"}`))}, nil
		})
		stdout, _, err := runRootCommand(t, []string{"storekit", "transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", "11111111-2222-4333-8444-555555555555", "--confirm", "--environment", "sandbox", "--output", "json"})
		var apiErr *storekitapi.APIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 || apiErr.Code != "4290000" || stdout != "" {
			t.Fatalf("stdout=%q err=%v", stdout, err)
		}
	})
}

// Regression: cancellation must reach the HTTP request, with no receipt.
func TestStoreKitServerActionCancellation(t *testing.T) {
	setupStoreKitAuth(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	root := RootCommand("test")
	if err := root.Parse([]string{"storekit", "transactions", "set-account-token", "--original-transaction-id", "123", "--app-account-token", "11111111-2222-4333-8444-555555555555", "--confirm", "--environment", "sandbox"}); err != nil {
		t.Fatal(err)
	}
	var runErr error
	stdout, _ := captureOutput(t, func() { runErr = root.Run(ctx) })
	if !errors.Is(runErr, context.Canceled) || stdout != "" {
		t.Fatalf("stdout=%q err=%v", stdout, runErr)
	}
}

// Ignore renderer padding while preserving empty cells and column boundaries.
func normalizeStoreKitMarkdown(value string) string {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		cells := strings.Split(line, "|")
		for j := range cells {
			cells[j] = strings.TrimSpace(cells[j])
		}
		lines[i] = strings.Join(cells, "|")
	}
	return strings.Join(lines, "\n")
}
