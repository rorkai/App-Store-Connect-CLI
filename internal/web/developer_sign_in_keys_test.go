package web

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
	"net/url"
	"strings"
	"testing"
)

func TestDeveloperSignInKeyLifecycle(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	p8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	primed := false
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"team"}, "csrf_ts": {"team-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/list":
			body, _ := io.ReadAll(r.Body)
			form, _ := url.ParseQuery(string(body))
			if form.Get("teamId") != "TEAM123456" || form.Get("pageSize") != "1000" {
				t.Fatalf("wrong list body %s", body)
			}
			primed = true
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[],"unknown":"preserved"}`, http.Header{"csrf": {"key"}, "csrf_ts": {"key-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/v2/create":
			if !primed || r.Method != "POST" || r.Header.Get("csrf") != "key" || r.Header.Get("Content-Type") != "application/json" {
				t.Fatal("create not key-CSRF primed")
			}
			var payload struct {
				TeamID   string `json:"teamId"`
				Name     string `json:"name"`
				Services []struct {
					IsNew       bool   `json:"isNew"`
					ID          string `json:"serviceId"`
					Identifiers struct {
						Bundle []string `json:"bundle"`
					} `json:"identifiers"`
				} `json:"serviceConfigurationsRequests"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.TeamID != "TEAM123456" || payload.Name != "Sway" || len(payload.Services) != 1 || payload.Services[0].ID != signInKeyService || !payload.Services[0].IsNew || len(payload.Services[0].Identifiers.Bundle) != 1 || payload.Services[0].Identifiers.Bundle[0] != "BUNDLE123" {
				t.Fatalf("wrong create payload %+v", payload)
			}
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123","keyName":"Sway","canDownload":true}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/get":
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123","keyName":"Sway","canDownload":true,"services":[{"id":"APPLE_ID_AUTH_KEY_CONFIGURATION","configurations":[{"id":"BUNDLE123","type":"bundle"}]}]}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/download":
			if r.Header.Get("Content-Type") != "" {
				t.Fatal("download GET must not set Content-Type")
			}
			if r.Method != "GET" || r.URL.Query().Get("keyId") != "KEY123" || r.URL.Query().Get("teamId") != "TEAM123456" {
				t.Fatal("wrong download request")
			}
			return developerPortalTestResponse(200, p8, nil), nil
		default:
			t.Fatalf("unexpected %s", r.URL)
			return nil, nil
		}
	})
	created, err := client.CreateDeveloperSignInKey(context.Background(), "Sway", "BUNDLE123")
	if err != nil {
		t.Fatal(err)
	}
	if created.KeyID != "KEY123" {
		t.Fatal("wrong created key")
	}
	downloaded, err := client.DownloadDeveloperSignInKey(context.Background(), created.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != p8 {
		t.Fatal("download mismatch")
	}
}

func TestDeveloperSignInKeyEnvelopeValidation(t *testing.T) {
	for _, raw := range []string{`{}`, `{"resultCode":1,"keys":[]}`, `{"resultCode":0}`, `{"resultCode":0,"keys":[{"keyId":"../../secret"}]}`} {
		if _, err := parseDeveloperSignInKeys([]byte(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	raw := `{"resultCode":0,"keys":[],"futureField":"kept"}`
	result, err := parseDeveloperSignInKeys([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	marshaled, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(marshaled), `"futureField":"kept"`) {
		t.Fatal("lost envelope")
	}
}

func TestDeveloperSignInKeyRejectsOtherServiceBeforeDownload(t *testing.T) {
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == developerPortalTeamsPath {
			return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"c"}, "csrf_ts": {"t"}}), nil
		}
		if strings.HasSuffix(r.URL.Path, "/list") {
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[]}`, http.Header{"csrf": {"c"}, "csrf_ts": {"t"}}), nil
		}
		if strings.HasSuffix(r.URL.Path, "/get") {
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123","canDownload":true,"services":[{"id":"APNS"}]}]}`, nil), nil
		}
		t.Fatalf("unexpected request %s", r.URL)
		return nil, nil
	})
	if _, err := client.DownloadDeveloperSignInKey(context.Background(), "KEY123"); err == nil || !strings.Contains(err.Error(), "not a Sign in") {
		t.Fatalf("expected service rejection, got %v", err)
	}
}

