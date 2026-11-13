package cmdtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
	appclipscli "github.com/rudrankriyam/App-Store-Connect-CLI/internal/cli/appclips"
)

func TestAppClipsInvocationsCreateSendsLocalizations(t *testing.T) {
	tests := []struct {
		name               string
		args               []string
		wantLocalizationID string
		wantIncluded       []string
	}{
		{
			name:               "inline localization",
			args:               []string{"--locale", "en-US", "--title", "Try it"},
			wantLocalizationID: "${localization-1}",
			wantIncluded:       []string{"en-US", "Try it"},
		},
		{
			name:               "existing localization id",
			args:               []string{"--localization-id", "loc-1"},
			wantLocalizationID: "loc-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v1/betaAppClipInvocations" {
					t.Fatalf("expected POST /v1/betaAppClipInvocations, got %s %s", r.Method, r.URL.Path)
				}

				var payload asc.BetaAppClipInvocationCreateRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatalf("decode payload: %v", err)
				}
				relationships := payload.Data.Relationships
				if relationships == nil || relationships.BuildBundle == nil || relationships.BuildBundle.Data.ID != "bundle-1" {
					t.Fatalf("expected build bundle relationship bundle-1, got %+v", relationships)
				}
				if relationships.BetaAppClipInvocationLocalizations == nil || len(relationships.BetaAppClipInvocationLocalizations.Data) != 1 {
					t.Fatalf("expected one localization relationship, got %+v", relationships.BetaAppClipInvocationLocalizations)
				}
				if got := relationships.BetaAppClipInvocationLocalizations.Data[0].ID; got != test.wantLocalizationID {
					t.Fatalf("expected localization id %s, got %s", test.wantLocalizationID, got)
				}
				if test.wantIncluded == nil {
					if len(payload.Included) != 0 {
						t.Fatalf("expected no included localizations, got %d", len(payload.Included))
					}
				} else if len(payload.Included) != 1 || payload.Included[0].Attributes.Locale != test.wantIncluded[0] || payload.Included[0].Attributes.Title != test.wantIncluded[1] {
					t.Fatalf("expected one included localization %v, got %+v", test.wantIncluded, payload.Included)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/clip"}},"links":{}}`))
			}))
			defer server.Close()

			serverURL, err := url.Parse(server.URL)
			if err != nil {
				t.Fatalf("parse server URL: %v", err)
			}
			keyPath := t.TempDir() + "/key.p8"
			writeECDSAPEM(t, keyPath)
			transport := appClipsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				cloned := req.Clone(req.Context())
				cloned.URL.Scheme = serverURL.Scheme
				cloned.URL.Host = serverURL.Host
				return server.Client().Transport.RoundTrip(cloned)
			})
			client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			restore := appclipscli.SetClientFactory(func() (*asc.Client, error) {
				return client, nil
			})
			t.Cleanup(restore)

			root := RootCommand("1.2.3")
			stdout, stderr := captureOutput(t, func() {
				if err := root.Parse(append([]string{
					"app-clips", "invocations", "create",
					"--build-bundle-id", "bundle-1",
					"--url", "https://example.com/clip",
				}, test.args...)); err != nil {
					t.Fatalf("parse error: %v", err)
				}
				if err := root.Run(context.Background()); err != nil {
					t.Fatalf("run error: %v", err)
				}
			})

			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			var response asc.BetaAppClipInvocationResponse
			if err := json.Unmarshal([]byte(stdout), &response); err != nil {
				t.Fatalf("unmarshal stdout: %v\nstdout=%q", err, stdout)
			}
			if response.Data.ID != "inv-1" {
				t.Fatalf("expected invocation id inv-1, got %s", response.Data.ID)
			}
		})
	}
}

type appClipsRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn appClipsRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
