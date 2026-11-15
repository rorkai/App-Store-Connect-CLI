package storekit

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestClientSignsStoreKitJWT(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	client, err := NewClient(testCredentials(t), Sandbox, WithNow(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	tokenString, err := client.signedToken()
	if err != nil {
		t.Fatalf("signedToken() error = %v", err)
	}

	parsed, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		t.Fatalf("ParseUnverified() error = %v", err)
	}
	if parsed.Method.Alg() != "ES256" {
		t.Fatalf("alg = %q, want ES256", parsed.Method.Alg())
	}
	if got := parsed.Header["kid"]; got != "STOREKIT_KEY" {
		t.Fatalf("kid = %v, want STOREKIT_KEY", got)
	}
	if got := parsed.Header["typ"]; got != "JWT" {
		t.Fatalf("typ = %v, want JWT", got)
	}
	claims := parsed.Claims.(jwt.MapClaims)
	if got := claims["iss"]; got != "STOREKIT_ISSUER" {
		t.Fatalf("iss = %v, want STOREKIT_ISSUER", got)
	}
	if got := claims["aud"]; got != "appstoreconnect-v1" {
		t.Fatalf("aud = %v, want appstoreconnect-v1", got)
	}
	if got := claims["bid"]; got != "com.example.app" {
		t.Fatalf("bid = %v, want com.example.app", got)
	}
	// Issued-at is backdated so a client clock running ahead of Apple's does not
	// produce a token that is rejected as issued in the future.
	if got, want := int64(claims["iat"].(float64)), now.Add(-jwtIssuedAtSkew).Unix(); got != want {
		t.Fatalf("iat = %d, want %d", got, want)
	}
	if got := int64(claims["exp"].(float64)); got != now.Add(20*time.Minute).Unix() {
		t.Fatalf("exp = %d, want %d", got, now.Add(20*time.Minute).Unix())
	}
}

func TestRetentionMessagingEndpoints(t *testing.T) {
	type invocation struct {
		name        string
		method      string
		path        string
		query       string
		contentType string
		response    string
		call        func(context.Context, *Client) error
	}

	message := Message{
		Header: "Still enjoying Example?",
		Body:   "Keep your subscription and keep everything you have unlocked.",
		Image:  &MessageImage{ImageIdentifier: "22222222-2222-4222-8222-222222222222", AltText: "Example app"},
	}
	tests := []invocation{
		{name: "upload image", method: http.MethodPut, path: "/inApps/v1/messaging/image/11111111-1111-4111-8111-111111111111", query: "imageSize=FULL_SIZE", contentType: "image/png", call: func(ctx context.Context, c *Client) error {
			return c.UploadImage(ctx, "11111111-1111-4111-8111-111111111111", ImageSizeFull, []byte("\x89PNG\r\n\x1a\nimage"))
		}},
		{name: "delete image", method: http.MethodDelete, path: "/inApps/v1/messaging/image/11111111-1111-4111-8111-111111111111", call: func(ctx context.Context, c *Client) error {
			return c.DeleteImage(ctx, "11111111-1111-4111-8111-111111111111")
		}},
		{name: "list images", method: http.MethodGet, path: "/inApps/v1/messaging/image/list", response: `{"imageIdentifiers":[]}`, call: func(ctx context.Context, c *Client) error { _, err := c.ListImages(ctx); return err }},
		{name: "upload message", method: http.MethodPut, path: "/inApps/v1/messaging/message/33333333-3333-4333-8333-333333333333", contentType: "application/json", call: func(ctx context.Context, c *Client) error {
			return c.UploadMessage(ctx, "33333333-3333-4333-8333-333333333333", message)
		}},
		{name: "delete message", method: http.MethodDelete, path: "/inApps/v1/messaging/message/33333333-3333-4333-8333-333333333333", call: func(ctx context.Context, c *Client) error {
			return c.DeleteMessage(ctx, "33333333-3333-4333-8333-333333333333")
		}},
		{name: "list messages", method: http.MethodGet, path: "/inApps/v1/messaging/message/list", response: `{"messageIdentifiers":[]}`, call: func(ctx context.Context, c *Client) error { _, err := c.ListMessages(ctx); return err }},
		{name: "set default", method: http.MethodPut, path: "/inApps/v1/messaging/default/monthly/en-US", contentType: "application/json", response: `{"messageIdentifier":"33333333-3333-4333-8333-333333333333"}`, call: func(ctx context.Context, c *Client) error {
			_, err := c.SetDefault(ctx, "monthly", "en-US", "33333333-3333-4333-8333-333333333333")
			return err
		}},
		{name: "get default", method: http.MethodGet, path: "/inApps/v1/messaging/default/monthly/en-US", response: `{"messageIdentifier":"33333333-3333-4333-8333-333333333333"}`, call: func(ctx context.Context, c *Client) error {
			_, err := c.GetDefault(ctx, "monthly", "en-US")
			return err
		}},
		{name: "delete default", method: http.MethodDelete, path: "/inApps/v1/messaging/default/monthly/en-US", call: func(ctx context.Context, c *Client) error { return c.DeleteDefault(ctx, "monthly", "en-US") }},
		{name: "set endpoint", method: http.MethodPut, path: "/inApps/v1/messaging/realtime/url", contentType: "application/json", response: `{"realtimeURL":"https://example.com/retention"}`, call: func(ctx context.Context, c *Client) error {
			_, err := c.SetRealtimeURL(ctx, "https://example.com/retention")
			return err
		}},
		{name: "get endpoint", method: http.MethodGet, path: "/inApps/v1/messaging/realtime/url", response: `{"realtimeURL":"https://example.com/retention"}`, call: func(ctx context.Context, c *Client) error { _, err := c.GetRealtimeURL(ctx); return err }},
		{name: "delete endpoint", method: http.MethodDelete, path: "/inApps/v1/messaging/realtime/url", call: func(ctx context.Context, c *Client) error { return c.DeleteRealtimeURL(ctx) }},
		{name: "start performance test", method: http.MethodPost, path: "/inApps/v1/messaging/performanceTest", contentType: "application/json", response: `{"config":{"duration":60},"requestId":"request-1"}`, call: func(ctx context.Context, c *Client) error {
			_, err := c.StartPerformanceTest(ctx, "2000000000000000")
			return err
		}},
		{name: "get performance test", method: http.MethodGet, path: "/inApps/v1/messaging/performanceTest/result/request-1", response: `{"requestId":"request-1","result":"PASS","successRate":1}`, call: func(ctx context.Context, c *Client) error {
			_, err := c.GetPerformanceTestResult(ctx, "request-1")
			return err
		}},
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
				if r.URL.RawQuery != tt.query {
					t.Errorf("query = %q, want %q", r.URL.RawQuery, tt.query)
				}
				if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
					t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
				}
				if tt.contentType != "" && r.Header.Get("Content-Type") != tt.contentType {
					t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), tt.contentType)
				}
				if tt.method == http.MethodPut || tt.method == http.MethodPost {
					body, err := io.ReadAll(r.Body)
					if err != nil || len(body) == 0 {
						t.Errorf("expected request body, body=%q err=%v", body, err)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if tt.response == "" {
					w.WriteHeader(http.StatusOK)
					return
				}
				_, _ = io.WriteString(w, tt.response)
			}))
			defer server.Close()

			client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}
			if err := tt.call(context.Background(), client); err != nil {
				t.Fatalf("call error = %v", err)
			}
		})
	}
}

func TestClientDecodesAPIErrorAndRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "1781798400000")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"errorCode": "RATE_LIMIT_EXCEEDED", "errorMessage": "Slow down"})
	}))
	defer server.Close()

	client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.ListMessages(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusTooManyRequests || apiErr.Code != "RATE_LIMIT_EXCEEDED" || apiErr.Message != "Slow down" {
		t.Fatalf("APIError = %#v", apiErr)
	}
	if got := apiErr.RetryAfter.UnixMilli(); got != 1781798400000 {
		t.Fatalf("RetryAfter = %d", got)
	}
}

func TestClientDecodesNumericAPIErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errorCode":4000001,"errorMessage":"Too many bullet points"}`)
	}))
	defer server.Close()

	client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.ListMessages(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %v, want *APIError", err, err)
	}
	if apiErr.Code != "4000001" || apiErr.Message != "Too many bullet points" {
		t.Fatalf("APIError = %#v", apiErr)
	}
}

func TestConfigureReturnsRequestedValueForEmptySuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := NewClient(testCredentials(t), Sandbox, WithHTTPClient(server.Client()), WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	configuredDefault, err := client.SetDefault(context.Background(), "monthly", "en-US", "33333333-3333-4333-8333-333333333333")
	if err != nil {
		t.Fatalf("SetDefault() error = %v", err)
	}
	if configuredDefault.MessageIdentifier != "33333333-3333-4333-8333-333333333333" {
		t.Fatalf("SetDefault() = %#v", configuredDefault)
	}
	configuredURL, err := client.SetRealtimeURL(context.Background(), "https://example.com/retention")
	if err != nil {
		t.Fatalf("SetRealtimeURL() error = %v", err)
	}
	if configuredURL.RealtimeURL != "https://example.com/retention" {
		t.Fatalf("SetRealtimeURL() = %#v", configuredURL)
	}
}

