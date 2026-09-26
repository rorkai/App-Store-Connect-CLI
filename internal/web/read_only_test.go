package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/rudrankriyam/App-Store-Connect-CLI/internal/readonly"
)

func TestReadOnlyModeRefusesIrisMutationsBeforeSending(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"apps","id":"123","attributes":{"name":"App"}}}`))
	}))
	t.Cleanup(server.Close)
	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/iris/v1"}

	_, err := client.DeleteApp(context.Background(), "123")
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("DeleteApp() error = %v, want readonly.ErrRefused", err)
	}
	want := "ASC_READ_ONLY is set; refusing PATCH " + server.URL + "/iris/v1/apps/123"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("DeleteApp() error = %q, want it to contain %q", err.Error(), want)
	}
	if got := sent.Load(); got != 0 {
		t.Fatalf("server received %d requests, want 0", got)
	}

	if _, err := client.GetApp(context.Background(), "123"); err != nil {
		t.Fatalf("GetApp() error = %v, want nil", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests after GET, want 1", got)
	}
}

func TestReadOnlyModeAllowsAnalyticsPostQueries(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"size":0,"results":[]}`))
	}))
	t.Cleanup(server.Close)
	client := &Client{httpClient: server.Client(), baseURL: server.URL + "/analytics/api/v1"}

	_, err := client.GetAnalyticsMeasures(context.Background(), AnalyticsMeasuresRequest{
		AppID:     "123",
		Measures:  []string{"units"},
		Frequency: "day",
		StartTime: "2026-01-01T00:00:00Z",
		EndTime:   "2026-01-02T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("GetAnalyticsMeasures() error = %v, want nil (POST analytics query is a read)", err)
	}
	if got := sent.Load(); got != 1 {
		t.Fatalf("server received %d requests, want 1", got)
	}
}

func TestReadOnlyModeDeveloperPortalProxyReadsPassAndWritesAreRefused(t *testing.T) {
	t.Setenv(readonly.EnvVar, "1")

	var sent atomic.Int32
	client := developerPortalTestClient(t, func(r *http.Request) (*http.Response, error) {
		sent.Add(1)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == developerPortalTeamsPath:
			return developerPortalTestResponse(http.StatusOK, developerPortalTeamsFixture(), nil), nil
		case r.Method == http.MethodPost && r.URL.Path == developerServicesPath+"/cloudContainers":
			if got := r.Header.Get("X-HTTP-Method-Override"); got != http.MethodGet {
				t.Errorf("method override = %q, want GET", got)
			}
			return developerPortalTestResponse(http.StatusOK, `{"data":[],"meta":{"paging":{"total":0,"limit":1000}}}`, nil), nil
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			return developerPortalTestResponse(http.StatusInternalServerError, `{}`, nil), nil
		}
	})

	if _, err := client.ListDeveloperICloudContainers(context.Background(), false); err != nil {
		t.Fatalf("ListDeveloperICloudContainers() error = %v, want nil (proxied GET is a read)", err)
	}
	if got := sent.Load(); got != 2 {
		t.Fatalf("server received %d requests, want 2 (team session + proxied list)", got)
	}

	_, err := client.doDeveloperPortalRequest(context.Background(), http.MethodPost, "/bundleIds", map[string]string{"teamId": "TEAM123456"}, developerPortalHeaders(""), false)
	if !errors.Is(err, readonly.ErrRefused) {
		t.Fatalf("Developer Portal POST error = %v, want readonly.ErrRefused", err)
	}
	if got := sent.Load(); got != 2 {
		t.Fatalf("server received %d requests after refused write, want 2", got)
	}
}
