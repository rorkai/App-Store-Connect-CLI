package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/appleauth"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/securitykey"
)

// Regression: Apple must receive the assertion for the exact signed client
// data, with raw authenticator bytes intact and no replay after submission.
func TestSubmitSecurityKeyAppleProtocol(t *testing.T) {
	for _, tt := range []struct {
		name         string
		verifyStatus int
		trustStatus  int
		signError    error
		wantError    string
		wantRequests []string
	}{
		{name: "accepted", verifyStatus: 204, trustStatus: 204, wantRequests: []string{"POST /appleauth/auth/verify/security/key", "GET /appleauth/auth/2sv/trust", "GET /olympus/v1/session"}},
		{name: "assertion rejected", verifyStatus: 401, wantError: "security-key verification rejected by Apple (HTTP 401)", wantRequests: []string{"POST /appleauth/auth/verify/security/key"}},
		{name: "accepted but trust failed", verifyStatus: 204, trustStatus: 401, wantError: "2fa trust failed with status 401", wantRequests: []string{"POST /appleauth/auth/verify/security/key", "GET /appleauth/auth/2sv/trust"}},
		{name: "signing cancelled", signError: context.Canceled, wantError: "context canceled"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			oldAvailable, oldSign := securityKeyAvailableFn, signSecurityKeyFn
			t.Cleanup(func() { securityKeyAvailableFn, signSecurityKeyFn = oldAvailable, oldSign })
			securityKeyAvailableFn = func() bool { return true }
			const clientData = `{"type":"webauthn.get","challenge":"-_8","origin":"https://idmsa.apple.com","crossOrigin":true}`
			const pin = "PIN-never-sent-to-Apple"
			credential := []byte{0xfb, 0xff, 1}
			authData := bytes.Repeat([]byte{0x41}, 48)
			signature := []byte{0xfb, 0xff, 2}
			signCalls := 0
			signSecurityKeyFn = func(ctx context.Context, request securitykey.Request, gotPIN string) (*securitykey.Assertion, error) {
				signCalls++
				if gotPIN != pin || request.RelyingParty != "apple.com" || request.ClientDataHash != sha256.Sum256([]byte(clientData)) || !reflect.DeepEqual(request.CredentialIDs, [][]byte{credential}) {
					t.Fatal("signer did not receive the Apple RP, exact client-data hash, decoded allow-list, and PIN")
				}
				if tt.signError != nil {
					return nil, tt.signError
				}
				return &securitykey.Assertion{CredentialID: credential, AuthenticatorData: authData, Signature: signature}, nil
			}
			var mu sync.Mutex
			var requests []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests = append(requests, r.Method+" "+r.URL.Path)
				mu.Unlock()
				switch r.URL.Path {
				case "/appleauth/auth/verify/security/key":
					for key, want := range map[string]string{"Content-Type": "application/json", "Accept": "application/json", "X-Apple-Widget-Key": "widget", "X-Apple-ID-Session-Id": "session", "scnt": "continuation"} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("header %s = %q, want %q", key, got, want)
						}
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					if bytes.Contains(body, []byte(pin)) {
						t.Error("PIN was sent to Apple")
					}
					var payload map[string]string
					if err := json.Unmarshal(body, &payload); err != nil {
						t.Error(err)
					}
					encode := base64.StdEncoding.EncodeToString
					want := map[string]string{"challenge": "+/8=", "clientData": encode([]byte(clientData)), "signatureData": encode(signature), "authenticatorData": encode(authData), "credentialID": "+/8B", "userHandle": "", "rpId": "apple.com"}
					if !reflect.DeepEqual(payload, want) {
						t.Errorf("Apple assertion payload = %#v, want %#v", payload, want)
					}
					w.WriteHeader(tt.verifyStatus)
				case "/appleauth/auth/2sv/trust":
					w.WriteHeader(tt.trustStatus)
				case "/olympus/v1/session":
					_, _ = io.WriteString(w, `{"provider":{"providerId":42,"publicProviderId":"public-team"},"user":{"emailAddress":"user@example.com"}}`)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			target, err := url.Parse(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			client := server.Client()
			transport := client.Transport
			client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				req := r.Clone(r.Context())
				req.URL.Scheme, req.URL.Host = target.Scheme, target.Host
				return transport.RoundTrip(req)
			})
			session := &AuthSession{
				Client: client, ServiceKey: "widget", AppleIDSessionID: "session", SCNT: "continuation",
				securityKeyChallenge: &appleauth.SecurityKeyChallenge{Challenge: "-_8", AllowedCredentials: "+/8B"},
			}
			err = SubmitSecurityKey(context.Background(), session, pin)
			if tt.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
				if session.ProviderID != 42 || session.PublicProviderID != "public-team" || session.UserEmail != "user@example.com" {
					t.Fatalf("accepted assertion did not populate ASC session: %+v", session)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("submit error = %v, want %q", err, tt.wantError)
			}
			if tt.signError != nil && !errors.Is(err, tt.signError) {
				t.Fatalf("signer cancellation was lost: %v", err)
			}
			if tt.name == "accepted but trust failed" {
				var finalization *TwoFactorFinalizationError
				if !errors.As(err, &finalization) || finalization.HTTPStatusCode() != 401 {
					t.Fatalf("accepted assertion was not distinguished from finalization failure: %v", err)
				}
			}
			if tt.signError == nil {
				if err := SubmitSecurityKey(context.Background(), session, pin); err == nil || err.Error() != "a prepared security-key challenge is required" {
					t.Fatalf("consumed assertion could be replayed: %v", err)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if !reflect.DeepEqual(requests, tt.wantRequests) || signCalls != 1 {
				t.Fatalf("requests = %v, want %v; signer calls = %d, want 1", requests, tt.wantRequests, signCalls)
			}
		})
	}
}
