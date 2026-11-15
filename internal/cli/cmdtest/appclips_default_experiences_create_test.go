package cmdtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/asc"
)

// newDefaultExperiencesCreateClient returns a client factory whose clients send
// every request to srv.
func newDefaultExperiencesCreateClient(t *testing.T, srv *httptest.Server) func() (*asc.Client, error) {
	t.Helper()

	serverURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}

	keyPath := t.TempDir() + "/key.p8"
	writeECDSAPEM(t, keyPath)

	transport := appClipsRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		cloned := req.Clone(req.Context())
		cloned.URL.Scheme = serverURL.Scheme
		cloned.URL.Host = serverURL.Host
		return srv.Client().Transport.RoundTrip(cloned)
	})
	client, err := asc.NewClientWithHTTPClient("TEST_KEY", "TEST_ISSUER", keyPath, &http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return func() (*asc.Client, error) {
		return client, nil
	}
}

func TestAppClipsDefaultExperiencesCreateSendsTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/appClipDefaultExperiences" {
			t.Fatalf("expected path /v1/appClipDefaultExperiences, got %s", r.URL.Path)
		}

		var payload asc.AppClipDefaultExperienceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		template := payload.Data.Relationships.AppClipDefaultExperienceTemplate
		if template == nil {
			t.Fatal("expected appClipDefaultExperienceTemplate relationship")
		}
		if template.Data.ID != "exp-current" {
			t.Fatalf("expected template id exp-current, got %s", template.Data.ID)
		}
		if template.Data.Type != asc.ResourceTypeAppClipDefaultExperiences {
			t.Fatalf("expected template type %s, got %s", asc.ResourceTypeAppClipDefaultExperiences, template.Data.Type)
		}
		if payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID != "version-1" {
			t.Fatalf("expected releaseWithAppStoreVersion id version-1, got %s", payload.Data.Relationships.ReleaseWithAppStoreVersion.Data.ID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appClipDefaultExperiences","id":"exp-new"},"links":{}}`))
	}))
	defer server.Close()

	client := newDefaultExperiencesCreateClient(t, server)

	stdout, stderr, err := runRootWithDeps(t, client,
		"app-clips", "default-experiences", "create",
		"--app-clip-id", "clip-1",
		"--release-version-id", "version-1",
		"--template-id", "exp-current",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}

	var response asc.AppClipDefaultExperienceResponse
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatalf("unmarshal stdout: %v\nstdout=%q", err, stdout)
	}
	if response.Data.ID != "exp-new" {
		t.Fatalf("expected experience id exp-new, got %s", response.Data.ID)
	}
}

func TestAppClipsDefaultExperiencesCreateOmitsTemplateWhenNotSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload asc.AppClipDefaultExperienceCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload.Data.Relationships.AppClipDefaultExperienceTemplate != nil {
			t.Fatalf("expected no template relationship, got %#v", payload.Data.Relationships.AppClipDefaultExperienceTemplate)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appClipDefaultExperiences","id":"exp-new"},"links":{}}`))
	}))
	defer server.Close()

	client := newDefaultExperiencesCreateClient(t, server)

	_, stderr, err := runRootWithDeps(t, client,
		"app-clips", "default-experiences", "create",
		"--app-clip-id", "clip-1",
		"--release-version-id", "version-1",
	)
	if err != nil {
		t.Fatalf("run error: %v", err)
	}

	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestAppClipsDefaultExperiencesCreateRejectsEmptyTemplateWhenSet(t *testing.T) {
	for _, templateID := range []string{"", "   "} {
		t.Run("value="+templateID, func(t *testing.T) {
			requestCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requestCount++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"data":{"type":"appClipDefaultExperiences","id":"exp-new"},"links":{}}`))
			}))
			defer server.Close()

			client := newDefaultExperiencesCreateClient(t, server)

			_, _, runErr := runRootWithDeps(t, client,
				"app-clips", "default-experiences", "create",
				"--app-clip-id", "clip-1",
				"--release-version-id", "version-1",
				"--template-id", templateID,
			)

			if runErr == nil {
				t.Fatal("expected error for explicitly empty --template-id")
			}
			if !strings.Contains(runErr.Error(), "--template-id must not be empty") {
				t.Fatalf("expected empty template error, got %v", runErr)
			}
			if requestCount != 0 {
				t.Fatalf("expected no request, got %d", requestCount)
			}
		})
	}
}
