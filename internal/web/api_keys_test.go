package web

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/handlertest"
)

func TestClientCreateAPIKeySendsTeamKeyPayload(t *testing.T) {
	var requestBody map[string]any
	var requestMethod, requestPath, requestCSRF string
	var requestDecodeErr error
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		requestCSRF = r.Header.Get("X-CSRF-ITC")
		requestDecodeErr = json.NewDecoder(r.Body).Decode(&requestBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
					"data": {
						"type": "apiKeys",
						"id": "ABC123XYZ",
						"attributes": {
							"nickname": "Release automation",
							"roles": ["APP_MANAGER"],
							"allAppsVisible": true,
							"canDownload": true,
							"isActive": true,
							"keyType": "PUBLIC_API"
						}
					}
				}`))
	}))

	key, err := client.CreateAPIKey(context.Background(), APIKeyCreateAttributes{
		Nickname: "Release automation",
		Role:     "APP_MANAGER",
	})
	if err != nil {
		t.Fatalf("CreateAPIKey() error: %v", err)
	}
	if requestMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", requestMethod)
	}
	if requestPath != "/iris/v1/apiKeys" {
		t.Fatalf("expected /iris/v1/apiKeys, got %s", requestPath)
	}
	if requestCSRF != "[asc-ui]" {
		t.Fatalf("expected integrations CSRF header, got %q", requestCSRF)
	}
	if requestDecodeErr != nil {
		t.Fatalf("decode request: %v", requestDecodeErr)
	}
	if key.KeyID != "ABC123XYZ" || key.Name != "Release automation" {
		t.Fatalf("unexpected key: %#v", key)
	}

	data, ok := requestBody["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected JSON:API data object, got %#v", requestBody)
	}
	if data["type"] != "apiKeys" {
		t.Fatalf("expected apiKeys type, got %#v", data["type"])
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("expected attributes object, got %#v", data)
	}
	if attrs["nickname"] != "Release automation" || attrs["keyType"] != "PUBLIC_API" || attrs["allAppsVisible"] != true {
		t.Fatalf("unexpected attributes: %#v", attrs)
	}
	roles, ok := attrs["roles"].([]any)
	if !ok || len(roles) != 1 || roles[0] != "APP_MANAGER" {
		t.Fatalf("unexpected roles: %#v", attrs["roles"])
	}
}

func TestClientDownloadAPIKeyDecodesP8(t *testing.T) {
	p8 := generateP256PKCS8PEM(t)
	var requestMethod, requestPath, requestedField string
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestMethod = r.Method
		requestPath = r.URL.Path
		requestedField = r.URL.Query().Get("fields[apiKeys]")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(apiKeyDownloadJSON("ABC123XYZ", p8))
	}))

	got, err := client.DownloadAPIKey(context.Background(), "ABC123XYZ")
	if err != nil {
		t.Fatalf("DownloadAPIKey() error: %v", err)
	}
	if requestMethod != http.MethodGet {
		t.Fatalf("expected GET, got %s", requestMethod)
	}
	if requestPath != "/iris/v1/apiKeys/ABC123XYZ" {
		t.Fatalf("unexpected path %q", requestPath)
	}
	if requestedField != "privateKey" {
		t.Fatalf("unexpected private-key field %q", requestedField)
	}
	assertSamePKCS8Key(t, p8, got)
	assertErrorHasNoKeyMaterial(t, err, p8)
}

func TestClientDownloadAPIKeyNormalizesSurroundingWhitespace(t *testing.T) {
	p8 := generateP256PKCS8PEM(t)
	padded := append(append([]byte("   \t"), p8...), []byte("  \n")...)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(apiKeyDownloadJSON("ABC123XYZ", padded))
	}))

	got, err := client.DownloadAPIKey(context.Background(), "ABC123XYZ")
	if err != nil {
		t.Fatalf("DownloadAPIKey() error: %v", err)
	}
	assertSamePKCS8Key(t, p8, got)
	if bytes.Contains(got, []byte("   \t")) {
		t.Fatal("expected leading whitespace to be stripped from persisted P8")
	}
	assertErrorHasNoKeyMaterial(t, err, p8, padded)
}

func TestClientDownloadAPIKeyRejectsInvalidP8Payloads(t *testing.T) {
	valid := generateP256PKCS8PEM(t)
	truncated := truncatedPKCS8PEM(t, valid)
	rsaKey := generateRSAPKCS8PEM(t)
	p384 := generateP384PKCS8PEM(t)
	multi := append(append([]byte{}, valid...), valid...)
	trailing := append(append([]byte{}, valid...), []byte("trailing-not-a-block\n")...)
	marker := []byte("-----BEGIN PRIVATE KEY-----\nfixture-secret\n-----END PRIVATE KEY-----\n")

	tests := []struct {
		name string
		id   string
		p8   []byte
	}{
		{name: "truncated", id: "ABC123XYZ", p8: truncated},
		{name: "non-pem", id: "ABC123XYZ", p8: []byte("not a key")},
		{name: "non-pkcs8 marker", id: "ABC123XYZ", p8: marker},
		{name: "rsa key type", id: "ABC123XYZ", p8: rsaKey},
		{name: "p384 key type", id: "ABC123XYZ", p8: p384},
		{name: "multi-block", id: "ABC123XYZ", p8: multi},
		{name: "trailing data", id: "ABC123XYZ", p8: trailing},
		{name: "leading data", id: "ABC123XYZ", p8: append(append([]byte{}, []byte("leading-junk\n")...), valid...)},
		{name: "mismatched resource id", id: "OTHERKEY", p8: valid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(apiKeyDownloadJSON(tt.id, tt.p8))
			}))
			got, err := client.DownloadAPIKey(context.Background(), "ABC123XYZ")
			if !errors.Is(err, ErrAPIKeyResponseInvalid) {
				t.Fatalf("expected invalid P8 response error, got %v", err)
			}
			if got != nil {
				t.Fatalf("expected no decoded P8, got %d bytes", len(got))
			}
			assertErrorHasNoKeyMaterial(t, err, valid, tt.p8)
		})
	}
}

func TestClientGetAPIKeyParsesIssuerID(t *testing.T) {
	var includedResource string
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		includedResource = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
					"data": {
						"type": "apiKeys",
						"id": "ABC123XYZ",
						"attributes": {"nickname":"Release automation","roles":["ADMIN"],"allAppsVisible":true,"isActive":true,"keyType":"PUBLIC_API"},
						"relationships": {"provider":{"data":{"type":"contentProviders","id":"69a6de00-aaaa-bbbb-cccc-123456789abc"}}}
					}
				}`))
	}))

	key, err := client.GetAPIKey(context.Background(), "ABC123XYZ")
	if err != nil {
		t.Fatalf("GetAPIKey() error: %v", err)
	}
	if includedResource != "provider" {
		t.Fatalf("expected provider include, got %q", includedResource)
	}
	if key.IssuerID != "69a6de00-aaaa-bbbb-cccc-123456789abc" {
		t.Fatalf("unexpected issuer ID %q", key.IssuerID)
	}
}

