package cmdtest

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	cmd "github.com/rudrankriyam/App-Store-Connect-CLI/cmd"
	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

func TestAppClipsMutationsRejectRepeatedCSVFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		flagName string
	}{
		{
			name: "review details create urls",
			args: []string{
				"app-clips", "review-details", "create",
				"--experience-id", "experience-1",
				"--url", "https://example.com/one",
				"--url", "https://example.com/two",
			},
			flagName: "url",
		},
		{
			name: "review details update urls",
			args: []string{
				"app-clips", "review-details", "update",
				"--id", "detail-1",
				"--url", "https://example.com/one",
				"--url", "https://example.com/two",
			},
			flagName: "url",
		},
		{
			name: "invocation localization ids",
			args: []string{
				"app-clips", "invocations", "create",
				"--build-bundle-id", "bundle-1",
				"--url", "https://example.com/clip",
				"--localization-id", "loc-1",
				"--localization-id", "loc-2",
			},
			flagName: "localization-id",
		},
		{
			name: "advanced experience create localization ids",
			args: []string{
				"app-clips", "advanced-experiences", "create",
				"--app-clip-id", "clip-1",
				"--link", "https://example.com/clip",
				"--default-language", "EN",
				"--is-powered-by",
				"--header-image-id", "image-1",
				"--localization-id", "loc-1",
				"--localization-id", "loc-2",
			},
			flagName: "localization-id",
		},
		{
			name: "advanced experience update localization ids",
			args: []string{
				"app-clips", "advanced-experiences", "update",
				"--experience-id", "experience-1",
				"--localization-id", "loc-1",
				"--localization-id", "loc-2",
			},
			flagName: "localization-id",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			factoryCalls := 0
			clientFactory := func() (*asc.Client, error) {
				factoryCalls++
				return nil, errors.New("unexpected client creation")
			}

			stdout, stderr, code := runWithDeps(t, clientFactory, test.args...)
			if code != cmd.ExitUsage {
				t.Fatalf("Run() exit code = %d, want %d", code, cmd.ExitUsage)
			}

			if factoryCalls != 0 {
				t.Fatalf("client factory calls = %d, want 0 before repeated-flag rejection", factoryCalls)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			want := "--" + test.flagName + " specified multiple times"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want repeated-flag diagnostic containing %q", stderr, want)
			}
		})
	}
}

func TestAppClipsReviewDetailsCSVValuesPreserveOrder(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		method string
		path   string
		status int
	}{
		{
			name: "create",
			args: []string{
				"app-clips", "review-details", "create",
				"--experience-id", "experience-1",
				"--url", "https://example.com/one,https://example.com/two",
			},
			method: http.MethodPost,
			path:   "/v1/appClipAppStoreReviewDetails",
			status: http.StatusCreated,
		},
		{
			name: "update",
			args: []string{
				"app-clips", "review-details", "update",
				"--id", "detail-1",
				"--url", "https://example.com/one,https://example.com/two",
			},
			method: http.MethodPatch,
			path:   "/v1/appClipAppStoreReviewDetails/detail-1",
			status: http.StatusOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != test.method || req.URL.Path != test.path {
					t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				}

				var payload struct {
					Data struct {
						Attributes struct {
							InvocationURLs []string `json:"invocationUrls"`
						} `json:"attributes"`
					} `json:"data"`
				}
				if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				want := []string{"https://example.com/one", "https://example.com/two"}
				if len(payload.Data.Attributes.InvocationURLs) != len(want) {
					t.Fatalf("invocation URL count = %d, want %d", len(payload.Data.Attributes.InvocationURLs), len(want))
				}
				for index, value := range want {
					if payload.Data.Attributes.InvocationURLs[index] != value {
						t.Fatalf("invocation URL %d = %q, want %q", index, payload.Data.Attributes.InvocationURLs[index], value)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(test.status)
				_, _ = io.WriteString(w, `{"data":{"type":"appClipAppStoreReviewDetails","id":"detail-1","attributes":{"invocationUrls":["https://example.com/one","https://example.com/two"]}}}`)
			}))
			t.Cleanup(server.Close)

			keyPath := t.TempDir() + "/AuthKey.p8"
			writeECDSAPEM(t, keyPath)
			client, err := newAppClipsTestServerClient(t, server, keyPath)
			if err != nil {
				t.Fatalf("new client: %v", err)
			}
			clientFactory := func() (*asc.Client, error) {
				return client, nil
			}

			stdout, stderr, err := runRootWithDeps(t, clientFactory, test.args...)
			if err != nil {
				t.Fatalf("run error: %v", err)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if !strings.Contains(stdout, `"id":"detail-1"`) {
				t.Fatalf("expected detail ID in stdout, got %q", stdout)
			}
		})
	}
}

func TestAppClipsInvocationsCreateCSVValuesRemainCompatible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost || req.URL.Path != "/v1/betaAppClipInvocations" {
			t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
		}
		var payload asc.BetaAppClipInvocationCreateRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		ids := payload.Data.Relationships.BetaAppClipInvocationLocalizations.Data
		if len(ids) != 2 || ids[0].ID != "loc-1" || ids[1].ID != "loc-2" {
			t.Fatalf("localization IDs = %#v, want loc-1, loc-2", ids)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"data":{"type":"betaAppClipInvocations","id":"inv-1","attributes":{"url":"https://example.com/clip"}}}`)
	}))
	t.Cleanup(server.Close)

	keyPath := t.TempDir() + "/AuthKey.p8"
	writeECDSAPEM(t, keyPath)
	client, err := newAppClipsTestServerClient(t, server, keyPath)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	clientFactory := func() (*asc.Client, error) {
		return client, nil
	}

	stdout, stderr, err := runRootWithDeps(t, clientFactory,
		"app-clips", "invocations", "create",
		"--build-bundle-id", "bundle-1",
		"--url", "https://example.com/clip",
		"--localization-id", "loc-1,loc-2",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
	if !strings.Contains(stdout, `"id":"inv-1"`) {
		t.Fatalf("expected invocation ID in stdout, got %q", stdout)
	}
}

func newAppClipsTestServerClient(t *testing.T, server *httptest.Server, keyPath string) (*asc.Client, error) {
	t.Helper()
	serverURL, err := url.Parse(server.URL)
	if err != nil {
		return nil, err
	}
	transport := appClipsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return server.Client().Transport.RoundTrip(cloned)
	})
	return asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
}