func TestDeveloperSignInKeyDownloadInvalid2xxRetainsRecoveryBody(t *testing.T) {
	const raw = "provider-error-secret"
	downloadCalls := 0
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"team"}, "csrf_ts": {"team-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/list":
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"keys":[{"keyId":"KEY123","canDownload":true}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/get":
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"keys":[{"keyId":"KEY123","canDownload":true,"services":[{"id":"APPLE_ID_AUTH_KEY_CONFIGURATION","configurations":[]}]}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/download":
			downloadCalls++
			return developerPortalTestResponse(http.StatusOK, raw, nil), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})

	_, err := client.DownloadDeveloperSignInKey(context.Background(), "KEY123")
	if err == nil {
		t.Fatal("expected malformed successful response to fail validation")
	}
	if !errors.Is(err, ErrAPIKeyResponseInvalid) {
		t.Fatalf("expected invalid response error, got %v", err)
	}
	var recovery interface {
		error
		RecoveryBody() []byte
	}
	if !errors.As(err, &recovery) {
		t.Fatalf("expected recoverable response error, got %T: %v", err, err)
	}
	if got := string(recovery.RecoveryBody()); got != raw {
		t.Fatalf("recovery body = %q, want %q", got, raw)
	}
	body := recovery.RecoveryBody()
	body[0] = 'X'
	if got := string(recovery.RecoveryBody()); got != raw {
		t.Fatalf("recovery body accessor did not return a copy: %q", got)
	}
	if downloadCalls != 1 {
		t.Fatalf("download calls = %d, want 1", downloadCalls)
	}
}

func TestDeveloperSignInKeyDownloadNon2xxHasNoRecoveryBody(t *testing.T) {
	const raw = "provider-error-secret"
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), http.Header{"csrf": {"team"}, "csrf_ts": {"team-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/list":
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"keys":[{"keyId":"KEY123","canDownload":true}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/get":
			return developerPortalTestResponse(http.StatusOK, `{"resultCode":0,"keys":[{"keyId":"KEY123","canDownload":true,"services":[{"id":"APPLE_ID_AUTH_KEY_CONFIGURATION","configurations":[]}]}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/download":
			return developerPortalTestResponse(http.StatusInternalServerError, raw, nil), nil
		default:
			t.Fatalf("unexpected request %s", r.URL)
			return nil, nil
		}
	})

	_, err := client.DownloadDeveloperSignInKey(context.Background(), "KEY123")
	if err == nil {
		t.Fatal("expected non-2xx response to fail")
	}
	var recovery interface {
		error
		RecoveryBody() []byte
	}
	if errors.As(err, &recovery) {
		t.Fatalf("non-2xx response unexpectedly exposed recovery body %q", recovery.RecoveryBody())
	}
	if errors.Is(err, ErrAPIKeyResponseInvalid) {
		t.Fatalf("non-2xx response was classified as malformed P8: %v", err)
	}
}

func TestDeveloperPushKeyCreateSendsTeamScopedAPNsRequestAndDownloads(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	p8 := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case developerPortalTeamsPath:
			return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"team"}, "csrf_ts": {"team-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/list":
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[]}`, http.Header{"csrf": {"key"}, "csrf_ts": {"key-ts"}}), nil
		case developerPortalLegacyPath + "/account/auth/key/v2/create":
			var payload struct {
				TeamID   string           `json:"teamId"`
				Name     string           `json:"name"`
				Services []map[string]any `json:"serviceConfigurationsRequests"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(payload.Services)
			want := `[{"environment":"all","identifiers":{},"isNew":true,"scope":"team","serviceId":"U27F4V844T"}]`
			if payload.TeamID != "TEAM123456" || payload.Name != "Push" || string(got) != want {
				t.Fatalf("wrong create payload team=%s name=%s services=%s", payload.TeamID, payload.Name, got)
			}
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123","keyName":"Push","canDownload":true}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/get":
			return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123","keyName":"Push","canDownload":true,"services":[{"name":"APNs","id":"U27F4V844T","configurations":[]}]}]}`, nil), nil
		case developerPortalLegacyPath + "/account/auth/key/download":
			return developerPortalTestResponse(200, p8, nil), nil
		default:
			t.Fatalf("unexpected %s", r.URL)
			return nil, nil
		}
	})
	created, err := client.CreateDeveloperPushKey(context.Background(), "Push")
	if err != nil {
		t.Fatal(err)
	}
	downloaded, err := client.DownloadDeveloperPushKey(context.Background(), created.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	if string(downloaded) != p8 {
		t.Fatal("download mismatch")
	}
	if _, err := client.DownloadDeveloperSignInKey(context.Background(), created.KeyID); err == nil || !strings.Contains(err.Error(), "not a Sign in with Apple key") {
		t.Fatalf("expected APNs key to be rejected by the Sign in with Apple download, got %v", err)
	}
}