func TestClientListAPIKeysByKindUsesOnlyRequestedEndpoint(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	tests := []struct {
		name           string
		kind           string
		path           string
		wantInclude    string
		wantVisibleApp string
	}{
		{
			name:        "team",
			kind:        APIKeyKindTeam,
			path:        "/iris/v1/apiKeys",
			wantInclude: "createdBy,revokedBy,provider",
		},
		{
			name:           "individual",
			kind:           APIKeyKindIndividual,
			path:           "/iris/v2/apiKeys",
			wantInclude:    "visibleApps,createdByActor,revokedByActor",
			wantVisibleApp: "3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []string
			client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests = append(requests, r.URL.Path)
				if r.URL.Path != tt.path {
					t.Fatalf("unexpected endpoint %s %s", r.Method, r.URL.Path)
				}
				if got := r.URL.Query().Get("include"); got != tt.wantInclude {
					t.Fatalf("unexpected include %q", got)
				}
				if tt.wantVisibleApp != "" {
					if got := r.URL.Query().Get("limit[visibleApps]"); got != tt.wantVisibleApp {
						t.Fatalf("unexpected visible-app limit %q", got)
					}
					if got := r.URL.Query().Get("limit"); got != "2000" {
						t.Fatalf("unexpected individual limit %q", got)
					}
				}
				if tt.kind == APIKeyKindTeam {
					if got := r.URL.Query().Get("sort"); got != "-isActive,-revokingDate" {
						t.Fatalf("unexpected team sort %q", got)
					}
					if got := r.URL.Query().Get("limit"); got != "2000" {
						t.Fatalf("unexpected team limit %q", got)
					}
				}
				_, _ = w.Write([]byte(`{"data":[{"id":"KEY123","attributes":{"nickname":"Example","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}]}`))
			}))

			keys, err := client.ListAPIKeysByKind(context.Background(), tt.kind)
			if err != nil {
				t.Fatalf("ListAPIKeysByKind() error: %v", err)
			}
			if len(requests) != 1 || requests[0] != tt.path {
				t.Fatalf("expected one %s request, got %#v", tt.kind, requests)
			}
			if len(keys) != 1 || keys[0].KeyID != "KEY123" || keys[0].Kind != tt.kind || !keys[0].Active {
				t.Fatalf("unexpected keys: %#v", keys)
			}
		})
	}
}

