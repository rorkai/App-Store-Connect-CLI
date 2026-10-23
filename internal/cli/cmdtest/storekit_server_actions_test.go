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

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
	storekitapi "github.com/rudrankriyam/App-Store-Connect-CLI/internal/storekit"
)

// Regression: each action must wire the caller's flags to the exact Apple
// endpoint and print Apple's response or a receipt on success.
func TestStoreKitServerActionsRequestContracts(t *testing.T) {
	const requestID = "11111111-2222-4333-8444-555555555555"
	tests := []struct {
		name, method, path, body, response string
		status                             int
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
			args := append([]string{"storekit"}, tt.args...)
			args = append(args, "--environment", env, "--output", "json")
			stdout, stderr := runStoreKitCommand(t, args...)
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