func TestValidateMessageRequiresImageAccessibilityText(t *testing.T) {
	message := Message{Header: "Stay", Body: "Keep your subscription", Image: &MessageImage{ImageIdentifier: "22222222-2222-4222-8222-222222222222"}}
	if err := ValidateMessage(message); err == nil || !strings.Contains(err.Error(), "image alt text is required") {
		t.Fatalf("ValidateMessage() error = %v", err)
	}
	message.Image = nil
	message.BulletPoints = []MessageBulletPoint{{ImageIdentifier: "22222222-2222-4222-8222-222222222222", Text: "One benefit"}}
	if err := ValidateMessage(message); err == nil || !strings.Contains(err.Error(), "bullet point 1 alt text is required") {
		t.Fatalf("ValidateMessage() error = %v", err)
	}
	message.BulletPoints = nil
	message.HeaderPosition = HeaderAboveImage
	if err := ValidateMessage(message); err == nil || !strings.Contains(err.Error(), "ABOVE_IMAGE requires an image") {
		t.Fatalf("ValidateMessage() error = %v", err)
	}
	message.HeaderPosition = ""
	message.BulletPoints = make([]MessageBulletPoint, 6)
	if err := ValidateMessage(message); err == nil || !strings.Contains(err.Error(), "at most 5 bullet points") {
		t.Fatalf("ValidateMessage() error = %v", err)
	}
}

func testCredentials(t *testing.T) Credentials {
	t.Helper()
	t.Setenv("ASC_BYPASS_KEYCHAIN", "1")
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey() error = %v", err)
	}
	return Credentials{
		KeyID:         "STOREKIT_KEY",
		IssuerID:      "STOREKIT_ISSUER",
		PrivateKeyPEM: string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		BundleID:      "com.example.app",
	}
}

func TestWithTransportKeepsTheRequestTimeout(t *testing.T) {
	t.Setenv("ASC_TIMEOUT", "17s")
	credentials := testCredentials(t)
	transport := &http.Transport{}
	withDefault, err := NewClient(credentials, Sandbox)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	client, err := NewClient(credentials, Sandbox, WithTransport(transport))
	if err != nil {
		t.Fatalf("NewClient(WithTransport) error: %v", err)
	}
	if client.httpClient.Transport != transport {
		t.Fatalf("transport = %v, want the injected transport", client.httpClient.Transport)
	}
	if client.httpClient.Timeout != 17*time.Second || withDefault.httpClient.Timeout != 17*time.Second {
		t.Fatalf("timeout = %v, default client timeout = %v, want 17s for both", client.httpClient.Timeout, withDefault.httpClient.Timeout)
	}
}