func TestClientListAPIKeysByKindRejectsUnknownTypeBeforeHTTP(t *testing.T) {
	called := false
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	_, err := client.ListAPIKeysByKind(context.Background(), "other")
	if err == nil || !strings.Contains(err.Error(), "api key type must be team or individual") {
		t.Fatalf("expected invalid type error, got %v", err)
	}
	if called {
		t.Fatal("did not expect HTTP for an invalid key type")
	}
}

func TestClientRevokeAPIKeySendsTypeSpecificPatch(t *testing.T) {
	t.Setenv("ASC_WEB_MIN_REQUEST_INTERVAL", "0")
	tests := []struct {
		name string
		kind string
		path string
	}{
		{name: "team", kind: APIKeyKindTeam, path: "/iris/v1/apiKeys/KEY123"},
		{name: "individual", kind: APIKeyKindIndividual, path: "/iris/v2/apiKeys/KEY123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestBody map[string]any
			var requestMethod, requestPath string
			client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestMethod = r.Method
				requestPath = r.URL.Path
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					t.Fatalf("decode request body: %v", err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))

			if err := client.RevokeAPIKey(context.Background(), "KEY123", tt.kind); err != nil {
				t.Fatalf("RevokeAPIKey() error: %v", err)
			}
			if requestMethod != http.MethodPatch || requestPath != tt.path {
				t.Fatalf("unexpected request %s %s, want PATCH %s", requestMethod, requestPath, tt.path)
			}
			data, ok := requestBody["data"].(map[string]any)
			if !ok {
				t.Fatalf("expected JSON:API data object, got %#v", requestBody)
			}
			if data["id"] != "KEY123" || data["type"] != "apiKeys" {
				t.Fatalf("unexpected resource identity: %#v", data)
			}
			attrs, ok := data["attributes"].(map[string]any)
			if !ok || attrs["isActive"] != false {
				t.Fatalf("unexpected revoke attributes: %#v", data["attributes"])
			}
		})
	}
}