func TestDeveloperKeyRevoke(t *testing.T) {
	const apnsKey = `{"resultCode":0,"keys":[{"keyId":"KEY123","canRevoke":true,"services":[{"id":"U27F4V844T","configurations":[]}]}]}`
	for _, tc := range []struct {
		name        string
		detail      string
		listAfter   string
		revoke      func(*Client) error
		wantRevoke  bool
		wantErrText string
	}{
		{name: "APNs key", detail: apnsKey, listAfter: `{"resultCode":0,"keys":[]}`, revoke: func(c *Client) error { return c.RevokeDeveloperPushKey(context.Background(), "KEY123") }, wantRevoke: true},
		{name: "wrong service", detail: apnsKey, revoke: func(c *Client) error { return c.RevokeDeveloperSignInKey(context.Background(), "KEY123") }, wantErrText: "not a Sign in with Apple key"},
		{name: "not revocable", detail: `{"resultCode":0,"keys":[{"keyId":"KEY123","canRevoke":false,"services":[{"id":"U27F4V844T"}]}]}`, revoke: func(c *Client) error { return c.RevokeDeveloperPushKey(context.Background(), "KEY123") }, wantErrText: "cannot be revoked"},
		{name: "still listed", detail: apnsKey, listAfter: `{"resultCode":0,"keys":[{"keyId":"KEY123"}]}`, revoke: func(c *Client) error { return c.RevokeDeveloperPushKey(context.Background(), "KEY123") }, wantRevoke: true, wantErrText: "still listed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			revoked := false
			client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
				switch r.URL.Path {
				case developerPortalTeamsPath:
					return developerPortalTestResponse(200, developerPortalTeamsFixture(), http.Header{"csrf": {"team"}, "csrf_ts": {"team-ts"}}), nil
				case developerPortalLegacyPath + "/account/auth/key/list":
					if revoked {
						return developerPortalTestResponse(200, tc.listAfter, nil), nil
					}
					return developerPortalTestResponse(200, `{"resultCode":0,"keys":[{"keyId":"KEY123"}]}`, http.Header{"csrf": {"key"}, "csrf_ts": {"key-ts"}}), nil
				case developerPortalLegacyPath + "/account/auth/key/get":
					return developerPortalTestResponse(200, tc.detail, nil), nil
				case developerPortalLegacyPath + "/account/auth/key/revoke":
					body, _ := io.ReadAll(r.Body)
					form, _ := url.ParseQuery(string(body))
					if r.Method != http.MethodPost || r.Header.Get("csrf") != "key" || form.Get("teamId") != "TEAM123456" || form.Get("keyId") != "KEY123" {
						t.Fatalf("wrong revoke request %s %s csrf=%q", r.Method, body, r.Header.Get("csrf"))
					}
					revoked = true
					return developerPortalTestResponse(200, `{"resultCode":0}`, nil), nil
				default:
					t.Fatalf("unexpected %s", r.URL)
					return nil, nil
				}
			})
			err := tc.revoke(client)
			if revoked != tc.wantRevoke {
				t.Fatalf("revoke sent = %t, want %t", revoked, tc.wantRevoke)
			}
			if tc.wantErrText == "" && err != nil {
				t.Fatal(err)
			}
			if tc.wantErrText != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErrText)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErrText)
			}
		})
	}
}