func TestClientRevokeAPIKeyValidatesBeforeHTTP(t *testing.T) {
	called := false
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tt := range []struct {
		name string
		id   string
		kind string
		want string
	}{
		{name: "missing id", id: " ", kind: APIKeyKindTeam, want: "api key id is required"},
		{name: "invalid type", id: "KEY123", kind: "other", want: "api key type must be team or individual"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := client.RevokeAPIKey(context.Background(), tt.id, tt.kind)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
	if called {
		t.Fatal("did not expect HTTP for invalid revoke input")
	}
}

func TestClientListAPIKeysCombinesTeamAndIndividualKeys(t *testing.T) {
	fixture := handlertest.New(t)
	var teamPath, individualPath string
	var teamInclude, individualInclude string
	p8 := generateP256PKCS8PEM(t)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/iris/v1/apiKeys":
			teamPath = r.URL.Path
			teamInclude = r.URL.Query().Get("include")
			_, _ = w.Write([]byte(`{
				"data":[{
					"id":"ABC123XYZ",
					"attributes":{
						"nickname":"Release automation",
						"roles":["ADMIN"],
						"isActive":true,
						"keyType":"PUBLIC_API",
						"lastUsed":"2026-03-15T11:48:57.844-07:00",
						"privateKey":"` + base64.StdEncoding.EncodeToString(p8) + `"
					},
					"relationships":{"createdBy":{"data":{"id":"user-1"}}}
				}],
				"included":[{"type":"users","id":"user-1","attributes":{"firstName":"Ada","lastName":"Lovelace"}}]
			}`))
		case r.Method == http.MethodGet && r.URL.Path == "/iris/v2/apiKeys":
			individualPath = r.URL.Path
			individualInclude = r.URL.Query().Get("include")
			_, _ = w.Write([]byte(`{
				"data":[{
					"id":"IND456ABC",
					"attributes":{"nickname":"Personal","roles":["APP_MANAGER"],"isActive":false,"keyType":"PUBLIC_API"},
					"relationships":{"createdByActor":{"data":{"id":"actor-1"}}}
				}]
			}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys() error: %v", err)
	}
	if teamPath != "/iris/v1/apiKeys" {
		t.Fatalf("expected team list path /iris/v1/apiKeys, got %q", teamPath)
	}
	if individualPath != "/iris/v2/apiKeys" {
		t.Fatalf("expected individual list path /iris/v2/apiKeys, got %q", individualPath)
	}
	if teamInclude != "createdBy,revokedBy,provider" {
		t.Fatalf("unexpected team include %q", teamInclude)
	}
	if individualInclude != "visibleApps,createdByActor,revokedByActor" {
		t.Fatalf("unexpected individual include %q", individualInclude)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %#v", keys)
	}
	if keys[0].KeyID != "ABC123XYZ" || keys[0].Kind != APIKeyKindTeam || keys[0].Name != "Release automation" {
		t.Fatalf("unexpected team key: %#v", keys[0])
	}
	if !keys[0].Active || len(keys[0].Roles) != 1 || keys[0].Roles[0] != "ADMIN" {
		t.Fatalf("unexpected team key state: %#v", keys[0])
	}
	if keys[0].GeneratedBy == nil || keys[0].GeneratedBy.Name != "Ada Lovelace" {
		t.Fatalf("unexpected generatedBy: %#v", keys[0].GeneratedBy)
	}
	if keys[1].KeyID != "IND456ABC" || keys[1].Kind != APIKeyKindIndividual || keys[1].Active {
		t.Fatalf("unexpected individual key: %#v", keys[1])
	}
	if keys[1].GeneratedBy == nil || keys[1].GeneratedBy.ID != "actor-1" {
		t.Fatalf("unexpected individual generatedBy: %#v", keys[1].GeneratedBy)
	}
	assertNoKeyMaterialInAPIKeys(t, p8, keys)
}

func TestClientListAPIKeysFallsBackWhenTeamKeysForbidden(t *testing.T) {
	fixture := handlertest.New(t)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/iris/v1/apiKeys":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"Forbidden"}]}`))
		case "/iris/v2/apiKeys":
			_, _ = w.Write([]byte(`{"data":[{"id":"IND456ABC","attributes":{"nickname":"Personal","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}]}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if err != nil {
		t.Fatalf("ListAPIKeys() error: %v", err)
	}
	if len(keys) != 1 || keys[0].KeyID != "IND456ABC" || keys[0].Kind != APIKeyKindIndividual {
		t.Fatalf("expected individual-only fallback, got %#v", keys)
	}
}

func TestClientListAPIKeysPropagatesIndividualPaginationForbidden(t *testing.T) {
	fixture := handlertest.New(t)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/iris/v1/apiKeys":
			_, _ = w.Write([]byte(`{"data":[{"id":"ABC123XYZ","attributes":{"nickname":"Release automation","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}]}`))
		case r.URL.Path == "/iris/v2/apiKeys" && r.URL.Query().Get("cursor") == "page-2":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"Forbidden"}]}`))
		case r.URL.Path == "/iris/v2/apiKeys":
			_, _ = w.Write([]byte(`{
				"data":[{"id":"IND456ABC","attributes":{"nickname":"Personal","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}],
				"links":{"next":"https://appstoreconnect.apple.com/iris/v2/apiKeys?cursor=page-2"}
			}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if keys != nil {
		t.Fatalf("expected no keys when individual pagination fails, got %#v", keys)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected individual pagination 403, got %v", err)
	}
	if !errors.Is(err, errAPIKeyListPagination) {
		t.Fatalf("expected pagination sentinel, got %v", err)
	}
}

func TestClientListAPIKeysPropagatesTeamPaginationNotFound(t *testing.T) {
	fixture := handlertest.New(t)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/iris/v1/apiKeys" && r.URL.Query().Get("cursor") == "page-2":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"status":"404","title":"Not Found"}]}`))
		case r.URL.Path == "/iris/v1/apiKeys":
			_, _ = w.Write([]byte(`{
				"data":[{"id":"ABC123XYZ","attributes":{"nickname":"Release automation","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}],
				"links":{"next":"https://appstoreconnect.apple.com/iris/v1/apiKeys?cursor=page-2"}
			}`))
		case r.URL.Path == "/iris/v2/apiKeys":
			_, _ = w.Write([]byte(`{"data":[{"id":"IND456ABC","attributes":{"nickname":"Personal","roles":["ADMIN"],"isActive":true,"keyType":"PUBLIC_API"}}]}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if keys != nil {
		t.Fatalf("expected no keys when team pagination fails, got %#v", keys)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("expected team pagination 404, got %v", err)
	}
	if !errors.Is(err, errAPIKeyListPagination) {
		t.Fatalf("expected pagination sentinel, got %v", err)
	}
}

func TestClientListAPIKeysReturnsIndividualErrorAfterTeamFallback(t *testing.T) {
	fixture := handlertest.New(t)
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/iris/v1/apiKeys":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"Forbidden"}]}`))
		case "/iris/v2/apiKeys":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"status":"500","title":"Internal Server Error"}]}`))
		default:
			fixture.Respond(w, "unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if keys != nil {
		t.Fatalf("expected no keys on error, got %#v", keys)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusInternalServerError {
		t.Fatalf("expected individual-list 500 after team fallback, got %v", err)
	}
}

func TestClientListAPIKeysErrorsWhenBothListsForbidden(t *testing.T) {
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"Forbidden"}]}`))
	}))

	keys, err := client.ListAPIKeys(context.Background())
	if err == nil {
		t.Fatal("expected error when both key lists are forbidden")
	}
	if keys != nil {
		t.Fatalf("expected no keys on error, got %#v", keys)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusForbidden {
		t.Fatalf("expected forbidden API error, got %v", err)
	}
}

func TestClientGetAPIKeyRequiresKeyID(t *testing.T) {
	called := false
	client := newAPIKeyHTTPTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"id":"x"}}`))
	}))

	_, err := client.GetAPIKey(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for empty key id")
	}
	if called {
		t.Fatal("did not expect HTTP for an empty key id")
	}
}

func TestIsAPIKeyDownloadRetryable(t *testing.T) {
	if IsAPIKeyDownloadRetryable(nil) {
		t.Fatal("expected nil error not to be retryable")
	}
	if IsAPIKeyDownloadRetryable(fmt.Errorf("temporary transport failure")) {
		t.Fatal("expected generic transport error not to be retryable")
	}
	if IsAPIKeyDownloadRetryable(fmt.Errorf("download failed: %w", ErrAPIKeyResponseInvalid)) {
		t.Fatal("expected invalid download response not to be retryable")
	}
	for _, status := range []int{http.StatusNotFound, http.StatusConflict, http.StatusTooManyRequests, http.StatusBadGateway} {
		if !IsAPIKeyDownloadRetryable(&APIError{Status: status}) {
			t.Fatalf("expected status %d to be retryable", status)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden} {
		if IsAPIKeyDownloadRetryable(&APIError{Status: status}) {
			t.Fatalf("expected status %d not to be retryable", status)
		}
	}
}

type apiKeyRewriteTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (t apiKeyRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	requestURL := *request.URL
	requestURL.Scheme = t.target.Scheme
	requestURL.Host = t.target.Host
	clone.URL = &requestURL
	return t.base.RoundTrip(clone)
}

func generateP256PKCS8PEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-256 key: %v", err)
	}
	return marshalPKCS8PEM(t, key)
}

func generateP384PKCS8PEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		t.Fatalf("generate P-384 key: %v", err)
	}
	return marshalPKCS8PEM(t, key)
}

func generateRSAPKCS8PEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return marshalPKCS8PEM(t, key)
}

func marshalPKCS8PEM(t *testing.T, key any) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal PKCS#8: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

func truncatedPKCS8PEM(t *testing.T, valid []byte) []byte {
	t.Helper()
	block, _ := pem.Decode(valid)
	if block == nil || len(block.Bytes) < 8 {
		t.Fatal("expected a decodable PKCS#8 PEM fixture")
	}
	truncated := append([]byte{}, block.Bytes[:len(block.Bytes)/2]...)
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: truncated})
}

func apiKeyDownloadJSON(id string, p8 []byte) []byte {
	encoded := base64.StdEncoding.EncodeToString(p8)
	return []byte(`{"data":{"type":"apiKeys","id":"` + id + `","attributes":{"privateKey":"` + encoded + `"}}}`)
}

func assertSamePKCS8Key(t *testing.T, want, got []byte) {
	t.Helper()
	wantBlock, _ := pem.Decode(want)
	gotBlock, rest := pem.Decode(got)
	if wantBlock == nil || gotBlock == nil {
		t.Fatal("expected PKCS#8 PEM blocks")
	}
	if len(bytes.TrimSpace(rest)) > 0 {
		t.Fatal("expected returned P8 to decode without trailing junk")
	}
	if gotBlock.Type != "PRIVATE KEY" {
		t.Fatalf("unexpected PEM type %q", gotBlock.Type)
	}
	if !bytes.Equal(wantBlock.Bytes, gotBlock.Bytes) {
		t.Fatalf("returned P8 DER does not match validated key")
	}
}

func assertErrorHasNoKeyMaterial(t *testing.T, err error, payloads ...[]byte) {
	t.Helper()
	text := ""
	if err != nil {
		text = err.Error()
	}
	for _, payload := range payloads {
		assertNoKeyMaterial(t, payload, text)
	}
}

func assertNoKeyMaterial(t *testing.T, p8 []byte, outputs ...string) {
	t.Helper()
	if len(p8) == 0 {
		return
	}
	full := strings.TrimSpace(string(p8))
	block, _ := pem.Decode(p8)
	for _, out := range outputs {
		if out == "" {
			continue
		}
		if full != "" && strings.Contains(out, full) {
			t.Fatal("output contained P8 contents")
		}
		if strings.Contains(out, "-----BEGIN PRIVATE KEY-----") || strings.Contains(out, "-----END PRIVATE KEY-----") {
			t.Fatal("output contained PEM boundary")
		}
		if block != nil && len(block.Bytes) > 0 {
			if strings.Contains(out, base64.StdEncoding.EncodeToString(block.Bytes)) {
				t.Fatal("output contained PKCS#8 DER")
			}
		}
	}
}

func assertNoKeyMaterialInAPIKeys(t *testing.T, p8 []byte, keys []APIKeyListItem) {
	t.Helper()
	encoded, err := json.Marshal(keys)
	if err != nil {
		t.Fatalf("marshal listed keys: %v", err)
	}
	assertNoKeyMaterial(t, p8, string(encoded))
}

func newAPIKeyHTTPTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	return &Client{
		httpClient: &http.Client{Transport: apiKeyRewriteTransport{
			target: target,
			base:   server.Client().Transport,
		}},
	}
}
